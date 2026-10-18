package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type companionTasksFixture struct {
	connection companionTasksConnection
	path       string
	uid        string
	commands   []companionTasksCommand
	respond    func(http.ResponseWriter, companionTasksCommand)
}

func newCompanionTasksFixture(t *testing.T) *companionTasksFixture {
	t.Helper()
	fixture := &companionTasksFixture{uid: "account-one", connection: companionTasksConnection{
		Version: 1, Protocol: companionTasksProtocol, InstallationID: "desktop", LaunchID: "launch-one", Token: strings.Repeat("b", 64),
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/identity" {
			if r.Header.Get("Authorization") != "" {
				t.Error("connection credentials reached the identity check")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "protocol": companionTasksProtocol,
				"installationId": fixture.connection.InstallationID, "launchId": fixture.connection.LaunchID})
			return
		}
		if r.URL.Path != "/v1/commands" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+fixture.connection.Token {
			t.Error("wrong local Tasks request")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var request struct {
			companionTasksCommand
			Actor string `json:"actor"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Actor != "owner" || request.ScopeID != "" {
			t.Error("command did not use the owner identity")
			return
		}
		if request.Operation == "session.open" {
			_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "ok": true, "result": map[string]any{
				"session": map[string]any{"uid": fixture.uid, "installationId": fixture.connection.InstallationID, "launchId": fixture.connection.LaunchID}}})
			return
		}
		fixture.commands = append(fixture.commands, request.companionTasksCommand)
		if fixture.respond != nil {
			fixture.respond(w, request.companionTasksCommand)
			return
		}
		result := map[string]any{"task": map[string]any{"id": "task-one", "status": "queued"}}
		if request.Operation == "snapshot" {
			result = map[string]any{"agents": []any{}, "tasks": []any{}, "session": map[string]any{"uid": fixture.uid, "email": "private@example.test"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "ok": true, "requestId": request.RequestID, "result": result})
	}))
	t.Cleanup(server.Close)
	fixture.connection.Endpoint = server.URL
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.path = filepath.Join(directory, "desktop.json")
	fixture.save(t)
	return fixture
}

func (fixture *companionTasksFixture) save(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(fixture.connection)
	if err != nil || os.WriteFile(fixture.path, data, 0o600) != nil {
		t.Fatal("could not save test connection")
	}
}

func companionTasksTestSession(t *testing.T) *companionSession {
	t.Helper()
	session := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/account/status" || r.Method != http.MethodGet {
			t.Error("unexpected internal request")
		}
		writeJSON(w, map[string]any{"version": 1, "status": "signed_in", "uid": "account-one"})
	}))
	// Keep tests isolated from the user's persistent machine scope.
	session.tasksScopeSalt = bytes.Repeat([]byte{1}, 32)
	return session
}

func companionTasksTestRequest(t *testing.T, session *companionSession, body string) *httptest.ResponseRecorder {
	t.Helper()
	var command map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &command) == nil && string(command["operation"]) != `"snapshot"` {
		if _, exists := command["scopeId"]; !exists {
			command["scopeId"], _ = json.Marshal(companionTasksScope(session.tasksScopeSalt, "desktop", "account-one"))
			encoded, _ := json.Marshal(command)
			body = string(encoded)
		}
	}
	request := companionRequest(session, http.MethodPost, "/tasks/commands", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	session.ServeHTTP(response, request)
	return response
}

func TestCompanionTasksPreservesPairingBoundary(t *testing.T) {
	session := testCompanion(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("an unauthenticated request reached the backend")
	}))
	for _, kind := range []string{"token", "origin", "host", "remote"} {
		t.Run(kind, func(t *testing.T) {
			request := companionRequest(session, http.MethodPost, "/tasks/commands", `{"version":1,"operation":"snapshot","payload":{}}`)
			request.Header.Set("Content-Type", "application/json")
			switch kind {
			case "token":
				request.Header.Set("Authorization", "Bearer wrong")
			case "origin":
				request.Header.Set("Origin", "https://example.test")
			case "host":
				request.Host = "example.test"
			case "remote":
				request.RemoteAddr = "8.8.8.8:5000"
			}
			response := httptest.NewRecorder()
			session.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
				t.Fatalf("unexpected status %d", response.Code)
			}
		})
	}
}

func TestCompanionTasksAllowsOnlyOwnerInboxCommands(t *testing.T) {
	session := testCompanion(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("an unsupported command reached discovery")
	}))
	for _, body := range []string{
		`{"version":1,"operation":"tasks.next","payload":{}}`,
		`{"version":1,"operation":"tasks.submit","payload":{}}`,
		`{"version":1,"operation":"agents.register","payload":{}}`,
		`{"version":1,"operation":"chat.send","payload":{}}`,
		`{"version":1,"operation":"tasks.delete","payload":{}}`,
		`{"version":1,"operation":"snapshot","actor":"agent","payload":{}}`,
		`{"version":1,"operation":"snapshot","actor":"owner","payload":{}}`,
		`{"version":1,"operation":"tasks.reply","requestId":"retry","payload":{"actor":"agent"}}`,
		`{"version":1,"operation":"tasks.reply","requestId":"retry","payload":{"taskId text":"wrong"}}`,
		`{"version":1,"operation":"tasks.reply","payload":{"taskId":"one","text":"hello"}}`,
		`{"version":1,"operation":"snapshot","payload":null}`,
		`{"version":2,"operation":"snapshot","payload":{}}`,
		`{"version":1,"operation":"snapshot","payload":{}} {}`,
	} {
		if response := companionTasksTestRequest(t, session, body); response.Code != http.StatusBadRequest {
			t.Fatalf("unsupported command was accepted: %s: %d", body, response.Code)
		}
	}
}

func TestCompanionTasksSnapshotAndRetryKeepIdentityPrivate(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	session := companionTasksTestSession(t)
	response := companionTasksTestRequest(t, session, `{"version":1,"operation":"snapshot","payload":{}}`)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "session") || strings.Contains(response.Body.String(), fixture.uid) || strings.Contains(response.Body.String(), fixture.connection.Token) || strings.Contains(response.Body.String(), fixture.connection.Endpoint) {
		t.Fatalf("snapshot exposed connection details or failed: %d %s", response.Code, response.Body)
	}
	var snapshot struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if json.Unmarshal(response.Body.Bytes(), &snapshot) != nil || len(snapshot.Result) != 3 || string(snapshot.Result["agents"]) != "[]" || string(snapshot.Result["tasks"]) != "[]" || len(snapshot.Result["scopeId"]) != 66 {
		t.Fatal("snapshot did not preserve its lowercase agents and tasks fields")
	}
	body := `{"version":1,"operation":"tasks.reply","requestId":"same-retry","payload":{"taskId":"task-one","revision":2,"text":"Please continue","attachments":[{"name":"note.txt","mimeType":"text/plain","dataBase64":"aGk="}]}}`
	for range 2 {
		response = companionTasksTestRequest(t, session, body)
		if response.Code != http.StatusOK {
			t.Fatalf("reply failed: %d %s", response.Code, response.Body)
		}
	}
	if len(fixture.commands) != 3 || fixture.commands[1].RequestID != "same-retry" || fixture.commands[2].RequestID != "same-retry" || string(fixture.commands[1].Payload["attachments"]) != string(fixture.commands[2].Payload["attachments"]) {
		t.Fatal("retry identity or attachments changed")
	}
}

func TestCompanionTasksScopeBlocksOldAccountAndSurvivesRestart(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	session := companionTasksTestSession(t)
	snapshotScope := func(session *companionSession) string {
		t.Helper()
		response := companionTasksTestRequest(t, session, `{"version":1,"operation":"snapshot","payload":{}}`)
		var snapshot struct {
			Result struct {
				ScopeID string `json:"scopeId"`
			} `json:"result"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &snapshot) != nil || len(snapshot.Result.ScopeID) != 64 {
			t.Fatalf("scope was not returned: %d %s", response.Code, response.Body)
		}
		return snapshot.Result.ScopeID
	}
	original := snapshotScope(session)
	fixture.connection.LaunchID = "new-launch"
	fixture.save(t)
	if next := snapshotScope(companionTasksTestSession(t)); next != original {
		t.Fatal("same machine, installation and account lost their scope after restart")
	}
	fixture.uid = "new-account"
	session.api = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"version": 1, "status": "signed_in", "uid": fixture.uid})
	})
	changed := snapshotScope(session)
	if changed == original {
		t.Fatal("different account retained the earlier inbox scope")
	}
	before := len(fixture.commands)
	response := companionTasksTestRequest(t, session, `{"version":1,"operation":"tasks.create","requestId":"pending-from-earlier-account","scopeId":"`+original+`","payload":{"agent":"jessica","instructions":"Continue"}}`)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "task_scope_changed") || len(fixture.commands) != before {
		t.Fatalf("old request reached the new account: %d %s", response.Code, response.Body)
	}
	for _, scope := range []string{"", original} {
		response = companionTasksTestRequest(t, session, `{"version":1,"operation":"files.read","scopeId":"`+scope+`","payload":{"id":"file-one"}}`)
		if response.Code != http.StatusConflict || len(fixture.commands) != before {
			t.Fatal("attachment read bypassed inbox scope")
		}
	}
	otherMachine := companionTasksTestSession(t)
	otherMachine.tasksScopeSalt = bytes.Repeat([]byte{2}, 32)
	otherMachine.api = session.api
	if snapshotScope(otherMachine) == changed {
		t.Fatal("two computers with the same account and installation label shared an inbox scope")
	}
}

