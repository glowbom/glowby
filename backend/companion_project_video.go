package main

// Phone-only orchestration reuses the existing project brief and build worker.
// A durable request receipt is saved before an agent can start.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type companionProjectMediaRequest struct {
	RequestID       string   `json:"requestId"`
	VideoID         string   `json:"videoId,omitempty"`
	Template        string   `json:"template,omitempty"`
	Prompt          string   `json:"prompt,omitempty"`
	Model           *string  `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoningEffort,omitempty"`
	PermissionMode  string   `json:"permissionMode,omitempty"`
	BuildTargets    []string `json:"buildTargets,omitempty"`
}

type companionProjectMediaReceipt struct {
	RequestID string           `json:"requestId"`
	ProjectID string           `json:"projectId"`
	Workflow  string           `json:"workflow"`
	Hash      string           `json:"hash"`
	Started   bool             `json:"started"`
	Plan      projectVideoPlan `json:"plan"`
	Snapshot  map[string]any   `json:"snapshot"`
}

var companionProjectMediaLocks [32]sync.Mutex

func companionProjectMediaError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"code": code, "message": message})
}

func companionProjectMediaLock(id string) *sync.Mutex {
	digest := sha256.Sum256([]byte(id))
	return &companionProjectMediaLocks[int(digest[0])%len(companionProjectMediaLocks)]
}

func companionProjectMediaDirectory() (string, error) {
	base, err := studioRootDirectory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return "", err
	}
	defer root.Close()
	const name = "PhoneProjectVideos"
	if err = root.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	info, err := root.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("The phone video request folder is unavailable.")
	}
	return filepath.Join(base, name), nil
}

func readCompanionProjectMediaReceipt(directory, id string) (*companionProjectMediaReceipt, error) {
	_, data, err := readStudioJSON(filepath.Join(directory, id+".json"), 2<<20)
	if err != nil {
		return nil, err
	}
	var record companionProjectMediaReceipt
	if json.Unmarshal(data, &record) != nil || record.RequestID != id || record.Snapshot == nil {
		return nil, errors.New("This video request could not be recovered.")
	}
	return &record, nil
}

func writeCompanionProjectMedia(directory string, record *companionProjectMediaReceipt) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return atomicChatFile(directory, record.RequestID+".json", data)
}

