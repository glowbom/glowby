package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// A Book keeps only references. Studio remains the owner of the saved media.
type bookVideo struct {
	ID        string `json:"id"`
	AssetID   string `json:"assetId"`
	Title     string `json:"title"`
	Caption   string `json:"caption"`
	CreatedAt string `json:"createdAt"`
}

type bookEntryJSON bookEntry

var bookEntryJSONFields = func() map[string]bool {
	fields := make(map[string]bool)
	typ := reflect.TypeOf(bookEntryJSON{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = true
		}
	}
	return fields
}()

// Preserve fields from newer clients when an older client edits a known field.
func (entry *bookEntry) UnmarshalJSON(data []byte) error {
	var known bookEntryJSON
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range bookEntryJSONFields {
		delete(fields, name)
	}
	*entry = bookEntry(known)
	entry.extraFields = fields
	return nil
}

func (entry bookEntry) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(bookEntryJSON(entry))
	if err != nil || len(entry.extraFields) == 0 {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for name, value := range entry.extraFields {
		if !bookEntryJSONFields[name] {
			fields[name] = value
		}
	}
	return json.Marshal(fields)
}

func validateBookVideos(videos []bookVideo) error {
	if len(videos) > 32 {
		return errors.New("An entry can contain up to 32 videos.")
	}
	ids, assets := make(map[string]bool), make(map[string]bool)
	for _, video := range videos {
		assetID := normalizedStudioUUID(video.AssetID)
		if !projectBookID.MatchString(video.ID) || ids[video.ID] || assetID == "" || assetID != video.AssetID || assets[assetID] || utf8.RuneCountInString(video.Title) > 240 || strings.ContainsAny(video.Title, "\r\n\x00") || utf8.RuneCountInString(video.Caption) > 2000 || len(video.CreatedAt) > 64 {
			return errors.New("Invalid Project Book video reference.")
		}
		if _, err := time.Parse(time.RFC3339Nano, video.CreatedAt); err != nil {
			return errors.New("Invalid Project Book video date.")
		}
		ids[video.ID], assets[assetID] = true, true
	}
	return nil
}

func hasBookVideo(entry bookEntry, assetID string) bool {
	for _, video := range entry.Videos {
		if video.AssetID == assetID {
			return true
		}
	}
	return false
}

func bookVideoReferenceIDs(entry bookEntry) []string {
	ids := make([]string, len(entry.Videos))
	for i, video := range entry.Videos {
		ids[i] = video.ID
	}
	return ids
}

func bookVideoReferencesMatch(entry bookEntry, expectedIDs []string) bool {
	if len(entry.Videos) != len(expectedIDs) {
		return false
	}
	for i, video := range entry.Videos {
		if video.ID != expectedIDs[i] {
			return false
		}
	}
	return true
}

func savedBookStudioVideo(id string) (studioAssetMetadata, error) {
	var empty studioAssetMetadata
	id = normalizedStudioUUID(id)
	if id == "" {
		return empty, errors.New("Choose a saved Studio video.")
	}
	path, err := studioClipRecordPath(id)
	if err != nil {
		return empty, errors.New("That video is no longer available in Studio.")
	}
	asset, err := readStudioAssetMetadata(path)
	if err != nil || normalizedStudioUUID(asset.ID) != id || asset.MediaType != "video" {
		return empty, errors.New("Choose a saved Studio video.")
	}
	return asset, nil
}

func appendBookVideo(entry *bookEntry, assetID, title, caption string) error {
	assetID = normalizedStudioUUID(assetID)
	if hasBookVideo(*entry, assetID) {
		return nil
	}
	if len(entry.Videos) != 0 {
		return errors.New("This story already has a video. Choose Replace video to change it.")
	}
	if _, err := savedBookStudioVideo(assetID); err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Story video"
	}
	video := bookVideo{ID: strings.ToLower(randomUUIDString()), AssetID: assetID, Title: title, Caption: caption, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	updated := append(append([]bookVideo(nil), entry.Videos...), video)
	if err := validateBookVideos(updated); err != nil {
		return err
	}
	entry.Videos = updated
	return nil
}

