package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectImageCredits(value float64) *float64 { return &value }

func readyProjectHiggsfield(t *testing.T) *fakeHiggsfield {
	t.Helper()
	isolateProjectIconCredentials(t)
	fake := signInFakeHiggsfield(t)
	old := higgsfieldImagePollInterval
	higgsfieldImagePollInterval = 0
	t.Cleanup(func() { higgsfieldImagePollInterval = old })
	return fake
}

func higgsfieldPaidImageCalls(fake *fakeHiggsfield) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	count := 0
	for _, call := range fake.toolCalls {
		params, _ := call.arguments["params"].(map[string]any)
		if call.name == "generate_image" && params["get_cost"] != true {
			count++
		}
	}
	return count
}

func TestHiggsfieldProjectImageAllowanceAndPriceValidation(t *testing.T) {
	for _, value := range []*float64{nil, projectImageCredits(-1), projectImageCredits(math.NaN()), projectImageCredits(math.Inf(1))} {
		if validateHiggsfieldImageAllowance(value) == nil {
			t.Fatal("invalid allowance accepted")
		}
	}
	if validateHiggsfieldImageAllowance(projectImageCredits(0)) != nil {
		t.Fatal("explicit zero allowance rejected")
	}
	for _, raw := range []string{`{}`, `{"cost":{}}`, `{"cost":{"credits":null}}`, `{"cost":{"credits":-1}}`, `{"cost":{"credits":"0"}}`, `not-json`} {
		if _, err := parseHiggsfieldImageCredits(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid price accepted: %s", raw)
		}
	}
	if credits, err := parseHiggsfieldImageCredits(json.RawMessage(`{"cost":{"credits":0}}`)); err != nil || credits != 0 {
		t.Fatal("numeric free price rejected", err)
	}
}

func TestHiggsfieldProjectImageBudgetAndReference(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	options := higgsfieldProjectImageOptions("16:9")
	for _, credits := range []*float64{nil, projectImageCredits(0), projectImageCredits(0.24)} {
		ctx := context.Background()
		if credits != nil {
			ctx, _ = withHiggsfieldImageAllowance(ctx, credits)
		}
		if _, _, err := generateHiggsfieldProjectImage(ctx, options, "Fixture harbor", ""); err == nil {
			t.Fatal("unapproved image submitted")
		}
	}
	if higgsfieldPaidImageCalls(fake) != 0 {
		t.Fatal("insufficient allowance reached paid generation")
	}
	ctx, _ := withHiggsfieldImageAllowance(context.Background(), projectImageCredits(.25))
	reference := "data:image/png;base64," + base64.StdEncoding.EncodeToString(fakeHiggsfieldPNG)
	data, label, err := generateHiggsfieldProjectImage(ctx, options, "Fixture harbor", reference)
	if err != nil || !strings.HasPrefix(data, "data:image/png;base64,") || label != "Higgsfield" {
		t.Fatal("approved image failed", err)
	}
	if _, _, err := generateHiggsfieldProjectImage(ctx, options, "Another fixture", ""); err == nil {
		t.Fatal("spent allowance reused")
	}
	fake.mu.Lock()
	params := fake.imageParams
	medias, _ := params["medias"].([]any)
	valid := params["model"] == higgsfieldProjectImageModel && params["aspect_ratio"] == "16:9" && params["resolution"] == "1k" && params["quality"] == "low" && len(medias) == 1
	fake.mu.Unlock()
	if !valid || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("wrong image defaults, missing reference, or extra generation", params)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = withHiggsfieldImageAllowance(ctx, projectImageCredits(1))
	cancel()
	if _, _, err := generateHiggsfieldProjectImage(ctx, options, "Canceled fixture", ""); err == nil || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("canceled request submitted")
	}
}

