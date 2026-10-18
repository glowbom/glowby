package main

// Higgsfield account connection: the person signs in to their own Higgsfield account and Glowbom uses
// its MCP server with that account's plan and credits. Sign-in is standard MCP authorization:
// dynamic client registration, then an authorization-code flow with PKCE on a loopback redirect.
// Tokens stay in the system keychain. Nothing here spends credits; `balance` is a free tool.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	neturl "net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	higgsfieldLoopbackAddr  = "127.0.0.1:53682"
	higgsfieldRedirectURI   = "http://127.0.0.1:53682/callback"
	higgsfieldScope         = "openid email offline_access"
	higgsfieldKeyService    = "Glowbom Providers"
	higgsfieldKeyAccount    = "Higgsfield"
	higgsfieldClientAccount = "Higgsfield Client"
	higgsfieldBalanceTTL    = 30 * time.Second
)

// Endpoints are variables so tests can point them at a local server.
var higgsfieldEndpoints = struct{ register, authorize, token, mcp string }{
	register:  "https://clerk.higgsfield.ai/oauth/register",
	authorize: "https://clerk.higgsfield.ai/oauth/authorize",
	token:     "https://clerk.higgsfield.ai/oauth/token",
	mcp:       "https://mcp.higgsfield.ai/mcp",
}

var higgsfieldKeyringGet = keyring.Get
var higgsfieldKeyringSet = keyring.Set
var higgsfieldKeyringDelete = keyring.Delete

// Callers bound each request with their own context; this cap only stops a stuck connection.
// Some tools (a voice clone) work for well over 30 s before they answer.
var higgsfieldHTTPClient = &http.Client{Timeout: 3 * time.Minute}
var higgsfieldNow = time.Now

// Cloudflare in front of Higgsfield rejects some default client signatures (error 1010), so every
// request names the app explicitly.
func higgsfieldUserAgent() string {
	return fmt.Sprintf("Glowbom (%s %s)", runtime.GOOS, runtime.GOARCH)
}

type higgsfieldCredential struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Email        string    `json:"email,omitempty"`
}

