package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func bookLaunchFixture(t *testing.T) (string, bookEntry, projectVideoPlan) {
	t.Helper()
	project, entry := bookVideoTestSetup(t)
	t.Setenv("GLOWBOM_FFMPEG_BIN", "/missing/story-launch-test")
	t.Setenv("GLOWBOM_FFPROBE_BIN", "/missing/story-launch-test")
	asset := bookVideoTestAsset(t, "")
	w, book := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID, "expectedContentVersion": entry.ContentVersion, "title": "Original story video", "caption": "Keep this caption"})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	entry = book.Entries[0]
	plan, err := prepareBookLaunchVideo(context.Background(), asset.ID, project, "launch", "Add clear infographics; preserve the voice.", strings.ToLower(randomUUIDString()), bookLaunchVideoTarget{EntryID: entry.ID, VideoReferenceID: entry.Videos[0].ID, ExpectedContentVersion: entry.ContentVersion})
	if err != nil {
		t.Fatal(err)
	}
	return plan.Project, entry, plan
}

func editBookLaunchReceipt(t *testing.T, id string, edit func(*bookLaunchVideoRecord)) bookLaunchVideoRecord {
	t.Helper()
	projectBookMu.Lock()
	defer projectBookMu.Unlock()
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if err != nil {
		t.Fatal(err)
	}
	defer folder.Close()
	record, err := readBookLaunchVideo(folder, id)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(&record)
		if err := saveBookLaunchVideo(folder, record); err != nil {
			t.Fatal(err)
		}
	}
	return record
}