// The caller checks the expected revision or reference IDs before replacing.
// Only Book references change; every original media file stays in Studio.
func replaceBookVideo(root *os.Root, entry *bookEntry, assetID, title, caption string) error {
	updated := *entry
	updated.Videos = nil
	if err := appendBookVideo(&updated, assetID, title, caption); err != nil {
		return err
	}
	for _, video := range entry.Videos {
		if video.AssetID == updated.Videos[0].AssetID {
			updated.Videos[0] = video
			continue
		}
		if err := finalizeBookVideoBindingsForUnlink(root, entry.ID, video); err != nil {
			return err
		}
	}
	entry.Videos = updated.Videos
	return nil
}

const bookVideoBindingFolder = "BookVideoBindings"
const bookVideoBindingLimit = 32 << 10

// This record is private to Studio. Project paths never enter the portable Book.
type bookVideoBinding struct {
	Version          int      `json:"version"`
	GenerationID     string   `json:"generationId"`
	ProjectPath      string   `json:"projectPath"`
	EntryID          string   `json:"entryId"`
	Title            string   `json:"title"`
	SourceVersion    string   `json:"sourceVersion"`
	ExpectedVideoIDs []string `json:"expectedVideoIds"`
	CreatedAt        string   `json:"createdAt"`
	AssetID          string   `json:"assetId,omitempty"`
	AttachedAt       string   `json:"attachedAt,omitempty"`
	SupersededAt     string   `json:"supersededAt,omitempty"`
	Error            string   `json:"error,omitempty"`
	LastAttemptAt    string   `json:"lastAttemptAt,omitempty"`
}

func readBookVideoBinding(folder *os.Root, id string) (bookVideoBinding, error) {
	var binding bookVideoBinding
	if !studioGenerationID.MatchString(id) {
		return binding, errors.New("Invalid Studio generation identifier.")
	}
	_, data, err := readStudioLibraryRecord(folder, id+".json", bookVideoBindingLimit)
	if err != nil {
		return binding, err
	}
	if json.Unmarshal(data, &binding) != nil || (binding.Version != 1 && binding.Version != 2) || binding.GenerationID != id || !projectBookID.MatchString(binding.EntryID) || !filepath.IsAbs(binding.ProjectPath) || utf8.RuneCountInString(binding.Title) > 240 || strings.ContainsAny(binding.Title, "\r\n\x00") || len(binding.SourceVersion) > 128 || (binding.AssetID != "" && binding.AssetID != studioGenerationAssetID(id)) {
		return bookVideoBinding{}, errors.New("The saved story video link is invalid.")
	}
	if binding.Version == 2 {
		if binding.ExpectedVideoIDs == nil || len(binding.ExpectedVideoIDs) > 32 {
			return bookVideoBinding{}, errors.New("The saved story video references are invalid.")
		}
		seen := make(map[string]bool)
		for _, videoID := range binding.ExpectedVideoIDs {
			if !projectBookID.MatchString(videoID) || seen[videoID] {
				return bookVideoBinding{}, errors.New("The saved story video references are invalid.")
			}
			seen[videoID] = true
		}
	}
	return binding, nil
}

func saveBookVideoBinding(folder *os.Root, binding bookVideoBinding) error {
	data, err := json.Marshal(binding)
	if err != nil || len(data) > bookVideoBindingLimit || !studioGenerationID.MatchString(binding.GenerationID) {
		return errors.New("Could not save the story video link.")
	}
	name := binding.GenerationID + ".json"
	if info, err := folder.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("The saved story video link is unavailable.")
	} else if err != nil && !os.IsNotExist(err) {
		return errors.New("Could not inspect the saved story video link.")
	}
	temporary := ".binding-" + randomUUIDString()
	file, err := folder.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("Could not save the story video link.")
	}
	defer folder.Remove(temporary)
	defer file.Close()
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = folder.Rename(temporary, name)
	}
	if err != nil {
		return errors.New("Could not save the story video link.")
	}
	return nil
}

