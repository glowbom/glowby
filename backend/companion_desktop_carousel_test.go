package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func companionCarouselFixture() *companionDesktopCarousel {
	return &companionDesktopCarousel{Version: 1, Cards: []companionDesktopCarouselCard{
		{ID: "drawing-one", Instructions: "A board game"},
		{ID: "drawing-two", Instructions: "A player portrait"},
	}}
}

func waitDesktopCarouselImages(t *testing.T, s *companionSession, id string) map[string]any {
	t.Helper()
	// Four mock image encodes need more time with the race detector enabled.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		value := desktopPrototypeTestSnapshot(t, s, id)
		if value["status"] == "completed" {
			return value
		}
		if value["status"] != "running" {
			t.Fatal("unexpected carousel status", value)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("carousel images did not finish")
	return nil
}

func TestCompanionDesktopCarouselCapabilitiesAndLegacyCompatibility(t *testing.T) {
	s := testCompanion(t, desktopPrototypeTestAPI(t, func(http.ResponseWriter, *http.Request, chatRequest) {}))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodGet, "/projects/prototype", ""))
	var capabilities map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &capabilities) != nil || capabilities["version"] != float64(1) || capabilities["carousel"] != true || capabilities["maxCarouselCards"] != float64(10) || capabilities["maxImages"] != float64(4) {
		t.Fatal("carousel capability changed existing output limits", w.Body.String())
	}
	legacy := desktopPrototypeTestRequest(t, s)
	legacy.AttachmentID = companionTransferTestID
	legacy.Images = &companionDesktopPrototypeImages{SourceID: "xai-api", PersonalizationReferenceID: companionImageInputID}
	if err := normalizeCompanionDesktopPrototype(&legacy); err != nil || legacy.Carousel != nil {
		t.Fatal("legacy single image request changed", err)
	}
}

func TestCompanionDesktopCarouselRejectsInvalidInputBeforeStarting(t *testing.T) {
	s := testCompanion(t, desktopPrototypeTestAPI(t, func(http.ResponseWriter, *http.Request, chatRequest) { t.Error("invalid request started generation") }))
	base := desktopPrototypeTestRequest(t, s)
	for name, change := range map[string]func(*companionDesktopPrototypeRequest){
		"unknown version": func(r *companionDesktopPrototypeRequest) { r.Carousel.Version = 2 },
		"empty":           func(r *companionDesktopPrototypeRequest) { r.Carousel.Cards = nil },
		"eleven": func(r *companionDesktopPrototypeRequest) {
			for len(r.Carousel.Cards) < 11 {
				r.Carousel.Cards = append(r.Carousel.Cards, companionDesktopCarouselCard{ID: strings.Repeat("a", len(r.Carousel.Cards)), Instructions: "Sketch"})
			}
		},
		"duplicate":       func(r *companionDesktopPrototypeRequest) { r.Carousel.Cards[1].ID = r.Carousel.Cards[0].ID },
		"path id":         func(r *companionDesktopPrototypeRequest) { r.Carousel.Cards[0].ID = "../photo" },
		"path attachment": func(r *companionDesktopPrototypeRequest) { r.Carousel.Cards[0].AttachmentID = "/private/photo.png" },
		"empty item":      func(r *companionDesktopPrototypeRequest) { r.Carousel.Cards[0].Instructions = " " },
		"long item": func(r *companionDesktopPrototypeRequest) {
			r.Carousel.Cards[0].Instructions = strings.Repeat("x", companionCarouselInstructionBytes+1)
		},
		"total instructions": func(r *companionDesktopPrototypeRequest) {
			r.Carousel.Cards = nil
			for i := 1; i <= 9; i++ {
				r.Carousel.Cards = append(r.Carousel.Cards, companionDesktopCarouselCard{ID: strings.Repeat("a", i), Instructions: strings.Repeat("x", companionCarouselInstructionBytes)})
			}
		},
		"legacy attachment": func(r *companionDesktopPrototypeRequest) { r.AttachmentID = companionTransferTestID },
		"global reference": func(r *companionDesktopPrototypeRequest) {
			r.Images = &companionDesktopPrototypeImages{SourceID: "xai-api", PersonalizationReferenceID: companionTransferTestID}
		},
		"reference without image source": func(r *companionDesktopPrototypeRequest) {
			r.Carousel.Cards[0].AttachmentID = companionTransferTestID
			r.Carousel.Cards[0].PersonalizationReferenceID = companionImageInputID
		},
		"reference without canvas": func(r *companionDesktopPrototypeRequest) {
			r.Images = &companionDesktopPrototypeImages{SourceID: "xai-api"}
			r.Carousel.Cards[0].PersonalizationReferenceID = companionImageInputID
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			r.Carousel = companionCarouselFixture()
			change(&r)
			w := companionTransferCall(t, s, "/projects/prototype", r)
			if w.Code != http.StatusBadRequest || len(s.prototypeJobs) != 0 {
				t.Fatal("invalid carousel accepted", w.Code, w.Body.String())
			}
		})
	}
	unknownPath := `{"requestId":"` + base.RequestID + `","carousel":{"version":1,"cards":[{"id":"one","attachmentPath":"/private/photo.png"}]}}`
	w := httptest.NewRecorder()
	s.ServeHTTP(w, companionRequest(s, http.MethodPost, "/projects/prototype", unknownPath))
	if w.Code != 400 {
		t.Fatal("filesystem path field accepted")
	}
}