func bookLaunchFinishedOutput(t *testing.T, plan projectVideoPlan, privateFinish bool) bookLaunchVideoRecord {
	t.Helper()
	ended := time.Now().Add(-10 * time.Second)
	record := editBookLaunchReceipt(t, plan.RequestID, func(record *bookLaunchVideoRecord) {
		if record.JobID == "" {
			record.JobID = strings.ToLower(randomUUIDString())
		}
		if record.Status != "superseded" {
			record.Status = "running"
		}
		record.Plan.PreparedAt = ended.Add(-time.Minute).UnixMilli()
		if privateFinish {
			record.WorkerStatus, record.WorkerFinishedAt = "completed", ended.UTC().Format(time.RFC3339Nano)
		}
	})
	output := filepath.Join(plan.Project, plan.Output)
	if err := os.WriteFile(output, []byte("finished-infographics-video"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(output, ended, ended); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestBookLaunchPrepareIsImmutableAndPreservesStory(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	record := editBookLaunchReceipt(t, plan.RequestID, nil)
	again, err := prepareBookLaunchVideo(context.Background(), entry.Videos[0].AssetID, project, "launch", "Add clear infographics; preserve the voice.", plan.RequestID, record.Target)
	if err != nil || again != plan || !strings.Contains(plan.Output, plan.RequestID) {
		t.Fatal("prepare retry changed identity", err)
	}
	if _, err := prepareBookLaunchVideo(context.Background(), entry.Videos[0].AssetID, project, "launch", "Different instructions", plan.RequestID, record.Target); err == nil {
		t.Fatal("same ID changed inputs")
	}
	other := bookTestProject(t)
	if _, err := prepareBookLaunchVideo(context.Background(), entry.Videos[0].AssetID, other, "launch", "Add clear infographics; preserve the voice.", plan.RequestID, record.Target); err == nil {
		t.Fatal("same ID changed project")
	}
	stale := record.Target
	stale.ExpectedContentVersion = "stale"
	if _, err := prepareBookLaunchVideo(context.Background(), entry.Videos[0].AssetID, project, "launch", "", randomUUIDString(), stale); err != errProjectBookConflict {
		t.Fatal("stale content accepted", err)
	}
	_, book := bookTestCall(t, project, "", nil)
	if book.Entries[0].ContentVersion != entry.ContentVersion || book.Entries[0].Body != entry.Body {
		t.Fatal("preparation changed the story")
	}
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "prepared" || result.Plan != plan || result.Book == nil {
		t.Fatal("prepared plan not recoverable", err)
	}
}

func TestBookLaunchHTTPContractKeepsAuthenticationAndConflictStatus(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "studio-progress-test")
	t.Setenv("GLOWBOM_ALLOWED_ORIGINS", "http://localhost:4572")
	path := "/studio/project-video?path=" + url.QueryEscape(project) + "&requestId=" + plan.RequestID
	w := httptest.NewRecorder()
	studioProjectVideoHandler(w, studioProgressRequest(http.MethodGet, path, ""))
	var result bookLaunchVideoResult
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Status != "prepared" || result.Plan.RequestID != plan.RequestID {
		t.Fatal("GET contract", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	studioProjectVideoHandler(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code == http.StatusOK {
		t.Fatal("saved request bypassed authentication")
	}
	request := map[string]any{"id": entry.Videos[0].AssetID, "path": project, "template": "launch", "prompt": "Add clear infographics; preserve the voice.", "requestId": plan.RequestID, "bookTarget": bookLaunchVideoTarget{entry.ID, entry.Videos[0].ID, entry.ContentVersion}}
	data, _ := json.Marshal(request)
	w = httptest.NewRecorder()
	studioProjectVideoHandler(w, studioProgressRequest(http.MethodPost, "/studio/project-video", string(data)))
	var again projectVideoPlan
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &again) != nil || again != plan {
		t.Fatal("POST retry contract", w.Code, w.Body.String())
	}
	request["requestId"] = randomUUIDString()
	request["bookTarget"] = bookLaunchVideoTarget{entry.ID, entry.Videos[0].ID, "stale"}
	data, _ = json.Marshal(request)
	w = httptest.NewRecorder()
	studioProjectVideoHandler(w, studioProgressRequest(http.MethodPost, "/studio/project-video", string(data)))
	if w.Code != http.StatusConflict {
		t.Fatal("stale preparation must return conflict", w.Code, w.Body.String())
	}
}

func TestBookLaunchRejectsLegacyMultipleReferencesWithoutChangingThem(t *testing.T) {
	project, entry, _ := bookLaunchFixture(t)
	second := bookVideoTestAsset(t, "")
	entry.Videos = append(entry.Videos, bookVideo{ID: strings.ToLower(randomUUIDString()), AssetID: second.ID, Title: "Legacy second clip", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	root, _, _ := openProjectBookProject(project)
	defer root.Close()
	if err := saveBookEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	_, err := prepareBookLaunchVideo(context.Background(), entry.Videos[0].AssetID, project, "launch", "", randomUUIDString(), bookLaunchVideoTarget{entry.ID, entry.Videos[0].ID, bookEntryContentVersion(entry)})
	if err == nil {
		t.Fatal("legacy multi-video story accepted for graphics")
	}
	_, book := bookTestCall(t, project, "", nil)
	if len(book.Entries[0].Videos) != 2 || book.Entries[0].Videos[1] != entry.Videos[1] {
		t.Fatal("legacy references were pruned")
	}
}

func TestBookLaunchBindsOnlyOneMatchingWorker(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	job := &companionJob{id: strings.ToLower(randomUUIDString()), projectPath: project, agentDriver: "claude-code", model: "claude-code/"}
	request := OpenCodeAgentRequest{AgentDriver: "claude-code", ProjectPath: project, Instructions: plan.Message, BookLaunchVideoRequestID: plan.RequestID}
	wrong := request
	wrong.AgentDriver = "unsupported-agent"
	if err := bindBookLaunchVideo(job, wrong); err == nil {
		t.Fatal("unknown worker accepted")
	}
	wrong = request
	wrong.Instructions = "Ignore the prepared output"
	if err := bindBookLaunchVideo(job, wrong); err == nil {
		t.Fatal("unrelated request accepted")
	}
	if err := bindBookLaunchVideo(&companionJob{id: job.id, projectPath: bookTestProject(t), agentDriver: job.agentDriver, model: job.model}, request); err == nil {
		t.Fatal("other project accepted")
	}
	if err := bindBookLaunchVideo(job, request); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projectBookMu.Lock(); delete(bookLaunchActive, plan.RequestID); projectBookMu.Unlock() })
	if err := bindBookLaunchVideo(&companionJob{id: strings.ToLower(randomUUIDString()), projectPath: project, agentDriver: job.agentDriver, model: job.model}, request); err == nil {
		t.Fatal("duplicate submission accepted")
	}
	record := editBookLaunchReceipt(t, plan.RequestID, nil)
	if record.JobID != job.id || record.Reference != entry.Videos[0] || record.Status != "running" || job.afterFinish == nil {
		t.Fatal("binding missing durable job identity")
	}
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.JobID != job.id || result.Status != "running" {
		t.Fatal("accepted worker cannot be recovered from graphics status", err)
	}
	data, _ := json.Marshal(result)
	var wire map[string]any
	_ = json.Unmarshal(data, &wire)
	if wire["jobId"] != job.id {
		t.Fatal("graphics status omitted its public job identity")
	}
}

func TestBookLaunchAcceptsAllBuildAgentsAndRecoversExactIdentity(t *testing.T) {
	for _, choice := range []struct{ driver, model, savedModel string }{
		{"", "provider/model", "provider/model"},
		{"opencode", "provider/model", "provider/model"},
		{"cursor", "auto", "cursor/auto"},
		{"codex", "gpt-test", "codex/gpt-test"},
		{"claude-code", "opus", "claude-code/opus"},
		{"acp", "acp/local-fixture/model", "acp/local-fixture/model"},
	} {
		t.Run(choice.driver+"/"+choice.model, func(t *testing.T) {
			project, entry, plan := bookLaunchFixture(t)
			request := OpenCodeAgentRequest{ProjectPath: project, AgentDriver: choice.driver, Model: choice.model, Instructions: plan.Message, BookLaunchVideoRequestID: plan.RequestID}
			m := newCompanionManager(http.NotFoundHandler())
			t.Cleanup(m.Shutdown)
			job := m.newRun(companionProject{ID: companionProjectID(project), path: project}, "desktop", request)
			if err := bindBookLaunchVideo(job, request); err != nil {
				t.Fatal("valid agent rejected", err)
			}
			t.Cleanup(func() { projectBookMu.Lock(); delete(bookLaunchActive, plan.RequestID); projectBookMu.Unlock() })
			result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
			if err != nil || result.Status != "running" || result.JobID != job.id || result.AgentDriver != normalizedBuildDriver(choice.driver) || result.Model != choice.savedModel {
				t.Fatal("accepted route and model were not exposed", result, err)
			}
			data, _ := json.Marshal(result)
			var wire map[string]any
			_ = json.Unmarshal(data, &wire)
			if wire["agentDriver"] != result.AgentDriver || wire["model"] != choice.savedModel {
				t.Fatal("wire response omitted accepted identity")
			}
			duplicate := request
			duplicate.AgentDriver, duplicate.Model = "codex", "other-model"
			if choice.driver == "codex" {
				duplicate.AgentDriver, duplicate.Model = "claude-code", "opus"
			}
			other := m.newRun(companionProject{ID: companionProjectID(project), path: project}, "desktop", duplicate)
			if err := bindBookLaunchVideo(other, duplicate); err == nil {
				t.Fatal("same receipt was accepted through a second driver")
			}
			record := bookLaunchFinishedOutput(t, plan, false)
			projectBookMu.Lock()
			delete(bookLaunchActive, plan.RequestID)
			projectBookMu.Unlock()
			// Only the saved receipt and finished run remain after a restart.
			run := map[string]any{"id": job.id, "projectId": companionProjectID(project), "kind": "build", "agentDriver": result.AgentDriver, "model": choice.savedModel, "status": "completed", "finishedAt": time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339Nano)}
			data, _ = json.Marshal(run)
			bookTestWrite(t, project, filepath.Join(".glowbom", "runs", job.id, "run.json"), data)
			result, err = reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
			if err != nil || result.Status != "completed" || result.Asset == nil || result.JobID != record.JobID || result.AgentDriver != normalizedBuildDriver(choice.driver) || result.Model != choice.savedModel || result.Book.Entries[0].Videos[0].AssetID == entry.Videos[0].AssetID {
				t.Fatal("exact saved worker did not recover completion", result, err)
			}
		})
	}
}

func TestBookLaunchRecoveryRejectsDifferentJobAgentOrModel(t *testing.T) {
	for _, field := range []string{"id", "agentDriver", "model"} {
		t.Run(field, func(t *testing.T) {
			project, entry, plan := bookLaunchFixture(t)
			record := bookLaunchFinishedOutput(t, plan, false)
			editBookLaunchReceipt(t, plan.RequestID, func(record *bookLaunchVideoRecord) {
				record.AgentDriver, record.Model = "codex", "codex/gpt-test"
			})
			run := map[string]any{"id": record.JobID, "projectId": companionProjectID(project), "kind": "build", "agentDriver": "codex", "model": "codex/gpt-test", "status": "completed", "finishedAt": time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339Nano)}
			run[field] = "different"
			data, _ := json.Marshal(run)
			bookTestWrite(t, project, filepath.Join(".glowbom", "runs", record.JobID, "run.json"), data)
			result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
			if err != nil || result.Status != "failed" || result.Asset != nil || result.Book.Entries[0].Videos[0] != entry.Videos[0] {
				t.Fatal("different worker imported output", result, err)
			}
		})
	}
}

func TestBookLaunchCompletionReplacesOnlyReferenceAndRecoversAfterRestart(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	record := bookLaunchFinishedOutput(t, plan, false)
	// Only the authoritative run file remains after restart; no browser storage or active worker.
	run, _ := json.Marshal(map[string]any{"id": record.JobID, "projectId": companionProjectID(project), "kind": "build", "agentDriver": "claude-code", "status": "completed", "finishedAt": time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339Nano)})
	bookTestWrite(t, project, filepath.Join(".glowbom", "runs", record.JobID, "run.json"), run)
	_, book := bookTestCall(t, project, "", nil)
	got := book.Entries[0]
	if len(got.Videos) != 1 || got.Videos[0].AssetID == entry.Videos[0].AssetID || got.Body != entry.Body || got.Title != entry.Title || got.Videos[0].Caption != entry.Videos[0].Caption {
		t.Fatal("restart did not replace only the original reference")
	}
	if _, err := findStudioAsset(entry.Videos[0].AssetID); err != nil {
		t.Fatal("original clip removed")
	}
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "completed" || result.Asset == nil || result.Asset.ID != got.Videos[0].AssetID || result.JobID != record.JobID || result.WorkerStatus != "completed" || result.AgentDriver != "claude-code" || result.Model != "" {
		t.Fatal("completion missing", err)
	}
	version := got.ContentVersion
	_, again := bookTestCall(t, project, "", nil)
	if again.Entries[0].ContentVersion != version {
		t.Fatal("collection duplicated the reference")
	}
	// A lost receipt checkpoint after the Book write must recover as completed.
	editBookLaunchReceipt(t, plan.RequestID, func(record *bookLaunchVideoRecord) { record.Status = "running" })
	result, err = reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "completed" {
		t.Fatal("successful Book write was mistaken for a superseding edit", err)
	}
}

func TestBookLaunchSupersededRequestRetainsActualWorkerState(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	job := &companionJob{id: strings.ToLower(randomUUIDString()), projectPath: project, agentDriver: "claude-code", model: "claude-code/"}
	request := OpenCodeAgentRequest{AgentDriver: "claude-code", ProjectPath: project, Instructions: plan.Message, BookLaunchVideoRequestID: plan.RequestID}
	if err := bindBookLaunchVideo(job, request); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projectBookMu.Lock(); delete(bookLaunchActive, plan.RequestID); projectBookMu.Unlock() })
	w, _ := bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": entry.Videos[0].ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "superseded" || result.WorkerStatus != "running" || result.JobID != job.id || result.Asset != nil {
		t.Fatal("superseding the reference hid a still-active worker", err)
	}
	editBookLaunchReceipt(t, plan.RequestID, func(record *bookLaunchVideoRecord) {
		record.WorkerStatus = "canceled"
		record.WorkerFinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	})
	result, err = reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "superseded" || result.WorkerStatus != "canceled" || result.Asset != nil {
		t.Fatal("superseded worker termination was not exposed", err)
	}
	data, _ := json.Marshal(result)
	var wire map[string]any
	_ = json.Unmarshal(data, &wire)
	if wire["workerStatus"] != "canceled" {
		t.Fatal("graphics status omitted actual worker status")
	}
}

