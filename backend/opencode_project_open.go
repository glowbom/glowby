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

type openCodeLaunchCommand struct {
	Executable string
	Args       []string
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
		if isPlatformUnsupportedOpenError(openErr) {
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

	if _, ok := folderOpenLaunchCommand(); !ok {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEFinder,
			Label:     label,
			Available: false,
			Path:      projectPath,
			Reason:    folderOpenUnavailableReason(),
		}
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEFinder,
		Label:     label,
		Available: true,
		Path:      projectPath,
		Reason:    folderOpenReason(),
	}
}

func inspectXcodeAction(projectPath string) openCodeProjectIDEAction {
	const label = "Open in Xcode"
	applePath := destinationPathForProject(projectPath, []string{
		"apple", "ios", "ipados", "macos", "visionos", "watchos", "tvos",
	}, "apple")

	if runtime.GOOS != "darwin" {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEXcode,
			Label:     label,
			Available: false,
			Path:      applePath,
			Reason:    "Xcode is only available on macOS.",
		}
	}

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

	if _, ok := androidStudioLaunchCommand(); ok {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEAndroidStudio,
			Label:     label,
			Available: true,
			Path:      androidPath,
			Reason:    "Detected Android Studio launcher for this platform.",
		}
	}

	if _, ok := folderOpenLaunchCommand(); ok {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEAndroidStudio,
			Label:     label,
			Available: true,
			Path:      androidPath,
			Reason:    "Android Studio launcher not found; folder will open instead.",
		}
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEAndroidStudio,
		Label:     label,
		Available: false,
		Path:      androidPath,
		Reason:    "No Android Studio launcher or folder opener found for this platform.",
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

	if _, ok := vsCodeLaunchCommand(); ok {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEVSCode,
			Label:     label,
			Available: true,
			Path:      webPath,
			Reason:    "Detected VS Code launcher for this platform.",
		}
	}

	if _, ok := folderOpenLaunchCommand(); ok {
		return openCodeProjectIDEAction{
			IDE:       openCodeIDEVSCode,
			Label:     label,
			Available: true,
			Path:      webPath,
			Reason:    "VS Code launcher not found; folder will open instead.",
		}
	}

	return openCodeProjectIDEAction{
		IDE:       openCodeIDEVSCode,
		Label:     label,
		Available: false,
		Path:      webPath,
		Reason:    "No VS Code launcher or folder opener found for this platform.",
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
	switch ide {
	case openCodeIDEFinder:
		return openProjectFolder(projectPath)
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

func openProjectFolder(projectPath string) (string, error) {
	if !projectDirectoryExists(projectPath) {
		return "", errors.New("Project folder not found")
	}

	if err := openFolderWithSystemDefault(projectPath); err != nil {
		return "", err
	}
	return projectPath, nil
}

func openInXcode(projectPath string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("Xcode opening is only available on macOS.")
	}

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

	if command, ok := androidStudioLaunchCommand(); ok {
		if err := runLaunchCommand(command, androidPath); err != nil {
			return "", err
		}
		return androidPath, nil
	}

	if err := openFolderWithSystemDefault(androidPath); err != nil {
		return "", err
	}
	return androidPath, nil
}

func openInVSCode(projectPath string) (string, error) {
	webPath := destinationPathForProject(projectPath, []string{"web"}, "web")

	if !projectDirectoryExists(webPath) {
		return "", errors.New("Web output folder not found")
	}

	if command, ok := vsCodeLaunchCommand(); ok {
		if err := runLaunchCommand(command, webPath); err != nil {
			return "", err
		}
		return webPath, nil
	}

	if err := openFolderWithSystemDefault(webPath); err != nil {
		return "", err
	}
	return webPath, nil
}

func runMacOpenWithApp(appPath, targetPath string) error {
	openExecutable := resolveLaunchExecutable("open")
	if openExecutable == "" {
		return errors.New("open command not found")
	}
	return runLaunchCommand(openCodeLaunchCommand{
		Executable: openExecutable,
		Args:       []string{"-a", appPath},
	}, targetPath)
}

func runMacOpenCommand(targetPath string) error {
	openExecutable := resolveLaunchExecutable("open")
	if openExecutable == "" {
		return errors.New("open command not found")
	}
	return runLaunchCommand(openCodeLaunchCommand{
		Executable: openExecutable,
	}, targetPath)
}

func folderOpenLaunchCommand() (openCodeLaunchCommand, bool) {
	switch runtime.GOOS {
	case "darwin":
		if openExecutable := resolveLaunchExecutable("open"); openExecutable != "" {
			return openCodeLaunchCommand{Executable: openExecutable}, true
		}
	case "windows":
		if explorerExecutable := resolveLaunchExecutable("explorer.exe"); explorerExecutable != "" {
			return openCodeLaunchCommand{Executable: explorerExecutable}, true
		}
	case "linux":
		if xdgOpen := resolveLaunchExecutable("xdg-open"); xdgOpen != "" {
			return openCodeLaunchCommand{Executable: xdgOpen}, true
		}
		if gio := resolveLaunchExecutable("gio"); gio != "" {
			return openCodeLaunchCommand{
				Executable: gio,
				Args:       []string{"open"},
			}, true
		}
	}

	return openCodeLaunchCommand{}, false
}

func androidStudioLaunchCommand() (openCodeLaunchCommand, bool) {
	switch runtime.GOOS {
	case "darwin":
		if applicationExists("/Applications/Android Studio.app") {
			if openExecutable := resolveLaunchExecutable("open"); openExecutable != "" {
				return openCodeLaunchCommand{
					Executable: openExecutable,
					Args:       []string{"-a", "/Applications/Android Studio.app"},
				}, true
			}
		}
	case "windows":
		candidates := []string{"studio64.exe", "studio.exe", "studio64", "studio"}
		if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
			candidates = append(candidates,
				filepath.Join(programFiles, "Android", "Android Studio", "bin", "studio64.exe"),
				filepath.Join(programFiles, "Android", "Android Studio", "bin", "studio.exe"),
			)
		}
		if localAppData := os.Getenv("LocalAppData"); localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Programs", "Android Studio", "bin", "studio64.exe"),
				filepath.Join(localAppData, "Programs", "Android Studio", "bin", "studio.exe"),
			)
		}
		if executable := firstResolvedExecutable(candidates); executable != "" {
			return openCodeLaunchCommand{Executable: executable}, true
		}
	case "linux":
		if executable := firstResolvedExecutable([]string{
			"android-studio",
			"studio.sh",
			"studio",
			"/opt/android-studio/bin/studio.sh",
			"/usr/local/android-studio/bin/studio.sh",
			"/snap/bin/android-studio",
		}); executable != "" {
			return openCodeLaunchCommand{Executable: executable}, true
		}
	}

	return openCodeLaunchCommand{}, false
}

