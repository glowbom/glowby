package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type controlTestReply struct {
	Version int             `json:"version"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result"`
	Error   struct {
		Code string `json:"code"`
	} `json:"error"`
}

func controlTestCall(t *testing.T, handler http.HandlerFunc, value any) (*httptest.ResponseRecorder, controlTestReply) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/desktop/control/test", bytes.NewReader(data))
	r.Header.Set("Authorization", "Bearer control-test")
	w := httptest.NewRecorder()
	handler(w, r)
	var reply controlTestReply
	if json.Unmarshal(w.Body.Bytes(), &reply) != nil || reply.Version != 1 {
		t.Fatal("invalid response envelope", w.Code, w.Body.String())
	}
	return w, reply
}

func controlTestRun(t *testing.T, reply controlTestReply) *desktopControlRun {
	t.Helper()
	var result struct {
		Run *desktopControlRun `json:"run"`
	}
	if !reply.OK || json.Unmarshal(reply.Result, &result) != nil {
		t.Fatalf("invalid run response: %+v", reply)
	}
	return result.Run
}

func controlTestFixture(t *testing.T) (*desktopControlService, desktopControlSync, *time.Time) {
	t.Helper()
	t.Setenv("GLOWBOM_SERVER_TOKEN", "control-test")
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s := newDesktopControlService()
	s.now = func() time.Time { return now }
	request := desktopControlSync{ClientID: "window-one", Context: desktopControlContext{
		ID: "conversation:one", Active: true,
		Project:   &desktopControlProject{ID: "/projects/test", Path: "/projects/test", Name: "Test"},
		Models:    []desktopControlModel{{ID: "chat/one", Name: "Talk", Provider: "test", Chat: true}, {ID: "build/one", Name: "Build", Provider: "test", Build: true}},
		ChatModel: "chat/one", BuildModel: "build/one",
	}}
	_, reply := controlTestCall(t, s.syncHandler, request)
	if !reply.OK {
		t.Fatal(reply.Error.Code)
	}
	return s, request, &now
}

func controlTestCommand(kind, requestID string) desktopControlCommand {
	request := desktopControlCommand{Version: 1, Operation: kind, RequestID: requestID}
	request.Payload.Message = "Explain the project.\nKeep it brief."
	if kind == "build.start" {
		request.Payload.Message = ""
		request.Payload.Instructions = "Update the title."
	}
	return request
}

func TestDesktopControlCurrentModelsAndNoProject(t *testing.T) {
	s, request, _ := controlTestFixture(t)
	_, reply := controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "project.current"})
	if !reply.OK || !bytes.Contains(reply.Result, []byte(`"contextId":"conversation:one"`)) || !bytes.Contains(reply.Result, []byte(`"path":"/projects/test"`)) {
		t.Fatal(string(reply.Result))
	}
	_, reply = controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "models.list"})
	if !reply.OK || !bytes.Contains(reply.Result, []byte(`"chat":true`)) || !bytes.Contains(reply.Result, []byte(`"buildModel":"build/one"`)) {
		t.Fatal(string(reply.Result))
	}
	request.Context.Project = nil
	controlTestCall(t, s.syncHandler, request)
	w, reply := controlTestCall(t, s.commandHandler, controlTestCommand("build.start", "build-one"))
	if w.Code != 409 || reply.Error.Code != "project_required" {
		t.Fatal(w.Code, reply.Error.Code)
	}
	_, reply = controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "chat-one"))
	if run := controlTestRun(t, reply); run == nil || run.ProjectPath != "" || run.Model != "chat/one" {
		t.Fatal(run)
	}
}

