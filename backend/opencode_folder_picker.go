package main

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const folderPickerCanceledToken = "__GLOWBOM_PICKER_CANCELED__"

type openCodeProjectPickResponse struct {
	Success  bool   `json:"success"`
	Path     string `json:"path,omitempty"`
	Canceled bool   `json:"canceled,omitempty"`
	Source   string `json:"source,omitempty"`
	Error    string `json:"error,omitempty"`
}

type openCodeInstructionPickedFile struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	MimeType  string `json:"mimeType,omitempty"`
}

type openCodeInstructionFilesPickResponse struct {
	Success  bool                            `json:"success"`
	Files    []openCodeInstructionPickedFile `json:"files,omitempty"`
	Canceled bool                            `json:"canceled,omitempty"`
	Source   string                          `json:"source,omitempty"`
	Error    string                          `json:"error,omitempty"`
}

func openCodePickProjectFolderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if runtime.GOOS != "darwin" {
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(openCodeProjectPickResponse{
			Success: false,
			Error:   "Native folder picker is only available on macOS.",
		})
		return
	}

	path, canceled, err := pickProjectFolderMacOS()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(openCodeProjectPickResponse{
			Success: false,
			Error:   "Failed to open native folder picker: " + err.Error(),
		})
		return
	}

	if canceled {
		_ = json.NewEncoder(w).Encode(openCodeProjectPickResponse{
			Success:  true,
			Canceled: true,
			Source:   "native",
		})
		return
	}

	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(openCodeProjectPickResponse{
			Success: false,
			Error:   "Selected path is not a valid folder.",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(openCodeProjectPickResponse{
		Success: true,
		Path:    path,
		Source:  "native",
	})
}

func openCodePickInstructionFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if runtime.GOOS != "darwin" {
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(openCodeInstructionFilesPickResponse{
			Success: false,
			Error:   "Native file picker is only available on macOS.",
		})
		return
	}

	paths, canceled, err := pickInstructionFilesMacOS()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(openCodeInstructionFilesPickResponse{
			Success: false,
			Error:   "Failed to open native file picker: " + err.Error(),
		})
		return
	}

	if canceled {
		_ = json.NewEncoder(w).Encode(openCodeInstructionFilesPickResponse{
			Success:  true,
			Canceled: true,
			Source:   "native",
		})
		return
	}

	files := make([]openCodeInstructionPickedFile, 0, len(paths))
	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}

		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			continue
		}

		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			continue
		}

		files = append(files, openCodeInstructionPickedFile{
			Path:      absPath,
			Name:      filepath.Base(absPath),
			SizeBytes: info.Size(),
			MimeType:  inferMimeTypeForPath(absPath),
		})
	}

	_ = json.NewEncoder(w).Encode(openCodeInstructionFilesPickResponse{
		Success: true,
		Files:   files,
		Source:  "native",
	})
}

func pickProjectFolderMacOS() (string, bool, error) {
	cmd := exec.Command(
		"osascript",
		"-e", "try",
		"-e", `set selectedFolder to POSIX path of (choose folder with prompt "Select local Glowbom project folder")`,
		"-e", "return selectedFolder",
		"-e", "on error number -128",
		"-e", `return "__GLOWBOM_PICKER_CANCELED__"`,
		"-e", "end try",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(strings.ToLower(stderr), "user canceled") {
				return "", true, nil
			}
			if stderr != "" {
				return "", false, errors.New(stderr)
			}
		}
		return "", false, err
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", false, errors.New("no folder path returned")
	}

	if selected == folderPickerCanceledToken {
		return "", true, nil
	}

	return selected, false, nil
}

func pickInstructionFilesMacOS() ([]string, bool, error) {
	cmd := exec.Command(
		"osascript",
		"-e", "try",
		"-e", `set selectedFiles to choose file with prompt "Select local files for custom instructions" with multiple selections allowed`,
		"-e", "set outputPaths to {}",
		"-e", "repeat with selectedFile in selectedFiles",
		"-e", "set end of outputPaths to POSIX path of selectedFile",
		"-e", "end repeat",
		"-e", "set AppleScript's text item delimiters to linefeed",
		"-e", "set outputText to outputPaths as text",
		"-e", "set AppleScript's text item delimiters to \"\"",
		"-e", "return outputText",
		"-e", "on error number -128",
		"-e", `return "__GLOWBOM_PICKER_CANCELED__"`,
		"-e", "end try",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(strings.ToLower(stderr), "user canceled") {
				return nil, true, nil
			}
			if stderr != "" {
				return nil, false, errors.New(stderr)
			}
		}
		return nil, false, err
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return nil, false, errors.New("no file paths returned")
	}

	if selected == folderPickerCanceledToken {
		return nil, true, nil
	}

	lines := strings.Split(selected, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil, false, errors.New("no valid file paths returned")
	}

	return paths, false, nil
}

func inferMimeTypeForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "application/octet-stream"
	}

	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	return "application/octet-stream"
}
