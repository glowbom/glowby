package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func carouselFixture(t *testing.T) (*chatService, *chatCarousel) {
	t.Helper()
	uploads := t.TempDir()
	cards := []chatCarouselCard{{ID: "scene-one", Role: "sketch", Title: "First scene", Instructions: "At the beach", PersonIDs: []string{"Alex Smith"}}, {ID: "scene-two", Role: "sketch", Title: "Second scene", Instructions: "At a park", PersonIDs: []string{"Blair"}}, {ID: "scene-none", Role: "sketch", Title: "Scenery", PersonIDs: []string{}}}
	for index, person := range []string{"Alex Smith", "Alex Smith", "Blair", "Unselected"} {
		filename := filepath.Join(uploads, fmt.Sprintf("person-%d.png", index))
		img := image.NewRGBA(image.Rect(0, 0, 12, 12))
		colors := []color.RGBA{{R: 255, A: 255}, {R: 180, A: 255}, {B: 255, A: 255}, {G: 255, A: 255}}
		for y := 0; y < 12; y++ {
			for x := 0; x < 12; x++ {
				img.SetRGBA(x, y, colors[index])
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, buffer.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		cards = append(cards, chatCarouselCard{ID: fmt.Sprintf("person-%d", index), Role: "person", Title: person, PersonID: person, AttachmentPath: filename})
	}
	return &chatService{uploads: uploads}, &chatCarousel{Version: 1, Cards: cards}
}

func TestChatCarouselValidatesRolesReferencesAndLegacyBoundary(t *testing.T) {
	service, manifest := carouselFixture(t)
	if err := validateChatCarousel(manifest); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*chatCarousel){
		func(c *chatCarousel) { c.Cards[1].ID = c.Cards[0].ID },
		func(c *chatCarousel) { c.Cards[0].PersonIDs = []string{"Unknown"} },
		func(c *chatCarousel) { c.Cards[0].PersonIDs = []string{"alex smith"} },
		func(c *chatCarousel) { c.Cards[3].AttachmentPath = "" },
		func(c *chatCarousel) { c.Cards[0].Role = "reference" },
		func(c *chatCarousel) { c.Cards[0].ID = "../outside" },
		func(c *chatCarousel) { c.Cards = append(c.Cards, c.Cards...) },
	} {
		data, _ := json.Marshal(manifest)
		var copy chatCarousel
		_ = json.Unmarshal(data, &copy)
		change(&copy)
		if validateChatCarousel(&copy) == nil {
			t.Fatal("invalid carousel accepted")
		}
	}
	request := chatRequest{Mode: "prototype", Model: "fixture/model", Messages: []chatMessage{{Role: "user", Text: "Build"}}, Carousel: manifest, AttachmentPaths: []string{"outside.png"}}
	if validateChatRequest(request) == nil {
		t.Fatal("legacy attachments accepted with carousel")
	}
	request.AttachmentPaths = nil
	request.Images = &chatImageOptions{SourceID: "openai-api", Personalization: true, ReferencePath: "global.png"}
	if validateChatRequest(request) == nil {
		t.Fatal("global personalization accepted")
	}
	if _, err := service.prepareChatCarousel(manifest, "picsum"); err == nil {
		t.Fatal("sample photos accepted selected persona")
	}
}

func TestChatCarouselScopesAndFreezesPersonPhotos(t *testing.T) {
	service, manifest := carouselFixture(t)
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.modelParts) != 0 {
		t.Fatal("person photos leaked to planning model")
	}
	prompt, reference, err := prepared.imageScene("[card:scene-one] The selected person at the beach")
	if err != nil || reference == "" || !strings.Contains(prompt, "Panel 2") || strings.Contains(prompt, "Blair") || strings.Contains(prompt, "Unselected") {
		t.Fatal("wrong person group selected", err)
	}
	data, _ := base64.StdEncoding.DecodeString(reference)
	sheet, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []image.Point{{X: 300, Y: 300}, {X: 1000, Y: 300}} {
		r, g, b, _ := sheet.At(point.X, point.Y).RGBA()
		if r == 0 || g != 0 || b != 0 {
			t.Fatal("unselected face pixels reached first scene")
		}
	}
	_, second, _ := prepared.imageScene("[card:scene-two] The selected person at the park")
	if second == reference {
		t.Fatal("separate people share a reference")
	}
	_, none, err := prepared.imageScene("[card:scene-none] An empty park")
	if err != nil || none != "" {
		t.Fatal("unpersonalized scene received a reference")
	}
	for _, value := range []string{"A missing tag", "[card:person-0] A person", "[card:missing] A park", "[card:scene-one] [card:scene-two] mixed"} {
		if _, _, err := prepared.imageScene(value); err == nil {
			t.Fatal("invalid scene tag accepted")
		}
	}
	manifest.Cards[0].PersonIDs = []string{"Unselected"}
	if err := os.WriteFile(manifest.Cards[3].AttachmentPath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, again, _ := prepared.imageScene("[card:scene-one] The selected person at the beach")
	if again != reference {
		t.Fatal("in-flight persona changed")
	}
	contextText := prepared.prompt()
	if strings.Contains(contextText, service.uploads) || strings.Contains(contextText, "data:image/") || !strings.Contains(contextText, "personIds") {
		t.Fatal("unsafe or incomplete planning context")
	}
}

func TestChatCarouselAcceptsUnicodeEditorLimitsAndSummarizedImageScenes(t *testing.T) {
	service, manifest := carouselFixture(t)
	manifest.Cards[0].Title = strings.Repeat("界", 200)
	manifest.Cards[0].Instructions = strings.Repeat("詳", 32000)
	manifest.Cards[3].Instructions = strings.Repeat("人", 32000)
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal("valid editor text rejected", err)
	}
	planning := prepared.prompt()
	if !strings.Contains(planning, manifest.Cards[0].Instructions) || !strings.Contains(planning, manifest.Cards[3].Instructions) {
		t.Fatal("planning lost full card instructions")
	}
	scene, _, err := prepared.imageScene("[card:scene-one] A concise scene with the selected person")
	if err != nil || !strings.Contains(scene, manifest.Cards[0].Title) || strings.Contains(scene, "詳") || strings.Contains(scene, "人") || !strings.Contains(scene, "A concise scene") {
		t.Fatal("image scene did not preserve the summary and title", err)
	}
	manifest.Cards[0].Title += "界"
	if validateChatCarousel(manifest) == nil {
		t.Fatal("oversized title accepted")
	}
	manifest.Cards[0].Title = "Title"
	manifest.Cards[0].Instructions += "詳"
	if validateChatCarousel(manifest) == nil {
		t.Fatal("oversized instructions accepted")
	}
}