// Call with projectBookMu held. Binding is immutable once submitted.
func bindBookVideoGeneration(root *os.Root, book *projectBook, req projectBookRequest) error {
	if !studioGenerationID.MatchString(req.GenerationID) || utf8.RuneCountInString(req.Title) > 240 || strings.ContainsAny(req.Title, "\r\n\x00") {
		return errors.New("Choose a valid story video request.")
	}
	index := bookVideoEntryIndex(book, req.EntryID)
	if index < 0 {
		if bookDeletedEntryIndex(book, req.EntryID) >= 0 {
			return errProjectBookConflict
		}
		return errors.New("That Project Book entry was not found.")
	}
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, true)
	if err != nil {
		return errors.New("Could not open saved story video links.")
	}
	defer folder.Close()
	if binding, err := readBookVideoBinding(folder, req.GenerationID); err == nil {
		if binding.ProjectPath == root.Name() && binding.EntryID == req.EntryID {
			if binding.SupersededAt != "" {
				return errProjectBookConflict
			}
			return nil
		}
		return errors.New("This video request already belongs to another story.")
	} else if !os.IsNotExist(err) {
		return err
	}
	entry := book.Entries[index]
	if (req.ExpectedContentVersion != "" && req.ExpectedContentVersion != bookEntryContentVersion(entry)) || (req.ExpectedUpdatedAt != "" && req.ExpectedUpdatedAt != entry.UpdatedAt) {
		return errProjectBookConflict
	}
	if len(entry.Videos) > 0 && (!req.ReplaceExistingVideo || req.ExpectedContentVersion == "") {
		return errors.New("This story already has a video. Review its replacement before creating another.")
	}
	if checkpoint, found, err := loadStudioGenerationCheckpoint(req.GenerationID); err != nil || found || checkpoint.State.Found {
		return errors.New("Link the story before starting its video. Start a new video request.")
	}
	studioGenerations.mu.Lock()
	_, started := studioGenerations.entries[req.GenerationID]
	studioGenerations.mu.Unlock()
	if _, err := studioClipRecordPath(studioGenerationAssetID(req.GenerationID)); err == nil || started {
		return errors.New("Link the story before starting its video. Start a new video request.")
	}
	return saveBookVideoBinding(folder, bookVideoBinding{Version: 2, GenerationID: req.GenerationID, ProjectPath: root.Name(), EntryID: entry.ID, Title: strings.TrimSpace(req.Title), SourceVersion: bookEntryContentVersion(entry), ExpectedVideoIDs: bookVideoReferenceIDs(entry), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

func bookVideoEntryIndex(book *projectBook, entryID string) int {
	for i := range book.Entries {
		if book.Entries[i].ID == entryID {
			return i
		}
	}
	return -1
}

// Merge only the reference into the latest revision of the original story.
func attachCompletedBookVideo(root *os.Root, book *projectBook, binding *bookVideoBinding) error {
	if binding.AttachedAt != "" || binding.SupersededAt != "" {
		return nil
	}
	index := bookVideoEntryIndex(book, binding.EntryID)
	if index < 0 {
		return errors.New("The video is saved in Studio. Its original Book story is unavailable; restore the story to retry the link.")
	}
	entry := book.Entries[index]
	assetID := studioGenerationAssetID(binding.GenerationID)
	if !hasBookVideo(entry, assetID) {
		// Legacy bindings have no replacement snapshot and may only fill an empty story.
		if (binding.Version == 1 && len(entry.Videos) != 0) || !bookVideoReferencesMatch(entry, binding.ExpectedVideoIDs) {
			binding.SupersededAt = time.Now().UTC().Format(time.RFC3339Nano)
			return errors.New("The new video is saved in Studio. This story's video changed while it was being made, so the current choice was kept.")
		}
		if err := replaceBookVideo(root, &entry, assetID, binding.Title, ""); err != nil {
			return err
		}
		entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := saveBookEntry(root, entry); err != nil {
			return errors.New("The video is saved in Studio. Its Book link could not be saved; open the Book again to retry.")
		}
		entry.ContentVersion = bookEntryContentVersion(entry)
		book.Entries[index] = entry
		orderProjectBook(book)
	}
	binding.AssetID, binding.AttachedAt, binding.Error = assetID, time.Now().UTC().Format(time.RFC3339Nano), ""
	return nil
}

func attemptBookVideoAttachment(folder *os.Root, root *os.Root, book *projectBook, binding *bookVideoBinding) error {
	binding.LastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	err := attachCompletedBookVideo(root, book, binding)
	if err != nil {
		binding.Error = err.Error()
	}
	if saveErr := saveBookVideoBinding(folder, *binding); saveErr != nil {
		return saveErr
	}
	return err
}

// Finalize a saved attachment before unlinking it. The earlier Book write may
// have succeeded even when its binding checkpoint did not, so recovery must be
// disabled durably before the reference disappears from the Book.
func finalizeBookVideoBindingsForUnlink(root *os.Root, entryID string, video bookVideo) error {
	failure := errors.New("Could not save the video unlink. The reference is unchanged; try again.")
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failure
	}
	defer folder.Close()
	dir, err := folder.Open(".")
	if err != nil {
		return failure
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return failure
	}
	for _, name := range names {
		id, valid := strings.CutSuffix(name, ".json")
		if !valid || studioGenerationAssetID(id) != video.AssetID {
			continue
		}
		binding, err := readBookVideoBinding(folder, id)
		if err != nil {
			return failure
		}
		if binding.ProjectPath != root.Name() || binding.EntryID != entryID || binding.AttachedAt != "" || binding.SupersededAt != "" {
			continue
		}
		binding.AssetID, binding.AttachedAt, binding.Error = video.AssetID, video.CreatedAt, ""
		binding.LastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := saveBookVideoBinding(folder, binding); err != nil {
			return failure
		}
	}
	return nil
}

// A deliberate video edit supersedes earlier requests even if later edits
// restore the same reference list, such as adding and then removing a video.
// Persist this before the edit so a restart cannot revive an older request.
func supersedeBookVideoBindingsForEdit(root *os.Root, entryID string) error {
	if err := supersedeBookLaunchVideosForEdit(root, entryID); err != nil {
		return err
	}
	failure := errors.New("Could not save the video change. The reference is unchanged; try again.")
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failure
	}
	defer folder.Close()
	dir, err := folder.Open(".")
	if err != nil {
		return failure
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return failure
	}
	for _, name := range names {
		id, valid := strings.CutSuffix(name, ".json")
		if !valid || !studioGenerationID.MatchString(id) {
			continue
		}
		binding, err := readBookVideoBinding(folder, id)
		if err != nil {
			return failure
		}
		if binding.ProjectPath != root.Name() || binding.EntryID != entryID || binding.AttachedAt != "" || binding.SupersededAt != "" {
			continue
		}
		binding.SupersededAt = time.Now().UTC().Format(time.RFC3339Nano)
		binding.Error = "The story's video changed. Any result from its earlier video request stays in Studio."
		if err := saveBookVideoBinding(folder, binding); err != nil {
			return failure
		}
	}
	return nil
}

