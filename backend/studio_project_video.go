package main

// Project video: a Studio clip goes into a project together with a short brief, and the project's
// coding agent finishes it (words behind the person, cards, real app screens) in the Glowbom house
// style. Glowbom does a quick local analysis first so the agent starts from facts, not guesses.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed project_video_tools/*.swift
var projectVideoTools embed.FS

type projectVideoSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type projectVideoAnalysis struct {
	DurationSeconds float64               `json:"durationSeconds,omitempty"`
	Width           int                   `json:"width,omitempty"`
	Height          int                   `json:"height,omitempty"`
	FrameRate       float64               `json:"frameRate,omitempty"`
	HasAudio        bool                  `json:"hasAudio"`
	Words           string                `json:"words,omitempty"`
	Speech          []projectVideoSegment `json:"speech,omitempty"`
	Brightness      int                   `json:"backgroundBrightness,omitempty"`
	BrightnessLeft  int                   `json:"backgroundBrightnessLeft,omitempty"`
	BrightnessRight int                   `json:"backgroundBrightnessRight,omitempty"`
	Ink             string                `json:"textColor,omitempty"`
}

var projectVideoSilence = regexp.MustCompile(`silence_(start|end): (-?[0-9.]+)`)

// Facts the agent needs: size and length, the spoken words when Studio knows them, when the person
// speaks (from the pauses), and whether white or black words will read on the background.
func analyzeProjectVideo(ctx context.Context, path, words string) projectVideoAnalysis {
	analysis := projectVideoAnalysis{Words: strings.TrimSpace(words)}
	ffmpeg, ffprobe, err := studioClipTools()
	if err != nil {
		return analysis
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if info, err := probeStudioClip(ctx, ffprobe, path); err == nil {
		analysis.DurationSeconds, analysis.Width, analysis.Height = round2(info.DurationSeconds), info.Width, info.Height
		analysis.FrameRate, analysis.HasAudio = round2(info.frameRate), info.HasAudio
		// Phone recordings store landscape pixels plus a rotation; describe the frames as they play.
		if out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream_side_data=rotation", "-of", "csv=p=0", path).Output(); err == nil {
			if rotation, err := strconv.Atoi(strings.TrimSpace(strings.Split(string(out), "\n")[0])); err == nil && (rotation%180+180)%180 == 90 {
				analysis.Width, analysis.Height = analysis.Height, analysis.Width
			}
		}
	}
	if analysis.HasAudio {
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostats", "-i", path, "-af", "silencedetect=noise=-30dB:d=0.25", "-f", "null", "-")
		cmd.Stderr = &stderr
		if cmd.Run() == nil {
			analysis.Speech = projectVideoSpeech(stderr.String(), analysis.DurationSeconds)
		}
	}
	// Brightness of the left and right edges of the middle frame, where the background usually is.
	if analysis.DurationSeconds > 0 {
		cmd := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", strconv.FormatFloat(analysis.DurationSeconds/2, 'f', 2, 64), "-i", path,
			"-frames:v", "1", "-vf", "scale=64:36,format=gray", "-f", "rawvideo", "-")
		if pixels, err := cmd.Output(); err == nil && len(pixels) == 64*36 {
			left, right := 0, 0
			for y := 0; y < 36; y++ {
				for x := 0; x < 12; x++ {
					left += int(pixels[y*64+x])
					right += int(pixels[y*64+52+x])
				}
			}
			analysis.BrightnessLeft, analysis.BrightnessRight = left/(36*12), right/(36*12)
			analysis.Brightness = (analysis.BrightnessLeft + analysis.BrightnessRight) / 2
			analysis.Ink = "white"
			if analysis.Brightness > 170 {
				analysis.Ink = "black"
			}
		}
	}
	return analysis
}

