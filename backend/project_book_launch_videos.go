package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const bookLaunchVideoFolder = "BookLaunchVideos"

type bookLaunchVideoTarget struct {
	EntryID                string `json:"entryId"`
	VideoReferenceID       string `json:"videoReferenceId"`
	ExpectedContentVersion string `json:"expectedContentVersion"`
}

// Local receipts are outside the portable Book. A receipt can start one worker.
type bookLaunchVideoRecord struct {
	Version          int                   `json:"version"`
	RequestID        string                `json:"requestId"`
	ProjectPath      string                `json:"projectPath"`
	InputHash        string                `json:"inputHash"`
	Target           bookLaunchVideoTarget `json:"target"`
	Reference        bookVideo             `json:"reference"`
	Plan             projectVideoPlan      `json:"plan"`
	Status           string                `json:"status"`
	JobID            string                `json:"jobId,omitempty"`
	AgentDriver      string                `json:"agentDriver,omitempty"`
	Model            string                `json:"model,omitempty"`
	WorkerStatus     string                `json:"workerStatus,omitempty"`
	WorkerFinishedAt string                `json:"workerFinishedAt,omitempty"`
	AssetID          string                `json:"assetId,omitempty"`
	Error            string                `json:"error,omitempty"`
}

type bookLaunchVideoResult struct {
	Status       string              `json:"status"`
	JobID        string              `json:"jobId,omitempty"`
	AgentDriver  string              `json:"agentDriver,omitempty"`
	Model        string              `json:"model,omitempty"`
	WorkerStatus string              `json:"workerStatus,omitempty"`
	Plan         projectVideoPlan    `json:"plan"`
	Asset        *studioImageSummary `json:"asset,omitempty"`
	Error        string              `json:"error,omitempty"`
	Book         *projectBook        `json:"book,omitempty"`
}

var bookLaunchPrepareLocks [32]sync.Mutex

// Access to receipts and this map uses projectBookMu. Never hold it during import.
var bookLaunchActive = make(map[string]*companionJob)

func bookLaunchRequestID(id string) string { return strings.ToLower(normalizedStudioUUID(id)) }

func readBookLaunchVideo(folder *os.Root, id string) (bookLaunchVideoRecord, error) {
	var record bookLaunchVideoRecord
	if id == "" || bookLaunchRequestID(id) != id {
		return record, errors.New("Invalid story graphics request.")
	}
	_, data, err := readStudioLibraryRecord(folder, id+".json", 64<<10)
	if err != nil {
		return record, err
	}
	if json.Unmarshal(data, &record) != nil || record.Version != 1 || record.RequestID != id || !filepath.IsAbs(record.ProjectPath) || record.Plan.Project != record.ProjectPath || record.Plan.RequestID != id || record.Plan.PreparedAt <= 0 || !projectBookID.MatchString(record.Target.EntryID) || record.Reference.ID != record.Target.VideoReferenceID || validateBookVideos([]bookVideo{record.Reference}) != nil || (record.JobID != "" && bookLaunchRequestID(record.JobID) != record.JobID) {
		return bookLaunchVideoRecord{}, errors.New("The saved story graphics request is invalid.")
	}
	if record.AgentDriver != "" && !bookLaunchBuildDriver(record.AgentDriver) {
		return bookLaunchVideoRecord{}, errors.New("The saved story graphics agent is invalid.")
	}
	switch record.Status {
	case "prepared", "running", "completed", "failed", "superseded":
	default:
		return bookLaunchVideoRecord{}, errors.New("The saved story graphics status is invalid.")
	}
	return record, nil
}

func saveBookLaunchVideo(folder *os.Root, record bookLaunchVideoRecord) error {
	data, err := json.Marshal(record)
	if err != nil || len(data) > 64<<10 || bookLaunchRequestID(record.RequestID) != record.RequestID || record.RequestID == "" {
		return errors.New("Could not save the story graphics request.")
	}
	name := record.RequestID + ".json"
	if info, err := folder.Lstat(name); err == nil && !info.Mode().IsRegular() || err != nil && !os.IsNotExist(err) {
		return errors.New("The story graphics request file is unavailable.")
	}
	temporary := ".launch-" + randomUUIDString()
	file, err := folder.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer folder.Remove(temporary)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = folder.Rename(temporary, name)
	}
	return err
}

