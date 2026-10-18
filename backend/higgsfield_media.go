package main

// Higgsfield as a Studio video source. A curated set of models, called through the person's own
// Higgsfield account (see higgsfield.go). Each generation is priced first with the tool's free
// `get_cost` preflight, so Studio can show and confirm the credit cost before anything renders.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const higgsfieldSourceID = "higgsfield"

func higgsfieldConnected() bool {
	credential, err := higgsfieldLoadCredential()
	return err == nil && credential != nil
}

// Curated models. Prices come from Higgsfield's preflight at request time, so none are hardcoded here.
func higgsfieldVideoModels() []studioVideoModelCapability {
	seedanceRatios := []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
	model := func(id, name string, minSeconds, maxSeconds, defaultSeconds int, resolutions, ratios []string, defaultResolution string) studioVideoModelCapability {
		return studioVideoModelCapability{SourceID: higgsfieldSourceID, ID: id, Name: name, MinDurationSeconds: minSeconds, MaxDurationSeconds: maxSeconds,
			DefaultDurationSeconds: defaultSeconds, Resolutions: resolutions, DefaultResolution: defaultResolution, AspectRatios: ratios,
			PricesPerSecondUSD: map[string]float64{}, PricingURL: "https://higgsfield.ai/pricing", SupportsTextToVideo: true}
	}
	return []studioVideoModelCapability{
		// Higgsfield retired Seedance 2.0 Mini on October 10, 2026; Kling 3.0 Turbo is now the cheapest good default.
		model("kling3_0_turbo", "Kling 3.0 Turbo", 3, 15, 5, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, "720p"),
		// Kling picks its resolution from a quality mode: std renders 720p, pro renders 1080p.
		model("kling3_0", "Kling 3.0", 3, 15, 5, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, "720p"),
		model("seedance_2_5", "Seedance 2.5", 4, 30, 5, []string{"480p", "720p", "1080p"}, seedanceRatios, "720p"),
	}
}

func higgsfieldVideoSource() studioVideoSourceCapability {
	return studioVideoSourceCapability{ID: higgsfieldSourceID, Name: "Higgsfield account", Provider: "Higgsfield", Connected: higgsfieldConnected(),
		Notice: "Uses your Higgsfield plan credits. Glowbom checks the cost before each video.", Models: higgsfieldVideoModels()}
}

// higgsfieldVideoParams builds generate_video params for one Studio request. mediaID is "" for text-to-video.
func higgsfieldVideoParams(options studioVideoOptions, prompt, mediaID string) map[string]any {
	return higgsfieldVideoParamsWithReferences(options, prompt, mediaID, "", "")
}

// Seedance 2.5 can also take a reference video (movement) and a reference voice in one request.
func higgsfieldVideoParamsWithReferences(options studioVideoOptions, prompt, mediaID, videoID, audioID string) map[string]any {
	params := map[string]any{"model": options.ModelID, "prompt": prompt, "aspect_ratio": options.AspectRatio, "duration": options.DurationSeconds, "use_unlim": false}
	switch options.ModelID {
	case "kling3_0":
		params["mode"] = map[bool]string{true: "pro", false: "std"}[options.Resolution == "1080p"]
		params["sound"] = "on"
	case "seedance_2_5":
		params["resolution"] = options.Resolution
		params["generate_audio"] = true
		params["mode"] = map[bool]string{true: "omni_reference", false: "t2v"}[mediaID != "" || videoID != "" || audioID != ""]
	default: // kling3_0_turbo takes a plain resolution
		params["resolution"] = options.Resolution
	}
	medias := []map[string]string{}
	if mediaID != "" {
		medias = append(medias, map[string]string{"value": mediaID, "role": "start_image"})
	}
	if videoID != "" {
		medias = append(medias, map[string]string{"value": videoID, "role": "video_references"})
	}
	if audioID != "" {
		medias = append(medias, map[string]string{"value": audioID, "role": "audio_references"})
	}
	if len(medias) > 0 {
		params["medias"] = medias
	}
	return params
}

// The MCP tools take {params, context}; context is analytics only.
func higgsfieldGenerateVideoCall(ctx context.Context, params map[string]any) (json.RawMessage, error) {
	return higgsfieldCallTool(ctx, "generate_video", map[string]any{"params": params, "context": "Studio video generation in Glowbom Desktop."})
}

