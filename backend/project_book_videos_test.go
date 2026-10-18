package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func bookVideoTestSetup(t *testing.T) (string, bookEntry) {
	t.Helper()
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	project := bookTestProject(t)
	bookTestBuild(t, project)
	response, book := bookTestCall(t, project, "sync", nil)
	if response.Code != http.StatusOK || len(book.Entries) != 1 {
		t.Fatal(response.Code, response.Body.String())
	}
	return project, book.Entries[0]
}

func bookVideoTestAsset(t *testing.T, generationID string) studioImageRecord {
	t.Helper()
	asset, err := saveStudioAsset(studioSaveOptions{Prompt: "Test story", MediaType: "video", Source: "Test fixture", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("private-test-video-content-never-in-book")), GenerationID: generationID})
	if err != nil {
		t.Fatal(err)
	}
	return asset
}

func bookVideoTestBinding(t *testing.T, generationID string) bookVideoBinding {
	t.Helper()
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if err != nil {
		t.Fatal(err)
	}
	defer folder.Close()
	binding, err := readBookVideoBinding(folder, generationID)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func bookVideoTestBind(t *testing.T, project string, entry bookEntry, generationID string) {
	t.Helper()
	response, book := bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "title": "Garden launch", "expectedContentVersion": entry.ContentVersion})
	if response.Code != http.StatusOK || book.Entries[0].ContentVersion != entry.ContentVersion || len(book.Entries[0].Videos) != 0 {
		t.Fatal("binding changed the Book", response.Code, response.Body.String())
	}
}