func bookLaunchEntry(record bookLaunchVideoRecord, checkVersion bool) (*os.Root, projectBook, int, error) {
	root, name, err := openProjectBookProject(record.ProjectPath)
	if err != nil {
		return nil, projectBook{}, -1, err
	}
	book, exists, err := readProjectBook(root, name)
	index := bookVideoEntryIndex(&book, record.Target.EntryID)
	if err != nil || !exists || root.Name() != record.ProjectPath || index < 0 {
		root.Close()
		return nil, book, -1, errors.New("The original story is unavailable.")
	}
	entry := book.Entries[index]
	if len(entry.Videos) != 1 || !bookVideoReferencesMatch(entry, []string{record.Target.VideoReferenceID}) || entry.Videos[0].AssetID != record.Reference.AssetID {
		root.Close()
		return nil, book, -1, errors.New("The story's video changed. The current choice was kept.")
	}
	if checkVersion && (record.Target.ExpectedContentVersion == "" || record.Target.ExpectedContentVersion != bookEntryContentVersion(entry)) {
		root.Close()
		return nil, book, -1, errProjectBookConflict
	}
	return root, book, index, nil
}

func prepareBookLaunchVideo(ctx context.Context, id, path, template, prompt, requestID string, target bookLaunchVideoTarget) (projectVideoPlan, error) {
	requestID = bookLaunchRequestID(requestID)
	if requestID == "" || !projectBookID.MatchString(target.EntryID) || !projectBookID.MatchString(target.VideoReferenceID) || target.ExpectedContentVersion == "" {
		return projectVideoPlan{}, errors.New("Choose a saved story video and review its current version first.")
	}
	root, _, err := openProjectBookProject(path)
	if err != nil {
		return projectVideoPlan{}, err
	}
	path = root.Name()
	root.Close()
	id = normalizedStudioUUID(id)
	input, _ := json.Marshal(struct {
		ID, Path, Template, Prompt string
		Target                     bookLaunchVideoTarget
	}{id, path, template, strings.TrimSpace(prompt), target})
	hash := sha256.Sum256(input)
	lock := &bookLaunchPrepareLocks[int(hashRequestID(requestID))%len(bookLaunchPrepareLocks)]
	lock.Lock()
	defer lock.Unlock()
	projectBookMu.Lock()
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, true)
	if err != nil {
		projectBookMu.Unlock()
		return projectVideoPlan{}, err
	}
	defer folder.Close()
	if prior, readErr := readBookLaunchVideo(folder, requestID); readErr == nil {
		projectBookMu.Unlock()
		if prior.InputHash != hex.EncodeToString(hash[:]) || prior.ProjectPath != path {
			return projectVideoPlan{}, errors.New("This graphics request already belongs to different inputs. Review a new request.")
		}
		return prior.Plan, nil
	} else if !os.IsNotExist(readErr) {
		projectBookMu.Unlock()
		return projectVideoPlan{}, readErr
	}
	record := bookLaunchVideoRecord{Version: 1, RequestID: requestID, ProjectPath: path, InputHash: hex.EncodeToString(hash[:]), Target: target, Reference: bookVideo{ID: target.VideoReferenceID, AssetID: id}, Status: "prepared"}
	bookRoot, book, index, err := bookLaunchEntry(record, true)
	if err == nil {
		record.Reference = book.Entries[index].Videos[0]
		bookRoot.Close()
	}
	projectBookMu.Unlock()
	if err != nil {
		return projectVideoPlan{}, err
	}
	plan, err := prepareProjectVideoWithRequest(ctx, id, path, template, strings.TrimSpace(prompt), requestID)
	if err != nil {
		return projectVideoPlan{}, err
	}
	plan.RequestID = requestID
	record.Plan = plan
	projectBookMu.Lock()
	defer projectBookMu.Unlock()
	bookRoot, _, _, err = bookLaunchEntry(record, true)
	if err != nil {
		return projectVideoPlan{}, err
	}
	bookRoot.Close()
	if err := saveBookLaunchVideo(folder, record); err != nil {
		return projectVideoPlan{}, err
	}
	return plan, nil
}

