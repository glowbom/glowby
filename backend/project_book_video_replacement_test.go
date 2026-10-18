package main

import (
	"net/http"
	"testing"
)

func bookVideoTestSaveEntry(t *testing.T, project string, entry bookEntry) {
	t.Helper()
	root, _, err := openProjectBookProject(project)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := saveBookEntry(root, entry); err != nil {
		t.Fatal(err)
	}
}

func TestProjectBookVideoReplacementRequiresExplicitCurrentRevision(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	old, replacement := bookVideoTestAsset(t, ""), bookVideoTestAsset(t, "")
	_, attached := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": old.ID})
	current := attached.Entries[0]
	for _, test := range []struct {
		action, version string
		status          int
	}{
		{"add-video", current.ContentVersion, http.StatusBadRequest},
		{"replace-video", "", http.StatusBadRequest},
		{"replace-video", entry.ContentVersion, http.StatusConflict},
	} {
		response, _ := bookTestCall(t, project, test.action, map[string]any{"entryId": entry.ID, "assetId": replacement.ID, "expectedContentVersion": test.version})
		if response.Code != test.status {
			t.Fatal(test.action, response.Code, response.Body.String())
		}
	}
	image, _ := studioTestAsset(t)
	response, _ := bookTestCall(t, project, "replace-video", map[string]any{"entryId": entry.ID, "assetId": image.ID, "expectedContentVersion": current.ContentVersion})
	if response.Code != http.StatusBadRequest {
		t.Fatal("replacement accepted an image", response.Code)
	}
	_, unchanged := bookTestCall(t, project, "", nil)
	if unchanged.Entries[0].ContentVersion != current.ContentVersion {
		t.Fatal("rejected replacement changed the story")
	}
	values := map[string]any{"entryId": entry.ID, "assetId": replacement.ID, "expectedContentVersion": current.ContentVersion, "title": "New video"}
	response, replaced := bookTestCall(t, project, "replace-video", values)
	if response.Code != http.StatusOK || len(replaced.Entries[0].Videos) != 1 || replaced.Entries[0].Videos[0].AssetID != replacement.ID || replaced.Entries[0].Body != entry.Body {
		t.Fatal("replacement did not keep one video and the story", response.Code, response.Body.String())
	}
	response, retried := bookTestCall(t, project, "replace-video", values)
	if response.Code != http.StatusOK || retried.Entries[0].ContentVersion != replaced.Entries[0].ContentVersion {
		t.Fatal("replacement retry created another revision")
	}
	for _, id := range []string{old.ID, replacement.ID} {
		if _, err := savedBookStudioVideo(id); err != nil {
			t.Fatal("replacement removed a Studio video", err)
		}
	}
}

func TestProjectBookVideosPreserveLegacyMultipleReferencesUntilReplacement(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	assets := []studioImageRecord{bookVideoTestAsset(t, ""), bookVideoTestAsset(t, "")}
	for _, asset := range assets {
		var reference bookEntry
		if err := appendBookVideo(&reference, asset.ID, "Older video", ""); err != nil {
			t.Fatal(err)
		}
		entry.Videos = append(entry.Videos, reference.Videos[0])
	}
	bookVideoTestSaveEntry(t, project, entry)
	response, synced := bookTestCall(t, project, "sync", nil)
	if response.Code != http.StatusOK || len(synced.Entries[0].Videos) != 2 {
		t.Fatal("legacy references were removed while reading")
	}
	response, edited := bookTestCall(t, project, "update-entry", map[string]any{"entryId": entry.ID, "title": "Edited story", "body": entry.Body, "expectedContentVersion": synced.Entries[0].ContentVersion})
	if response.Code != http.StatusOK || len(edited.Entries[0].Videos) != 2 {
		t.Fatal("legacy references were removed by a text edit")
	}
	response, replaced := bookTestCall(t, project, "replace-video", map[string]any{"entryId": entry.ID, "assetId": assets[1].ID, "expectedContentVersion": edited.Entries[0].ContentVersion})
	if response.Code != http.StatusOK || len(replaced.Entries[0].Videos) != 1 || replaced.Entries[0].Videos[0] != entry.Videos[1] {
		t.Fatal("explicit replacement did not retain the selected legacy reference", response.Code)
	}
	for _, asset := range assets {
		if _, err := savedBookStudioVideo(asset.ID); err != nil {
			t.Fatal("legacy replacement removed a Studio video", err)
		}
	}
}

