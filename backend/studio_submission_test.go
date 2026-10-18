package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioGenerationNotSubmittedEvidenceForLocalRejections(t *testing.T) {
	for _, endpoint := range []struct {
		name, path string
		handler    http.HandlerFunc
	}{
		{"image", "/studio/images/generate", studioImageGenerateHandler},
		{"video", "/studio/videos/generate", studioVideoGenerateHandler},
	} {
		for _, rejection := range []string{"prompt", "options", "authentication", "identifier", "reference"} {
			t.Run(endpoint.name+"/"+rejection, func(t *testing.T) {
				isolateStudioRecovery(t)
				providerCalls := 0
				mockProjectIconProvider(t, func(r *http.Request) (*http.Response, error) {
					providerCalls++
					return nil, errors.New("local rejection must not contact provider")
				})
				values := map[string]any{"generationId": "fixture-local-rejection", "prompt": "Test generation"}
				if endpoint.name == "image" {
					values["sourceId"], values["apiKey"] = "openai-api", "fixture-key"
				}
				switch rejection {
				case "prompt":
					values["prompt"] = ""
				case "options":
					values["sourceId"] = "unknown-source"
				case "authentication":
					values["sourceId"] = "higgsfield"
				case "identifier":
					values["generationId"] = "bad-id"
				case "reference":
					values["referenceId"] = "missing-reference"
				}
				body := studioRecoveryJSON(t, values)
				request := studioProgressRequest(http.MethodPost, endpoint.path, body)
				if rejection == "authentication" {
					request.Header.Del("X-Glowbom-Token")
				}
				response := httptest.NewRecorder()
				endpoint.handler(response, request)
				if response.Code < 400 || response.Header().Get(studioGenerationNotSubmittedHeader) != "true" || providerCalls != 0 {
					t.Fatalf("missing pre-submission proof: status=%d marker=%q providerCalls=%d", response.Code, response.Header().Get(studioGenerationNotSubmittedHeader), providerCalls)
				}
				if !strings.Contains(response.Header().Get("Access-Control-Expose-Headers"), studioGenerationNotSubmittedHeader) {
					t.Fatal("pre-submission proof is not exposed to browser clients")
				}
			})
		}
	}
}

func TestStudioGenerationNotSubmittedEvidenceExcludesExistingOrUnreadableRequests(t *testing.T) {
	for _, endpoint := range []struct {
		kind, path string
		handler    http.HandlerFunc
	}{
		{"image", "/studio/images/generate", studioImageGenerateHandler},
		{"video", "/studio/videos/generate", studioVideoGenerateHandler},
	} {
		for _, existing := range []string{"started", "unreadable"} {
			t.Run(endpoint.kind+"/"+existing, func(t *testing.T) {
				isolateStudioRecovery(t)
				id := "fixture-existing-generation"
				if existing == "started" {
					if status := studioGenerations.begin(id, endpoint.kind); status != http.StatusOK {
						t.Fatal(status)
					}
					progress := studioProgress{id: id, store: studioGenerations}
					defer progress.finish()
				} else {
					dir, _, err := studioGenerationRecordsDirectory(true)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte("{broken"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				for _, rejection := range []string{"duplicate", "prompt", "options", "authentication", "body"} {
					values := map[string]any{"generationId": id, "prompt": "Keep original request"}
					switch rejection {
					case "prompt":
						values["prompt"] = ""
					case "options":
						values["sourceId"] = "unknown-source"
					case "authentication":
						values["sourceId"] = "higgsfield"
					}
					body := studioRecoveryJSON(t, values)
					if rejection == "body" {
						body = `{"generationId":"` + id + `","prompt":`
					}
					request := studioProgressRequest(http.MethodPost, endpoint.path, body)
					if rejection == "authentication" {
						request.Header.Del("X-Glowbom-Token")
					}
					response := httptest.NewRecorder()
					endpoint.handler(response, request)
					if response.Code < 400 || response.Header().Get(studioGenerationNotSubmittedHeader) != "" {
						t.Fatal("existing request was incorrectly declared never submitted", rejection, response.Code, response.Header())
					}
				}
			})
		}
	}
}

func TestStudioGenerationNotSubmittedEvidenceStopsAtProviderBoundary(t *testing.T) {
	for _, kind := range []string{"image", "video"} {
		t.Run(kind, func(t *testing.T) {
			isolateStudioRecovery(t)
			providerCalls := 0
			mockProjectIconProvider(t, func(r *http.Request) (*http.Response, error) {
				providerCalls++
				return nil, errors.New("response lost after possible submission")
			})
			values := map[string]any{"generationId": "fixture-ambiguous-submission", "prompt": "Keep this request"}
			handler, path := http.HandlerFunc(studioVideoGenerateHandler), "/studio/videos/generate"
			if kind == "image" {
				values["sourceId"], values["apiKey"] = "openai-api", "fixture-key"
				handler, path = studioImageGenerateHandler, "/studio/images/generate"
			}
			response := httptest.NewRecorder()
			handler(response, studioProgressRequest(http.MethodPost, path, studioRecoveryJSON(t, values)))
			if response.Code < 400 || providerCalls == 0 || response.Header().Get(studioGenerationNotSubmittedHeader) != "" {
				t.Fatalf("ambiguous provider outcome was incorrectly declared not submitted: status=%d calls=%d marker=%q", response.Code, providerCalls, response.Header().Get(studioGenerationNotSubmittedHeader))
			}
		})
	}
}

func TestStudioGenerationNotSubmittedEvidenceNeverMarksResumeRejection(t *testing.T) {
	isolateStudioRecovery(t)
	response := httptest.NewRecorder()
	studioGenerationResumeHandler(response, studioProgressRequest(http.MethodPost, "/studio/generation/resume", studioRecoveryJSON(t, map[string]string{"generationId": "fixture-unknown-saved-request"})))
	if response.Code < 400 || response.Header().Get(studioGenerationNotSubmittedHeader) != "" {
		t.Fatal("resume rejection claimed the original request was never submitted")
	}
}
