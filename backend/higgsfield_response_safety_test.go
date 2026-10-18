package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type higgsfieldSafetyTransport func(*http.Request) (*http.Response, error)

func (transport higgsfieldSafetyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type higgsfieldSafetyProbe struct {
	params []map[string]any
}

// Keep the fixture's authentication and session checks, then replace one tool's replies.
func useHiggsfieldSafetyReplies(t *testing.T, fake *fakeHiggsfield, tool string, replies ...string) *higgsfieldSafetyProbe {
	t.Helper()
	probe := &higgsfieldSafetyProbe{}
	fixture, err := url.Parse(fake.baseURL)
	if err != nil {
		t.Fatal(err)
	}
	previous := higgsfieldHTTPClient
	client := *previous
	base := previous.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = higgsfieldSafetyTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != fixture.Host {
			return nil, errors.New("response-safety test refused a non-fixture host")
		}
		var rpc struct {
			Method string `json:"method"`
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					Params map[string]any `json:"params"`
				} `json:"arguments"`
			} `json:"params"`
		}
		if request.GetBody != nil {
			body, err := request.GetBody()
			if err != nil {
				return nil, err
			}
			decodeErr := json.NewDecoder(body).Decode(&rpc)
			_ = body.Close()
			if decodeErr != nil {
				return nil, decodeErr
			}
		}
		response, err := base.RoundTrip(request)
		if err != nil || rpc.Method != "tools/call" || rpc.Params.Name != tool || response.StatusCode != http.StatusOK {
			return response, err
		}
		_ = response.Body.Close()
		probe.params = append(probe.params, rpc.Params.Arguments.Params)
		if len(probe.params) > len(replies) {
			return nil, fmt.Errorf("unexpected retry of %s", tool)
		}
		reply := replies[len(probe.params)-1]
		response.Body = io.NopCloser(strings.NewReader(reply))
		response.ContentLength = int64(len(reply))
		response.Header.Set("Content-Type", "application/json")
		return response, nil
	})
	higgsfieldHTTPClient = &client
	t.Cleanup(func() { higgsfieldHTTPClient = previous })
	return probe
}

func higgsfieldSafetyResult(content string) string {
	return `{"jsonrpc":"2.0","id":2,"result":{"structuredContent":` + content + `}}`
}

func TestHiggsfieldVideoAndSpeechEstimatesRequireExplicitPrices(t *testing.T) {
	for _, kind := range []string{"video", "speech"} {
		for _, test := range []struct {
			name    string
			content string
			valid   bool
			credits float64
		}{
			{name: "missing cost", content: `{}`},
			{name: "null response", content: `null`},
			{name: "null cost", content: `{"cost":null}`},
			{name: "missing credits", content: `{"cost":{}}`},
			{name: "null credits", content: `{"cost":{"credits":null}}`},
			{name: "negative credits", content: `{"cost":{"credits":-0.25}}`},
			{name: "overflowing credits", content: `{"cost":{"credits":1e999}}`},
			{name: "explicit zero", content: `{"cost":{"credits":0}}`, valid: true},
			{name: "fractional price", content: `{"cost":{"credits":1.25}}`, valid: true, credits: 1.25},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				fake := signInFakeHiggsfield(t)
				tool := "generate_video"
				if kind == "speech" {
					tool = "generate_audio"
				}
				probe := useHiggsfieldSafetyReplies(t, fake, tool, higgsfieldSafetyResult(test.content))
				var credits float64
				var err error
				if kind == "video" {
					credits, err = higgsfieldVideoEstimate(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"})
				} else {
					credits, err = higgsfieldSpeechEstimate(context.Background(), higgsfieldSpeechRequest{Prompt: "Hello.", VoiceID: "preset-1", VoiceType: "preset"})
				}
				if test.valid {
					if err != nil || credits != test.credits {
						t.Errorf("explicit price = %v, %v; want %v", credits, err, test.credits)
					}
				} else if err == nil {
					t.Errorf("invalid price was accepted as %v credits", credits)
				}
				if len(probe.params) != 1 || probe.params[0]["get_cost"] != true || fake.submits != 0 || fake.audioParams != nil {
					t.Errorf("price check must make one free preflight: calls=%v video submits=%d audio=%v", probe.params, fake.submits, fake.audioParams)
				}
			})
		}
	}
}