func TestDesktopControlFollowsFocusedProjectInsteadOfFirstEmptyTab(t *testing.T) {
	s, first, now := controlTestFixture(t)
	first.Context.Project = nil
	first.Context.Focused = true
	first.Context.FocusedAt = now.Add(-time.Second).UnixMilli()
	controlTestCall(t, s.syncHandler, first)
	_, reply := controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "project.current"})
	if !reply.OK || !bytes.Contains(reply.Result, []byte(`"project":null`)) {
		t.Fatal(string(reply.Result))
	}
	project := first
	project.ClientID = "project-window"
	project.Context.ID = "project-conversation"
	project.Context.Project = &desktopControlProject{ID: "/projects/selected", Path: "/projects/selected", Name: "Selected"}
	project.Context.FocusedAt = now.UnixMilli()
	controlTestCall(t, s.syncHandler, project)
	_, reply = controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "project.current"})
	if !reply.OK || !bytes.Contains(reply.Result, []byte(`"path":"/projects/selected"`)) || !bytes.Contains(reply.Result, []byte(`"contextId":"project-conversation"`)) {
		t.Fatal(string(reply.Result))
	}
	for range 3 {
		controlTestCall(t, s.syncHandler, first)
		if s.client.id != project.ClientID {
			t.Fatal("a stale focused heartbeat replaced the most recent project")
		}
		controlTestCall(t, s.syncHandler, project)
	}
	// The user can leave Glowbom to ask an agent to work on the selected project.
	project.Context.Focused = false
	controlTestCall(t, s.syncHandler, project)
	first.Context.Focused = false
	controlTestCall(t, s.syncHandler, first)
	if s.client.id != project.ClientID || s.client.context.Project == nil {
		t.Fatal("an unfocused empty tab replaced the selected project")
	}
	_, reply = controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "models.list"})
	if !reply.OK {
		t.Fatal("focus loss made the selected project unavailable", reply.Error.Code)
	}
	first.Context.Active, first.Context.Focused = false, true
	first.Context.FocusedAt = now.Add(time.Second).UnixMilli()
	controlTestCall(t, s.syncHandler, first)
	if s.client.id != project.ClientID {
		t.Fatal("a focused window outside Chat replaced the selected project")
	}
	first.Context.Active = true
	controlTestCall(t, s.syncHandler, first)
	if s.client.id != first.ClientID {
		t.Fatal("a later interaction did not select its Chat window")
	}
	first.Context.Active = false
	controlTestCall(t, s.syncHandler, first)
	controlTestCall(t, s.syncHandler, project)
	if s.client.id != first.ClientID {
		t.Fatal("an older background tab replaced an inactive recent window")
	}
	_, reply = controlTestCall(t, s.commandHandler, desktopControlCommand{Version: 1, Operation: "project.current"})
	if reply.Error.Code != "ui_unavailable" {
		t.Fatal("inactive current Chat should be unavailable", reply.Error.Code)
	}
}

func TestDesktopControlFocusedWindowCannotMovePendingWork(t *testing.T) {
	for _, status := range []string{"queued", "running", "needs_review"} {
		t.Run(status, func(t *testing.T) {
			s, owner, now := controlTestFixture(t)
			_, reply := controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "pending-one"))
			run := controlTestRun(t, reply)
			if status != "queued" {
				controlTestCall(t, s.syncHandler, owner)
				owner.Update = &desktopControlUpdate{ID: run.ID, Status: status}
				controlTestCall(t, s.syncHandler, owner)
			}
			other := owner
			other.ClientID, other.Context.ID = "focused-window", "different-conversation"
			other.Context.Focused = true
			other.Context.FocusedAt = now.UnixMilli()
			other.Update = nil
			_, reply = controlTestCall(t, s.syncHandler, other)
			if controlTestRun(t, reply) != nil || s.client.id != owner.ClientID || s.runs[run.ID].Status != status {
				t.Fatal("focus moved or changed pending work")
			}
			if status == "queued" {
				controlTestCall(t, s.syncHandler, owner)
			}
			owner.Update = &desktopControlUpdate{ID: run.ID, Status: "completed", Text: "Done"}
			controlTestCall(t, s.syncHandler, owner)
			other.Context.Focused = false
			controlTestCall(t, s.syncHandler, other)
			if s.client.id != other.ClientID || s.runs[run.ID].Status != "completed" {
				t.Fatal("focused window could not become current after work completed")
			}
		})
	}
}