func hashRequestID(id string) byte { sum := sha256.Sum256([]byte(id)); return sum[0] }

func bookLaunchBuildDriver(driver string) bool {
	switch driver {
	case "opencode", "cursor", "codex", "claude-code", "acp":
		return true
	default:
		return false
	}
}

func bookLaunchWorkerIdentity(record bookLaunchVideoRecord) (string, string) {
	if record.JobID == "" {
		return "", ""
	}
	if record.AgentDriver == "" {
		// Receipts written before agent selection supported only Claude Code.
		return "claude-code", ""
	}
	return record.AgentDriver, record.Model
}

func bookLaunchWorkerMatches(record bookLaunchVideoRecord, id, driver, model string) bool {
	expectedDriver, expectedModel := bookLaunchWorkerIdentity(record)
	return id == record.JobID && driver == expectedDriver && (record.AgentDriver == "" || model == expectedModel)
}

// Bind before entering the worker. A second submission never starts another build.
func bindBookLaunchVideo(job *companionJob, request OpenCodeAgentRequest) error {
	if request.BookLaunchVideoRequestID == "" {
		return nil
	}
	projectBookMu.Lock()
	defer projectBookMu.Unlock()
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if err != nil {
		return errors.New("Prepare this story's graphics before starting the build.")
	}
	defer folder.Close()
	record, err := readBookLaunchVideo(folder, bookLaunchRequestID(request.BookLaunchVideoRequestID))
	if err != nil {
		return errors.New("The saved story graphics request is unavailable.")
	}
	driver := normalizedBuildDriver(request.AgentDriver)
	model := request.Model
	if driver != "opencode" && !strings.HasPrefix(model, driver+"/") {
		model = driver + "/" + model
	}
	if !bookLaunchBuildDriver(driver) || job.agentDriver != driver || job.model != model {
		return errors.New("Choose a supported build agent and its matching model for this graphics request.")
	}
	if record.ProjectPath != job.projectPath || record.Status != "prepared" || record.JobID != "" {
		return errors.New("This story graphics request cannot start another build. Check its saved status.")
	}
	if !strings.Contains(request.Instructions, record.Plan.Brief) || !strings.Contains(request.Instructions, record.Plan.Output) {
		return errors.New("Keep the prepared video brief and output path in the build request.")
	}
	root, _, _, err := bookLaunchEntry(record, false)
	if err != nil {
		return err
	}
	root.Close()
	if _, err := savedBookStudioVideo(record.Reference.AssetID); err != nil {
		return err
	}
	record.JobID, record.Status, record.Error = job.id, "running", ""
	record.AgentDriver, record.Model = job.agentDriver, job.model
	if err := saveBookLaunchVideo(folder, record); err != nil {
		return errors.New("Could not save the accepted graphics request. No build started.")
	}
	bookLaunchActive[record.RequestID] = job
	job.afterFinish = func() { finishBookLaunchVideo(record.RequestID, job) }
	return nil
}

func finishBookLaunchVideo(id string, job *companionJob) {
	job.mu.Lock()
	status, finished, driver, model := job.status, job.finishedAt, job.agentDriver, job.model
	job.mu.Unlock()
	projectBookMu.Lock()
	delete(bookLaunchActive, id)
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if err == nil {
		if record, readErr := readBookLaunchVideo(folder, id); readErr == nil && bookLaunchWorkerMatches(record, job.id, driver, model) {
			record.WorkerStatus, record.WorkerFinishedAt = status, finished
			_ = saveBookLaunchVideo(folder, record)
		}
		folder.Close()
	}
	projectBookMu.Unlock()
	// The importer requires three seconds of stable output after the worker exits.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		if status == "completed" {
			timer := time.NewTimer(3500 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
		}
		_, _ = reconcileBookLaunchVideo(ctx, job.projectPath, id)
	}()
}