func TestChatCarouselOwnImagePersonalizationStaysWithinItsCard(t *testing.T) {
	service, photos := carouselFixture(t)
	manifest := &chatCarousel{Version: 1, Cards: []chatCarouselCard{
		{ID: "first", Role: "sketch", Title: "First", AttachmentPath: photos.Cards[3].AttachmentPath, Personalization: true},
		{ID: "second", Role: "sketch", Title: "Second", AttachmentPath: photos.Cards[5].AttachmentPath, Personalization: true},
		{ID: "off", Role: "sketch", Title: "Scenery", AttachmentPath: photos.Cards[6].AttachmentPath},
	}}
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.modelParts) != 6 || !strings.Contains(prepared.prompt(), `"personalization":true`) {
		t.Fatal("sketch planning lost its image or personalization selection")
	}
	want := map[string]string{}
	for _, id := range []string{"first", "second"} {
		data, err := normalizePreviousPrototypeImage(prepared.inputs[id].data, "openai-api")
		if err != nil {
			t.Fatal(err)
		}
		want[id] = base64.StdEncoding.EncodeToString(data)
	}
	if want["first"] == want["second"] {
		t.Fatal("fixture needs distinct images")
	}
	manifest.Cards[0].Personalization = false
	if err := os.WriteFile(manifest.Cards[0].AttachmentPath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := `<!doctype html><html><img id="off" src="assets/previous-person.png"></html>`
	html := `<!doctype html><html><img src="glowbomimages:[card:first] First scene"><img src="glowbomimages:[card:second] Second scene"><img id="off" src="glowbomimages:[card:off] Scenery"></html>`
	root, record, snapshot, _ := chatContinuityFixture(t, previous, html, "previous-person.png")
	ids := []string{"first", "second", "off"}
	calls := 0
	mockProjectIconProvider(t, func(request *http.Request) (*http.Response, error) {
		if calls >= len(ids) {
			t.Error("unexpected additional generation")
			return nil, fmt.Errorf("unexpected generation")
		}
		id := ids[calls]
		calls++
		reference, prompt := chatPersonalizationReference(t, request, "openai-api", want[id] != "")
		if reference != want[id] || strings.Contains(prompt, "Panel 1") {
			t.Error("own-image selection leaked another image or changed in flight")
		}
		return iconProviderResponse(projectIconTestImage(t, "png")), nil
	})
	final, warnings, err := materializeChatImages(context.Background(), root, record, html, chatImageOptions{SourceID: "openai-api", APIKey: "image-key", previousImages: snapshot, carousel: prepared}, func(map[string]any) {})
	if err != nil || len(warnings) != 0 || calls != 3 || strings.Contains(final, "glowbomimages:") {
		t.Fatal("own-image generation failed", calls, warnings, err)
	}
	evidence := t.TempDir()
	if _, err := prepared.save(evidence); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(evidence, "input-carousel.json"))
	if err != nil || !bytes.Contains(raw, []byte(`"personalization": true`)) || bytes.Contains(raw, []byte(service.uploads)) {
		t.Fatal("saved evidence lost the frozen selection", err)
	}
}

