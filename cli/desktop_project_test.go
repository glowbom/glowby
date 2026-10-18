package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDesktopProjectReadCommands(t *testing.T) {
	for _, test := range []struct {
		args      []string
		operation string
		payload   map[string]any
	}{
		{[]string{"project", "current", "--json"}, "project.current", map[string]any{}},
		{[]string{"models", "list", "--json"}, "models.list", map[string]any{}},
		{[]string{"runs", "show", "run-123", "--json"}, "runs.show", map[string]any{"id": "run-123"}},
	} {
		t.Run(test.operation, func(t *testing.T) {
			invocation, err := parseDesktopCommand(test.args)
			if err != nil || invocation.command.Version != 1 || invocation.command.Operation != test.operation ||
				!reflect.DeepEqual(invocation.command.Payload, test.payload) || !invocation.compact || invocation.command.Actor != "" || invocation.command.RequestID != "" {
				t.Fatalf("unexpected read command: %#v, %v", invocation, err)
			}
		})
	}
}

func TestDesktopProjectMutationsPreserveSelectionAndAttribution(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Project with spaces")
	for _, test := range []struct{ group, action, textFlag string }{
		{"chat", "send", "message"},
		{"build", "start", "instructions"},
	} {
		t.Run(test.group, func(t *testing.T) {
			args := []string{test.group, test.action, "--" + test.textFlag, "First line\nSecond line", "--project", project,
				"--context", "desktop:chat/context-1", "--model=provider/model", "--actor", "@jessica", "--request-id", "same-request", "--json"}
			invocation, err := parseDesktopCommand(args)
			expected := desktopCommand{Version: 1, Operation: test.group + "." + test.action, Actor: "@jessica", RequestID: "same-request",
				Payload: map[string]any{"project": project, "contextId": "desktop:chat/context-1", "model": "provider/model", test.textFlag: "First line\nSecond line"}}
			if err != nil || !reflect.DeepEqual(invocation.command, expected) || !invocation.compact {
				t.Fatalf("selection, author, or request ID changed: %#v, %v", invocation, err)
			}
			retry, err := parseDesktopCommand(args)
			if err != nil || !reflect.DeepEqual(retry.command, expected) {
				t.Fatalf("explicit retry changed: %#v, %v", retry, err)
			}
			defaults, err := parseDesktopCommand([]string{test.group, test.action, "--" + test.textFlag, "Use current selection"})
			if err != nil || !reflect.DeepEqual(defaults.command.Payload, map[string]any{test.textFlag: "Use current selection"}) ||
				defaults.command.Actor != "" || len(defaults.command.RequestID) != 32 || !validDesktopIdentifier(defaults.command.RequestID) {
				t.Fatalf("defaults or generated retry ID changed: %#v, %v", defaults, err)
			}
		})
	}
}

func TestDesktopProjectTextFilesSendContents(t *testing.T) {
	file := filepath.Join(t.TempDir(), "request.txt")
	text := "Review the café app.\nKeep the current layout.\n"
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ group, action, field string }{
		{"chat", "send", "message"}, {"build", "start", "instructions"},
	} {
		invocation, err := parseDesktopCommand([]string{test.group, test.action, "--" + test.field + "-file", file})
		if err != nil || invocation.command.Payload[test.field] != text || len(invocation.command.Payload) != 1 {
			t.Fatalf("file contents not preserved: %#v, %v", invocation, err)
		}
	}
}

func TestDesktopProjectInvalidArgumentsDoNotConnect(t *testing.T) {
	t.Setenv(desktopConnectionEnv, "/missing/connection.json")
	for _, args := range [][]string{
		{"project", "open"},
		{"project", "current", "extra"},
		{"models", "list", "--model", "provider/model"},
		{"runs", "show"},
		{"runs", "show", "bad/id"},
		{"runs", "show", "run-1", "run-2"},
		{"chat", "send"},
		{"build", "start"},
		{"chat", "send", "--instructions", "Build"},
		{"build", "start", "--message", "Hello"},
		{"chat", "send", "--message", "Hello", "--file", "ignored.txt"},
		{"build", "start", "--instructions", "Build", "--file", "ignored.txt"},
		{"chat", "send", "--message", "Hello", "--project", "relative/project"},
		{"chat", "send", "--message", "Hello", "--context", "context\nchanged"},
		{"chat", "send", "--message", "Hello", "--context", strings.Repeat("x", 129)},
		{"chat", "send", "--message", "Hello", "--model"},
		{"chat", "send", "--message", "Hello", "--model", "one", "--model", "two"},
		{"chat", "send", "--message", "Hello", "--request-id", "invalid/id"},
		{"chat", "send", "--message", "Hello", "--message-file", "ignored.txt"},
		{"build", "start", "--instructions", "Build", "--instructions-file", "ignored.txt"},
		{"chat", "send", "--message", " "},
		{"chat", "send", "--message", "Hello", "--json=true"},
		{"build", "start", "--instructions", "Build", "--unknown", "value"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runDesktopWithIO(context.Background(), append(args, "--json"), &out, &errOut)
			var response desktopResponse
			if code != 2 || json.Unmarshal(out.Bytes(), &response) != nil || response.OK ||
				response.Error == nil || response.Error.Code != "invalid_arguments" || errOut.Len() != 0 {
				t.Fatalf("invalid command reached discovery or lost JSON error: %d %s %s", code, out.String(), errOut.String())
			}
		})
	}
}

