package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
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

func companionPaidVideo() companionVideoRequest {
	return companionVideoRequest{studioVideoOptions: studioVideoOptions{SourceID: "higgsfield", ModelID: "seedance_2_5", DurationSeconds: 10, Resolution: "720p", AspectRatio: "9:16", SpokenText: "Meet my new project."}}
}

func companionMediaGet(t *testing.T, s *companionSession, path string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodGet, path, ""))
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result
}

func companionVideoQuote(t *testing.T, s *companionSession, request companionVideoRequest) string {
	t.Helper()
	w := companionTransferCall(t, s, "/videos/estimate", request)
	var quote struct {
		ID        string  `json:"quoteId"`
		Credits   float64 `json:"credits"`
		PriceKind string  `json:"priceKind"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &quote) != nil || quote.ID == "" || quote.Credits != 5 || quote.PriceKind != "live" {
		t.Fatalf("bad quote: %d %s", w.Code, w.Body.String())
	}
	return quote.ID
}

func waitCompanionMedia(t *testing.T, s *companionSession, path string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		code, result := companionMediaGet(t, s, path)
		if code != 200 {
			t.Fatalf("media progress: %d %+v", code, result)
		}
		if result["status"] != "running" {
			// Let the completion receipt finish, so a new pairing must read it.
			directory, _ := companionMediaDirectory()
			key := companionReceiptKey(directory, result["id"].(string))
			companionMediaMu.Lock()
			active := companionMediaJobs[key] != nil || companionVoiceJobs[key]
			companionMediaMu.Unlock()
			if !active {
				return result
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("media request did not finish")
	return nil
}

func TestCompanionHiggsfieldQuoteIsFreeAndRequired(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	fake := signInFakeHiggsfield(t)
	calls := 0
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Error("unexpected paid API call") }))
	request := companionPaidVideo()
	response := companionTransferCall(t, s, "/videos", request)
	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("legacy client can spend: %d %s", response.Code, response.Body.String())
	}
	request.QuoteID = companionVideoQuote(t, s, request)
	request.RequestID = randomUUIDString()
	response = companionTransferCall(t, s, "/videos", request)
	if response.Code != http.StatusPreconditionRequired {
		t.Fatal("unconfirmed quote accepted", response.Code)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if calls != 0 || fake.submits != 0 || len(fake.uploaded) != 0 || len(fake.voiceClones) != 0 {
		t.Fatal("preflight spent credits or uploaded media")
	}
	for _, call := range fake.toolCalls {
		if call.name == "generate_video" && call.arguments["params"].(map[string]any)["get_cost"] != true {
			t.Fatal("price check submitted video")
		}
	}
}

func TestCompanionHiggsfieldQuoteBindsEverySettingAndReference(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("changed request reached paid API") }))
	audio, err := saveStudioAsset(studioSaveOptions{MediaType: "audio", DataURI: "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(testDictationWAV(30, 500)), MimeType: "audio/wav"})
	if err != nil {
		t.Fatal(err)
	}
	motion, err := saveStudioAsset(studioSaveOptions{MediaType: "video", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("fixture movement")), Duration: 10})
	if err != nil {
		t.Fatal(err)
	}
	image, err := saveStudioAsset(studioSaveOptions{MediaType: "image", DataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(companionImageFixture(t, "png", 2, 2))})
	if err != nil {
		t.Fatal(err)
	}
	original := companionPaidVideo()
	original.ReferenceID, original.MotionVideoID, original.VoiceAudioID = image.ID, motion.ID, audio.ID
	original.QuoteID = companionVideoQuote(t, s, original)
	original.Confirmed, original.RequestID = true, randomUUIDString()
	changes := []func(*companionVideoRequest){
		func(r *companionVideoRequest) { r.Prompt = "A different scene" },
		func(r *companionVideoRequest) { r.SpokenText = "Different approved words" },
		func(r *companionVideoRequest) { r.Resolution = "1080p" },
		func(r *companionVideoRequest) { r.DurationSeconds = 15 },
		func(r *companionVideoRequest) { r.AspectRatio = "16:9" },
		func(r *companionVideoRequest) { r.ReferenceID = "" },
		func(r *companionVideoRequest) { r.MotionVideoID = "" },
		func(r *companionVideoRequest) { r.VoiceAudioID = ""; r.HiggsfieldVoiceID = "clone-1" },
	}
	for index, change := range changes {
		request := original
		change(&request)
		w := companionTransferCall(t, s, "/videos", request)
		if w.Code != http.StatusPreconditionRequired {
			t.Fatalf("change %d: %d %s", index, w.Code, w.Body.String())
		}
	}
	assets, _ := studioAssetsDirectory()
	audio.DataBase64 = base64.StdEncoding.EncodeToString(testDictationWAV(30, 1000))
	encoded, _ := json.Marshal(audio)
	files, _ := filepath.Glob(filepath.Join(assets, "*"+audio.ID+".json"))
	if len(files) != 1 {
		t.Fatal("fixture asset missing")
	}
	if err := os.WriteFile(files[0], encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if w := companionTransferCall(t, s, "/videos", original); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("replaced reference reused review: %d %s", w.Code, w.Body.String())
	}
}

func TestCompanionHiggsfieldQuoteExpiresAndCannotBeForged(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	s := testCompanion(t, http.NotFoundHandler())
	request := companionPaidVideo()
	request.QuoteID = companionVideoQuote(t, s, request)
	request.Confirmed, request.RequestID = true, randomUUIDString()
	now := time.Now().Add(companionMediaQuoteTTL + time.Second)
	s.now = func() time.Time { return now }
	if w := companionTransferCall(t, s, "/videos", request); w.Code != http.StatusPreconditionRequired {
		t.Fatal("expired quote accepted", w.Code)
	}
	request.QuoteID = randomUUIDString()
	if w := companionTransferCall(t, s, "/videos", request); w.Code != http.StatusPreconditionRequired {
		t.Fatal("forged quote accepted", w.Code)
	}
}

func TestCompanionHiggsfieldVideoRetryAndRePairCannotSpendTwice(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	var paid atomic.Int32
	release := make(chan struct{})
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paid.Add(1)
		if expected, _ := r.Context().Value(companionHiggsfieldAccountContextKey{}).(string); expected == "" {
			t.Error("paid video lost approved account context")
		}
		var payload companionVideoRequest
		if r.URL.Path != "/studio/videos/generate" {
			t.Error("wrong internal path")
		}
		var fields map[string]any
		if json.NewDecoder(r.Body).Decode(&fields) != nil {
			t.Error("invalid payload")
		}
		bytes, _ := json.Marshal(fields)
		_ = json.Unmarshal(bytes, &payload)
		if payload.SpokenText != "Meet my new project." || payload.Prompt != "" || fields["useSavedKey"] != true {
			t.Errorf("voice fields lost: %+v", fields)
		}
		<-release
		writeJSON(w, map[string]any{"video": map[string]any{"id": "result-video", "mediaType": "video"}})
	}))
	request := companionPaidVideo()
	request.QuoteID = companionVideoQuote(t, s, request)
	request.Confirmed = true
	request.RequestID = randomUUIDString()
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); statuses <- companionTransferCall(t, s, "/videos", request).Code }()
	}
	wg.Wait()
	close(statuses)
	for code := range statuses {
		if code != 200 && code != http.StatusAccepted {
			t.Error("retry failed", code)
		}
	}
	close(release)
	result := waitCompanionMedia(t, s, "/videos/requests/"+request.RequestID)
	if result["status"] != "completed" || result["video"] == nil || paid.Load() != 1 {
		t.Fatalf("result=%+v paid=%d", result, paid.Load())
	}
	next := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("re-pair submitted again") }))
	if w := companionTransferCall(t, next, "/videos", request); w.Code != 200 {
		t.Fatalf("retry after pairing %d %s", w.Code, w.Body.String())
	}
	request.RequestID = randomUUIDString()
	if w := companionTransferCall(t, next, "/videos", request); w.Code != http.StatusPreconditionRequired {
		t.Fatal("one quote charged twice", w.Code)
	}
}

func TestCompanionHiggsfieldUncertainVideoCannotRetry(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	var paid atomic.Int32
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paid.Add(1)
		http.Error(w, "private provider token=secret", http.StatusBadGateway)
	}))
	request := companionPaidVideo()
	request.QuoteID = companionVideoQuote(t, s, request)
	request.Confirmed = true
	request.RequestID = randomUUIDString()
	if w := companionTransferCall(t, s, "/videos", request); w.Code != http.StatusAccepted {
		t.Fatal(w.Code, w.Body.String())
	}
	result := waitCompanionMedia(t, s, "/videos/requests/"+request.RequestID)
	if result["status"] != "failed" || result["outcomeUncertain"] != true || !strings.Contains(result["error"].(string), "will not be submitted again") || strings.Contains(result["error"].(string), "secret") {
		t.Fatalf("unsafe outcome %+v", result)
	}
	code, generation := companionMediaGet(t, s, "/videos/generations/"+strings.ToLower(request.RequestID))
	if code != 200 || generation["status"] != "failed" || generation["outcomeUncertain"] != true {
		t.Fatalf("generic video progress lost uncertainty: %d %+v", code, generation)
	}
	if w := companionTransferCall(t, s, "/videos", request); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if paid.Load() != 1 {
		t.Fatal("uncertain request submitted twice")
	}
}

func TestCompanionVoiceEstimateAndCloneRetryAreSafe(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	fake := signInFakeHiggsfield(t)
	fake.cloneReplyLost = true
	s := testCompanion(t, http.NotFoundHandler())
	audio, err := saveStudioAsset(studioSaveOptions{MediaType: "audio", DataURI: "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(testDictationWAV(30, 500)), MimeType: "audio/wav"})
	if err != nil {
		t.Fatal(err)
	}
	request := companionVoiceRequest{Name: "Maria", AudioID: audio.ID}
	w := companionTransferCall(t, s, "/higgsfield/voices/estimate", request)
	var quote companionMediaQuote
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &quote) != nil || quote.Credits != 40 || quote.PriceKind != "estimate" {
		t.Fatal(w.Code, w.Body.String())
	}
	fake.mu.Lock()
	clones := len(fake.voiceClones)
	fake.mu.Unlock()
	if clones != 0 {
		t.Fatal("voice quote submitted clone")
	}
	request.QuoteID, request.RequestID = quote.ID, randomUUIDString()
	if w := companionTransferCall(t, s, "/higgsfield/voices", request); w.Code != http.StatusPreconditionRequired {
		t.Fatal("unconfirmed clone accepted", w.Code)
	}
	request.Confirmed = true
	if w := companionTransferCall(t, s, "/higgsfield/voices", request); w.Code != http.StatusAccepted {
		t.Fatal(w.Code, w.Body.String())
	}
	result := waitCompanionMedia(t, s, "/higgsfield/voices/requests/"+request.RequestID)
	if result["status"] != "completed" || result["voice"] == nil {
		t.Fatalf("clone recovery %+v", result)
	}
	assets, _ := studioAssetsDirectory()
	files, _ := filepath.Glob(filepath.Join(assets, "*"+audio.ID+".json"))
	if len(files) != 1 {
		t.Fatal("missing voice sample fixture")
	}
	if err := os.Remove(files[0]); err != nil {
		t.Fatal(err)
	}
	next := testCompanion(t, http.NotFoundHandler())
	if w := companionTransferCall(t, next, "/higgsfield/voices", request); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.voiceClones) != 1 {
		t.Fatal("clone retried", len(fake.voiceClones))
	}
}

func TestCompanionMediaRoutesPreservePairingAuthorization(t *testing.T) {
	s := testCompanion(t, http.NotFoundHandler())
	for _, path := range []string{"/higgsfield/status", "/higgsfield/voices", "/studio/capabilities", "/videos/requests/" + randomUUIDString()} {
		for _, change := range []func(*http.Request){func(r *http.Request) { r.Header.Del("Authorization") }, func(r *http.Request) { r.Header.Set("Origin", "https://untrusted.example") }} {
			r := companionRequest(s, http.MethodGet, path, "")
			change(r)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Fatalf("unsafe route %s %d", path, w.Code)
			}
		}
	}
}

func TestCompanionAudioImportDerivesDurationAndRejectsSpoofedMedia(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	s := testCompanion(t, http.NotFoundHandler())
	request := companionMediaImport{RequestID: randomUUIDString(), Filename: "voice.wav", MimeType: "audio/wav", DataBase64: base64.StdEncoding.EncodeToString(testDictationWAV(30, 500)), DurationSeconds: 30}
	w := companionTransferCall(t, s, "/studio/import", request)
	var receipt companionMediaImported
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.MediaType != "audio" || receipt.DurationSeconds != 30 {
		t.Fatal(w.Code, w.Body.String())
	}
	asset, err := findStudioAsset(receipt.ID)
	if err != nil || asset.Duration == nil || *asset.Duration != 30 || asset.AudioType != "voice" {
		t.Fatal("audio metadata not saved", asset.Duration, err)
	}
	if retry := companionTransferCall(t, s, "/studio/import", request); retry.Code != 200 {
		t.Fatal("audio retry", retry.Code)
	}
	for _, change := range []func(*companionMediaImport){func(r *companionMediaImport) { r.DurationSeconds = 1 }, func(r *companionMediaImport) { r.MimeType = "video/mp4" }, func(r *companionMediaImport) {
		r.DataBase64 = base64.StdEncoding.EncodeToString(testDictationWAV(181, 500))
	}, func(r *companionMediaImport) { r.DataBase64 = base64.StdEncoding.EncodeToString([]byte("not audio")) }} {
		invalid := request
		invalid.RequestID = randomUUIDString()
		change(&invalid)
		if w := companionTransferCall(t, s, "/studio/import", invalid); w.Code != 400 {
			t.Fatal("bad recording accepted", w.Code)
		}
	}
}

func TestCompanionAudioMP3FramesAndWAVBounds(t *testing.T) {
	frame := make([]byte, 417)
	binary.BigEndian.PutUint32(frame, 0xfffb9000)
	mp3 := []byte{}
	for i := 0; i < 100; i++ {
		mp3 = append(mp3, frame...)
	}
	mime, duration, err := companionAudioInfo(mp3)
	if err != nil || mime != "audio/mpeg" || duration < 2.6 || duration > 2.7 {
		t.Fatal("valid MP3 rejected", mime, duration, err)
	}
	for _, invalid := range [][]byte{mp3[:len(mp3)-1], append([]byte("ID3\x04\x00\x00\x7f\x7f\x7f\x7f"), mp3...), testDictationWAV(0, 500), testDictationWAV(181, 500)} {
		if _, _, err := companionAudioInfo(invalid); err == nil {
			t.Fatal("invalid audio accepted")
		}
	}
}

func TestCompanionMediaRecoveryDistinguishesMissingAndUncertain(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	s := testCompanion(t, http.NotFoundHandler())
	id := strings.ToLower(randomUUIDString())
	code, missing := companionMediaGet(t, s, "/videos/requests/"+id)
	if code != 404 || missing["code"] != "request_missing" {
		t.Fatalf("missing: %d %+v", code, missing)
	}
	directory, _ := companionMediaDirectory()
	account, _ := companionHiggsfieldAccountHash()
	receipt := companionMediaReceipt{Kind: "video", Hash: "test", AccountHash: account, Result: map[string]any{"id": id, "status": "running"}}
	if err := companionWriteMediaState(directory, "request-"+id+".json", receipt); err != nil {
		t.Fatal(err)
	}
	code, result := companionMediaGet(t, s, "/videos/requests/"+id)
	if code != 200 || result["status"] != "failed" || result["outcomeUncertain"] != true {
		t.Fatalf("restart recovery: %d %+v", code, result)
	}
	receipt.AccountHash = "different-account"
	_ = companionWriteMediaState(directory, "request-"+id+".json", receipt)
	code, result = companionMediaGet(t, s, "/videos/requests/"+id)
	if code != 409 || result["code"] == "request_missing" {
		t.Fatalf("wrong account treated as missing: %d %+v", code, result)
	}
	_ = os.WriteFile(filepath.Join(directory, "request-"+id+".json"), []byte("broken"), 0600)
	code, result = companionMediaGet(t, s, "/videos/requests/"+id)
	if code != 503 || result["code"] == "request_missing" {
		t.Fatalf("corrupt receipt treated as missing: %d %+v", code, result)
	}
}

func TestCompanionCapabilitiesKeepDesktopIdentityAcrossPairing(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	s := testCompanion(t, http.NotFoundHandler())
	code, first := companionMediaGet(t, s, "/studio/capabilities")
	if code != 200 || first["videoQuotes"] != true || first["voiceCloning"] != true || normalizedStudioUUID(first["desktopId"].(string)) == "" {
		t.Fatalf("capabilities: %d %+v", code, first)
	}
	next := testCompanion(t, http.NotFoundHandler())
	next.pairing.Token = strings.Repeat("b", 64)
	code, second := companionMediaGet(t, next, "/studio/capabilities")
	if code != 200 || second["desktopId"] != first["desktopId"] {
		t.Fatalf("pairing changed desktop identity: %+v", second)
	}
}

func TestCompanionVoiceStopsIfAccountChangesDuringUpload(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		t.Fatal(err)
	}
	oldGet := higgsfieldKeyringGet
	higgsfieldKeyringGet = func(service, name string) (string, error) {
		value, err := oldGet(service, name)
		if err != nil || service != higgsfieldKeyService || name != higgsfieldKeyAccount {
			return value, err
		}
		fake.mu.Lock()
		confirmed := false
		for _, call := range fake.toolCalls {
			confirmed = confirmed || call.name == "media_confirm"
		}
		fake.mu.Unlock()
		if confirmed {
			var credential higgsfieldCredential
			_ = json.Unmarshal([]byte(value), &credential)
			credential.Email = "another-account@example.test"
			encoded, _ := json.Marshal(credential)
			value = string(encoded)
		}
		return value, nil
	}
	t.Cleanup(func() { higgsfieldKeyringGet = oldGet })
	ctx := withCompanionHiggsfieldAccount(context.Background(), account)
	if _, err := createHiggsfieldVoice(ctx, "Maria", testDictationWAV(30, 500), "audio/wav"); err == nil {
		t.Fatal("switched account accepted")
	}
	fake.mu.Lock()
	clones := len(fake.voiceClones)
	lists := 0
	for _, call := range fake.toolCalls {
		if call.name == "list_voices" {
			lists++
		}
	}
	fake.mu.Unlock()
	if clones != 0 || lists != 2 {
		t.Fatalf("account switch reached clone or recovery list: clones=%d list pages=%d", clones, lists)
	}
	// Ordinary Desktop operations have no companion account constraint.
	if _, err := higgsfieldListVoices(context.Background()); err != nil {
		t.Fatal("normal Desktop voice list changed", err)
	}
}

func TestCompanionHiggsfieldCanceledVideoKeepsUncertainIdentityOnEveryRoute(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	signInFakeHiggsfield(t)
	entered := make(chan struct{})
	stopped := make(chan struct{})
	var paid atomic.Int32
	s := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paid.Add(1)
		close(entered)
		<-r.Context().Done()
		close(stopped)
	}))
	request := companionPaidVideo()
	request.QuoteID = companionVideoQuote(t, s, request)
	request.RequestID = strings.ToLower(randomUUIDString())
	request.Confirmed = true
	response := companionTransferCall(t, s, "/videos", request)
	var running map[string]any
	if response.Code != http.StatusAccepted || json.Unmarshal(response.Body.Bytes(), &running) != nil || running["outcomeUncertain"] == true {
		t.Fatalf("running job stopped polling: %d %s", response.Code, response.Body.String())
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("video never started")
	}
	cancelResponse := httptest.NewRecorder()
	s.ServeHTTP(cancelResponse, companionRequest(s, http.MethodDelete, "/videos/generations/"+request.RequestID, ""))
	var canceled map[string]any
	if cancelResponse.Code != 200 || json.Unmarshal(cancelResponse.Body.Bytes(), &canceled) != nil || canceled["status"] != "canceled" || canceled["outcomeUncertain"] != true {
		t.Fatalf("cancellation released paid identity: %d %s", cancelResponse.Code, cancelResponse.Body.String())
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("canceled API did not stop")
	}
	result := waitCompanionMedia(t, s, "/videos/requests/"+request.RequestID)
	if result["status"] != "canceled" || result["outcomeUncertain"] != true {
		t.Fatalf("saved cancellation lost uncertainty: %+v", result)
	}
	code, generation := companionMediaGet(t, s, "/videos/generations/"+request.RequestID)
	if code != 200 || generation["status"] != "canceled" || generation["outcomeUncertain"] != true {
		t.Fatalf("live cancellation lost uncertainty: %d %+v", code, generation)
	}
	if response := companionTransferCall(t, s, "/videos", request); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if paid.Load() != 1 {
		t.Fatal("canceled paid request was repeated")
	}
}