type higgsfieldStatus struct {
	Connected    bool     `json:"connected"`
	LoginPending bool     `json:"loginPending,omitempty"`
	Email        string   `json:"email,omitempty"`
	Plan         string   `json:"plan,omitempty"`
	Credits      *float64 `json:"credits,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type higgsfieldLogin struct {
	verifier  string
	createdAt time.Time
	phase     string // pending | succeeded | failed
	err       string
}

type higgsfieldBalance struct {
	plan    string
	credits float64
	at      time.Time
}

type higgsfieldManager struct {
	mu        sync.Mutex
	logins    map[string]*higgsfieldLogin
	server    *http.Server
	balance   *higgsfieldBalance
	refreshMu sync.Mutex
}

var glowbomHiggsfield = &higgsfieldManager{logins: map[string]*higgsfieldLogin{}}

var errHiggsfieldSignedOut = errors.New("Higgsfield sign-in expired. Sign in again.")

// ---------- credential storage ----------

func higgsfieldReadSecret(account string) (string, error) {
	value, err := higgsfieldKeyringGet(higgsfieldKeyService, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func higgsfieldLoadCredential() (*higgsfieldCredential, error) {
	raw, err := higgsfieldReadSecret(higgsfieldKeyAccount)
	if err != nil || raw == "" {
		return nil, err
	}
	var credential higgsfieldCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil || credential.AccessToken == "" {
		return nil, nil
	}
	return &credential, nil
}

func higgsfieldSaveCredential(credential higgsfieldCredential) error {
	raw, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	return higgsfieldKeyringSet(higgsfieldKeyService, higgsfieldKeyAccount, string(raw))
}

func higgsfieldDeleteCredential() error {
	err := higgsfieldKeyringDelete(higgsfieldKeyService, higgsfieldKeyAccount)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

// ---------- OAuth ----------

func higgsfieldPost(ctx context.Context, endpoint, contentType string, body []byte, headers map[string]string) (int, http.Header, []byte, error) {
	started := time.Now()
	operation := "request"
	switch endpoint {
	case higgsfieldEndpoints.mcp:
		operation = "mcp"
	case higgsfieldEndpoints.token:
		operation = "token"
	case higgsfieldEndpoints.register:
		operation = "register"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", higgsfieldUserAgent())
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := higgsfieldHTTPClient.Do(req)
	if err != nil {
		debugHiggsfieldHTTP(ctx, operation, 0, started, err)
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	debugHiggsfieldHTTP(ctx, operation, resp.StatusCode, started, err)
	return resp.StatusCode, resp.Header, data, err
}

// The client ID is public. Keep it across sign-outs so Glowbom registers once per computer.
func higgsfieldClientID(ctx context.Context) (string, error) {
	if id, err := higgsfieldReadSecret(higgsfieldClientAccount); err != nil || id != "" {
		return id, err
	}
	payload, _ := json.Marshal(map[string]any{
		"client_name":                "Glowbom",
		"redirect_uris":              []string{higgsfieldRedirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      higgsfieldScope,
	})
	status, _, data, err := higgsfieldPost(ctx, higgsfieldEndpoints.register, "application/json", payload, nil)
	if err != nil {
		return "", fmt.Errorf("could not reach Higgsfield: %w", err)
	}
	var registered struct {
		ClientID string `json:"client_id"`
	}
	if status/100 != 2 || json.Unmarshal(data, &registered) != nil || registered.ClientID == "" {
		return "", fmt.Errorf("Higgsfield did not accept Glowbom's sign-in request (HTTP %d)", status)
	}
	if err := higgsfieldKeyringSet(higgsfieldKeyService, higgsfieldClientAccount, registered.ClientID); err != nil {
		return "", err
	}
	return registered.ClientID, nil
}

type higgsfieldTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

func higgsfieldTokenRequest(ctx context.Context, form neturl.Values, previous *higgsfieldCredential) (higgsfieldCredential, error) {
	form.Set("resource", higgsfieldEndpoints.mcp)
	status, _, data, err := higgsfieldPost(ctx, higgsfieldEndpoints.token, "application/x-www-form-urlencoded", []byte(form.Encode()), nil)
	if err != nil {
		return higgsfieldCredential{}, fmt.Errorf("could not reach Higgsfield: %w", err)
	}
	var token higgsfieldTokenResponse
	if status != http.StatusOK || json.Unmarshal(data, &token) != nil || token.AccessToken == "" {
		if status == http.StatusBadRequest || status == http.StatusUnauthorized {
			return higgsfieldCredential{}, errHiggsfieldSignedOut
		}
		return higgsfieldCredential{}, fmt.Errorf("Higgsfield sign-in failed (HTTP %d)", status)
	}
	credential := higgsfieldCredential{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, Email: higgsfieldEmailFromIDToken(token.IDToken)}
	if token.ExpiresIn > 0 {
		credential.ExpiresAt = higgsfieldNow().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	if previous != nil {
		if credential.RefreshToken == "" {
			credential.RefreshToken = previous.RefreshToken
		}
		if credential.Email == "" {
			credential.Email = previous.Email
		}
	}
	return credential, nil
}

// The ID token is only read for the display email; it is not used to authorize anything.
func higgsfieldEmailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(payload, &claims)
	return claims.Email
}

// Returns a usable access token, refreshing it when it is about to expire.
func higgsfieldAccessToken(ctx context.Context, force bool) (*higgsfieldCredential, error) {
	glowbomHiggsfield.refreshMu.Lock()
	defer glowbomHiggsfield.refreshMu.Unlock()
	credential, err := higgsfieldLoadCredential()
	if err != nil || credential == nil {
		return nil, err
	}
	fresh := credential.ExpiresAt.IsZero() || higgsfieldNow().Add(time.Minute).Before(credential.ExpiresAt)
	if fresh && !force {
		return credential, nil
	}
	if credential.RefreshToken == "" {
		return nil, errHiggsfieldSignedOut
	}
	clientID, err := higgsfieldClientID(ctx)
	if err != nil {
		return nil, err
	}
	form := neturl.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {credential.RefreshToken}}
	next, err := higgsfieldTokenRequest(ctx, form, credential)
	if err != nil {
		return nil, err
	}
	if err := higgsfieldSaveCredential(next); err != nil {
		return nil, err
	}
	return &next, nil
}

// ---------- MCP ----------

type higgsfieldRPCResult struct {
	Result *struct {
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		Content           []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Streamable HTTP answers either as JSON or as a server-sent event stream; take the last data line.
func higgsfieldRPCBody(data []byte) []byte {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return trimmed
	}
	var last []byte
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data:") {
			last = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return last
}

var errHiggsfieldUnauthorized = errors.New("higgsfield unauthorized")

func higgsfieldRPC(ctx context.Context, accessToken, sessionID string, payload map[string]any) (http.Header, []byte, error) {
	payload["jsonrpc"] = "2.0"
	body, _ := json.Marshal(payload)
	headers := map[string]string{"Authorization": "Bearer " + accessToken, "Accept": "application/json, text/event-stream"}
	if sessionID != "" {
		headers["Mcp-Session-Id"] = sessionID
	}
	status, header, data, err := higgsfieldPost(ctx, higgsfieldEndpoints.mcp, "application/json", body, headers)
	if err != nil {
		return nil, nil, fmt.Errorf("could not reach Higgsfield: %w", err)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, nil, errHiggsfieldUnauthorized
	}
	if status/100 != 2 {
		return nil, nil, fmt.Errorf("Higgsfield returned HTTP %d", status)
	}
	return header, higgsfieldRPCBody(data), nil
}

// Calls one MCP tool in a fresh session and returns its structured content.
func higgsfieldCallToolWithToken(ctx context.Context, accessToken, name string, arguments map[string]any) (json.RawMessage, error) {
	header, _, err := higgsfieldRPC(ctx, accessToken, "", map[string]any{"id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "Glowbom", "version": "1"},
	}})
	if err != nil {
		return nil, err
	}
	sessionID := header.Get("Mcp-Session-Id")
	if _, _, err := higgsfieldRPC(ctx, accessToken, sessionID, map[string]any{"method": "notifications/initialized"}); err != nil {
		return nil, err
	}
	_, data, err := higgsfieldRPC(ctx, accessToken, sessionID, map[string]any{"id": 2, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
	if err != nil {
		return nil, err
	}
	var result higgsfieldRPCResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("Higgsfield sent an unexpected response")
	}
	if result.Error != nil {
		return nil, &higgsfieldToolError{"Higgsfield: " + result.Error.Message}
	}
	if result.Result == nil {
		return nil, errors.New("Higgsfield sent an empty response")
	}
	if result.Result.IsError {
		message := "Higgsfield could not run " + name
		if len(result.Result.Content) > 0 && result.Result.Content[0].Text != "" {
			message = "Higgsfield: " + result.Result.Content[0].Text
		}
		return nil, &higgsfieldToolError{message}
	}
	if len(bytes.TrimSpace(result.Result.StructuredContent)) > 0 {
		return result.Result.StructuredContent, nil
	}
	// Some tools return their JSON only as text content.
	for _, content := range result.Result.Content {
		if text := strings.TrimSpace(content.Text); strings.HasPrefix(text, "{") && json.Valid([]byte(text)) {
			return json.RawMessage(text), nil
		}
	}
	return nil, errors.New("Higgsfield sent an empty result for " + name)
}

// Calls a tool with the saved account, refreshing the token once if Higgsfield rejects it.
func higgsfieldCallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	credential, err := higgsfieldAccessToken(ctx, false)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, errHiggsfieldSignedOut
	}
	if err = validateCompanionHiggsfieldAccount(ctx, credential); err != nil {
		return nil, err
	}
	out, err := higgsfieldCallToolWithToken(ctx, credential.AccessToken, name, arguments)
	if !errors.Is(err, errHiggsfieldUnauthorized) {
		return out, err
	}
	if credential, err = higgsfieldAccessToken(ctx, true); err != nil {
		return nil, err
	}
	if err = validateCompanionHiggsfieldAccount(ctx, credential); err != nil {
		return nil, err
	}
	out, err = higgsfieldCallToolWithToken(ctx, credential.AccessToken, name, arguments)
	if errors.Is(err, errHiggsfieldUnauthorized) {
		return nil, errHiggsfieldSignedOut
	}
	return out, err
}

func (m *higgsfieldManager) readBalance(ctx context.Context, force bool) (*higgsfieldBalance, error) {
	m.mu.Lock()
	cached := m.balance
	m.mu.Unlock()
	if cached != nil && !force && higgsfieldNow().Sub(cached.at) < higgsfieldBalanceTTL {
		return cached, nil
	}
	raw, err := higgsfieldCallTool(ctx, "balance", map[string]any{})
	if err != nil {
		return nil, err
	}
	var balance struct {
		Credits float64 `json:"credits"`
		Plan    string  `json:"subscription_plan_type"`
	}
	if err := json.Unmarshal(raw, &balance); err != nil {
		return nil, errors.New("Higgsfield sent an unexpected balance")
	}
	next := &higgsfieldBalance{plan: balance.Plan, credits: balance.Credits, at: higgsfieldNow()}
	m.mu.Lock()
	m.balance = next
	m.mu.Unlock()
	return next, nil
}

// ---------- sign-in sessions ----------

func (m *higgsfieldManager) pendingLocked() bool {
	for state, login := range m.logins {
		if higgsfieldNow().Sub(login.createdAt) > 15*time.Minute {
			delete(m.logins, state)
			continue
		}
		if login.phase == "pending" {
			return true
		}
	}
	return false
}

func (m *higgsfieldManager) ensureLoopback() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		return nil
	}
	listener, err := net.Listen("tcp", higgsfieldLoopbackAddr)
	if err != nil {
		return fmt.Errorf("could not start the sign-in listener on %s. Close other apps using it and try again", higgsfieldLoopbackAddr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", m.callbackHandler)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	m.server = server
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[HIGGSFIELD] sign-in listener stopped: %v", err)
		}
		m.mu.Lock()
		if m.server == server {
			m.server = nil
		}
		m.mu.Unlock()
	}()
	return nil
}

func (m *higgsfieldManager) startLogin(ctx context.Context) (string, string, error) {
	clientID, err := higgsfieldClientID(ctx)
	if err != nil {
		return "", "", err
	}
	if err := m.ensureLoopback(); err != nil {
		return "", "", err
	}
	state, err := randomURLSafeBase64(24)
	if err != nil {
		return "", "", err
	}
	verifier, err := randomURLSafeBase64(48)
	if err != nil {
		return "", "", err
	}
	m.mu.Lock()
	for _, login := range m.logins { // one sign-in at a time
		if login.phase == "pending" {
			login.phase, login.err = "failed", "Replaced by a newer sign-in."
		}
	}
	m.logins[state] = &higgsfieldLogin{verifier: verifier, createdAt: higgsfieldNow(), phase: "pending"}
	m.mu.Unlock()
	query := neturl.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {higgsfieldRedirectURI},
		"scope": {higgsfieldScope}, "code_challenge": {sha256Base64URL(verifier)}, "code_challenge_method": {"S256"},
		"state": {state}, "resource": {higgsfieldEndpoints.mcp},
	}
	return higgsfieldEndpoints.authorize + "?" + query.Encode(), state, nil
}

func (m *higgsfieldManager) finish(state, code, providerError string) (bool, string) {
	m.mu.Lock()
	login := m.logins[state]
	if login == nil || login.phase != "pending" {
		m.mu.Unlock()
		return false, "This sign-in link has expired. Start again from Glowbom."
	}
	verifier := login.verifier
	m.mu.Unlock()
	fail := func(message string) (bool, string) {
		m.mu.Lock()
		login.phase, login.err = "failed", message
		m.mu.Unlock()
		return false, message
	}
	if providerError != "" {
		return fail("Higgsfield sign-in was not completed.")
	}
	if code == "" {
		return fail("Higgsfield did not return a sign-in code.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clientID, err := higgsfieldClientID(ctx)
	if err != nil {
		return fail(err.Error())
	}
	form := neturl.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code},
		"redirect_uri": {higgsfieldRedirectURI}, "code_verifier": {verifier}}
	credential, err := higgsfieldTokenRequest(ctx, form, nil)
	if err != nil {
		return fail(err.Error())
	}
	if err := higgsfieldSaveCredential(credential); err != nil {
		return fail("Could not save the Higgsfield sign-in on this computer.")
	}
	m.mu.Lock()
	login.phase, login.err = "succeeded", ""
	m.balance = nil
	m.mu.Unlock()
	return true, "Your Higgsfield account is connected."
}

func (m *higgsfieldManager) callbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Host != higgsfieldLoopbackAddr {
		http.Error(w, "Unexpected host", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	ok, message := m.finish(strings.TrimSpace(q.Get("state")), strings.TrimSpace(q.Get("code")), strings.TrimSpace(q.Get("error")))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title := "Higgsfield sign-in failed"
	if ok {
		title = "Higgsfield connected"
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title><style>body{font:16px system-ui,-apple-system,sans-serif;background:#111;color:#fff;display:grid;place-items:center;height:100vh;margin:0}div{max-width:460px;padding:32px;border:1px solid #2e2e2e;border-radius:16px;background:#1b1b1b}h2{margin:0 0 8px;letter-spacing:-.02em}p{color:#a8a8a8;margin:6px 0}</style></head><body><div><h2>%s</h2><p>%s</p><p>You can close this tab and return to Glowbom.</p></div><script>setTimeout(function(){try{window.close()}catch(_){}},900)</script></body></html>`,
		html.EscapeString(title), html.EscapeString(title), html.EscapeString(message))
}

