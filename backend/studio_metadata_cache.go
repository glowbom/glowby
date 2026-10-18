package main

// Studio keeps every asset in one folder, and image and video records carry their media inline, so
// reading a record's metadata streams the whole file. Listings remember each file's metadata by size
// and modification time and only read files that are new or changed. Studio is shared with the Mac
// app, which can rewrite a record at any time; a changed size or time makes the entry stale.

import (
	"os"
	"sync"
	"time"
)

var studioMetadataCache struct {
	sync.Mutex
	files map[string]studioMetadataCacheEntry
}

type studioMetadataCacheEntry struct {
	size     int64
	modTime  time.Time
	metadata studioAssetMetadata
	err      error
}

func cachedStudioAssetMetadata(path string) (studioAssetMetadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return studioAssetMetadata{}, err
	}
	studioMetadataCache.Lock()
	entry, ok := studioMetadataCache.files[path]
	studioMetadataCache.Unlock()
	if ok && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		return entry.metadata, entry.err
	}
	metadata, err := readStudioAssetMetadata(path)
	studioMetadataCache.Lock()
	if studioMetadataCache.files == nil {
		studioMetadataCache.files = map[string]studioMetadataCacheEntry{}
	}
	studioMetadataCache.files[path] = studioMetadataCacheEntry{size: info.Size(), modTime: info.ModTime(), metadata: metadata, err: err}
	studioMetadataCache.Unlock()
	return metadata, err
}

// Drops entries for files that are no longer in the folder; present holds every current path.
func pruneStudioMetadataCache(dir string, present map[string]bool) {
	studioMetadataCache.Lock()
	defer studioMetadataCache.Unlock()
	for path := range studioMetadataCache.files {
		if !present[path] && len(path) > len(dir) && path[:len(dir)] == dir {
			delete(studioMetadataCache.files, path)
		}
	}
}

// Reads the Studio folder once in the background at launch, so the first lists are quick.
func warmStudioMetadataCache() {
	_, _, _ = listStudioAudioPage("", 0, 1)
	_, _, _, _, _ = listStudioCatalogPage(0, 1, 0, 1)
}