func TestCompanionTasksScopeSaltIsPrivateStableAndAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop", "companion-tasks-scope")
	var group sync.WaitGroup
	results := make(chan []byte, 12)
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			salt, err := readCompanionTasksScopeSalt(path)
			if err != nil {
				t.Error(err)
				return
			}
			results <- salt
		}()
	}
	group.Wait()
	close(results)
	var first []byte
	for salt := range results {
		if first == nil {
			first = salt
		}
		if len(salt) != 32 || !bytes.Equal(first, salt) {
			t.Fatal("concurrent launches created inconsistent scopes")
		}
	}
	if len(first) != 32 {
		t.Fatal("scope was not saved")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("scope file is not private")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompanionTasksScopeSalt(path); err == nil {
		t.Fatal("public scope file was accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, first, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompanionTasksScopeSalt(path); err == nil {
		t.Fatal("scope symlink was accepted")
	}
}

func TestCompanionTasksScopeUsesPrivateFolderInsideExistingDesktopFolder(t *testing.T) {
	desktop := filepath.Join(t.TempDir(), "desktop")
	if err := os.Mkdir(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(desktop, "companion-tasks", "scope")
	first, err := readCompanionTasksScopeSalt(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := readCompanionTasksScopeSalt(path)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("scope did not remain stable inside the existing Desktop folder")
	}
	for directory, expected := range map[string]os.FileMode{desktop: 0o755, filepath.Dir(path): 0o700, path: 0o600} {
		info, err := os.Stat(directory)
		if err != nil || info.Mode().Perm() != expected {
			t.Fatalf("unexpected permissions for %s: expected %o", filepath.Base(directory), expected)
		}
	}
}

func TestCompanionTasksSupportedCommandsPreservePayloads(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	session := companionTasksTestSession(t)
	for _, command := range []struct{ operation, payload string }{
		{"agents.list", `{}`},
		{"tasks.show", `{"taskId":"task-one"}`},
		{"tasks.create", `{"agent":"jessica","instructions":"Review the draft","attachments":[{"name":"draft.txt","mimeType":"text/plain","dataBase64":"aGk="}]}`},
		{"tasks.accept", `{"taskId":"task-one","resultId":"result-one","revision":2}`},
		{"tasks.cancel", `{"taskId":"task-one","revision":2}`},
		{"files.read", `{"id":"attachment-one"}`},
	} {
		body := `{"version":1,"operation":"` + command.operation + `","requestId":"request-one","payload":` + command.payload + `}`
		response := companionTasksTestRequest(t, session, body)
		if response.Code != http.StatusOK {
			t.Fatalf("%s failed: %d %s", command.operation, response.Code, response.Body)
		}
		forwarded := fixture.commands[len(fixture.commands)-1]
		actual, _ := json.Marshal(forwarded.Payload)
		var expected map[string]json.RawMessage
		_ = json.Unmarshal([]byte(command.payload), &expected)
		wanted, _ := json.Marshal(expected)
		if forwarded.Operation != command.operation || string(actual) != string(wanted) {
			t.Fatalf("%s changed its payload", command.operation)
		}
	}
}

func TestCompanionTasksAccountAndServiceFailuresStaySeparateFromPairing(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	session := companionTasksTestSession(t)
	fixture.uid = "another-account"
	response := companionTasksTestRequest(t, session, `{"version":1,"operation":"snapshot","payload":{}}`)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "tasks_account_changed") || len(fixture.commands) != 0 {
		t.Fatalf("another account was exposed: %d %s", response.Code, response.Body)
	}
	fixture.uid = "account-one"
	fixture.respond = func(w http.ResponseWriter, command companionTasksCommand) {
		companionTasksFailure(w, http.StatusUnauthorized, command.RequestID, "unauthorized", "Reopen Tasks.")
	}
	response = companionTasksTestRequest(t, session, `{"version":1,"operation":"snapshot","payload":{}}`)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "tasks_unavailable") {
		t.Fatalf("a task credential failure would discard pairing: %d %s", response.Code, response.Body)
	}
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", filepath.Join(t.TempDir(), "missing.json"))
	response = companionTasksTestRequest(t, session, `{"version":1,"operation":"snapshot","payload":{}}`)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "Open Tasks") {
		t.Fatalf("missing service was not actionable: %d %s", response.Code, response.Body)
	}
}

