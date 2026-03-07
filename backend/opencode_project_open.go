package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	openCodeIDEFinder        = "finder"
	openCodeIDEXcode         = "xcode"
	openCodeIDEAndroidStudio = "android-studio"
	openCodeIDEVSCode        = "vscode"
)

type openCodeProjectIDERequest struct {
	Path string `json:"path"`
}

type openCodeProjectIDEAction struct {
	IDE       string `json:"ide"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type openCodeProjectIDEStatusResponse struct {
	Success bool                       `json:"success"`
	Path    string                     `json:"path,omitempty"`
	Actions []openCodeProjectIDEAction `json:"actions,omitempty"`
	Error   string                     `json:"error,omitempty"`
}

type openCodeProjectOpenRequest struct {
	Path string `json:"path"`
	IDE  string `json:"ide"`
}

type openCodeProjectOpenResponse struct {
	Success    bool   `json:"success"`
	IDE        string `json:"ide,omitempty"`
	OpenedPath string `json:"openedPath,omitempty"`
	Error      string `json:"error,omitempty"`
}

func openCodeProjectIDEStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectPath, err := readProjectPathFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(openCodeProjectIDEStatusResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	normalizedPath, err := normalizeExistingDirectoryPath(projectPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(openCodeProjectIDEStatusResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	actions := inspectProjectIDEActions(normalizedPath)
	_ = json.NewEncoder(w).Encode(openCodeProjectIDEStatusResponse{
		Success: true,
		Path:    normalizedPath,
		Actions: actions,
	})
}

func openCodeProjectOpenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req openCodeProjectOpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(openCodeProjectOpenResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		return
	}

	normalizedPath, err := normalizeExistingDirectoryPath(req.Path)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(openCodeProjectOpenResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	ide := strings.TrimSpace(strings.ToLower(req.IDE))
	openedPath, openErr := openProjectInIDE(normalizedPath, ide)
	if openErr != nil {
		statusCode := http.StatusInternalServerError
		if runtime.GOOS != "darwin" {
			statusCode = http.StatusNotImplemented
		}
		if errors.Is(openErr, os.ErrNotExist) || strings.Contains(strings.ToLower(openErr.Error()), "not found") {
			statusCode = http.StatusBadRequest
		}
		if strings.Contains(strings.ToLower(openErr.Error()), "unsupported ide") {
			statusCode = http.StatusBadRequest
		}
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(openCodeProjectOpenResponse{
			Success: false,
			IDE:     ide,
			Error:   openErr.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(openCodeProjectOpenResponse{
		Success:    true,
		IDE:        ide,
		OpenedPath: openedPath,
	})
}

func readProjectPathFromRequest(r *http.Request) (string, error) {
	if r.Method == http.MethodGet {
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if path == "" {
			return "", errors.New("path query parameter required")
		}
		return path, nil
	}

	var req openCodeProjectIDERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", errors.New("Invalid JSON body")
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		return "", errors.New("path is required")
	}
	return path, nil
}

func normalizeExistingDirectoryPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path is required")
	}

	cleaned := filepath.Clean(trimmed)
	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("project path not found")
		}
		return "", fmt.Errorf("failed to access project path: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("project path must be a directory")
	}
	return absolute, nil
}

func inspectProjectIDEActions(projectPath string) []openCodeProjectIDEAction {
	return []openCodeProjectIDEAction{
		inspectFinderAction(projectPath),
		inspectXcodeAction(projectPath),
		inspectAndroidStudioAction(projectPath),
		inspectVSCodeAction(projectPath),
	}
}

func inspectFinderAction(projectPath string) openCodeProjectIDEAction {
	const label = "Open Folder"
	if !projectDirectoryExists(projectPath) {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEFinder,
			Label:     label,
			Available: false,
			Path:      projectPath,
			Reason:    "Project folder not found.",
		}
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEFinder,
		Label:     label,
		Available: true,
		Path:      projectPath,
		Reason:    "Open project folder in Finder.",
	}
}

func inspectXcodeAction(projectPath string) openCodeProjectIDEAction {
	const label = "Open in Xcode"
	applePath := destinationPathForProject(projectPath, []string{
		"apple", "ios", "ipados", "macos", "visionos", "watchos", "tvos",
	}, "apple")

	if !projectDirectoryExists(applePath) {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEXcode,
			Label:     label,
			Available: false,
			Path:      applePath,
			Reason:    "No Apple project output folder found.",
		}
	}

	if workspaceOrProject := preferredXcodePath(applePath); workspaceOrProject != "" {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEXcode,
			Label:     label,
			Available: true,
			Path:      workspaceOrProject,
			Reason:    "Detected Xcode workspace/project.",
		}
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEXcode,
		Label:     label,
		Available: true,
		Path:      applePath,
		Reason:    "No .xcworkspace/.xcodeproj found yet; folder will open.",
	}
}

func inspectAndroidStudioAction(projectPath string) openCodeProjectIDEAction {
	const label = "Open in Android Studio"
	androidPath := destinationPathForProject(projectPath, []string{
		"android", "wearos", "androidtv", "androidauto",
	}, "android")

	if !projectDirectoryExists(androidPath) {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEAndroidStudio,
			Label:     label,
			Available: false,
			Path:      androidPath,
			Reason:    "No Android project output folder found.",
		}
	}

	reason := "Detected Android project folder."
	if !applicationExists("/Applications/Android Studio.app") {
		reason = "Android Studio.app not found; folder will open instead."
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEAndroidStudio,
		Label:     label,
		Available: true,
		Path:      androidPath,
		Reason:    reason,
	}
}

func inspectVSCodeAction(projectPath string) openCodeProjectIDEAction {
	const label = "Open in VS Code"
	webPath := destinationPathForProject(projectPath, []string{"web"}, "web")

	if !projectDirectoryExists(webPath) {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEVSCode,
			Label:     label,
			Available: false,
			Path:      webPath,
			Reason:    "No Web project output folder found.",
		}
	}

	reason := "Detected Web project folder."
	if !applicationExists("/Applications/Visual Studio Code.app") {
		reason = "Visual Studio Code.app not found; folder will open instead."
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEVSCode,
		Label:     label,
		Available: true,
		Path:      webPath,
		Reason:    reason,
	}
}

func destinationPathForProject(projectPath string, outputDirs []string, fallback string) string {
	for _, dirName := range outputDirs {
		candidate := filepath.Join(projectPath, dirName)
		if projectDirectoryExists(candidate) {
			return candidate
		}
	}
	return filepath.Join(projectPath, fallback)
}

func preferredXcodePath(basePath string) string {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".xcworkspace") {
			return filepath.Join(basePath, name)
		}
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".xcodeproj") {
			return filepath.Join(basePath, name)
		}
	}

	return ""
}

func projectDirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func applicationExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func openProjectInIDE(projectPath, ide string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("Project opening is only available on macOS.")
	}

	switch ide {
	case openCodeIDEFinder:
		return openInFinder(projectPath)
	case openCodeIDEXcode:
		return openInXcode(projectPath)
	case openCodeIDEAndroidStudio:
		return openInAndroidStudio(projectPath)
	case openCodeIDEVSCode:
		return openInVSCode(projectPath)
	default:
		return "", fmt.Errorf("unsupported IDE: %s", ide)
	}
}

func openInFinder(projectPath string) (string, error) {
	if !projectDirectoryExists(projectPath) {
		return "", errors.New("Project folder not found")
	}

	if err := runMacOpenCommand(projectPath); err != nil {
		return "", err
	}
	return projectPath, nil
}

func openInXcode(projectPath string) (string, error) {
	applePath := destinationPathForProject(projectPath, []string{
		"apple", "ios", "ipados", "macos", "visionos", "watchos", "tvos",
	}, "apple")

	if !projectDirectoryExists(applePath) {
		return "", errors.New("Apple output folder not found")
	}

	openPath := preferredXcodePath(applePath)
	if openPath == "" {
		openPath = applePath
	}

	if err := runMacOpenCommand(openPath); err != nil {
		return "", err
	}
	return openPath, nil
}

func openInAndroidStudio(projectPath string) (string, error) {
	androidPath := destinationPathForProject(projectPath, []string{
		"android", "wearos", "androidtv", "androidauto",
	}, "android")

	if !projectDirectoryExists(androidPath) {
		return "", errors.New("Android output folder not found")
	}

	if applicationExists("/Applications/Android Studio.app") {
		if err := runMacOpenWithApp("/Applications/Android Studio.app", androidPath); err != nil {
			return "", err
		}
		return androidPath, nil
	}

	if err := runMacOpenCommand(androidPath); err != nil {
		return "", err
	}
	return androidPath, nil
}

func openInVSCode(projectPath string) (string, error) {
	webPath := destinationPathForProject(projectPath, []string{"web"}, "web")

	if !projectDirectoryExists(webPath) {
		return "", errors.New("Web output folder not found")
	}

	if applicationExists("/Applications/Visual Studio Code.app") {
		if err := runMacOpenWithApp("/Applications/Visual Studio Code.app", webPath); err != nil {
			return "", err
		}
		return webPath, nil
	}

	if err := runMacOpenCommand(webPath); err != nil {
		return "", err
	}
	return webPath, nil
}

func runMacOpenWithApp(appPath, targetPath string) error {
	cmd := exec.Command("open", "-a", appPath, targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("failed to open project: %s", message)
		}
		return fmt.Errorf("failed to open project: %w", err)
	}
	return nil
}

func runMacOpenCommand(targetPath string) error {
	cmd := exec.Command("open", targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("failed to open path: %s", message)
		}
		return fmt.Errorf("failed to open path: %w", err)
	}
	return nil
}
