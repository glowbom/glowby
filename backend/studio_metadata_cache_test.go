package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStudioMetadataCacheFollowsChangedAndDeletedFiles(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	saved, err := saveStudioAsset(studioSaveOptions{Prompt: "first take", DataURI: "data:audio/mpeg;base64," + base64.StdEncoding.EncodeToString([]byte("ID3audio")), MediaType: "audio", AudioType: "voice"})
	if err != nil {
		t.Fatal(err)
	}
	list := func() []studioAudioSummary {
		items, _, err := listStudioAudioPage("", 0, 48)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	if items := list(); len(items) != 1 || items[0].Prompt != "first take" {
		t.Fatalf("first list = %+v", items)
	}
	dir, _ := studioAssetsDirectory()
	matches, _ := filepath.Glob(filepath.Join(dir, "*"+saved.ID+"*.json"))
	if len(matches) != 1 {
		t.Fatalf("record file not found: %v", matches)
	}
	data, _ := os.ReadFile(matches[0])
	var record map[string]any
	_ = json.Unmarshal(data, &record)
	record["prompt"] = "second, longer take"
	data, _ = json.Marshal(record)
	if err := os.WriteFile(matches[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	if items := list(); len(items) != 1 || items[0].Prompt != "second, longer take" {
		t.Fatalf("a changed record should be read again: %+v", items)
	}
	if err := os.Remove(matches[0]); err != nil {
		t.Fatal(err)
	}
	if items := list(); len(items) != 0 {
		t.Fatalf("a deleted record should disappear: %+v", items)
	}
	studioMetadataCache.Lock()
	_, cached := studioMetadataCache.files[matches[0]]
	studioMetadataCache.Unlock()
	if cached {
		t.Fatalf("the deleted file should be pruned from the cache")
	}
}

// The Mac app rewrites a record in place, which leaves the folder's modification time unchanged.
func TestStudioCatalogShowsRecordsRewrittenInPlace(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	saved, err := saveStudioAsset(studioSaveOptions{Prompt: "Clip", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("fake-mp4")), MediaType: "video"})
	if err != nil {
		t.Fatal(err)
	}
	_, videos, _, _, err := listStudioCatalogPage(0, 24, 0, 24)
	if err != nil || len(videos) != 1 || videos[0].Prompt != "Clip" {
		t.Fatalf("first list = %+v, %v", videos, err)
	}
	dir, _ := studioAssetsDirectory()
	before, _ := os.Stat(dir)
	matches, _ := filepath.Glob(filepath.Join(dir, "*"+saved.ID+"*.json"))
	data, _ := os.ReadFile(matches[0])
	var record map[string]any
	_ = json.Unmarshal(data, &record)
	record["prompt"] = "Renamed in the Mac app"
	data, _ = json.Marshal(record)
	if err := os.WriteFile(matches[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.Stat(dir); !after.ModTime().Equal(before.ModTime()) {
		t.Skip("this file system changes the folder time on an in-place write")
	}
	_, videos, _, _, err = listStudioCatalogPage(0, 24, 0, 24)
	if err != nil || len(videos) != 1 || videos[0].Prompt != "Renamed in the Mac app" {
		t.Fatalf("an in-place rewrite should show up: %+v, %v", videos, err)
	}
}
