package main

// Reference media for Higgsfield Seedance 2.5: a short video of the person for movement and a voice
// recording for the speech, both chosen from Studio. Higgsfield prices these requests the same as a
// plain video of the same length, so the free preflight still gives the cost.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	higgsfieldReferenceVideoMaxSeconds = 90
	higgsfieldReferenceVideoMaxBytes   = studioImportVideoMaxBytes
	higgsfieldReferenceAudioMaxBytes   = studioImportAudioMaxBytes
	higgsfieldSpokenTextMaxChars       = 600
	// Seedance 2.5 accepted a 20-second reference video and refused a 35-second one with HTTP 422
	// (October 10). Glowbom sends at most this much of each reference.
	higgsfieldReferenceSendSeconds = 20
)

func validateHiggsfieldVideoReferences(options studioVideoOptions) error {
	if options.MotionVideoID == "" && options.VoiceAudioID == "" && options.HiggsfieldVoiceID == "" && options.SpokenText == "" {
		return nil
	}
	if options.SourceID != higgsfieldSourceID || options.ModelID != "seedance_2_5" {
		return errors.New("Movement and voice references need Higgsfield Seedance 2.5.")
	}
	if len([]rune(options.SpokenText)) > higgsfieldSpokenTextMaxChars {
		return errors.New("Keep the words to say under 600 characters.")
	}
	if options.MotionVideoID != "" {
		record, err := findStudioAsset(options.MotionVideoID)
		if err != nil || record.MediaType != "video" {
			return errors.New("Choose a saved Studio video for the movement.")
		}
		if record.Duration != nil && *record.Duration > higgsfieldReferenceVideoMaxSeconds+0.5 {
			return errors.New("Use a movement video of 90 seconds or less. Trim it first, for example in QuickTime Player (Edit → Trim).")
		}
		if record.Duration != nil && *record.Duration > higgsfieldReferenceSendSeconds+0.5 && !higgsfieldCanTrimReferences() {
			return fmt.Errorf("Seedance accepts movement clips up to %d seconds. Trim this clip, or enable video editing in Studio so Glowbom can trim it for you.", higgsfieldReferenceSendSeconds)
		}
	}
	if options.VoiceAudioID != "" && options.HiggsfieldVoiceID != "" {
		return errors.New("Choose one voice reference.")
	}
	if options.HiggsfieldVoiceID != "" && !higgsfieldVoiceIDPattern.MatchString(options.HiggsfieldVoiceID) {
		return errors.New("Choose a Higgsfield voice.")
	}
	if options.VoiceAudioID != "" {
		record, err := findStudioAsset(options.VoiceAudioID)
		if err != nil || record.MediaType != "audio" {
			return errors.New("Choose saved Studio audio for the voice.")
		}
	}
	return nil
}

// Uploads the chosen Studio video and audio to Higgsfield and returns their media IDs.
func higgsfieldUploadVideoReferences(ctx context.Context, options studioVideoOptions) (string, string, error) {
	if err := validateHiggsfieldVideoReferences(options); err != nil {
		return "", "", err
	}
	upload := func(id, kind string, maxBytes int) (string, error) {
		if id == "" {
			return "", nil
		}
		record, err := findStudioAsset(id)
		if err != nil {
			return "", fmt.Errorf("The %s reference is no longer in Studio", kind)
		}
		fallback := map[string]string{"video": "video/mp4", "audio": "audio/mpeg"}[kind]
		data, mimeType, err := decodeBase64Payload(record.DataBase64, fallback)
		if err != nil || len(data) == 0 {
			return "", fmt.Errorf("The %s reference could not be opened", kind)
		}
		if len(data) > maxBytes {
			return "", fmt.Errorf("The %s reference is too large. Use a shorter clip", kind)
		}
		if strings.HasPrefix(record.MimeType, kind+"/") {
			mimeType = record.MimeType
		}
		filename, mimeType := higgsfieldReferenceFile("glowbom-"+kind+"-reference", kind, mimeType, data)
		silentMovement := kind == "video" && (options.VoiceAudioID != "" || options.HiggsfieldVoiceID != "")
		if trimmed, trimmedName, trimmedType, ok := higgsfieldPrepareReference(ctx, data, filename, kind, silentMovement); ok {
			data, filename, mimeType = trimmed, trimmedName, trimmedType
		} else if silentMovement {
			return "", errors.New("The movement clip could not be prepared without sound. Enable video editing in Studio and try again. No video generation was started")
		}
		return higgsfieldUploadMedia(ctx, data, filename, mimeType, kind)
	}
	videoID, err := upload(options.MotionVideoID, "video", higgsfieldReferenceVideoMaxBytes)
	if err != nil {
		return "", "", err
	}
	audioID, err := upload(options.VoiceAudioID, "audio", higgsfieldReferenceAudioMaxBytes)
	if err != nil {
		return "", "", err
	}
	if options.HiggsfieldVoiceID != "" {
		if audioID, err = higgsfieldUploadVoiceSample(ctx, options.HiggsfieldVoiceID); err != nil {
			return "", "", err
		}
	}
	return videoID, audioID, nil
}

