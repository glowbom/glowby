package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioDebugOptInPrivateFileAndNoSensitiveMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	t.Setenv("GLOWBOM_DEBUG_LOG", "")
	writeGlowbomDebugEvent("test_event", glowbomDebugMetadata{RequestID: "fixture-debug-request"})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("debug logging wrote a file while disabled")
	}
	t.Setenv("GLOWBOM_DEBUG_LOG", path)
	debugStudioGeneration("studio_generation_stage", studioProgressState{ID: "fixture-debug-request", Kind: "video", Stage: "failed", SourceID: "higgsfield", ModelID: "seedance_2_5", Prompt: "private prompt", Error: "private raw provider message", VoiceID: "private-voice-id", ReferenceID: "private-reference-id", FailureCategory: "provider_timeout"})
	writeGlowbomDebugEvent("provider_http", glowbomDebugMetadata{RequestID: "invalid request id", Model: "https://signed.example/private", Operation: "/private/local/path"})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private prompt", "private raw provider", "private-voice-id", "private-reference-id", "signed.example", "/private/local/path", "invalid request id"} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatal("debug log exposed forbidden metadata")
		}
	}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record map[string]any
		if json.Unmarshal(line, &record) != nil || record["source"] != "backend" || record["timestamp"] == "" {
			t.Fatal("invalid structured debug event")
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("debug log must be private", err)
	}
	if err := os.Truncate(path, glowbomDebugLogLimit); err != nil {
		t.Fatal(err)
	}
	writeGlowbomDebugEvent("test_event", glowbomDebugMetadata{RequestID: "fixture-debug-request"})
	if info, err := os.Stat(path); err != nil || info.Size() != glowbomDebugLogLimit {
		t.Fatal("debug log exceeded its size limit", err)
	}
}

func TestStudioDebugRejectsLinksAndNeverChangesGenerationOutcome(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(target, []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "debug.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLOWBOM_DEBUG_LOG", link)
	store := newStudioProgressStore()
	if status := store.begin("fixture-debug-unwritable", "video"); status != http.StatusOK {
		t.Fatal("unavailable log blocked generation")
	}
	progress := studioProgress{id: "fixture-debug-unwritable", store: store}
	defer progress.finish()
	progress.stage("generating")
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "unchanged" {
		t.Fatal("debug writer followed a symlink", err)
	}
}

func TestStudioDebugHiggsfieldHTTPRecordsStatusWithoutPayloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	t.Setenv("GLOWBOM_DEBUG_LOG", path)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("private response with private-media-bytes"))
	}))
	defer server.Close()
	ctx := context.WithValue(context.Background(), studioDebugRequestKey{}, "fixture-debug-http-request")
	status, _, _, err := higgsfieldPost(ctx, server.URL+"/signed?token=private-query", "application/json", []byte(`{"prompt":"private prompt"}`), map[string]string{"Authorization": "Bearer private-token"})
	if err != nil || status != http.StatusTooManyRequests {
		t.Fatal(status, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(`"httpStatus":429`)) || !bytes.Contains(data, []byte(`"errorCategory":"rate_limit"`)) || !bytes.Contains(data, []byte(`"requestId":"fixture-debug-http-request"`)) {
		t.Fatal("missing safe HTTP diagnostics", err)
	}
	for _, private := range []string{"private", server.URL, "Authorization", "prompt"} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatal("HTTP diagnostics exposed provider payload or URL")
		}
	}
}

func TestHiggsfieldTerminalVideoFailuresUseFixedSafeReasons(t *testing.T) {
	for _, example := range []struct{ status, detail, category string }{
		{"failed", "render timeout", "provider_timeout"},
		{"nsfw", "private content", "provider_moderation"},
		{"failed", "insufficient_credits", "provider_credits"},
		{"cancelled", "", "provider_cancelled"},
		{"failed", "internal_error", "provider_unavailable"},
		{"rejected", "", "provider_rejected"},
		{"failed", "unrecognized reason", "provider_failed_unknown"},
	} {
		t.Run(example.category, func(t *testing.T) {
			err := higgsfieldTerminalVideoFailure(example.status, example.detail+" private-prompt https://signed.example/?secret=private-token")
			var failure *studioVideoFailure
			if !errors.As(err, &failure) || failure.category != example.category || !failure.terminal {
				t.Fatal("provider failure lost its category or terminal status")
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "signed.example") {
				t.Fatal("provider failure exposed raw detail")
			}
			if example.category == "provider_failed_unknown" && (strings.Contains(err.Error(), "moderation") || strings.Contains(err.Error(), "account limit")) {
				t.Fatal("unknown failure invented a cause")
			}
		})
	}
}