func TestDesktopControlRejectsInvalidFocusTimesAndAcceptsLegacyContexts(t *testing.T) {
	s, first, now := controlTestFixture(t)
	other := first
	other.ClientID = "other-window"
	for _, stamp := range []int64{-1, now.Add(2 * time.Minute).UnixMilli()} {
		other.Context.FocusedAt = stamp
		_, reply := controlTestCall(t, s.syncHandler, other)
		if reply.Error.Code != "invalid_context" || s.client.id != first.ClientID {
			t.Fatal("invalid focus time changed the current window", stamp, reply.Error.Code)
		}
	}
	other.Context.FocusedAt, other.Context.Focused = 0, true
	_, reply := controlTestCall(t, s.syncHandler, other)
	if !reply.OK || s.client.id != other.ClientID {
		t.Fatal("legacy focus contexts stopped working")
	}
	other.Context.FocusedAt = now.UnixMilli()
	controlTestCall(t, s.syncHandler, other)
	first.Context.Focused = true
	controlTestCall(t, s.syncHandler, first)
	if s.client.id != other.ClientID {
		t.Fatal("legacy focus replaced a window with a recorded interaction")
	}
}

func TestDesktopControlPinsContextAndChecksBusyAndCapability(t *testing.T) {
	for _, test := range []struct {
		name, want string
		change     func(*desktopControlCommand, *desktopControlSync)
	}{
		{"conversation changed", "context_changed", func(c *desktopControlCommand, _ *desktopControlSync) { c.Payload.ContextID = "older" }},
		{"project changed", "context_changed", func(c *desktopControlCommand, _ *desktopControlSync) { c.Payload.Project = "/projects/other" }},
		{"UI busy", "busy", func(_ *desktopControlCommand, u *desktopControlSync) { u.Context.Busy = true }},
		{"chat capability", "model_unavailable", func(c *desktopControlCommand, _ *desktopControlSync) { c.Payload.Model = "build/one" }},
		{"unavailable model", "model_unavailable", func(c *desktopControlCommand, _ *desktopControlSync) { c.Payload.Model = "missing" }},
		{"no request identity", "request_id_required", func(c *desktopControlCommand, _ *desktopControlSync) { c.RequestID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, ui, _ := controlTestFixture(t)
			command := controlTestCommand("chat.send", "chat-one")
			test.change(&command, &ui)
			controlTestCall(t, s.syncHandler, ui)
			_, reply := controlTestCall(t, s.commandHandler, command)
			if reply.OK || reply.Error.Code != test.want || len(s.runs) != 0 {
				t.Fatal(reply.Error.Code, len(s.runs))
			}
		})
	}
	s, ui, _ := controlTestFixture(t)
	command := controlTestCommand("build.start", "build-one")
	command.Payload.Project, command.Payload.ContextID, command.Payload.Model = ui.Context.Project.Path, ui.Context.ID, "chat/one"
	_, reply := controlTestCall(t, s.commandHandler, command)
	if reply.Error.Code != "model_unavailable" {
		t.Fatal(reply.Error.Code)
	}
	command.Payload.Model = "build/one"
	_, reply = controlTestCall(t, s.commandHandler, command)
	if run := controlTestRun(t, reply); run == nil || run.Kind != "build" || run.ProjectPath != ui.Context.Project.Path || run.ContextID != ui.Context.ID {
		t.Fatal(run)
	}
	_, reply = controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "other"))
	if reply.Error.Code != "busy" {
		t.Fatal(reply.Error.Code)
	}
}

