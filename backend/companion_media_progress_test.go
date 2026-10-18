package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func desktopMediaStatusJob(t *testing.T, manager *companionManager, id string) map[string]any {
	t.Helper()
	for _, raw := range manager.status()["jobs"].([]any) {
		job := raw.(map[string]any)
		if job["id"] == id {
			return job
		}
	}
	t.Fatalf("Desktop status did not include phone media %s", id)
	return nil
}

func cancelDesktopMediaJob(t *testing.T, manager *companionManager, id string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/companion/cancel", strings.NewReader(`{"jobId":"`+id+`"}`))
	response := httptest.NewRecorder()
	manager.cancelJob(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Desktop cancellation returned %d: %s", response.Code, response.Body.String())
	}
}

func TestCompanionMediaProgressAppearsInDesktopAndStopsOnCancel(t *testing.T) {
	for _, kind := range []string{"image", "video"} {
		t.Run(kind, func(t *testing.T) {
			entered, stopped := make(chan struct{}), make(chan struct{})
			api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				progress, ok := beginStudioProgress(w, r, "", kind)
				if !ok {
					t.Error("phone context could not create its progress observer")
					return
				}
				progress.stage("downloading")
				close(entered)
				<-r.Context().Done()
				progress.stage("complete")
				close(stopped)
			})
			manager := newCompanionManager(api)
			t.Cleanup(manager.Shutdown)
			session := testCompanion(t, api)
			session.manager = manager
			payload := map[string]any{"prompt": "A saved project introduction", "sourceId": "fixture-provider",
				"modelId": "fixture-model", "resolution": "720p", "aspectRatio": "9:16", "durationSeconds": 10,
				"referenceId": "local-image-id", "referenceImage": "data:image/png;base64,PRIVATE_IMAGE_BYTES",
				"voiceAudioId": "private-voice-id", "authorization": "private-token"}
			response := httptest.NewRecorder()
			session.startJob(response, httptest.NewRequest(http.MethodPost, "/", nil), kind, companionProject{}, "/fixture-media", payload)
			var accepted struct {
				ID string `json:"id"`
			}
			if response.Code != http.StatusAccepted || json.Unmarshal(response.Body.Bytes(), &accepted) != nil || accepted.ID == "" {
				t.Fatalf("start failed: %d %s", response.Code, response.Body.String())
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("media worker did not enter")
			}
			status := desktopMediaStatusJob(t, manager, accepted.ID)
			media, ok := status["media"].(companionMediaDetails)
			if !ok || status["source"] != "phone" || status["kind"] != kind || status["status"] != "running" ||
				media.Stage != "downloading" || media.SourceID != "fixture-provider" || media.Prompt != payload["prompt"] || media.ReferenceID != "local-image-id" {
				t.Fatalf("missing or incorrect Desktop media details: %+v", status)
			}
			encoded, _ := json.Marshal(status)
			for _, secret := range []string{"PRIVATE_IMAGE_BYTES", "private-token", "private-voice-id"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("Desktop progress exposed %s", secret)
				}
			}
			phone := manager.run(accepted.ID).snapshot(false)
			if phone["media"].(companionMediaDetails).ReferenceID != "" {
				t.Fatal("phone snapshot leaked a Desktop reference handle")
			}
			cancelDesktopMediaJob(t, manager, accepted.ID)
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("Desktop cancellation did not stop the phone media worker")
			}
			status = desktopMediaStatusJob(t, manager, accepted.ID)
			if status["status"] != "canceled" || status["media"].(companionMediaDetails).Stage != "downloading" {
				t.Fatalf("late progress changed a canceled media job: %+v", status)
			}
		})
	}
}