type higgsfieldCostResponse struct {
	Cost struct {
		Credits *float64 `json:"credits"`
	} `json:"cost"`
}

func parseHiggsfieldCredits(raw json.RawMessage, kind string) (float64, error) {
	var cost higgsfieldCostResponse
	if json.Unmarshal(raw, &cost) != nil || cost.Cost.Credits == nil || math.IsNaN(*cost.Cost.Credits) || math.IsInf(*cost.Cost.Credits, 0) || *cost.Cost.Credits < 0 {
		return 0, fmt.Errorf("Higgsfield did not return a valid price for this %s.", kind)
	}
	return *cost.Cost.Credits, nil
}

func higgsfieldVideoEstimate(ctx context.Context, options studioVideoOptions) (float64, error) {
	params := higgsfieldVideoParams(options, "Studio cost check", "")
	params["get_cost"] = true
	raw, err := higgsfieldGenerateVideoCall(ctx, params)
	if err != nil {
		return 0, err
	}
	return parseHiggsfieldCredits(raw, "video")
}

type higgsfieldSubmitResponse struct {
	Results []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"results"`
	Jobs []struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"jobs"`
	PresetRecommendation *struct {
		PresetID string `json:"preset_id"`
	} `json:"preset_recommendation"`
	// Higgsfield asks before submitting in some cases, for example when a prompt looks like one of its
	// presets: {"notice":{"type":"preset_recommendation","message":…,"data":{"preset":{"id":…}},
	// "retry_literal_with":{"declined_preset_id":…}}}.
	Notice *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Data    struct {
			Preset struct {
				ID string `json:"id"`
			} `json:"preset"`
		} `json:"data"`
		RetryLiteralWith struct {
			DeclinedPresetID string `json:"declined_preset_id"`
		} `json:"retry_literal_with"`
	} `json:"notice"`
	Error string `json:"error"`
}

// declinedPreset returns the preset to decline so the request runs as written, if Higgsfield suggested one.
func (r higgsfieldSubmitResponse) declinedPreset() string {
	if r.PresetRecommendation != nil && r.PresetRecommendation.PresetID != "" {
		return r.PresetRecommendation.PresetID
	}
	if r.Notice != nil && r.Notice.Type == "preset_recommendation" {
		if r.Notice.RetryLiteralWith.DeclinedPresetID != "" {
			return r.Notice.RetryLiteralWith.DeclinedPresetID
		}
		return r.Notice.Data.Preset.ID
	}
	return ""
}

// A request Higgsfield did not accept, so no job (and no charge) was created.
type higgsfieldNotStartedError struct{ reason string }

func (e *higgsfieldNotStartedError) Error() string { return e.reason }

// Submits one generation and returns its job ID. Higgsfield may suggest a preset instead of
// submitting; Studio requests are explicit, so decline it and submit as written.
func higgsfieldSubmit(ctx context.Context, call func(context.Context, map[string]any) (json.RawMessage, error), params map[string]any, kind string) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := call(ctx, params)
		var refused *higgsfieldToolError
		if errors.As(err, &refused) {
			// Higgsfield refused the request itself, so no job exists and nothing was charged.
			log.Printf("[HIGGSFIELD] %s refused: %s", kind, sanitizeProviderError(err))
			return "", &higgsfieldNotStartedError{sanitizeProviderError(err)}
		}
		if err != nil {
			// A transport failure leaves the outcome unknown; the caller must not resubmit blindly.
			log.Printf("[HIGGSFIELD] %s request failed: %s", kind, sanitizeProviderError(err))
			return "", err
		}
		var submitted higgsfieldSubmitResponse
		if json.Unmarshal(raw, &submitted) != nil {
			log.Printf("[HIGGSFIELD] unexpected %s reply: %.300s", kind, raw)
			return "", fmt.Errorf("Higgsfield sent an unexpected reply to the %s request. Its status is unknown", kind)
		}
		for _, result := range submitted.Results {
			if result.ID != "" {
				return result.ID, nil
			}
		}
		for _, job := range submitted.Jobs {
			if job.JobID != "" {
				return job.JobID, nil
			}
		}
		if preset := submitted.declinedPreset(); preset != "" {
			if attempt == 0 {
				params["declined_preset_id"] = preset
				continue
			}
			return "", &higgsfieldNotStartedError{fmt.Sprintf("Higgsfield requested a preset instead of starting the %s", kind)}
		}
		if submitted.Error != "" {
			return "", &higgsfieldNotStartedError{"Higgsfield: " + submitted.Error}
		}
		if submitted.Notice != nil && submitted.Notice.Message != "" {
			if submitted.Notice.Type == "preset_recommendation" {
				return "", &higgsfieldNotStartedError{"Higgsfield: " + submitted.Notice.Message}
			}
			return "", fmt.Errorf("Higgsfield returned an unexpected notice for the %s request. Its status is unknown", kind)
		}
		log.Printf("[HIGGSFIELD] %s reply without a job: %.300s", kind, raw)
		break
	}
	// Missing job details do not prove that the provider declined the paid request.
	return "", fmt.Errorf("Higgsfield did not return a job ID for the %s request. Its status is unknown", kind)
}