func TestChatCarouselOwnImageValidationAndTaggedInstructions(t *testing.T) {
	service, photos := carouselFixture(t)
	manifest := &chatCarousel{Version: 1, Cards: []chatCarouselCard{{ID: "own", Role: "sketch", Personalization: true}}}
	if validateChatCarousel(manifest) == nil {
		t.Fatal("personalization without an image accepted")
	}
	manifest.Cards[0].AttachmentPath = photos.Cards[3].AttachmentPath
	if _, err := service.prepareChatCarousel(manifest, "picsum"); err == nil {
		t.Fatal("sample photos accepted own-image personalization")
	}
	manifest.Cards[0].PersonIDs = []string{"Someone"}
	if validateChatCarousel(manifest) == nil {
		t.Fatal("own-image and person-group personalization accepted together")
	}
	manifest.Cards[0].PersonIDs = nil
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	instructions := chatImageInstructions(chatImageOptions{SourceID: "openai-api", carousel: prepared})
	if !strings.Contains(instructions, `src="glowbomimages:[card:SKETCH_ID] scene description"`) || !strings.Contains(instructions, "Do not invent remote image URLs or embed image data.") || strings.Contains(instructions, "glowbomimages:a quiet") {
		t.Fatal("carousel instructions conflict with required tagged URLs")
	}
	planning := prepared.prompt()
	if !strings.Contains(planning, "same project, not a separate page") || !strings.Contains(planning, "design context only, not an identity reference") || !strings.Contains(planning, "tag must name the card supplying its personalization reference, not the card supplying the layout") {
		t.Fatal("planning must combine layout and photo inputs without changing the reference owner")
	}
}

