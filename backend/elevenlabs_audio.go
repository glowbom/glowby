package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	elevenLabsBaseURL         = "https://api.elevenlabs.io"
	defaultElevenVoiceID      = "JBFqnCBsd6RMkjVDRZzb"
	defaultElevenVoiceModel   = "eleven_multilingual_v2"
	defaultElevenOutputFormat = "mp3_44100_128"
)

type ElevenLabsAudioRequest struct {
	Prompt            string   `json:"prompt"`
	AudioType         string   `json:"audioType"` // "voice" | "sound" | "music"
	ElevenLabsKey     string   `json:"elevenLabsKey,omitempty"`
	VoiceID           string   `json:"voiceId,omitempty"`
	VoiceModel        string   `json:"voiceModel,omitempty"`
	SoundModel        string   `json:"soundModel,omitempty"`
	MusicModel        string   `json:"musicModel,omitempty"`
	OutputFormat      string   `json:"outputFormat,omitempty"`
	DurationSeconds   float64  `json:"durationSeconds,omitempty"`
	PromptInfluence   *float64 `json:"promptInfluence,omitempty"`
	Loop              bool     `json:"loop,omitempty"`
	ForceInstrumental bool     `json:"forceInstrumental,omitempty"`
}

type ElevenLabsAudioResponse struct {
	Prompt    string `json:"prompt"`
	AudioType string `json:"audioType"`
	Filename  string `json:"filename"`
	SavedPath string `json:"saved_path"`
	Audio     string `json:"audio"`
	MimeType  string `json:"mimeType"`
}

type ElevenLabsVoicesRequest struct {
	ElevenLabsKey string `json:"elevenLabsKey"`
}

