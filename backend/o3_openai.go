package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func callo3miniOpenAiDrawToCodeApiFull(imageBase64, userPrompt, prompt, template, imageSource, openAiKey string) (*R1Response, error) {
	// No key => error response
	if openAiKey == "" {
		return &R1Response{
			AIResponse: "No OpenAI API key provided. Please check config.",
			TokenUsage: map[string]int{"inputTokens": 0, "outputTokens": 0, "totalTokens": 0},
			Cost:       0,
		}, nil
	}

	// 1) Describe image (OpenAI vision if needed; or skip if not available)
	imageDescription := "Image description not available"
	desc, err := getOpenAiImageDescription(imageBase64, openAiKey)
	if err == nil && desc != "" {
		imageDescription = desc
	}

	fmt.Println("imageDescription: ", imageDescription)

	// 2) Build system message
	systemMsg := prompt + " Replace @tailwind placeholders with the Tailwind CDN link."

	// 3) Build detailed task
	detailedTask := ""
	if template != "" {
		detailedTask = template
	} else {
		detailedTask = fmt.Sprintf(
			"Turn this into a single HTML file using Tailwind. Show real pictures from %s. The user describes this image as: %s",
			imageSource, userPrompt,
		)
	}

	// 4) Build final messages
	messages := []map[string]interface{}{
		{"role": "system", "content": systemMsg},
		{"role": "user", "content": fmt.Sprintf("Image description: %s", imageDescription)},
		{"role": "user", "content": fmt.Sprintf("Detailed Task Description: %s", detailedTask)},
		{"role": "user", "content": fmt.Sprintf("Human: %s\nAssistant:", userPrompt)},
	}

	fmt.Println("final messages: ", messages)

	// 5) Call the streaming OpenAI endpoint
	aiResp, err := callOpenAiSSE(messages, openAiKey)
	if err != nil {
		return nil, err
	}

	// 6) Build and return usage/cost
	inputLen := len(prompt) + len(imageDescription) + len(userPrompt)
	outputLen := len(aiResp)
	inputTokens := (inputLen + 3) / 4
	outputTokens := (outputLen + 3) / 4
	totalTokens := inputTokens + outputTokens

	return &R1Response{
		AIResponse: aiResp,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  totalTokens,
		},
		Cost: 0, // or compute as needed
	}, nil
}

// Optional chat version (no image):
func callo3miniOpenAiApiGo(prevMsgs []ChatMessage, newMsg, openAiKey string) (*R1Response, error) {
	if openAiKey == "" {
		return &R1Response{AIResponse: "No OpenAI key provided.", TokenUsage: nil, Cost: 0}, nil
	}
	// Gather system + user messages
	var systemMsg string
	var msgArr []map[string]interface{}
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}
	if systemMsg == "" {
		systemMsg = "You are a helpful assistant."
	}
	msgArr = append(msgArr, map[string]interface{}{"role": "system", "content": systemMsg})

	for _, m := range prevMsgs {
		if m.Role != "system" {
			msgArr = append(msgArr, map[string]interface{}{"role": m.Role, "content": m.Content})
		}
	}
	msgArr = append(msgArr, map[string]interface{}{"role": "user", "content": newMsg + "\nAssistant:"})

	aiResp, err := callOpenAiSSE(msgArr, openAiKey)
	if err != nil {
		return nil, err
	}

	inputLen := 0
	for _, m := range prevMsgs {
		inputLen += len(m.Content)
	}
	inputLen += len(newMsg)
	outputLen := len(aiResp)
	inputTokens := (inputLen + 3) / 4
	outputTokens := (outputLen + 3) / 4

	fmt.Println("aiResp: ", aiResp)

	return &R1Response{
		AIResponse: aiResp,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
		},
		Cost: 0,
	}, nil
}

// Utility to describe image with OpenAI Vision (if needed)
func getOpenAiImageDescription(imageBase64, openAiKey string) (string, error) {
	if imageBase64 == "" {
		return "", nil
	}
	// Example prompt
	payload := map[string]interface{}{
		"model": "llama-3.2-11b-vision-preview",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Describe this image in detail.", "images": []string{imageBase64}},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.openai.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("Vision error: %s", string(b))
	}

	var desc strings.Builder
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if strings.HasPrefix(strings.TrimSpace(line), "data:") {
				dataStr := strings.TrimSpace(line)[5:]
				if dataStr == "[DONE]" {
					break
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					if len(chunk.Choices) > 0 {
						desc.WriteString(chunk.Choices[0].Delta.Content)
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}
	return desc.String(), nil
}

// Reusable SSE fetch
func callOpenAiSSE(messages []map[string]interface{}, openAiKey string) (string, error) {
	reqBody := map[string]interface{}{
		"model":                 "deepseek-r1-distill-llama-70b",
		"messages":              messages,
		"stream":                true,
		"temperature":           0.1,
		"max_completion_tokens": 4096,
		"top_p":                 0.95,
	}
	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.openai.com/openai/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(b))
	}

	var aiResp strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				dataStr := strings.TrimSpace(line[5:])
				if dataStr == "[DONE]" {
					break
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					if len(chunk.Choices) > 0 {
						aiResp.WriteString(chunk.Choices[0].Delta.Content)
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}
	return aiResp.String(), nil
}