type higgsfieldUploadResponse struct {
	Uploads []struct {
		UploadURL   string `json:"upload_url"`
		MediaID     string `json:"media_id"`
		ContentType string `json:"content_type"`
	} `json:"uploads"`
}

// Uploads a starting image (base64) and returns its confirmed Higgsfield media ID.
func higgsfieldUploadImage(ctx context.Context, data, mimeType string) (string, error) {
	body, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(body) == 0 {
		return "", errors.New("Invalid starting image")
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(body)
	}
	extension := ".png"
	if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		extension, mimeType = ".jpg", "image/jpeg"
	} else {
		mimeType = "image/png"
	}
	return higgsfieldUploadMedia(ctx, body, "glowbom-reference"+extension, mimeType, "image")
}

// Uploads bytes through media_upload → PUT → media_confirm. kind is Higgsfield's media type ("image" or "audio").
func higgsfieldUploadMedia(ctx context.Context, body []byte, filename, mimeType, kind string) (string, error) {
	// Higgsfield's upload host sometimes answers with a passing Cloudflare 5xx (520 on October 10).
	// An upload starts no job and costs nothing, so try again with a fresh upload address.
	var lastErr error
	for attempt := 0; attempt < higgsfieldUploadAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("[HIGGSFIELD] retrying the %s upload after: %s", kind, sanitizeProviderError(lastErr))
			timer := time.NewTimer(time.Duration(attempt) * higgsfieldUploadRetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		mediaID, retry, err := higgsfieldUploadMediaOnce(ctx, body, filename, mimeType, kind)
		if err == nil {
			return mediaID, nil
		}
		if !retry || ctx.Err() != nil {
			return "", err
		}
		lastErr = err
	}
	return "", lastErr
}

var (
	higgsfieldUploadAttempts   = 3
	higgsfieldUploadRetryDelay = 2 * time.Second
)

// One upload: media_upload, PUT, media_confirm. retry reports a failure worth another attempt.
func higgsfieldUploadMediaOnce(ctx context.Context, body []byte, filename, mimeType, kind string) (string, bool, error) {
	raw, err := higgsfieldCallTool(ctx, "media_upload", map[string]any{"filename": filename, "content_type": mimeType})
	if err != nil {
		return "", false, err
	}
	var upload higgsfieldUploadResponse
	if json.Unmarshal(raw, &upload) != nil || len(upload.Uploads) == 0 || upload.Uploads[0].UploadURL == "" || upload.Uploads[0].MediaID == "" {
		return "", false, fmt.Errorf("Higgsfield could not prepare the %s upload", kind)
	}
	target := upload.Uploads[0]
	if !higgsfieldTrustedHost(target.UploadURL) {
		return "", false, errors.New("Higgsfield returned an unexpected upload address")
	}
	if target.ContentType != "" {
		mimeType = target.ContentType
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.UploadURL, bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("User-Agent", higgsfieldUserAgent())
	// References can be tens of megabytes: allow about 2 minutes plus 4 seconds per megabyte
	// (a 2 Mbit/s upload). The caller's context still bounds the whole request.
	timeout := 2*time.Minute + time.Duration(len(body)>>20)*4*time.Second
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		// Keep the signed upload address out of the message.
		var urlErr *neturl.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return "", true, fmt.Errorf("could not upload the %s: %w", kind, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", resp.StatusCode >= 500, fmt.Errorf("Higgsfield rejected the %s (HTTP %d)", kind, resp.StatusCode)
	}
	if _, err := higgsfieldCallTool(ctx, "media_confirm", map[string]any{"media_id": target.MediaID, "type": kind}); err != nil {
		return "", false, err
	}
	return target.MediaID, false, nil
}

// Uploads and results come from Higgsfield's own hosts and its CDN, never from an arbitrary address.
func higgsfieldTrustedHost(raw string) bool {
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return false
	}
	if higgsfieldTestHost != "" && parsed.Host == higgsfieldTestHost {
		return true
	}
	if parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "higgsfield.ai" || strings.HasSuffix(host, ".higgsfield.ai") || strings.HasSuffix(host, ".cloudfront.net")
}