func (m *higgsfieldManager) cancel(state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if login := m.logins[state]; login != nil && login.phase == "pending" {
		login.phase, login.err = "failed", "Sign-in was canceled."
	}
}

func (m *higgsfieldManager) status(ctx context.Context, force bool) higgsfieldStatus {
	m.mu.Lock()
	pending := m.pendingLocked()
	var failure string
	for _, login := range m.logins {
		if login.phase == "failed" && login.err != "" && higgsfieldNow().Sub(login.createdAt) < 2*time.Minute {
			failure = login.err
		}
	}
	m.mu.Unlock()
	credential, err := higgsfieldLoadCredential()
	if err != nil {
		return higgsfieldStatus{LoginPending: pending, Error: "Could not read the Higgsfield sign-in from the keychain."}
	}
	if credential == nil {
		return higgsfieldStatus{LoginPending: pending, Error: failure}
	}
	status := higgsfieldStatus{Connected: true, LoginPending: pending, Email: credential.Email}
	balance, err := m.readBalance(ctx, force)
	if errors.Is(err, errHiggsfieldSignedOut) {
		return higgsfieldStatus{LoginPending: pending, Email: credential.Email, Error: err.Error()}
	}
	if err != nil {
		status.Error = sanitizeProviderError(err)
		return status
	}
	status.Plan, status.Credits = balance.plan, &balance.credits
	return status
}

