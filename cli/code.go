package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	backendPort = 4569
	webPort     = 4572
)

func runCode(args []string) int {
	glowbyRoot, err := findGlowbyRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	backendDir := filepath.Join(glowbyRoot, "backend")
	webDir := filepath.Join(glowbyRoot, "web")

	// Validate directories exist
	if !isDir(backendDir) {
		fmt.Fprintf(os.Stderr, "error: backend directory not found at %s\n", backendDir)
		return 1
	}
	if !isDir(webDir) {
		fmt.Fprintf(os.Stderr, "error: web directory not found at %s\n", webDir)
		return 1
	}

	// Parse optional project path
	projectPath := ""
	if len(args) > 0 {
		p, err := filepath.Abs(args[0])
		if err == nil && isDir(p) {
			projectPath = p
		} else if isDir(args[0]) {
			projectPath = args[0]
		} else {
			fmt.Fprintf(os.Stderr, "error: %s is not a directory\n", args[0])
			return 1
		}
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := reclaimManagedService(glowbyRoot, "backend", backendPort); err != nil {
		fmt.Fprintf(os.Stderr, "error preparing backend port: %v\n", err)
		return 1
	}
	if err := reclaimManagedService(glowbyRoot, "web", webPort); err != nil {
		fmt.Fprintf(os.Stderr, "error preparing web port: %v\n", err)
		return 1
	}

	// Start backend
	fmt.Println("Starting backend...")
	backendCmd := exec.CommandContext(ctx, "go", "run", ".")
	backendCmd.Dir = backendDir
	backendCmd.Stdout = os.Stdout
	backendCmd.Stderr = os.Stderr
	if err := backendCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting backend: %v\n", err)
		return 1
	}

	// Install web dependencies if needed
	nodeModules := filepath.Join(webDir, "node_modules")
	if !isDir(nodeModules) {
		fmt.Println("Installing web dependencies...")
		installCmd := exec.CommandContext(ctx, "bun", "install")
		installCmd.Dir = webDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error installing web dependencies: %v\n", err)
			cancel()
			return 1
		}
	}

	go func() {
		if waitForServer(ctx, fmt.Sprintf("http://127.0.0.1:%d", backendPort), 30*time.Second) {
			if err := recordManagedService(glowbyRoot, "backend", backendPort); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not record backend process info: %v\n", err)
			}
		}
	}()

	// Start web dev server
	fmt.Println("Starting web UI...")
	webCmd := exec.CommandContext(ctx, "bun", "run", "dev")
	webCmd.Dir = webDir
	webCmd.Stdout = os.Stdout
	webCmd.Stderr = os.Stderr
	if err := webCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting web UI: %v\n", err)
		cancel()
		return 1
	}

	// Wait for web server to be ready, then open browser
	go func() {
		url := fmt.Sprintf("http://127.0.0.1:%d", webPort)
		if waitForServer(ctx, url, 30*time.Second) {
			if err := recordManagedService(glowbyRoot, "web", webPort); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not record web process info: %v\n", err)
			}
			if projectPath != "" {
				fmt.Printf("\nProject path hint: %s\n", projectPath)
				fmt.Println("Paste or choose this folder in the Glowby UI to load your project.")
			}
			fmt.Printf("\nOpening %s\n\n", url)
			openBrowser(url)
		}
	}()

	// Wait for both processes
	var wg sync.WaitGroup
	exitCode := 0

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := backendCmd.Wait(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "backend exited: %v\n", err)
			exitCode = 1
			cancel()
		}
	}()
	go func() {
		defer wg.Done()
		if err := webCmd.Wait(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "web UI exited: %v\n", err)
			exitCode = 1
			cancel()
		}
	}()

	wg.Wait()
	clearManagedService(glowbyRoot, "backend")
	clearManagedService(glowbyRoot, "web")
	return exitCode
}

func findGlowbyRoot() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		if root, ok := searchGlowbyRoot(filepath.Dir(exe)); ok {
			return root, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	if root, ok := searchGlowbyRoot(cwd); ok {
		return root, nil
	}

	return "", fmt.Errorf("cannot find the Glowby checkout root (expected sibling backend/ and web/ directories). Run from the glowby repo root or one of its subdirectories")
}

func searchGlowbyRoot(start string) (string, bool) {
	dir := start
	for {
		if isDir(filepath.Join(dir, "backend")) && isDir(filepath.Join(dir, "web")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func waitForServer(ctx context.Context, url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return false
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				return true
			}
		}
	}
}

