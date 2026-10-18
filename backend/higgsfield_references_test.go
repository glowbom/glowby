package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestHiggsfieldSeedanceTakesMovementAndVoiceReferences(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	movement, ffprobe := higgsfieldMovementWithAudioFixture(t)
	clip, err := saveStudioAsset(studioSaveOptions{Prompt: "me at my desk", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(movement), MediaType: "video", AssetType: "reference", SourceType: "uploaded", Duration: 1})
	if err != nil {
		t.Fatal(err)
	}
	long, _ := saveStudioAsset(studioSaveOptions{Prompt: "long", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("x")), MediaType: "video", Duration: 95})
	voice, err := saveStudioAsset(studioSaveOptions{Prompt: "line", DataURI: "data:audio/mpeg;base64," + base64.StdEncoding.EncodeToString([]byte("ID3voice")), MediaType: "audio", AudioType: "voice"})
	if err != nil {
		t.Fatal(err)
	}

	base := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 10, Resolution: "480p", AspectRatio: "16:9"}
	for _, bad := range []studioVideoOptions{
		{SourceID: higgsfieldSourceID, ModelID: "kling3_0", MotionVideoID: clip.ID},
		{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", MotionVideoID: long.ID},
		{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", MotionVideoID: voice.ID},
		{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", VoiceAudioID: clip.ID},
	} {
		if validateHiggsfieldVideoReferences(bad) == nil {
			t.Fatalf("references should be rejected: %+v", bad)
		}
	}

	options := base
	options.MotionVideoID, options.VoiceAudioID, options.SpokenText = clip.ID, voice.ID, "Introducing Glowbom."
	start, err := startHiggsfieldVideo(context.Background(), options, "Same room, same mic.", nil)
	if err != nil || start.OperationID != "job-1" {
		t.Fatalf("start = %+v, %v", start, err)
	}
	var params map[string]any
	confirmed := []string{}
	for _, call := range fake.toolCalls {
		if call.name == "generate_video" {
			params, _ = call.arguments["params"].(map[string]any)
		}
		if call.name == "media_confirm" {
			confirmed = append(confirmed, call.arguments["type"].(string))
		}
	}
	if strings.Join(confirmed, ",") != "video,audio" {
		t.Fatalf("expected a video then an audio upload, got %v", confirmed)
	}
	if len(fake.uploads) != 2 {
		t.Fatalf("expected separate movement and voice files, got %d", len(fake.uploads))
	}
	assertHiggsfieldMovementStreams(t, fake.uploads[0], ffprobe, true)
	medias, _ := params["medias"].([]any)
	roles := []string{}
	for _, media := range medias {
		roles = append(roles, media.(map[string]any)["role"].(string))
	}
	if params["mode"] != "omni_reference" || strings.Join(roles, ",") != "video_references,audio_references" {
		t.Fatalf("unexpected params: mode=%v roles=%v", params["mode"], roles)
	}
	prompt, _ := params["prompt"].(string)
	if !strings.HasPrefix(prompt, "Same room, same mic.") || !strings.Contains(prompt, "Video 1") || !strings.Contains(prompt, "Audio 1") || !strings.Contains(prompt, "\u201cIntroducing Glowbom.\u201d") {
		t.Fatalf("prompt should explain each reference and carry the words: %q", prompt)
	}
	if !strings.Contains(prompt, "Video 1 is silent") || !strings.Contains(prompt, "Audio 1 is the only speaking-voice reference") {
		t.Fatalf("the chosen voice must take priority over the movement reference: %q", prompt)
	}
}

func higgsfieldMovementWithAudioFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	ffmpeg, ffprobe, err := studioClipTools()
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	source := filepath.Join(t.TempDir(), "movement.mp4")
	err = exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=black:size=64x64:rate=10", "-f", "lavfi", "-i", "sine=frequency=440", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", source).Run()
	if err != nil {
		t.Fatalf("could not create movement fixture: %v", err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	previous := higgsfieldReferenceFFmpeg
	higgsfieldReferenceFFmpeg = func() (string, error) { return ffmpeg, nil }
	t.Cleanup(func() { higgsfieldReferenceFFmpeg = previous })
	return data, ffprobe
}

func TestHiggsfieldSeparateVoiceRemovesMovementAudio(t *testing.T) {
	data, ffprobe := higgsfieldMovementWithAudioFixture(t)
	for _, silent := range []bool{false, true} {
		prepared, _, _, ok := higgsfieldPrepareReference(context.Background(), data, "movement.mp4", "video", silent)
		if !ok {
			t.Fatal("movement preparation failed")
		}
		assertHiggsfieldMovementStreams(t, prepared, ffprobe, silent)
	}
}

func assertHiggsfieldMovementStreams(t *testing.T, data []byte, ffprobe string, silent bool) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "prepared.mp4")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	streams, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", file).Output()
	if err != nil || !strings.Contains(string(streams), "video") || strings.Contains(string(streams), "audio") == silent {
		t.Fatalf("silent=%v streams=%q err=%v", silent, streams, err)
	}
}