// Turns ffmpeg's silence log into the stretches where someone is speaking.
func projectVideoSpeech(log string, duration float64) []projectVideoSegment {
	segments := []projectVideoSegment{}
	speechStart, inSilence := 0.0, false
	for _, match := range projectVideoSilence.FindAllStringSubmatch(log, -1) {
		at, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		at = math.Max(0, at)
		if match[1] == "start" && !inSilence {
			if at-speechStart >= 0.15 {
				segments = append(segments, projectVideoSegment{Start: round2(speechStart), End: round2(at)})
			}
			inSilence = true
		} else if match[1] == "end" && inSilence {
			speechStart, inSilence = at, false
		}
	}
	if !inSilence && duration-speechStart >= 0.15 {
		segments = append(segments, projectVideoSegment{Start: round2(speechStart), End: round2(duration)})
	}
	return segments
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

// The words of a Seedance talking video are saved as its quoted title.
func projectVideoWords(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if strings.HasPrefix(prompt, "“") && strings.HasSuffix(prompt, "”") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(prompt, "“"), "”"))
	}
	return ""
}

var projectVideoTemplates = map[string]string{
	"launch": `## Template: Launch video
Introduce this project in the length of the clip. The person talking is the spine of the video.
1. Key words behind the person at 2–4 speech moments: one to three words each, taken from what is said. Split a word or short phrase around the head (for example "GLOW | BOM"). The project's name in capitals.
2. If you can capture it quickly, add one 2–3 second moment of the real app (a screenshot or short screen recording of this project running) as a rounded card beside the person. Real app footage is what makes a launch video credible.
3. An end card for the last 1.5 seconds, starting after the last speech if the person stops before the end. While the person is still talking, place it beside them, never over the face. It holds the product's name, its icon if the project has one, and one short call to action (its URL or "Try it").
For the name, use what the person says in the clip or the request; the project folder's name may be a working title.`,
	"custom": `## Template: Custom
Follow the request below. Keep the house style and the rules for the clip.`,
}

func projectVideoBrief(clip, output, toolsDir, template, request string, analysis projectVideoAnalysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Project video\n\nMake a short video for this project from the clip below. Work only inside `media/`; do not change any other file.\n\n")
	fmt.Fprintf(&b, "## Clip\n\n- File: `%s`\n", clip)
	if analysis.Width > 0 {
		fmt.Fprintf(&b, "- %d×%d, %.2f seconds, %.0f fps, %s\n", analysis.Width, analysis.Height, analysis.DurationSeconds, analysis.FrameRate, map[bool]string{true: "with sound", false: "no sound"}[analysis.HasAudio])
	}
	if analysis.Words != "" {
		fmt.Fprintf(&b, "- Spoken words: “%s”\n", strings.Join(strings.Fields(analysis.Words), " "))
	}
	if len(analysis.Speech) > 0 {
		fmt.Fprintf(&b, "- Speech (seconds, from the pauses):")
		for _, segment := range analysis.Speech {
			fmt.Fprintf(&b, " %.2f–%.2f", segment.Start, segment.End)
		}
		fmt.Fprintf(&b, ". Match the spoken words to these stretches in order.\n")
	}
	if analysis.Ink != "" {
		fmt.Fprintf(&b, "- Background brightness about %d/255 on the left and %d/255 on the right, so use **%s** words (check the side you place them on).\n", analysis.BrightnessLeft, analysis.BrightnessRight, analysis.Ink)
	}
	fmt.Fprintf(&b, "\n%s\n", projectVideoTemplates[template])
	if request = strings.TrimSpace(request); request != "" {
		fmt.Fprintf(&b, "\n## Request from the person\n\n%s\n", request)
	}
	fmt.Fprintf(&b, `
## House style (follow exactly)

- Font: the system font (SF Pro), weight 750, tight tracking (−0.04 to −0.055 em). Key words in capitals.
- Words behind the person: pure white, no outline, soft shadow. On a bright background (above about 170/255) use near-black #111 with a softer shadow.
- Cards: dark glass rgba(27,27,27,0.92), 2 px #2e2e2e border, 22–32 px corner radius, white title at weight 700, grey secondary text at weight 500, white pill button.
- One idea on screen at a time. Words fade in over 0.35 s and out over 0.25 s, during the matching speech.
- Never green, never outlined, stretched or tilted type, no emoji, no busy multi-word graphics, no generated text inside images.
- Keep every letter inside the frame and off the eyes and mouth. Keep the clip's own sound.

## Method that works on a Mac

Tools are in `+"`%s`"+`.
1. Frames: `+"`ffmpeg -i CLIP frames/%%04d.png`"+`.
2. Person masks with Apple Vision: `+"`swift %s/person_matte.swift frames masks`"+` (white is the person).
3. Words as transparent PNGs in the house style: `+"`swift %s/render_text.swift \"GLOW\" glow.png --size 120 --ink white`"+` (or `+"`--ink black`"+`).
4. Measure the person's left and right edges in the masks at the height of the words, for each moment (the head moves). Tuck slanted or round letter edges (W, Y, V, A, O) about 15 px behind the person and upright strokes (B, D, H, I, L) only about 5 px, so no letter loses its shape; check close-ups of each edge. Keep every letter inside the frame; make the words smaller, or place them lower beside the neck, if they do not fit.
5. Layer with ffmpeg: the clip, then the words (looped PNGs with `+"`fade=...:alpha=1`"+` and `+"`overlay`"+`), then the person on top (`+"`[clip][masks]alphamerge`"+`), then the cards on top of everything. Encode H.264 with AAC at the clip's size and frame rate.
Without Apple Vision (Windows, Linux), put the words beside the person instead of behind.

## Finish

- Render to a temporary name in `+"`media/`"+` that starts with a dot (for example `+"`media/.render.mp4`"+`), and when it passes the checks below, move it to `+"`%s`"+`. Studio adds that file to the person's library as soon as it appears, so only the finished video may have that name. Leave the original clip unchanged.
- Check a contact sheet of the whole result (`+"`ffmpeg -i OUT -vf fps=%s,scale=320:-1,tile=6x2 -frames:v 1 sheet.png`"+`, 12 frames across the clip) and fix anything cut off or covering the face.
- Delete temporary frames and masks. Reply with the output path and one sentence about what you added.
`, toolsDir, toolsDir, toolsDir, output, projectVideoSheetRate(analysis.DurationSeconds))
	return b.String()
}