// Tests serve uploads and results from a local http server.
var higgsfieldTestHost = ""

func startHiggsfieldVideo(ctx context.Context, options studioVideoOptions, prompt string, images []VeoImageInput) (*VeoGenerationResponse, error) {
	// Uploads happen before anything is submitted, so a failure here never starts a job.
	notStarted := func(err error) error {
		if err != nil {
			log.Printf("[HIGGSFIELD] video references did not upload: %s", sanitizeProviderError(err))
		}
		if err == nil || ctx.Err() != nil || errors.Is(err, errHiggsfieldSignedOut) {
			return err
		}
		return &higgsfieldNotStartedError{sanitizeProviderError(err)}
	}
	mediaID := ""
	if len(images) > 0 {
		var err error
		if mediaID, err = higgsfieldUploadImage(ctx, images[0].Data, images[0].MimeType); err != nil {
			return nil, notStarted(err)
		}
	}
	videoID, audioID, err := higgsfieldUploadVideoReferences(ctx, options)
	if err != nil {
		return nil, notStarted(err)
	}
	jobID, err := higgsfieldSubmit(ctx, higgsfieldGenerateVideoCall, higgsfieldVideoParamsWithReferences(options, higgsfieldReferencePrompt(options, prompt), mediaID, videoID, audioID), "video")
	if err != nil {
		return nil, err
	}
	return &VeoGenerationResponse{OperationID: jobID, Message: "Higgsfield is rendering the video."}, nil
}

type higgsfieldJobsResponse struct {
	Jobs []struct {
		JobID     string `json:"job_id"`
		Status    string `json:"status"`
		ResultURL string `json:"result_url"`
		Error     string `json:"error"`
	} `json:"jobs"`
}

func pollHiggsfieldVideo(ctx context.Context, jobID string) (*VeoPollResponse, error) {
	raw, err := higgsfieldCallTool(ctx, "jobs_wait", map[string]any{"jobs": []map[string]any{{"index": 0, "job_id": jobID}}, "timeout_seconds": 0})
	if err != nil {
		debugHiggsfieldPoll(ctx, "unknown", err)
		return nil, err
	}
	var jobs higgsfieldJobsResponse
	if json.Unmarshal(raw, &jobs) != nil || len(jobs.Jobs) == 0 {
		debugHiggsfieldPoll(ctx, "unknown", errors.New("invalid status response"))
		return nil, errors.New("Higgsfield sent an unexpected job status")
	}
	job := jobs.Jobs[0]
	switch strings.ToLower(job.Status) {
	case "completed", "succeeded", "success":
		if job.ResultURL == "" {
			debugHiggsfieldPoll(ctx, "failed", &studioVideoFailure{category: "missing_result", terminal: true})
			return &VeoPollResponse{Done: true, Status: "failed", Error: "Higgsfield finished without a video file."}, nil
		}
		debugHiggsfieldPoll(ctx, "complete", nil)
		return &VeoPollResponse{Done: true, Status: "completed", VideoURL: job.ResultURL}, nil
	case "failed", "error", "canceled", "cancelled", "nsfw", "rejected":
		err := higgsfieldTerminalVideoFailure(job.Status, job.Error)
		debugHiggsfieldPoll(ctx, "failed", err)
		return nil, err
	default:
		debugHiggsfieldPoll(ctx, "pending", nil)
		return &VeoPollResponse{Done: false, Status: "pending"}, nil
	}
}

