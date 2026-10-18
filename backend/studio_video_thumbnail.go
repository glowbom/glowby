package main

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const studioVideoThumbnailLimit = 256 << 10
const studioVideoThumbnailEdge = 384
const studioVideoThumbnailEntries = 64

var errStudioVideoThumbnailUnavailable = errors.New("Video thumbnails need the local video tools.")
var studioVideoThumbnails = newStudioVideoThumbnailService()

type studioVideoThumbnailEntry struct {
	stamp os.FileInfo
	data  []byte
	used  uint64
}

type studioVideoThumbnailService struct {
	mu       sync.Mutex
	cache    map[string]studioVideoThumbnailEntry
	pending  map[string]chan struct{}
	sequence uint64
	slots    chan struct{}
	render   func(context.Context, string, string) ([]byte, error)
}

func newStudioVideoThumbnailService() *studioVideoThumbnailService {
	return &studioVideoThumbnailService{cache: make(map[string]studioVideoThumbnailEntry), pending: make(map[string]chan struct{}), slots: make(chan struct{}, 2), render: renderStudioVideoThumbnail}
}

// Resolve a Studio ID, never a caller-provided filesystem path. Native and older
// records can have either a timestamp prefix or just their UUID as the filename.
func studioVideoThumbnailRecord(id string) (string, os.FileInfo, error) {
	id = normalizedStudioUUID(id)
	if id == "" {
		return "", nil, os.ErrNotExist
	}
	dir, err := studioAssetsDirectory()
	if err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if entry.IsDir() || !(name == id+".JSON" || strings.HasSuffix(name, "_"+id+".JSON")) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > studioClipRecordLimit {
			return "", nil, os.ErrNotExist
		}
		return path, info, nil
	}
	return "", nil, os.ErrNotExist
}

func sameStudioVideoThumbnailSource(a, b os.FileInfo) bool {
	return a != nil && b != nil && b.Mode().IsRegular() && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (s *studioVideoThumbnailService) load(ctx context.Context, id string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path, stamp, err := studioVideoThumbnailRecord(id)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		if cached, ok := s.cache[path]; ok && sameStudioVideoThumbnailSource(cached.stamp, stamp) {
			s.sequence++
			cached.used = s.sequence
			s.cache[path] = cached
			s.mu.Unlock()
			return cached.data, nil
		}
		delete(s.cache, path)
		if pending := s.pending[path]; pending != nil {
			s.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		ready := make(chan struct{})
		s.pending[path] = ready
		s.mu.Unlock()
		data, err := s.generate(ctx, path, id)
		current, statErr := os.Lstat(path)
		if err == nil && (statErr != nil || !sameStudioVideoThumbnailSource(stamp, current)) {
			data, err = nil, os.ErrNotExist
		}
		s.mu.Lock()
		if err == nil {
			for len(s.cache) >= studioVideoThumbnailEntries {
				oldest := ""
				var order uint64
				for key, entry := range s.cache {
					if oldest == "" || entry.used < order {
						oldest, order = key, entry.used
					}
				}
				delete(s.cache, oldest)
			}
			s.sequence++
			s.cache[path] = studioVideoThumbnailEntry{stamp: current, data: data, used: s.sequence}
		}
		delete(s.pending, path)
		close(ready)
		s.mu.Unlock()
		return data, err
	}
}

func (s *studioVideoThumbnailService) generate(ctx context.Context, path, id string) ([]byte, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	data, err := s.render(ctx, path, id)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > studioVideoThumbnailLimit {
		return nil, errors.New("The video thumbnail is unavailable.")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > studioVideoThumbnailEdge || config.Height > studioVideoThumbnailEdge {
		return nil, errors.New("The video thumbnail is unavailable.")
	}
	return data, nil
}

func renderStudioVideoThumbnail(ctx context.Context, path, id string) ([]byte, error) {
	ffmpeg, _, err := studioClipTools()
	if err != nil {
		return nil, errStudioVideoThumbnailUnavailable
	}
	dir, err := studioClipPrivateDirectory("VideoThumbnails")
	if err != nil {
		return nil, errors.New("The video thumbnail folder is unavailable.")
	}
	file, err := os.CreateTemp(dir, ".source-*.mp4")
	if err != nil {
		return nil, errors.New("Could not prepare this video thumbnail.")
	}
	defer os.Remove(file.Name())
	metadata, readErr := readStudioClipEnvelope(ctx, path, file)
	closeErr := file.Close()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if readErr != nil || closeErr != nil || metadata.MediaType != "video" || !strings.EqualFold(metadata.ID, id) {
		return nil, os.ErrNotExist
	}
	cmd := studioClipCommand(ctx, ffmpeg, "-v", "error", "-nostdin", "-threads", "2", "-protocol_whitelist", "file,pipe",
		"-format_whitelist", "mov,matroska,webm,avi,mpegts,mpeg,ogg,flv", "-i", file.Name(), "-map", "0:V:0", "-frames:v", "1", "-an", "-sn",
		"-vf", "scale=384:384:force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1", "-map_metadata", "-1", "-map_chapters", "-1",
		"-c:v", "mjpeg", "-threads", "2", "-q:v", "5", "-f", "image2pipe", "pipe:1")
	cmd.WaitDelay = time.Second
	configureCursorCancellation(cmd)
	output, stderr := &studioClipBoundedOutput{limit: studioVideoThumbnailLimit}, &studioClipBoundedOutput{limit: 4 << 10}
	cmd.Stdout, cmd.Stderr = output, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("Could not preview this video.")
	}
	if output.overflow {
		return nil, errors.New("The video thumbnail is too large.")
	}
	return output.data, nil
}

func studioVideoThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if !authorizeStudioClip(w, r, http.MethodGet) {
		return
	}
	id := normalizedStudioUUID(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Video not found.", http.StatusNotFound)
		return
	}
	data, err := studioVideoThumbnails.load(r.Context(), id)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		status, message := http.StatusUnprocessableEntity, "A preview is unavailable for this video."
		if os.IsNotExist(err) {
			status, message = http.StatusNotFound, "Video not found."
		}
		if errors.Is(err, errStudioVideoThumbnailUnavailable) {
			status, message = http.StatusServiceUnavailable, errStudioVideoThumbnailUnavailable.Error()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			status, message = http.StatusGatewayTimeout, "The video preview took too long. Try again."
		}
		http.Error(w, message, status)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Keep browser storage empty; only the bounded local memory cache retains thumbnails.
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}
