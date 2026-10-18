package main

import (
	"context"
	"testing"
)

func TestHiggsfieldVoicesListCustomFirstAndDropUntrusted(t *testing.T) {
	signInFakeHiggsfield(t)
	voices, err := higgsfieldListVoices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) != 2 || voices[0].ID != "clone-1" || voices[0].Type != "element" || voices[1].ID != "preset-1" {
		t.Fatalf("custom voices should come first and invalid ids be dropped: %+v", voices)
	}
	if voices[0].PreviewURL != "" || voices[1].PreviewURL == "" {
		t.Fatalf("only Higgsfield-hosted samples should be kept: %+v", voices)
	}
}

func TestHiggsfieldSpeechEstimateAndGenerate(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	ctx := context.Background()
	oldInterval := higgsfieldImagePollInterval
	higgsfieldImagePollInterval = 0
	t.Cleanup(func() { higgsfieldImagePollInterval = oldInterval })

	if _, err := normalizeHiggsfieldSpeech(higgsfieldSpeechRequest{Prompt: "Hi", VoiceID: "clone-1", VoiceType: "custom"}); err == nil {
		t.Fatalf("unknown voice types must be rejected")
	}
	if _, err := normalizeHiggsfieldSpeech(higgsfieldSpeechRequest{Prompt: "Hi", VoiceID: "clone-1", VoiceType: "element", SpeechRate: 101}); err == nil {
		t.Fatalf("speech rate above 100 must be rejected")
	}
	req, err := normalizeHiggsfieldSpeech(higgsfieldSpeechRequest{Prompt: "  Vibe code from your couch.  ", VoiceID: "clone-1", VoiceType: "element"})
	if err != nil {
		t.Fatal(err)
	}
	if credits, err := higgsfieldSpeechEstimate(ctx, req); err != nil || credits != 1.1 {
		t.Fatalf("estimate = %v, %v", credits, err)
	}
	if fake.audioParams != nil {
		t.Fatalf("an estimate must not submit speech")
	}
	data, mimeType, err := generateHiggsfieldSpeech(ctx, req)
	if err != nil || mimeType != "audio/mpeg" || string(data) != "ID3fake-mp3-bytes" {
		t.Fatalf("speech = %q %q %v", data, mimeType, err)
	}
	p := fake.audioParams
	if p["model"] != "seed_audio" || p["voice_id"] != "clone-1" || p["voice_type"] != "element" || p["prompt"] != "Vibe code from your couch." || p["use_unlim"] != false || p["format"] != "mp3" {
		t.Fatalf("unexpected speech params: %v", p)
	}
}

func TestHiggsfieldCreateVoiceUploadsAudioAndClones(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	recording := append([]byte("RIFF"), make([]byte, 2048)...)
	if _, err := createHiggsfieldVoice(context.Background(), "Jacob", recording, "audio/webm"); err == nil {
		t.Fatalf("only WAV or MP3 recordings should be sent")
	}
	voice, err := createHiggsfieldVoice(context.Background(), "Jacob", recording, "audio/wav")
	if err != nil || voice.ID != "clone-2" || voice.Status != "processing" || voice.Type != "element" {
		t.Fatalf("voice = %+v, %v", voice, err)
	}
	if string(fake.uploaded) != string(recording) {
		t.Fatalf("recording was not uploaded")
	}
	if len(fake.voiceClones) != 1 || fake.voiceClones[0]["audio_media_id"] != "media-1" || fake.voiceClones[0]["name"] != "Jacob" {
		t.Fatalf("clone call = %v", fake.voiceClones)
	}
	for _, call := range fake.toolCalls {
		if call.name == "media_confirm" && call.arguments["type"] != "audio" {
			t.Fatalf("recording should be confirmed as audio: %v", call.arguments)
		}
	}
}

func TestHiggsfieldCreateVoiceFindsACloneWhoseReplyWasUnclear(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	fake.cloneReplyLost = true
	recording := append([]byte("RIFF"), make([]byte, 2048)...)
	voice, err := createHiggsfieldVoice(context.Background(), "Maria", recording, "audio/wav")
	if err != nil || voice.ID != "clone-3" || voice.Status != "processing" {
		t.Fatalf("a clone that exists should be reported as created: %+v, %v", voice, err)
	}
	if len(fake.voiceClones) != 1 {
		t.Fatalf("the clone must not be retried: %d calls", len(fake.voiceClones))
	}
}