func TestDesktopControlClaimsOnceAndPreservesCompletion(t *testing.T) {
	s, ui, now := controlTestFixture(t)
	command := controlTestCommand("chat.send", "one")
	command.Actor = &chatActor{ID: "agent-id", Username: "jessica", Name: "Jessica", Provider: "ChatGPT Dot"}
	_, reply := controlTestCall(t, s.commandHandler, command)
	queued := controlTestRun(t, reply)
	if queued == nil || queued.Status != "queued" || !reflect.DeepEqual(queued.Actor, command.Actor) {
		t.Fatal(queued)
	}
	other := ui
	other.ClientID = "window-two"
	_, reply = controlTestCall(t, s.syncHandler, other)
	if controlTestRun(t, reply) != nil {
		t.Fatal("another window claimed work")
	}
	_, reply = controlTestCall(t, s.syncHandler, ui)
	claimed := controlTestRun(t, reply)
	if claimed == nil || claimed.Status != "running" || claimed.ID != queued.ID {
		t.Fatal(claimed)
	}
	_, reply = controlTestCall(t, s.syncHandler, ui)
	if controlTestRun(t, reply) != nil {
		t.Fatal("claim was replayed")
	}
	other.Update = &desktopControlUpdate{ID: claimed.ID, Status: "completed", Text: "Wrong window"}
	_, reply = controlTestCall(t, s.syncHandler, other)
	if reply.Error.Code != "run_owner_mismatch" {
		t.Fatal(reply.Error.Code)
	}
	ui.Context.Busy = true
	ui.Update = &desktopControlUpdate{ID: claimed.ID, Status: "needs_review", Message: "Review the existing permission prompt.", NativeRunID: "native-one"}
	controlTestCall(t, s.syncHandler, ui)
	*now = now.Add(5 * time.Second)
	ui.Update = &desktopControlUpdate{ID: claimed.ID, Status: "completed", Text: "The answer.\nWith details.", Model: "chat/one"}
	controlTestCall(t, s.syncHandler, ui)
	_, reply = controlTestCall(t, s.commandHandler, command)
	completed := controlTestRun(t, reply)
	if completed.ID != claimed.ID || completed.Status != "completed" || completed.Result.Text != ui.Update.Text || completed.Result.Model != "chat/one" || completed.Result.NativeRunID != "native-one" || !completed.UpdatedAt.After(completed.CreatedAt) {
		t.Fatal(completed)
	}
	ui.Update = &desktopControlUpdate{ID: claimed.ID, Status: "running", Text: "Late response"}
	controlTestCall(t, s.syncHandler, ui)
	if s.runs[claimed.ID].Status != "completed" || s.runs[claimed.ID].Result.Text != completed.Result.Text {
		t.Fatal("terminal result changed")
	}
	command.Payload.Message = "Different request"
	_, reply = controlTestCall(t, s.commandHandler, command)
	if reply.Error.Code != "request_conflict" || len(s.runs) != 1 {
		t.Fatal(reply.Error.Code)
	}
}

func TestDesktopControlNeverReplaysAfterLeaseOrContextLoss(t *testing.T) {
	for _, mode := range []string{"stale queued", "stale running", "lost claim response", "changed project", "changed context", "new human work", "left Chat", "model disconnected"} {
		t.Run(mode, func(t *testing.T) {
			s, ui, now := controlTestFixture(t)
			command := controlTestCommand("chat.send", "one")
			_, reply := controlTestCall(t, s.commandHandler, command)
			run := controlTestRun(t, reply)
			if mode == "stale running" || mode == "lost claim response" {
				controlTestCall(t, s.syncHandler, ui)
				if mode == "stale running" {
					ui.Update = &desktopControlUpdate{ID: run.ID, Status: "running"}
					controlTestCall(t, s.syncHandler, ui)
					ui.Update = nil
				} else {
					*now = now.Add(10 * time.Second)
					controlTestCall(t, s.syncHandler, ui)
				}
			}
			switch mode {
			case "stale queued", "stale running":
				*now = now.Add(desktopControlLease)
			case "lost claim response":
				*now = now.Add(11 * time.Second)
			case "changed project":
				ui.Context.Project = &desktopControlProject{ID: "other", Path: "/projects/other", Name: "Other"}
			case "changed context":
				ui.Context.ID = "new conversation"
			case "new human work":
				ui.Context.Busy = true
			case "left Chat":
				ui.Context.Active = false
			case "model disconnected":
				ui.Context.Models = nil
			}
			_, reply = controlTestCall(t, s.syncHandler, ui)
			if controlTestRun(t, reply) != nil {
				t.Fatal("unsafe claim")
			}
			_, reply = controlTestCall(t, s.commandHandler, command)
			failed := controlTestRun(t, reply)
			if failed.ID != run.ID || failed.Status != "failed" || failed.Message == "" {
				t.Fatal(failed)
			}
			ui.Update = &desktopControlUpdate{ID: run.ID, Status: "completed", Text: "Late completion"}
			controlTestCall(t, s.syncHandler, ui)
			if s.runs[run.ID].Status != "failed" {
				t.Fatal("stale job was revived")
			}
		})
	}
}

