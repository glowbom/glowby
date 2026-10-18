package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var errStudioLibraryChanged = errors.New("The Studio item changed. Try again.")

// Keep mutations inside the selected Studio folder even if a path is replaced.
func openStudioLibraryFolder(name string, create bool) (*os.Root, error) {
	dir, err := studioRootDirectory()
	if err != nil {
		return nil, err
	}
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if create {
		if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("The Studio folder is unavailable.")
	}
	return root.OpenRoot(name)
}

func readStudioLibraryRecord(root *os.Root, name string, limit int64) (map[string]json.RawMessage, []byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, nil, errors.New("The Studio record is unavailable.")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameStudioProjectRecord(info, opened) {
		return nil, nil, errStudioLibraryChanged
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, nil, errors.New("Could not read the Studio record.")
	}
	current, err := root.Lstat(name)
	finished, fileErr := file.Stat()
	if err != nil || fileErr != nil || !sameStudioProjectRecord(opened, current) || !sameStudioProjectRecord(opened, finished) {
		return nil, nil, errStudioLibraryChanged
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return nil, nil, errors.New("The Studio record is invalid.")
	}
	return fields, data, nil
}

func writeStudioLibraryRecord(root *os.Root, name string, data []byte) error {
	temporary := ".glowbom-" + randomUUIDString()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}

func findStudioLibraryAsset(root *os.Root, id string) (string, map[string]json.RawMessage, []byte, error) {
	dir, err := root.Open(".")
	if err != nil {
		return "", nil, nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return "", nil, nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") || !strings.Contains(strings.ToUpper(entry.Name()), id) {
			continue
		}
		fields, data, err := readStudioLibraryRecord(root, entry.Name(), 128<<20)
		if err == nil && normalizedStudioUUID(studioJSONString(fields, "id")) == id {
			return entry.Name(), fields, data, nil
		}
	}
	return "", nil, nil, os.ErrNotExist
}

func authorizeStudioLibraryMutation(w http.ResponseWriter, r *http.Request) bool {
	if !isAllowedOrigin(r, glowbomAllowedOrigins()) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "This request is not allowed.", http.StatusForbidden)
		return false
	}
	if token := glowbomServerToken(); token != "" && !hasValidGlowbomServerToken(r, token) {
		http.Error(w, "Local authentication is required.", http.StatusUnauthorized)
		return false
	}
	return true
}

func studioAssetFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !authorizeStudioLibraryMutation(w, r) {
		return
	}
	var request struct {
		ID         string `json:"id"`
		IsFavorite *bool  `json:"isFavorite"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request) != nil || normalizedStudioUUID(request.ID) == "" || request.IsFavorite == nil {
		http.Error(w, "Choose a valid Studio item and favorite setting.", http.StatusBadRequest)
		return
	}
	studioProjectMu.Lock()
	defer studioProjectMu.Unlock()
	root, err := openStudioLibraryFolder("Assets", false)
	if err != nil {
		http.Error(w, "Studio item not found.", http.StatusNotFound)
		return
	}
	defer root.Close()
	name, fields, previous, err := findStudioLibraryAsset(root, normalizedStudioUUID(request.ID))
	if err != nil {
		http.Error(w, "Studio item not found.", http.StatusNotFound)
		return
	}
	fields["isFavorite"], _ = json.Marshal(*request.IsFavorite)
	data, err := json.Marshal(fields)
	if err != nil {
		http.Error(w, "Could not save the favorite setting.", http.StatusInternalServerError)
		return
	}
	_, current, err := readStudioLibraryRecord(root, name, 128<<20)
	if err != nil || !bytes.Equal(current, previous) {
		http.Error(w, errStudioLibraryChanged.Error(), http.StatusConflict)
		return
	}
	if err := writeStudioLibraryRecord(root, name, data); err != nil {
		http.Error(w, "Could not save the favorite setting.", http.StatusInternalServerError)
		return
	}
	invalidateStudioLibraryAsset(name)
	var metadata studioAssetMetadata
	_ = json.Unmarshal(data, &metadata)
	writeJSON(w, map[string]any{"success": true, "id": metadata.ID, "isFavorite": *request.IsFavorite, "asset": summarizeStudioMetadata(metadata)})
}

func invalidateStudioLibraryAsset(name string) {
	dir, _ := studioAssetsDirectory()
	studioMetadataCache.Lock()
	delete(studioMetadataCache.files, filepath.Join(dir, name))
	studioMetadataCache.Unlock()
	studioCatalogCache.Lock()
	studioCatalogCache.ready = false
	studioCatalogCache.Unlock()
}

func deleteStudioAssetHandler(w http.ResponseWriter, r *http.Request) {
	deleteStudioLibraryAsset(w, r, "")
}

func deleteStudioLibraryAsset(w http.ResponseWriter, r *http.Request, mediaType string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !authorizeStudioLibraryMutation(w, r) {
		return
	}
	id := normalizedStudioUUID(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Choose a valid Studio item.", http.StatusBadRequest)
		return
	}
	studioProjectMu.Lock()
	defer studioProjectMu.Unlock()
	root, err := openStudioLibraryFolder("Assets", false)
	if err != nil {
		http.Error(w, "Studio item not found.", http.StatusNotFound)
		return
	}
	defer root.Close()
	name, fields, previous, err := findStudioLibraryAsset(root, id)
	if err != nil || (mediaType != "" && studioJSONString(fields, "mediaType") != mediaType) {
		http.Error(w, "Studio item not found.", http.StatusNotFound)
		return
	}
	_, current, err := readStudioLibraryRecord(root, name, 128<<20)
	if err != nil || !bytes.Equal(current, previous) {
		http.Error(w, errStudioLibraryChanged.Error(), http.StatusConflict)
		return
	}
	// Media is embedded in this record. Files copied into a project are independent.
	removed, err := openStudioLibraryFolder("RemovedAssets", true)
	if err != nil {
		http.Error(w, "Could not save the Studio removal.", http.StatusInternalServerError)
		return
	}
	defer removed.Close()
	marker, _ := json.Marshal(map[string]string{"id": id})
	if err := writeStudioLibraryRecord(removed, id+".json", marker); err != nil {
		http.Error(w, "Could not save the Studio removal.", http.StatusInternalServerError)
		return
	}
	if err := root.Remove(name); err != nil {
		_ = removed.Remove(id + ".json")
		http.Error(w, "Could not remove the Studio item.", http.StatusInternalServerError)
		return
	}
	invalidateStudioLibraryAsset(name)
	writeJSON(w, map[string]bool{"success": true})
}

func studioAssetWasRemoved(id string) bool {
	id = normalizedStudioUUID(id)
	if id == "" {
		return false
	}
	root, err := openStudioLibraryFolder("RemovedAssets", false)
	if err != nil {
		return false
	}
	defer root.Close()
	info, err := root.Lstat(id + ".json")
	return err == nil && info.Mode().IsRegular()
}

// Preserve completed requests while omitting media the owner explicitly deleted.
func studioLibraryResultSnapshot(result map[string]any) map[string]any {
	if status, _ := result["status"].(string); status != "completed" && status != "failed" && status != "canceled" {
		return result
	}
	var filtered map[string]any
	for _, key := range []string{"image", "video", "voice", "asset"} {
		value, present := result[key]
		if !present {
			continue
		}
		data, err := json.Marshal(value)
		var asset struct {
			ID string `json:"id"`
		}
		if err != nil || json.Unmarshal(data, &asset) != nil || !studioAssetWasRemoved(asset.ID) {
			continue
		}
		if filtered == nil {
			filtered = make(map[string]any, len(result))
			for field, value := range result {
				filtered[field] = value
			}
		}
		delete(filtered, key)
		filtered["resultRemoved"] = true
	}
	if filtered != nil {
		return filtered
	}
	return result
}

// Tombstones also hide IDs found only through asset links or retained native records.
func removedStudioProjectIDs() (map[string]bool, error) {
	removed := map[string]bool{}
	root, err := openStudioLibraryFolder("RemovedProjects", false)
	if os.IsNotExist(err) {
		return removed, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if id := normalizedStudioUUID(strings.TrimSuffix(entry.Name(), ".json")); id != "" {
			removed[id] = true
		}
	}
	return removed, nil
}

func deleteStudioProjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !authorizeStudioLibraryMutation(w, r) {
		return
	}
	id := normalizedStudioUUID(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Choose a valid Studio project.", http.StatusBadRequest)
		return
	}
	studioProjectMu.Lock()
	defer studioProjectMu.Unlock()
	root, err := openStudioLibraryFolder("RemovedProjects", true)
	if err != nil {
		http.Error(w, "Could not remove the Studio project reference.", http.StatusInternalServerError)
		return
	}
	defer root.Close()
	markerName := id + ".json"
	createdMarker, removedReference := false, false
	defer func() {
		if createdMarker && !removedReference {
			_ = root.Remove(markerName)
		}
	}()
	if info, err := root.Lstat(markerName); os.IsNotExist(err) {
		data, _ := json.Marshal(map[string]string{"id": id})
		if err := writeStudioLibraryRecord(root, markerName, data); err != nil {
			http.Error(w, "Could not remove the Studio project reference.", http.StatusInternalServerError)
			return
		}
		createdMarker = true
	} else if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "Could not remove the Studio project reference.", http.StatusInternalServerError)
		return
	}
	// Native Projects files may contain the only saved project history. Preserve them.
	registry, err := openStudioLibraryFolder("DesktopProjects", false)
	if err == nil {
		defer registry.Close()
		if err := registry.Remove(id + ".json"); err != nil && !os.IsNotExist(err) {
			http.Error(w, "Could not remove the Studio project reference.", http.StatusInternalServerError)
			return
		}
	} else if !os.IsNotExist(err) {
		http.Error(w, "Could not remove the Studio project reference.", http.StatusInternalServerError)
		return
	}
	removedReference = true
	writeJSON(w, map[string]bool{"success": true})
}

func registerStudioProjectWithRestore(path string, restore bool) (studioProjectSummary, error) {
	studioProjectMu.Lock()
	defer studioProjectMu.Unlock()
	return registerStudioProjectRestoringLocked(path, restore)
}

func registerStudioProjectRestoringLocked(path string, restore bool) (studioProjectSummary, error) {
	project, err := registerStudioProjectLocked(path)
	if err != nil || !restore || !project.Removed {
		return project, err
	}
	root, err := openStudioLibraryFolder("RemovedProjects", false)
	if err != nil {
		return project, err
	}
	defer root.Close()
	if err := root.Remove(project.ID + ".json"); err != nil && !os.IsNotExist(err) {
		return project, err
	}
	id := project.ID
	project, err = registerStudioProjectLocked(path)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"id": id})
		_ = writeStudioLibraryRecord(root, id+".json", data)
	}
	return project, err
}