func TestDesktopProjectCommandsUseExistingTransport(t *testing.T) {
	for _, test := range []struct {
		args      []string
		operation string
		payload   map[string]any
		mutation  bool
	}{
		{[]string{"project", "current"}, "project.current", map[string]any{}, false},
		{[]string{"models", "list"}, "models.list", map[string]any{}, false},
		{[]string{"chat", "send", "--message", "Review", "--actor", "jessica", "--request-id", "retry-1"}, "chat.send", map[string]any{"message": "Review"}, true},
		{[]string{"build", "start", "--instructions", "Build", "--actor", "jessica", "--request-id", "retry-1"}, "build.start", map[string]any{"instructions": "Build"}, true},
		{[]string{"runs", "show", "run-1"}, "runs.show", map[string]any{"id": "run-1"}, false},
	} {
		t.Run(test.operation, func(t *testing.T) {
			calls := 0
			desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				var command desktopCommand
				if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
					t.Fatal(err)
				}
				if command.Operation != test.operation || !reflect.DeepEqual(command.Payload, test.payload) ||
					test.mutation && (command.Actor != "jessica" || command.RequestID != "retry-1") {
					t.Errorf("unexpected wire command: %#v", command)
				}
				fmt.Fprint(w, `{"version":1,"ok":true,"result":{"run":{"id":"run-1","status":"queued"}}}`)
			})
			var out, errOut bytes.Buffer
			code := runDesktopWithIO(context.Background(), append(test.args, "--json"), &out, &errOut)
			var response desktopResponse
			if code != 0 || calls != 1 || json.Unmarshal(out.Bytes(), &response) != nil || !response.OK ||
				!strings.Contains(string(response.Result), `"id":"run-1"`) || errOut.Len() != 0 || strings.Count(out.String(), "\n") != 1 ||
				test.mutation && response.RequestID != "retry-1" {
				t.Fatalf("lost Desktop response or retry ID: %d %s %s", code, out.String(), errOut.String())
			}
		})
	}
}

func TestDesktopProjectHelpDoesNotNeedDesktop(t *testing.T) {
	t.Setenv(desktopConnectionEnv, "/missing/connection.json")
	var out, errOut bytes.Buffer
	if code := runDesktopWithIO(context.Background(), []string{"chat", "send", "--help"}, &out, &errOut); code != 0 {
		t.Fatalf("help requires Desktop: %d %s", code, errOut.String())
	}
	for _, text := range []string{"project current", "models list", "chat send", "build start", "runs show", "tasks submit", "--context", "does not open Desktop", "polling schedules remain external"} {
		if !strings.Contains(out.String(), text) {
			t.Errorf("help omitted %q", text)
		}
	}
}

func TestDesktopProjectOlderServerDiagnostic(t *testing.T) {
	for _, test := range []struct {
		name       string
		args       []string
		serverCode string
		wantCode   string
	}{
		{"project", []string{"project", "current"}, "request_id_required", "desktop_update_required"},
		{"models", []string{"models", "list"}, "request_id_required", "desktop_update_required"},
		{"run", []string{"runs", "show", "run-1"}, "request_id_required", "desktop_update_required"},
		{"chat error unchanged", []string{"chat", "send", "--message", "Review", "--request-id", "retry-1"}, "request_id_required", "request_id_required"},
		{"build error unchanged", []string{"build", "start", "--instructions", "Build", "--request-id", "retry-1"}, "request_id_required", "request_id_required"},
		{"task read error unchanged", []string{"tasks", "list"}, "request_id_required", "request_id_required"},
		{"other project error unchanged", []string{"project", "current"}, "ui_unavailable", "ui_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			desktopTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"version":1,"ok":false,"error":{"code":%q,"message":"Original server error"}}`, test.serverCode)
			})
			var out, errOut bytes.Buffer
			code := runDesktopWithIO(context.Background(), append(test.args, "--json"), &out, &errOut)
			var response desktopResponse
			if code != 1 || calls != 1 || json.Unmarshal(out.Bytes(), &response) != nil || response.OK ||
				response.Error == nil || response.Error.Code != test.wantCode || errOut.Len() != 0 {
				t.Fatalf("unexpected compatibility response: %d %s %s", code, out.String(), errOut.String())
			}
			if test.wantCode == "desktop_update_required" {
				if !strings.Contains(response.Error.Message, "does not support project commands") ||
					!strings.Contains(response.Error.Message, desktopConnectionEnv) {
					t.Fatalf("missing recovery instructions: %s", response.Error.Message)
				}
			} else if response.Error.Message != "Original server error" {
				t.Fatalf("unrelated server error changed: %s", response.Error.Message)
			}
			if (test.args[0] == "chat" || test.args[0] == "build") && response.RequestID != "retry-1" {
				t.Fatalf("mutation lost retry identity: %s", out.String())
			}
		})
	}
}
