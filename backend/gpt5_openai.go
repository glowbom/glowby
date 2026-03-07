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

// callGPT5DrawToCodeApiFull handles draw-to-code using GPT-5 with native vision
func callGPT5DrawToCodeApiFull(imageBase64, userPrompt, template, imageSource, apiKey, openAIModel string) (*R1Response, error) {
	if apiKey == "" {
		return &R1Response{
			AIResponse: "No OpenAI API key provided. Please add your key in Settings.",
			TokenUsage: map[string]int{"inputTokens": 0, "outputTokens": 0, "totalTokens": 0},
			Cost:       0,
		}, nil
	}
	resolvedModel := normalizeOpenAIModelID(openAIModel)

	// 1) Build system prompt (developer role)
	systemPrompt := getSystemPrompt(template, imageSource)
	systemPrompt += " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."

	// 2) Build detailed task
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	// 3) Construct GPT-5.2 request with vision
	// GPT-5.2 uses "developer" role for system messages and supports vision in user messages
	messages := []map[string]interface{}{
		{
			"role":    "developer",
			"content": systemPrompt,
		},
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": detailedTask,
				},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64),
					},
				},
			},
		},
	}

	// 4) Call OpenAI API with GPT-5.2
	aiResp, inputTokens, outputTokens, err := callGPT5API(messages, apiKey, 32768, "low", resolvedModel)
	if err != nil {
		return nil, err
	}

	cost := estimateOpenAITextCost(resolvedModel, inputTokens, outputTokens)

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

// callGPT5DrawToCodeStreaming handles draw-to-code with streaming for GPT-5.2
func callGPT5DrawToCodeStreaming(w http.ResponseWriter, imageBase64, userPrompt, template, imageSource, apiKey, openAIModel string) error {
	if apiKey == "" {
		return fmt.Errorf("no OpenAI API key provided")
	}
	resolvedModel := normalizeOpenAIModelID(openAIModel)

	// Build system prompt (developer role)
	systemPrompt := getSystemPrompt(template, imageSource)
	systemPrompt += " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."

	// Build detailed task
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	// Construct GPT-5.2 request with vision
	messages := []map[string]interface{}{
		{
			"role":    "developer",
			"content": systemPrompt,
		},
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": detailedTask,
				},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64),
					},
				},
			},
		},
	}

	// Use the same high output budget as non-streaming draw-to-code to avoid truncated translations.
	return callGPT5APIStreaming(w, messages, apiKey, 32768, "low", resolvedModel)
}

