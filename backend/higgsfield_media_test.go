package main

import (
	"context"
	"encoding/base64"
	"testing"
)

func signInFakeHiggsfield(t *testing.T) *fakeHiggsfield {
	t.Helper()
	fake, _ := useFakeHiggsfield(t)
	_, state, err := glowbomHiggsfield.startLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok, message := glowbomHiggsfield.finish(state, "good-code", ""); !ok {
		t.Fatal(message)
	}
	return fake
}

func TestHiggsfieldVideoSourceIsCuratedAndFollowsSignIn(t *testing.T) {
	useFakeHiggsfield(t)
	source := higgsfieldVideoSource()
	if source.Connected {
		t.Fatalf("source should not be connected before sign-in")
	}
	ids := []string{}
	for _, model := range source.Models {
		ids = append(ids, model.ID)
		if len(model.PricesPerSecondUSD) != 0 {
			t.Fatalf("%s should not hardcode prices; Higgsfield prices each request", model.ID)
		}
	}
	if len(ids) != 3 || ids[0] != "kling3_0_turbo" || ids[1] != "kling3_0" || ids[2] != "seedance_2_5" {
		t.Fatalf("unexpected curated models: %v", ids)
	}
	if _, err := normalizeStudioVideoOptions(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 30, Resolution: "1080p", AspectRatio: "9:16"}); err != nil {
		t.Fatalf("Seedance 2.5 should accept 30 s at 1080p: %v", err)
	}
	if _, err := normalizeStudioVideoOptions(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0", DurationSeconds: 5, Resolution: "720p", AspectRatio: "21:9"}); err == nil {
		t.Fatalf("Kling 3.0 must reject 21:9")
	}
	if estimateStudioVideoCost(studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "seedance_2_5", DurationSeconds: 10, Resolution: "720p"}, true) != 0 {
		t.Fatalf("Higgsfield costs are credits from the preflight, not USD estimates")
	}
}

func TestHiggsfieldVideoParamsPerModel(t *testing.T) {
	kling := higgsfieldVideoParams(studioVideoOptions{ModelID: "kling3_0", DurationSeconds: 5, Resolution: "1080p", AspectRatio: "16:9"}, "p", "")
	if kling["mode"] != "pro" || kling["resolution"] != nil || kling["use_unlim"] != false {
		t.Fatalf("Kling should map 1080p to pro mode and never use unlimited generations: %v", kling)
	}
	seedance := higgsfieldVideoParams(studioVideoOptions{ModelID: "seedance_2_5", DurationSeconds: 8, Resolution: "720p", AspectRatio: "9:16"}, "p", "media-1")
	if seedance["mode"] != "omni_reference" || seedance["resolution"] != "720p" {
		t.Fatalf("Seedance 2.5 with an image should use reference mode: %v", seedance)
	}
	medias, _ := seedance["medias"].([]map[string]string)
	if len(medias) != 1 || medias[0]["role"] != "start_image" || medias[0]["value"] != "media-1" {
		t.Fatalf("starting image should be passed as start_image: %v", seedance["medias"])
	}
	if turbo := higgsfieldVideoParams(studioVideoOptions{ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "1080p", AspectRatio: "9:16"}, "p", ""); turbo["resolution"] != "1080p" || turbo["mode"] != nil || turbo["generate_audio"] != nil {
		t.Fatalf("Kling 3.0 Turbo takes a plain resolution only: %v", turbo)
	}
	if text := higgsfieldVideoParams(studioVideoOptions{ModelID: "seedance_2_5", DurationSeconds: 5, Resolution: "480p", AspectRatio: "16:9"}, "p", ""); text["mode"] != "t2v" || text["medias"] != nil {
		t.Fatalf("Seedance 2.5 without an image should be text-to-video: %v", text)
	}
}