func TestChatCarouselTextOnlyAndUploadedDesignInputs(t *testing.T) {
	service, manifest := carouselFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.prepareChatCarousel(manifest, "openai-api", ctx); err != context.Canceled {
		t.Fatal("canceled carousel preparation continued", err)
	}
	text := &chatCarousel{Version: 1, Cards: []chatCarouselCard{{ID: "text-scene", Role: "sketch", Title: "Overview", Instructions: "A simple page"}, {ID: "text-reference", Role: "reference", Title: "Style", Instructions: "Calm and readable"}}}
	prepared, err := service.prepareChatCarousel(text, "openai-api")
	if err != nil || len(prepared.modelParts) != 0 {
		t.Fatal("text-only carousel rejected", err)
	}
	manifest.Cards = append(manifest.Cards, chatCarouselCard{ID: "style-ref", Role: "reference", Title: "Layout", AttachmentPath: manifest.Cards[3].AttachmentPath})
	prepared, err = service.prepareChatCarousel(manifest, "openai-api")
	if err != nil || len(prepared.modelParts) != 2 || prepared.modelParts[1]["filename"] != "card-style-ref.png" {
		t.Fatal("design reference not passed to planning", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	_ = os.WriteFile(outside, projectIconTestImage(t, "png"), 0600)
	manifest.Cards[len(manifest.Cards)-1].AttachmentPath = outside
	if _, err := service.prepareChatCarousel(manifest, "openai-api"); err == nil {
		t.Fatal("outside upload boundary accepted")
	}
}

func TestChatCarouselPersistsPortableOriginalEvidence(t *testing.T) {
	service, manifest := carouselFixture(t)
	manifest.Cards[0].InputSketch = &sketchDocument{Version: 1, Width: 200, Height: 100, Annotations: []json.RawMessage{}}
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	root, err := createSketchProject(t.TempDir(), "Carousel")
	if err != nil {
		t.Fatal(err)
	}
	original := append([]byte{}, prepared.inputs["person-0"].data...)
	_ = os.WriteFile(manifest.Cards[3].AttachmentPath, []byte("changed after capture"), 0600)
	record := ""
	if err := saveChatPrototypeWithCarousel(root, "<!doctype html><html><body>Carousel</body></html>", "", []chatMessage{{Role: "user", Text: "Build scenes"}}, "fixture/model", prepared.modelParts, nil, prepared, &record); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(record, "input-carousel.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(service.uploads)) || bytes.Contains(raw, []byte("data:image/")) || !bytes.Contains(raw, []byte(`"personIds"`)) || !bytes.Contains(raw, []byte("inputs/card-scene-one-sketch.json")) {
		t.Fatal("carousel evidence is not portable")
	}
	stored, err := os.ReadFile(filepath.Join(record, "inputs/card-person-0.png"))
	if err != nil || !bytes.Equal(stored, original) {
		t.Fatal("original frozen photo not preserved", err)
	}
	files, _ := os.ReadDir(filepath.Join(root, "prototype/assets"))
	if len(files) != 0 {
		t.Fatal("private person photos copied into public page assets")
	}
	request, _ := os.ReadFile(filepath.Join(record, "request.json"))
	if !bytes.Contains(request, []byte(`"inputCarouselPath": "input-carousel.json"`)) || bytes.Contains(request, []byte(service.uploads)) {
		t.Fatal("request lost portable carousel reference")
	}
}

func TestChatCarouselRoutesOnlySelectedPeopleAndCapsGeneration(t *testing.T) {
	isolateProjectIconCredentials(t)
	service, manifest := carouselFixture(t)
	prepared, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><html><img src="glowbomimages:missing tag"><img src="glowbomimages:[card:person-0] invalid">`
	scenes := []string{"scene-one", "scene-two", "scene-none", "scene-one", "scene-two", "scene-none"}
	for index, id := range scenes {
		html += fmt.Sprintf(`<img src="glowbomimages:[card:%s] Scene %d">`, id, index)
	}
	html += "</html>"
	root, record := chatImagesFixture(t, html)
	calls := 0
	mockProjectIconProvider(t, func(request *http.Request) (*http.Response, error) {
		if calls >= 4 {
			t.Fatal("carousel exceeded four image cap")
		}
		id := scenes[calls]
		calls++
		expected := prepared.scenes[id].reference
		reference, prompt := chatPersonalizationReference(t, request, "openai-api", expected != "")
		if reference != expected || strings.Contains(prompt, "Unselected") {
			t.Fatal("wrong persona reached generator")
		}
		return iconProviderResponse(projectIconTestImage(t, "png")), nil
	})
	options := chatImageOptions{SourceID: "openai-api", APIKey: "image-key", carousel: prepared}
	final, warnings, err := materializeChatImages(context.Background(), root, record, html, options, func(map[string]any) {})
	if err != nil || calls != 4 || len(warnings) != 2 || strings.Count(final, "assets/glowbom-image-") != 4 {
		t.Fatal("carousel routing or limit failed", calls, warnings, err)
	}
	if err := saveChatPrototype(root, html, final, nil, "fixture/model", nil, &record); err != nil {
		t.Fatal(err)
	}
	_, _, err = materializeChatImages(context.Background(), root, record, html, options, func(map[string]any) {})
	if err != nil || calls != 4 {
		t.Fatal("carousel retry regenerated cached images", calls, err)
	}
}

func TestChatCarouselPreservesSharedHiggsfieldCreditBudget(t *testing.T) {
	fake := readyProjectHiggsfield(t)
	service, manifest := carouselFixture(t)
	prepared, err := service.prepareChatCarousel(manifest, higgsfieldSourceID)
	if err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><html><img src="glowbomimages:[card:scene-one] One"><img src="glowbomimages:[card:scene-two] Two"></html>`
	root, record := chatImagesFixture(t, html)
	options := chatImageOptions{SourceID: higgsfieldSourceID, HiggsfieldMaxCredits: projectImageCredits(.25), carousel: prepared}
	final, warnings, err := materializeChatImages(context.Background(), root, record, html, options, func(map[string]any) {})
	if err != nil || higgsfieldPaidImageCalls(fake) != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "credit limit") || strings.Count(final, "assets/glowbom-image-") != 1 {
		t.Fatal("carousel escaped approved credit budget", warnings, err)
	}
}

func TestChatCarouselStreamKeepsPlanningAndPersonReferencesSeparate(t *testing.T) {
	isolateProjectIconCredentials(t)
	t.Setenv("GLOWBOM_CURSOR_BIN", "/missing/cursor-test")
	rememberCursorModels(nil)
	service, manifest := carouselFixture(t)
	original := `<!doctype html><html>Existing</html>`
	root, _ := chatImagesFixture(t, original)
	expected, err := service.prepareChatCarousel(manifest, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	planned := false
	mockProjectIconProvider(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "api.openai.com" {
			calls++
			reference, _ := chatPersonalizationReference(t, request, "openai-api", true)
			if reference != expected.scenes["scene-one"].reference {
				t.Error("stream used changed or wrong person photos")
			}
			return iconProviderResponse(projectIconTestImage(t, "png")), nil
		}
		switch request.URL.Path {
		case "/provider":
			return chatImageReply(`{"connected":["test"],"all":[{"id":"test","models":{"model":{"name":"Model","capabilities":{"input":{"image":true}}}}}]}`), nil
		case "/session":
			return chatImageReply(`{"id":"carousel-session","permission":[{"permission":"*","pattern":"*","action":"deny"}]}`), nil
		case "/experimental/tool/ids":
			return chatImageReply(`[]`), nil
		case "/event":
			return chatImageReply("data: {}\n\n"), nil
		case "/session/carousel-session/message":
			planned = true
			body, _ := io.ReadAll(request.Body)
			if bytes.Contains(body, []byte(service.uploads)) || bytes.Contains(body, []byte("data:image/")) || !bytes.Contains(body, []byte("scene-one")) || !bytes.Contains(body, []byte("Alex Smith")) || !bytes.Contains(body, []byte("[card:SKETCH_ID]")) {
				t.Error("planning request leaked person photos or omitted routing context")
			}
			if err := os.WriteFile(manifest.Cards[3].AttachmentPath, []byte("changed while planning"), 0600); err != nil {
				t.Error(err)
			}
			reply, _ := json.Marshal(map[string]any{"info": map[string]any{}, "parts": []map[string]string{{"type": "text", "text": `<!doctype html><html><img src="glowbomimages:[card:scene-one] At the beach"></html>`}}})
			return chatImageReply(string(reply)), nil
		default:
			return chatImageReply(`true`), nil
		}
	})
	service.directory = "/isolated"
	service.serverURL = "http://fixture"
	service.client = http.DefaultClient
	service.prepare = func() error { return nil }
	payload, _ := json.Marshal(chatRequest{ProjectPath: root, Mode: "prototype", Model: "test/model", Messages: []chatMessage{{Role: "user", Text: "Build the carousel scenes"}}, Carousel: manifest, Images: &chatImageOptions{SourceID: "openai-api", APIKey: "image-key"}})
	response := httptest.NewRecorder()
	service.streamHandler(response, httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(payload)))
	if !planned || calls != 1 || !strings.Contains(response.Body.String(), `"success":true`) {
		t.Fatal("carousel stream failed", response.Code, calls, response.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, ".glowbom/prototypes"))
	if err != nil || len(entries) != 2 {
		t.Fatal("carousel history missing", err)
	}
	for _, entry := range entries {
		record := filepath.Join(root, ".glowbom/prototypes", entry.Name())
		if _, err := os.Stat(filepath.Join(record, "input-carousel.json")); err != nil {
			continue
		}
		saved, err := os.ReadFile(filepath.Join(record, "inputs/card-person-0.png"))
		if err != nil || !bytes.Equal(saved, expected.inputs["person-0"].data) {
			t.Fatal("history did not preserve frozen input", err)
		}
		return
	}
	t.Fatal("carousel input manifest not saved")
}