func TestProjectBookVideoGeneratedReplacementKeepsOldVideoUntilSuccess(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	old := bookVideoTestAsset(t, "")
	_, attached := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": old.ID})
	current := attached.Entries[0]
	generationID := "book-video-replacement-recovery"
	for _, values := range []map[string]any{
		{"entryId": entry.ID, "generationId": generationID, "expectedContentVersion": current.ContentVersion},
		{"entryId": entry.ID, "generationId": generationID, "replaceExistingVideo": true},
	} {
		response, _ := bookTestCall(t, project, "bind-video-generation", values)
		if response.Code != http.StatusBadRequest {
			t.Fatal("replacement bound without an explicit current review", response.Code)
		}
	}
	response, bound := bookTestCall(t, project, "bind-video-generation", map[string]any{"entryId": entry.ID, "generationId": generationID, "title": "Replacement", "replaceExistingVideo": true, "expectedContentVersion": current.ContentVersion})
	if response.Code != http.StatusOK || bound.Entries[0].ContentVersion != current.ContentVersion {
		t.Fatal("binding removed the old video", response.Code, response.Body.String())
	}
	binding := bookVideoTestBinding(t, generationID)
	if binding.Version != 2 || len(binding.ExpectedVideoIDs) != 1 || binding.ExpectedVideoIDs[0] != current.Videos[0].ID {
		t.Fatal("binding did not save the expected reference")
	}
	response, edited := bookTestCall(t, project, "update-entry", map[string]any{"entryId": entry.ID, "title": "New text during generation", "body": "Owner edits stay intact.", "expectedContentVersion": current.ContentVersion})
	if response.Code != http.StatusOK || edited.Entries[0].Videos[0].AssetID != old.ID {
		t.Fatal("in-progress replacement removed the old video")
	}
	generated := bookVideoTestAsset(t, generationID)
	// No completion callback runs: opening the Book after restart must recover.
	_, completed := bookTestCall(t, project, "", nil)
	final := completed.Entries[0]
	if len(final.Videos) != 1 || final.Videos[0].AssetID != generated.ID || final.Title != edited.Entries[0].Title || final.Body != edited.Entries[0].Body || bookVideoTestBinding(t, generationID).AttachedAt == "" {
		t.Fatal("recovered replacement lost the result or concurrent text")
	}
	completeBookVideoGeneration(generationID, generated.ID)
	_, repeated := bookTestCall(t, project, "", nil)
	if repeated.Entries[0].ContentVersion != final.ContentVersion {
		t.Fatal("replacement recovery wrote twice")
	}
	if _, err := savedBookStudioVideo(old.ID); err != nil {
		t.Fatal("generated replacement deleted old Studio media", err)
	}
}

func TestProjectBookVideoLateCompletionCannotUndoManualChoiceOrRemoval(t *testing.T) {
	for _, removeLater := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep_new_choice", true: "keep_removal"}[removeLater], func(t *testing.T) {
			project, entry := bookVideoTestSetup(t)
			generationID := "book-video-late-original-request"
			bookVideoTestBind(t, project, entry, generationID)
			manual := bookVideoTestAsset(t, "")
			_, chosen := bookTestCall(t, project, "add-video", map[string]any{"entryId": entry.ID, "assetId": manual.ID})
			if bookVideoTestBinding(t, generationID).SupersededAt == "" {
				t.Fatal("manual choice did not durably supersede the earlier request")
			}
			if removeLater {
				response, _ := bookTestCall(t, project, "remove-video", map[string]any{"entryId": entry.ID, "videoId": chosen.Entries[0].Videos[0].ID})
				if response.Code != http.StatusOK {
					t.Fatal(response.Code, response.Body.String())
				}
			}
			generated := bookVideoTestAsset(t, generationID)
			completeBookVideoGeneration(generationID, generated.ID)
			_, current := bookTestCall(t, project, "sync", nil)
			if removeLater && len(current.Entries[0].Videos) != 0 || !removeLater && (len(current.Entries[0].Videos) != 1 || current.Entries[0].Videos[0].AssetID != manual.ID) {
				t.Fatal("late completion replaced a newer manual choice")
			}
			if _, err := savedBookStudioVideo(generated.ID); err != nil {
				t.Fatal("superseded generated video was not kept in Studio", err)
			}
		})
	}
}

func TestProjectBookVideoCompletionChecksSnapshotAfterExternalReferenceChange(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "book-video-external-reference-change"
	bookVideoTestBind(t, project, entry, generationID)
	manual := bookVideoTestAsset(t, "")
	if err := appendBookVideo(&entry, manual.ID, "External choice", ""); err != nil {
		t.Fatal(err)
	}
	bookVideoTestSaveEntry(t, project, entry)
	generated := bookVideoTestAsset(t, generationID)
	completeBookVideoGeneration(generationID, generated.ID)
	_, current := bookTestCall(t, project, "", nil)
	binding := bookVideoTestBinding(t, generationID)
	if len(current.Entries[0].Videos) != 1 || current.Entries[0].Videos[0].AssetID != manual.ID || binding.SupersededAt == "" || binding.AttachedAt != "" || len(current.Warnings) == 0 {
		t.Fatal("completion ignored the reference snapshot")
	}
}

func TestProjectBookVideoLegacyBindingCannotReplaceExistingVideo(t *testing.T) {
	project, entry := bookVideoTestSetup(t)
	generationID := "book-video-legacy-bound-generation"
	bookVideoTestBind(t, project, entry, generationID)
	binding := bookVideoTestBinding(t, generationID)
	binding.Version, binding.ExpectedVideoIDs = 1, nil
	folder, err := openStudioLibraryFolder(bookVideoBindingFolder, false)
	if err != nil {
		t.Fatal(err)
	}
	defer folder.Close()
	if err := saveBookVideoBinding(folder, binding); err != nil {
		t.Fatal(err)
	}
	manual := bookVideoTestAsset(t, "")
	if err := appendBookVideo(&entry, manual.ID, "Existing video", ""); err != nil {
		t.Fatal(err)
	}
	bookVideoTestSaveEntry(t, project, entry)
	generated := bookVideoTestAsset(t, generationID)
	completeBookVideoGeneration(generationID, generated.ID)
	_, current := bookTestCall(t, project, "", nil)
	if len(current.Entries[0].Videos) != 1 || current.Entries[0].Videos[0].AssetID != manual.ID || bookVideoTestBinding(t, generationID).SupersededAt == "" {
		t.Fatal("legacy binding replaced an existing video without review")
	}
}