type ElevenLabsVoiceOption struct {
	VoiceID     string            `json:"voiceId"`
	Name        string            `json:"name"`
	Category    string            `json:"category,omitempty"`
	Description string            `json:"description,omitempty"`
	PreviewURL  string            `json:"previewUrl,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ElevenLabsVoicesResponse struct {
	Voices []ElevenLabsVoiceOption `json:"voices"`
}

func listElevenLabsVoicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ElevenLabsVoicesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(req.ElevenLabsKey)
	if apiKey == "" {
		http.Error(w, "elevenLabsKey is required", http.StatusBadRequest)
		return
	}

	voices, err := fetchElevenLabsVoices(apiKey)
	if err != nil {
		msg := fmt.Sprintf("[ERROR] ElevenLabs voices fetch failed: %v", err)
		fmt.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(ElevenLabsVoicesResponse{Voices: voices})
}

func generateAudioHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ElevenLabsAudioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Prompt = strings.TrimSpace(req.Prompt)
	audioType := normalizeAudioType(req.AudioType)
	apiKey := strings.TrimSpace(req.ElevenLabsKey)
	log.Printf("[ELEVENLABS] generation request type=%s prompt=%q", audioType, req.Prompt)

	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	if apiKey == "" {
		http.Error(w, "elevenLabsKey is required", http.StatusBadRequest)
		return
	}

	var (
		audioBytes []byte
		mimeType   string
		err        error
	)

	switch audioType {
	case "voice":
		audioBytes, mimeType, err = callElevenLabsVoice(req, apiKey)
	case "sound":
		audioBytes, mimeType, err = callElevenLabsSound(req, apiKey)
	case "music":
		audioBytes, mimeType, err = callElevenLabsMusic(req, apiKey)
	default:
		http.Error(w, "audioType must be one of: voice, sound, music", http.StatusBadRequest)
		return
	}

	if err != nil {
		msg := fmt.Sprintf("[ERROR] ElevenLabs %s generation failed: %v", audioType, err)
		fmt.Println(msg)
		log.Printf("[ELEVENLABS] generation failed type=%s err=%s", audioType, sanitizeElevenLabsError(err))
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	audioOutputDir := filepath.Join("saved_images", "audio")
	if err := os.MkdirAll(audioOutputDir, 0755); err != nil {
		fmt.Printf("[WARN] Failed creating %s folder: %v\n", audioOutputDir, err)
	}

	ext := extensionForAudioMimeType(mimeType)
	filename := fmt.Sprintf("%s_%d.%s", audioType, time.Now().UnixNano(), ext)
	savedPath := filepath.Join(audioOutputDir, filename)

	if err := os.WriteFile(savedPath, audioBytes, 0644); err != nil {
		msg := fmt.Sprintf("[ERROR] writing audio file: %v", err)
		fmt.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	resp := ElevenLabsAudioResponse{
		Prompt:    req.Prompt,
		AudioType: audioType,
		Filename:  filename,
		SavedPath: savedPath,
		Audio:     fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioBytes)),
		MimeType:  mimeType,
	}

	log.Printf("[ELEVENLABS] generation success type=%s bytes=%d mime=%s file=%s", audioType, len(audioBytes), mimeType, filename)
	json.NewEncoder(w).Encode(resp)
}

func callElevenLabsVoice(req ElevenLabsAudioRequest, apiKey string) ([]byte, string, error) {
	voiceID := strings.TrimSpace(req.VoiceID)
	if voiceID == "" {
		voiceID = defaultElevenVoiceID
	}

	modelID := normalizeElevenLabsVoiceModel(req.VoiceModel)
	if modelID == "" {
		modelID = defaultElevenVoiceModel
	}

	params := neturl.Values{}
	outputFormat := strings.TrimSpace(req.OutputFormat)
	if outputFormat == "" {
		outputFormat = defaultElevenOutputFormat
	}
	params.Set("output_format", outputFormat)

	body := map[string]interface{}{
		"text":     req.Prompt,
		"model_id": modelID,
	}

	endpoint := fmt.Sprintf("%s/v1/text-to-speech/%s", elevenLabsBaseURL, neturl.PathEscape(voiceID))
	audioBytes, mimeType, err := callElevenLabsBinaryAPI(endpoint, params, body, apiKey)
	if err != nil && modelID != defaultElevenVoiceModel && isElevenLabsModelNotFound(err) {
		log.Printf("[ELEVENLABS][VOICE] model=%s not found, retrying with %s", modelID, defaultElevenVoiceModel)
		body["model_id"] = defaultElevenVoiceModel
		return callElevenLabsBinaryAPI(endpoint, params, body, apiKey)
	}
	return audioBytes, mimeType, err
}

func callElevenLabsSound(req ElevenLabsAudioRequest, apiKey string) ([]byte, string, error) {
	params := neturl.Values{}
	outputFormat := strings.TrimSpace(req.OutputFormat)
	if outputFormat == "" {
		outputFormat = defaultElevenOutputFormat
	}
	params.Set("output_format", outputFormat)

	body := map[string]interface{}{
		"text": req.Prompt,
	}
	if modelID := strings.TrimSpace(req.SoundModel); modelID != "" {
		body["model_id"] = modelID
	}

	if req.DurationSeconds > 0 {
		body["duration_seconds"] = req.DurationSeconds
	}
	if req.PromptInfluence != nil {
		body["prompt_influence"] = *req.PromptInfluence
	}
	if req.Loop {
		body["loop"] = true
	}

	return callElevenLabsBinaryAPI(elevenLabsBaseURL+"/v1/sound-generation", params, body, apiKey)
}

func callElevenLabsMusic(req ElevenLabsAudioRequest, apiKey string) ([]byte, string, error) {
	params := neturl.Values{}
	outputFormat := strings.TrimSpace(req.OutputFormat)
	if outputFormat == "" {
		outputFormat = defaultElevenOutputFormat
	}
	params.Set("output_format", outputFormat)

	body := map[string]interface{}{
		"prompt": req.Prompt,
	}
	if modelID := strings.TrimSpace(req.MusicModel); modelID != "" {
		body["model_id"] = modelID
	}

	if req.DurationSeconds > 0 {
		body["music_length_ms"] = int(req.DurationSeconds * 1000.0)
	}
	if req.ForceInstrumental {
		body["is_instrumental"] = true
	}

	return callElevenLabsBinaryAPI(elevenLabsBaseURL+"/v1/music", params, body, apiKey)
}

func callElevenLabsBinaryAPI(endpoint string, query neturl.Values, body map[string]interface{}, apiKey string) ([]byte, string, error) {
	url := endpoint
	if len(query) > 0 {
		url += "?" + query.Encode()
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	request, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("xi-api-key", apiKey)
	modelID, _ := body["model_id"].(string)
	promptText := ""
	if text, ok := body["text"].(string); ok {
		promptText = text
	} else if prompt, ok := body["prompt"].(string); ok {
		promptText = prompt
	}
	log.Printf("[ELEVENLABS] request endpoint=%s model=%s prompt=%q", endpoint, strings.TrimSpace(modelID), promptText)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to call ElevenLabs API: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read ElevenLabs response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("[ELEVENLABS] error endpoint=%s status=%d body=%s", endpoint, response.StatusCode, truncateElevenLabsLog(string(data), 320))
		return nil, "", fmt.Errorf("ElevenLabs API error (status %d): %s", response.StatusCode, string(data))
	}

	mimeType := inferAudioMimeType(response.Header.Get("Content-Type"), data)
	log.Printf("[ELEVENLABS] success endpoint=%s status=%d bytes=%d mime=%s", endpoint, response.StatusCode, len(data), mimeType)
	return data, mimeType, nil
}

func fetchElevenLabsVoices(apiKey string) ([]ElevenLabsVoiceOption, error) {
	request, err := http.NewRequest("GET", elevenLabsBaseURL+"/v1/voices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("xi-api-key", apiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to call ElevenLabs API: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ElevenLabs response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("ElevenLabs API error (status %d): %s", response.StatusCode, string(data))
	}

	var decoded struct {
		Voices []struct {
			VoiceID     string            `json:"voice_id"`
			Name        string            `json:"name"`
			Category    string            `json:"category"`
			Description string            `json:"description"`
			PreviewURL  string            `json:"preview_url"`
			Labels      map[string]string `json:"labels"`
		} `json:"voices"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("failed to parse ElevenLabs response: %w", err)
	}

	voices := make([]ElevenLabsVoiceOption, 0, len(decoded.Voices))
	for _, v := range decoded.Voices {
		if strings.TrimSpace(v.VoiceID) == "" {
			continue
		}
		voices = append(voices, ElevenLabsVoiceOption{
			VoiceID:     v.VoiceID,
			Name:        v.Name,
			Category:    v.Category,
			Description: v.Description,
			PreviewURL:  v.PreviewURL,
			Labels:      v.Labels,
		})
	}

	sort.SliceStable(voices, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(voices[i].Name))
		right := strings.ToLower(strings.TrimSpace(voices[j].Name))
		if left == right {
			return voices[i].VoiceID < voices[j].VoiceID
		}
		return left < right
	})

	return voices, nil
}

