package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func studioLibraryRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(method, path, bytes.NewReader(data)))
	return response
}

func TestStudioFavoritePreservesNativeFieldsAndMedia(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	asset, _ := studioTestAsset(t)
	path, fields, _, err := studioAssetRaw(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields["isFavorite"] = json.RawMessage(`true`)
	fields["unknownNativeField"] = json.RawMessage(`{"precise":9007199254740993,"keep":"native"}`)
	fields["tags"] = json.RawMessage(`["saved","favorite"]`)
	fields["notes"] = json.RawMessage(`"Original notes"`)
	data, _ := json.Marshal(fields)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	images, _, _, _, err := listStudioCatalogPage(0, 24, 0, 0)
	if err != nil || len(images) != 1 || !images[0].IsFavorite {
		t.Fatalf("native favorite is missing: %+v %v", images, err)
	}
	for _, favorite := range []bool{false, true} {
		response := studioLibraryRequest(t, studioAssetFavoriteHandler, http.MethodPatch, "/studio/assets/favorite", map[string]any{"id": strings.ToLower(asset.ID), "isFavorite": favorite})
		if response.Code != http.StatusOK {
			t.Fatalf("favorite response: %d %s", response.Code, response.Body.String())
		}
		_, retained, _, err := studioAssetRaw(asset.ID)
		if err != nil {
			t.Fatal(err)
		}
		for key, original := range fields {
			if key != "isFavorite" && !bytes.Equal(original, retained[key]) {
				t.Fatalf("favorite changed %s", key)
			}
		}
		if string(retained["isFavorite"]) != fmt.Sprint(favorite) {
			t.Fatalf("favorite was not saved: %s", retained["isFavorite"])
		}
		images, _, _, _, err = listStudioCatalogPage(0, 24, 0, 0)
		if err != nil || len(images) != 1 || images[0].IsFavorite != favorite {
			t.Fatalf("stale favorite listing: %+v %v", images, err)
		}
	}
}

func TestStudioFavoritePaginationScansBeyondFirstUnfilteredPage(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	dir, _ := studioAssetsDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	projectID := strings.ToUpper(randomUUIDString())
	for _, mediaType := range []string{"image", "video", "audio"} {
		for i := range 60 {
			id := strings.ToUpper(randomUUIDString())
			record := studioImageRecord{ID: id, Timestamp: fmt.Sprintf("2026-10-%02dT%02d:00:00Z", 1+i/24, i%24), MediaType: mediaType, DataBase64: "aGVsbG8=", IsFavorite: i == 1 || i == 2, UsedInProjects: []string{projectID}}
			data, _ := json.Marshal(record)
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d_%s.json", 1000+i, id)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, project := range []string{"", projectID} {
		for offset := range 3 {
			response := studioLibraryRequest(t, studioAssetsHandler, http.MethodGet, fmt.Sprintf("/studio/assets?favoritesOnly=true&imageLimit=1&videoLimit=1&imageOffset=%d&videoOffset=%d&projectId=%s", offset, offset, project), nil)
			var result struct {
				Images             []studioImageSummary `json:"images"`
				Videos             []studioImageSummary `json:"videos"`
				HasMoreImages      bool                 `json:"hasMoreImages"`
				HasMoreVideos      bool                 `json:"hasMoreVideos"`
				FavoritesSupported bool                 `json:"favoritesSupported"`
			}
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil {
				t.Fatalf("favorite page: %d %s", response.Code, response.Body.String())
			}
			want := 1
			if offset == 2 {
				want = 0
			}
			if !result.FavoritesSupported || len(result.Images) != want || len(result.Videos) != want || result.HasMoreImages != (offset == 0) || result.HasMoreVideos != (offset == 0) {
				t.Fatalf("wrong page %d: %+v", offset, result)
			}
			for _, item := range append(result.Images, result.Videos...) {
				if !item.IsFavorite {
					t.Fatal("favorite page included another asset")
				}
			}
			audioResponse := studioLibraryRequest(t, studioAudioHandler, http.MethodGet, fmt.Sprintf("/studio/audio?favoritesOnly=true&offset=%d&limit=1&projectId=%s", offset, project), nil)
			var audio struct {
				Audio              []studioAudioSummary `json:"audio"`
				HasMore            bool                 `json:"hasMore"`
				FavoritesSupported bool                 `json:"favoritesSupported"`
			}
			if audioResponse.Code != 200 || json.Unmarshal(audioResponse.Body.Bytes(), &audio) != nil || len(audio.Audio) != want || audio.HasMore != (offset == 0) || !audio.FavoritesSupported {
				t.Fatalf("wrong audio page %d: %s", offset, audioResponse.Body.String())
			}
		}
	}
}

func TestStudioDeleteMediaKeepsExportedProjectCopy(t *testing.T) {
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	for _, mediaType := range []string{"image", "video", "audio"} {
		t.Run(mediaType, func(t *testing.T) {
			t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
			asset, err := saveStudioAsset(studioSaveOptions{MediaType: mediaType, DataURI: "data:application/octet-stream;base64,aGVsbG8="})
			if err != nil {
				t.Fatal(err)
			}
			root := studioTestProject(t, "Keep exported files")
			project, err := registerStudioProject(root)
			if err != nil {
				t.Fatal(err)
			}
			path, fields, _, err := studioAssetRaw(asset.ID)
			if err != nil {
				t.Fatal(err)
			}
			fields["usedInProjects"], _ = json.Marshal([]string{project.ID})
			data, _ := json.Marshal(fields)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			copyPath := filepath.Join(root, "prototype", "assets", "saved-"+mediaType)
			if err := os.WriteFile(copyPath, []byte("independent project copy"), 0600); err != nil {
				t.Fatal(err)
			}
			_, _, _ = allStudioProjectAssets()
			_, _, _ = listStudioAudioPage("", 0, 24)
			response := studioLibraryRequest(t, studioAssetsHandler, http.MethodDelete, "/studio/assets?id="+strings.ToLower(asset.ID), nil)
			if response.Code != 200 {
				t.Fatalf("delete: %d %s", response.Code, response.Body.String())
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("Studio media record remains")
			}
			copied, err := os.ReadFile(copyPath)
			if err != nil || string(copied) != "independent project copy" {
				t.Fatal("exported project file changed")
			}
			images, videos, err := allStudioProjectAssets()
			audio, _, audioErr := listStudioAudioPage("", 0, 24)
			if err != nil || audioErr != nil || len(images)+len(videos)+len(audio) != 0 {
				t.Fatal("deleted media remains in cached library")
			}
		})
	}
}

func TestStudioRemoveProjectPreservesFilesAndStaysRemoved(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	asset, _ := studioTestAsset(t)
	root := studioTestProject(t, "Keep my project")
	_, project, relative, err := useStudioImage(asset.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	studio, _ := studioRootDirectory()
	nativeDir := filepath.Join(studio, "Projects")
	if err := os.MkdirAll(nativeDir, 0700); err != nil {
		t.Fatal(err)
	}
	native, _ := json.Marshal(map[string]any{"id": project.ID, "projectName": project.Name, "exportedProjectPath": root, "isFavorite": true, "history": []string{"Keep this saved work"}})
	nativePath := filepath.Join(nativeDir, project.ID+".json")
	if err := os.WriteFile(nativePath, native, 0600); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, path := range []string{filepath.Join(root, "glowbom.json"), filepath.Join(root, ".glowbom", "studio.json"), filepath.Join(root, relative), nativePath} {
		before[path], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	projects, err := listStudioProjects()
	if err != nil || len(projects) != 1 || !projects[0].IsFavorite {
		t.Fatalf("native project favorite lost: %+v %v", projects, err)
	}
	response := studioLibraryRequest(t, studioProjectsHandler, http.MethodDelete, "/studio/projects?id="+project.ID, nil)
	if response.Code != 200 {
		t.Fatalf("remove project: %d %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(studio, "DesktopProjects", project.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("desktop reference remains")
	}
	for range 2 {
		registered, err := registerStudioProject(root)
		if err != nil || !registered.Removed {
			t.Fatalf("automatic registration restored removed project: %+v %v", registered, err)
		}
		for _, list := range []func() ([]studioProjectSummary, error){listStudioProjects, listRegisteredStudioProjects} {
			projects, err := list()
			if err != nil || len(projects) != 0 {
				t.Fatalf("removed project reappeared: %+v %v", projects, err)
			}
		}
	}
	for path, original := range before {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("removal changed project data: %s", path)
		}
	}
	response = studioLibraryRequest(t, studioProjectsHandler, http.MethodPost, "/studio/projects", map[string]any{"path": root, "restore": true})
	if response.Code != 200 {
		t.Fatalf("restore project: %d %s", response.Code, response.Body.String())
	}
	projects, err = listStudioProjects()
	if err != nil || len(projects) != 1 || projects[0].Removed {
		t.Fatalf("explicit restore failed: %+v %v", projects, err)
	}
}

func TestStudioLibraryMutationsRejectSymlinksAndUnauthorizedRequests(t *testing.T) {
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	for _, kind := range []string{"record", "folder"} {
		t.Run(kind, func(t *testing.T) {
			studio := t.TempDir()
			t.Setenv("GLOWBOM_STUDIO_DIR", studio)
			outside := t.TempDir()
			id := strings.ToUpper(randomUUIDString())
			data := []byte(fmt.Sprintf(`{"id":%q,"mediaType":"image","dataBase64":"aGVsbG8=","isFavorite":false}`, id))
			outsideFile := filepath.Join(outside, id+".json")
			if err := os.WriteFile(outsideFile, data, 0600); err != nil {
				t.Fatal(err)
			}
			assets := filepath.Join(studio, "Assets")
			if kind == "folder" {
				if err := os.Symlink(outside, assets); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(assets, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outsideFile, filepath.Join(assets, id+".json")); err != nil {
					t.Fatal(err)
				}
			}
			response := studioLibraryRequest(t, studioAssetFavoriteHandler, http.MethodPatch, "/studio/assets/favorite", map[string]any{"id": id, "isFavorite": true})
			if response.Code < 400 {
				t.Fatal("favorite followed a symlink")
			}
			response = studioLibraryRequest(t, studioAssetsHandler, http.MethodDelete, "/studio/assets?id="+id, nil)
			if response.Code < 400 {
				t.Fatal("delete followed a symlink")
			}
			retained, err := os.ReadFile(outsideFile)
			if err != nil || !bytes.Equal(retained, data) {
				t.Fatal("mutation touched outside file")
			}
		})
	}
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	asset, _ := studioTestAsset(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-server-token")
	response := studioLibraryRequest(t, studioAssetsHandler, http.MethodDelete, "/studio/assets?id="+asset.ID, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized delete: %d", response.Code)
	}
	request := httptest.NewRequest(http.MethodDelete, "/studio/assets?id="+asset.ID, nil)
	request.Header.Set("Authorization", "Bearer fixture-server-token")
	request.Header.Set("Origin", "https://untrusted.example")
	response = httptest.NewRecorder()
	studioAssetsHandler(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin delete: %d", response.Code)
	}
	if _, err := findStudioAsset(asset.ID); err != nil {
		t.Fatal("unauthorized request removed media")
	}
}

func TestStudioDeletedMediaDoesNotReappearInCompletedRequests(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	asset, _ := studioTestAsset(t)
	summary := summarizeStudioRecord(asset)
	raw, _ := json.Marshal(summary)
	job := &companionJob{id: "completed-request", kind: "image", status: "completed", image: raw, imagePrompt: "Keep the original prompt"}
	checkpoint := studioGenerationCheckpoint{State: studioProgressState{Found: true, ID: "deleted-image-generation", Kind: "image", Stage: "complete", Prompt: "Keep the original prompt", Asset: &summary, FirstFrame: &summary}}
	if err := saveStudioGenerationCheckpoint(checkpoint); err != nil {
		t.Fatal(err)
	}
	if job.snapshot(true)["image"] == nil {
		t.Fatal("available result was hidden")
	}
	response := studioLibraryRequest(t, studioAssetsHandler, http.MethodDelete, "/studio/assets?id="+asset.ID, nil)
	if response.Code != 200 {
		t.Fatalf("delete: %d %s", response.Code, response.Body.String())
	}
	for _, local := range []bool{false, true} {
		state := job.snapshot(local)
		if state["image"] != nil || state["resultRemoved"] != true || state["status"] != "completed" || state["prompt"] != "Keep the original prompt" {
			t.Fatalf("stale completed result: %+v", state)
		}
	}
	for _, key := range []string{"video", "voice"} {
		receipt := companionMediaReceipt{Result: map[string]any{"status": "completed", key: summary, "prompt": "Saved prompt"}}
		result := companionMediaResult(t.TempDir(), "request-id", receipt)
		if result[key] != nil || result["resultRemoved"] != true || result["prompt"] != "Saved prompt" {
			t.Fatalf("stale recovered %s: %+v", key, result)
		}
		if receipt.Result[key] == nil {
			t.Fatal("snapshot rewrote a saved receipt")
		}
	}
	// A fresh store proves deletion filtering survives a process restart.
	state := newStudioProgressStore().read(checkpoint.State.ID)
	if !state.ResultRemoved || state.Asset != nil || state.FirstFrame != nil || state.Stage != "complete" || state.Prompt != checkpoint.State.Prompt {
		t.Fatalf("stale persisted progress: %+v", state)
	}
	retained, found, err := loadStudioGenerationCheckpoint(checkpoint.State.ID)
	if err != nil || !found || retained.State.Asset == nil {
		t.Fatal("filter rewrote historical generation data")
	}
	job.status = "running"
	if state := job.snapshot(true); state["image"] == nil || state["resultRemoved"] != nil {
		t.Fatal("removal changed an active request")
	}
	checkpoint.State.Stage = "generating"
	if state := studioLibraryProgressSnapshot(checkpoint.State); state.Asset == nil || state.ResultRemoved {
		t.Fatal("removal changed active progress")
	}
}

func TestStudioFailedProjectRemovalRestoresPreviousVisibility(t *testing.T) {
	t.Setenv("GLOWBOM_SERVER_TOKEN", "")
	for _, alreadyRemoved := range []bool{false, true} {
		t.Run(fmt.Sprintf("already_removed_%t", alreadyRemoved), func(t *testing.T) {
			studio := t.TempDir()
			t.Setenv("GLOWBOM_STUDIO_DIR", studio)
			project, err := registerStudioProject(studioTestProject(t, "Keep the project reference"))
			if err != nil {
				t.Fatal(err)
			}
			registry := filepath.Join(studio, "DesktopProjects")
			outside := filepath.Join(t.TempDir(), "registry")
			if err := os.Rename(registry, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, registry); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(studio, "RemovedProjects", project.ID+".json")
			previousMarker := []byte(`{"preserve":"earlier removal"}`)
			if alreadyRemoved {
				if err := os.Mkdir(filepath.Dir(marker), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, previousMarker, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(filepath.Join(outside, project.ID+".json"))
			if err != nil {
				t.Fatal(err)
			}
			response := studioLibraryRequest(t, studioProjectsHandler, http.MethodDelete, "/studio/projects?id="+project.ID, nil)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("unsafe registry delete: %d %s", response.Code, response.Body.String())
			}
			current, err := os.ReadFile(filepath.Join(outside, project.ID+".json"))
			if err != nil || !bytes.Equal(before, current) {
				t.Fatal("failed removal changed the project reference")
			}
			if alreadyRemoved {
				current, err := os.ReadFile(marker)
				if err != nil || !bytes.Equal(previousMarker, current) {
					t.Fatal("failed removal changed the earlier removal marker")
				}
			} else if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("failed removal left a new removal marker")
			}
			projects, err := listStudioProjects()
			want := 1
			if alreadyRemoved {
				want = 0
			}
			if err != nil || len(projects) != want {
				t.Fatalf("failed removal changed project visibility: %+v %v", projects, err)
			}
		})
	}
}
