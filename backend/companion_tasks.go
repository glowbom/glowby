package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const companionTasksProtocol = "glowbom-desktop-tasks-v1"
const companionTasksRequestLimit = 24 << 20
const companionTasksResponseLimit = 32 << 20

type companionTasksCommand struct {
	Version   int                        `json:"version"`
	Operation string                     `json:"operation"`
	RequestID string                     `json:"requestId,omitempty"`
	ScopeID   string                     `json:"scopeId,omitempty"`
	Payload   map[string]json.RawMessage `json:"payload"`
}

type companionTasksError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type companionTasksReply struct {
	Version   int                  `json:"version"`
	OK        bool                 `json:"ok"`
	Result    json.RawMessage      `json:"result,omitempty"`
	Error     *companionTasksError `json:"error,omitempty"`
	RequestID string               `json:"requestId,omitempty"`
}

type companionTasksConnection struct {
	Version        int    `json:"version"`
	Protocol       string `json:"protocol"`
	Endpoint       string `json:"endpoint"`
	InstallationID string `json:"installationId"`
	LaunchID       string `json:"launchId"`
	Token          string `json:"token"`
}

var companionTasksFields = map[string]string{
	"snapshot":     "",
	"agents.list":  "",
	"tasks.show":   "taskId",
	"tasks.create": "agent agentId title instructions project attachments attachmentIds",
	"tasks.reply":  "taskId text revision attachments attachmentIds",
	"tasks.accept": "taskId resultId revision",
	"tasks.cancel": "taskId revision",
	"files.read":   "id",
}

func companionTasksFailure(w http.ResponseWriter, status int, requestID, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(companionTasksReply{Version: 1, RequestID: requestID, Error: &companionTasksError{code, message}})
}

