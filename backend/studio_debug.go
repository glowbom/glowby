package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var glowbomDebugMu sync.Mutex
var glowbomDebugLabel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]{0,119}$`)

const glowbomDebugLogLimit = 8 << 20

type studioDebugRequestKey struct{}

// Only this fixed metadata schema can enter the opt-in log. Never include
// prompts, provider response bodies, media, URLs, account IDs, or credentials.
type glowbomDebugMetadata struct {
	RequestID     string `json:"requestId,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Stage         string `json:"stage,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	Operation     string `json:"operation,omitempty"`
	HTTPStatus    int    `json:"httpStatus,omitempty"`
	ErrorCategory string `json:"errorCategory,omitempty"`
	ElapsedMS     int64  `json:"elapsedMs"`
	CanResume     *bool  `json:"canResume,omitempty"`
}

func writeGlowbomDebugEvent(event string, metadata glowbomDebugMetadata) {
	path := os.Getenv("GLOWBOM_DEBUG_LOG")
	if !filepath.IsAbs(path) {
		return
	}
	if !studioGenerationID.MatchString(metadata.RequestID) {
		metadata.RequestID = ""
	}
	for _, label := range []*string{&event, &metadata.Kind, &metadata.Stage, &metadata.Provider, &metadata.Model, &metadata.Operation, &metadata.ErrorCategory} {
		if !glowbomDebugLabel.MatchString(*label) {
			*label = ""
		}
	}
	if event == "" {
		return
	}
	record := struct {
		Timestamp string               `json:"timestamp"`
		Source    string               `json:"source"`
		Event     string               `json:"event"`
		Metadata  glowbomDebugMetadata `json:"metadata"`
	}{time.Now().UTC().Format(time.RFC3339Nano), "backend", event, metadata}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	glowbomDebugMu.Lock()
	defer glowbomDebugMu.Unlock()
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return
	}
	defer root.Close()
	name := filepath.Base(path)
	if info, err := root.Lstat(name); err == nil && (!info.Mode().IsRegular() || info.Size() >= glowbomDebugLogLimit) || err != nil && !os.IsNotExist(err) {
		return
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	opened, err := file.Stat()
	current, currentErr := root.Lstat(name)
	if err != nil || currentErr != nil || !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(opened, current) || opened.Size()+int64(len(data)+1) > glowbomDebugLogLimit {
		return
	}
	if err := file.Chmod(0600); err != nil {
		return
	}
	// One append keeps each JSON line together with the launcher's own events.
	_, _ = file.Write(append(data, '\n'))
}

func debugStudioGeneration(event string, state studioProgressState) {
	elapsed := int64(0)
	if started, err := time.Parse(time.RFC3339Nano, state.StartedAt); err == nil {
		elapsed = max(0, time.Since(started).Milliseconds())
	}
	writeGlowbomDebugEvent(event, glowbomDebugMetadata{RequestID: state.ID, Kind: state.Kind, Stage: state.Stage, Provider: state.SourceID, Model: state.ModelID, ErrorCategory: state.FailureCategory, ElapsedMS: elapsed, CanResume: &state.CanResume})
}

func studioDebugErrorCategory(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	var failure *studioVideoFailure
	if errors.As(err, &failure) {
		if failure.category != "" {
			return failure.category
		}
		if failure.terminal {
			return "provider_failed_unknown"
		}
	}
	return "transport_or_provider_error"
}

func debugHiggsfieldPoll(ctx context.Context, stage string, err error) {
	requestID, _ := ctx.Value(studioDebugRequestKey{}).(string)
	writeGlowbomDebugEvent("provider_job_status", glowbomDebugMetadata{RequestID: requestID, Provider: "higgsfield", Operation: "jobs_wait", Stage: stage, ErrorCategory: studioDebugErrorCategory(err)})
}

func debugHiggsfieldHTTP(ctx context.Context, operation string, status int, started time.Time, err error) {
	requestID, _ := ctx.Value(studioDebugRequestKey{}).(string)
	category := studioDebugErrorCategory(err)
	if err == nil && status >= 400 {
		switch status {
		case http.StatusUnauthorized, http.StatusForbidden:
			category = "authentication_or_permission"
		case 429:
			category = "rate_limit"
		default:
			category = "provider_http_error"
		}
	}
	writeGlowbomDebugEvent("provider_http", glowbomDebugMetadata{RequestID: requestID, Provider: "higgsfield", Operation: operation, HTTPStatus: status, ErrorCategory: category, ElapsedMS: time.Since(started).Milliseconds()})
}
