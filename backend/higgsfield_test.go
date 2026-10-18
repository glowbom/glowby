package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// fakeHiggsfield stands in for clerk.higgsfield.ai and mcp.higgsfield.ai.
type fakeHiggsfield struct {
	mu              sync.Mutex
	registrations   int
	accessToken     string
	refreshes       int
	userAgents      []string
	lastForm        neturl.Values
	toolCalls       []fakeToolCall
	baseURL         string
	recommendPreset bool
	submits, polls  int
	imageParams     map[string]any
	audioParams     map[string]any
	voiceClones     []map[string]any
	cloneReplyLost  bool
	cloneHasSample  bool
	cloneStatus     string
	refuseVideo     string
	videoJobStatus  string
	videoJobError   string
	failPuts        int
	uploaded        []byte
	uploads         [][]byte
}

func (f *fakeHiggsfield) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.registrations++
		f.userAgents = append(f.userAgents, r.UserAgent())
		f.mu.Unlock()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["token_endpoint_auth_method"] != "none" {
			t.Errorf("registration should be a public client, got %v", body["token_endpoint_auth_method"])
		}
		writeJSON(w, map[string]string{"client_id": "client-123"})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastForm = r.PostForm
		claims, _ := json.Marshal(map[string]string{"email": "jacob@example.com"})
		idToken := "h." + base64.RawURLEncoding.EncodeToString(claims) + ".s"
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			if r.PostForm.Get("code") != "good-code" || r.PostForm.Get("code_verifier") == "" || r.PostForm.Get("resource") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.accessToken = "access-1"
			writeJSON(w, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": 3600, "id_token": idToken})
		case "refresh_token":
			if r.PostForm.Get("refresh_token") != "refresh-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.refreshes++
			f.accessToken = "access-2"
			writeJSON(w, map[string]any{"access_token": "access-2", "expires_in": 3600})
		}
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		want := f.accessToken
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var rpc map[string]any
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		switch rpc["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "session-1")
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"serverInfo": map[string]string{"name": "Higgsfield AI"}}})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			if r.Header.Get("Mcp-Session-Id") != "session-1" {
				t.Errorf("tool call missing session id")
			}
			params, _ := rpc["params"].(map[string]any)
			name, _ := params["name"].(string)
			arguments, _ := params["arguments"].(map[string]any)
			f.mu.Lock()
			f.toolCalls = append(f.toolCalls, fakeToolCall{name: name, arguments: arguments})
			f.mu.Unlock()
			if name == "balance" {
				// Answer as a server-sent event, like the real server can.
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "event: message\ndata: "+`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"Credits: 782.1 | Plan: plus"}],"structuredContent":{"credits":782.1,"subscription_plan_type":"plus"}}}`+"\n\n")
				return
			}
			f.mu.Lock()
			refusal := f.refuseVideo
			f.mu.Unlock()
			if name == "generate_video" && refusal != "" {
				if params, _ := arguments["params"].(map[string]any); params["get_cost"] != true {
					writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": refusal}}}})
					return
				}
			}
			// The other tools return their JSON as text content only.
			text := f.toolResult(name, arguments)
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}})
		}
	})
	mux.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		if f.failPuts > 0 {
			f.failPuts--
			f.mu.Unlock()
			w.WriteHeader(520)
			return
		}
		f.uploaded = body
		f.uploads = append(f.uploads, body)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/result/image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(fakeHiggsfieldPNG)
	})
	mux.HandleFunc("/result/speech.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, "ID3fake-mp3-bytes")
	})
	mux.HandleFunc("/voices/sample.mp3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ID3sample")
	})
	mux.HandleFunc("/voices/clone.mp3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ID3selected-clone")
	})
	mux.HandleFunc("/result/video.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "fake-mp4-bytes")
	})
	return mux
}

type fakeToolCall struct {
	name      string
	arguments map[string]any
}

