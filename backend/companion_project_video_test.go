package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func companionProjectMediaFixture(t *testing.T, build func(http.ResponseWriter, OpenCodeAgentRequest)) (*companionSession, companionProject) {
	t.Helper()
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	t.Setenv("GLOWBOM_FFMPEG_BIN", "/missing/phone-video-test")
	t.Setenv("GLOWBOM_FFPROBE_BIN", "/missing/phone-video-test")
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/models":
			io.WriteString(w, `{"models":[{"id":"codex/future-video-agent","build":true,"reasoningEfforts":["high"]},{"id":"claude-code/opus","build":true}]}`)
		case "/opencode/refine":
			var request OpenCodeAgentRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			build(w, request)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
	}))
	project := sharedCompanionProject(t, s)
	s.setModel(project, "claude-code/opus", "desktop")
	return s, project
}

func companionProjectMediaCall(t *testing.T, s *companionSession, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	data, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, method, path, string(data)))
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return w, result
}

func waitCompanionProjectMedia(t *testing.T, s *companionSession, id string, ready bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if job := s.sharedJob(id); job != nil {
			result := job.snapshot(false)
			if (ready && result["resultStatus"] == "ready") || (!ready && result["status"] != "running" && result["status"] != "waiting") {
				return result
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("phone project video did not finish")
	return nil
}

func TestCompanionVideoIdeasPreservesExplicitModelAndRetryIdentity(t *testing.T) {
	var calls atomic.Int32
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, request OpenCodeAgentRequest) {
		calls.Add(1)
		if request.AgentDriver != "codex" || request.Model != "future-video-agent" || request.ReasoningEffort != "high" || request.PermissionMode != "ask" || request.MediaGenerationPolicy != "skip" {
			t.Error("selected model, effort, permissions or paid-media policy changed")
		}
		if !strings.Contains(request.Instructions, projectVideoIdeasFile) {
			t.Error("ideas brief was not passed to the existing worker")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"done\":true,\"success\":true}\n\n")
	})
	id := "11111111-2222-4333-8444-555555555555"
	path := "/projects/" + project.ID + "/video-ideas"
	request := map[string]any{"requestId": id, "model": "codex/future-video-agent", "reasoningEffort": "high", "permissionMode": "ask"}
	w, result := companionProjectMediaCall(t, s, http.MethodPost, path, request)
	if w.Code != http.StatusAccepted || result["id"] != id || result["workflow"] != "video-ideas" || result["model"] != "codex/future-video-agent" {
		t.Fatalf("build not accepted: %d %s", w.Code, w.Body.String())
	}
	w, retry := companionProjectMediaCall(t, s, http.MethodPost, path, request)
	if w.Code != http.StatusOK || retry["id"] != id {
		t.Fatal("matching retry did not recover the same build")
	}
	waitCompanionProjectMedia(t, s, id, false)
	if calls.Load() != 1 || s.modelChoice(project, false).Model != "claude-code/opus" {
		t.Fatal("retry started another build or replaced the project model")
	}
	request["model"] = "claude-code/opus"
	w, _ = companionProjectMediaCall(t, s, http.MethodPost, path, request)
	if w.Code != http.StatusConflict || calls.Load() != 1 {
		t.Fatal("request ID accepted different settings")
	}
	w, _ = companionProjectMediaCall(t, s, http.MethodGet, path+"/"+id, nil)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), project.path) {
		t.Fatal("recovery failed or exposed an absolute project path")
	}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, path, nil)
	if w.Code != http.StatusOK || result["exists"] != false {
		t.Fatal("missing ideas file was not reported")
	}
	if err := os.WriteFile(filepath.Join(project.path, projectVideoIdeasFile), []byte(`{"ideas":[{"id":"launch","words":"Meet the project.","title":"Launch"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, path, nil)
	if w.Code != http.StatusOK || result["exists"] != true || len(result["ideas"].([]any)) != 1 {
		t.Fatal("saved ideas were not read")
	}
}

func TestCompanionVideoIdeasReadsSavedDesktopIdeasWithoutStartingAnAgent(t *testing.T) {
	var calls atomic.Int32
	s, project := companionProjectMediaFixture(t, func(http.ResponseWriter, OpenCodeAgentRequest) { calls.Add(1) })
	media := filepath.Join(project.path, projectVideoMediaDir)
	if err := os.MkdirAll(media, 0700); err != nil {
		t.Fatal(err)
	}
	saved := []byte(`{"ideas":[{"title":"Launch","words":"Meet the project I made.","seconds":10,"scene":"At my desk.","screens":["Home"]},{"title":"One feature","words":"Here is how the main feature works."}]}`)
	file := filepath.Join(project.path, projectVideoIdeasFile)
	if err := os.WriteFile(file, saved, 0600); err != nil {
		t.Fatal(err)
	}
	w, result := companionProjectMediaCall(t, s, http.MethodGet, "/projects/"+project.ID+"/video-ideas", nil)
	if w.Code != http.StatusOK || result["exists"] != true || len(result["ideas"].([]any)) != 2 {
		t.Fatalf("saved ideas not available on first phone read: %d %s", w.Code, w.Body.String())
	}
	ideas := result["ideas"].([]any)
	first := ideas[0].(map[string]any)
	if first["id"] != "idea-1" || first["seconds"] != float64(10) || first["words"] != "Meet the project I made." {
		t.Fatal("phone did not receive the Desktop idea fields")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(saved) || calls.Load() != 0 {
		t.Fatal("reading existing ideas changed the file or started an agent")
	}
	if err := os.WriteFile(file, []byte(`{"ideas":`), 0600); err != nil {
		t.Fatal(err)
	}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, "/projects/"+project.ID+"/video-ideas", nil)
	if w.Code != http.StatusOK || result["exists"] != true || result["error"] == nil || calls.Load() != 0 {
		t.Fatal("an unreadable ideas file was confused with missing ideas or caused regeneration")
	}
}

func TestCompanionProjectVideoImportsWithoutOpeningDesktopStudio(t *testing.T) {
	var calls atomic.Int32
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, request OpenCodeAgentRequest) {
		calls.Add(1)
		if request.AgentDriver != "claude-code" || request.Model != "opus" || request.PermissionMode != "" {
			t.Error("shared choice or default permission policy changed")
		}
		directory, _ := companionProjectMediaDirectory()
		record, err := readCompanionProjectMediaReceipt(directory, id)
		if err != nil || !record.Started || !strings.Contains(record.Plan.Output, id) {
			t.Error("durable identity and unique output must exist before the agent starts")
			http.Error(w, "no plan", 500)
			return
		}
		if err := os.WriteFile(filepath.Join(request.ProjectPath, record.Plan.Output), []byte("synthetic-finished-video"), 0600); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"done\":true,\"success\":true}\n\n")
	})
	clip, err := saveStudioAsset(studioSaveOptions{Prompt: "Introducing Glowbom", MediaType: "video", Source: "Test", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("synthetic-source-video"))})
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID + "/project-video"
	request := map[string]any{"requestId": id, "videoId": clip.ID, "template": "launch"}
	w, result := companionProjectMediaCall(t, s, http.MethodPost, path, request)
	if w.Code != http.StatusAccepted || result["resultStatus"] != "pending" {
		t.Fatalf("video not accepted: %d %s", w.Code, w.Body.String())
	}
	result = waitCompanionProjectMedia(t, s, id, true)
	if result["status"] != "completed" || result["video"] == nil {
		t.Fatal("finished output was not imported automatically")
	}
	w, first := companionProjectMediaCall(t, s, http.MethodGet, path+"/"+id, nil)
	if w.Code != http.StatusOK || first["resultStatus"] != "ready" {
		t.Fatal("finished output did not reach the phone")
	}
	finalID := first["video"].(map[string]any)["id"].(string)
	finalAsset, err := findStudioAsset(finalID)
	if err != nil || finalID == clip.ID {
		t.Fatal("the launch result must be a distinct saved asset")
	}
	finalBytes, _, err := decodeBase64Payload(finalAsset.DataBase64, "video/mp4")
	if err != nil || !bytes.Equal(finalBytes, []byte("synthetic-finished-video")) || finalAsset.DataBase64 == clip.DataBase64 {
		t.Fatal("the launch asset must contain the finished output, not the talking clip")
	}
	// A new pairing recovers the durable result without another build.
	s.mu.Lock()
	s.jobs = map[string]*companionJob{}
	s.mu.Unlock()
	w, recovered := companionProjectMediaCall(t, s, http.MethodPost, path, request)
	if w.Code != http.StatusOK || calls.Load() != 1 || recovered["resultStatus"] != "ready" || first["video"].(map[string]any)["id"] != recovered["video"].(map[string]any)["id"] {
		t.Fatal("recovery regenerated or duplicated the saved video")
	}
	other := sharedCompanionProject(t, s)
	w, _ = companionProjectMediaCall(t, s, http.MethodGet, "/projects/"+other.ID+"/project-video/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Fatal("another project could recover this video request")
	}
}

func TestCompanionProjectVideoWaitsForWorkerBeforeImportingFinalOutput(t *testing.T) {
	id := "abababab-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	completionSent := make(chan struct{})
	releaseWorker := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseWorker) }) }
	defer release()
	sourceBytes, finalBytes := []byte("synthetic-talking-clip"), []byte("synthetic-finished-launch")
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, request OpenCodeAgentRequest) {
		directory, _ := companionProjectMediaDirectory()
		record, err := readCompanionProjectMediaReceipt(directory, id)
		if err != nil {
			t.Error(err)
			close(completionSent)
			return
		}
		output := filepath.Join(request.ProjectPath, record.Plan.Output)
		if err := os.WriteFile(output, sourceBytes, 0600); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"done\":true,\"success\":true}\n\n")
		close(completionSent)
		<-releaseWorker
		if err := os.WriteFile(output, finalBytes, 0600); err != nil {
			t.Error(err)
		}
	})
	clip, err := saveStudioAsset(studioSaveOptions{Prompt: "Talking clip", MediaType: "video", Source: "Test", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(sourceBytes)})
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID + "/project-video"
	w, _ := companionProjectMediaCall(t, s, http.MethodPost, path, map[string]any{"requestId": id, "videoId": clip.ID, "template": "launch"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("video not accepted: %d", w.Code)
	}
	select {
	case <-completionSent:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not send its completion event")
	}
	// A quiet output file alone must not allow import while the handler is still running.
	time.Sleep(3200 * time.Millisecond)
	w, early := companionProjectMediaCall(t, s, http.MethodGet, path+"/"+id, nil)
	if w.Code != http.StatusOK || early["resultStatus"] != "pending" || early["video"] != nil {
		t.Error("a completion event exposed the unfinished source copy before the worker returned")
	}
	release()
	waitCompanionProjectMedia(t, s, id, true)
	w, finished := companionProjectMediaCall(t, s, http.MethodGet, path+"/"+id, nil)
	if w.Code != http.StatusOK || finished["resultStatus"] != "ready" {
		t.Fatal("the final output was not imported after the worker returned")
	}
	finalID := finished["video"].(map[string]any)["id"].(string)
	asset, err := findStudioAsset(finalID)
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := decodeBase64Payload(asset.DataBase64, "video/mp4")
	if err != nil || finalID == clip.ID || !bytes.Equal(data, finalBytes) {
		t.Fatal("the saved launch asset did not contain the worker's final output")
	}
}

func TestCompanionProjectMediaRejectsUnsupportedOptionsBeforeBuild(t *testing.T) {
	var calls atomic.Int32
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, _ OpenCodeAgentRequest) { calls.Add(1) })
	path := "/projects/" + project.ID + "/video-ideas"
	for _, request := range []map[string]any{
		{"requestId": "bad"},
		{"requestId": randomUUIDString(), "model": "codex/not-connected"},
		{"requestId": randomUUIDString(), "model": "codex/future-video-agent", "reasoningEffort": "unknown"},
		{"requestId": randomUUIDString(), "permissionMode": "always"},
		{"requestId": randomUUIDString(), "path": "/private"},
	} {
		w, result := companionProjectMediaCall(t, s, http.MethodPost, path, request)
		if w.Code != http.StatusBadRequest || result["code"] != "invalid_request" {
			t.Errorf("invalid options accepted: %d", w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid options started an agent")
	}
}

func TestCompanionProjectMediaMissingReceiptIsDistinctFromUnknownOutcome(t *testing.T) {
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, _ OpenCodeAgentRequest) { t.Error("must not start a build") })
	id := "33333333-4444-4555-8666-777777777777"
	path := "/projects/" + project.ID + "/video-ideas/" + id
	w, result := companionProjectMediaCall(t, s, http.MethodGet, path, nil)
	if w.Code != http.StatusNotFound || result["code"] != "request_missing" {
		t.Fatal("a request with no receipt or job should allow another reviewed attempt")
	}
	directory, _ := companionProjectMediaDirectory()
	if err := os.WriteFile(filepath.Join(directory, id+".json"), []byte("damaged receipt"), 0600); err != nil {
		t.Fatal(err)
	}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, path, nil)
	if result["code"] == "request_missing" {
		t.Fatal("a damaged receipt must never be treated as no submission")
	}
	if err := os.Remove(filepath.Join(directory, id+".json")); err != nil {
		t.Fatal(err)
	}
	s.jobs[id] = &companionJob{id: id, projectID: project.ID, kind: "build", status: "running"}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, path, nil)
	if result["code"] == "request_missing" {
		t.Fatal("an existing job must never be treated as no submission")
	}
	w, result = companionProjectMediaCall(t, s, http.MethodGet, "/projects/not-shared/video-ideas/"+id, nil)
	if w.Code != http.StatusNotFound || result["code"] == "request_missing" {
		t.Fatal("an unshared project must not prove a request was not submitted")
	}
}

func TestCompanionProjectMediaInterruptedReceiptNeverStartsAnotherBuild(t *testing.T) {
	s, project := companionProjectMediaFixture(t, func(w http.ResponseWriter, _ OpenCodeAgentRequest) { t.Error("must not start a build") })
	id := "22222222-3333-4444-8555-666666666666"
	directory, err := companionProjectMediaDirectory()
	if err != nil {
		t.Fatal(err)
	}
	record := &companionProjectMediaReceipt{RequestID: id, ProjectID: project.ID, Workflow: "video-ideas", Started: true,
		Snapshot: map[string]any{"id": id, "projectId": project.ID, "kind": "build", "workflow": "video-ideas", "status": "running", "output": []string{}}}
	if err := writeCompanionProjectMedia(directory, record); err != nil {
		t.Fatal(err)
	}
	w, result := companionProjectMediaCall(t, s, http.MethodGet, "/projects/"+project.ID+"/video-ideas/"+id, nil)
	if w.Code != http.StatusOK || result["status"] != "failed" {
		t.Fatal("uncertain interrupted request was not surfaced for review")
	}
}