func (s *companionSession) routeProjectVideo(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if len(parts) < 3 || parts[0] != "projects" || (parts[2] != "project-video" && parts[2] != "video-ideas") {
		return false
	}
	project, ok := s.project(parts[1])
	if !ok {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	if len(parts) == 3 && parts[2] == "video-ideas" && r.Method == http.MethodGet {
		ideas, exists, err := readProjectVideoIdeas(project.path)
		if ideas == nil {
			ideas = []projectVideoIdea{}
		}
		response := map[string]any{"ideas": ideas, "exists": exists}
		if err != nil {
			response["error"] = err.Error()
		}
		writeJSON(w, response)
		return true
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		s.recoverProjectMedia(w, r, project, parts[2], parts[3])
		return true
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		s.startProjectMedia(w, r, project, parts[2])
		return true
	}
	http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
	return true
}

func (s *companionSession) startProjectMedia(w http.ResponseWriter, r *http.Request, project companionProject, workflow string) {
	var request companionProjectMediaRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Check this video request and try again. No build started.")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Send one video request. No build started.")
		return
	}
	request.RequestID = strings.ToLower(normalizedStudioUUID(request.RequestID))
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.RequestID == "" || !validBuildPermissionMode(request.PermissionMode) || len([]rune(request.Prompt)) > 2000 {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Use a request ID, a prompt up to 2,000 characters, and ask or all for build permissions.")
		return
	}
	if workflow == "project-video" {
		request.VideoID = normalizedStudioUUID(request.VideoID)
		if request.Template == "" {
			request.Template = "launch"
		}
		if request.VideoID == "" || (request.Template != "launch" && request.Template != "custom") || (request.Template == "custom" && request.Prompt == "") {
			companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Choose a saved video and a launch or custom video template. Describe custom videos.")
			return
		}
	} else if request.VideoID != "" || request.Template != "" || request.Prompt != "" {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Video ideas need the shared project and a build model.")
		return
	}
	lock := companionProjectMediaLock(request.RequestID)
	lock.Lock()
	defer lock.Unlock()
	directory, err := companionProjectMediaDirectory()
	if err != nil {
		http.Error(w, "Desktop could not save this request safely. Try again.", http.StatusInternalServerError)
		return
	}
	data, _ := json.Marshal(request)
	digest := sha256.Sum256(append([]byte(project.ID+"\x00"+workflow+"\x00"), data...))
	hash := hex.EncodeToString(digest[:])
	if previous, err := readCompanionProjectMediaReceipt(directory, request.RequestID); err == nil {
		if previous.Hash != hash || previous.ProjectID != project.ID || previous.Workflow != workflow {
			http.Error(w, "This request ID was already used with different settings. Review the request again.", http.StatusConflict)
			return
		}
		s.writeProjectMediaRecovery(w, r, project, directory, previous)
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "Desktop could not recover this request. Check its progress before starting another.", http.StatusConflict)
		return
	}
	if s.sharedJob(request.RequestID) != nil {
		http.Error(w, "This request ID already belongs to another build.", http.StatusConflict)
		return
	}
	model := s.modelChoice(project, false).Model
	if request.Model != nil {
		model = *request.Model
	}
	models, err := s.buildModelOptions(r.Context())
	var selected *chatModel
	for i := range models {
		if models[i].ID == model {
			selected = &models[i]
			break
		}
	}
	if err != nil || selected == nil {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Choose a connected Desktop build model. No build started.")
		return
	}
	driver, buildModel := companionBuildDriver(model)
	if request.ReasoningEffort != "" && (driver != "codex" || !studioVideoStringIn(request.ReasoningEffort, selected.ReasoningEfforts)) {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", "Choose a reasoning effort reported by the selected model. No build started.")
		return
	}
	if err := companionValidateBuildTargets(project.path, request.BuildTargets); err != nil {
		companionProjectMediaError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	record := &companionProjectMediaReceipt{RequestID: request.RequestID, ProjectID: project.ID, Workflow: workflow, Hash: hash,
		Snapshot: map[string]any{"id": request.RequestID, "projectId": project.ID, "kind": "build", "workflow": workflow, "status": "running", "output": []string{}, "startedAt": s.now().UTC().Format(time.RFC3339Nano), "model": model, "agentDriver": driver}}
	if workflow == "project-video" {
		record.Snapshot["resultStatus"] = "pending"
	}
	if err := writeCompanionProjectMedia(directory, record); err != nil {
		http.Error(w, "Desktop could not save this request safely. No build started.", http.StatusInternalServerError)
		return
	}
	ctx, stop := s.chatContext(r.Context())
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	message := ""
	if workflow == "project-video" {
		record.Plan, err = prepareProjectVideoWithRequest(ctx, request.VideoID, project.path, request.Template, request.Prompt, request.RequestID)
		message = record.Plan.Message
	} else {
		message, err = prepareProjectVideoIdeas(project.path)
	}
	if err != nil || ctx.Err() != nil {
		record.Snapshot["status"], record.Snapshot["error"] = "failed", "Desktop could not prepare the video request. Review it and try again."
		if err != nil {
			record.Snapshot["error"] = err.Error()
		}
		record.Snapshot["resultStatus"] = "failed"
		_ = writeCompanionProjectMedia(directory, record)
		writeJSON(w, record.Snapshot)
		return
	}
	payload := OpenCodeAgentRequest{ProjectPath: project.path, Instructions: message, Model: buildModel, AgentDriver: driver,
		ReasoningEffort: request.ReasoningEffort, PermissionMode: request.PermissionMode, BuildTargets: request.BuildTargets,
		OpenAIAuthMode: "opencode-config", PersistCurrentInstructionsToHistory: true, MediaGenerationPolicy: "skip"}
	s.startJobWithSetup(w, r, "build", project, "/opencode/refine", payload, func(job *companionJob) error {
		job.id, job.workflow = request.RequestID, workflow
		if workflow == "project-video" {
			job.resultStatus = "pending"
		}
		record.Started, record.Snapshot = true, job.snapshot(false)
		if err := writeCompanionProjectMedia(directory, record); err != nil {
			return errors.New("Desktop could not save this request safely. No build started.")
		}
		job.afterFinish = func() { s.finishProjectMedia(project, directory, record.RequestID, job) }
		return nil
	})
	if !record.Started {
		record.Snapshot["status"] = "failed"
		record.Snapshot["error"] = "The build did not start. Review the request after the current work finishes."
		_ = writeCompanionProjectMedia(directory, record)
	}
}

func (s *companionSession) recoverProjectMedia(w http.ResponseWriter, r *http.Request, project companionProject, workflow, id string) {
	id = strings.ToLower(normalizedStudioUUID(id))
	if id == "" {
		http.NotFound(w, r)
		return
	}
	lock := companionProjectMediaLock(id)
	lock.Lock()
	defer lock.Unlock()
	directory, err := companionProjectMediaDirectory()
	if err != nil {
		http.Error(w, "Desktop could not open the saved video request.", http.StatusInternalServerError)
		return
	}
	record, err := readCompanionProjectMediaReceipt(directory, id)
	if errors.Is(err, os.ErrNotExist) && s.sharedJob(id) == nil {
		companionProjectMediaError(w, http.StatusNotFound, "request_missing", "Desktop has no saved submission for this request. Review the step again before starting it.")
		return
	}
	if err != nil || record.ProjectID != project.ID || record.Workflow != workflow {
		http.NotFound(w, r)
		return
	}
	s.writeProjectMediaRecovery(w, r, project, directory, record)
}