func TestCompanionDesktopCarouselFreezesScopedOriginalsAndSavesPortableHistory(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	var calls atomic.Int32
	var prepared *preparedChatCarousel
	s := testCompanion(t, desktopPrototypeTestAPI(t, func(w http.ResponseWriter, r *http.Request, request chatRequest) {
		calls.Add(1)
		data := companionChatRequestData(r.Context())
		prepared = data.carousel
		if prepared == nil || request.Carousel == nil || len(request.Carousel.Cards) != 2 || len(data.images) != 0 || data.personalizationReference != nil || request.Images == nil || request.Images.Personalization || request.Images.ReferencePath != "" {
			t.Error("carousel escaped its own images and personalization scope")
			return
		}
		root := request.ProjectPath
		var record string
		if err := saveChatPrototypeWithCarousel(root, "<!doctype html><html><body>Two inputs</body></html>", "", request.Messages, request.Model, prepared.modelParts, nil, prepared, &record); err != nil {
			t.Error(err)
		}
		emitCompanionChatTest(w, map[string]any{"done": true, "success": true, "previewReady": true})
	}))
	canvas := desktopPrototypeUpload(t, s, "canvas.jpg", "jpeg", 5, 4)
	original := desktopPrototypeUpload(t, s, "original.png", "png", 3, 4)
	request := desktopPrototypeTestRequest(t, s)
	request.Carousel = companionCarouselFixture()
	request.Carousel.Cards[0].AttachmentID = canvas.ID
	request.Carousel.Cards[0].PersonalizationReferenceID = original.ID
	request.Images = &companionDesktopPrototypeImages{SourceID: "xai-api"}
	w := companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != http.StatusAccepted {
		t.Fatal("carousel not accepted", w.Code, w.Body.String())
	}
	value := waitDesktopCarouselImages(t, s, strings.ToLower(request.RequestID))
	if prepared == nil || len(prepared.modelParts) != 2 || prepared.modelParts[1]["mime"] != "image/jpeg" || prepared.scenes["drawing-two"].reference != "" {
		t.Fatal("planner got private original or reference crossed cards")
	}
	reference := prepared.references["drawing-one"]
	expected, err := normalizePreviousPrototypeImage(reference.data, "xai-api")
	if err != nil || prepared.scenes["drawing-one"].reference != base64.StdEncoding.EncodeToString(expected) || bytes.Equal(reference.data, prepared.inputs["drawing-one"].data) {
		t.Fatal("annotated drawing replaced original personalization photo", err)
	}
	root := value["projectPath"].(string)
	records, err := os.ReadDir(filepath.Join(root, ".glowbom/prototypes"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range records {
		path := filepath.Join(root, ".glowbom/prototypes", record.Name())
		manifest, err := os.ReadFile(filepath.Join(path, "input-carousel.json"))
		if err != nil {
			continue
		}
		found = true
		if bytes.Contains(manifest, []byte(canvas.ID)) || bytes.Contains(manifest, []byte(original.ID)) || !bytes.Contains(manifest, []byte(`"personalizationReferencePath": "inputs/reference-card-drawing-one.png"`)) {
			t.Fatal("history lost portable reference identity", string(manifest))
		}
		stored, err := os.ReadFile(filepath.Join(path, "inputs/reference-card-drawing-one.png"))
		if err != nil || !bytes.Equal(stored, reference.data) {
			t.Fatal("original photo not saved", err)
		}
	}
	if !found {
		t.Fatal("carousel history missing")
	}
	w = companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatal("uncertain retry generated twice", w.Code, calls.Load())
	}
	request.Carousel.Cards[1].Instructions = "Changed second card"
	w = companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 409 || calls.Load() != 1 {
		t.Fatal("changed carousel reused accepted ID", w.Code)
	}
}

func TestCompanionDesktopCarouselOwnershipMutationAndVisionChecks(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	s := testCompanion(t, desktopPrototypeTestAPI(t, func(http.ResponseWriter, *http.Request, chatRequest) {
		t.Error("invalid attachment reached generation")
	}))
	canvas := desktopPrototypeUpload(t, s, "canvas.jpg", "jpeg", 5, 4)
	request := desktopPrototypeTestRequest(t, s)
	request.Carousel = companionCarouselFixture()
	request.Carousel.Cards[0].AttachmentID = canvas.ID
	request.Model = "provider/text"
	w := companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 400 {
		t.Fatal("non-vision model accepted carousel photo", w.Code)
	}
	request.Model = "provider/exact"
	stored := s.attachments[canvas.ID]
	changedOwner := stored
	changedOwner.ProjectID = "another-project"
	s.attachments[canvas.ID] = changedOwner
	w = companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 410 || len(s.prototypeJobs) != 0 {
		t.Fatal("other project attachment accepted", w.Code)
	}
	s.attachments[canvas.ID] = stored
	project, err := s.localChatProject(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.prepareDesktopCarousel(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	frozen := append([]byte{}, prepared.inputs["drawing-one"].data...)
	if err := os.WriteFile(filepath.Join(project.path, stored.relativePath), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frozen, prepared.inputs["drawing-one"].data) {
		t.Fatal("prepared request changed with upload")
	}
	w = companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 410 || len(s.prototypeJobs) != 0 {
		t.Fatal("modified attachment accepted", w.Code)
	}
}

func TestCompanionDesktopCarouselTenInputsDoNotRaiseBuildAttachmentLimit(t *testing.T) {
	s := testCompanion(t, desktopPrototypeTestAPI(t, func(http.ResponseWriter, *http.Request, chatRequest) {}))
	request := desktopPrototypeTestRequest(t, s)
	request.Carousel = &companionDesktopCarousel{Version: 1}
	ids := []string{}
	for i := 1; i <= 10; i++ {
		image := desktopPrototypeUpload(t, s, "sketch.png", "png", 4, 4)
		ids = append(ids, image.ID)
		request.Carousel.Cards = append(request.Carousel.Cards, companionDesktopCarouselCard{ID: strings.Repeat("a", i), AttachmentID: image.ID})
	}
	if err := normalizeCompanionDesktopPrototype(&request); err != nil {
		t.Fatal(err)
	}
	prepared, err := s.prepareDesktopCarousel(context.Background(), request)
	if err != nil || len(prepared.inputs) != 10 {
		t.Fatal("ten carousel inputs not accepted", err)
	}
	project, _ := s.localChatProject(context.Background())
	if _, _, err := s.buildAttachments(context.Background(), project, ids); err == nil {
		t.Fatal("carousel raised ordinary Build attachment cap")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.prepareDesktopCarousel(ctx, request); err == nil {
		t.Fatal("canceled carousel request continued")
	}
}

func TestCompanionDesktopCarouselRealHandlerRoutesOriginalAndKeepsFourOutputLimit(t *testing.T) {
	isolateProjectIconCredentials(t)
	withCodexModels(t, nil)
	t.Setenv("OPENCODE_URL", "http://fixture")
	t.Setenv("OPENAI_API_KEY", "image-key")
	originalBytes := companionImageFixture(t, "png", 3, 4)
	expectedReference, err := normalizePreviousPrototypeImage(originalBytes, "openai-api")
	if err != nil {
		t.Fatal(err)
	}
	var modelCalls, imageCalls atomic.Int32
	mockProjectIconProvider(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.openai.com" {
			imageCalls.Add(1)
			reference, _ := chatPersonalizationReference(t, r, "openai-api", true)
			if reference != base64.StdEncoding.EncodeToString(expectedReference) {
				t.Error("wrong carousel original reached image provider")
			}
			return iconProviderResponse(projectIconTestImage(t, "png")), nil
		}
		if r.URL.Host != "fixture" {
			return nil, fmt.Errorf("unexpected mock-only host")
		}
		switch r.URL.Path {
		case "/provider":
			if r.URL.Query().Get("directory") != "/isolated" {
				return chatImageReply(`{"connected":[],"all":[]}`), nil
			}
			return chatImageReply(`{"connected":["test"],"all":[{"id":"test","models":{"selected":{"name":"Selected","capabilities":{"input":{"image":true}}}}}]}`), nil
		case "/session":
			return chatImageReply(`{"id":"native-carousel","permission":[{"permission":"*","pattern":"*","action":"deny"}]}`), nil
		case "/experimental/tool/ids":
			return chatImageReply(`[]`), nil
		case "/event":
			return chatImageReply("data: {}\n\n"), nil
		case "/session/native-carousel/message":
			modelCalls.Add(1)
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte("card-drawing-one.jpg")) || !bytes.Contains(body, []byte("A player portrait")) || bytes.Contains(body, []byte(base64.StdEncoding.EncodeToString(originalBytes))) || bytes.Contains(body, []byte("image-key")) {
				t.Error("planner lost carousel input or received private original")
			}
			html := "<!doctype html><html><body>"
			for i := 1; i <= 5; i++ {
				html += fmt.Sprintf(`<img src="glowbomimages:[card:drawing-one] Portrait %d">`, i)
			}
			html += "</body></html>"
			reply, _ := json.Marshal(map[string]any{"info": map[string]any{}, "parts": []map[string]string{{"type": "text", "text": html}}})
			return chatImageReply(string(reply)), nil
		default:
			return chatImageReply(`true`), nil
		}
	})
	previousTransport := http.DefaultTransport
	http.DefaultTransport = http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	service := &chatService{uploads: t.TempDir(), directory: "/isolated", serverURL: "http://fixture", client: http.DefaultClient, prepare: func() error { return nil }}
	api := http.NewServeMux()
	api.HandleFunc("/chat/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"models": []chatModel{{ID: "test/selected", Name: "Selected", Images: true, Build: true}}})
	})
	api.HandleFunc("/opencode/project/icon/sources", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"sources": []projectIconSource{{ID: "openai-api", Available: true}}})
	})
	api.HandleFunc("/chat/stream", service.streamHandler)
	s := testCompanion(t, api)
	request := desktopPrototypeTestRequest(t, s)
	request.Model = "test/selected"
	request.Carousel = companionCarouselFixture()
	request.Carousel.Cards[0].AttachmentID = desktopPrototypeUpload(t, s, "canvas.jpg", "jpeg", 4, 3).ID
	request.Carousel.Cards[0].PersonalizationReferenceID = desktopPrototypeUpload(t, s, "original.png", "png", 3, 4).ID
	request.Images = &companionDesktopPrototypeImages{SourceID: "openai-api"}
	w := companionTransferCall(t, s, "/projects/prototype", request)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	value := waitDesktopCarouselImages(t, s, strings.ToLower(request.RequestID))
	if modelCalls.Load() != 1 || imageCalls.Load() != 4 || value["previewReady"] != true {
		t.Fatal("real carousel workflow lost output cap", modelCalls.Load(), imageCalls.Load(), value)
	}
	progress, _ := json.Marshal(value["generatedImages"])
	var generated []companionPrototypeImage
	if json.Unmarshal(progress, &generated) != nil || len(generated) != 4 {
		t.Fatal("carousel images not published to native progress", string(progress))
	}
}