func vsCodeLaunchCommand() (openCodeLaunchCommand, bool) {
	switch runtime.GOOS {
	case "darwin":
		if applicationExists("/Applications/Visual Studio Code.app") {
			if openExecutable := resolveLaunchExecutable("open"); openExecutable != "" {
				return openCodeLaunchCommand{
					Executable: openExecutable,
					Args:       []string{"-a", "/Applications/Visual Studio Code.app"},
				}, true
			}
		}
	case "windows":
		candidates := []string{"code.cmd", "code.exe", "code"}
		if localAppData := os.Getenv("LocalAppData"); localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Programs", "Microsoft VS Code", "Code.exe"),
			)
		}
		if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
			candidates = append(candidates,
				filepath.Join(programFiles, "Microsoft VS Code", "Code.exe"),
			)
		}
		if programFilesX86 := os.Getenv("ProgramFiles(x86)"); programFilesX86 != "" {
			candidates = append(candidates,
				filepath.Join(programFilesX86, "Microsoft VS Code", "Code.exe"),
			)
		}
		if executable := firstResolvedExecutable(candidates); executable != "" {
			return openCodeLaunchCommand{Executable: executable}, true
		}
	case "linux":
		if executable := firstResolvedExecutable([]string{
			"code",
			"code-insiders",
			"codium",
			"code-oss",
		}); executable != "" {
			return openCodeLaunchCommand{Executable: executable}, true
		}
	}

	return openCodeLaunchCommand{}, false
}

func folderOpenReason() string {
	switch runtime.GOOS {
	case "darwin":
		return "Open project folder in Finder."
	case "windows":
		return "Open project folder in File Explorer."
	case "linux":
		return "Open project folder in your file manager."
	default:
		return "Open project folder."
	}
}

func folderOpenUnavailableReason() string {
	switch runtime.GOOS {
	case "darwin", "windows", "linux":
		return "No supported folder opener found for this platform."
	default:
		return "Project folder opening is not available on this platform."
	}
}

func openFolderWithSystemDefault(targetPath string) error {
	command, ok := folderOpenLaunchCommand()
	if !ok {
		return errors.New("Project opening is not available on this platform.")
	}

	return runLaunchCommand(command, targetPath)
}

func runLaunchCommand(command openCodeLaunchCommand, targetPath string) error {
	args := append([]string{}, command.Args...)
	args = append(args, targetPath)

	cmd := exec.Command(command.Executable, args...)
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

func resolveLaunchExecutable(candidate string) string {
	if strings.TrimSpace(candidate) == "" {
		return ""
	}

	if filepath.IsAbs(candidate) || strings.ContainsAny(candidate, `/\`) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
		return ""
	}

	resolved, err := exec.LookPath(candidate)
	if err != nil {
		return ""
	}
	return resolved
}

func firstResolvedExecutable(candidates []string) string {
	for _, candidate := range candidates {
		if executable := resolveLaunchExecutable(candidate); executable != "" {
			return executable
		}
	}
	return ""
}

func isPlatformUnsupportedOpenError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "only available on") || strings.Contains(message, "not available on this platform")
}