const projectVideoMediaDir = "media"

var projectVideoNameUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// A short, readable file name from the clip's title, made unique by the asset ID.
func projectVideoBaseName(record studioImageRecord) string {
	title := strings.ToLower(strings.TrimSpace(projectVideoWords(record.Prompt)))
	if title == "" {
		title = strings.ToLower(strings.TrimSpace(record.Prompt))
	}
	slug := strings.Trim(projectVideoNameUnsafe.ReplaceAllString(title, "-"), "-")
	if words := strings.Split(slug, "-"); len(words) > 5 {
		slug = strings.Join(words[:5], "-")
	}
	if slug == "" {
		slug = "clip"
	}
	id := strings.ToLower(normalizedStudioUUID(record.ID))
	if len(id) >= 8 {
		id = id[:8]
	}
	return slug + "-" + id
}

type projectVideoPlan struct {
	RequestID string `json:"requestId,omitempty"`
	Clip      string `json:"clip"`
	Brief     string `json:"brief"`
	Output    string `json:"output"`
	Message   string `json:"message"`
	Project   string `json:"project"`
	// Studio only picks up an output written after this moment (Unix milliseconds).
	PreparedAt int64 `json:"preparedAt"`
}

// Copies the clip, the two helper tools and the brief into the project's media folder.
func prepareProjectVideo(ctx context.Context, id, projectPath, template, request string) (projectVideoPlan, error) {
	return prepareProjectVideoWithRequest(ctx, id, projectPath, template, request, "")
}

