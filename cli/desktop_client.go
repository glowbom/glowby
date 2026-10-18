package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const desktopProtocol = "glowbom-desktop-tasks-v1"
const desktopConnectionEnv = "GLOWBOM_DESKTOP_CONNECTION_FILE"
const desktopResponseLimit = 32 << 20

type desktopConnection struct {
	Version        int    `json:"version"`
	Protocol       string `json:"protocol"`
	Endpoint       string `json:"endpoint"`
	InstallationID string `json:"installationId"`
	LaunchID       string `json:"launchId"`
	Token          string `json:"token"`
}

type desktopCommand struct {
	Version   int            `json:"version"`
	Operation string         `json:"operation"`
	RequestID string         `json:"requestId,omitempty"`
	Actor     string         `json:"actor,omitempty"`
	Payload   map[string]any `json:"payload"`
}

type desktopError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type desktopResponse struct {
	Version   int             `json:"version"`
	OK        bool            `json:"ok"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *desktopError   `json:"error,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
}

type desktopClient struct {
	connection desktopConnection
	http       *http.Client
}

func desktopHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:             nil,
			DialContext:       (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("Desktop requests cannot follow redirects")
		},
	}
}

func readDesktopConnection(path string) (desktopConnection, error) {
	var connection desktopConnection
	if !filepath.IsAbs(path) {
		return connection, errors.New(desktopConnectionEnv + " must name an absolute file path")
	}
	if err := validateCredentialLocation(path); err != nil {
		return connection, errors.New("the Desktop connection file must be outside projects and source checkouts")
	}
	if err := privateCredentialPath(path, false); err != nil {
		return connection, errors.New("the Desktop connection file and directory must be private and must not be symbolic links; reopen Desktop Tasks")
	}
	data, err := readDesktopFile(path, 16<<10)
	if err != nil || json.Unmarshal(data, &connection) != nil {
		return connection, errors.New("the Desktop connection file is invalid; reopen Desktop Tasks")
	}
	if connection.Version != 1 || connection.Protocol != desktopProtocol ||
		!validDesktopIdentifier(connection.InstallationID) || !validDesktopIdentifier(connection.LaunchID) ||
		len(connection.Token) < 32 || len(connection.Token) > 512 || strings.ContainsAny(connection.Token, " \t\r\n") {
		return desktopConnection{}, errors.New("the Desktop connection format is not supported; reopen Desktop Tasks")
	}
	endpoint, err := url.Parse(connection.Endpoint)
	if err != nil || endpoint.Scheme != "http" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		endpoint.ForceQuery || endpoint.RawPath != "" || !validDesktopEndpointPath(endpoint.Path) {
		return desktopConnection{}, errors.New("the Desktop connection must use a local loopback HTTP address")
	}
	host, portText, err := net.SplitHostPort(endpoint.Host)
	port, portErr := strconv.Atoi(portText)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || port < 1 || port > 65535 || ip == nil || !ip.IsLoopback() {
		return desktopConnection{}, errors.New("the Desktop connection must use a numeric loopback address and port")
	}
	connection.Endpoint = strings.TrimRight(connection.Endpoint, "/")
	return connection, nil
}

func validDesktopEndpointPath(path string) bool {
	if path == "" || path == "/" {
		return true
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/"), "/") {
		if !validDesktopIdentifier(part) {
			return false
		}
	}
	return true
}

func validDesktopIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func (client *desktopClient) verify(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.connection.Endpoint+"/v1/identity", nil)
	if err != nil {
		return errors.New("could not prepare the Desktop connection check")
	}
	response, err := client.http.Do(req)
	if err != nil {
		return errors.New("Desktop Tasks is not available; open it and try again")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	var identity desktopConnection
	if err != nil || len(data) > 16<<10 || response.StatusCode != http.StatusOK || json.Unmarshal(data, &identity) != nil ||
		identity.Version != 1 || identity.Protocol != desktopProtocol || identity.InstallationID != client.connection.InstallationID ||
		identity.LaunchID != client.connection.LaunchID {
		return errors.New("the Desktop connection is out of date; reopen Desktop Tasks and try again")
	}
	return nil
}

func discoverDesktopClient(ctx context.Context) (*desktopClient, error) {
	paths := []string{}
	explicit := os.Getenv(desktopConnectionEnv)
	if explicit != "" {
		paths = append(paths, explicit)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.New("could not find your Desktop connection directory")
		}
		dir := filepath.Join(home, ".glowbom", "desktop", "tasks-connections")
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, errors.New("open Tasks in Glowbom Desktop before using these commands")
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				paths = append(paths, filepath.Join(dir, entry.Name()))
			}
		}
		if len(paths) > 64 {
			return nil, errors.New("too many Desktop connection records; select one with " + desktopConnectionEnv)
		}
	}
	var active *desktopClient
	for _, path := range paths {
		connection, err := readDesktopConnection(path)
		if err == nil {
			client := &desktopClient{connection: connection, http: desktopHTTPClient()}
			err = client.verify(ctx)
			if err == nil {
				if active != nil {
					return nil, errors.New("more than one Desktop session is open; select its connection file with " + desktopConnectionEnv)
				}
				active = client
			}
		}
		if ctx.Err() != nil {
			return nil, errors.New("Desktop command cancelled")
		}
		if err != nil && explicit != "" {
			return nil, err
		}
	}
	if active == nil {
		return nil, errors.New("no open Desktop task session was found; open Tasks in Glowbom Desktop")
	}
	return active, nil
}

func (client *desktopClient) call(ctx context.Context, command desktopCommand) (desktopResponse, error) {
	var result desktopResponse
	body, err := json.Marshal(command)
	if err != nil {
		return result, errors.New("could not prepare the Desktop command")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.connection.Endpoint+"/v1/commands", bytes.NewReader(body))
	if err != nil {
		return result, errors.New("could not prepare the Desktop request")
	}
	req.Header.Set("Authorization", "Bearer "+client.connection.Token)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return result, errors.New("Desktop command cancelled; use the same request ID when retrying")
		}
		return result, errors.New("Desktop did not respond; check Tasks and use the same request ID when retrying")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, desktopResponseLimit+1))
	if err != nil || len(data) > desktopResponseLimit || json.Unmarshal(data, &result) != nil || result.Version != 1 ||
		(result.OK && (result.Error != nil || len(result.Result) == 0 || response.StatusCode < 200 || response.StatusCode >= 300)) ||
		(!result.OK && (result.Error == nil || result.Error.Code == "" || result.Error.Message == "")) ||
		(result.RequestID != "" && result.RequestID != command.RequestID) {
		return desktopResponse{}, errors.New("Desktop returned an invalid response; check Tasks before retrying with the same request ID")
	}
	if result.RequestID == "" {
		result.RequestID = command.RequestID
	}
	return result, nil
}
