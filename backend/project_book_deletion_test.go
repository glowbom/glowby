package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

func bookDeleteTestTransition(t *testing.T, project, action string, entry bookEntry) projectBook {
	t.Helper()
	w, book := bookTestCall(t, project, action, map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusOK {
		t.Fatal(action, w.Code, w.Body.String())
	}
	return book
}

func TestProjectBookDeleteRestorePreservesFilesAndSurvivesSync(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	asset := bookVideoTestAsset(t, "")
	w, book := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	entry = book.Entries[0]
	root, _, err := openProjectBookProject(project)
	if err != nil {
		t.Fatal(err)
	}
	entry.extraFields = map[string]json.RawMessage{"futureDetail": json.RawMessage(`{"precise":9007199254740993}`)}
	if err := saveBookEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	root.Close()
	_, book = bookTestCall(t, project, "", nil)
	entry = book.Entries[0]
	files := map[string][]byte{}
	if err := filepath.WalkDir(project, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = data
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assetPath, err := studioClipRecordPath(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	files[assetPath], err = os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	book = bookDeleteTestTransition(t, project, "delete-entry", entry)
	if len(book.Entries) != 0 || len(book.DeletedEntries) != 1 {
		t.Fatal("story was not hidden")
	}
	deleted := book.DeletedEntries[0]
	if deleted.DeletedAt == "" || deleted.ContentVersion == entry.ContentVersion || book.UpdatedAt != deleted.UpdatedAt || deleted.CanWriteStory || deleted.Body != entry.Body || deleted.Videos[0] != entry.Videos[0] {
		t.Fatal("deleted story lost content or revision identity")
	}
	for _, action := range []string{"", "sync", "sync"} {
		w, book = bookTestCall(t, project, action, nil)
		if w.Code != http.StatusOK || len(book.Entries) != 0 || len(book.DeletedEntries) != 1 || book.DeletedEntries[0].ContentVersion != deleted.ContentVersion {
			t.Fatal("read or sync revived story", action, w.Code)
		}
	}
	book = bookDeleteTestTransition(t, project, "delete-entry", entry)
	if book.DeletedEntries[0].ContentVersion != deleted.ContentVersion {
		t.Fatal("delete retry made another revision")
	}
	for _, action := range []string{"update-entry", "add-video", "bind-video-generation"} {
		w, _ = bookTestCall(t, project, action, map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion, "title": "Stale change", "body": "Stale body", "assetId": asset.ID, "generationId": "deleted-story-generation"})
		if w.Code != http.StatusConflict {
			t.Fatal("stale mutation not rejected", action, w.Code)
		}
	}
	for _, action := range []string{"delete-entry", "restore-entry"} {
		w, _ = bookTestCall(t, project, action, map[string]any{"entryId": entry.ID})
		if w.Code != http.StatusBadRequest {
			t.Fatal("missing version accepted", action)
		}
	}
	w, _ = bookTestCall(t, project, "restore-entry", map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusConflict {
		t.Fatal("stale restore accepted")
	}
	book = bookDeleteTestTransition(t, project, "restore-entry", deleted)
	restored := book.Entries[0]
	if len(book.DeletedEntries) != 0 || restored.DeletedAt != "" || restored.ContentVersion == entry.ContentVersion || restored.ContentVersion == deleted.ContentVersion || restored.Body != entry.Body || restored.Videos[0] != entry.Videos[0] || string(restored.extraFields["futureDetail"]) != string(entry.extraFields["futureDetail"]) {
		t.Fatal("restore lost content or reused a stale version")
	}
	book = bookDeleteTestTransition(t, project, "restore-entry", deleted)
	if book.Entries[0].ContentVersion != restored.ContentVersion {
		t.Fatal("restore retry made another revision")
	}
	w, _ = bookTestCall(t, project, "delete-entry", map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusConflict {
		t.Fatal("old delete removed restored story")
	}
	for path, before := range files {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("deletion or restore changed an existing file", path, err)
		}
	}
}

func TestProjectBookDeleteBlocksPendingVideoAfterRestore(t *testing.T) {
	project, entry, plan := bookLaunchFixture(t)
	generationID := "deleted-story-pending-video"
	w, _ := bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "expectedContentVersion": entry.ContentVersion, "replaceExistingVideo": true})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	book := bookDeleteTestTransition(t, project, "delete-entry", entry)
	if len(book.Warnings) != 0 {
		t.Fatal("deleted story left a global pending-video warning", book.Warnings)
	}
	if bookVideoTestBinding(t, generationID).SupersededAt == "" || editBookLaunchReceipt(t, plan.RequestID, nil).Status != "superseded" {
		t.Fatal("pending jobs were not durably superseded")
	}
	book = bookDeleteTestTransition(t, project, "restore-entry", book.DeletedEntries[0])
	restored := book.Entries[0]
	asset := bookVideoTestAsset(t, generationID)
	completeBookVideoGeneration(generationID, asset.ID)
	bookLaunchFinishedOutput(t, plan, true)
	result, err := reconcileBookLaunchVideo(context.Background(), project, plan.RequestID)
	if err != nil || result.Status != "superseded" || result.Asset == nil {
		t.Fatal("late graphics were not kept in Studio", err)
	}
	_, book = bookTestCall(t, project, "sync", nil)
	if len(book.Entries) != 1 || book.Entries[0].ContentVersion != restored.ContentVersion || book.Entries[0].Videos[0] != entry.Videos[0] {
		t.Fatal("late result changed restored story")
	}
	w, _ = bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "expectedContentVersion": restored.ContentVersion})
	if w.Code != http.StatusConflict {
		t.Fatal("superseded request could be reused", w.Code)
	}
}