func (s *companionSession) writeProjectMediaRecovery(w http.ResponseWriter, r *http.Request, project companionProject, directory string, record *companionProjectMediaReceipt) {
	job := s.sharedJob(record.RequestID)
	if job != nil && job.projectID == project.ID {
		// Restored generic build snapshots do not carry phone workflow metadata.
		job.mu.Lock()
		job.workflow = record.Workflow
		if job.resultStatus == "" && record.Snapshot["resultStatus"] == "ready" && record.Snapshot["video"] != nil {
			job.resultStatus = "ready"
			job.video, _ = json.Marshal(record.Snapshot["video"])
		}
		job.mu.Unlock()
		record.Snapshot = job.snapshot(false)
		record.Snapshot["workflow"] = record.Workflow
	} else if record.Snapshot["status"] == "running" || record.Snapshot["status"] == "waiting" {
		record.Snapshot["status"] = "failed"
		record.Snapshot["error"] = "Desktop restarted or lost this build's progress. Check the project before starting another build."
	}
	status, _ := record.Snapshot["status"].(string)
	if record.Workflow == "project-video" {
		ready := record.Snapshot["resultStatus"] == "ready" && record.Snapshot["video"] != nil
		finishedAt, _ := record.Snapshot["finishedAt"].(string)
		finished, _ := time.Parse(time.RFC3339Nano, finishedAt)
		// The stream can report completion before the worker returns. Its final
		// timestamp confirms that the worker has stopped writing the output.
		settling := status == "completed" && (finished.IsZero() || time.Since(finished) < 4*time.Second)
		// The imported Studio asset stays available if the source file is moved.
		if !ready {
			if status != "running" && status != "waiting" && !settling {
				s.collectProjectMedia(r.Context(), project, record, job)
			} else {
				record.Snapshot["resultStatus"] = "pending"
			}
		}
	}
	_ = writeCompanionProjectMedia(directory, record)
	writeJSON(w, record.Snapshot)
}

func (s *companionSession) finishProjectMedia(project companionProject, directory, id string, job *companionJob) {
	// The existing importer requires a file to be unchanged for three seconds.
	finished := job.snapshot(false)
	if finished["status"] == "completed" && finished["workflow"] == "project-video" {
		timer := time.NewTimer(3500 * time.Millisecond)
		select {
		case <-timer.C:
		case <-job.ctx.Done():
			timer.Stop()
		}
	}
	lock := companionProjectMediaLock(id)
	lock.Lock()
	defer lock.Unlock()
	record, err := readCompanionProjectMediaReceipt(directory, id)
	if err != nil {
		return
	}
	record.Snapshot = job.snapshot(false)
	if record.Workflow == "project-video" {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		s.collectProjectMedia(ctx, project, record, job)
		cancel()
	}
	_ = writeCompanionProjectMedia(directory, record)
	if job.save != nil {
		job.save()
	}
}

func (s *companionSession) collectProjectMedia(ctx context.Context, project companionProject, record *companionProjectMediaReceipt, job *companionJob) {
	var asset *studioImageSummary
	root, err := chatProjectRoot(project.path)
	if err == nil && root == project.path && record.Plan.PreparedAt > 0 {
		asset, err = importProjectVideoResult(ctx, project.path, record.Plan.Output, record.Plan.PreparedAt)
	} else {
		err = errors.New("The shared project is no longer available.")
	}
	resultStatus, resultError := "ready", ""
	if asset == nil {
		resultStatus = "failed"
		resultError = "The build finished without a playable project video. Check the build result before trying again."
		if err != nil {
			resultError = err.Error()
		}
	}
	if job != nil {
		job.mu.Lock()
		job.workflow, job.resultStatus, job.resultError = record.Workflow, resultStatus, resultError
		if asset != nil {
			job.video, _ = json.Marshal(asset)
		}
		job.mu.Unlock()
		record.Snapshot = job.snapshot(false)
	} else {
		record.Snapshot["resultStatus"] = resultStatus
		if resultError != "" {
			record.Snapshot["resultError"] = resultError
		} else {
			delete(record.Snapshot, "resultError")
		}
		if asset != nil {
			record.Snapshot["video"] = asset
		}
	}
}