// Provider errors can echo prompts and signed URLs. Classify them internally
// and expose only fixed messages, without guessing moderation or account limits.
func higgsfieldTerminalVideoFailure(status, detail string) error {
	status, detail = strings.ToLower(status), strings.ToLower(detail)
	category := "provider_failed_unknown"
	message := "Higgsfield reported that this video failed. Its detailed reason is unavailable here. Your request is saved."
	switch {
	case status == "canceled" || status == "cancelled":
		category, message = "provider_cancelled", "Higgsfield reported that this video was cancelled. Your request is saved."
	case status == "nsfw" || strings.Contains(detail, "moderation") || strings.Contains(detail, "content policy") || strings.Contains(detail, "nsfw"):
		category, message = "provider_moderation", "Higgsfield rejected this video during its content checks. Review your scene and reference media before starting a new request."
	case strings.Contains(detail, "insufficient credits") || strings.Contains(detail, "insufficient_credits") || strings.Contains(detail, "not enough credits"):
		category, message = "provider_credits", "Higgsfield reported insufficient credits for this video. Check your Higgsfield balance before starting a new request."
	case strings.Contains(detail, "timed out") || strings.Contains(detail, "timeout") || strings.Contains(detail, "time limit"):
		category, message = "provider_timeout", "Higgsfield reported that rendering this video timed out. Your request is saved; this provider job cannot be resumed."
	case strings.Contains(detail, "temporarily unavailable") || strings.Contains(detail, "service_unavailable") || strings.Contains(detail, "internal error") || strings.Contains(detail, "internal_error"):
		category, message = "provider_unavailable", "Higgsfield reported a service error while rendering this video. Your request is saved."
	case status == "rejected":
		category, message = "provider_rejected", "Higgsfield rejected this video request. Its detailed reason is unavailable here. Your request is saved."
	}
	return &studioVideoFailure{message: message, category: category, terminal: true}
}

func downloadHiggsfieldVideo(ctx context.Context, videoURL string) ([]byte, string, error) {
	return downloadHiggsfieldMedia(ctx, videoURL, "video/")
}

// Downloads a finished file from Higgsfield's CDN. kindPrefix is "video/", "image/" or "audio/".
func downloadHiggsfieldMedia(ctx context.Context, videoURL, kindPrefix string) ([]byte, string, error) {
	if !higgsfieldTrustedHost(videoURL) {
		return nil, "", errors.New("Higgsfield returned an unexpected file address")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", higgsfieldUserAgent())
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Higgsfield download failed (HTTP %d)", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil {
		return nil, "", err
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(mimeType, kindPrefix) {
		mimeType = http.DetectContentType(body)
	}
	if !strings.HasPrefix(mimeType, kindPrefix) {
		mimeType = map[string]string{"video/": "video/mp4", "image/": "image/png", "audio/": "audio/mpeg"}[kindPrefix]
	}
	return body, mimeType, nil
}

// POST /studio/higgsfield/estimate {kind:"video", modelId, durationSeconds, resolution, aspectRatio},
// {kind:"image", modelId, resolution, aspectRatio, quality} or {kind:"audio", prompt, voiceId, voiceType, speechRate}
// → {credits, balance?}. Free: it only asks Higgsfield for the price.
func higgsfieldEstimateHandler(w http.ResponseWriter, r *http.Request) {
	if token := glowbomServerToken(); token == "" || !hasValidGlowbomServerToken(r, token) {
		http.Error(w, "Local authentication required.", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	var request struct {
		Kind            string `json:"kind"`
		ModelID         string `json:"modelId"`
		DurationSeconds int    `json:"durationSeconds"`
		Resolution      string `json:"resolution"`
		AspectRatio     string `json:"aspectRatio"`
		Quality         string `json:"quality"`
		higgsfieldSpeechRequest
	}
	if json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&request) != nil || (request.Kind != "" && request.Kind != "video" && request.Kind != "image" && request.Kind != "audio") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid estimate request."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var credits float64
	var err error
	if request.Kind == "audio" {
		var speech higgsfieldSpeechRequest
		if speech, err = normalizeHiggsfieldSpeech(request.higgsfieldSpeechRequest); err == nil {
			credits, err = higgsfieldSpeechEstimate(ctx, speech)
		}
	} else if request.Kind == "image" {
		var options studioImageOptions
		options, err = normalizeStudioImageOptions(studioImageOptions{SourceID: higgsfieldSourceID, ModelID: request.ModelID, AspectRatio: request.AspectRatio, Resolution: request.Resolution, Quality: request.Quality})
		if err == nil {
			credits, err = higgsfieldImageEstimate(ctx, options)
		}
	} else {
		var options studioVideoOptions
		options, err = normalizeStudioVideoOptions(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: request.ModelID, DurationSeconds: request.DurationSeconds, Resolution: request.Resolution, AspectRatio: request.AspectRatio})
		if err == nil {
			credits, err = higgsfieldVideoEstimate(ctx, options)
		}
	}
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": sanitizeProviderError(err)})
		return
	}
	response := map[string]any{"credits": credits}
	if balance, err := glowbomHiggsfield.readBalance(ctx, false); err == nil {
		response["balance"] = balance.credits
	}
	writeJSON(w, response)
}

