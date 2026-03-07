package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

func callR1GroqDrawToCodeApiFull(imageBase64, userPrompt, prompt, template, imageSource, groqKey string) (*R1Response, error) {
	// No key => error response
	if groqKey == "" {
		return &R1Response{
			AIResponse: "No Groq API key provided for GPT-OSS 120B. Please check config.",
			TokenUsage: map[string]int{"inputTokens": 0, "outputTokens": 0, "totalTokens": 0},
			Cost:       0,
		}, nil
	}

	// 1) Describe image (Groq vision if needed; or skip if not available)
	imageDescription := "Image description not available"
	desc, err := getGroqImageDescription(imageBase64, groqKey)
	if err == nil && desc != "" {
		imageDescription = desc
	}

	fmt.Println("imageDescription: ", imageDescription)

	// 2) Build system message
	systemMsg := prompt + " Replace @tailwind placeholders with the Tailwind CDN link. Think through your approach before responding so the plan is solid."

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

	// 5) Call the streaming Groq endpoint
	aiResp, err := callGroqSSE(messages, groqKey)
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

// callGroqDrawToCodeStreaming handles draw-to-code with streaming for Groq
func callGroqDrawToCodeStreaming(w http.ResponseWriter, imageBase64, userPrompt, template, imageSource, groqKey string) error {
	if groqKey == "" {
		return fmt.Errorf("no Groq API key provided")
	}

	// Get image description
	imageDescription := "Image description not available"
	desc, err := getGroqImageDescription(imageBase64, groqKey)
	if err == nil && desc != "" {
		imageDescription = desc
	}

	// Build system message
	systemMsg := "You are a skilled web developer. Replace @tailwind placeholders with the Tailwind CDN link. Think through your approach before responding so the plan is solid."
	if template != "" {
		systemMsg = getSystemPrompt(template, imageSource)
	}

	// Build detailed task
	detailedTask := ""
	if template != "" {
		detailedTask = template
	} else {
		detailedTask = fmt.Sprintf(
			"Turn this into a single HTML file using Tailwind. Show real pictures from %s. The user describes this image as: %s",
			imageSource, userPrompt,
		)
	}

	// Build ChatMessage array with system message
	prevMsgs := []ChatMessage{
		{Role: "system", Content: systemMsg},
	}

	// Combine all user content into newMsg
	newMsg := fmt.Sprintf("Image description: %s\n\nDetailed Task Description: %s\n\nHuman: %s",
		imageDescription, detailedTask, userPrompt)

	// Call existing streaming function
	return callGroqGPTOSSChatStreaming(w, prevMsgs, newMsg, groqKey)
}

// Optional chat version (no image):
func callR1GroqApiGo(prevMsgs []ChatMessage, newMsg, groqKey string) (*R1Response, error) {
	if groqKey == "" {
		return &R1Response{AIResponse: "No Groq key provided for GPT-OSS 120B.", TokenUsage: nil, Cost: 0}, nil
	}
	// Gather system + user messages
	systemMsg := defaultSystemPrompt
	var msgArr []map[string]interface{}
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}
	systemMsg += "\nThink through the problem step by step before you respond so your answer is accurate and helpful."
	msgArr = append(msgArr, map[string]interface{}{"role": "system", "content": systemMsg})

	for _, m := range prevMsgs {
		if m.Role != "system" {
			msgArr = append(msgArr, map[string]interface{}{"role": m.Role, "content": m.Content})
		}
	}
	msgArr = append(msgArr, map[string]interface{}{"role": "user", "content": newMsg + "\nAssistant:"})

	aiResp, err := callGroqSSE(msgArr, groqKey)
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

// Utility to describe image with Groq Vision (if needed)
func getGroqImageDescription(imageBase64, groqKey string) (string, error) {
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
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+groqKey)

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
func callGroqSSE(messages []map[string]interface{}, groqKey string) (string, error) {
	reqBody := map[string]interface{}{
		"model":                 "openai/gpt-oss-120b",
		"messages":              messages,
		"stream":                true,
		"temperature":           1.0,
		"max_completion_tokens": 8192,
		"top_p":                 1.0,
		"reasoning_effort":      "medium",
		"stop":                  nil,
	}
	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+groqKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Groq API error (%d): %s", resp.StatusCode, string(b))
	}

	var aiResp strings.Builder
	var reasoning strings.Builder
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
							Content   string `json:"content"`
							Channel   string `json:"channel"`
							Reasoning string `json:"reasoning"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					if len(chunk.Choices) > 0 {
						delta := chunk.Choices[0].Delta
						if delta.Reasoning != "" {
							reasoning.WriteString(delta.Reasoning)
						}
						if strings.EqualFold(delta.Channel, "analysis") {
							if delta.Content != "" {
								reasoning.WriteString(delta.Content)
							}
							continue
						}
						aiResp.WriteString(delta.Content)
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
	finalResp := aiResp.String()
	reasoningStr := strings.TrimSpace(reasoning.String())
	if reasoningStr != "" {
		finalResp = fmt.Sprintf("<think>%s</think>\n%s", reasoningStr, finalResp)
	}
	return finalResp, nil
}

// callGroqGPTOSSChatStreaming streams Groq GPT-OSS 120B responses via SSE
func callGroqGPTOSSChatStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, groqKey string) error {
	if groqKey == "" {
		return fmt.Errorf("no Groq API key provided for GPT-OSS 120B")
	}

	systemMsg := defaultSystemPrompt
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}
	systemMsg += "\nThink through the problem step by step before you respond so your answer is accurate and helpful."

	messages := []map[string]interface{}{
		{"role": "system", "content": systemMsg},
	}
	for _, m := range prevMsgs {
		if m.Role != "system" {
			messages = append(messages, map[string]interface{}{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": newMsg + "\nAssistant:",
	})

	reqBody := map[string]interface{}{
		"model":                 "openai/gpt-oss-120b",
		"messages":              messages,
		"stream":                true,
		"temperature":           1.0,
		"max_completion_tokens": 8192,
		"top_p":                 1.0,
		"reasoning_effort":      "medium",
	}

	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+groqKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Groq API error (%d): %s", resp.StatusCode, string(b))
	}

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var thinkingSent bool
	var thinkingStart time.Time
	var thinkingDurationSent bool

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				dataStr := strings.TrimSpace(line[5:])
				if dataStr == "" {
					continue
				}
				if dataStr == "[DONE]" {
					break
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content   string `json:"content"`
							Channel   string `json:"channel"`
							Reasoning string `json:"reasoning"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					if len(chunk.Choices) > 0 {
						delta := chunk.Choices[0].Delta
						if delta.Reasoning != "" {
							if thinkingStart.IsZero() {
								thinkingStart = time.Now()
							}
							reasoningBuilder.WriteString(delta.Reasoning)
						}
						if strings.EqualFold(delta.Channel, "analysis") {
							if delta.Content != "" {
								if thinkingStart.IsZero() {
									thinkingStart = time.Now()
								}
								reasoningBuilder.WriteString(delta.Content)
							}
							continue
						}
						if delta.Content != "" {
							contentBuilder.WriteString(delta.Content)
							if !thinkingSent && reasoningBuilder.Len() > 0 {
								thinkingText := fmt.Sprintf("<think>%s</think>", strings.TrimSpace(reasoningBuilder.String()))
								chunkData := map[string]string{"chunk": thinkingText}
								if chunkBytes, err := json.Marshal(chunkData); err == nil {
									fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
									if f, ok := w.(http.Flusher); ok {
										f.Flush()
									}
								}
								thinkingSent = true
							}

							chunkData := map[string]string{"chunk": delta.Content}
							if chunkBytes, err := json.Marshal(chunkData); err == nil {
								fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
							}

							if !thinkingDurationSent && !thinkingStart.IsZero() {
								duration := time.Since(thinkingStart).Seconds()
								duration = math.Round(duration*10) / 10
								metaData := map[string]any{
									"meta": map[string]any{
										"thinkingSeconds": duration,
									},
								}
								if metaBytes, err := json.Marshal(metaData); err == nil {
									fmt.Fprintf(w, "data: %s\n\n", metaBytes)
									if f, ok := w.(http.Flusher); ok {
										f.Flush()
									}
								}
								thinkingDurationSent = true
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	if !thinkingSent && reasoningBuilder.Len() > 0 {
		thinkingText := fmt.Sprintf("<think>%s</think>", strings.TrimSpace(reasoningBuilder.String()))
		chunkData := map[string]string{"chunk": thinkingText}
		if chunkBytes, err := json.Marshal(chunkData); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}

	if !thinkingDurationSent && !thinkingStart.IsZero() {
		duration := time.Since(thinkingStart).Seconds()
		duration = math.Round(duration*10) / 10
		metaData := map[string]any{
			"meta": map[string]any{
				"thinkingSeconds": duration,
			},
		}
		if metaBytes, err := json.Marshal(metaData); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", metaBytes)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}

	// Send done event with usage estimate
	inputLen := len(newMsg)
	for _, m := range prevMsgs {
		inputLen += len(m.Content)
	}
	outputLen := len(contentBuilder.String()) + reasoningBuilder.Len()
	inputTokens := (inputLen + 3) / 4
	outputTokens := (outputLen + 3) / 4
	doneData := map[string]any{
		"done": true,
		"tokenUsage": map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
		},
		"cost": 0.0,
	}
	if doneBytes, err := json.Marshal(doneData); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", doneBytes)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return nil
}