// A phone request reserves a distinct output even before the agent writes it.
func prepareProjectVideoWithRequest(ctx context.Context, id, projectPath, template, request, requestID string) (projectVideoPlan, error) {
	if _, ok := projectVideoTemplates[template]; !ok {
		template = "launch"
	}
	if len([]rune(request)) > 2000 {
		return projectVideoPlan{}, errors.New("Keep the request under 2,000 characters.")
	}
	record, err := findStudioAsset(id)
	if err != nil || record.MediaType != "video" {
		return projectVideoPlan{}, errors.New("Choose a saved Studio video.")
	}
	data, _, err := decodeBase64Payload(record.DataBase64, "video/mp4")
	if err != nil || len(data) == 0 || len(data) > studioClipSourceLimit {
		return projectVideoPlan{}, errors.New("This video cannot be opened. Videos up to 80 MB work.")
	}
	studioProjectMu.Lock()
	project, err := registerStudioProjectLocked(projectPath)
	studioProjectMu.Unlock()
	if err != nil {
		return projectVideoPlan{}, errors.New("Open a Glowbom project in chat first.")
	}
	media, err := chatWriteDirectory(project.Path, projectVideoMediaDir)
	if err != nil {
		return projectVideoPlan{}, errors.New("Could not create the media folder in this project.")
	}
	tools, err := chatWriteDirectory(project.Path, projectVideoMediaDir+"/.glowbom-tools")
	if err != nil {
		return projectVideoPlan{}, errors.New("Could not create the media folder in this project.")
	}
	base := projectVideoBaseName(record)
	clipName := base + ".mp4"
	if _, err := os.Lstat(filepath.Join(media, clipName)); errors.Is(err, os.ErrNotExist) {
		if err := createChatImageFile(media, clipName, data); err != nil {
			return projectVideoPlan{}, errors.New("Could not copy the video into this project.")
		}
	}
	for _, name := range []string{"person_matte.swift", "render_text.swift"} {
		script, _ := projectVideoTools.ReadFile("project_video_tools/" + name)
		if err := atomicChatFile(tools, name, script); err != nil {
			return projectVideoPlan{}, errors.New("Could not copy the video tools into this project.")
		}
	}
	// Every run gets its own output, so an earlier result is never overwritten or mistaken for this one.
	suffix := map[string]string{"launch": "launch", "custom": "video"}[template]
	if requestID != "" {
		if normalizedStudioUUID(requestID) == "" {
			return projectVideoPlan{}, errors.New("Invalid project video request ID.")
		}
		suffix += "-" + strings.ToLower(requestID)
	}
	outputName := base + "-" + suffix + ".mp4"
	for n := 2; n < 100; n++ {
		if _, err := os.Lstat(filepath.Join(media, outputName)); errors.Is(err, os.ErrNotExist) {
			break
		}
		outputName = fmt.Sprintf("%s-%s-%d.mp4", base, suffix, n)
	}
	plan := projectVideoPlan{
		Clip: projectVideoMediaDir + "/" + clipName, Brief: projectVideoMediaDir + "/" + strings.TrimSuffix(outputName, ".mp4") + ".project-video.md",
		Output: projectVideoMediaDir + "/" + outputName, Project: project.Path, PreparedAt: time.Now().UnixMilli(),
	}
	analysis := analyzeProjectVideo(ctx, filepath.Join(media, clipName), projectVideoWords(record.Prompt))
	brief := projectVideoBrief(plan.Clip, plan.Output, projectVideoMediaDir+"/.glowbom-tools", template, request, analysis)
	if err := atomicChatFile(media, filepath.Base(plan.Brief), []byte(brief)); err != nil {
		return projectVideoPlan{}, errors.New("Could not write the brief into this project.")
	}
	kind := map[string]string{"launch": "a launch video", "custom": "a project video"}[template]
	plan.Message = fmt.Sprintf("Make %s from `%s`. Follow the brief in `%s` exactly, work only in `media/`, and save the result as `%s`.", kind, plan.Clip, plan.Brief, plan.Output)
	return plan, nil
}

var (
	projectVideoImportMu    sync.Mutex
	errProjectVideoTooLarge = errors.New("The project video is over 80 MB, so it was not added to Studio. Ask the agent to export a smaller file.")
)