// ---------- images ----------

// Curated image models. Studio's shapes are a subset of what both models accept.
func higgsfieldImageModels() []studioImageModelCapability {
	ratios := []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
	model := func(id, name string, qualities []string, defaultQuality string) studioImageModelCapability {
		return studioImageModelCapability{SourceID: higgsfieldSourceID, ID: id, Name: name, AspectRatios: ratios, DefaultAspectRatio: "1:1",
			Resolutions: []string{"1k", "2k", "4k"}, DefaultResolution: "1k", Qualities: qualities, DefaultQuality: defaultQuality,
			PricingNotice: "Higgsfield prices each image in plan credits. Glowbom checks the cost first.", PricingURL: "https://higgsfield.ai/pricing"}
	}
	return []studioImageModelCapability{
		model("gpt_image_2_5", "GPT Image 2.5", []string{"low", "medium", "high"}, "low"),
		model("nano_banana_2_1", "Nano Banana 2.1", []string{}, ""),
	}
}

func higgsfieldImageSource() studioImageSourceCapability {
	return studioImageSourceCapability{ID: higgsfieldSourceID, Name: "Higgsfield account", Provider: "Higgsfield", Connected: higgsfieldConnected(),
		Notice: "Uses your Higgsfield plan credits. Reference images keep a face or product consistent.", Models: higgsfieldImageModels()}
}

func higgsfieldImageParams(o studioImageOptions, prompt, mediaID string) map[string]any {
	params := map[string]any{"model": o.ModelID, "prompt": prompt, "aspect_ratio": o.AspectRatio, "resolution": strings.ToLower(o.Resolution), "use_unlim": false}
	if o.Quality != "" {
		params["quality"] = o.Quality
	}
	if mediaID != "" {
		params["medias"] = []map[string]string{{"value": mediaID, "role": "image_references"}}
	}
	return params
}

func higgsfieldGenerateImageCall(ctx context.Context, params map[string]any) (json.RawMessage, error) {
	return higgsfieldCallTool(ctx, "generate_image", map[string]any{"params": params, "context": "Studio image generation in Glowbom Desktop."})
}

func higgsfieldImageEstimate(ctx context.Context, o studioImageOptions) (float64, error) {
	params := higgsfieldImageParams(o, "Studio cost check", "")
	params["get_cost"] = true
	raw, err := higgsfieldGenerateImageCall(ctx, params)
	if err != nil {
		return 0, err
	}
	return parseHiggsfieldImageCredits(raw)
}

// Generates one image and returns it as a data URI. Studio's image request waits for the
// finished file, so this submits, polls, and downloads within the caller's deadline.
func generateHiggsfieldImage(ctx context.Context, o studioImageOptions, prompt, reference string) (string, error) {
	mediaID := ""
	if reference != "" {
		data, mimeType := reference, ""
		if strings.HasPrefix(reference, "data:") {
			parts := strings.SplitN(reference, ",", 2)
			if len(parts) != 2 {
				return "", errors.New("Invalid reference image.")
			}
			mimeType = strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
			data = parts[1]
		}
		var err error
		if mediaID, err = higgsfieldUploadImage(ctx, data, mimeType); err != nil {
			return "", err
		}
	}
	jobID, err := higgsfieldSubmit(ctx, higgsfieldGenerateImageCall, higgsfieldImageParams(o, prompt, mediaID), "image")
	if err != nil {
		return "", err
	}
	for {
		poll, err := pollHiggsfieldVideo(ctx, jobID) // jobs_wait is the same for every media type
		if err != nil {
			return "", err
		}
		if poll.Done {
			if poll.Status != "completed" {
				return "", errors.New("Higgsfield could not finish this image. Check its moderation or account limits.")
			}
			body, mimeType, err := downloadHiggsfieldMedia(ctx, poll.VideoURL, "image/")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(body)), nil
		}
		timer := time.NewTimer(higgsfieldImagePollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

var higgsfieldImagePollInterval = 2 * time.Second