func (m *higgsfieldManager) logout() error {
	m.mu.Lock()
	m.balance = nil
	m.mu.Unlock()
	return higgsfieldDeleteCredential()
}

// ---------- HTTP routes ----------

// /settings/higgsfield/status (GET, ?refresh=1 skips the 30 s balance cache), /login, /login/cancel, /logout (POST).
func higgsfieldSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if token := glowbomServerToken(); token == "" || !hasValidGlowbomServerToken(r, token) {
		http.Error(w, "Local authentication required.", http.StatusUnauthorized)
		return
	}
	method := http.MethodPost
	if r.URL.Path == "/settings/higgsfield/status" {
		method = http.MethodGet
	}
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	fail := func(err error, status int) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": sanitizeProviderError(err)})
	}
	switch r.URL.Path {
	case "/settings/higgsfield/status":
		writeJSON(w, glowbomHiggsfield.status(ctx, r.URL.Query().Get("refresh") == "1"))
	case "/settings/higgsfield/login":
		authorizationURL, loginID, err := glowbomHiggsfield.startLogin(ctx)
		if err != nil {
			fail(err, http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]string{"authorizationURL": authorizationURL, "loginId": loginID})
	case "/settings/higgsfield/login/cancel":
		var body struct {
			LoginID string `json:"loginId"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
		glowbomHiggsfield.cancel(body.LoginID)
		writeJSON(w, map[string]bool{"canceled": true})
	case "/settings/higgsfield/logout":
		if err := glowbomHiggsfield.logout(); err != nil {
			fail(errors.New("Could not remove the Higgsfield sign-in from the keychain."), http.StatusInternalServerError)
			return
		}
		writeJSON(w, higgsfieldStatus{})
	default:
		http.NotFound(w, r)
	}
}

// The tool ran and answered with an error, so the request was refused rather than lost in transit.
type higgsfieldToolError struct{ message string }

func (e *higgsfieldToolError) Error() string { return e.message }
