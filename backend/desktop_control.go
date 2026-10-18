package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const desktopControlLease = 20 * time.Second
const desktopControlRunLimit = 128

type desktopControlModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Chat     bool   `json:"chat"`
	Build    bool   `json:"build"`
}

type desktopControlProject struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
}

type desktopControlContext struct {
	ID         string                 `json:"id"`
	Project    *desktopControlProject `json:"project"`
	Models     []desktopControlModel  `json:"models"`
	ChatModel  string                 `json:"chatModel"`
	BuildModel string                 `json:"buildModel"`
	Busy       bool                   `json:"busy"`
	Active     bool                   `json:"active"`
	Focused    bool                   `json:"focused"`
	FocusedAt  int64                  `json:"focusedAt"`
}

type desktopControlResult struct {
	Text        string `json:"text,omitempty"`
	Model       string `json:"model,omitempty"`
	NativeRunID string `json:"nativeRunId,omitempty"`
}

type desktopControlRun struct {
	ID           string                `json:"id"`
	Kind         string                `json:"kind"`
	RequestID    string                `json:"requestId"`
	ProjectPath  string                `json:"projectPath"`
	ContextID    string                `json:"contextId"`
	Model        string                `json:"model"`
	Text         string                `json:"text"`
	Actor        *chatActor            `json:"actor,omitempty"`
	Status       string                `json:"status"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	Message      string                `json:"message,omitempty"`
	Result       *desktopControlResult `json:"result,omitempty"`
	clientID     string
	fingerprint  string
	acknowledged bool
}

type desktopControlCommand struct {
	Version   int        `json:"version"`
	Operation string     `json:"operation"`
	RequestID string     `json:"requestId,omitempty"`
	Actor     *chatActor `json:"actor,omitempty"`
	Payload   struct {
		Project      string `json:"project,omitempty"`
		ContextID    string `json:"contextId,omitempty"`
		Model        string `json:"model,omitempty"`
		Message      string `json:"message,omitempty"`
		Instructions string `json:"instructions,omitempty"`
		ID           string `json:"id,omitempty"`
	} `json:"payload"`
}

type desktopControlUpdate struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	Text        string `json:"text,omitempty"`
	Model       string `json:"model,omitempty"`
	NativeRunID string `json:"nativeRunId,omitempty"`
}

type desktopControlSync struct {
	ClientID string                `json:"clientId"`
	Context  desktopControlContext `json:"context"`
	Update   *desktopControlUpdate `json:"update,omitempty"`
}

type desktopControlClient struct {
	id      string
	context desktopControlContext
	seen    time.Time
}

// Commands are handed once to the existing Desktop UI. The backend never starts
// an agent or retries a job whose UI may already have accepted it.
type desktopControlService struct {
	mu       sync.Mutex
	now      func() time.Time
	client   *desktopControlClient
	runs     map[string]*desktopControlRun
	requests map[string]string
}

func newDesktopControlService() *desktopControlService {
	return &desktopControlService{now: time.Now, runs: map[string]*desktopControlRun{}, requests: map[string]string{}}
}

func desktopControlID() string {
	var id [16]byte
	_, _ = rand.Read(id[:])
	id[6], id[8] = id[6]&0x0f|0x40, id[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
}

func controlString(value string, limit int, required bool) bool {
	if !utf8.ValidString(value) || len(value) > limit || required && strings.TrimSpace(value) == "" {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func desktopControlWrite(w http.ResponseWriter, status int, result any, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	value := map[string]any{"version": 1, "ok": code == ""}
	if code == "" {
		value["result"] = result
	} else {
		value["error"] = map[string]string{"code": code, "message": message}
	}
	_ = json.NewEncoder(w).Encode(value)
}

func desktopControlRead(w http.ResponseWriter, r *http.Request, value any) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		desktopControlWrite(w, http.StatusMethodNotAllowed, nil, "method_not_allowed", "Use POST for Desktop control.")
		return false
	}
	// These routes remain authenticated even in an OSS server without a token.
	if token := glowbomServerToken(); token == "" || !hasValidGlowbomServerToken(r, token) {
		desktopControlWrite(w, http.StatusUnauthorized, nil, "unauthorized", "Local Desktop authentication is required.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err == nil {
		var extra any
		if decoder.Decode(&extra) == io.EOF && r.Context().Err() == nil {
			return true
		}
	}
	desktopControlWrite(w, http.StatusBadRequest, nil, "invalid_request", "Send one valid Desktop control request.")
	return false
}

func desktopControlTerminal(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}

func (s *desktopControlService) pending() *desktopControlRun {
	for _, run := range s.runs {
		if !desktopControlTerminal(run.Status) {
			return run
		}
	}
	return nil
}

func (s *desktopControlService) fail(run *desktopControlRun, message string, now time.Time) {
	run.Status, run.Message, run.UpdatedAt = "failed", message, now
}

func (s *desktopControlService) expire(now time.Time) {
	if run := s.pending(); run != nil && run.Status != "queued" && !run.acknowledged && now.Sub(run.UpdatedAt) >= desktopControlLease {
		s.fail(run, "Desktop did not confirm receiving this request. Check the project before sending another request. This request will not run again.", now)
	}
	if s.client != nil && now.Sub(s.client.seen) >= desktopControlLease {
		if run := s.pending(); run != nil {
			s.fail(run, "Desktop stopped reporting this request. Check the project before sending another request. This request will not run again.", now)
		}
		s.client = nil
	}
}

func desktopControlPath(context desktopControlContext) string {
	if context.Project != nil {
		return context.Project.Path
	}
	return ""
}

func desktopControlSupports(context desktopControlContext, model, kind string) bool {
	for _, candidate := range context.Models {
		if candidate.ID == model {
			return kind == "chat" && candidate.Chat || kind == "build" && candidate.Build
		}
	}
	return false
}

func (s *desktopControlService) commandHandler(w http.ResponseWriter, r *http.Request) {
	var request desktopControlCommand
	if !desktopControlRead(w, r, &request) {
		return
	}
	if request.Version != 1 {
		desktopControlWrite(w, 400, nil, "invalid_version", "Use Desktop control protocol version 1.")
		return
	}
	if !controlString(request.RequestID, 128, false) || !controlString(request.Payload.ID, 128, false) || !controlString(request.Payload.ContextID, 128, false) || !controlString(request.Payload.Project, 4096, false) || !controlString(request.Payload.Model, 256, false) || validateChatActor(request.Actor) != nil {
		desktopControlWrite(w, 400, nil, "invalid_request", "The request contains invalid identifiers or agent details.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	s.expire(now)
	if request.Operation == "runs.show" {
		run := s.runs[request.Payload.ID]
		if run == nil {
			desktopControlWrite(w, 404, nil, "run_not_found", "This request is not in the current Desktop session.")
			return
		}
		desktopControlWrite(w, 200, map[string]any{"run": run}, "", "")
		return
	}
	mutation := request.Operation == "chat.send" || request.Operation == "build.start"
	if !mutation && request.Operation != "project.current" && request.Operation != "models.list" {
		desktopControlWrite(w, 400, nil, "unknown_operation", "This Desktop operation is not supported.")
		return
	}
	encoded, _ := json.Marshal(request)
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	if mutation {
		if !controlString(request.RequestID, 128, true) {
			desktopControlWrite(w, 400, nil, "request_id_required", "Include a requestId and reuse it only when retrying this exact request.")
			return
		}
		if previous := s.runs[s.requests[request.RequestID]]; previous != nil {
			if previous.fingerprint != fingerprint {
				desktopControlWrite(w, 409, nil, "request_conflict", "This requestId belongs to a different request.")
			} else {
				desktopControlWrite(w, 200, map[string]any{"run": previous}, "", "")
			}
			return
		}
	}
	if s.client == nil || !s.client.context.Active {
		desktopControlWrite(w, 503, nil, "ui_unavailable", "Open Chat in Glowbom Desktop and try again.")
		return
	}
	current := s.client.context
	if request.Operation == "project.current" {
		desktopControlWrite(w, 200, map[string]any{"project": current.Project, "contextId": current.ID, "chatModel": current.ChatModel, "buildModel": current.BuildModel}, "", "")
		return
	}
	if request.Operation == "models.list" {
		models := current.Models
		if models == nil {
			models = []desktopControlModel{}
		}
		desktopControlWrite(w, 200, map[string]any{"models": models, "chatModel": current.ChatModel, "buildModel": current.BuildModel}, "", "")
		return
	}
	if request.Payload.ContextID != "" && request.Payload.ContextID != current.ID || request.Payload.Project != "" && (current.Project == nil || request.Payload.Project != current.Project.Path && request.Payload.Project != current.Project.ID) {
		desktopControlWrite(w, 409, nil, "context_changed", "The selected Desktop project or conversation changed. Read the current project before trying again.")
		return
	}
	if current.Busy || s.pending() != nil {
		desktopControlWrite(w, 409, nil, "busy", "Desktop is already handling a request. Wait for it to finish.")
		return
	}
	kind, text, model := "chat", request.Payload.Message, current.ChatModel
	if request.Operation == "build.start" {
		kind, text, model = "build", request.Payload.Instructions, current.BuildModel
		if current.Project == nil {
			desktopControlWrite(w, 409, nil, "project_required", "Select a project in Desktop before starting a Build.")
			return
		}
	}
	if strings.TrimSpace(text) == "" || !utf8.ValidString(text) || len(text) > maxChatContextBytes || strings.ContainsRune(text, 0) {
		desktopControlWrite(w, 400, nil, "invalid_text", "Include a message or Build instructions up to 600000 bytes.")
		return
	}
	if request.Payload.Model != "" {
		model = request.Payload.Model
	}
	if !desktopControlSupports(current, model, kind) {
		desktopControlWrite(w, 409, nil, "model_unavailable", "Choose a connected model that supports this action in Desktop.")
		return
	}
	// Do not discard receipts: a later retry must never create a second job.
	if len(s.runs) >= desktopControlRunLimit {
		desktopControlWrite(w, 429, nil, "session_capacity", "This Desktop session has reached its request limit. Finish pending work, then restart Desktop for new requests.")
		return
	}
	run := &desktopControlRun{ID: desktopControlID(), Kind: kind, RequestID: request.RequestID, ProjectPath: desktopControlPath(current), ContextID: current.ID, Model: model, Text: text, Actor: request.Actor, Status: "queued", CreatedAt: now, UpdatedAt: now, clientID: s.client.id, fingerprint: fingerprint}
	s.runs[run.ID], s.requests[request.RequestID] = run, run.ID
	desktopControlWrite(w, 200, map[string]any{"run": run}, "", "")
}

func validDesktopControlSync(request desktopControlSync) bool {
	context := request.Context
	if !controlString(request.ClientID, 128, true) || !controlString(context.ID, 128, true) || !controlString(context.ChatModel, 256, false) || !controlString(context.BuildModel, 256, false) || len(context.Models) > 2048 || context.FocusedAt < 0 {
		return false
	}
	if project := context.Project; project != nil && (!controlString(project.ID, 4096, true) || !controlString(project.Path, 4096, true) || !filepath.IsAbs(project.Path) || !controlString(project.Name, 256, true)) {
		return false
	}
	seen := map[string]bool{}
	for _, model := range context.Models {
		if !controlString(model.ID, 256, true) || !controlString(model.Name, 256, true) || !controlString(model.Provider, 160, false) || seen[model.ID] {
			return false
		}
		seen[model.ID] = true
	}
	if update := request.Update; update != nil {
		if !controlString(update.ID, 128, true) || !controlString(update.Message, 4096, false) || !controlString(update.Model, 256, false) || !controlString(update.NativeRunID, 256, false) || !utf8.ValidString(update.Text) || len(update.Text) > maxChatContextBytes || strings.ContainsRune(update.Text, 0) {
			return false
		}
		if update.Status != "running" && update.Status != "needs_review" && !desktopControlTerminal(update.Status) {
			return false
		}
	}
	return true
}

func (s *desktopControlService) syncHandler(w http.ResponseWriter, r *http.Request) {
	var request desktopControlSync
	if !desktopControlRead(w, r, &request) {
		return
	}
	if !validDesktopControlSync(request) {
		desktopControlWrite(w, 400, nil, "invalid_context", "Send a valid Desktop context and request update.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if request.Context.FocusedAt > now.Add(time.Minute).UnixMilli() {
		desktopControlWrite(w, 400, nil, "invalid_context", "Send a valid Desktop focus time.")
		return
	}
	s.expire(now)
	if update := request.Update; update != nil {
		run := s.runs[update.ID]
		if run == nil || run.clientID != request.ClientID {
			desktopControlWrite(w, 409, nil, "run_owner_mismatch", "This Desktop window does not own that request.")
			return
		}
		if !desktopControlTerminal(run.Status) {
			if run.Status == "queued" {
				desktopControlWrite(w, 409, nil, "run_not_claimed", "This request has not been claimed by Desktop.")
				return
			}
			if run.ContextID != request.Context.ID || run.ProjectPath != desktopControlPath(request.Context) {
				s.fail(run, "The Desktop project or conversation changed. This request will not run again.", now)
			} else {
				run.acknowledged = true
				run.Status, run.Message, run.UpdatedAt = update.Status, update.Message, now
				if update.Text != "" || update.Model != "" || update.NativeRunID != "" {
					if run.Result == nil {
						run.Result = &desktopControlResult{}
					}
					if update.Text != "" {
						run.Result.Text = update.Text
					}
					if update.Model != "" {
						run.Result.Model = update.Model
					}
					if update.NativeRunID != "" {
						run.Result.NativeRunID = update.NativeRunID
					}
				}
			}
		}
	}
	// Heartbeats retain the last interaction time, even when browsers disagree on focus.
	// Focus loss keeps the selection available; pending work keeps its original window.
	newerFocus := s.client != nil && request.Context.Active && request.Context.FocusedAt > s.client.context.FocusedAt
	legacyFocus := s.client != nil && request.Context.FocusedAt == 0 && s.client.context.FocusedAt == 0 && (!s.client.context.Active || request.Context.Active && request.Context.Focused)
	if s.client == nil || s.client.id == request.ClientID || (newerFocus || legacyFocus) && s.pending() == nil {
		s.client = &desktopControlClient{id: request.ClientID, context: request.Context, seen: now}
	}
	var claim *desktopControlRun
	if s.client.id == request.ClientID {
		if run := s.pending(); run != nil {
			switch {
			case run.clientID != request.ClientID || run.ContextID != request.Context.ID || run.ProjectPath != desktopControlPath(request.Context):
				s.fail(run, "The Desktop project or conversation changed. This request will not run again.", now)
			case run.Status == "queued" && request.Context.Busy:
				s.fail(run, "Desktop became busy before this request started. Send a new request when it is ready.", now)
			case run.Status == "queued" && !request.Context.Active:
				s.fail(run, "Desktop left Chat before this request started. Open Chat before sending a new request.", now)
			case run.Status == "queued" && request.Context.Active:
				if !desktopControlSupports(request.Context, run.Model, run.Kind) {
					s.fail(run, "The selected model is no longer available for this action. This request did not start.", now)
				} else {
					run.Status, run.UpdatedAt = "running", now
					claim = run
				}
			}
		}
	}
	desktopControlWrite(w, 200, map[string]any{"run": claim}, "", "")
}