// callGPT5ApiGo handles normal chat using GPT-5.2
func callGPT5ApiGo(prevMsgs []ChatMessage, newMsg, apiKey, openAIModel string) (*R1Response, error) {
	if apiKey == "" {
		return &R1Response{
			AIResponse: "No OpenAI API key provided. Please add your key in Settings.",
			TokenUsage: map[string]int{},
			Cost:       0,
		}, nil
	}
	resolvedModel := normalizeOpenAIModelID(openAIModel)

	// Gather system message and convert to GPT-5.2 format
	systemMsg := defaultSystemPrompt
	var messages []map[string]interface{}

	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}

	// Add developer message (GPT-5.2's system role)
	messages = append(messages, map[string]interface{}{
		"role":    "developer",
		"content": systemMsg,
	})

	// Add conversation history
	for _, m := range prevMsgs {
		if m.Role == "user" || m.Role == "assistant" {
			messages = append(messages, map[string]interface{}{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}

	// Add new user message
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": newMsg,
	})

	// Call GPT-5.2 API
	aiResp, inputTokens, outputTokens, err := callGPT5API(messages, apiKey, 8192, "low", resolvedModel)
	if err != nil {
		return nil, err
	}

	cost := estimateOpenAITextCost(resolvedModel, inputTokens, outputTokens)

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

// callGPT5API makes the actual HTTP request to OpenAI API with streaming support
func callGPT5API(messages []map[string]interface{}, apiKey string, maxTokens int, reasoningEffort, openAIModel string) (string, int, int, error) {
	resolvedModel := normalizeOpenAIModelID(openAIModel)
	reqBody := map[string]interface{}{
		"model":                 resolvedModel,
		"reasoning_effort":      reasoningEffort, // "low", "medium", or "high"
		"max_completion_tokens": maxTokens,
		"messages":              messages,
		"stream":                true,
		"stream_options": map[string]interface{}{
			"include_usage": true,
		},
	}

	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	fmt.Println("[DEBUG] Calling GPT-5 API with streaming...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		if isOpenAIMissingModelScope(bodyText) {
			return "", 0, 0, fmt.Errorf("%s", openAIModelScopeHelpMessage())
		}
		return "", 0, 0, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, bodyText)
	}

	// Parse streaming response
	var responseText strings.Builder
	var reasoning strings.Builder
	var inputTokens, outputTokens int

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
							Reasoning struct {
								Summary []struct {
									Type string `json:"type"`
									Text string `json:"text"`
								} `json:"summary"`
							} `json:"reasoning"`
						} `json:"delta"`
					} `json:"choices"`
					Usage struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
					} `json:"usage"`
				}

				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					if len(chunk.Choices) > 0 {
						// Extract content
						responseText.WriteString(chunk.Choices[0].Delta.Content)

						// Extract reasoning summaries
						for _, summary := range chunk.Choices[0].Delta.Reasoning.Summary {
							if summary.Type == "summary_text" {
								reasoning.WriteString(summary.Text)
							}
						}
					}

					// Extract usage from final chunk
					if chunk.Usage.PromptTokens > 0 {
						inputTokens = chunk.Usage.PromptTokens
						outputTokens = chunk.Usage.CompletionTokens
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", 0, 0, err
		}
	}

	// Prepend reasoning wrapped in <think> tags if present (matching Ollama format)
	fullResponse := responseText.String()
	if reasoning.Len() > 0 {
		fullResponse = "<think>" + reasoning.String() + "</think>" + fullResponse
	}

	fmt.Printf("[DEBUG] GPT-5 response: %d input tokens, %d output tokens\n", inputTokens, outputTokens)

	return fullResponse, inputTokens, outputTokens, nil
}

// callGPT5ChatStreaming streams GPT-5.2 response via SSE to the writer (for chat)
func callGPT5ChatStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, apiKey, openAIModel string) error {
	if apiKey == "" {
		return fmt.Errorf("no OpenAI API key provided")
	}
	resolvedModel := normalizeOpenAIModelID(openAIModel)

	// Gather system message and convert to GPT-5.2 format
	systemMsg := defaultSystemPrompt
	var messages []map[string]interface{}

	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}

	// Add developer message (GPT-5.2's system role)
	messages = append(messages, map[string]interface{}{
		"role":    "developer",
		"content": systemMsg,
	})

	// Add conversation history
	for _, m := range prevMsgs {
		if m.Role == "user" || m.Role == "assistant" {
			messages = append(messages, map[string]interface{}{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}

	// Add new user message
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": newMsg,
	})

	// Call streaming GPT-5.2 API
	return callGPT5APIStreaming(w, messages, apiKey, 8192, "low", resolvedModel)
}