func TestHiggsfieldUnrecognizedGenerationRepliesStayUncertain(t *testing.T) {
	for _, test := range []struct {
		name  string
		reply string
	}{
		{name: "malformed RPC", reply: `{"jsonrpc":`},
		{name: "wrong result type", reply: higgsfieldSafetyResult(`{"results":"queued"}`)},
		{name: "empty result", reply: higgsfieldSafetyResult(`{}`)},
		{name: "unrecognized result", reply: higgsfieldSafetyResult(`{"queued":true}`)},
		{name: "unrecognized notice", reply: higgsfieldSafetyResult(`{"notice":{"type":"queued","message":"Your request is processing."}}`)},
		{name: "missing job ID", reply: higgsfieldSafetyResult(`{"jobs":[{"status":"pending"}]}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := signInFakeHiggsfield(t)
			probe := useHiggsfieldSafetyReplies(t, fake, "generate_video", test.reply)
			start, err := generateStudioSelectedVideo(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"}, "", "A quiet room.", nil)
			var notStarted *higgsfieldNotStartedError
			if err == nil || errors.As(err, &notStarted) || start != nil {
				t.Errorf("unrecognized reply must leave submission uncertain: start=%+v error=%T %v", start, err, err)
			}
			if len(probe.params) != 1 || fake.submits != 1 {
				t.Errorf("uncertain submission was retried: calls=%d submits=%d", len(probe.params), fake.submits)
			}
		})
	}
}

func TestHiggsfieldConfirmedRefusalsRemainNotStarted(t *testing.T) {
	for _, reply := range []string{
		higgsfieldSafetyResult(`{"error":"Not enough credits."}`),
		`{"jsonrpc":"2.0","id":2,"result":{"isError":true,"content":[{"type":"text","text":"Not enough credits."}]}}`,
	} {
		t.Run(reply, func(t *testing.T) {
			fake := signInFakeHiggsfield(t)
			probe := useHiggsfieldSafetyReplies(t, fake, "generate_video", reply)
			_, err := generateStudioSelectedVideo(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"}, "", "A quiet room.", nil)
			var notStarted *higgsfieldNotStartedError
			if !errors.As(err, &notStarted) || !strings.Contains(err.Error(), "Not enough credits.") {
				t.Errorf("confirmed refusal must preserve its reason: %T %v", err, err)
			}
			if len(probe.params) != 1 {
				t.Errorf("refused submission was retried %d times", len(probe.params)-1)
			}
		})
	}
}

func TestHiggsfieldPresetRecommendationStillDeclinesOnce(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	probe := useHiggsfieldSafetyReplies(t, fake, "generate_video",
		higgsfieldSafetyResult(`{"notice":{"type":"preset_recommendation","message":"Try a preset.","retry_literal_with":{"declined_preset_id":"preset-1"}}}`),
		higgsfieldSafetyResult(`{"results":[{"id":"job-after-preset","status":"pending"}]}`))
	start, err := generateStudioSelectedVideo(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"}, "", "A quiet room.", nil)
	if err != nil || start == nil || start.OperationID != "job-after-preset" {
		t.Fatalf("literal request after a preset recommendation failed: %+v %v", start, err)
	}
	if len(probe.params) != 2 || probe.params[0]["declined_preset_id"] != nil || probe.params[1]["declined_preset_id"] != "preset-1" {
		t.Errorf("only the recommended preset should be declined: %v", probe.params)
	}
}

func TestHiggsfieldRepeatedPresetRecommendationDoesNotLoop(t *testing.T) {
	fake := signInFakeHiggsfield(t)
	recommendation := higgsfieldSafetyResult(`{"notice":{"type":"preset_recommendation","message":"Try a preset.","retry_literal_with":{"declined_preset_id":"preset-1"}}}`)
	probe := useHiggsfieldSafetyReplies(t, fake, "generate_video", recommendation, recommendation)
	start, err := generateStudioSelectedVideo(context.Background(), studioVideoOptions{SourceID: higgsfieldSourceID, ModelID: "kling3_0_turbo", DurationSeconds: 5, Resolution: "720p", AspectRatio: "9:16"}, "", "A quiet room.", nil)
	var notStarted *higgsfieldNotStartedError
	if start != nil || !errors.As(err, &notStarted) {
		t.Errorf("a repeated preset recommendation must remain unsubmitted: start=%+v error=%T %v", start, err, err)
	}
	if len(probe.params) != 2 || probe.params[1]["declined_preset_id"] != "preset-1" {
		t.Errorf("preset recommendation should get only one decline retry: %v", probe.params)
	}
}