func TestProjectBookDeleteFailsBeforeRemovalWhenBindingsCannotSave(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "delete-binding-write-failure"
	bookVideoTestBind(t, project, entry, generationID)
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if err != nil {
		t.Fatal(err)
	}
	path := folder.Name()
	folder.Close()
	if err := os.Chmod(path, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0700) })
	w, _ := bookTestCall(t, project, "delete-entry", map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "was not deleted") {
		t.Fatal("failed binding write did not block deletion", w.Code, w.Body.String())
	}
	_, book := bookTestCall(t, project, "", nil)
	if len(book.Entries) != 1 || len(book.DeletedEntries) != 0 || book.Entries[0].ContentVersion != entry.ContentVersion || bookVideoTestBinding(t, generationID).SupersededAt != "" {
		t.Fatal("failed delete changed story or binding")
	}
}

func TestProjectBookDeleteFailedRevisionLeavesStoryActive(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	path := filepath.Join(project, "project-book/history", entry.ID)
	if err := os.Chmod(path, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0755) })
	w, _ := bookTestCall(t, project, "delete-entry", map[string]any{"entryId": entry.ID, "expectedContentVersion": entry.ContentVersion})
	if w.Code != http.StatusBadRequest {
		t.Fatal("failed revision unexpectedly deleted story", w.Code)
	}
	_, book := bookTestCall(t, project, "", nil)
	if len(book.DeletedEntries) != 0 || len(book.Entries) != 1 || book.Entries[0].ContentVersion != entry.ContentVersion {
		t.Fatal("failed revision hid story")
	}
}

func TestProjectBookDeletedEntriesCountTowardHistoryLimit(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	root, _, err := openProjectBookProject(project)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	book := projectBook{Entries: []bookEntry{}}
	for i := 0; i < projectBookEntryLimit; i++ {
		book.DeletedEntries = append(book.DeletedEntries, bookEntry{ID: fmt.Sprintf("deleted-%d", i)})
	}
	if err := syncProjectBook(root, &book); err == nil || len(book.Entries) != 0 {
		t.Fatal("deleted entries bypassed total history limit")
	}
	book.DeletedEntries[0] = entry
	book.DeletedEntries[0].DeletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := syncProjectBook(root, &book); err != nil || len(book.Entries) != 0 {
		t.Fatal("sync did not skip known deleted source", err)
	}
}

func TestProjectBookDeletedStoryIsAbsentFromCompanionAndMedia(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	book := bookDeleteTestTransition(t, project, "delete-entry", entry)
	project, _ = filepath.EvalSymlinks(project)
	s := testCompanion(t, http.NotFoundHandler())
	s.projects["shared"] = companionProject{ID: "shared", Name: "Garden", path: project}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodGet, "/projects/shared/book", ""))
	if w.Code != http.StatusOK || bytes.Contains(w.Body.Bytes(), []byte(entry.ID)) || bytes.Contains(w.Body.Bytes(), []byte("deletedEntries")) {
		t.Fatal("companion exposed deleted story", w.Code)
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodGet, "/projects/shared/book/media/"+entry.Images[0].ID, ""))
	if w.Code != http.StatusNotFound {
		t.Fatal("deleted media remained exposed", w.Code)
	}
	bookDeleteTestTransition(t, project, "restore-entry", book.DeletedEntries[0])
	w = httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodGet, "/projects/shared/book/media/"+entry.Images[0].ID, ""))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), bookTestPNG(t)) {
		t.Fatal("restore lost the original image", w.Code)
	}
}

func TestProjectBookDeletedStoryRejectsLateStoryGeneration(t *testing.T) {
	for _, preview := range []bool{false, true} {
		for _, restore := range []bool{false, true} {
			t.Run(fmt.Sprintf("preview_%t_restore_%t", preview, restore), func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				project, entry := bookVideoTestSetup(t)
				bookTestWrite(t, project, "web/page.tsx", []byte("export function Garden() { return <button>Plant</button> }"))
				answer := `{"story":{"title":"A new garden story","body":"The project now includes a garden page with a visible Plant button. This gives visitors a clear starting point for the garden experience."}}`
				var calls atomic.Int32
				model := bookStoryModelServer(t, &answer, &calls)
				defer model.Close()
				started, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/session/book-session/message" {
						once.Do(func() { close(started) })
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
					}
					model.Config.Handler.ServeHTTP(w, r)
				}))
				defer server.Close()
				t.Setenv("OPENCODE_URL", server.URL)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				type outcome struct {
					book    projectBook
					changed bool
					err     error
				}
				done := make(chan outcome, 1)
				go func() {
					book, changed, err := writeProjectBookStory(ctx, project, entry.ID, bookStoryOptions{Preview: preview, Style: "announcement"})
					done <- outcome{book, changed, err}
				}()
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("local model did not start")
				}
				book := bookDeleteTestTransition(t, project, "delete-entry", entry)
				if restore {
					book = bookDeleteTestTransition(t, project, "restore-entry", book.DeletedEntries[0])
				}
				close(release)
				result := <-done
				if result.err == nil || result.changed || len(result.book.Entries) != len(book.Entries) {
					t.Fatal("late generation returned an active stale story", result.err)
				}
				if restore && (!errors.Is(result.err, errProjectBookConflict) || result.book.Entries[0].ContentVersion != book.Entries[0].ContentVersion) {
					t.Fatal("late generation overwrote restored story", result.err)
				}
				_, after := bookTestCall(t, project, "", nil)
				if restore && after.Entries[0].Body != entry.Body {
					t.Fatal("late generated text was committed")
				}
			})
		}
	}
}
