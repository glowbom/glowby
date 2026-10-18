package main

// POST /studio/import?kind=video|audio&name=…&duration=… with the file as the body saves a video or
// audio file from the computer into Studio, so it can be used as a reference or in a project.

import (
	"encoding/base64"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Studio lists, plays, trims and copies videos up to studioClipSourceLimit and audio up to
// elevenLabsAudioMaxBytes, so imports stop there too.
const (
	studioImportVideoMaxBytes = studioClipSourceLimit
	studioImportAudioMaxBytes = elevenLabsAudioMaxBytes
)

func studioImportHandler(w http.ResponseWriter, r *http.Request) {
	if !authorizeStudioGeneration(w, r, http.MethodPost) {
		return
	}
	query := r.URL.Query()
	kind := query.Get("kind")
	limit := map[string]int64{"video": studioImportVideoMaxBytes, "audio": studioImportAudioMaxBytes}[kind]
	if limit == 0 {
		http.Error(w, "Choose a video or audio file.", http.StatusBadRequest)
		return
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		http.Error(w, map[string]string{"video": "Videos can be up to 80 MB. Export it at 720p or 1080p, or use a shorter clip.", "audio": "Audio files can be up to 32 MB."}[kind], http.StatusRequestEntityTooLarge)
		return
	}
	if len(data) < 512 {
		http.Error(w, "The file is empty.", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(mimeType, kind+"/") {
		if detected := http.DetectContentType(data); strings.HasPrefix(detected, kind+"/") {
			mimeType = detected
		} else if kind == "video" {
			mimeType = "video/mp4" // DetectContentType misses MOV and some MP4 files.
		} else {
			http.Error(w, "Choose an audio file.", http.StatusBadRequest)
			return
		}
	}
	name := strings.TrimSpace(query.Get("name"))
	if runes := []rune(name); len(runes) > 200 {
		name = string(runes[:200])
	}
	if name == "" {
		name = map[string]string{"video": "Imported video", "audio": "Imported audio"}[kind]
	}
	duration, _ := strconv.ParseFloat(query.Get("duration"), 64)
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 || duration > 24*3600 {
		duration = 0
	}
	options := studioSaveOptions{
		Prompt: name, DataURI: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
		MediaType: kind, MimeType: mimeType, Source: "Studio upload", AssetType: "reference", SourceType: "uploaded", Duration: duration,
	}
	if kind == "audio" {
		options.AudioType = "voice"
	}
	record, err := saveStudioAsset(options)
	if err != nil {
		http.Error(w, "Could not save the file in Studio.", http.StatusInternalServerError)
		return
	}
	if kind == "audio" {
		writeJSON(w, map[string]any{"asset": summarizeStudioAudio(record)})
		return
	}
	writeJSON(w, map[string]any{"asset": summarizeStudioRecord(record)})
}
