package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func desktopTestConnection(t *testing.T, endpoint string) desktopConnection {
	t.Helper()
	return desktopConnection{
		Version: 1, Protocol: desktopProtocol, Endpoint: endpoint,
		InstallationID: "test-installation", LaunchID: "test-launch", Token: strings.Repeat("a", 64),
	}
}

func writeDesktopTestConnection(t *testing.T, dir string, connection desktopConnection) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(connection)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, connection.InstallationID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func desktopTestServer(t *testing.T, commandHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/identity" {
			if r.Header.Get("Authorization") != "" {
				t.Error("identity check included a bearer token")
			}
			fmt.Fprintf(w, `{"version":1,"protocol":%q,"installationId":"test-installation","launchId":"test-launch"}`, desktopProtocol)
			return
		}
		if r.URL.Path != "/v1/commands" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("a", 64) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		commandHandler(w, r)
	}))
	t.Cleanup(server.Close)
	path := writeDesktopTestConnection(t, t.TempDir(), desktopTestConnection(t, server.URL))
	t.Setenv(desktopConnectionEnv, path)
	return server
}

func TestDesktopSubmitSendsStableBytesAndRetryIdentity(t *testing.T) {
	file := filepath.Join(t.TempDir(), "result.txt")
	contents := []byte("A local result\n")
	if err := os.WriteFile(file, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var command desktopCommand
		if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
			t.Fatal(err)
		}
		calls++
		if command.Version != 1 || command.Operation != "tasks.submit" || command.Actor != "codex" || command.RequestID != "same-result" {
			t.Errorf("unexpected command envelope: %#v", command)
		}
		if command.Payload["taskId"] != "task-1" || command.Payload["attemptId"] != "attempt-1" || command.Payload["revision"] != float64(2) || command.Payload["text"] != "Ready to review" {
			t.Errorf("unexpected task payload: %#v", command.Payload)
		}
		attachments := command.Payload["attachments"].([]any)
		attachment := attachments[0].(map[string]any)
		digest := sha256.Sum256(contents)
		if len(attachments) != 1 || attachment["name"] != "result.txt" || attachment["dataBase64"] != base64.StdEncoding.EncodeToString(contents) || attachment["sha256"] != hex.EncodeToString(digest[:]) {
			t.Errorf("attachment bytes or metadata changed: %#v", attachment)
		}
		encoded, _ := json.Marshal(command)
		if strings.Contains(string(encoded), filepath.Dir(file)) {
			t.Error("request leaked a local file path")
		}
		fmt.Fprint(w, `{"version":1,"ok":true,"requestId":"same-result","result":{"task":{"id":"task-1","status":"review","messages":[{"text":"Earlier context"}],"results":[{"id":"result-1"}]}}}`)
	})
	args := []string{"tasks", "submit", "task-1", "--agent", "codex", "--attempt", "attempt-1", "--revision", "2", "--message", "Ready to review", "--file", file, "--request-id", "same-result", "--json"}
	for attempt := 0; attempt < 2; attempt++ {
		var out, errOut bytes.Buffer
		if code := runDesktopWithIO(context.Background(), args, &out, &errOut); code != 0 {
			t.Fatalf("submit returned %d: %s %s", code, out.String(), errOut.String())
		}
		var response desktopResponse
		if err := json.Unmarshal(out.Bytes(), &response); err != nil || !response.OK || response.RequestID != "same-result" || !strings.Contains(string(response.Result), "Earlier context") {
			t.Fatalf("response lost history or retry identity: %s", out.String())
		}
		if errOut.Len() != 0 || strings.Count(out.String(), "\n") != 1 {
			t.Fatalf("JSON output was mixed with diagnostics: %q %q", out.String(), errOut.String())
		}
	}
	if calls != 2 {
		t.Fatalf("server received %d submit calls", calls)
	}
}

func TestDesktopNextWithoutWorkSucceeds(t *testing.T) {
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var command desktopCommand
		_ = json.NewDecoder(r.Body).Decode(&command)
		if command.Operation != "tasks.next" || command.Payload["agent"] != "claude" || command.Actor != "claude" || command.RequestID == "" {
			t.Errorf("unexpected next command: %#v", command)
		}
		fmt.Fprint(w, `{"version":1,"ok":true,"result":{"task":null}}`)
	})
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"tasks", "next", "--agent", "claude", "--json"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), `"task":null`) {
		t.Fatalf("no work should succeed: %d %s %s", code, out.String(), errOut.String())
	}
}