// Saved run completion is authoritative after a restart. A bare SSE done event
// has no finishedAt and cannot authorize import of an unfinished output.
func bookLaunchWorkerState(record bookLaunchVideoRecord) (string, string) {
	if record.WorkerFinishedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, record.WorkerFinishedAt); err == nil {
			return record.WorkerStatus, record.WorkerFinishedAt
		}
	}
	if active := bookLaunchActive[record.RequestID]; active != nil {
		active.mu.Lock()
		matches := bookLaunchWorkerMatches(record, active.id, active.agentDriver, active.model)
		active.mu.Unlock()
		if matches {
			return "running", ""
		}
		return "interrupted", ""
	}
	root, err := os.OpenRoot(record.ProjectPath)
	if err != nil {
		return "interrupted", ""
	}
	defer root.Close()
	data, err := bookRead(root, filepath.Join(".glowbom", "runs", record.JobID, "run.json"), 1<<20)
	var run struct{ ID, ProjectID, Kind, Status, AgentDriver, Model, FinishedAt string }
	if err != nil || json.Unmarshal(data, &run) != nil || !bookLaunchWorkerMatches(record, run.ID, run.AgentDriver, run.Model) || run.ProjectID != companionProjectID(record.ProjectPath) || run.Kind != "build" {
		return "interrupted", ""
	}
	if _, err := time.Parse(time.RFC3339Nano, run.FinishedAt); err != nil {
		return "interrupted", ""
	}
	return run.Status, run.FinishedAt
}

func reconcileBookLaunchVideo(ctx context.Context, path, id string) (bookLaunchVideoResult, error) {
	projectBookMu.Lock()
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if err != nil {
		projectBookMu.Unlock()
		return bookLaunchVideoResult{}, err
	}
	defer folder.Close()
	record, err := readBookLaunchVideo(folder, bookLaunchRequestID(id))
	resolved, pathErr := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) || pathErr != nil || resolved != record.ProjectPath {
		projectBookMu.Unlock()
		return bookLaunchVideoResult{}, errors.New("The saved story graphics request was not found in this project.")
	}
	workerStatus, finished := "", ""
	if record.JobID != "" {
		workerStatus, finished = bookLaunchWorkerState(record)
	}
	projectBookMu.Unlock()
	var asset *studioImageSummary
	if record.AssetID != "" {
		if saved, readErr := findStudioAsset(record.AssetID); readErr == nil {
			summary := summarizeStudioRecord(saved)
			asset = &summary
		}
	} else if record.JobID != "" && workerStatus == "completed" && finished != "" {
		asset, err = importProjectVideoResult(ctx, record.ProjectPath, record.Plan.Output, record.Plan.PreparedAt)
	}
	projectBookMu.Lock()
	defer projectBookMu.Unlock()
	record, readErr := readBookLaunchVideo(folder, record.RequestID)
	if readErr != nil {
		return bookLaunchVideoResult{}, readErr
	}
	if record.Status != "completed" && record.Status != "superseded" {
		if record.JobID != "" && workerStatus != "running" && workerStatus != "completed" {
			record.Status, record.Error = "failed", "The graphics build did not finish. Your original story video is unchanged. Review the build before starting another attempt."
		} else if err != nil {
			record.Status, record.Error = "failed", "The finished graphics video could not be imported. Your original video is unchanged; check this request again."
		}
		if workerStatus == "completed" && asset == nil && err == nil {
			record.Status, record.Error = "running", "The build finished. Waiting for its final playable video; your original video is unchanged."
			if ended, parseErr := time.Parse(time.RFC3339Nano, finished); parseErr == nil && time.Since(ended) > 40*time.Second {
				record.Status, record.Error = "failed", "The build finished without a playable final video. Your original video is unchanged. Check the build result, then check this request again."
			}
		}
	}
	if asset != nil {
		record.AssetID = asset.ID
		if record.Status != "completed" && record.Status != "superseded" {
			// Checkpoint the imported asset before modifying the portable Book.
			if saveErr := saveBookLaunchVideo(folder, record); saveErr != nil {
				return bookLaunchVideoResult{}, saveErr
			}
			// A previous Book write can succeed even if its receipt checkpoint fails.
			if root, name, openErr := openProjectBookProject(record.ProjectPath); openErr == nil {
				if book, exists, readErr := readProjectBook(root, name); readErr == nil && exists {
					if index := bookVideoEntryIndex(&book, record.Target.EntryID); index >= 0 && len(book.Entries[index].Videos) == 1 && hasBookVideo(book.Entries[index], asset.ID) {
						record.Status, record.Error = "completed", ""
					}
				}
				root.Close()
			}
		}
		if record.Status != "completed" && record.Status != "superseded" {
			root, book, index, targetErr := bookLaunchEntry(record, false)
			if targetErr != nil {
				record.Status, record.Error = "superseded", "The finished video is saved in Studio. The original story video changed, so its current choice was kept."
			} else {
				entry := book.Entries[index]
				if replaceErr := replaceBookVideo(root, &entry, asset.ID, record.Reference.Title, record.Reference.Caption); replaceErr != nil {
					record.Error = "The finished video is saved in Studio. Open the Book again to retry its link."
				} else {
					entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
					if saveErr := saveBookEntry(root, entry); saveErr != nil {
						record.Error = "The finished video is saved in Studio. Open the Book again to retry its link."
					} else {
						record.Status, record.Error = "completed", ""
					}
				}
				root.Close()
			}
		}
	}
	if err := saveBookLaunchVideo(folder, record); err != nil {
		return bookLaunchVideoResult{}, err
	}
	driver, model := bookLaunchWorkerIdentity(record)
	result := bookLaunchVideoResult{Status: record.Status, JobID: record.JobID, AgentDriver: driver, Model: model, WorkerStatus: workerStatus, Plan: record.Plan, Asset: asset, Error: record.Error}
	if root, name, openErr := openProjectBookProject(record.ProjectPath); openErr == nil {
		if book, exists, readErr := readProjectBook(root, name); readErr == nil && exists {
			setBookStoryEligibility(&book)
			result.Book = &book
		}
		root.Close()
	}
	return result, nil
}