func TestHiggsfieldCannotSubmitConflictingMovementAudioWhenPreparationFails(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	clip, err := saveStudioAsset(studioSaveOptions{DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("unprepared clip")), MediaType: "video", Duration: 1})
	if err != nil {
		t.Fatal(err)
	}
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16", MotionVideoID: clip.ID, HiggsfieldVoiceID: "preset-1", SpokenText: "Hello."}
	if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err == nil || !strings.Contains(err.Error(), "without sound") {
		t.Fatalf("expected movement preparation refusal, got %v", err)
	}
	if fake.submits != 0 || len(fake.uploaded) != 0 {
		t.Fatal("unprepared movement reached Higgsfield")
	}
}

func TestHiggsfieldSelectedCloneUsesItsOwnSampleAndNeverAnotherVoice(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	fake.cloneHasSample, fake.cloneStatus = true, "ready"
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16", HiggsfieldVoiceID: "clone-1", SpokenText: "Hello."}
	if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err != nil {
		t.Fatal(err)
	}
	if string(fake.uploaded) != "ID3selected-clone" {
		t.Fatalf("selected clone was replaced by another voice: %q", fake.uploaded)
	}
	before := fake.submits
	for _, status := range []string{"processing", "failed"} {
		fake.cloneStatus = status
		if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err == nil || !strings.Contains(err.Error(), "not ready") {
			t.Fatalf("unavailable selected voice must fail, status=%s error=%v", status, err)
		}
	}
	fake.cloneHasSample = false
	if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err == nil || !strings.Contains(err.Error(), "no sample") {
		t.Fatalf("missing selected sample must not fall back to a preset: %v", err)
	}
	if fake.submits != before {
		t.Fatal("unavailable clone started another generation")
	}
}

func TestHiggsfieldVideoKeepsSelectedVoiceInSavedMetadata(t *testing.T) {
	isolateStudioRecovery(t)
	fake := signInFakeHiggsfield(t)
	fake.cloneHasSample, fake.cloneStatus, fake.polls = true, "ready", 1
	id := "fixture-video-selected-voice"
	body := studioRecoveryJSON(t, map[string]any{
		"generationId": id, "prompt": "A quiet room.", "sourceId": higgsfieldSourceID,
		"modelId": "seedance_2_5", "durationSeconds": 5, "resolution": "720p", "aspectRatio": "9:16",
		"higgsfieldVoiceId": "clone-1", "spokenText": "Hello.",
	})
	w := httptest.NewRecorder()
	studioVideoGenerateHandler(w, studioProgressRequest(http.MethodPost, "/studio/videos/generate", body))
	if w.Code != http.StatusOK {
		t.Fatalf("generation failed: %d %s", w.Code, w.Body.String())
	}
	state := studioGenerations.read(id)
	if state.VoiceID != "clone-1" || state.Asset == nil {
		t.Fatalf("selected voice missing from request provenance: %+v", state)
	}
	record, err := findStudioAsset(state.Asset.ID)
	if err != nil || record.VoiceID != "clone-1" || record.MediaType != "video" || record.SourceID != higgsfieldSourceID || record.Model != "seedance_2_5" {
		t.Fatalf("selected voice changed video metadata: voice=%q type=%q source=%q model=%q err=%v", record.VoiceID, record.MediaType, record.SourceID, record.Model, err)
	}
	if restored := newStudioProgressStore().read(id); restored.VoiceID != "clone-1" {
		t.Fatal("saved generation lost selected voice after reload")
	}
}

func TestHiggsfieldTalkingVideoNeedsOnlyItsWords(t *testing.T) {
	prompt := higgsfieldReferencePrompt(studioVideoOptions{SpokenText: "Introducing Glowbom."}, "")
	if !strings.HasPrefix(prompt, "Live-action, real camera") || !strings.Contains(prompt, "\u201cIntroducing Glowbom.\u201d") {
		t.Fatalf("an empty description should get a plain scene line before the words: %q", prompt)
	}
}