func TestDesktopCreateSeparatesAssigneeAndAuthor(t *testing.T) {
	args := []string{"tasks", "create", "--agent", "jessica", "--title", "Launch art", "--instructions", "Create the image"}
	invocation, err := parseDesktopCommand(args)
	if err != nil || invocation.command.Actor != "" || invocation.command.Payload["agent"] != "jessica" {
		t.Fatalf("owner create attribution changed: %#v %v", invocation, err)
	}
	invocation, err = parseDesktopCommand(append(args, "--actor", "herakles"))
	if err != nil || invocation.command.Actor != "herakles" || invocation.command.Payload["agent"] != "jessica" {
		t.Fatalf("delegated create attribution changed: %#v %v", invocation, err)
	}
}

func TestDesktopServiceErrorsStayStructured(t *testing.T) {
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"version":1,"ok":false,"error":{"code":"stale_attempt","message":"Read the latest task before submitting."}}`)
	})
	var out, errOut bytes.Buffer
	code := runDesktopWithIO(context.Background(), []string{"tasks", "show", "task-1", "--json"}, &out, &errOut)
	var response desktopResponse
	if code != 1 || json.Unmarshal(out.Bytes(), &response) != nil || response.Error == nil || response.Error.Code != "stale_attempt" || errOut.Len() != 0 {
		t.Fatalf("lost structured service error: %d %s %s", code, out.String(), errOut.String())
	}
}

func TestDesktopHelpAndInvalidArgumentsDoNotNeedDesktop(t *testing.T) {
	t.Setenv(desktopConnectionEnv, "/missing/connection.json")
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "tasks submit") {
		t.Fatalf("help failed without Desktop: %d %s", code, errOut.String())
	}
	for _, args := range [][]string{
		{"tasks", "submit", "task-1", "--message", "Done", "--json"},
		{"tasks", "accept", "task-1", "--json"},
		{"tasks", "create", "--agent", "codex", "--title", "Test", "--instructions", "One", "--instructions-file", "two", "--json"},
		{"agents", "register", "--username", "codex", "--name", "Codex", "--role", "worker", "--json"},
		{"tasks", "next", "--agent", "codex", "--request-id", "bad/id", "--json"},
	} {
		out.Reset()
		errOut.Reset()
		code := runDesktopWithIO(context.Background(), args, &out, &errOut)
		var response desktopResponse
		if code != 2 || json.Unmarshal(out.Bytes(), &response) != nil || response.Error == nil || response.Error.Code != "invalid_arguments" {
			t.Errorf("invalid input %q was not rejected before discovery: %d %s", args, code, out.String())
		}
	}
}

func TestDesktopConnectionRejectsUnsafeRecords(t *testing.T) {
	for _, endpoint := range []string{"https://example.com:443", "http://localhost:4567", "http://192.168.1.5:4567", "http://127.0.0.1:4567/../path", "http://127.0.0.1:4567//path", "http://user@127.0.0.1:4567", "http://127.0.0.1:4567?token=value"} {
		path := writeDesktopTestConnection(t, t.TempDir(), desktopTestConnection(t, endpoint))
		if _, err := readDesktopConnection(path); err == nil {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
	path := writeDesktopTestConnection(t, t.TempDir(), desktopTestConnection(t, "http://127.0.0.1:4567"))
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopConnection(path); err == nil {
		t.Error("accepted a connection file readable by other users")
	}
	link := filepath.Join(t.TempDir(), "connection.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopConnection(link); err == nil {
		t.Error("accepted a symbolic connection file")
	}
}

func TestDesktopSupportsServicePathPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__glowbom/tasks/v1/identity":
			fmt.Fprintf(w, `{"version":1,"protocol":%q,"installationId":"test-installation","launchId":"test-launch"}`, desktopProtocol)
		case "/__glowbom/tasks/v1/commands":
			fmt.Fprint(w, `{"version":1,"ok":true,"result":{"agents":[]}}`)
		default:
			t.Errorf("lost endpoint prefix: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv(desktopConnectionEnv, writeDesktopTestConnection(t, t.TempDir(), desktopTestConnection(t, server.URL+"/__glowbom/tasks")))
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"agents", "list", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("prefixed Desktop service failed: %d %s %s", code, out.String(), errOut.String())
	}
}

func TestDesktopStaleIdentityNeverReceivesBearer(t *testing.T) {
	var authenticated atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			authenticated.Add(1)
		}
		fmt.Fprintf(w, `{"version":1,"protocol":%q,"installationId":"test-installation","launchId":"different-launch"}`, desktopProtocol)
	}))
	defer server.Close()
	t.Setenv(desktopConnectionEnv, writeDesktopTestConnection(t, t.TempDir(), desktopTestConnection(t, server.URL)))
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"agents", "list", "--json"}, &out, &errOut); code != 1 || authenticated.Load() != 0 || !strings.Contains(out.String(), "out of date") {
		t.Fatalf("stale service received auth or succeeded: %d %s", code, out.String())
	}
}

func TestDesktopDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Add(1) }))
	defer other.Close()
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	})
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"agents", "list", "--json"}, &out, &errOut); code != 1 || followed.Load() != 0 || strings.Contains(out.String(), strings.Repeat("a", 64)) {
		t.Fatalf("redirect succeeded or exposed auth: %d %s", code, out.String())
	}
}

func TestDesktopCancelledCommandDoesNotSendMutation(t *testing.T) {
	var calls atomic.Int32
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(ctx, []string{"tasks", "next", "--agent", "codex", "--json"}, &out, &errOut); code != 1 || calls.Load() != 0 {
		t.Fatalf("cancelled command sent a mutation: %d %s", code, out.String())
	}
}

func TestDesktopFilesRejectSymlinksOversizeAndInvalidText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "result.txt")
	if err := os.WriteFile(file, []byte{0xff, 0, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopText(file); err == nil {
		t.Error("accepted binary input as text")
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, dir} {
		if _, err := readDesktopAttachments([]string{path}); err == nil {
			t.Errorf("accepted non-regular attachment %s", path)
		}
	}
	if err := os.Truncate(file, desktopAttachmentLimit+1); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopAttachments([]string{file}); err == nil {
		t.Error("accepted oversized attachment")
	}
	if _, err := readDesktopAttachments(make([]string, 9)); err == nil {
		t.Error("accepted too many attachments")
	}
}

func TestDesktopFilesGetVerifiesBytesAndPreservesExistingOutput(t *testing.T) {
	contents := []byte("A brief attached to the task.\n")
	digest := sha256.Sum256(contents)
	result, err := json.Marshal(map[string]any{
		"attachment": desktopSavedAttachment{ID: "file-1", Name: "brief.txt", MimeType: "text/plain", SizeBytes: int64(len(contents)), SHA256: hex.EncodeToString(digest[:])},
		"dataBase64": base64.StdEncoding.EncodeToString(contents),
	})
	if err != nil {
		t.Fatal(err)
	}
	desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var command desktopCommand
		if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
			t.Error(err)
		}
		if command.Operation != "attachments.read" || command.Payload["id"] != "file-1" {
			t.Errorf("unexpected attachment read: %#v", command)
		}
		_ = json.NewEncoder(w).Encode(desktopResponse{Version: 1, OK: true, Result: result})
	})
	dir := t.TempDir()
	output := filepath.Join(dir, "brief.txt")
	args := []string{"files", "get", "file-1", "--output", output, "--json"}
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("file download failed: %d %s %s", code, out.String(), errOut.String())
	}
	data, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("download changed attachment bytes: %q %v", data, err)
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("download was not private: %v %v", info, err)
	}
	if strings.Contains(out.String(), "dataBase64") || !strings.Contains(out.String(), `"output":`) {
		t.Fatalf("download output should contain the saved path, not encoded bytes: %s", out.String())
	}
	if err := os.WriteFile(output, []byte("Keep this existing file."), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := runDesktopWithIO(context.Background(), args, &out, &errOut); code != 1 {
		t.Fatalf("download replaced existing output: %d %s", code, out.String())
	}
	data, _ = os.ReadFile(output)
	if string(data) != "Keep this existing file." {
		t.Fatal("existing output was changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary download was left behind: %v %v", entries, err)
	}
}

func TestDesktopFileDownloadRejectsCorruptionAndCancellation(t *testing.T) {
	contents := []byte("Complete file")
	digest := sha256.Sum256(contents)
	for _, test := range []struct {
		name, id, digest, data string
		cancel                 bool
	}{
		{"wrong checksum", "file-1", strings.Repeat("0", 64), base64.StdEncoding.EncodeToString(contents), false},
		{"wrong ID", "file-2", hex.EncodeToString(digest[:]), base64.StdEncoding.EncodeToString(contents), false},
		{"truncated", "file-1", hex.EncodeToString(digest[:]), base64.StdEncoding.EncodeToString(contents[:3]), false},
		{"cancelled", "file-1", hex.EncodeToString(digest[:]), base64.StdEncoding.EncodeToString(contents), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, _ := json.Marshal(map[string]any{
				"attachment": desktopSavedAttachment{ID: test.id, Name: "file.txt", SizeBytes: int64(len(contents)), SHA256: test.digest},
				"dataBase64": test.data,
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancel {
				cancel()
			}
			dir := t.TempDir()
			output := filepath.Join(dir, "new.txt")
			if _, err := saveDesktopAttachment(ctx, result, "file-1", output); err == nil {
				t.Fatal("accepted invalid download")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed download left files behind: %v %v", entries, err)
			}
		})
	}
}