// callGPT5APIStreaming streams GPT-5.2 responses via SSE directly to the HTTP response writer
func callGPT5APIStreaming(w http.ResponseWriter, messages []map[string]interface{}, apiKey string, maxTokens int, reasoningEffort, openAIModel string) error {
	resolvedModel := normalizeOpenAIModelID(openAIModel)
	reqBody := map[string]interface{}{
		"model":                 resolvedModel,
		"reasoning_effort":      reasoningEffort,
		"max_completion_tokens": maxTokens,
		"messages":              messages,
		"stream":                true,
		"stream_options": map[string]interface{}{
			"include_usage": true,
		},
	}

	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	fmt.Println("[DEBUG] Calling GPT-5 API with streaming (SSE)...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		if isOpenAIMissingModelScope(bodyText) {
			return fmt.Errorf("%s", openAIModelScopeHelpMessage())
		}
		return fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, bodyText)
	}

	// Stream response chunks to client
	var responseBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var reasoningSent bool
	var reasoningStart time.Time
	var reasoningDurationSent bool
	var inputTokens, outputTokens int

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
							Reasoning struct {
								Summary []struct {
									Type string `json:"type"`
									Text string `json:"text"`
								} `json:"summary"`
							} `json:"reasoning"`
						} `json:"delta"`
					} `json:"choices"`
					Usage struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
					} `json:"usage"`
				}

				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					// Debug: print raw chunk to see what we're getting
					if len(chunk.Choices) > 0 {
						fmt.Printf("[GPT-5 DEBUG] Chunk delta: %+v\n", chunk.Choices[0].Delta)
					}

					if len(chunk.Choices) > 0 {
						// Accumulate reasoning content
						for _, summary := range chunk.Choices[0].Delta.Reasoning.Summary {
							if summary.Type == "summary_text" {
								if reasoningStart.IsZero() {
									reasoningStart = time.Now()
								}
								fmt.Printf("[GPT-5 DEBUG] Found reasoning summary: %s\n", summary.Text)
								reasoningBuilder.WriteString(summary.Text)

								// Send individual thinking step to client immediately
								thinkingStepData := map[string]string{"thinkingStep": summary.Text}
								thinkingStepBytes, _ := json.Marshal(thinkingStepData)
								fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
								fmt.Println("[GPT-5] Sending thinking step:", summary.Text)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
							}
						}

						content := chunk.Choices[0].Delta.Content
						responseBuilder.WriteString(content)

						// Send accumulated reasoning before first content chunk
						if !reasoningSent && reasoningBuilder.Len() > 0 && (content != "" || chunk.Usage.PromptTokens > 0) {
							thinkingText := "<think>" + reasoningBuilder.String() + "</think>"
							chunkData := map[string]string{"chunk": thinkingText}
							chunkBytes, _ := json.Marshal(chunkData)
							fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
							fmt.Println("[GPT-5] Sending reasoning:", reasoningBuilder.String())
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
							reasoningSent = true
						}

						// Send content chunk
						if content != "" {
							chunkData := map[string]string{"chunk": content}
							chunkBytes, _ := json.Marshal(chunkData)
							fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
							fmt.Println("[GPT-5] Sending chunk:", content)
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}

							// Send reasoning duration after first content
							if !reasoningDurationSent && !reasoningStart.IsZero() {
								duration := time.Since(reasoningStart).Seconds()
								duration = math.Round(duration*10) / 10
								metaData := map[string]any{
									"meta": map[string]any{
										"thinkingSeconds": duration,
									},
								}
								metaBytes, _ := json.Marshal(metaData)
								fmt.Fprintf(w, "data: %s\n\n", metaBytes)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
								reasoningDurationSent = true
							}
						}
					}

					// Extract usage from final chunk
					if chunk.Usage.PromptTokens > 0 {
						inputTokens = chunk.Usage.PromptTokens
						outputTokens = chunk.Usage.CompletionTokens

						// Send thinking duration if not sent yet
						if !reasoningDurationSent && !reasoningStart.IsZero() {
							duration := time.Since(reasoningStart).Seconds()
							duration = math.Round(duration*10) / 10
							metaData := map[string]any{
								"meta": map[string]any{
									"thinkingSeconds": duration,
								},
							}
							metaBytes, _ := json.Marshal(metaData)
							fmt.Fprintf(w, "data: %s\n\n", metaBytes)
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
							reasoningDurationSent = true
						}

						// Calculate cost (GPT-5 pricing: $5 per 1M input, $20 per 1M output)
						cost := estimateOpenAITextCost(resolvedModel, inputTokens, outputTokens)

						// Send done event with usage
						doneData := map[string]interface{}{
							"done": true,
							"tokenUsage": map[string]int{
								"inputTokens":  inputTokens,
								"outputTokens": outputTokens,
								"totalTokens":  inputTokens + outputTokens,
							},
							"cost": cost,
						}
						doneBytes, _ := json.Marshal(doneData)
						fmt.Fprintf(w, "data: %s\n\n", doneBytes)
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
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

	fmt.Printf("[GPT-5] Stream complete: %d input tokens, %d output tokens\n", inputTokens, outputTokens)
	return nil
}
