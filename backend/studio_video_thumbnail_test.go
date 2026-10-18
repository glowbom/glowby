package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func studioThumbnailTestJPEG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			canvas.Set(x, y, color.RGBA{230, 70, 40, 255})
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func studioThumbnailFixture(t *testing.T) (studioImageRecord, *studioVideoThumbnailService) {
	t.Helper()
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	t.Setenv("GLOWBOM_SERVER_TOKEN", "thumbnail-test-token")
	t.Setenv("GLOWBOM_ALLOWED_ORIGINS", "http://localhost:4572")
	asset := bookVideoTestAsset(t, "")
	previous := studioVideoThumbnails
	s := newStudioVideoThumbnailService()
	studioVideoThumbnails = s
	t.Cleanup(func() { studioVideoThumbnails = previous })
	return asset, s
}

func studioThumbnailRequest(method, id string) *http.Request {
	r := httptest.NewRequest(method, "/studio/videos/thumbnail?id="+id, nil)
	r.Header.Set("X-Glowbom-Token", "thumbnail-test-token")
	r.Header.Set("Origin", "http://localhost:4572")
	return r
}

func TestStudioVideoThumbnailCachesBoundedJPEGAndInvalidatesChangedSource(t *testing.T) {
	asset, s := studioThumbnailFixture(t)
	jpeg := studioThumbnailTestJPEG(t)
	var renders atomic.Int32
	s.render = func(context.Context, string, string) ([]byte, error) { renders.Add(1); return jpeg, nil }
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		studioVideoThumbnailHandler(w, studioThumbnailRequest(http.MethodGet, strings.ToLower(asset.ID)))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(w.Body.Bytes(), jpeg) {
			t.Fatal("thumbnail response", w.Code, w.Header())
		}
		if bytes.Contains(w.Body.Bytes(), []byte("private-test-video")) || w.Body.Len() > studioVideoThumbnailLimit {
			t.Fatal("original media exposed")
		}
	}
	if renders.Load() != 1 {
		t.Fatal("unchanged source was decoded again")
	}
	path, _, err := studioVideoThumbnailRecord(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.load(context.Background(), asset.ID); err != nil || renders.Load() != 2 {
		t.Fatal("changed record reused stale thumbnail", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	studioVideoThumbnailHandler(w, studioThumbnailRequest(http.MethodGet, asset.ID))
	if w.Code != http.StatusNotFound {
		t.Fatal("deleted video returned a cached preview")
	}
}

func TestStudioVideoThumbnailAuthenticatesBeforeReadingOrRendering(t *testing.T) {
	asset, s := studioThumbnailFixture(t)
	var calls atomic.Int32
	s.render = func(context.Context, string, string) ([]byte, error) { calls.Add(1); return nil, nil }
	for _, kind := range []string{"token", "origin", "cross-site", "method", "path", "missing"} {
		t.Run(kind, func(t *testing.T) {
			r := studioThumbnailRequest(http.MethodGet, asset.ID)
			switch kind {
			case "token":
				r.Header.Del("X-Glowbom-Token")
			case "origin":
				r.Header.Set("Origin", "https://untrusted.example")
			case "cross-site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			case "method":
				r.Method = http.MethodPost
			case "path":
				r.URL.RawQuery = "id=..%2Fprivate"
			case "missing":
				r.URL.RawQuery = "id=" + randomUUIDString()
			}
			w := httptest.NewRecorder()
			studioVideoThumbnailHandler(w, r)
			if w.Code < 400 || calls.Load() != 0 {
				t.Fatal("invalid thumbnail request rendered media", w.Code)
			}
		})
	}
}

func TestStudioVideoThumbnailRejectsLinkedRecordsAndOversizedOutput(t *testing.T) {
	asset, s := studioThumbnailFixture(t)
	path, _, _ := studioVideoThumbnailRecord(asset.ID)
	original := path + ".original"
	if err := os.Rename(path, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.load(context.Background(), asset.ID); !os.IsNotExist(err) {
		t.Fatal("linked record accepted", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, path); err != nil {
		t.Fatal(err)
	}
	s.render = func(context.Context, string, string) ([]byte, error) {
		return make([]byte, studioVideoThumbnailLimit+1), nil
	}
	if _, err := s.load(context.Background(), asset.ID); err == nil {
		t.Fatal("oversized thumbnail accepted")
	}
	if len(s.cache) != 0 {
		t.Fatal("invalid thumbnail was cached")
	}
}

func TestStudioVideoThumbnailCancellationStopsOwnerAndDoesNotCancelWaiter(t *testing.T) {
	asset, s := studioThumbnailFixture(t)
	jpeg := studioThumbnailTestJPEG(t)
	started := make(chan struct{})
	var calls atomic.Int32
	s.render = func(ctx context.Context, _ string, _ string) ([]byte, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return jpeg, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	owner := make(chan error, 1)
	go func() { _, err := s.load(ctx, asset.ID); owner <- err }()
	<-started
	waiter := make(chan error, 1)
	go func() { _, err := s.load(context.Background(), asset.ID); waiter <- err }()
	cancel()
	if err := <-owner; err != context.Canceled {
		t.Fatal("owner ignored cancellation", err)
	}
	select {
	case err := <-waiter:
		if err != nil {
			t.Fatal("independent preview canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter never resumed")
	}
	if calls.Load() != 2 {
		t.Fatal("waiter did not retry canceled extraction")
	}
}

func TestStudioVideoThumbnailDeduplicatesConcurrentRequestsAndBoundsCache(t *testing.T) {
	asset, s := studioThumbnailFixture(t)
	jpeg := studioThumbnailTestJPEG(t)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.render = func(context.Context, string, string) ([]byte, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return jpeg, nil
	}
	results := make(chan error, 2)
	go func() { _, err := s.load(context.Background(), asset.ID); results <- err }()
	<-started
	go func() { _, err := s.load(context.Background(), asset.ID); results <- err }()
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate thumbnail processes ran")
	}
	for i := 0; i < studioVideoThumbnailEntries; i++ {
		next := bookVideoTestAsset(t, "")
		if _, err := s.load(context.Background(), next.ID); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.cache) != studioVideoThumbnailEntries {
		t.Fatal("thumbnail memory cache exceeded its limit", len(s.cache))
	}
}

func TestStudioVideoThumbnailUnavailableToolsDoNotStartSetup(t *testing.T) {
	asset, _ := studioThumbnailFixture(t)
	t.Setenv("GLOWBOM_FFMPEG_BIN", "/missing/thumbnail-test-tools")
	t.Setenv("GLOWBOM_FFPROBE_BIN", "/missing/thumbnail-test-tools")
	w := httptest.NewRecorder()
	studioVideoThumbnailHandler(w, studioThumbnailRequest(http.MethodGet, asset.ID))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "local video tools") {
		t.Fatal("missing tools were not recoverable", w.Code, w.Body.String())
	}
}

func TestStudioVideoThumbnailExtractsActualFirstFrameWithoutOriginalPayload(t *testing.T) {
	_, s := studioThumbnailFixture(t)
	ffmpeg, _, err := studioClipTools()
	if err != nil {
		t.Skip("local video tools unavailable")
	}
	path := filepath.Join(t.TempDir(), "portrait.mp4")
	if err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=red:size=360x640:rate=24:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", path).Run(); err != nil {
		t.Skip("cannot create local test video")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := saveStudioAsset(studioSaveOptions{Prompt: "thumbnail fixture", MediaType: "video", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		t.Fatal(err)
	}
	thumbnail, err := s.load(context.Background(), asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := jpeg.Decode(bytes.NewReader(thumbnail))
	if err != nil {
		t.Fatal(err)
	}
	if frame.Bounds().Dx() > 384 || frame.Bounds().Dy() != 384 || frame.Bounds().Dx() >= frame.Bounds().Dy() {
		t.Fatal("thumbnail lost portrait framing", frame.Bounds())
	}
	r, g, b, _ := frame.At(frame.Bounds().Dx()/2, frame.Bounds().Dy()/2).RGBA()
	if r < 40000 || g > 10000 || b > 10000 {
		t.Fatal("thumbnail is not the source frame")
	}
	cacheRoot, _ := studioClipPrivateDirectory("VideoThumbnails")
	leftovers, _ := os.ReadDir(cacheRoot)
	if len(leftovers) != 0 {
		t.Fatal("decoded full video was left behind")
	}
	imageAsset, _ := studioTestAsset(t)
	if _, err := s.load(context.Background(), imageAsset.ID); !os.IsNotExist(err) {
		t.Fatal("image accepted as video", err)
	}
}