func TestHiggsfieldVideoEstimateIsAPreflight(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	credits, err := higgsfieldVideoEstimate(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"})
	if err != nil || credits != 5 {
		t.Fatalf("estimate = %v, %v", credits, err)
	}
	if fake.submits != 0 {
		t.Fatalf("an estimate must not submit a generation")
	}
}

func TestHiggsfieldVideoGenerateDeclinesPresetUploadsPollsAndDownloads(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	fake.recommendPreset = true
	ctx := context.Background()
	options := studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"}
	image := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake"))

	start, err := generateStudioSelectedVideo(ctx, options, "", "A man on a couch smiling at his phone", []VeoImageInput{{Data: image, MimeType: "image/png"}})
	if err != nil || start.OperationID != "job-1" {
		t.Fatalf("start = %+v, %v", start, err)
	}
	if fake.submits != 2 {
		t.Fatalf("expected a declined preset and one resubmission, got %d submits", fake.submits)
	}
	if string(fake.uploaded) != "\x89PNG\r\n\x1a\nfake" {
		t.Fatalf("starting image was not uploaded: %q", fake.uploaded)
	}
	var submitted map[string]any
	for _, call := range fake.toolCalls {
		if call.name == "generate_video" {
			submitted, _ = call.arguments["params"].(map[string]any)
		}
	}
	if submitted["declined_preset_id"] != "preset-1" || submitted["use_unlim"] != false {
		t.Fatalf("resubmission should decline the preset and pass use_unlim=false: %v", submitted)
	}

	poll, err := pollStudioSelectedVideo(ctx, options, "", "job-1")
	if err != nil || poll.Done {
		t.Fatalf("first poll should be pending: %+v %v", poll, err)
	}
	poll, err = pollStudioSelectedVideo(ctx, options, "", "job-1")
	if err != nil || !poll.Done || poll.Status != "completed" || poll.VideoURL == "" {
		t.Fatalf("second poll should complete: %+v %v", poll, err)
	}
	dataURI, err := downloadStudioSelectedVideo(ctx, options, "", poll.VideoURL)
	if err != nil || dataURI != "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("fake-mp4-bytes")) {
		t.Fatalf("download = %q, %v", dataURI, err)
	}
}

func TestHiggsfieldRejectsUntrustedHosts(t *testing.T) {
	useFakeHiggsfield(t)
	for _, raw := range []string{"http://d8j0ntlcm91z4.cloudfront.net/x.mp4", "https://evil.example.com/x.mp4", "https://higgsfield.ai.evil.com/x"} {
		if higgsfieldTrustedHost(raw) {
			t.Fatalf("%s must not be trusted", raw)
		}
	}
	for _, raw := range []string{"https://d8j0ntlcm91z4.cloudfront.net/x.mp4", "https://upload.higgsfield.ai/a.png"} {
		if !higgsfieldTrustedHost(raw) {
			t.Fatalf("%s should be trusted", raw)
		}
	}
	if _, err := resolveStudioVideoKey(context.Background(), higgsfieldSourceID, "", false); err == nil {
		t.Fatalf("generation must require a connected Higgsfield account")
	}
}

func TestHiggsfieldImageSourceEstimateAndGenerate(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	ctx := context.Background()
	found := false
	for _, source := range studioImageModels() {
		if source.ID == higgsfieldSourceID {
			found = true
			if len(source.Models) != 2 || source.Models[0].ID != "gpt_image_2_5" || source.Models[1].ID != "nano_banana_2_1" {
				t.Fatalf("unexpected curated image models: %+v", source.Models)
			}
		}
	}
	if !found {
		t.Fatalf("Higgsfield should be a Studio image source")
	}
	options, err := normalizeStudioImageOptions(studioImageOptions{SourceID: higgsfieldSourceID, ModelID: "gpt_image_2_5", AspectRatio: "9:16", Resolution: "2K"})
	if err != nil || options.Resolution != "2k" || options.Quality != "low" {
		t.Fatalf("normalize = %+v, %v", options, err)
	}
	if credits, err := higgsfieldImageEstimate(ctx, options); err != nil || credits != 0.25 {
		t.Fatalf("estimate = %v, %v", credits, err)
	}

	oldInterval := higgsfieldImagePollInterval
	higgsfieldImagePollInterval = 0
	t.Cleanup(func() { higgsfieldImagePollInterval = oldInterval })
	reference := "data:image/png;base64," + base64.StdEncoding.EncodeToString(fakeHiggsfieldPNG)
	dataURI, label, err := generateStudioSelectedImageWithOptions(ctx, options, "", "Jacob on a couch", reference)
	if err != nil || label != "Higgsfield" || dataURI != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(fakeHiggsfieldPNG) {
		t.Fatalf("generate = %q %q %v", dataURI[:min(40, len(dataURI))], label, err)
	}
	medias, _ := fake.imageParams["medias"].([]any)
	if len(medias) != 1 || fake.imageParams["use_unlim"] != false || fake.imageParams["resolution"] != "2k" || fake.imageParams["quality"] != "low" {
		t.Fatalf("image request should carry the reference and explicit settings: %v", fake.imageParams)
	}
	if media, _ := medias[0].(map[string]any); media["role"] != "image_references" {
		t.Fatalf("reference should use the image_references role: %v", medias[0])
	}
}