func TestBookLaunchLateCompletionKeepsManualVideoChoice(t *testing.T) {
	for _, action := range []string{"remove-video", "replace-video"} {
		t.Run(action, func(t *testing.T) {
			project, entry, plan := bookLaunchFixture(t)
			record := bookLaunchFinishedOutput(t, plan, true)
			values := map[string]any{"entryId": entry.ID, "videoId": entry.Videos[0].ID, "expectedContentVersion": entry.ContentVersion}
			if action == "replace-video" {
				values["assetId"] = bookVideoTestAsset(t, "").ID
			}
			w, before := bookTestCall(t, project, action, values)
			if w.Code != http.StatusOK {
				t.Fatal(w.Code, w.Body.String())
			}
			result, err := reconcileBookLaunchVideo(context.Background(), project, record.RequestID)
			if err != nil || result.Status != "superseded" || result.Asset == nil {
				t.Fatal("late result was not retained safely in Studio", err)
			}
			_, after := bookTestCall(t, project, "", nil)
			if after.Entries[0].ContentVersion != before.Entries[0].ContentVersion {
				t.Fatal("late graphics result overwrote the newer choice")
			}
		})
	}
}

func TestBookLaunchInterruptedOrFailedWorkerNeverImportsItsOutput(t *testing.T) {
	for _, status := range []string{"running", "failed", "completed-without-finish"} {
		t.Run(status, func(t *testing.T) {
			project, entry, plan := bookLaunchFixture(t)
			record := bookLaunchFinishedOutput(t, plan, false)
			run := map[string]any{"id": record.JobID, "projectId": companionProjectID(project), "kind": "build", "agentDriver": "claude-code", "status": status}
			if status == "failed" {
				run["finishedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if status == "completed-without-finish" {
				run["status"] = "completed"
			}
			data, _ := json.Marshal(run)
			bookTestWrite(t, project, filepath.Join(".glowbom", "runs", record.JobID, "run.json"), data)
			result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
			if err != nil || result.Status != "failed" || result.Asset != nil || result.Book.Entries[0].Videos[0] != entry.Videos[0] {
				t.Fatal("unfinished worker output imported", err)
			}
		})
	}
}

func TestBookLaunchDesktopWorkerWaitsPastSSEDoneAndRejectsDuplicate(t *testing.T) {
	for _, choice := range []struct{ driver, model string }{
		{"opencode", "provider/model"}, {"cursor", "auto"}, {"codex", "gpt-test"}, {"claude-code", "opus"}, {"acp", "acp/local-fixture/model"},
	} {
		t.Run(choice.driver, func(t *testing.T) {
			testBookLaunchDesktopWorker(t, choice.driver, choice.model)
		})
	}
}

func testBookLaunchDesktopWorker(t *testing.T, driver, model string) {
	t.Helper()
	project, entry, plan := bookLaunchFixture(t)
	m := newCompanionManager(http.NotFoundHandler())
	t.Cleanup(m.Shutdown)
	started, release, ended := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseWorker := func() { once.Do(func() { close(release) }) }
	defer releaseWorker()
	var calls atomic.Int32
	handler := m.guardBuild(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if buildPermissionPolicyFromContext(r.Context()) == nil || buildPermissionAllEnabled(r.Context(), driver, project, "fixture-session") {
			t.Error("graphics build lost its permission policy or enabled broad permission without review")
		}
		if err := os.WriteFile(filepath.Join(project, plan.Output), []byte("final-graphics-after-worker"), 0600); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"done\":true,\"success\":true}\n\n")
		close(started)
		<-release
	})
	request := OpenCodeAgentRequest{ProjectPath: project, AgentDriver: driver, Model: model, Instructions: plan.Message, BookLaunchVideoRequestID: plan.RequestID}
	body, _ := json.Marshal(request)
	go func() {
		defer close(ended)
		handler(httptest.NewRecorder(), registryLocalRequest("POST", "/opencode/refine", string(body)))
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker never started")
	}
	// Even an old, stable file cannot be collected while the worker is active.
	record := bookLaunchFinishedOutput(t, plan, false)
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "running" || result.Asset != nil || result.Book.Entries[0].Videos[0] != entry.Videos[0] {
		t.Fatal("SSE done was treated as worker completion", err)
	}
	duplicate := httptest.NewRecorder()
	handler(duplicate, registryLocalRequest("POST", "/opencode/refine", string(body)))
	if duplicate.Code != http.StatusConflict || calls.Load() != 1 {
		t.Fatal("duplicate started another worker")
	}
	otherAgent := request
	otherAgent.AgentDriver, otherAgent.Model = "codex", "another-model"
	if driver == "codex" {
		otherAgent.AgentDriver, otherAgent.Model = "claude-code", "opus"
	}
	otherBody, _ := json.Marshal(otherAgent)
	duplicate = httptest.NewRecorder()
	handler(duplicate, registryLocalRequest("POST", "/opencode/refine", string(otherBody)))
	if duplicate.Code != http.StatusConflict || calls.Load() != 1 {
		t.Fatal("changing agent started the same graphics request twice")
	}
	releaseWorker()
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("worker failed to end")
	}
	result, err = reconcileBookLaunchVideo(context.Background(), project, record.RequestID)
	if err != nil || result.Status != "completed" || result.Asset == nil {
		t.Fatal("actual worker completion did not collect", err)
	}
	duplicate = httptest.NewRecorder()
	handler(duplicate, registryLocalRequest("POST", "/opencode/refine", string(body)))
	if duplicate.Code != http.StatusConflict || calls.Load() != 1 {
		t.Fatal("completed request started again")
	}
}