// GET and sync recover attachments saved before a shutdown or failed Book write.
// This function never contacts a provider and never restarts paid generation.
func reconcileBookVideoGenerations(root *os.Root, book *projectBook) {
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		book.Warnings = append(book.Warnings, "Saved story video links could not be checked. Open the Book again to retry.")
		return
	}
	defer folder.Close()
	dir, err := folder.Open(".")
	if err != nil {
		book.Warnings = append(book.Warnings, "Saved story video links could not be checked.")
		return
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		book.Warnings = append(book.Warnings, "Saved story video links could not be checked.")
		return
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		binding, err := readBookVideoBinding(folder, strings.TrimSuffix(name, ".json"))
		if err != nil || binding.ProjectPath != root.Name() || binding.AttachedAt != "" || bookDeletedEntryIndex(book, binding.EntryID) >= 0 {
			continue
		}
		if binding.SupersededAt != "" {
			if binding.Error != "" {
				book.Warnings = append(book.Warnings, binding.Error)
			}
			continue
		}
		if _, err := studioClipRecordPath(studioGenerationAssetID(binding.GenerationID)); err != nil {
			if binding.Error != "" {
				book.Warnings = append(book.Warnings, binding.Error)
			}
			continue
		}
		if err := attemptBookVideoAttachment(folder, root, book, &binding); err != nil {
			book.Warnings = append(book.Warnings, err.Error())
		}
	}
}

// A completed generation stays successful even when its optional Book link fails.
func completeBookVideoGeneration(generationID, assetID string) {
	if assetID != studioGenerationAssetID(generationID) {
		return
	}
	projectBookMu.Lock()
	defer projectBookMu.Unlock()
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if err != nil {
		return
	}
	defer folder.Close()
	binding, err := readBookVideoBinding(folder, generationID)
	if err != nil || binding.AttachedAt != "" || binding.SupersededAt != "" {
		return
	}
	root, name, err := openProjectBookProject(binding.ProjectPath)
	if err == nil {
		defer root.Close()
		var book projectBook
		var exists bool
		book, exists, err = readProjectBook(root, name)
		if err == nil && exists && root.Name() == binding.ProjectPath {
			_ = attemptBookVideoAttachment(folder, root, &book, &binding)
			return
		}
	}
	binding.Error = "The video is saved in Studio. Its original Project Book is unavailable; open that Book again to retry the link."
	binding.LastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = saveBookVideoBinding(folder, binding)
}
