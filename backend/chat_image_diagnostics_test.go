package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHiggsfieldPrototypeUsesFullStudioTimeoutForEveryImage(t *testing.T) {
	for count := 1; count <= maxChatImages; count++ {
		batch, image := chatImageGenerationTimeouts(higgsfieldSourceID, count)
		if image != studioImageGenerationTimeout(higgsfieldSourceID) || image != 3*time.Minute || batch != time.Duration(count)*image {
			t.Fatal("a prototype image lost part of its Studio waiting allowance", count, batch, image)
		}
	}
	for _, source := range []string{"openai-api", "gemini-api", "xai-api", "picsum"} {
		batch, image := chatImageGenerationTimeouts(source, 3)
		if batch != 5*time.Minute || image != 2*time.Minute {
			t.Fatal("an unrelated provider timeout changed", source, batch, image)
		}
	}
}

func TestHiggsfieldPrototypeReportsFailuresWithoutReplayingPaidRequests(t *testing.T) {
	for _, mode := range []string{"complete", "provider_failure", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			fake := readyProjectHiggsfield(t)
			diagnosticFile := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("GLOWBOM_DEBUG_LOG", diagnosticFile)
			original := `<!doctype html><html><img src="glowbomimages:First"><img src="glowbomimages:Second"><img src="glowbomimages:Third"></html>`
			root, record := chatImagesFixture(t, original)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := higgsfieldHTTPClient
			transport := client.Transport
			if transport == nil {
				transport = http.DefaultTransport
			}
			submitted := 0
			higgsfieldHTTPClient = &http.Client{Timeout: client.Timeout, Transport: projectIconTestTransport(func(request *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(request.Body)
				if err != nil {
					return nil, err
				}
				request.Body = io.NopCloser(bytes.NewReader(body))
				var rpc struct {
					Method string `json:"method"`
					Params struct {
						Name      string `json:"name"`
						Arguments struct {
							Params map[string]any `json:"params"`
						} `json:"arguments"`
					} `json:"params"`
				}
				_ = json.Unmarshal(body, &rpc)
				if rpc.Method == "tools/call" && rpc.Params.Name == "generate_image" && rpc.Params.Arguments.Params["get_cost"] != true {
					submitted++
					deadline, ok := request.Context().Deadline()
					if !ok || time.Until(deadline) < 170*time.Second || time.Until(deadline) > 3*time.Minute {
						t.Error("paid request did not receive the full Studio image deadline")
					}
				}
				if rpc.Method == "tools/call" && rpc.Params.Name == "jobs_wait" && submitted == 2 {
					switch mode {
					case "provider_failure":
						body := `{"jsonrpc":"2.0","id":2,"result":{"structuredContent":{"jobs":[{"job_id":"job-img","status":"failed","error":"unknown private-prompt https://signed.example/?token=token-secret"}]}}}`
						return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
					case "timeout":
						return nil, context.DeadlineExceeded
					case "cancel":
						cancel()
						return nil, context.Canceled
					}
				}
				return transport.RoundTrip(request)
			})}
			t.Cleanup(func() { higgsfieldHTTPClient = client })
			result, warnings, err := materializeChatImages(ctx, root, record, original, chatImageOptions{SourceID: higgsfieldSourceID, HiggsfieldMaxCredits: projectImageCredits(.75)}, func(map[string]any) {})
			expectedSubmits, expectedImages := 3, 3
			category, stage := "", "saved"
			switch mode {
			case "complete":
				if err != nil || len(warnings) != 0 {
					t.Fatal("successful images failed", warnings, err)
				}
			case "provider_failure":
				expectedImages, category, stage = 2, "provider_failed_unknown", "failed"
				if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "Image 2: Higgsfield reported that this image failed") {
					t.Fatal("known provider failure lost its truthful reason", warnings, err)
				}
			case "timeout":
				expectedImages, category, stage = 2, "timeout", "stopped_waiting"
				if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "Image 2: Glowbom reached its time limit") || !strings.Contains(warnings[0], "may still finish") {
					t.Fatal("local timeout claimed a terminal provider failure", warnings, err)
				}
			case "cancel":
				expectedSubmits, expectedImages, category, stage = 2, 1, "cancelled", "stopped_waiting"
				if err == nil || !strings.Contains(err.Error(), "Glowbom stopped waiting") || !strings.Contains(err.Error(), "may still finish") {
					t.Fatal("cancellation lost its uncertain provider outcome", err)
				}
			}
			if submitted != expectedSubmits || higgsfieldPaidImageCalls(fake) != expectedSubmits || strings.Count(result, "assets/glowbom-image-") != expectedImages {
				t.Fatal("wrong saved image count or a paid request was replayed", submitted, expectedImages)
			}
			data, err := os.ReadFile(diagnosticFile)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
				var event struct {
					Event    string               `json:"event"`
					Metadata glowbomDebugMetadata `json:"metadata"`
				}
				_ = json.Unmarshal(line, &event)
				if event.Event == "prototype_image" && event.Metadata.RequestID == prototypeImageDebugID(record, 2) && event.Metadata.Model == higgsfieldProjectImageModel && event.Metadata.ErrorCategory == category && event.Metadata.Stage == stage {
					found = true
				}
			}
			if !found {
				t.Fatal("missing scoped image diagnostics", category, stage)
			}
			for _, private := range []string{root, "private-prompt", "signed.example", "token-secret", "job-img", fake.accessToken} {
				if bytes.Contains(data, []byte(private)) || strings.Contains(strings.Join(warnings, " "), private) {
					t.Fatal("image diagnostics exposed private data")
				}
			}
		})
	}
}

func TestHiggsfieldPrototypeSafeImageReasons(t *testing.T) {
	for _, example := range []struct{ status, detail, text string }{
		{"nsfw", "private", "content checks"},
		{"failed", "insufficient_credits", "insufficient credits"},
		{"failed", "render timeout", "rendering this image timed out"},
		{"cancelled", "private", "image was cancelled"},
		{"failed", "internal_error", "service error"},
		{"rejected", "private", "rejected this image request"},
	} {
		message := higgsfieldPrototypeImageFailure(higgsfieldTerminalVideoFailure(example.status, example.detail))
		if !strings.Contains(message, example.text) || strings.Contains(message, "video") || strings.Contains(message, "private") {
			t.Fatal("incorrect or unsafe image reason", message)
		}
	}
	message := higgsfieldPrototypeImageFailure(errors.New("private raw provider error"))
	if strings.Contains(message, "private") || !strings.Contains(message, "could not confirm or save") {
		t.Fatal("unknown failure guessed a provider result")
	}
}
