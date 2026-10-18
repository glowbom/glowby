package main

// Higgsfield voices in Studio Audio: list the person's custom and preset voices, clone a new voice
// from a recording, and speak a script with Seed Audio. Speech is priced with the free get_cost
// preflight; a voice clone is charged by Higgsfield when it is created.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	higgsfieldSpeechModel     = "seed_audio"
	higgsfieldSpeechMaxChars  = 5000
	higgsfieldVoiceMaxBytes   = 24 << 20
	higgsfieldVoicePageSize   = 100
	higgsfieldVoiceMaxPages   = 5
	higgsfieldSpeechTimeout   = 10 * time.Minute
	higgsfieldVoiceNameLength = 128
)

var higgsfieldVoiceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

type higgsfieldVoice struct {
	ID         string `json:"voiceId"`
	Type       string `json:"voiceType"`
	Name       string `json:"name"`
	Gender     string `json:"gender,omitempty"`
	PreviewURL string `json:"previewUrl,omitempty"`
	Status     string `json:"status,omitempty"`
}

// Custom voices come first, so a person's own clone is the obvious choice.
func higgsfieldListVoices(ctx context.Context) ([]higgsfieldVoice, error) {
	voices := []higgsfieldVoice{}
	cursor := ""
	for page := 0; page < higgsfieldVoiceMaxPages; page++ {
		args := map[string]any{"size": higgsfieldVoicePageSize}
		if cursor != "" {
			args["cursor"] = cursor
		}
		raw, err := higgsfieldCallTool(ctx, "list_voices", args)
		if err != nil {
			return nil, err
		}
		var result struct {
			Voices []struct {
				VoiceID    string  `json:"voice_id"`
				VoiceType  string  `json:"voice_type"`
				Name       string  `json:"name"`
				Gender     *string `json:"gender"`
				PreviewURL string  `json:"preview_url"`
				Status     string  `json:"status"`
			} `json:"voices"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		}
		if json.Unmarshal(raw, &result) != nil {
			return nil, errors.New("Higgsfield sent an unexpected voice list")
		}
		for _, voice := range result.Voices {
			if !higgsfieldVoiceIDPattern.MatchString(voice.VoiceID) || (voice.VoiceType != "preset" && voice.VoiceType != "element") {
				continue
			}
			item := higgsfieldVoice{ID: voice.VoiceID, Type: voice.VoiceType, Name: strings.TrimSpace(voice.Name), Status: voice.Status}
			if voice.Gender != nil {
				item.Gender = *voice.Gender
			}
			if higgsfieldTrustedHost(voice.PreviewURL) {
				item.PreviewURL = voice.PreviewURL
			}
			if item.Name == "" {
				item.Name = "Untitled voice"
			}
			voices = append(voices, item)
		}
		if !result.HasMore || result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	custom, presets := []higgsfieldVoice{}, []higgsfieldVoice{}
	for _, voice := range voices {
		if voice.Type == "element" {
			custom = append(custom, voice)
		} else {
			presets = append(presets, voice)
		}
	}
	return append(custom, presets...), nil
}

type higgsfieldSpeechRequest struct {
	Prompt     string `json:"prompt"`
	VoiceID    string `json:"voiceId"`
	VoiceType  string `json:"voiceType"`
	SpeechRate int    `json:"speechRate"`
}

func normalizeHiggsfieldSpeech(req higgsfieldSpeechRequest) (higgsfieldSpeechRequest, error) {
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return req, errors.New("Enter the words to speak.")
	}
	if len([]rune(req.Prompt)) > higgsfieldSpeechMaxChars {
		return req, errors.New("Higgsfield speech is limited to 5,000 characters. Split the script into shorter parts.")
	}
	if !higgsfieldVoiceIDPattern.MatchString(req.VoiceID) || (req.VoiceType != "preset" && req.VoiceType != "element") {
		return req, errors.New("Choose a Higgsfield voice.")
	}
	if req.SpeechRate < -50 || req.SpeechRate > 100 {
		return req, errors.New("Speech speed must be between -50 and 100.")
	}
	return req, nil
}

func higgsfieldSpeechParams(req higgsfieldSpeechRequest) map[string]any {
	return map[string]any{
		"model": higgsfieldSpeechModel, "prompt": req.Prompt, "voice_id": req.VoiceID, "voice_type": req.VoiceType,
		"speech_rate": req.SpeechRate, "format": "mp3", "sample_rate": 48000, "use_unlim": false,
	}
}

func higgsfieldGenerateAudioCall(ctx context.Context, params map[string]any) (json.RawMessage, error) {
	return higgsfieldCallTool(ctx, "generate_audio", map[string]any{"params": params, "context": "Studio voiceover in Glowbom Desktop."})
}

func higgsfieldSpeechEstimate(ctx context.Context, req higgsfieldSpeechRequest) (float64, error) {
	params := higgsfieldSpeechParams(req)
	params["get_cost"] = true
	raw, err := higgsfieldGenerateAudioCall(ctx, params)
	if err != nil {
		return 0, err
	}
	return parseHiggsfieldCredits(raw, "speech")
}

// Speaks the script and returns the finished audio.
func generateHiggsfieldSpeech(ctx context.Context, req higgsfieldSpeechRequest) ([]byte, string, error) {
	jobID, err := higgsfieldSubmit(ctx, higgsfieldGenerateAudioCall, higgsfieldSpeechParams(req), "speech")
	if err != nil {
		return nil, "", err
	}
	for {
		poll, err := pollHiggsfieldVideo(ctx, jobID) // jobs_wait is the same for every media type
		if err != nil {
			return nil, "", err
		}
		if poll.Done {
			if poll.Status != "completed" {
				return nil, "", errors.New("Higgsfield could not finish this speech. Check the voice and your account limits.")
			}
			return downloadHiggsfieldMedia(ctx, poll.VideoURL, "audio/")
		}
		timer := time.NewTimer(higgsfieldImagePollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, "", ctx.Err()
		case <-timer.C:
		}
	}
}

// Speech for Studio Audio. Called from studioAudioGenerateHandler when the source is Higgsfield.
func studioHiggsfieldSpeechHandler(w http.ResponseWriter, r *http.Request, req higgsfieldSpeechRequest, projectPath string) {
	req, err := normalizeHiggsfieldSpeech(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !higgsfieldConnected() {
		http.Error(w, "Connect your Higgsfield account in Settings → Agents, then try again.", http.StatusBadRequest)
		return
	}
	projectID := ""
	if strings.TrimSpace(projectPath) != "" {
		project, err := registerStudioProject(projectPath)
		if err != nil {
			http.Error(w, "Open a writable Glowbom project first.", http.StatusBadRequest)
			return
		}
		projectID = project.ID
	}
	ctx, cancel := context.WithTimeout(r.Context(), higgsfieldSpeechTimeout)
	defer cancel()
	data, mimeType, err := generateHiggsfieldSpeech(ctx, req)
	if ctx.Err() != nil {
		http.Error(w, "The audio request was stopped. Check Higgsfield before generating again.", http.StatusRequestTimeout)
		return
	}
	if err != nil {
		http.Error(w, "Higgsfield could not generate this speech: "+sanitizeProviderError(err), http.StatusBadGateway)
		return
	}
	if len(data) == 0 || len(data) > elevenLabsAudioMaxBytes {
		http.Error(w, "The generated audio is invalid or exceeds 32 MB.", http.StatusBadGateway)
		return
	}
	asset, err := saveStudioAsset(studioSaveOptions{
		Prompt: req.Prompt, DataURI: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
		MediaType: "audio", Source: "Higgsfield (Voice)", AssetType: "generated", SourceProjectID: projectID,
		AudioType: "voice", MimeType: mimeType, Model: higgsfieldSpeechModel, VoiceID: req.VoiceID, OutputFormat: "mp3_48000",
	})
	if err != nil {
		http.Error(w, "The audio was generated but could not be saved to Studio.", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"asset": summarizeStudioAudio(asset)})
}

// Clones a voice from a WAV or MP3 recording and returns it. Higgsfield charges for the clone
// when it succeeds; the new voice is usually still processing. A clone can succeed even when its
// reply is lost or unclear, so any doubt is settled against the voice list before reporting a failure.
func createHiggsfieldVoice(ctx context.Context, name string, audio []byte, mimeType string) (higgsfieldVoice, error) {
	extension := map[string]string{"audio/wav": ".wav", "audio/mpeg": ".mp3"}[mimeType]
	if extension == "" {
		return higgsfieldVoice{}, errors.New("Use a WAV or MP3 recording.")
	}
	existing := map[string]bool{}
	voices, listErr := higgsfieldListVoices(ctx)
	for _, voice := range voices {
		existing[voice.ID] = true
	}
	mediaID, err := higgsfieldUploadMedia(ctx, audio, "glowbom-voice"+extension, mimeType, "audio")
	if err != nil {
		return higgsfieldVoice{}, err
	}
	raw, err := higgsfieldCallTool(ctx, "create_voice_from_confirmed_audio", map[string]any{"audio_media_id": mediaID, "name": name, "description": "Created in Glowbom Studio."})
	if err == nil {
		if voice, ok := higgsfieldCreatedVoice(raw, name); ok {
			return voice, nil
		}
		err = higgsfieldCreateVoiceMessage(raw)
	}
	// Without a list from before the clone, an older voice of the same name would look new.
	if listErr == nil {
		if voice, ok := higgsfieldFindNewVoice(name, existing, ctx); ok {
			return voice, nil
		}
	}
	return higgsfieldVoice{}, err
}

func higgsfieldCreatedVoice(raw json.RawMessage, name string) (higgsfieldVoice, bool) {
	var created struct {
		VoiceID   string `json:"voice_id"`
		ID        string `json:"id"`
		ElementID string `json:"element_id"`
		Status    string `json:"status"`
		Voice     *struct {
			VoiceID string `json:"voice_id"`
			ID      string `json:"id"`
			Status  string `json:"status"`
		} `json:"voice"`
	}
	if json.Unmarshal(raw, &created) != nil {
		return higgsfieldVoice{}, false
	}
	id, status := created.VoiceID, created.Status
	if created.Voice != nil {
		id, status = created.Voice.VoiceID, created.Voice.Status
		if id == "" {
			id = created.Voice.ID
		}
	}
	for _, candidate := range []string{created.ElementID, created.ID} {
		if id == "" {
			id = candidate
		}
	}
	if !higgsfieldVoiceIDPattern.MatchString(id) {
		return higgsfieldVoice{}, false
	}
	return higgsfieldVoice{ID: id, Type: "element", Name: name, Status: status}, true
}

func higgsfieldCreateVoiceMessage(raw json.RawMessage) error {
	var reply struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &reply)
	if message := strings.TrimSpace(reply.Error + " " + reply.Message); message != "" {
		return errors.New("Higgsfield: " + message)
	}
	return errors.New("Higgsfield did not confirm the new voice")
}

// Looks for a custom voice with this name that was not there before the clone.
func higgsfieldFindNewVoice(name string, existing map[string]bool, parents ...context.Context) (higgsfieldVoice, bool) {
	parent := context.Background()
	if len(parents) > 0 {
		if account, _ := parents[0].Value(companionHiggsfieldAccountContextKey{}).(string); account != "" {
			parent = withCompanionHiggsfieldAccount(parent, account)
		}
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	voices, err := higgsfieldListVoices(ctx)
	if err != nil {
		return higgsfieldVoice{}, false
	}
	for _, voice := range voices {
		if voice.Type == "element" && !existing[voice.ID] && strings.EqualFold(strings.TrimSpace(voice.Name), strings.TrimSpace(name)) {
			return voice, true
		}
	}
	return higgsfieldVoice{}, false
}

// GET /studio/higgsfield/voices → {voices}. POST {name, audio (base64), mimeType} → {voice}.
func higgsfieldVoicesHandler(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	if method != http.MethodPost {
		method = http.MethodGet
	}
	if !authorizeStudioGeneration(w, r, method) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if !higgsfieldConnected() {
		http.Error(w, "Connect your Higgsfield account in Settings → Agents.", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodGet {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		voices, err := higgsfieldListVoices(ctx)
		if err != nil {
			http.Error(w, "Could not load Higgsfield voices: "+sanitizeProviderError(err), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{"voices": voices})
		return
	}
	var request struct {
		Name     string `json:"name"`
		Audio    string `json:"audio"`
		MimeType string `json:"mimeType"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, higgsfieldVoiceMaxBytes*4/3+4096)).Decode(&request) != nil {
		http.Error(w, "The recording is too large. Use up to 3 minutes of speech.", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(request.Name)
	audio, err := base64.StdEncoding.DecodeString(request.Audio)
	switch {
	case name == "" || len([]rune(name)) > higgsfieldVoiceNameLength:
		http.Error(w, "Name the voice (up to 128 characters).", http.StatusBadRequest)
		return
	case err != nil || len(audio) < 1024 || len(audio) > higgsfieldVoiceMaxBytes:
		http.Error(w, "The recording is missing or too large.", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	voice, err := createHiggsfieldVoice(ctx, name, audio, request.MimeType)
	if err != nil {
		http.Error(w, "Higgsfield could not create this voice: "+sanitizeProviderError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"voice": voice})
}

// GET /studio/higgsfield/voices/preview?url= streams a voice sample from Higgsfield's CDN.
func higgsfieldVoicePreviewHandler(w http.ResponseWriter, r *http.Request) {
	if !authorizeStudioGeneration(w, r, http.MethodGet) {
		return
	}
	previewURL := r.URL.Query().Get("url")
	if !higgsfieldTrustedHost(previewURL) {
		http.Error(w, "Unknown voice sample.", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, previewURL, nil)
	if err != nil {
		http.Error(w, "Unknown voice sample.", http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", higgsfieldUserAgent())
	resp, err := higgsfieldHTTPClient.Do(req)
	if err != nil {
		http.Error(w, "Could not load the voice sample.", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Could not load the voice sample.", http.StatusBadGateway)
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		http.Error(w, "Could not load the voice sample.", http.StatusBadGateway)
		return
	}
	mimeType := http.DetectContentType(body)
	if !strings.HasPrefix(mimeType, "audio/") {
		mimeType = "audio/mpeg"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(body)
}