func TestHiggsfieldVoiceSampleBecomesTheVoiceReference(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "480p", AspectRatio: "16:9", HiggsfieldVoiceID: "preset-1", SpokenText: "Hi."}
	if validateHiggsfieldVideoReferences(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", HiggsfieldVoiceID: "preset-1", VoiceAudioID: "x"}) == nil {
		t.Fatalf("only one voice reference should be allowed")
	}
	if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err != nil {
		t.Fatal(err)
	}
	if string(fake.uploaded) != "ID3sample" {
		t.Fatalf("the voice's sample should be uploaded as the reference, got %q", fake.uploaded)
	}
	var params map[string]any
	for _, call := range fake.toolCalls {
		if call.name == "generate_video" {
			params, _ = call.arguments["params"].(map[string]any)
		}
	}
	medias, _ := params["medias"].([]any)
	if len(medias) != 1 || medias[0].(map[string]any)["role"] != "audio_references" || !strings.Contains(params["prompt"].(string), "Audio 1") {
		t.Fatalf("expected one audio reference: %v", params)
	}
	options.HiggsfieldVoiceID = "missing"
	if _, err := startHiggsfieldVideo(context.Background(), options, "", nil); err == nil {
		t.Fatalf("a voice that is not in the account must fail before submitting")
	}
}

func TestHiggsfieldVideoThatDidNotStartSaysWhy(t *testing.T) {
	signInFakeHiggsfield(t)
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "480p", AspectRatio: "16:9", HiggsfieldVoiceID: "missing", SpokenText: "Hi."}
	_, err := generateStudioSelectedVideo(context.Background(), options, "", "", nil)
	var notStarted *higgsfieldNotStartedError
	if !errors.As(err, &notStarted) || !strings.Contains(err.Error(), "no longer in your Higgsfield account") {
		t.Fatalf("expected a not-started error with the reason, got %T %v", err, err)
	}
	// Words alone are enough for a talking video; the empty description must not be rejected first.
	options.HiggsfieldVoiceID = "preset-1"
	if start, err := generateStudioSelectedVideo(context.Background(), options, "", "", nil); err != nil || start.OperationID == "" {
		t.Fatalf("a talking video with only its words should start: %+v %v", start, err)
	}
}

func TestHiggsfieldRefusedVideoShowsHiggsfieldsReason(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	fake.refuseVideo = "The reference video is too short."
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "480p", AspectRatio: "1:1"}
	_, err := generateStudioSelectedVideo(context.Background(), options, "", "A quiet room.", nil)
	var notStarted *higgsfieldNotStartedError
	if !errors.As(err, &notStarted) || !strings.Contains(err.Error(), "The reference video is too short.") {
		t.Fatalf("a refusal should carry Higgsfield's reason as a not-started error, got %T %v", err, err)
	}
}

func TestHiggsfieldUploadRetriesAPassingCloudflareError(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	oldDelay := higgsfieldUploadRetryDelay
	higgsfieldUploadRetryDelay = 0
	t.Cleanup(func() { higgsfieldUploadRetryDelay = oldDelay })
	fake.failPuts = 2
	if _, err := higgsfieldUploadMedia(context.Background(), []byte("clip"), "a.mp4", "video/mp4", "video"); err != nil {
		t.Fatalf("two passing 520s should be retried: %v", err)
	}
	fake.failPuts = 3
	_, err := higgsfieldUploadMedia(context.Background(), []byte("clip"), "a.mp4", "video/mp4", "video")
	if err == nil || !strings.Contains(err.Error(), "HTTP 520") {
		t.Fatalf("three failures should report the last one: %v", err)
	}
}

func TestHiggsfieldLongMovementClipNeedsTrimmingWithoutFFmpeg(t *testing.T) {
	signInFakeHiggsfield(t) // tests run without ffmpeg
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	clip, _ := saveStudioAsset(studioSaveOptions{Prompt: "35 s", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("x")), MediaType: "video", Duration: 35})
	err := validateHiggsfieldVideoReferences(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", MotionVideoID: clip.ID})
	if err == nil || !strings.Contains(err.Error(), "up to 20 seconds") {
		t.Fatalf("a 35-second clip without ffmpeg should ask for a trim: %v", err)
	}
}

func TestHiggsfieldTrimReferenceKeepsTwentySeconds(t *testing.T) {
	ffmpeg, _, err := studioClipTools()
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	if err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=24", "-f", "lavfi", "-i", "sine=frequency=440", "-t", "30", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", source).Run(); err != nil {
		t.Skipf("could not make a test clip: %v", err)
	}
	data, _ := os.ReadFile(source)
	trimmed, name, mimeType, ok := higgsfieldTrimReference(context.Background(), data, "clip.mp4", "video")
	if !ok || name != "glowbom-video-reference.mp4" || mimeType != "video/mp4" || len(trimmed) >= len(data) {
		t.Fatalf("trim = %v %q %q, %d of %d bytes", ok, name, mimeType, len(trimmed), len(data))
	}
	out := filepath.Join(dir, "out.mp4")
	_ = os.WriteFile(out, trimmed, 0o600)
	probe, _ := exec.Command(strings.TrimSuffix(ffmpeg, "ffmpeg")+"ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", out).Output()
	if seconds, err := strconv.ParseFloat(strings.TrimSpace(string(probe)), 64); err != nil || seconds > 20.6 || seconds < 19 {
		t.Fatalf("trimmed clip should be about 20 s, got %q", probe)
	}
}