func TestDesktopControlCapacityKeepsRetryReceipts(t *testing.T) {
	s, ui, _ := controlTestFixture(t)
	var first *desktopControlRun
	for index := 0; index < desktopControlRunLimit; index++ {
		_, reply := controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", fmt.Sprintf("request-%d", index)))
		run := controlTestRun(t, reply)
		if index == 0 {
			first = run
		}
		ui.Update = nil
		controlTestCall(t, s.syncHandler, ui)
		ui.Update = &desktopControlUpdate{ID: run.ID, Status: "completed", Text: "Done"}
		controlTestCall(t, s.syncHandler, ui)
	}
	w, reply := controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "overflow"))
	if w.Code != 429 || reply.Error.Code != "session_capacity" || len(s.runs) != desktopControlRunLimit {
		t.Fatal(w.Code, reply.Error.Code, len(s.runs))
	}
	_, reply = controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "request-0"))
	if run := controlTestRun(t, reply); run.ID != first.ID || run.Status != "completed" {
		t.Fatal(run)
	}
}

func TestDesktopControlAuthenticationAndValidation(t *testing.T) {
	s, ui, now := controlTestFixture(t)
	for _, token := range []string{"", "wrong"} {
		r := httptest.NewRequest(http.MethodPost, "/desktop/control/command", strings.NewReader(`{"operation":"project.current"}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.commandHandler(w, r)
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	t.Setenv("GLOWBOM_ALLOWED_ORIGINS", "http://127.0.0.1:4587")
	r := httptest.NewRequest(http.MethodPost, "/desktop/control/command", strings.NewReader(`{"operation":"project.current"}`))
	r.Header.Set("Authorization", "Bearer control-test")
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	withGlowbomSecurity(http.HandlerFunc(s.commandHandler)).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	for _, change := range []func(*desktopControlSync){
		func(u *desktopControlSync) { u.ClientID = strings.Repeat("x", 129) },
		func(u *desktopControlSync) { u.Context.ID = "bad\ncontext" },
		func(u *desktopControlSync) {
			u.Context.Project = &desktopControlProject{ID: "bad", Path: "relative", Name: "Invalid"}
		},
		func(u *desktopControlSync) {
			u.Context.Models = append(append([]desktopControlModel{}, u.Context.Models...), u.Context.Models[0])
		},
	} {
		invalid := ui
		change(&invalid)
		w, reply := controlTestCall(t, s.syncHandler, invalid)
		if w.Code != 400 || reply.Error.Code != "invalid_context" {
			t.Fatal(w.Code, reply.Error.Code)
		}
	}
	*now = now.Add(desktopControlLease)
	w, reply := controlTestCall(t, s.commandHandler, controlTestCommand("chat.send", "one"))
	if w.Code != 503 || reply.Error.Code != "ui_unavailable" {
		t.Fatal(w.Code, reply.Error.Code)
	}
	query := desktopControlCommand{Version: 1, Operation: "runs.show"}
	query.Payload.ID = "does-not-exist"
	w, reply = controlTestCall(t, s.commandHandler, query)
	if w.Code != 404 || reply.Error.Code != "run_not_found" {
		t.Fatal(w.Code, reply.Error.Code)
	}
}

func TestDesktopControlProxyEnvelopeAndRejectedWireRequests(t *testing.T) {
	s, _, _ := controlTestFixture(t)
	for _, test := range []struct {
		body string
		want int
	}{
		{`{"version":1,"operation":"chat.send","requestId":"proxy-one","actor":{"id":"agent-id","username":"jessica","name":"Jessica","provider":"ChatGPT Dot"},"payload":{"message":"Hello","contextId":"conversation:one","project":"/projects/test"}}`, 200},
		{`{"operation":"project.current","payload":{}}`, 400},
		{`{"version":2,"operation":"project.current","payload":{}}`, 400},
		{`{"version":1,"operation":"project.current","payload":{"file":"/private/file"}}`, 400},
		{`{"version":1,"operation":"project.current","payload":{}} {}`, 400},
	} {
		r := httptest.NewRequest(http.MethodPost, "/desktop/control/command", strings.NewReader(test.body))
		r.Header.Set("Authorization", "Bearer control-test")
		w := httptest.NewRecorder()
		s.commandHandler(w, r)
		if w.Code != test.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if len(s.runs) != 1 {
		t.Fatal("invalid wire requests changed queued work")
	}
}

func TestDesktopControlAcknowledgedBuildKeepsLeaseWithoutReclaim(t *testing.T) {
	s, ui, now := controlTestFixture(t)
	_, reply := controlTestCall(t, s.commandHandler, controlTestCommand("build.start", "build-one"))
	run := controlTestRun(t, reply)
	controlTestCall(t, s.syncHandler, ui)
	ui.Context.Busy = true
	ui.Update = &desktopControlUpdate{ID: run.ID, Status: "running", NativeRunID: "build-job"}
	controlTestCall(t, s.syncHandler, ui)
	ui.Update = nil
	for index := 0; index < 4; index++ {
		*now = now.Add(15 * time.Second)
		_, reply = controlTestCall(t, s.syncHandler, ui)
		if controlTestRun(t, reply) != nil || s.runs[run.ID].Status != "running" {
			t.Fatal("healthy build lease failed or job replayed")
		}
	}
	query := desktopControlCommand{Version: 1, Operation: "runs.show"}
	query.Payload.ID = run.ID
	_, reply = controlTestCall(t, s.commandHandler, query)
	if observed := controlTestRun(t, reply); observed.Status != "running" || observed.Result.NativeRunID != "build-job" {
		t.Fatal(observed)
	}
}

func TestChatActorHistoryPersistenceAndProviderRoles(t *testing.T) {
	project := translationProject(t)
	actor := &chatActor{ID: "stable-agent-id", Username: "herakles", Name: "Herakles", Provider: "Grok Bot"}
	messages := []chatMessage{{Role: "user", Text: "Please review this.", Actor: actor}, {Role: "assistant", Text: "Reviewed.", Model: "test/model"}}
	if response := chatHistoryTestPost(t, project, messages); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	w := httptest.NewRecorder()
	(&chatService{}).historyHandler(w, httptest.NewRequest(http.MethodGet, "/chat/history?path="+url.QueryEscape(project), nil))
	var saved struct {
		Messages []chatMessage `json:"messages"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &saved) != nil || !reflect.DeepEqual(saved.Messages, messages) {
		t.Fatal(w.Code, w.Body.String())
	}
	// A companion append and retry must retain attribution on earlier messages.
	if _, err := beginCompanionProjectChat(project, "Continue"); err != nil {
		t.Fatal(err)
	}
	endCompanionProjectChat(project)
	if _, err := beginCompanionProjectChat(project, "Continue"); err != nil {
		t.Fatal(err)
	}
	defer endCompanionProjectChat(project)
	history, err := readSharedChatHistory(project)
	if err != nil || len(history) != 3 || !reflect.DeepEqual(history[0].Actor, actor) {
		t.Fatal(history, err)
	}
	for _, message := range recentChatModelMessages(history) {
		if message.Actor != nil || message.Role != "user" && message.Role != "assistant" {
			t.Fatal("attribution changed model roles or leaked into model metadata")
		}
	}
}

func TestChatActorRejectsInvalidMetadataWithoutChangingSavedHistory(t *testing.T) {
	project := translationProject(t)
	actor := &chatActor{ID: "id", Username: "jessica", Name: "Jessica"}
	baseline := []chatMessage{{Role: "user", Text: "Keep this", Actor: actor}}
	if w := chatHistoryTestPost(t, project, baseline); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, test := range []struct {
		name, role string
		actor      *chatActor
	}{
		{"assistant", "assistant", actor},
		{"missing id", "user", &chatActor{Username: "name", Name: "Name"}},
		{"long name", "user", &chatActor{ID: "id", Username: "name", Name: strings.Repeat("x", 161)}},
		{"control characters", "user", &chatActor{ID: "id", Username: "name\nother", Name: "Name"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := chatMessage{Role: test.role, Text: "Invalid", Actor: test.actor}
			if w := chatHistoryTestPost(t, project, []chatMessage{bad}); w.Code != 400 {
				t.Fatal(w.Code, w.Body.String())
			}
			if err := validateChatRequest(chatRequest{Mode: "chat", Model: "test/model", Messages: []chatMessage{bad, {Role: "user", Text: "Continue"}}}); err == nil {
				t.Fatal("invalid attribution reached the model")
			}
			current, err := readSharedChatHistory(project)
			if err != nil || !reflect.DeepEqual(current, baseline) {
				t.Fatal("failed save replaced history", err)
			}
		})
	}
}
