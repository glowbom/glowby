package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// callGeminiVideoAnalysis sends a recorded video (base64) to Gemini for understanding.
// It returns a short analysis that can be attached to the user's chat message.
func callGeminiVideoAnalysis(videoBase64, mimeType, prompt, apiKey string) (*R1Response, error) {
	if strings.TrimSpace(apiKey) == "" {
		return &R1Response{
			AIResponse: "No Gemini API key provided. Add it in Settings to analyze video.",
			TokenUsage: map[string]int{},
			Cost:       0,
		}, nil
	}

	if mimeType == "" {
		mimeType = "video/mp4"
	}

	contents := []map[string]interface{}{
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"text": prompt},
				map[string]interface{}{
					"inline_data": map[string]interface{}{
						"mime_type": mimeType,
						"data":      videoBase64,
					},
				},
			},
		},
	}

	// Use Gemini 3 Flash preview by default for faster multimodal analysis.
	model := "gemini-3-flash-preview"
	aiResp, inputTokens, outputTokens, err := callGeminiAPIWithModel(contents, apiKey, 12000, "", model)
	if err != nil {
		return nil, err
	}

	cost := (float64(inputTokens) / 1_000_000.0 * 1.25) + (float64(outputTokens) / 1_000_000.0 * 5.0)

	return &R1Response{
		AIResponse: aiResp,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
		},
		Cost: cost,
	}, nil
}

// callGeminiAPIWithModel mirrors callGeminiAPI but lets callers choose the model (needed for video support).
func callGeminiAPIWithModel(contents []map[string]interface{}, apiKey string, maxTokens int, thinkingBudget string, modelName string) (string, int, int, error) {
	start := time.Now()
	fmt.Printf("[GeminiVideo] request model=%s\n", modelName)
	thinkingCfg := map[string]interface{}{
		"thinkingBudget":  -1,
		"includeThoughts": true,
	}

	if thinkingBudget != "" {
		thinkingCfg["thinkingBudget"] = thinkingBudget
	}

	generationConfig := map[string]interface{}{
		"maxOutputTokens": maxTokens,
		"temperature":     0.6,
		"thinkingConfig":  thinkingCfg,
	}

	reqBody := map[string]interface{}{
		"contents":         contents,
		"generationConfig": generationConfig,
	}

	jsonBytes, _ := json.Marshal(reqBody)
	if modelName == "" {
		modelName = "gemini-3-flash-preview"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	fmt.Printf("[GeminiVideo] response status=%d elapsed=%s\n", resp.StatusCode, elapsed)

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Gemini API error (%d): %s", resp.StatusCode, string(b))
		return "", 0, 0, fmt.Errorf("%s", errMsg)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []map[string]interface{} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", 0, 0, fmt.Errorf("no candidates in Gemini response")
	}

	var responseText strings.Builder
	var thinking strings.Builder

	for _, part := range geminiResp.Candidates[0].Content.Parts {
		isThought := false
		if thoughtVal, hasThought := part["thought"]; hasThought {
			if thoughtBool, ok := thoughtVal.(bool); ok && thoughtBool {
				isThought = true
			} else if thoughtStr, ok := thoughtVal.(string); ok && thoughtStr != "" {
				isThought = true
			}
		}

		if text, ok := part["text"].(string); ok {
			if isThought {
				fmt.Fprintf(&thinking, "<think>%s</think>\n", text)
			} else {
				responseText.WriteString(text)
			}
		}
	}

	final := strings.TrimSpace(thinking.String() + "\n" + responseText.String())
	inputTokens := geminiResp.UsageMetadata.PromptTokenCount
	outputTokens := geminiResp.UsageMetadata.CandidatesTokenCount

	return final, inputTokens, outputTokens, nil
}
