package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHiggsfieldProjectImageSettingsReuseStudioCatalogue(t *testing.T) {
	useFakeHiggsfield(t)
	for _, source := range studioImageModels() {
		if source.ID != higgsfieldSourceID {
			continue
		}
		for _, model := range source.Models {
			for _, resolution := range model.Resolutions {
				for _, aspect := range model.AspectRatios {
					input := studioImageOptions{SourceID: source.ID, ModelID: model.ID, Resolution: resolution, AspectRatio: aspect, Quality: model.DefaultQuality}
					want, err := normalizeStudioImageOptions(input)
					got, actualErr := normalizeHiggsfieldProjectImageOptions(input, false)
					if err != nil || actualErr != nil || got != want {
						t.Fatalf("project options differ from Studio: %+v %v %v", input, err, actualErr)
					}
				}
			}
		}
	}
	legacy, err := normalizeHiggsfieldProjectImageOptions(studioImageOptions{SourceID: higgsfieldSourceID}, false)
	if err != nil || legacy != higgsfieldProjectImageOptions("16:9") {
		t.Fatal("legacy prototype defaults changed", legacy, err)
	}
	icon, err := normalizeHiggsfieldProjectImageOptions(studioImageOptions{SourceID: higgsfieldSourceID}, true)
	if err != nil || icon != higgsfieldProjectImageOptions("1:1") {
		t.Fatal("legacy icon defaults changed", icon, err)
	}
}

func TestHiggsfieldProjectImageSettingsAcceptNewCatalogueModels(t *testing.T) {
	sources := []studioImageSourceCapability{{ID: higgsfieldSourceID, Models: []studioImageModelCapability{{ID: "future-fixture-model", AspectRatios: []string{"3:2"}, DefaultAspectRatio: "3:2", Resolutions: []string{"new-resolution"}, DefaultResolution: "new-resolution", Qualities: []string{"new-quality"}, DefaultQuality: "new-quality"}}}}
	got, err := normalizeHiggsfieldProjectImageOptionsInCatalogue(studioImageOptions{SourceID: higgsfieldSourceID, ModelID: "future-fixture-model"}, false, sources)
	if err != nil || got.AspectRatio != "3:2" || got.Resolution != "new-resolution" || got.Quality != "new-quality" {
		t.Fatal("new catalogue model required a project allowlist change", got, err)
	}
	if _, err := normalizeHiggsfieldProjectImageOptionsInCatalogue(got, true, sources); err == nil {
		t.Fatal("nonsquare model accepted for an icon")
	}
}

func TestHiggsfieldProjectImageSettingsRejectMismatches(t *testing.T) {
	useFakeHiggsfield(t)
	for _, input := range []studioImageOptions{
		{SourceID: "openai-api", ModelID: "gpt_image_2_5"},
		{SourceID: higgsfieldSourceID, ModelID: "unknown-model"},
		{SourceID: higgsfieldSourceID, ModelID: "nano_banana_2_1", Quality: "high"},
		{SourceID: higgsfieldSourceID, Resolution: "8k"},
		{SourceID: higgsfieldSourceID, AspectRatio: "8:1"},
	} {
		if _, err := normalizeHiggsfieldProjectImageOptions(input, false); err == nil {
			t.Fatal("invalid settings accepted", input)
		}
	}
	if _, err := normalizeHiggsfieldProjectImageOptions(studioImageOptions{SourceID: higgsfieldSourceID, AspectRatio: "16:9"}, true); err == nil {
		t.Fatal("non-square icon accepted")
	}
	if err := validateChatImageOptions(chatImageOptions{SourceID: higgsfieldSourceID, ModelID: "missing", HiggsfieldMaxCredits: projectImageCredits(1)}); err == nil {
		t.Fatal("invalid prototype settings reached model request")
	}
}

func assertHiggsfieldImageSettingsQuotedAndSubmitted(t *testing.T, fake *fakeHiggsfield, expected studioImageOptions) {
	t.Helper()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	quotes, submits := 0, 0
	for _, call := range fake.toolCalls {
		if call.name != "generate_image" {
			continue
		}
		params, _ := call.arguments["params"].(map[string]any)
		if params["model"] != expected.ModelID || params["aspect_ratio"] != expected.AspectRatio || params["resolution"] != expected.Resolution || params["use_unlim"] != false {
			t.Fatalf("quoted/submitted options differ: %v", params)
		}
		quality, _ := params["quality"].(string)
		if quality != expected.Quality {
			t.Fatal("quality changed", params)
		}
		if params["get_cost"] == true {
			quotes++
		} else {
			submits++
		}
	}
	if quotes != 1 || submits != 1 {
		t.Fatal("expected one quote and one paid submission", quotes, submits)
	}
}