func (f *fakeHiggsfield) toolResult(name string, arguments map[string]any) string {
	params, _ := arguments["params"].(map[string]any)
	switch name {
	case "generate_video":
		if params["get_cost"] == true {
			return `{"cost":{"credits":5,"credits_exact":5}}`
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.submits++
		if f.recommendPreset && params["declined_preset_id"] == nil {
			return `{"notice":{"type":"preset_recommendation","message":"This prompt looks like the Higgsfield preset \"IN THE DARK\".","data":{"preset":{"id":"preset-1","name":"IN THE DARK"}},"retry_literal_with":{"declined_preset_id":"preset-1"}}}`
		}
		return `{"results":[{"id":"job-1","type":"video","status":"pending"}]}`
	case "generate_image":
		if params["get_cost"] == true {
			return `{"cost":{"credits":0.25,"credits_exact":0.25}}`
		}
		f.mu.Lock()
		f.imageParams = params
		f.mu.Unlock()
		return `{"results":[{"id":"job-img","type":"image","status":"pending"}]}`
	case "generate_audio":
		if params["get_cost"] == true {
			return `{"cost":{"credits":1.1,"credits_exact":1.1}}`
		}
		f.mu.Lock()
		f.audioParams = params
		f.mu.Unlock()
		return `{"results":[{"id":"job-audio","type":"audio","status":"pending"}]}`
	case "list_voices":
		if arguments["cursor"] == nil {
			return `{"voices":[{"voice_id":"preset-1","voice_type":"preset","name":"Grady","gender":"male","preview_url":"` + f.baseURL + `/voices/sample.mp3"},{"voice_id":"bad id","voice_type":"preset","name":"Broken"}],"has_more":true,"next_cursor":":2"}`
		}
		f.mu.Lock()
		cloned := len(f.voiceClones) > 0 && f.cloneReplyLost
		cloneHasSample, cloneStatus := f.cloneHasSample, f.cloneStatus
		f.mu.Unlock()
		if cloned {
			return `{"voices":[{"voice_id":"clone-1","voice_type":"element","name":"Jacob","gender":null},{"voice_id":"clone-3","voice_type":"element","name":"Maria","status":"processing"}],"has_more":false}`
		}
		if cloneHasSample {
			body, _ := json.Marshal(map[string]any{"voices": []map[string]any{{
				"voice_id": "clone-1", "voice_type": "element", "name": "Jacob", "status": cloneStatus,
				"preview_url": f.baseURL + "/voices/clone.mp3",
			}}, "has_more": false})
			return string(body)
		}
		return `{"voices":[{"voice_id":"clone-1","voice_type":"element","name":"Jacob","gender":null,"preview_url":"https://evil.example.com/a.mp3"}],"has_more":false}`
	case "create_voice_from_confirmed_audio":
		f.mu.Lock()
		f.voiceClones = append(f.voiceClones, arguments)
		lost := f.cloneReplyLost
		f.mu.Unlock()
		if lost {
			return `{"note":"voice clone queued"}`
		}
		return `{"voice_id":"clone-2","status":"processing","is_audio_eligible":false}`
	case "media_upload":
		return `{"uploads":[{"upload_url":"` + f.baseURL + `/upload/ref.png","media_id":"media-1","content_type":"image/png"}]}`
	case "media_confirm":
		return `{"results":[{"media_id":"media-1","status":"uploaded"}]}`
	case "jobs_wait":
		jobs, _ := arguments["jobs"].([]any)
		if len(jobs) == 1 {
			if job, _ := jobs[0].(map[string]any); job["job_id"] == "job-audio" {
				return `{"jobs":[{"index":0,"job_id":"job-audio","status":"completed","result_url":"` + f.baseURL + `/result/speech.mp3"}],"all_terminal":true}`
			}
			if job, _ := jobs[0].(map[string]any); job["job_id"] == "job-img" {
				return `{"jobs":[{"index":0,"job_id":"job-img","status":"completed","result_url":"` + f.baseURL + `/result/image.png"}],"all_terminal":true}`
			}
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.polls++
		if f.videoJobStatus != "" {
			body, _ := json.Marshal(map[string]any{"jobs": []map[string]any{{"index": 0, "job_id": "job-1", "status": f.videoJobStatus, "error": f.videoJobError}}, "all_terminal": true})
			return string(body)
		}
		if f.polls < 2 {
			return `{"jobs":[{"index":0,"job_id":"job-1","status":"in_progress"}],"all_terminal":false}`
		}
		return `{"jobs":[{"index":0,"job_id":"job-1","status":"completed","result_url":"` + f.baseURL + `/result/video.mp4"}],"all_terminal":true}`
	}
	return `{}`
}

func useFakeHiggsfield(t *testing.T) (*fakeHiggsfield, map[string]string) {
	t.Helper()
	fake := &fakeHiggsfield{}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	fake.baseURL = server.URL
	oldTestHost := higgsfieldTestHost
	higgsfieldTestHost = strings.TrimPrefix(server.URL, "http://")
	t.Cleanup(func() { higgsfieldTestHost = oldTestHost })
	store := map[string]string{}
	var storeMu sync.Mutex
	oldEndpoints, oldGet, oldSet, oldDelete, oldManager := higgsfieldEndpoints, higgsfieldKeyringGet, higgsfieldKeyringSet, higgsfieldKeyringDelete, glowbomHiggsfield
	higgsfieldEndpoints.register, higgsfieldEndpoints.authorize = server.URL+"/oauth/register", server.URL+"/oauth/authorize"
	higgsfieldEndpoints.token, higgsfieldEndpoints.mcp = server.URL+"/oauth/token", server.URL+"/mcp"
	higgsfieldKeyringGet = func(service, account string) (string, error) {
		storeMu.Lock()
		defer storeMu.Unlock()
		if value, ok := store[service+"/"+account]; ok {
			return value, nil
		}
		return "", keyring.ErrNotFound
	}
	higgsfieldKeyringSet = func(service, account, value string) error {
		storeMu.Lock()
		defer storeMu.Unlock()
		store[service+"/"+account] = value
		return nil
	}
	higgsfieldKeyringDelete = func(service, account string) error {
		storeMu.Lock()
		defer storeMu.Unlock()
		if _, ok := store[service+"/"+account]; !ok {
			return keyring.ErrNotFound
		}
		delete(store, service+"/"+account)
		return nil
	}
	glowbomHiggsfield = &higgsfieldManager{logins: map[string]*higgsfieldLogin{}, server: &http.Server{}} // skip the real loopback listener
	oldFFmpeg := higgsfieldReferenceFFmpeg
	higgsfieldReferenceFFmpeg = func() (string, error) { return "", errors.New("no ffmpeg in tests") }
	t.Cleanup(func() { higgsfieldReferenceFFmpeg = oldFFmpeg })
	t.Cleanup(func() {
		higgsfieldEndpoints, higgsfieldKeyringGet, higgsfieldKeyringSet, higgsfieldKeyringDelete, glowbomHiggsfield = oldEndpoints, oldGet, oldSet, oldDelete, oldManager
	})
	return fake, store
}

func TestHiggsfieldSignInStatusAndLogout(t *testing.T) {
	fake, store := useFakeHiggsfield(t)
	ctx := context.Background()

	authorizationURL, state, err := glowbomHiggsfield.startLogin(ctx)
	if err != nil {
		t.Fatalf("startLogin: %v", err)
	}
	parsed, _ := neturl.Parse(authorizationURL)
	q := parsed.Query()
	if q.Get("client_id") != "client-123" || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != higgsfieldRedirectURI || q.Get("state") != state {
		t.Fatalf("unexpected authorization URL: %s", authorizationURL)
	}
	if status := glowbomHiggsfield.status(ctx, false); status.Connected || !status.LoginPending {
		t.Fatalf("expected pending, not connected: %+v", status)
	}
	if ok, message := glowbomHiggsfield.finish(state, "good-code", ""); !ok {
		t.Fatalf("finish failed: %s", message)
	}
	if ok, _ := glowbomHiggsfield.finish(state, "good-code", ""); ok {
		t.Fatalf("a sign-in state must not be reusable")
	}

	status := glowbomHiggsfield.status(ctx, false)
	if !status.Connected || status.Plan != "plus" || status.Credits == nil || *status.Credits != 782.1 || status.Email != "jacob@example.com" {
		t.Fatalf("unexpected status: %+v", status)
	}
	for _, ua := range fake.userAgents {
		if !strings.HasPrefix(ua, "Glowbom (") {
			t.Fatalf("requests must send the Glowbom user agent, got %q", ua)
		}
	}
	if strings.Contains(store[higgsfieldKeyService+"/"+higgsfieldKeyAccount], "client-123") {
		t.Fatalf("client id should be stored separately from tokens")
	}

	if err := glowbomHiggsfield.logout(); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if status := glowbomHiggsfield.status(ctx, false); status.Connected {
		t.Fatalf("expected signed out after logout: %+v", status)
	}
	if store[higgsfieldKeyService+"/"+higgsfieldClientAccount] != "client-123" {
		t.Fatalf("logout should keep the public client registration")
	}
	if _, _, err := glowbomHiggsfield.startLogin(ctx); err != nil || fake.registrations != 1 {
		t.Fatalf("a second sign-in should reuse the registration (registrations=%d, err=%v)", fake.registrations, err)
	}
}

func TestHiggsfieldRefreshesExpiredToken(t *testing.T) {
	fake, _ := useFakeHiggsfield(t)
	ctx := context.Background()
	_, state, err := glowbomHiggsfield.startLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	glowbomHiggsfield.finish(state, "good-code", "")

	credential, _ := higgsfieldLoadCredential()
	credential.ExpiresAt = time.Now().Add(-time.Minute)
	_ = higgsfieldSaveCredential(*credential)

	status := glowbomHiggsfield.status(ctx, true)
	if !status.Connected || status.Credits == nil || fake.refreshes != 1 {
		t.Fatalf("expected one refresh and a balance: %+v refreshes=%d", status, fake.refreshes)
	}
	saved, _ := higgsfieldLoadCredential()
	if saved.AccessToken != "access-2" || saved.RefreshToken != "refresh-1" || saved.Email != "jacob@example.com" {
		t.Fatalf("refresh should keep the refresh token and email: %+v", saved)
	}
}

func TestHiggsfieldRejectsBadCodeAndCancel(t *testing.T) {
	useFakeHiggsfield(t)
	ctx := context.Background()
	_, state, _ := glowbomHiggsfield.startLogin(ctx)
	if ok, _ := glowbomHiggsfield.finish(state, "bad-code", ""); ok {
		t.Fatalf("a bad code must not connect")
	}
	if status := glowbomHiggsfield.status(ctx, false); status.Connected || status.Error == "" {
		t.Fatalf("expected a visible failure: %+v", status)
	}
	_, state, _ = glowbomHiggsfield.startLogin(ctx)
	glowbomHiggsfield.cancel(state)
	if ok, _ := glowbomHiggsfield.finish(state, "good-code", ""); ok {
		t.Fatalf("a canceled sign-in must not complete")
	}
	if ok, _ := glowbomHiggsfield.finish("unknown-state", "good-code", ""); ok {
		t.Fatalf("an unknown state must not complete")
	}
}

func TestHiggsfieldSettingsHandlerRequiresLocalToken(t *testing.T) {
	useFakeHiggsfield(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "local-token")
	req := httptest.NewRequest(http.MethodGet, "/settings/higgsfield/status", nil)
	rec := httptest.NewRecorder()
	higgsfieldSettingsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without the local token, got %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/settings/higgsfield/status", nil)
	req.Header.Set("X-Glowbom-Token", "local-token")
	rec = httptest.NewRecorder()
	higgsfieldSettingsHandler(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"connected":false`) {
		t.Fatalf("expected a signed-out status, got %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/settings/higgsfield/logout", nil)
	req.Header.Set("X-Glowbom-Token", "local-token")
	rec = httptest.NewRecorder()
	higgsfieldSettingsHandler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("logout must require POST, got %d", rec.Code)
	}
}

// A real 1x1 PNG, so Studio's image normalization can decode the result.
var fakeHiggsfieldPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