func TestCompanionTasksDiscoveryRejectsAmbiguityAndStaleIdentity(t *testing.T) {
	first, second := newCompanionTasksFixture(t), newCompanionTasksFixture(t)
	client := companionTasksHTTPClient()
	defer client.CloseIdleConnections()
	if _, err := discoverCompanionTasks(context.Background(), client, []string{first.path, second.path}); !errors.Is(err, errCompanionTasksAmbiguous) {
		t.Fatal("multiple live sessions were accepted")
	}
	first.connection.LaunchID = "changed-after-file-written"
	if _, err := discoverCompanionTasks(context.Background(), client, []string{first.path}); err == nil {
		t.Fatal("stale launch identity was accepted")
	}
	connection, err := discoverCompanionTasks(context.Background(), client, []string{first.path, second.path})
	if err != nil || connection.Endpoint != second.connection.Endpoint {
		t.Fatal("the only matching live session was not selected")
	}
}

func TestCompanionTasksConnectionRequiresPrivateLoopbackFile(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	original := fixture.connection.Endpoint
	for _, endpoint := range []string{
		"http://example.test:1234", "http://127.0.0.1.example.test:1234", "http://localhost:1234", "http://192.168.1.2:1234",
		"https://127.0.0.1:1234", "http://user@127.0.0.1:1234", "http://127.0.0.1:1234/path?key=value",
		"http://127.0.0.1:1234/#fragment", "http://127.0.0.1:1234/../other", "http://127.0.0.1:1234/%2fother",
	} {
		fixture.connection.Endpoint = endpoint
		fixture.save(t)
		if _, err := companionTasksReadConnection(fixture.path); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
	fixture.connection.Endpoint = original
	fixture.save(t)
	if err := os.Chmod(fixture.path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := companionTasksReadConnection(fixture.path); err == nil {
		t.Fatal("non-private connection accepted")
	}
	if err := os.Chmod(fixture.path, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(fixture.path), "linked.json")
	if err := os.Symlink(fixture.path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := companionTasksReadConnection(link); err == nil {
		t.Fatal("symbolic link connection accepted")
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(fixture.path), "glowbom.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := companionTasksReadConnection(fixture.path); err == nil {
		t.Fatal("connection inside a project accepted")
	}
}

func TestCompanionTasksDoesNotFollowRedirects(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("Tasks followed a redirect")
	}))
	defer target.Close()
	fixture.respond = func(w http.ResponseWriter, command companionTasksCommand) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	response := companionTasksTestRequest(t, companionTasksTestSession(t), `{"version":1,"operation":"snapshot","payload":{}}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("redirect was accepted: %d", response.Code)
	}
}

func TestCompanionTasksBoundsRequestsAndResponses(t *testing.T) {
	session := companionTasksTestSession(t)
	request := companionRequest(session, http.MethodPost, "/tasks/commands", strings.Repeat(" ", companionTasksRequestLimit+1))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	session.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request accepted: %d", response.Code)
	}
	fixture := newCompanionTasksFixture(t)
	fixture.respond = func(w http.ResponseWriter, command companionTasksCommand) {
		_, _ = io.WriteString(w, `{"version":1,"ok":true,"result":{"content":"`)
		_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", companionTasksResponseLimit+1)), companionTasksResponseLimit+1)
		_, _ = io.WriteString(w, `"}}`)
	}
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	response = companionTasksTestRequest(t, session, `{"version":1,"operation":"files.read","payload":{"id":"attachment"}}`)
	if response.Code != http.StatusBadGateway || response.Body.Len() > 1000 {
		t.Fatalf("oversized response escaped its limit: %d", response.Code)
	}
}

func TestCompanionTasksForwardsTaskConflictsAndRejectsWrongReceipts(t *testing.T) {
	fixture := newCompanionTasksFixture(t)
	t.Setenv("GLOWBOM_DESKTOP_CONNECTION_FILE", fixture.path)
	session := companionTasksTestSession(t)
	fixture.respond = func(w http.ResponseWriter, command companionTasksCommand) {
		companionTasksFailure(w, http.StatusConflict, command.RequestID, "stale_revision", "Read this task again before replying.")
	}
	body := `{"version":1,"operation":"tasks.reply","requestId":"retry-one","payload":{"taskId":"one","revision":1,"text":"Please continue"}}`
	response := companionTasksTestRequest(t, session, body)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "stale_revision") {
		t.Fatalf("task conflict was hidden: %d %s", response.Code, response.Body)
	}
	fixture.respond = func(w http.ResponseWriter, command companionTasksCommand) {
		writeJSON(w, map[string]any{"version": 1, "ok": true, "requestId": "another-request", "result": map[string]any{"task": map[string]any{}}})
	}
	response = companionTasksTestRequest(t, session, body)
	if response.Code != http.StatusBadGateway {
		t.Fatal("receipt for another request was accepted")
	}
}