// Seedance accepts an audio reference, not the voice_id/voice_type pair used by speech models.
// Use only the selected voice's preview. This guides generation without guaranteeing an exact match.
func higgsfieldUploadVoiceSample(ctx context.Context, voiceID string) (string, error) {
	voices, err := higgsfieldListVoices(ctx)
	if err != nil {
		return "", err
	}
	for _, voice := range voices {
		if voice.ID != voiceID {
			continue
		}
		if voice.Status != "" && voice.Status != "ready" && voice.Status != "completed" {
			return "", errors.New("This Higgsfield voice is not ready. Wait for it to finish before creating a video")
		}
		if voice.PreviewURL == "" {
			return "", errors.New("This Higgsfield voice has no sample yet. Try again when it is ready, or choose a recording")
		}
		data, mimeType, err := downloadHiggsfieldMedia(ctx, voice.PreviewURL, "audio/")
		if err != nil {
			return "", err
		}
		if len(data) == 0 || len(data) > higgsfieldReferenceAudioMaxBytes {
			return "", errors.New("The voice sample is unavailable")
		}
		// Samples of voices cloned from a WAV recording are WAV files served as audio/mpeg.
		filename, mimeType := higgsfieldReferenceFile("glowbom-voice-sample", "audio", mimeType, data)
		// A clone's sample is the whole recording it was made from, often a minute or more.
		if trimmed, trimmedName, trimmedType, ok := higgsfieldTrimReference(ctx, data, filename, "audio"); ok {
			data, filename, mimeType = trimmed, trimmedName, trimmedType
		}
		return higgsfieldUploadMedia(ctx, data, filename, mimeType, "audio")
	}
	return "", errors.New("That voice is no longer in your Higgsfield account")
}

// Tells Seedance what each reference is for, the way the founder intro was prompted.
func higgsfieldReferencePrompt(options studioVideoOptions, prompt string) string {
	scene := strings.TrimSpace(prompt)
	if scene == "" {
		scene = "Live-action, real camera, a steady shot of the same person from the references talking to the camera."
	}
	lines := []string{scene}
	hasVoice := options.VoiceAudioID != "" || options.HiggsfieldVoiceID != ""
	if options.MotionVideoID != "" {
		lines = append(lines, "Video 1 is the reference for the same person, setting, framing and natural movement. Keep them consistent with it.")
		if hasVoice {
			lines = append(lines, "Video 1 is silent and is only a visual and movement reference. Do not infer the speaking voice from the person's appearance or movement.")
		}
	}
	if hasVoice {
		lines = append(lines, "Audio 1 is the only speaking-voice reference for the person. Match its speaker identity, timbre, accent, pitch and cadence. Give this voice reference priority over visual references. Do not use a generic narrator or reuse the sample's words.")
	}
	if options.SpokenText != "" {
		words := strings.Join(strings.Fields(options.SpokenText), " ")
		lines = append(lines, "The person looks into the lens and says, at a natural pace with natural lip-sync: \u201c"+words+"\u201d No music, no other speech, no text on screen.")
	}
	return strings.Join(lines, "\n\n")
}