func TestHiggsfieldProjectIconUsesSelectedSettings(t *testing.T) {
	for _, options := range []studioImageOptions{
		{SourceID: higgsfieldSourceID, ModelID: "gpt_image_2_5", AspectRatio: "1:1", Resolution: "4k", Quality: "high"},
		{SourceID: higgsfieldSourceID, ModelID: "nano_banana_2_1", AspectRatio: "1:1", Resolution: "2k"},
	} {
		t.Run(options.ModelID, func(t *testing.T) {
			fake := readyProjectHiggsfield(t)
			t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-token")
			body, _ := json.Marshal(projectIconRequest{Path: t.TempDir(), Prompt: "Fixture icon", SourceID: options.SourceID, ModelID: options.ModelID, AspectRatio: options.AspectRatio, Resolution: options.Resolution, Quality: options.Quality, HiggsfieldMaxCredits: projectImageCredits(.25)})
			request := httptest.NewRequest(http.MethodPost, "/opencode/project/icon/generate", bytes.NewReader(body))
			request.Header.Set("Authorization", "Bearer fixture-token")
			response := httptest.NewRecorder()
			openCodeGenerateIconHandler(response, request)
			var result struct {
				Success bool   `json:"success"`
				ID      string `json:"studioAssetId"`
			}
			if json.Unmarshal(response.Body.Bytes(), &result) != nil || !result.Success {
				t.Fatal("icon generation failed", response.Body.String())
			}
			asset, err := findStudioAsset(result.ID)
			if err != nil || asset.Model != options.ModelID || asset.Resolution != options.Resolution || asset.Quality != options.Quality {
				t.Fatal("saved icon lost settings", err)
			}
			assertHiggsfieldImageSettingsQuotedAndSubmitted(t, fake, options)
		})
	}
}

func TestHiggsfieldPrototypeSettingsSeparateCacheAndPreserveReuse(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	original := `<!doctype html><html><img src="glowbomimages:Fixture harbor"></html>`
	root, record := chatImagesFixture(t, original)
	choices := []chatImageOptions{
		{SourceID: higgsfieldSourceID, ModelID: "nano_banana_2_1", AspectRatio: "9:16", Resolution: "4k", HiggsfieldMaxCredits: projectImageCredits(.25)},
		{SourceID: higgsfieldSourceID, ModelID: "gpt_image_2_5", AspectRatio: "4:3", Resolution: "2k", Quality: "high", HiggsfieldMaxCredits: projectImageCredits(.25)},
	}
	results := []string{}
	current := original
	for _, options := range choices {
		if err := saveChatPrototype(root, original, current, nil, "fixture/model", nil, &record); err != nil {
			t.Fatal(err)
		}
		final, warnings, err := materializeChatImages(context.Background(), root, record, original, options, func(map[string]any) {})
		if err != nil || len(warnings) != 0 || strings.Contains(final, "glowbomimages:") {
			t.Fatal("selected prototype settings failed", warnings, err)
		}
		results = append(results, final)
		current = final
	}
	if results[0] == results[1] || higgsfieldPaidImageCalls(fake) != 2 {
		t.Fatal("different model/settings reused the wrong image")
	}
	if err := saveChatPrototype(root, original, current, nil, "fixture/model", nil, &record); err != nil {
		t.Fatal(err)
	}
	choices[0].HiggsfieldMaxCredits = projectImageCredits(0)
	final, warnings, err := materializeChatImages(context.Background(), root, record, original, choices[0], func(map[string]any) {})
	if err != nil || len(warnings) != 0 || final != results[0] || higgsfieldPaidImageCalls(fake) != 2 {
		t.Fatal("same settings regenerated or price changed cache identity", warnings, err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	calls := []map[string]any{}
	for _, call := range fake.toolCalls {
		if call.name == "generate_image" {
			params, _ := call.arguments["params"].(map[string]any)
			calls = append(calls, params)
		}
	}
	if len(calls) != 4 {
		t.Fatal("unexpected estimate/submission count", len(calls))
	}
	for index, choice := range choices {
		for _, params := range calls[index*2 : index*2+2] {
			if params["model"] != choice.ModelID || params["aspect_ratio"] != choice.AspectRatio || params["resolution"] != choice.Resolution {
				t.Fatal("prototype quote differed from submitted settings", params)
			}
		}
	}
}

func TestHiggsfieldLargePrototypeImageCanBeReadFromCache(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "prototype", "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 5000, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prototype", "assets", "wide.png"), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := readChatImage(root, "wide.png")
	if err != nil || len(data) == 0 {
		t.Fatal("Studio-sized image could not be reused", err)
	}
}

func TestHiggsfieldProjectIconRejectsInvalidSettingsBeforeSpending(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-token")
	for _, settings := range []studioImageOptions{
		{ModelID: "unknown"},
		{ModelID: "nano_banana_2_1", Quality: "high"},
		{ModelID: "gpt_image_2_5", AspectRatio: "16:9"},
		{ModelID: "gpt_image_2_5", Resolution: "8k"},
	} {
		body, _ := json.Marshal(projectIconRequest{Path: t.TempDir(), Prompt: "Fixture", SourceID: higgsfieldSourceID, ModelID: settings.ModelID, AspectRatio: settings.AspectRatio, Resolution: settings.Resolution, Quality: settings.Quality, HiggsfieldMaxCredits: projectImageCredits(1)})
		request := httptest.NewRequest(http.MethodPost, "/opencode/project/icon/generate", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer fixture-token")
		response := httptest.NewRecorder()
		openCodeGenerateIconHandler(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatal("invalid icon settings accepted", settings, response.Code)
		}
	}
	if higgsfieldPaidImageCalls(fake) != 0 {
		t.Fatal("invalid icon settings spent credits")
	}
}
