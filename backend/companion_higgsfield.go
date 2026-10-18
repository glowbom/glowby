package main

// These routes are available only after companion authentication. Desktop Studio
// keeps its existing handlers and review behavior.
import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const companionMediaQuoteTTL = 10 * time.Minute

// Matches Desktop's published estimate. Higgsfield has no free clone price tool.
const companionVoiceCloneEstimateCredits = 40

type companionMediaQuote struct {
	ID          string    `json:"quoteId"`
	Credits     float64   `json:"credits"`
	Balance     *float64  `json:"balance,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt"`
	PriceKind   string    `json:"priceKind"`
	Kind        string    `json:"kind"`
	Hash        string    `json:"hash"`
	AccountHash string    `json:"accountHash"`
	RequestID   string    `json:"requestId,omitempty"`
}

type companionMediaReceipt struct {
	Kind        string         `json:"kind"`
	Hash        string         `json:"hash"`
	RequestHash string         `json:"requestHash"`
	AccountHash string         `json:"accountHash"`
	Result      map[string]any `json:"result"`
}

type companionVoiceRequest struct {
	Name      string `json:"name"`
	AudioID   string `json:"audioId"`
	QuoteID   string `json:"quoteId,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Confirmed bool   `json:"confirmed,omitempty"`
}

var companionMediaMu sync.Mutex
var companionMediaJobs = map[string]*companionJob{}
var companionVoiceJobs = map[string]bool{}

func (s *companionSession) routeHiggsfield(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case (path == "/studio/capabilities" || path == "/higgsfield/capabilities") && r.Method == http.MethodGet:
		desktopID, err := companionMediaDesktopID()
		if err != nil {
			companionMediaError(w, "request_unavailable", "Desktop could not open its phone workflow state.", http.StatusServiceUnavailable)
			return true
		}
		writeJSON(w, map[string]any{"desktopId": desktopID, "version": 1, "videoQuotes": true, "higgsfieldQuotes": true, "talkingVideo": true, "voiceLibrary": true, "voiceCloning": true, "audioImport": true, "projectVideo": true, "videoIdeas": true, "maxAudioBytes": companionImportAudioBytes, "maxAudioDurationSeconds": 180})
	case path == "/higgsfield/status" && r.Method == http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		status := glowbomHiggsfield.status(ctx, false)
		status.Email = ""
		if status.Error != "" {
			status.Error = "Could not refresh Higgsfield. Check the account on Desktop."
		}
		writeJSON(w, status)
	case path == "/higgsfield/voices" && r.Method == http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		voices, err := higgsfieldListVoices(ctx)
		if err != nil {
			http.Error(w, "Could not load Higgsfield voices. Check the account on Desktop.", http.StatusBadGateway)
			return true
		}
		writeJSON(w, map[string]any{"voices": voices})
	case path == "/videos/estimate" && r.Method == http.MethodPost:
		s.quoteStudioVideo(w, r)
	case path == "/higgsfield/voices/estimate" && r.Method == http.MethodPost:
		s.quoteHiggsfieldVoice(w, r)
	case path == "/higgsfield/voices" && r.Method == http.MethodPost:
		s.startHiggsfieldVoice(w, r)
	case strings.HasPrefix(path, "/videos/requests/") && r.Method == http.MethodGet:
		s.mediaRequest(w, r, strings.TrimPrefix(path, "/videos/requests/"), "video")
	case strings.HasPrefix(path, "/higgsfield/voices/requests/") && r.Method == http.MethodGet:
		s.mediaRequest(w, r, strings.TrimPrefix(path, "/higgsfield/voices/requests/"), "voice")
	default:
		return false
	}
	return true
}

func companionMediaDirectory() (string, error) {
	imports, err := companionTransferDirectory()
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(filepath.Dir(imports))
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err = root.Mkdir("PhoneMediaRequests", 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	info, err := root.Lstat("PhoneMediaRequests")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("Desktop could not open its media review folder.")
	}
	return filepath.Join(filepath.Dir(imports), "PhoneMediaRequests"), nil
}

func companionMediaError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"code": code, "message": message})
}

func companionMediaDesktopID() (string, error) {
	companionMediaMu.Lock()
	defer companionMediaMu.Unlock()
	directory, err := companionMediaDirectory()
	if err != nil {
		return "", err
	}
	var state struct {
		ID string `json:"desktopId"`
	}
	err = companionReadMediaState(directory, "desktop.json", &state)
	if err == nil && normalizedStudioUUID(state.ID) != "" {
		return state.ID, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("Desktop's phone workflow identity could not be read.")
	}
	state.ID = strings.ToLower(randomUUIDString())
	return state.ID, companionWriteMediaState(directory, "desktop.json", state)
}

type companionHiggsfieldAccountContextKey struct{}

func withCompanionHiggsfieldAccount(ctx context.Context, account string) context.Context {
	return context.WithValue(ctx, companionHiggsfieldAccountContextKey{}, account)
}

func companionHiggsfieldCredentialHash(credential *higgsfieldCredential) string {
	if credential == nil {
		return ""
	}
	identity := strings.ToLower(strings.TrimSpace(credential.Email))
	if identity == "" {
		identity = credential.RefreshToken
	}
	if identity == "" {
		identity = credential.AccessToken
	}
	if identity == "" {
		return ""
	}
	return chatSourceHash(identity)
}

// No expected account is attached to ordinary Desktop calls.
func validateCompanionHiggsfieldAccount(ctx context.Context, credential *higgsfieldCredential) error {
	if expected, _ := ctx.Value(companionHiggsfieldAccountContextKey{}).(string); expected != "" && expected != companionHiggsfieldCredentialHash(credential) {
		return errors.New("The Higgsfield account changed after the phone's review. Check the selected account before trying again.")
	}
	return nil
}

func companionHiggsfieldAccountHash() (string, error) {
	credential, err := higgsfieldLoadCredential()
	if err != nil || credential == nil {
		return "", errors.New("Connect Higgsfield on Desktop before reviewing this request.")
	}
	hash := companionHiggsfieldCredentialHash(credential)
	if hash == "" {
		return "", errors.New("Reconnect Higgsfield on Desktop.")
	}
	return hash, nil
}

func companionWriteMediaState(directory, filename string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return atomicChatFile(directory, filename, data)
}

func companionReadMediaState(directory, filename string, value any) error {
	_, data, err := readStudioJSON(filepath.Join(directory, filename), 256<<10)
	if err != nil {
		return err
	}
	if json.Unmarshal(data, value) != nil {
		return errors.New("Desktop could not recover this media request.")
	}
	return nil
}

func companionVideoRequestHash(request companionVideoRequest) string {
	data, _ := json.Marshal(request.payload())
	return chatSourceHash(string(data))
}

func companionVoiceRequestHash(request companionVoiceRequest) string {
	data, _ := json.Marshal(struct{ Name, AudioID string }{strings.TrimSpace(request.Name), normalizedStudioUUID(request.AudioID)})
	return chatSourceHash(string(data))
}

func companionVideoFingerprint(request companionVideoRequest) (string, error) {
	if err := validateHiggsfieldVideoReferences(request.studioVideoOptions); err != nil {
		return "", err
	}
	payload := request.payload()
	hashes := map[string]string{}
	for _, item := range []struct{ id, kind string }{{request.ReferenceID, "image"}, {request.MotionVideoID, "video"}, {request.VoiceAudioID, "audio"}} {
		if item.id == "" {
			continue
		}
		record, err := findStudioAsset(item.id)
		if err != nil || (record.MediaType != item.kind && !(item.kind == "image" && record.MediaType == "")) {
			return "", errors.New("A selected Studio reference is no longer available. Choose it again.")
		}
		// Bind a quote to bytes as well as identifiers, so replacement assets need review.
		hashes[item.id] = chatSourceHash(record.DataBase64)
	}
	data, err := json.Marshal(struct {
		Payload    map[string]any
		References map[string]string
	}{payload, hashes})
	if err != nil {
		return "", err
	}
	return chatSourceHash(string(data)), nil
}

func companionVideoPrice(ctx context.Context, request companionVideoRequest) (float64, error) {
	params := higgsfieldVideoParams(request.studioVideoOptions, higgsfieldReferencePrompt(request.studioVideoOptions, request.Prompt), "")
	// Higgsfield prices these reference videos by model, length and resolution.
	// No reference uploads or paid submissions occur during this preflight.
	if request.ModelID == "seedance_2_5" && (request.ReferenceID != "" || request.ReferenceImage != "" || request.MotionVideoID != "" || request.VoiceAudioID != "" || request.HiggsfieldVoiceID != "") {
		params["mode"] = "omni_reference"
	}
	params["get_cost"] = true
	raw, err := higgsfieldGenerateVideoCall(ctx, params)
	if err != nil {
		return 0, err
	}
	var reply struct {
		Cost *struct {
			Credits *float64 `json:"credits"`
			Exact   *float64 `json:"credits_exact"`
		} `json:"cost"`
	}
	if json.Unmarshal(raw, &reply) != nil || reply.Cost == nil || reply.Cost.Credits == nil {
		return 0, errors.New("Higgsfield did not return a price.")
	}
	credits := *reply.Cost.Credits
	if reply.Cost.Exact != nil && *reply.Cost.Exact > credits {
		credits = *reply.Cost.Exact
	}
	if math.IsNaN(credits) || math.IsInf(credits, 0) || credits < 0 {
		return 0, errors.New("Higgsfield returned an invalid price.")
	}
	return credits, nil
}

func (s *companionSession) storeMediaQuote(w http.ResponseWriter, r *http.Request, kind, hash, account string, credits float64, priceKind string) {
	quote := companionMediaQuote{ID: strings.ToLower(randomUUIDString()), Credits: credits, ExpiresAt: s.now().Add(companionMediaQuoteTTL).UTC(), PriceKind: priceKind, Kind: kind, Hash: hash, AccountHash: account}
	ctx, cancel := context.WithTimeout(withCompanionHiggsfieldAccount(r.Context(), account), 30*time.Second)
	defer cancel()
	if balance, err := glowbomHiggsfield.readBalance(ctx, false); err == nil {
		value := balance.credits
		quote.Balance = &value
	}
	companionMediaMu.Lock()
	defer companionMediaMu.Unlock()
	directory, err := companionMediaDirectory()
	if err == nil {
		err = companionWriteMediaState(directory, "quote-"+quote.ID+".json", quote)
	}
	if err != nil {
		http.Error(w, "Desktop could not save this price review. No generation was started.", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"quoteId": quote.ID, "credits": quote.Credits, "balance": quote.Balance, "expiresAt": quote.ExpiresAt, "priceKind": quote.PriceKind})
}

func (s *companionSession) quoteStudioVideo(w http.ResponseWriter, r *http.Request) {
	var request companionVideoRequest
	if !companionDecode(w, r, &request, 3<<20) {
		return
	}
	request, err := normalizeCompanionVideo(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.SourceID != higgsfieldSourceID {
		http.Error(w, "Choose Higgsfield for a credit price check.", http.StatusBadRequest)
		return
	}
	hash, err := companionVideoFingerprint(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(withCompanionHiggsfieldAccount(r.Context(), account), 30*time.Second)
	defer cancel()
	credits, err := companionVideoPrice(ctx, request)
	if err != nil {
		http.Error(w, "Could not check the Higgsfield price. No generation was started.", http.StatusBadGateway)
		return
	}
	s.storeMediaQuote(w, r, "video", hash, account, credits, "live")
}

// Caller holds companionMediaMu. A consumed quote can never authorize another job.
func (s *companionSession) mediaApproval(directory, kind, hash, account, quoteID, requestID string, confirmed bool) (companionMediaQuote, error) {
	var quote companionMediaQuote
	if !confirmed || normalizedStudioUUID(quoteID) == "" || normalizedStudioUUID(requestID) == "" {
		return quote, errors.New("Review and confirm the Higgsfield credit cost before creating this media. Update the phone app if needed.")
	}
	err := companionReadMediaState(directory, "quote-"+strings.ToLower(normalizedStudioUUID(quoteID))+".json", &quote)
	if err != nil || quote.Kind != kind || quote.Hash != hash || quote.AccountHash != account || !s.now().Before(quote.ExpiresAt) || quote.RequestID != "" {
		return quote, errors.New("This price review expired, changed, or was already used. Check the price again before confirming.")
	}
	return quote, nil
}

func companionReceiptKey(directory, id string) string {
	return filepath.Join(directory, strings.ToLower(id))
}

func companionMediaResult(directory, id string, receipt companionMediaReceipt) map[string]any {
	key := companionReceiptKey(directory, id)
	if job := companionMediaJobs[key]; job != nil {
		result := job.snapshot(false)
		if result["status"] == "failed" || result["status"] == "canceled" {
			result["outcomeUncertain"] = true
		}
		return result
	}
	result := receipt.Result
	if result["status"] == "running" && !companionVoiceJobs[key] {
		result = map[string]any{"id": strings.ToLower(id), "kind": receipt.Kind, "status": "failed", "outcomeUncertain": true, "error": "Desktop could not recover the result. Check Higgsfield and Studio before starting another request. This request will not be submitted again."}
	}
	return studioLibraryResultSnapshot(result)
}

func (s *companionSession) recoverMediaRequest(w http.ResponseWriter, directory, id, kind, hash, account string) bool {
	var receipt companionMediaReceipt
	err := companionReadMediaState(directory, "request-"+strings.ToLower(id)+".json", &receipt)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil || receipt.Kind != kind || receipt.RequestHash != hash || receipt.AccountHash != account {
		companionMediaError(w, "request_changed", "This request ID is already used or cannot be recovered. Check your existing media before starting another request.", http.StatusConflict)
		return true
	}
	if job := companionMediaJobs[companionReceiptKey(directory, id)]; job != nil {
		s.mu.Lock()
		s.jobs[job.id] = job
		s.mu.Unlock()
	}
	writeJSON(w, companionMediaResult(directory, id, receipt))
	return true
}

func (s *companionSession) mediaRequest(w http.ResponseWriter, r *http.Request, id, kind string) {
	id = normalizedStudioUUID(id)
	if id == "" {
		companionMediaError(w, "invalid_request", "Choose a valid media request.", http.StatusBadRequest)
		return
	}
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		companionMediaError(w, "account_unavailable", err.Error(), http.StatusConflict)
		return
	}
	companionMediaMu.Lock()
	defer companionMediaMu.Unlock()
	directory, err := companionMediaDirectory()
	if err != nil {
		companionMediaError(w, "request_unavailable", "Desktop could not open its saved requests. Check the existing result before starting another.", http.StatusServiceUnavailable)
		return
	}
	var receipt companionMediaReceipt
	err = companionReadMediaState(directory, "request-"+strings.ToLower(id)+".json", &receipt)
	if errors.Is(err, os.ErrNotExist) {
		companionMediaError(w, "request_missing", "Desktop has no saved request with this identity.", http.StatusNotFound)
		return
	}
	if err != nil {
		companionMediaError(w, "request_unavailable", "Desktop could not recover this request. Check Higgsfield before starting another.", http.StatusServiceUnavailable)
		return
	}
	if receipt.Kind != kind || receipt.AccountHash != account {
		companionMediaError(w, "request_changed", "This request belongs to another media type or Higgsfield account. Check the original account before starting another.", http.StatusConflict)
		return
	}
	if job := companionMediaJobs[companionReceiptKey(directory, id)]; job != nil {
		s.mu.Lock()
		s.jobs[job.id] = job
		s.mu.Unlock()
	}
	writeJSON(w, companionMediaResult(directory, id, receipt))
}

func (s *companionSession) startQuotedStudioVideo(w http.ResponseWriter, r *http.Request, request companionVideoRequest) {
	if !request.Confirmed || normalizedStudioUUID(request.QuoteID) == "" || normalizedStudioUUID(request.RequestID) == "" {
		companionMediaError(w, "quote_required", "Review and confirm the Higgsfield credit cost before creating this video. Update the phone app if needed.", http.StatusPreconditionRequired)
		return
	}
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		companionMediaError(w, "account_unavailable", err.Error(), http.StatusConflict)
		return
	}
	companionMediaMu.Lock()
	defer companionMediaMu.Unlock()
	directory, err := companionMediaDirectory()
	if err != nil {
		http.Error(w, "Desktop could not save this request. No generation was started.", http.StatusInternalServerError)
		return
	}
	id := strings.ToLower(normalizedStudioUUID(request.RequestID))
	if s.recoverMediaRequest(w, directory, id, "video", companionVideoRequestHash(request), account) {
		return
	}
	hash, err := companionVideoFingerprint(request)
	if err != nil {
		companionMediaError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}
	quote, err := s.mediaApproval(directory, "video", hash, account, request.QuoteID, id, request.Confirmed)
	if err != nil {
		companionMediaError(w, "quote_expired", err.Error(), http.StatusPreconditionRequired)
		return
	}
	ctx, cancel := context.WithTimeout(withCompanionHiggsfieldAccount(r.Context(), account), 30*time.Second)
	defer cancel()
	credits, err := companionVideoPrice(ctx, request)
	if err != nil || credits > quote.Credits {
		companionMediaError(w, "quote_expired", "The Higgsfield price changed or could not be checked. Review the price again. No generation was started.", http.StatusPreconditionRequired)
		return
	}
	s.startJobWithSetup(w, r, "video", companionProject{}, "/studio/videos/generate", request.payload(), func(job *companionJob) error {
		job.id = id
		job.uncertainMediaFailure = true
		job.ctx = withCompanionHiggsfieldAccount(job.ctx, account)
		receipt := companionMediaReceipt{Kind: "video", Hash: hash, RequestHash: companionVideoRequestHash(request), AccountHash: account, Result: job.snapshot(false)}
		quote.RequestID = id
		if companionWriteMediaState(directory, "quote-"+quote.ID+".json", quote) != nil || companionWriteMediaState(directory, "request-"+id+".json", receipt) != nil {
			return errors.New("Desktop could not save the approved request. Check its status before trying again.")
		}
		companionMediaJobs[companionReceiptKey(directory, id)] = job
		job.save = func() {
			// A provider error may follow a successful charge. Keep that uncertainty explicit.
			job.mu.Lock()
			if job.status == "failed" || job.status == "canceled" {
				job.errorText = "The video result was not confirmed. Check Higgsfield and Studio before creating another video. This request will not be submitted again."
			}
			job.mu.Unlock()
			result := job.snapshot(false)
			if result["status"] == "failed" || result["status"] == "canceled" {
				result["outcomeUncertain"] = true
			}
			companionMediaMu.Lock()
			defer companionMediaMu.Unlock()
			receipt.Result = result
			if companionWriteMediaState(directory, "request-"+id+".json", receipt) == nil && result["status"] != "running" {
				delete(companionMediaJobs, companionReceiptKey(directory, id))
			}
		}
		return nil
	})
}

func normalizeCompanionVoice(request companionVoiceRequest) (companionVoiceRequest, []byte, string, string, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.AudioID = normalizedStudioUUID(request.AudioID)
	if request.Name == "" || len([]rune(request.Name)) > higgsfieldVoiceNameLength || request.AudioID == "" {
		return request, nil, "", "", errors.New("Name the voice and choose a saved WAV or MP3 recording.")
	}
	record, err := findStudioAsset(request.AudioID)
	if err != nil || record.MediaType != "audio" {
		return request, nil, "", "", errors.New("The voice recording is no longer available in Studio.")
	}
	data, _, err := decodeBase64Payload(record.DataBase64, record.MimeType)
	if err != nil {
		return request, nil, "", "", errors.New("The voice recording could not be read.")
	}
	mime, duration, err := companionAudioInfo(data)
	if err != nil || duration < 30 || duration > 180 {
		return request, nil, "", "", errors.New("Use a complete WAV or MP3 recording with 30 to 180 seconds of speech, up to 24 MB.")
	}
	encoded, _ := json.Marshal(struct{ Name, AudioID, AudioHash string }{request.Name, request.AudioID, chatSourceHash(string(data))})
	return request, data, mime, chatSourceHash(string(encoded)), nil
}

func (s *companionSession) quoteHiggsfieldVoice(w http.ResponseWriter, r *http.Request) {
	var request companionVoiceRequest
	if !companionDecode(w, r, &request, 4<<10) {
		return
	}
	_, _, _, hash, err := normalizeCompanionVoice(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.storeMediaQuote(w, r, "voice", hash, account, companionVoiceCloneEstimateCredits, "estimate")
}

func (s *companionSession) startHiggsfieldVoice(w http.ResponseWriter, r *http.Request) {
	var request companionVoiceRequest
	if !companionDecode(w, r, &request, 4<<10) {
		return
	}
	if !request.Confirmed || normalizedStudioUUID(request.QuoteID) == "" || normalizedStudioUUID(request.RequestID) == "" {
		companionMediaError(w, "quote_required", "Review and confirm the estimated Higgsfield credit cost before creating this voice.", http.StatusPreconditionRequired)
		return
	}
	account, err := companionHiggsfieldAccountHash()
	if err != nil {
		companionMediaError(w, "account_unavailable", err.Error(), http.StatusConflict)
		return
	}
	companionMediaMu.Lock()
	defer companionMediaMu.Unlock()
	directory, err := companionMediaDirectory()
	if err != nil {
		http.Error(w, "Desktop could not save this request. No voice was created.", http.StatusInternalServerError)
		return
	}
	id := strings.ToLower(normalizedStudioUUID(request.RequestID))
	if s.recoverMediaRequest(w, directory, id, "voice", companionVoiceRequestHash(request), account) {
		return
	}
	request, data, mime, hash, err := normalizeCompanionVoice(request)
	if err != nil {
		companionMediaError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}
	quote, err := s.mediaApproval(directory, "voice", hash, account, request.QuoteID, id, request.Confirmed)
	if err != nil {
		companionMediaError(w, "quote_expired", err.Error(), http.StatusPreconditionRequired)
		return
	}
	for key := range companionVoiceJobs {
		if strings.HasPrefix(key, directory+string(filepath.Separator)) {
			http.Error(w, "Wait for the current voice request before creating another.", http.StatusConflict)
			return
		}
	}
	owner := context.Background()
	if s.manager != nil {
		owner = s.manager.runContext
	}
	ctx, cancel := context.WithTimeout(withCompanionHiggsfieldAccount(owner, account), 4*time.Minute)
	job := &companionJob{id: id, kind: "voice", source: "phone", status: "running", output: []string{},
		startedAt: s.now().UTC().Format(time.RFC3339Nano), ctx: ctx, cancel: cancel, uncertainMediaFailure: true,
		media: &companionMediaDetails{Prompt: request.Name, SourceID: "higgsfield", ModelID: "Voice clone", Stage: "generating"}}
	receipt := companionMediaReceipt{Kind: "voice", Hash: hash, RequestHash: companionVoiceRequestHash(request), AccountHash: account, Result: job.snapshot(false)}
	quote.RequestID = id
	if companionWriteMediaState(directory, "quote-"+quote.ID+".json", quote) != nil || companionWriteMediaState(directory, "request-"+id+".json", receipt) != nil {
		cancel()
		http.Error(w, "Desktop could not save the approved request. Check its status before trying again.", http.StatusInternalServerError)
		return
	}
	job.save = func() {
		result := job.snapshot(false)
		companionMediaMu.Lock()
		defer companionMediaMu.Unlock()
		receipt.Result = result
		_ = companionWriteMediaState(directory, "request-"+id+".json", receipt)
	}
	companionVoiceJobs[companionReceiptKey(directory, id)] = true
	companionMediaJobs[companionReceiptKey(directory, id)] = job
	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()
	if s.manager != nil {
		s.manager.registerRun(job)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, receipt.Result)
	go func() {
		defer cancel()
		voice, err := createHiggsfieldVoice(ctx, request.Name, data, mime)
		if current, accountErr := companionHiggsfieldAccountHash(); accountErr != nil || current != account {
			err = errors.New("The Higgsfield account changed during the voice request.")
		}
		job.mu.Lock()
		job.finishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err == nil && job.status != "canceled" {
			job.status = "completed"
			job.voice, _ = json.Marshal(voice)
		} else {
			if job.status != "canceled" {
				job.status = "failed"
			}
			job.errorText = "Higgsfield did not confirm this voice. Check your Higgsfield voices before starting another request. This request will not be submitted again."
		}
		job.mu.Unlock()
		job.save()
		companionMediaMu.Lock()
		defer companionMediaMu.Unlock()
		delete(companionVoiceJobs, companionReceiptKey(directory, id))
		delete(companionMediaJobs, companionReceiptKey(directory, id))
	}()
}