func TestCompanionDesktopMediaCancelKeepsPaidReceiptAndDoesNotResubmit(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	entered, stopped := make(chan struct{}), make(chan struct{})
	var submitted atomic.Int32
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitted.Add(1)
		close(entered)
		<-r.Context().Done()
		close(stopped)
	})
	manager := newCompanionManager(api)
	t.Cleanup(manager.Shutdown)
	session := testCompanion(t, api)
	session.manager = manager
	request := companionPaidVideo()
	request.QuoteID = companionVideoQuote(t, session, request)
	request.RequestID, request.Confirmed = strings.ToLower(randomUUIDString()), true
	if response := companionTransferCall(t, session, "/videos", request); response.Code != http.StatusAccepted {
		t.Fatalf("video start: %d %s", response.Code, response.Body.String())
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("paid fixture did not start")
	}
	cancelDesktopMediaJob(t, manager, request.RequestID)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Desktop cancel did not reach the paid fixture")
	}
	status := desktopMediaStatusJob(t, manager, request.RequestID)
	if status["status"] != "canceled" || status["outcomeUncertain"] != true {
		t.Fatalf("Desktop cancellation lost paid outcome uncertainty: %+v", status)
	}
	result := waitCompanionMedia(t, session, "/videos/requests/"+request.RequestID)
	if result["status"] != "canceled" || result["outcomeUncertain"] != true {
		t.Fatalf("saved receipt lost Desktop cancellation: %+v", result)
	}
	if response := companionTransferCall(t, session, "/videos", request); response.Code != http.StatusOK || submitted.Load() != 1 {
		t.Fatalf("retry after Desktop cancel submitted again: %d, calls=%d", response.Code, submitted.Load())
	}
}

func TestCompanionVoiceCloneRegistersDesktopActivityAndCancel(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		name := "explicit_cancel"
		if shutdown {
			name = "desktop_shutdown"
		}
		t.Run(name, func(t *testing.T) { testCompanionVoiceCloneDesktopStop(t, shutdown) })
	}
}

func testCompanionVoiceCloneDesktopStop(t *testing.T, shutdown bool) {
	t.Helper()
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	fake := signInFakeHiggsfield(t)
	manager := newCompanionManager(http.NotFoundHandler())
	t.Cleanup(manager.Shutdown)
	session := testCompanion(t, manager.api)
	session.manager = manager
	audio, err := saveStudioAsset(studioSaveOptions{MediaType: "audio", MimeType: "audio/wav",
		DataURI: "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(testDictationWAV(30, 500))})
	if err != nil {
		t.Fatal(err)
	}
	request := companionVoiceRequest{Name: "My voice", AudioID: audio.ID}
	response := companionTransferCall(t, session, "/higgsfield/voices/estimate", request)
	var quote companionMediaQuote
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &quote) != nil {
		t.Fatalf("voice quote: %d %s", response.Code, response.Body.String())
	}
	request.QuoteID, request.RequestID, request.Confirmed = quote.ID, strings.ToLower(randomUUIDString()), true
	// Hold the fake provider while Desktop receives and cancels the accepted job.
	fake.mu.Lock()
	var unlockOnce sync.Once
	unlockProvider := func() { unlockOnce.Do(fake.mu.Unlock) }
	defer unlockProvider()
	response = companionTransferCall(t, session, "/higgsfield/voices", request)
	if response.Code != http.StatusAccepted {
		unlockProvider()
		t.Fatalf("voice start: %d %s", response.Code, response.Body.String())
	}
	status := desktopMediaStatusJob(t, manager, request.RequestID)
	media, ok := status["media"].(companionMediaDetails)
	if !ok || status["kind"] != "voice" || status["source"] != "phone" || media.Prompt != "My voice" || media.Stage != "generating" {
		unlockProvider()
		t.Fatalf("voice clone missing from Desktop: %+v", status)
	}
	wantStatus := "canceled"
	if shutdown {
		manager.Shutdown()
		wantStatus = "failed"
	} else {
		cancelDesktopMediaJob(t, manager, request.RequestID)
	}
	unlockProvider()
	if manager.run(request.RequestID).ctx.Err() != context.Canceled {
		t.Fatal("Desktop stop did not cancel voice context")
	}
	result := waitCompanionMedia(t, session, "/higgsfield/voices/requests/"+request.RequestID)
	if result["status"] != wantStatus || result["outcomeUncertain"] != true {
		t.Fatalf("voice cancellation claimed a known outcome: %+v", result)
	}
	fake.mu.Lock()
	before := len(fake.voiceClones)
	fake.mu.Unlock()
	if response := companionTransferCall(t, session, "/higgsfield/voices", request); response.Code != http.StatusOK {
		t.Fatalf("voice retry did not recover receipt: %d %s", response.Code, response.Body.String())
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.voiceClones) != before {
		t.Fatal("retry after Desktop cancel cloned another voice")
	}
}