func normalizeAudioType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "voice", "speech", "tts":
		return "voice"
	case "sound", "sfx", "sound_effect":
		return "sound"
	case "music", "song":
		return "music"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeElevenLabsVoiceModel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	switch normalized {
	case "default", "standard", "base", "multilingual", "multilingual_v2":
		return defaultElevenVoiceModel
	default:
		return strings.TrimSpace(value)
	}
}

func isElevenLabsModelNotFound(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "model_not_found") ||
		(strings.Contains(lower, "model id") && strings.Contains(lower, "does not exist"))
}

func sanitizeElevenLabsError(err error) string {
	if err == nil {
		return ""
	}
	return truncateElevenLabsLog(strings.ReplaceAll(strings.TrimSpace(err.Error()), "\n", " "), 320)
}

func truncateElevenLabsLog(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= max {
		return trimmed
	}
	if max < 4 {
		return trimmed[:max]
	}
	return trimmed[:max-3] + "..."
}

func inferAudioMimeType(contentType string, data []byte) string {
	trimmed := strings.TrimSpace(strings.ToLower(contentType))
	if trimmed != "" {
		if semi := strings.Index(trimmed, ";"); semi >= 0 {
			trimmed = strings.TrimSpace(trimmed[:semi])
		}
		if trimmed != "" && trimmed != "application/octet-stream" {
			return trimmed
		}
	}

	if len(data) >= 3 {
		if string(data[:3]) == "ID3" {
			return "audio/mpeg"
		}
		// MPEG frame sync.
		if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
			return "audio/mpeg"
		}
	}

	if len(data) >= 12 {
		if string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
			return "audio/wav"
		}
	}

	if len(data) >= 4 {
		if string(data[:4]) == "OggS" {
			return "audio/ogg"
		}
		if string(data[:4]) == "fLaC" {
			return "audio/flac"
		}
	}

	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		return "audio/mp4"
	}

	return "audio/mpeg"
}

func extensionForAudioMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return "wav"
	case "audio/ogg":
		return "ogg"
	case "audio/flac":
		return "flac"
	case "audio/mp4", "audio/x-m4a":
		return "m4a"
	default:
		return "mp3"
	}
}