// The paired device acts as the owner. Agent execution and profile management
// remain on Desktop, and its local connection credentials never leave this host.
func (s *companionSession) routeTasks(w http.ResponseWriter, r *http.Request, path string) bool {
	if path != "/tasks/commands" {
		return false
	}
	if r.Method != http.MethodPost {
		companionTasksFailure(w, http.StatusMethodNotAllowed, "", "method_not_allowed", "Send a Tasks command, then retry.")
		return true
	}
	contentType, _, contentTypeError := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentTypeError != nil || contentType != "application/json" {
		companionTasksFailure(w, http.StatusUnsupportedMediaType, "", "invalid_request", "Send the Tasks command as JSON.")
		return true
	}
	if r.ContentLength > companionTasksRequestLimit {
		companionTasksFailure(w, http.StatusRequestEntityTooLarge, "", "request_too_large", "Attach up to 16 MB of files in one message, then retry.")
		return true
	}
	var command companionTasksCommand
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, companionTasksRequestLimit))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&command)
	var extra any
	if err == nil {
		err = decoder.Decode(&extra)
		if err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("multiple commands")
		}
	}
	if err != nil || r.ContentLength > companionTasksRequestLimit {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) || r.ContentLength > companionTasksRequestLimit {
			companionTasksFailure(w, http.StatusRequestEntityTooLarge, "", "request_too_large", "Attach up to 16 MB of files in one message, then retry.")
		} else {
			companionTasksFailure(w, http.StatusBadRequest, "", "invalid_request", "Send one valid Tasks command without an author override.")
		}
		return true
	}
	fields, allowed := companionTasksFields[command.Operation]
	if command.Version != 1 || !allowed || command.Payload == nil || len(command.ScopeID) > 64 || !controlString(command.RequestID, 160, false) || command.RequestID != strings.TrimSpace(command.RequestID) {
		companionTasksFailure(w, http.StatusBadRequest, "", "invalid_request", "This Tasks command is unavailable on the companion.")
		return true
	}
	for key := range command.Payload {
		if !slices.Contains(strings.Fields(fields), key) {
			companionTasksFailure(w, http.StatusBadRequest, command.RequestID, "invalid_request", "This Tasks command contains an unsupported field.")
			return true
		}
	}
	if command.Operation != "snapshot" && command.Operation != "agents.list" && command.Operation != "tasks.show" && command.Operation != "files.read" && strings.TrimSpace(command.RequestID) == "" {
		companionTasksFailure(w, http.StatusBadRequest, "", "request_id_required", "Keep a request ID when sending or retrying a Tasks change.")
		return true
	}
	ctx, stop := s.chatContext(r.Context())
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	uid, err := s.tasksAccount(ctx)
	if err != nil {
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, "account_unavailable", "Sign in to Glowbom Desktop, reopen Tasks, then retry.")
		return true
	}
	paths, err := companionTasksConnectionPaths()
	if err != nil {
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, "tasks_unavailable", "Open Tasks in Glowbom Desktop, then retry.")
		return true
	}
	client := companionTasksHTTPClient()
	defer client.CloseIdleConnections()
	connection, err := discoverCompanionTasks(ctx, client, paths)
	if err != nil {
		code, message := "tasks_unavailable", "Open Tasks in Glowbom Desktop, then retry."
		if errors.Is(err, errCompanionTasksAmbiguous) {
			code, message = "tasks_ambiguous", "More than one Desktop Tasks session is open. Close the other session, then retry."
		}
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, code, message)
		return true
	}
	identity, _, err := connection.call(ctx, client, companionTasksCommand{Version: 1, Operation: "session.open", Payload: map[string]json.RawMessage{}})
	var session struct {
		Session struct {
			UID            string `json:"uid"`
			InstallationID string `json:"installationId"`
			LaunchID       string `json:"launchId"`
		} `json:"session"`
	}
	if err != nil || !identity.OK || json.Unmarshal(identity.Result, &session) != nil || session.Session.UID != uid || session.Session.InstallationID != connection.InstallationID || session.Session.LaunchID != connection.LaunchID {
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, "tasks_account_changed", "Reopen Tasks in the signed-in Glowbom Desktop session, then retry.")
		return true
	}
	scopeID, err := s.tasksScope(connection.InstallationID, uid)
	if err != nil {
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, "tasks_unavailable", "Desktop could not open its private Tasks connection. Reopen Tasks, then retry.")
		return true
	}
	if command.Operation != "snapshot" && command.ScopeID != scopeID {
		companionTasksFailure(w, http.StatusConflict, command.RequestID, "task_scope_changed", "This request belongs to a different Tasks inbox. Reopen Tasks for the original Desktop account to continue.")
		return true
	}
	if ctx.Err() != nil || !s.now().Before(s.expires) {
		companionTasksFailure(w, http.StatusServiceUnavailable, command.RequestID, "tasks_unavailable", "Reconnect to Desktop, then retry the same Tasks request.")
		return true
	}
	reply, status, err := connection.call(ctx, client, command)
	if err != nil {
		companionTasksFailure(w, http.StatusBadGateway, command.RequestID, "tasks_response_unconfirmed", "Desktop could not confirm this Tasks request. Keep its request ID and retry.")
		return true
	}
	if reply.OK {
		// Snapshot contains local account details for the Desktop CLI. Only the
		// inbox data is needed by the paired device.
		if command.Operation == "snapshot" {
			var result struct {
				Agents  json.RawMessage `json:"agents"`
				Tasks   json.RawMessage `json:"tasks"`
				ScopeID string          `json:"scopeId"`
			}
			if json.Unmarshal(reply.Result, &result) != nil || len(result.Agents) == 0 || len(result.Tasks) == 0 {
				companionTasksFailure(w, http.StatusBadGateway, command.RequestID, "invalid_response", "Update Glowbom Desktop, reopen Tasks, then retry.")
				return true
			}
			result.ScopeID = scopeID
			reply.Result, _ = json.Marshal(result)
		}
	} else if status == http.StatusUnauthorized || status == http.StatusForbidden {
		// A Tasks token failure is separate from the phone's valid pairing.
		status = http.StatusServiceUnavailable
		reply.Error = &companionTasksError{"tasks_unavailable", "Reopen Tasks in Glowbom Desktop, then retry."}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(reply)
	return true
}

func (s *companionSession) tasksAccount(ctx context.Context) (string, error) {
	writer := &companionWriter{header: make(http.Header), job: &companionJob{}}
	s.api.ServeHTTP(writer, s.internalRequest(ctx, http.MethodGet, "/account/status", nil))
	var account localAccountStatus
	if ctx.Err() != nil || writer.code != http.StatusOK || len(writer.buffer) > 16<<10 || json.Unmarshal(writer.buffer, &account) != nil || account.Version != 1 || account.Status != "signed_in" || !controlString(account.UID, 128, true) {
		return "", errors.New("account unavailable")
	}
	return account.UID, nil
}

func companionTasksHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, _, err := net.SplitHostPort(address)
				if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
					return nil, errors.New("Tasks requires loopback")
				}
				return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, network, address)
			}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Tasks cannot redirect") },
	}
}

func companionTasksConnectionPaths() ([]string, error) {
	if explicit := os.Getenv("GLOWBOM_DESKTOP_CONNECTION_FILE"); explicit != "" {
		return []string{explicit}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(home, ".glowbom", "desktop", "tasks-connections")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			paths = append(paths, filepath.Join(directory, entry.Name()))
			if len(paths) > 64 {
				return nil, errors.New("too many Tasks connections")
			}
		}
	}
	return paths, nil
}

func companionTasksIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func companionTasksReadConnection(path string) (companionTasksConnection, error) {
	var connection companionTasksConnection
	invalid := errors.New("invalid private Tasks connection")
	if !filepath.IsAbs(path) {
		return connection, invalid
	}
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return connection, invalid
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return connection, invalid
	}
	for _, candidate := range []string{directory, resolved} {
		for parent := candidate; ; parent = filepath.Dir(parent) {
			for _, marker := range []string{".git", ".hg", ".svn", "glowbom.json", ".glowbom/chat.json", "project-book/book.json"} {
				if _, err := os.Lstat(filepath.Join(parent, marker)); !errors.Is(err, os.ErrNotExist) {
					return connection, invalid
				}
			}
			if filepath.Dir(parent) == parent {
				break
			}
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return connection, invalid
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0o077 != 0 {
		return connection, invalid
	}
	name := filepath.Base(path)
	info, err = root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 16<<10 {
		return connection, invalid
	}
	file, err := root.Open(name)
	if err != nil {
		return connection, invalid
	}
	defer file.Close()
	opened, err = file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0o077 != 0 {
		return connection, invalid
	}
	data, err := io.ReadAll(io.LimitReader(file, (16<<10)+1))
	if err != nil || len(data) > 16<<10 || json.Unmarshal(data, &connection) != nil || connection.Version != 1 || connection.Protocol != companionTasksProtocol || !companionTasksIdentifier(connection.InstallationID) || !companionTasksIdentifier(connection.LaunchID) || len(connection.Token) < 32 || len(connection.Token) > 512 || strings.ContainsAny(connection.Token, " \t\r\n") {
		return companionTasksConnection{}, invalid
	}
	endpoint, err := url.Parse(connection.Endpoint)
	if err != nil || endpoint.Scheme != "http" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || endpoint.RawPath != "" {
		return companionTasksConnection{}, invalid
	}
	host, portText, err := net.SplitHostPort(endpoint.Host)
	port, portErr := strconv.Atoi(portText)
	if err != nil || portErr != nil || port < 1 || port > 65535 || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return companionTasksConnection{}, invalid
	}
	if endpoint.Path != "" && endpoint.Path != "/" {
		for _, part := range strings.Split(strings.Trim(endpoint.Path, "/"), "/") {
			if !companionTasksIdentifier(part) {
				return companionTasksConnection{}, invalid
			}
		}
	}
	connection.Endpoint = strings.TrimRight(connection.Endpoint, "/")
	return connection, nil
}

var errCompanionTasksAmbiguous = errors.New("multiple active Tasks connections")

func discoverCompanionTasks(parent context.Context, client *http.Client, paths []string) (companionTasksConnection, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var active companionTasksConnection
	for _, path := range paths {
		connection, err := companionTasksReadConnection(path)
		if err != nil {
			continue
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, connection.Endpoint+"/v1/identity", nil)
		response, err := client.Do(request)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
		_ = response.Body.Close()
		var identity companionTasksConnection
		if err != nil || response.StatusCode != http.StatusOK || len(data) > 16<<10 || json.Unmarshal(data, &identity) != nil || identity.Version != 1 || identity.Protocol != companionTasksProtocol || identity.InstallationID != connection.InstallationID || identity.LaunchID != connection.LaunchID {
			continue
		}
		if active.Endpoint != "" {
			return companionTasksConnection{}, errCompanionTasksAmbiguous
		}
		active = connection
	}
	if ctx.Err() != nil || active.Endpoint == "" {
		return companionTasksConnection{}, errors.New("Tasks unavailable")
	}
	return active, nil
}

func (connection companionTasksConnection) call(ctx context.Context, client *http.Client, command companionTasksCommand) (companionTasksReply, int, error) {
	var reply companionTasksReply
	// Actor is never read from a companion request.
	// The companion scope is a transport guard, not part of the Tasks receipt.
	command.ScopeID = ""
	body, err := json.Marshal(struct {
		companionTasksCommand
		Actor string `json:"actor"`
	}{command, "owner"})
	if err != nil {
		return reply, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, connection.Endpoint+"/v1/commands", bytes.NewReader(body))
	if err != nil {
		return reply, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+connection.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return reply, 0, err
	}
	defer response.Body.Close()
	if response.ContentLength > companionTasksResponseLimit {
		return companionTasksReply{}, 0, errors.New("Tasks response too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, companionTasksResponseLimit+1))
	if err != nil || len(data) > companionTasksResponseLimit || json.Unmarshal(data, &reply) != nil || reply.Version != 1 || reply.RequestID != command.RequestID ||
		(reply.OK && (reply.Error != nil || len(reply.Result) == 0 || response.StatusCode != http.StatusOK)) ||
		(!reply.OK && (reply.Error == nil || !controlString(reply.Error.Code, 80, true) || !controlString(reply.Error.Message, 1000, true) || response.StatusCode < 400 || response.StatusCode > 599)) {
		return companionTasksReply{}, 0, errors.New("invalid Tasks response")
	}
	return reply, response.StatusCode, nil
}