// Adds the agent's finished video to Studio once. The same file always gets the same Studio ID.
// since (Unix milliseconds) skips files from before the request, such as an earlier run's result.
func importProjectVideoResult(ctx context.Context, projectPath, output string, since int64) (*studioImageSummary, error) {
	projectVideoImportMu.Lock()
	defer projectVideoImportMu.Unlock()
	if !strings.HasPrefix(output, projectVideoMediaDir+"/") || !strings.HasSuffix(output, ".mp4") ||
		!filepath.IsLocal(output) || filepath.ToSlash(filepath.Clean(output)) != output || strings.Contains(output, `\`) {
		return nil, errors.New("Unknown project video.")
	}
	studioProjectMu.Lock()
	project, err := registerStudioProjectLocked(projectPath)
	studioProjectMu.Unlock()
	if err != nil {
		return nil, errors.New("Open the project in chat first.")
	}
	info, err := os.Lstat(filepath.Join(project.Path, filepath.FromSlash(output)))
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.ModTime().UnixMilli() < since {
		return nil, nil // the agent has not saved this run's video yet
	}
	// A file still being written changes size; wait until it has been still for a few seconds.
	if time.Since(info.ModTime()) < 3*time.Second {
		return nil, nil
	}
	if info.Size() > studioClipSourceLimit {
		return nil, errProjectVideoTooLarge
	}
	digest := sha256.Sum256([]byte(project.ID + "\x00" + output + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + info.ModTime().UTC().Format(time.RFC3339Nano)))
	generationID := "project-video-" + hex.EncodeToString(digest[:12])
	if existing, err := findStudioAsset(studioGenerationAssetID(generationID)); err == nil {
		summary := summarizeStudioRecord(existing)
		return &summary, nil
	}
	// Only a complete, playable video counts; anything else is still being made.
	var media studioClipMediaInfo
	if _, ffprobe, err := studioClipTools(); err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		media, err = probeStudioClip(probeCtx, ffprobe, filepath.Join(project.Path, filepath.FromSlash(output)))
		cancel()
		if err != nil || media.DurationSeconds <= 0 {
			return nil, nil
		}
	}
	data, err := readCompanionProjectMedia(project.Path, output, studioClipSourceLimit)
	if err != nil || int64(len(data)) != info.Size() {
		return nil, nil // changed while reading; try again later
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	name := strings.TrimSuffix(filepath.Base(output), ".mp4")
	options := studioSaveOptions{
		GenerationID: generationID, Prompt: "Project video: " + name, DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(data),
		MediaType: "video", MimeType: "video/mp4", Source: "Project video", AssetType: "generated", SourceType: "generated",
		SourceProjectID: project.ID, UsedInProjects: []string{project.ID}, Duration: media.DurationSeconds,
	}
	if media.Width > 0 && media.Height > 0 {
		options.Dimensions = &studioDimensions{Width: media.Width, Height: media.Height}
	}
	saved, err := saveStudioAsset(options)
	if err != nil {
		return nil, errors.New("Could not save the project video in Studio.")
	}
	summary := summarizeStudioRecord(saved)
	return &summary, nil
}

// POST /studio/project-video {id, path, template, prompt} → {clip, brief, output, message}.
// GET /studio/project-video?path=…&output=… → {asset} once the agent has saved the video, else {pending: true}.
func studioProjectVideoHandler(w http.ResponseWriter, r *http.Request) {
	method := http.MethodGet
	if r.Method == http.MethodPost {
		method = http.MethodPost
	}
	if !authorizeStudioGeneration(w, r, method) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		if requestID := r.URL.Query().Get("requestId"); requestID != "" {
			result, err := reconcileBookLaunchVideo(r.Context(), r.URL.Query().Get("path"), requestID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, result)
			return
		}
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		asset, err := importProjectVideoResult(r.Context(), r.URL.Query().Get("path"), r.URL.Query().Get("output"), since)
		if errors.Is(err, errProjectVideoTooLarge) {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if asset == nil {
			writeJSON(w, map[string]any{"pending": true})
			return
		}
		writeJSON(w, map[string]any{"asset": asset})
		return
	}
	var request struct {
		ID         string                 `json:"id"`
		Path       string                 `json:"path"`
		Template   string                 `json:"template"`
		Prompt     string                 `json:"prompt"`
		RequestID  string                 `json:"requestId,omitempty"`
		BookTarget *bookLaunchVideoTarget `json:"bookTarget,omitempty"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request) != nil {
		http.Error(w, "Invalid project video request.", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	var plan projectVideoPlan
	var err error
	if request.BookTarget != nil {
		plan, err = prepareBookLaunchVideo(ctx, request.ID, request.Path, request.Template, request.Prompt, request.RequestID, *request.BookTarget)
	} else {
		plan, err = prepareProjectVideo(ctx, request.ID, request.Path, request.Template, request.Prompt)
	}
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errProjectBookConflict) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, plan)
}

// Frames per second for a 12-frame contact sheet that spans the whole clip.
func projectVideoSheetRate(duration float64) string {
	if duration <= 0 {
		return "1"
	}
	return strconv.FormatFloat(math.Max(0.05, 12/duration), 'f', 2, 64)
}
