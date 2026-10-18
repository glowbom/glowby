package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

// Only forward known account messages, never raw provider output or credentials.
func companionStudioFailure(body []byte) string {
	message := strings.TrimSpace(string(body))
	for _, code := range []string{"sign_in_required", "allowance_required", "rate_limited", "invalid_image_request", "cli_update_required", "image_generation_failed"} {
		if message == glowbomImageError(&accountCommandFailure{code: code}).Error() {
			return message
		}
	}
	for _, safe := range []string{
		"Update or install the Glowbom CLI to use Glowbom images.",
		"Restart Glowbom with its secure local connection to use account images.",
		"Sign in from Account to use Glowbom images.",
		"Could not check your Glowbom account. Open Account and try again.",
		"Could not check your Glowbom account. Update the Glowbom CLI and try again.",
		"Your Glowbom account is busy. Wait for its current operation to finish.",
		"The Glowbom reference photo is too large. Use a smaller photo.",
		"Choose a valid PNG or JPEG reference photo.",
		"Glowbom reference photos must be PNG or JPEG images up to 4 megapixels. Use a smaller photo.",
		"Choose an aspect ratio supported by this image model.",
	} {
		if message == safe {
			return message
		}
	}
	return "Desktop could not finish this request. Check its connection and try again."
}

func validateCompanionStudioReference(id, inline string) error {
	if id != "" && (normalizedStudioUUID(id) == "" || inline != "") {
		return errors.New("Choose one Studio image or upload one reference.")
	}
	if inline == "" {
		return nil
	}
	if len(inline) > base64.StdEncoding.EncodedLen(companionAttachmentMaxBytes)+64 {
		return errors.New("Choose a reference smaller than 2 MB.")
	}
	var mime, payload string
	for _, candidate := range []string{"image/jpeg", "image/png"} {
		prefix := "data:" + candidate + ";base64,"
		if strings.HasPrefix(inline, prefix) {
			mime, payload = candidate, strings.TrimPrefix(inline, prefix)
			break
		}
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || mime == "" {
		return errors.New("Choose a JPEG or PNG reference.")
	}
	_, actualMIME, err := companionAttachmentImage(data)
	if err != nil {
		return err
	}
	if actualMIME != mime {
		return errors.New("This reference image could not be read.")
	}
	return nil
}

// Companion video requests carry only saved Desktop references and explicit review.
type companionVideoRequest struct {
	studioVideoOptions
	Prompt         string `json:"prompt"`
	ReferenceID    string `json:"referenceId,omitempty"`
	ReferenceImage string `json:"referenceImage,omitempty"`
	QuoteID        string `json:"quoteId,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
	Confirmed      bool   `json:"confirmed,omitempty"`
}

func normalizeCompanionVideo(request companionVideoRequest) (companionVideoRequest, error) {
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.SpokenText = strings.TrimSpace(request.SpokenText)
	if (request.Prompt == "" && request.SpokenText == "") || len(request.Prompt) > 4000 || request.SourceID == "" {
		return request, errors.New("Choose a Desktop video source and enter a description or words to say.")
	}
	if err := validateCompanionStudioReference(request.ReferenceID, request.ReferenceImage); err != nil {
		return request, err
	}
	for _, id := range []*string{&request.ReferenceID, &request.MotionVideoID, &request.VoiceAudioID} {
		if *id != "" {
			*id = normalizedStudioUUID(*id)
			if *id == "" {
				return request, errors.New("Choose a saved Studio reference.")
			}
		}
	}
	options, err := normalizeStudioVideoOptions(request.studioVideoOptions)
	if err != nil {
		return request, err
	}
	request.studioVideoOptions = options
	return request, nil
}

func (request companionVideoRequest) payload() map[string]any {
	return map[string]any{"prompt": request.Prompt, "sourceId": request.SourceID, "modelId": request.ModelID,
		"resolution": request.Resolution, "aspectRatio": request.AspectRatio, "durationSeconds": request.DurationSeconds,
		"referenceId": request.ReferenceID, "referenceImage": request.ReferenceImage, "useSavedKey": true,
		"motionVideoId": request.MotionVideoID, "voiceAudioId": request.VoiceAudioID,
		"higgsfieldVoiceId": request.HiggsfieldVoiceID, "spokenText": request.SpokenText}
}

func (s *companionSession) startStudioVideo(w http.ResponseWriter, r *http.Request) {
	var request companionVideoRequest
	if !companionDecode(w, r, &request, 3<<20) {
		return
	}
	request, err := normalizeCompanionVideo(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.SourceID == higgsfieldSourceID {
		s.startQuotedStudioVideo(w, r, request)
		return
	}
	if err := validateHiggsfieldVideoReferences(request.studioVideoOptions); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.startJob(w, r, "video", companionProject{}, "/studio/videos/generate", request.payload())
}