// Called outside projectBookMu on Book reads. No provider or build is started.
func reconcileBookLaunchVideos(path string) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) {
		return
	}
	path = resolved
	projectBookMu.Lock()
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if err != nil {
		projectBookMu.Unlock()
		return
	}
	dir, err := folder.Open(".")
	var ids []string
	if err == nil {
		names, _ := dir.Readdirnames(-1)
		dir.Close()
		for _, name := range names {
			id := strings.TrimSuffix(name, ".json")
			if id == name {
				continue
			}
			record, err := readBookLaunchVideo(folder, id)
			if err == nil && record.ProjectPath == path && record.JobID != "" && record.Status != "completed" {
				ids = append(ids, id)
			}
		}
	}
	folder.Close()
	projectBookMu.Unlock()
	// Bound the whole Book read, including projects with several older receipts.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		_, _ = reconcileBookLaunchVideo(ctx, path, id)
	}
}

// Called with projectBookMu held before an intentional reference edit.
func supersedeBookLaunchVideosForEdit(root *os.Root, entryID string) error {
	folder, err := openStudioLibraryFolder(bookLaunchVideoFolder, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer folder.Close()
	dir, err := folder.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		id := strings.TrimSuffix(name, ".json")
		if id == name {
			continue
		}
		record, err := readBookLaunchVideo(folder, id)
		if err != nil {
			return err
		}
		if record.ProjectPath != root.Name() || record.Target.EntryID != entryID || record.Status == "completed" || record.Status == "superseded" {
			continue
		}
		record.Status, record.Error = "superseded", "The story's video changed. Any graphics result stays in Studio."
		if err := saveBookLaunchVideo(folder, record); err != nil {
			return err
		}
	}
	return nil
}