// Names and labels a reference by what the bytes are, preferring a sniffed media type over a label.
func higgsfieldReferenceFile(base, kind, mimeType string, data []byte) (string, string) {
	if detected := http.DetectContentType(data); strings.HasPrefix(detected, kind+"/") {
		mimeType = detected
	} else if !strings.HasPrefix(mimeType, kind+"/") {
		mimeType = map[string]string{"video": "video/mp4", "audio": "audio/mpeg"}[kind]
	}
	if mimeType == "audio/wave" || mimeType == "audio/x-wav" {
		mimeType = "audio/wav"
	}
	extension := map[string]string{"video/mp4": ".mp4", "video/quicktime": ".mov", "video/webm": ".webm", "audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/mp4": ".m4a", "audio/aac": ".aac", "audio/ogg": ".ogg"}[mimeType]
	if extension == "" {
		extension = map[string]string{"video": ".mp4", "audio": ".mp3"}[kind]
	}
	return base + extension, mimeType
}

// Tests replace this to run without ffmpeg.
var higgsfieldReferenceFFmpeg = func() (string, error) {
	ffmpeg, _, err := studioClipTools()
	return ffmpeg, err
}

func higgsfieldCanTrimReferences() bool {
	_, err := higgsfieldReferenceFFmpeg()
	return err == nil
}

// Keeps the first higgsfieldReferenceSendSeconds of a reference with ffmpeg. ok is false when
// ffmpeg is unavailable or fails, and the caller then sends the original.
func higgsfieldTrimReference(ctx context.Context, data []byte, filename, kind string) ([]byte, string, string, bool) {
	return higgsfieldPrepareReference(ctx, data, filename, kind, false)
}

// A separate voice reference must not compete with sound from the movement clip.
// Callers that request silentVideo must reject a failure instead of uploading the original.
func higgsfieldPrepareReference(ctx context.Context, data []byte, filename, kind string, silentVideo bool) ([]byte, string, string, bool) {
	ffmpeg, err := higgsfieldReferenceFFmpeg()
	if err != nil {
		return nil, "", "", false
	}
	dir, err := os.MkdirTemp("", "glowbom-reference-")
	if err != nil {
		return nil, "", "", false
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input"+filepath.Ext(filename))
	if os.WriteFile(input, data, 0o600) != nil {
		return nil, "", "", false
	}
	seconds := strconv.Itoa(higgsfieldReferenceSendSeconds)
	run := func(output string, args ...string) ([]byte, bool) {
		trimCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		command := append([]string{"-v", "error", "-y", "-i", input, "-t", seconds}, args...)
		if err := studioClipCommand(trimCtx, ffmpeg, append(command, output)...).Run(); err != nil {
			return nil, false
		}
		trimmed, err := os.ReadFile(output)
		return trimmed, err == nil && len(trimmed) > 0
	}
	if kind == "video" {
		output := filepath.Join(dir, "reference.mp4")
		if silentVideo {
			if trimmed, ok := run(output, "-map", "0:v:0", "-an", "-c:v", "copy", "-movflags", "+faststart"); ok {
				return trimmed, "glowbom-video-reference.mp4", "video/mp4", true
			}
			if trimmed, ok := run(output, "-map", "0:v:0", "-an", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-movflags", "+faststart"); ok {
				return trimmed, "glowbom-video-reference.mp4", "video/mp4", true
			}
			return nil, "", "", false
		}
		// Copy the streams when the container allows it; otherwise re-encode.
		if trimmed, ok := run(output, "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+faststart"); ok {
			return trimmed, "glowbom-video-reference.mp4", "video/mp4", true
		}
		if trimmed, ok := run(output, "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart"); ok {
			return trimmed, "glowbom-video-reference.mp4", "video/mp4", true
		}
		log.Printf("[HIGGSFIELD] could not trim the movement video; sending the original")
		return nil, "", "", false
	}
	output := filepath.Join(dir, "reference.mp3")
	if trimmed, ok := run(output, "-vn", "-ac", "1", "-ar", "44100", "-b:a", "128k"); ok {
		return trimmed, "glowbom-voice-reference.mp3", "audio/mpeg", true
	}
	log.Printf("[HIGGSFIELD] could not trim the voice reference; sending the original")
	return nil, "", "", false
}