func TestHiggsfieldProjectSourcesAndLegacyCompanion(t *testing.T) {
	readyProjectHiggsfield(t)
	for _, legacy := range []bool{false, true} {
		target := "/opencode/project/icon/sources"
		if legacy {
			target += "?legacyCompanion=1"
		}
		recorder := httptest.NewRecorder()
		openCodeIconSourcesHandler(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		var result struct {
			Sources []projectIconSource `json:"sources"`
		}
		if json.Unmarshal(recorder.Body.Bytes(), &result) != nil || recorder.Code != 200 {
			t.Fatal("source catalog failed")
		}
		found := false
		for _, source := range result.Sources {
			if source.ID == higgsfieldSourceID {
				found = true
				if !source.Available || source.AuthType != "subscription" || source.Model != higgsfieldProjectImageModel {
					t.Fatal("wrong subscription source", source)
				}
			}
		}
		if found == legacy {
			t.Fatal("legacy companion credit review boundary not preserved")
		}
		if strings.Contains(recorder.Body.String(), "access-1") || strings.Contains(recorder.Body.String(), "refresh-1") {
			t.Fatal("credentials in source catalog")
		}
	}
	if id, key, err := resolveProjectIconSource(projectIconRequest{SourceID: higgsfieldSourceID, APIKey: "must-not-use"}); err != nil || id != higgsfieldSourceID || key != "" {
		t.Fatal("subscription resolved an API key", err)
	}
}

func TestHiggsfieldProjectIconRequiresReviewAndKeepsExisting(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-token")
	project := t.TempDir()
	previous := projectIconTestImage(t, "png")
	if err := os.WriteFile(filepath.Join(project, "icon.png"), previous, 0600); err != nil {
		t.Fatal(err)
	}
	post := func(allowance *float64, authenticated bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(projectIconRequest{Path: project, Prompt: "A fixture app", SourceID: higgsfieldSourceID, HiggsfieldMaxCredits: allowance, ReferenceImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(previous)})
		request := httptest.NewRequest(http.MethodPost, "/opencode/project/icon/generate", bytes.NewReader(body))
		if authenticated {
			request.Header.Set("Authorization", "Bearer fixture-token")
		}
		recorder := httptest.NewRecorder()
		openCodeGenerateIconHandler(recorder, request)
		return recorder
	}
	if r := post(projectImageCredits(.25), false); r.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated generation accepted", r.Code)
	}
	if r := post(nil, true); r.Code != http.StatusBadRequest {
		t.Fatal("unreviewed icon accepted", r.Code)
	}
	if r := post(projectImageCredits(0), true); !strings.Contains(r.Body.String(), `"success":false`) || !strings.Contains(r.Body.String(), "credit limit") {
		t.Fatal("zero budget submitted", r.Body.String())
	}
	if current, _ := os.ReadFile(filepath.Join(project, "icon.png")); !bytes.Equal(current, previous) || higgsfieldPaidImageCalls(fake) != 0 {
		t.Fatal("failed review changed icon or submitted")
	}
	result := post(projectImageCredits(.25), true)
	var saved struct {
		Success bool   `json:"success"`
		AssetID string `json:"studioAssetId"`
	}
	if json.Unmarshal(result.Body.Bytes(), &saved) != nil || !saved.Success || saved.AssetID == "" || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("approved icon failed", result.Body.String())
	}
	asset, err := findStudioAsset(saved.AssetID)
	if err != nil || asset.SourceID != higgsfieldSourceID || asset.Model != higgsfieldProjectImageModel || asset.Resolution != "1k" {
		t.Fatal("icon settings not saved", err)
	}
}

func TestHiggsfieldPrototypeImagesBudgetDeduplicationAndReuse(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	original := `<!doctype html><html><img src="glowbomimages:The person from the reference photo in a harbor"><img src="glowbomimages:The person from the reference photo in a harbor"><img src="glowbomimages:A forest"></html>`
	root, record := chatImagesFixture(t, original)
	events := []map[string]any{}
	options := chatImageOptions{SourceID: higgsfieldSourceID, HiggsfieldMaxCredits: projectImageCredits(.25), Personalization: true, ReferencePath: "fixture.png", referencePNG: base64.StdEncoding.EncodeToString(fakeHiggsfieldPNG)}
	final, warnings, err := materializeChatImages(context.Background(), root, record, original, options, func(event map[string]any) { events = append(events, event) })
	if err != nil || len(warnings) != 1 || higgsfieldPaidImageCalls(fake) != 1 || strings.Count(final, "assets/glowbom-image-") != 2 || !strings.Contains(warnings[0], "credit limit") {
		t.Fatal("prototype budget or deduplication failed", warnings, err)
	}
	updated := false
	for _, event := range events {
		updated = updated || event["text"] == final
	}
	if !updated {
		t.Fatal("updated image progress missing")
	}
	if err := saveChatPrototype(root, original, final, nil, "test/model", nil, &record); err != nil {
		t.Fatal(err)
	}
	options.HiggsfieldMaxCredits = projectImageCredits(0)
	_, _, err = materializeChatImages(context.Background(), root, record, original, options, func(map[string]any) {})
	if err != nil || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("cached image regenerated", err)
	}
	assets, _ := os.ReadDir(filepath.Join(os.Getenv("GLOWBOM_STUDIO_DIR"), "Assets"))
	if len(assets) != 1 {
		t.Fatal("saved Studio image duplicated", len(assets))
	}
}