func TestProjectBookVideoReferencesStaySmallAndUnlinkWithoutDeletingStudio(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	asset := bookVideoTestAsset(t, "")
	assetPath, err := studioClipRecordPath(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	imageFiles, _ := filepath.Glob(filepath.Join(project, "project-book/assets/*"))
	response, book := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": strings.ToLower(asset.ID), "title": "Garden launch", "caption": "A short walk through", "expectedContentVersion": entry.ContentVersion, "dataURL": "data:video/mp4;base64," + asset.DataBase64, "path": assetPath})
	if response.Code != http.StatusOK || len(book.Entries[0].Videos) != 1 {
		t.Fatal(response.Code, response.Body.String())
	}
	video := book.Entries[0].Videos[0]
	if video.AssetID != asset.ID || video.Title != "Garden launch" || video.Caption != "A short walk through" || video.ID == "" || video.CreatedAt == "" || book.Entries[0].Title != entry.Title || book.Entries[0].Body != entry.Body {
		t.Fatalf("bad reference or changed story: %+v", book.Entries[0])
	}
	afterFiles, _ := filepath.Glob(filepath.Join(project, "project-book/assets/*"))
	if len(imageFiles) != len(afterFiles) {
		t.Fatal("video attachment copied an asset into the Book")
	}
	if err := filepath.WalkDir(filepath.Join(project, "project-book"), func(path string, info fs.DirEntry, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, forbidden := range []string{asset.DataBase64, "private-test-video-content", "data:video", "dataBase64", assetPath, project} {
			if bytes.Contains(data, []byte(forbidden)) {
				t.Errorf("portable Book contains media or local path: %s", filepath.Base(path))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	version := book.Entries[0].ContentVersion
	response, duplicate := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID, "expectedContentVersion": entry.ContentVersion})
	if response.Code != http.StatusOK || len(duplicate.Entries[0].Videos) != 1 || duplicate.Entries[0].Videos[0].ID != video.ID || duplicate.Entries[0].ContentVersion != version {
		t.Fatal("retry created another reference or revision", response.Code, response.Body.String())
	}
	response, removed := bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": video.ID, "expectedContentVersion": version})
	if response.Code != http.StatusOK || len(removed.Entries[0].Videos) != 0 {
		t.Fatal(response.Code, response.Body.String())
	}
	after, err := os.ReadFile(assetPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("unlink changed or removed the Studio original", err)
	}
}

func TestProjectBookVideoRejectsImagesMissingAssetsAndStaleEdits(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	image, _ := studioTestAsset(t)
	for _, assetID := range []string{image.ID, "AAAAAAAA-BBBB-CCCC-DDDD-111111111111", "../../outside", ""} {
		response, _ := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": assetID})
		if response.Code != http.StatusBadRequest {
			t.Fatal("invalid video accepted", assetID, response.Code)
		}
	}
	video := bookVideoTestAsset(t, "")
	response, _ := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": video.ID, "expectedContentVersion": "stale-version"})
	if response.Code != http.StatusConflict {
		t.Fatal("stale attachment accepted", response.Code)
	}
	_, book := bookTestCall(t, project, "", nil)
	if book.Entries[0].ContentVersion != entry.ContentVersion || len(book.Entries[0].Videos) != 0 {
		t.Fatal("rejected attachments changed the story")
	}
}

func TestProjectBookVideosSurviveOldClientEditsSyncAndUnknownFields(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	paths, err := filepath.Glob(filepath.Join(project, "project-book/history", entry.ID, "*", "source.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["futureStoryDetail"] = json.RawMessage(`{"precise":9007199254740993,"state":"keep"}`)
	data, _ = json.Marshal(fields)
	if err := os.WriteFile(paths[0], data, 0644); err != nil {
		t.Fatal(err)
	}
	asset := bookVideoTestAsset(t, "")
	response, added := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID})
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	response, edited := bookTestCall(t, project, "update-entry", map[string]any{"entryId": entry.ID, "title": "Owner title", "body": "Owner revised story", "expectedContentVersion": added.Entries[0].ContentVersion})
	if response.Code != http.StatusOK || len(edited.Entries[0].Videos) != 1 || edited.Entries[0].Videos[0] != added.Entries[0].Videos[0] {
		t.Fatal("old client edit lost references", response.Code, response.Body.String())
	}
	response, synced := bookTestCall(t, project, "sync", nil)
	if response.Code != http.StatusOK || synced.Entries[0].Body != "Owner revised story" || synced.Entries[0].Videos[0] != added.Entries[0].Videos[0] {
		t.Fatal("sync lost the saved story or video", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"futureStoryDetail":{"precise":9007199254740993,"state":"keep"}`)) {
		t.Fatal("unknown newer Book metadata was lost", response.Body.String())
	}
}

func TestProjectBookVideoCompletionPreservesConcurrentStoryEditsAndUnlink(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "book-video-completion-test"
	bookVideoTestBind(t, project, entry, generationID)
	response, edited := bookTestCall(t, project, "update-entry", map[string]any{"entryId": entry.ID, "title": "A later title", "body": "This owner edit happened while the video was generating.", "expectedContentVersion": entry.ContentVersion})
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	asset := bookVideoTestAsset(t, generationID)
	store := newStudioProgressStore()
	if status := store.begin(generationID, "video"); status != http.StatusOK {
		t.Fatal(status)
	}
	progress := studioProgress{id: generationID, store: store}
	defer progress.finish()
	if err := progress.completed(summarizeStudioRecord(asset)); err != nil {
		t.Fatal(err)
	}
	binding := bookVideoTestBinding(t, generationID)
	if binding.AttachedAt == "" || binding.AssetID != asset.ID || binding.Error != "" {
		t.Fatalf("completion was not durably attached: %+v", binding)
	}
	_, completed := bookTestCall(t, project, "", nil)
	current := completed.Entries[0]
	if current.Title != edited.Entries[0].Title || current.Body != edited.Entries[0].Body || len(current.Videos) != 1 || current.Videos[0].AssetID != asset.ID {
		t.Fatalf("completion lost a concurrent edit: %+v", current)
	}
	completeBookVideoGeneration(generationID, asset.ID)
	_, again := bookTestCall(t, project, "sync", nil)
	if again.Entries[0].ContentVersion != current.ContentVersion || len(again.Entries[0].Videos) != 1 {
		t.Fatal("completion replay changed the story")
	}
	response, _ = bookTestCall(t, project, "update-entry", map[string]any{"entryId": entry.ID, "title": "Stale", "body": "Must not overwrite completion", "expectedContentVersion": edited.Entries[0].ContentVersion})
	if response.Code != http.StatusConflict {
		t.Fatal("stale edit after attachment was accepted", response.Code)
	}
	response, _ = bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": current.Videos[0].ID})
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	completeBookVideoGeneration(generationID, asset.ID)
	_, unlinked := bookTestCall(t, project, "", nil)
	if len(unlinked.Entries[0].Videos) != 0 {
		t.Fatal("completed binding reattached an explicitly unlinked video")
	}
	if _, err := studioClipRecordPath(asset.ID); err != nil {
		t.Fatal("Studio original was removed", err)
	}
}

func TestProjectBookVideoUnlinkFinalizesAnAttachmentWhoseCheckpointFailed(t *testing.T) {
	for _, failUnlink := range []bool{false, true} {
		name := "retry_after_failed_checkpoint"
		if failUnlink {
			name = "failed_finalization_preserves_reference"
		}
		t.Run(name, func(t *testing.T) {
			project, entry := bookVideoTestSetup(t)
			generationID := "book-video-failed-finalization"
			bookVideoTestBind(t, project, entry, generationID)
			asset := bookVideoTestAsset(t, generationID)
			binding := bookVideoTestBinding(t, generationID)
			root, projectName, err := openProjectBookProject(project)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			book, exists, err := readProjectBook(root, projectName)
			if err != nil || !exists {
				t.Fatal(err)
			}
			folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
			if err != nil {
				t.Fatal(err)
			}
			// The Book write succeeds, but a closed checkpoint folder forces the
			// subsequent persistence to fail without modifying the saved binding.
			folder.Close()
			if err := attemptBookVideoAttachment(folder, root, &book, &binding); err == nil || len(book.Entries[0].Videos) != 1 {
				t.Fatal("expected a saved reference and failed binding checkpoint", err)
			}
			if bookVideoTestBinding(t, generationID).AttachedAt != "" {
				t.Fatal("the failed checkpoint was unexpectedly saved")
			}
			video := book.Entries[0].Videos[0]
			if failUnlink {
				privateFolder := filepath.Join(os.Getenv("GLOWBOM_STUDIO_DIR"), bookVideoBindingFolder)
				if err := os.Chmod(privateFolder, 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(privateFolder, 0700) })
				probe := filepath.Join(privateFolder, ".permission-probe")
				if file, err := os.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600); err == nil {
					file.Close()
					os.Remove(probe)
					t.Skip("filesystem ignores directory write permissions")
				}
				response, _ := bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": video.ID})
				if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "reference is unchanged") {
					t.Fatal("unlink proceeded without durable finalization", response.Code, response.Body.String())
				}
				unchanged, _, err := readProjectBook(root, projectName)
				if err != nil || len(unchanged.Entries[0].Videos) != 1 || unchanged.Entries[0].ContentVersion != book.Entries[0].ContentVersion || bookVideoTestBinding(t, generationID).AttachedAt != "" {
					t.Fatal("failed unlink changed the saved reference or binding", err)
				}
				if err := os.Chmod(privateFolder, 0700); err != nil {
					t.Fatal(err)
				}
			}
			response, unlinked := bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": video.ID})
			if response.Code != http.StatusOK || len(unlinked.Entries[0].Videos) != 0 || bookVideoTestBinding(t, generationID).AttachedAt == "" {
				t.Fatal("unlink was undone by recovery", response.Code, response.Body.String())
			}
			completeBookVideoGeneration(generationID, asset.ID)
			_, afterRestart := bookTestCall(t, project, "", nil)
			if len(afterRestart.Entries[0].Videos) != 0 || afterRestart.Entries[0].ContentVersion != unlinked.Entries[0].ContentVersion {
				t.Fatal("later completion or restart restored the removed reference")
			}
			if _, err := studioClipRecordPath(asset.ID); err != nil {
				t.Fatal("unlink deleted the saved Studio video", err)
			}
		})
	}
}

func TestProjectBookVideoTextLimitsCountUnicodeCharacters(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	asset := bookVideoTestAsset(t, "")
	title, caption := strings.Repeat("庭", 240), strings.Repeat("花", 2000)
	for _, values := range []map[string]any{
		{"entryId": entry.ID, "assetId": asset.ID, "title": title + "庭"},
		{"entryId": entry.ID, "assetId": asset.ID, "caption": caption + "花"},
	} {
		response, _ := bookTestCall(t, project, "add-video", values)
		if response.Code != http.StatusBadRequest {
			t.Fatal("overlong Unicode metadata was accepted", response.Code)
		}
	}
	response, attached := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID, "title": title, "caption": caption})
	if response.Code != http.StatusOK || len(attached.Entries[0].Videos) != 1 || attached.Entries[0].Videos[0].Title != title || attached.Entries[0].Videos[0].Caption != caption {
		t.Fatal("valid Unicode metadata was rejected or changed", response.Code, response.Body.String())
	}
	generationID := "book-video-unicode-title"
	response, _ = bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "title": title + "庭"})
	if response.Code != http.StatusBadRequest {
		t.Fatal("overlong Unicode binding title was accepted", response.Code)
	}
	response, _ = bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "title": title, "expectedContentVersion": attached.Entries[0].ContentVersion, "replaceExistingVideo": true})
	if response.Code != http.StatusOK || bookVideoTestBinding(t, generationID).Title != title {
		t.Fatal("valid Unicode binding title was rejected", response.Code, response.Body.String())
	}
	generated := bookVideoTestAsset(t, generationID)
	completeBookVideoGeneration(generationID, generated.ID)
	_, completed := bookTestCall(t, project, "", nil)
	if len(completed.Entries[0].Videos) != 1 || completed.Entries[0].Videos[0].Title != title {
		t.Fatal("Unicode binding title did not survive completion")
	}
}

func TestProjectBookVideoBindingRecoversAfterRestartAndCannotMoveStories(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "book-video-restart-recovery"
	bookVideoTestBind(t, project, entry, generationID)
	bookVideoTestBind(t, project, entry, generationID)
	other := bookTestProject(t)
	bookTestBuild(t, other)
	_, otherBook := bookTestCall(t, other, "sync", nil)
	response, _ := bookTestCall(t, other, "bind-video-generation", map[string]any{"entryId": otherBook.Entries[0].ID, "generationId": generationID})
	if response.Code != http.StatusBadRequest {
		t.Fatal("generation could be rebound to another project", response.Code)
	}
	asset := bookVideoTestAsset(t, generationID)
	// No completion callback or in-memory generation exists. Only disk survives.
	_, unrelated := bookTestCall(t, other, "", nil)
	if len(unrelated.Entries[0].Videos) != 0 {
		t.Fatal("recovery attached to another project with the same story ID")
	}
	response, recovered := bookTestCall(t, project, "", nil)
	if response.Code != http.StatusOK || len(recovered.Entries[0].Videos) != 1 || recovered.Entries[0].Videos[0].AssetID != asset.ID || bookVideoTestBinding(t, generationID).AttachedAt == "" {
		t.Fatal("persisted binding was not recovered", response.Code, response.Body.String())
	}
	_, again := bookTestCall(t, project, "", nil)
	if again.Entries[0].ContentVersion != recovered.Entries[0].ContentVersion {
		t.Fatal("restart recovery attached twice")
	}
}

func TestProjectBookVideoFailedAttachmentRemainsRetryableWithoutFailingGeneration(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "book-video-retry-missing-story"
	bookVideoTestBind(t, project, entry, generationID)
	storyPath := filepath.Join(project, "project-book/history", entry.ID)
	hiddenPath := filepath.Join(t.TempDir(), "saved-story")
	if err := os.Rename(storyPath, hiddenPath); err != nil {
		t.Fatal(err)
	}
	asset := bookVideoTestAsset(t, generationID)
	store := newStudioProgressStore()
	if status := store.begin(generationID, "video"); status != http.StatusOK {
		t.Fatal(status)
	}
	progress := studioProgress{id: generationID, store: store}
	defer progress.finish()
	if err := progress.completed(summarizeStudioRecord(asset)); err != nil || store.read(generationID).Stage != "complete" {
		t.Fatal("Book link failure changed successful generation", err)
	}
	binding := bookVideoTestBinding(t, generationID)
	if binding.Error == "" || binding.AttachedAt != "" || binding.LastAttemptAt == "" {
		t.Fatalf("link failure was not recorded for retry: %+v", binding)
	}
	_, unavailable := bookTestCall(t, project, "", nil)
	if len(unavailable.Entries) != 0 || len(unavailable.Warnings) == 0 {
		t.Fatal("failed link was hidden or attached to another story")
	}
	if err := os.Rename(hiddenPath, storyPath); err != nil {
		t.Fatal(err)
	}
	_, recovered := bookTestCall(t, project, "sync", nil)
	if len(recovered.Entries[0].Videos) != 1 || bookVideoTestBinding(t, generationID).Error != "" {
		t.Fatal("restored story did not recover its video")
	}
}

func TestProjectBookVideoBindingMustPrecedeGenerationAndRejectStaleStory(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	response, _ := bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": "book-video-stale-version", "expectedContentVersion": "outdated"})
	if response.Code != http.StatusConflict {
		t.Fatal("stale story was bound", response.Code)
	}
	generationID := "book-video-already-started"
	if err := saveStudioGenerationCheckpoint(studioGenerationCheckpoint{State: studioProgressState{Found: true, ID: generationID, Kind: "video", Stage: "generating"}}); err != nil {
		t.Fatal(err)
	}
	response, _ = bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID})
	if response.Code != http.StatusBadRequest {
		t.Fatal("running generation was bound after starting", response.Code)
	}
}

func TestProjectBookVideosSurviveStoryRegeneration(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	bookTestWrite(t, project, "web/page.tsx", []byte("export function Garden() { return <button>Plant</button> }"))
	asset := bookVideoTestAsset(t, "")
	response, attached := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": asset.ID})
	if response.Code != http.StatusOK || !attached.Entries[0].CanWriteStory {
		t.Fatal("video attachment prevented story generation", response.Code, response.Body.String())
	}
	answer := `{"story":{"title":"Garden launch","body":"The project now includes a garden page with a visible Plant button. The saved page gives visitors a clear place to begin the garden experience."}}`
	var calls atomic.Int32
	server := bookStoryModelServer(t, &answer, &calls)
	defer server.Close()
	t.Setenv("OPENCODE_URL", server.URL)
	body, _ := json.Marshal(map[string]string{"projectPath": project, "entryId": entry.ID, "model": "provider/model"})
	response = httptest.NewRecorder()
	projectBookStoryHandler(response, httptest.NewRequest("POST", "/project-book/story", bytes.NewReader(body)))
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatal(response.Code, response.Body.String())
	}
	_, written := bookTestCall(t, project, "", nil)
	if written.Entries[0].Title != "Garden launch" || len(written.Entries[0].Videos) != 1 || written.Entries[0].Videos[0] != attached.Entries[0].Videos[0] {
		t.Fatal("story generation lost the video reference")
	}
}