func reclaimManagedService(glowbyRoot, service string, port int) error {
	pids, err := listPortPIDs(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		clearManagedService(glowbyRoot, service)
		return nil
	}

	recorded, err := readManagedService(glowbyRoot, service)
	if err != nil {
		return err
	}
	if len(recorded) == 0 || !isSubset(pids, recorded) {
		if !isExpectedGlowbyService(service, port) {
			return fmt.Errorf("port %d is already in use by another app. Stop it and retry", port)
		}
	}

	fmt.Printf("Stopping existing Glowby %s on port %d...\n", service, port)
	for _, pid := range pids {
		proc, findErr := os.FindProcess(pid)
		if findErr != nil {
			return fmt.Errorf("could not find existing %s process %d: %w", service, pid, findErr)
		}
		if killErr := proc.Kill(); killErr != nil {
			return fmt.Errorf("could not stop existing %s process %d: %w", service, pid, killErr)
		}
	}
	if !waitForPortFree(port, 5*time.Second) {
		return fmt.Errorf("port %d did not become available after stopping the existing %s", port, service)
	}

	clearManagedService(glowbyRoot, service)
	return nil
}

func recordManagedService(glowbyRoot, service string, port int) error {
	pids, err := listPortPIDs(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return fmt.Errorf("no process found listening on port %d", port)
	}

	stateDir := managedStateDir(glowbyRoot)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	lines := make([]string, 0, len(pids))
	for _, pid := range pids {
		lines = append(lines, strconv.Itoa(pid))
	}
	return os.WriteFile(managedStatePath(glowbyRoot, service), []byte(strings.Join(lines, "\n")), 0o644)
}

func readManagedService(glowbyRoot, service string) ([]int, error) {
	data, err := os.ReadFile(managedStatePath(glowbyRoot, service))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pid, convErr := strconv.Atoi(line)
		if convErr != nil {
			return nil, fmt.Errorf("invalid pid %q in %s state: %w", line, service, convErr)
		}
		pids = append(pids, pid)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return uniqueSortedInts(pids), nil
}

func clearManagedService(glowbyRoot, service string) {
	_ = os.Remove(managedStatePath(glowbyRoot, service))
}

func managedStateDir(glowbyRoot string) string {
	sum := sha1.Sum([]byte(glowbyRoot))
	return filepath.Join(os.TempDir(), "glowby", fmt.Sprintf("%x", sum[:8]))
}

func managedStatePath(glowbyRoot, service string) string {
	return filepath.Join(managedStateDir(glowbyRoot), fmt.Sprintf("%s.pid", service))
}

func waitForPortFree(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pids, err := listPortPIDs(port)
		if err == nil && len(pids) == 0 {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func isExpectedGlowbyService(service string, port int) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	url := ""
	expected := ""

	switch service {
	case "backend":
		url = fmt.Sprintf("http://127.0.0.1:%d/opencode/about", port)
		expected = `"name":"Glowby"`
	case "web":
		url = fmt.Sprintf("http://127.0.0.1:%d", port)
		expected = "<title>Glowby</title>"
	default:
		return false
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	return strings.Contains(string(body), expected)
}

func listPortPIDs(port int) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		return listPortPIDsWindows(port)
	default:
		return listPortPIDsUnix(port)
	}
}

func listPortPIDsUnix(port int) ([]int, error) {
	out, err := exec.Command("lsof", "-nP", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) == 0 && len(out) == 0 {
			return nil, nil
		}
		if errors.As(err, &exitErr) && len(out) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("could not inspect port %d with lsof: %w", port, err)
	}
	return parsePIDLines(string(out))
}

func listPortPIDsWindows(port int) ([]int, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("could not inspect port %d with netstat: %w", port, err)
	}

	target := fmt.Sprintf(":%d", port)
	var pids []int
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.HasSuffix(fields[1], target) || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		pid, convErr := strconv.Atoi(fields[4])
		if convErr != nil {
			continue
		}
		pids = append(pids, pid)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return uniqueSortedInts(pids), nil
}

func parsePIDLines(text string) ([]int, error) {
	var pids []int
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		pids = append(pids, pid)
	}
	return uniqueSortedInts(pids), nil
}

func uniqueSortedInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func isSubset(values, allowed []int) bool {
	if len(values) == 0 {
		return true
	}
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	for _, value := range values {
		if _, ok := allowedSet[value]; !ok {
			return false
		}
	}
	return true
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Run()
}