func TestHiggsfieldPrototypeRequiresLocalAuthentication(t *testing.T) {
	t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-token")
	body, _ := json.Marshal(chatRequest{Model: "fixture/model", Mode: "prototype", Messages: []chatMessage{{Role: "user", Text: "Fixture app"}}, Images: &chatImageOptions{SourceID: higgsfieldSourceID, HiggsfieldMaxCredits: projectImageCredits(1)}})
	recorder := httptest.NewRecorder()
	(&chatService{}).streamHandler(recorder, httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(body)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated prototype reached chat provider", recorder.Code, recorder.Body.String())
	}
}

func TestHiggsfieldBuildMediaReviewAndGeneration(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	t.Setenv("GLOWBOM_SERVER_TOKEN", "fixture-token")
	project, plan := imageSettingsFixture(t, `<img src="glowbomimage:Fixture harbor">`)
	item := plan.Items[0]
	setImageApprovalOptions(&item, higgsfieldProjectImageOptions("1:1"))
	if validateMediaApprovalItem(item) == nil {
		t.Fatal("unreviewed Build image accepted")
	}
	item.HiggsfieldMaxCredits = projectImageCredits(.25)
	item.ReferenceImages = []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(fakeHiggsfieldPNG)}
	id, responses := registerOpenCodeMediaApproval(project, plan)
	t.Cleanup(func() { removeOpenCodeMediaApproval(id) })
	body, _ := json.Marshal(openCodeMediaApprovalResponse{ApprovalID: id, ProjectPath: project, Response: "generate", Items: []OpenCodeMediaApprovalItem{item}})
	for _, authenticated := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodPost, "/opencode/media/approval/respond", bytes.NewReader(body))
		if authenticated {
			request.Header.Set("Authorization", "Bearer fixture-token")
		}
		recorder := httptest.NewRecorder()
		openCodeMediaApprovalRespondHandler(recorder, request)
		expected := http.StatusUnauthorized
		if authenticated {
			expected = http.StatusOK
		}
		if recorder.Code != expected {
			t.Fatal("wrong approval authentication", recorder.Code, recorder.Body.String())
		}
	}
	decision, err := waitForOpenCodeMediaApproval(context.Background(), id, responses)
	if err != nil || !decision.imageSubscriptionAuthorized || *decision.Items[0].HiggsfieldMaxCredits != .25 {
		t.Fatal("review allowance lost", err)
	}
	result, err := runOpenCodeMediaPostPass(context.Background(), OpenCodeMediaPostPassRequest{ProjectPath: project, Items: decision.Items, imageSubscriptionAuthorized: decision.imageSubscriptionAuthorized})
	if err != nil || result == nil || len(result.GeneratedAssets) != 1 || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("approved Build image failed", result, err)
	}
	asset, err := findStudioAsset(result.GeneratedAssets[0].StudioAssetID)
	if err != nil || asset.SourceID != higgsfieldSourceID || asset.Model != higgsfieldProjectImageModel || asset.Quality != "low" {
		t.Fatal("Build image metadata lost", err)
	}
	_, _, err = generateImageForPostPass(OpenCodeMediaPostPassRequest{ImageSource: higgsfieldSourceID, imageSubscriptionAuthorized: true}, "Unreviewed fixture", "")
	if err == nil || higgsfieldPaidImageCalls(fake) != 1 {
		t.Fatal("unreviewed automatic postpass submitted")
	}
}

func TestHiggsfieldProjectImageDecimalBudget(t *testing.T) {
	ctx, _ := withHiggsfieldImageAllowance(context.Background(), projectImageCredits(1.2))
	budget := ctx.Value(higgsfieldImageBudgetKey{}).(*higgsfieldImageBudget)
	for index := 0; index < 4; index++ {
		if !budget.reserve(.3) {
			t.Fatalf("approved image %d rejected by decimal rounding", index+1)
		}
	}
	if budget.reserve(.01) {
		t.Fatal("exhausted budget accepted another image")
	}
	if !budget.reserve(0) {
		t.Fatal("a reported free image should not spend credits")
	}
}

func TestHiggsfieldLegacyCompanionCannotBypassCreditReview(t *testing.T) {
	readyProjectHiggsfield(t)
	isolateCompanionStudio(t)
	session := testCompanion(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("legacy companion forwarded an unreviewed Higgsfield request")
	}))
	response := httptest.NewRecorder()
	session.ServeHTTP(response, companionRequest(session, http.MethodPost, "/images", `{"prompt":"Fixture harbor","sourceId":"higgsfield","aspectRatio":"1:1"}`))
	if response.Code != http.StatusBadRequest {
		t.Fatal("legacy companion image accepted", response.Code)
	}
	if session.projectIconSourceAvailable(context.Background(), higgsfieldSourceID) {
		t.Fatal("legacy companion icon accepted")
	}
	if _, available := session.prototypeImageProvider(context.Background(), higgsfieldSourceID); available {
		t.Fatal("legacy companion prototype accepted")
	}
}