func TestHiggsfieldVideoFailurePersistsSafeReasonAndDebugEvents(t *testing.T) {
	isolateStudioRecovery(t)
	fake := signInFakeHiggsfield(t)
	fake.videoJobStatus, fake.videoJobError = "nsfw", "private-prompt https://signed.example/?token=private-secret"
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	t.Setenv("GLOWBOM_DEBUG_LOG", path)
	id := "fixture-debug-terminal-higgsfield"
	response := httptest.NewRecorder()
	studioVideoGenerateHandler(response, studioProgressRequest(http.MethodPost, "/studio/videos/generate", studioRecoveryJSON(t, map[string]any{"generationId": id, "prompt": "private-prompt", "sourceId": "higgsfield", "modelId": "seedance_2_5", "resolution": "480p", "aspectRatio": "16:9", "durationSeconds": 10})))
	state, _ := readStudioRecoveryStatus(t, id)
	if response.Code != http.StatusBadGateway || state.Stage != "failed" || state.CanResume || state.FailureCategory != "provider_moderation" || !strings.Contains(state.Error, "Higgsfield rejected") || state.Error != strings.TrimSpace(response.Body.String()) || fake.submits != 1 {
		t.Fatalf("specific terminal failure was not retained: status=%d stage=%s category=%s resumes=%t submits=%d", response.Code, state.Stage, state.FailureCategory, state.CanResume, fake.submits)
	}
	if response.Header().Get(studioGenerationNotSubmittedHeader) != "" {
		t.Fatal("submitted provider rejection was incorrectly marked not submitted")
	}
	checkpoint, found, err := loadStudioGenerationCheckpoint(id)
	if err != nil || !found || checkpoint.State.Error != state.Error || checkpoint.State.FailureCategory != "provider_moderation" || checkpoint.OperationID == "" {
		t.Fatal("safe provider reason was not durably saved", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"studio_generation_started", "studio_generation_submitted", "studio_generation_stage", "provider_http", "provider_job_status", "provider_moderation", `"httpStatus":200`, id} {
		if !bytes.Contains(data, []byte(expected)) {
			t.Fatalf("missing debug evidence %s", expected)
		}
	}
	for _, private := range []string{"private-prompt", "private-secret", "signed.example", "job-1", fake.accessToken, "dataBase64"} {
		if private != "" && bytes.Contains(data, []byte(private)) {
			t.Fatal("debug log exposed private provider inputs or identifiers")
		}
	}
	studioGenerations = newStudioProgressStore()
	recovered, _ := readStudioRecoveryStatus(t, id)
	if recovered.Error != state.Error || recovered.FailureCategory != state.FailureCategory || recovered.CanResume {
		t.Fatal("restart lost the terminal reason")
	}
}

func TestStudioVideoWaitTimeoutAndAuthenticationKeepOriginalJobRecoverable(t *testing.T) {
	for _, failure := range []error{studioVideoWaitError(context.DeadlineExceeded), studioHiggsfieldVideoError(errHiggsfieldSignedOut)} {
		isolateStudioRecovery(t)
		id := "fixture-debug-recoverable-video"
		store := studioGenerations
		if status := store.begin(id, "video"); status != http.StatusOK {
			t.Fatal(status)
		}
		progress := studioProgress{id: id, store: store}
		if err := progress.configure("private prompt", "16:9", "", "higgsfield", 10); err != nil {
			t.Fatal(err)
		}
		if err := progress.operation("private-provider-operation"); err != nil {
			t.Fatal(err)
		}
		message, _ := progress.failedVideo(context.Background(), failure, "fallback")
		state := store.read(id)
		if state.Stage != "failed" || !state.CanResume || state.Error != message || state.FailureCategory == "" {
			t.Fatal("an unknown provider outcome was incorrectly made terminal")
		}
		if state.FailureCategory == "poll_timeout" && strings.Contains(message, "still rendering") {
			t.Fatal("local timeout guessed the remote job state")
		}
		progress.finish()
	}
}
