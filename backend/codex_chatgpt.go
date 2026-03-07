package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// buildCodexInput converts ChatMessage history + new user message into the
// structured input array expected by chatgpt.com/backend-api/codex/responses.
// User messages use {"type":"input_text"}, assistant messages use {"type":"output_text"}.
func buildCodexInput(prevMsgs []ChatMessage, newMsg string) []interface{} {
	var input []interface{}
	for _, m := range prevMsgs {
		switch m.Role {
		case "user":
			input = append(input, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "input_text", "text": m.Content},
				},
			})
		case "assistant":
			input = append(input, map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []map[string]interface{}{
					{"type": "output_text", "text": m.Content, "annotations": []string{}},
				},
				"status": "completed",
			})
		}
	}
	input = append(input, map[string]interface{}{
		"role": "user",
		"content": []map[string]interface{}{
			{"type": "input_text", "text": newMsg},
		},
	})
	return input
}

// buildCodexRequestBody builds the standard request body for the Codex Responses API.
func buildCodexRequestBody(instructions string, input []interface{}, reasoningEffort string, stream bool, modelID string) map[string]interface{} {
	resolvedModel := normalizeOpenAIModelID(modelID)
	return map[string]interface{}{
		"model":        resolvedModel,
		"store":        false,
		"stream":       stream,
		"instructions": instructions,
		"input":        input,
		"reasoning": map[string]interface{}{
			"effort":  reasoningEffort,
			"summary": "auto",
		},
		"text": map[string]interface{}{
			"verbosity": "medium",
		},
		"include":             []string{"reasoning.encrypted_content"},
		"tool_choice":         "auto",
		"parallel_tool_calls": true,
	}
}

// callChatGPTCodexStreaming proxies a chat request through the ChatGPT backend
// Codex Responses API, using the JWT access_token and chatgpt_account_id.
func callChatGPTCodexStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, accessToken, accountID, modelID string) error {
	if accessToken == "" {
		return fmt.Errorf("no ChatGPT access token provided")
	}
	if accountID == "" {
		return fmt.Errorf("no ChatGPT account ID provided")
	}

	systemMsg := defaultSystemPrompt
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
			break
		}
	}

	input := buildCodexInput(prevMsgs, newMsg)
	reqBody := buildCodexRequestBody(systemMsg, input, "medium", true, modelID)

	return doChatGPTCodexRequest(w, reqBody, accessToken, accountID)
}

// callChatGPTCodexDrawToCodeStreaming handles draw-to-code via the ChatGPT backend
// Codex Responses API.
func callChatGPTCodexDrawToCodeStreaming(w http.ResponseWriter, imageBase64, userPrompt, template, imageSource, accessToken, accountID, modelID string) error {
	if accessToken == "" {
		return fmt.Errorf("no ChatGPT access token provided")
	}
	if accountID == "" {
		return fmt.Errorf("no ChatGPT account ID provided")
	}

	systemPrompt := getSystemPrompt(template, imageSource)
	systemPrompt += " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	userMessage := detailedTask + "\n\n[An image/screenshot was provided for reference. Generate the code based on the description above.]"
	input := []interface{}{
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "input_text", "text": userMessage},
			},
		},
	}

	reqBody := buildCodexRequestBody(systemPrompt, input, "low", true, modelID)

	return doChatGPTCodexRequest(w, reqBody, accessToken, accountID)
}

// callChatGPTCodexNonStreaming makes a non-streaming call through the ChatGPT backend API.
func callChatGPTCodexNonStreaming(prevMsgs []ChatMessage, newMsg, accessToken, accountID, modelID string) (*R1Response, error) {
	if accessToken == "" {
		return &R1Response{
			AIResponse: "No ChatGPT access token provided. Please re-login in AI Settings.",
			TokenUsage: map[string]int{},
			Cost:       0,
		}, nil
	}

	systemMsg := defaultSystemPrompt
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
			break
		}
	}

	input := buildCodexInput(prevMsgs, newMsg)
	reqBody := buildCodexRequestBody(systemMsg, input, "medium", false, modelID)

	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST",
		"https://chatgpt.com/backend-api/codex/responses",
		bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}

	setChatGPTCodexHeaders(req, accessToken, accountID)

	fmt.Println("[CODEX] Calling ChatGPT Codex API (non-streaming)...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ChatGPT Codex API error (%d): %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse Responses API format
	var result struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Codex response: %v", err)
	}

	var text strings.Builder
	for _, out := range result.Output {
		for _, c := range out.Content {
			text.WriteString(c.Text)
		}
	}

	cost := estimateOpenAITextCost(modelID, result.Usage.InputTokens, result.Usage.OutputTokens)

	return &R1Response{
		AIResponse: text.String(),
		TokenUsage: map[string]int{
			"inputTokens":  result.Usage.InputTokens,
			"outputTokens": result.Usage.OutputTokens,
			"totalTokens":  result.Usage.InputTokens + result.Usage.OutputTokens,
		},
		Cost: cost,
	}, nil
}

// doChatGPTCodexRequest sends a streaming request to the ChatGPT backend Codex API
// and proxies SSE events back to the client in the same format as gpt5_responses_api.go.
func doChatGPTCodexRequest(w http.ResponseWriter, reqBody map[string]interface{}, accessToken, accountID string) error {
	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST",
		"https://chatgpt.com/backend-api/codex/responses",
		bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}

	setChatGPTCodexHeaders(req, accessToken, accountID)

	fmt.Println("[CODEX] Calling ChatGPT Codex API with streaming...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		return fmt.Errorf("ChatGPT Codex API error (%d): %s", resp.StatusCode, bodyText)
	}

	// Parse and proxy SSE — same event format as Responses API
	var responseBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var reasoningSent bool
	var reasoningStart time.Time
	var reasoningDurationSent bool
	var inputTokens, outputTokens, reasoningTokens int
	var lastSummaryIndex int = -1
	var currentThinkingStep strings.Builder
	var currentStepTitleSent bool

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
					Type           string `json:"type"`
					SequenceNumber int    `json:"sequence_number"`
					SummaryIndex   int    `json:"summary_index"`
					Delta          string `json:"delta"`
					Text           string `json:"text"`
					Response       struct {
						Usage struct {
							InputTokens         int `json:"input_tokens"`
							OutputTokens        int `json:"output_tokens"`
							OutputTokensDetails struct {
								ReasoningTokens int `json:"reasoning_tokens"`
							} `json:"output_tokens_details"`
						} `json:"usage"`
					} `json:"response"`
				}

				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					// Handle reasoning delta events
					if chunk.Type == "response.reasoning_summary_text.delta" {
						if reasoningStart.IsZero() {
							reasoningStart = time.Now()
						}
						if chunk.Delta != "" {
							if chunk.SummaryIndex != lastSummaryIndex {
								if lastSummaryIndex != -1 {
									reasoningBuilder.WriteString("\n\n")
								}
								currentThinkingStep.Reset()
								currentStepTitleSent = false
								lastSummaryIndex = chunk.SummaryIndex
							}

							reasoningBuilder.WriteString(chunk.Delta)
							currentThinkingStep.WriteString(chunk.Delta)

							if !currentStepTitleSent {
								text := currentThinkingStep.String()
								hasCompleteMarkdown := strings.HasPrefix(text, "**") && strings.Count(text, "**") >= 2
								hasNewlineSeparation := strings.Count(text, "\n") >= 2

								if hasCompleteMarkdown || hasNewlineSeparation {
									stepTitle := extractThinkingStepTitle(text)
									if stepTitle != "" {
										thinkingStepData := map[string]string{"thinkingStep": stepTitle}
										thinkingStepBytes, _ := json.Marshal(thinkingStepData)
										fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
										fmt.Printf("[CODEX] Sending thinking step: %s\n", stepTitle)
										if f, ok := w.(http.Flusher); ok {
											f.Flush()
										}
										currentStepTitleSent = true
									}
								}
							}
						}
					}

					// Handle output text delta events
					if chunk.Type == "response.output_text.delta" {
						if chunk.Delta != "" {
							// Send complete reasoning before first content chunk
							if !reasoningSent && reasoningBuilder.Len() > 0 {
								if !currentStepTitleSent && currentThinkingStep.Len() > 0 {
									stepTitle := extractThinkingStepTitle(currentThinkingStep.String())
									if stepTitle != "" {
										thinkingStepData := map[string]string{"thinkingStep": stepTitle}
										thinkingStepBytes, _ := json.Marshal(thinkingStepData)
										fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
										if f, ok := w.(http.Flusher); ok {
											f.Flush()
										}
									}
								}

								thinkingText := "<think>" + reasoningBuilder.String() + "</think>"
								chunkData := map[string]string{"chunk": thinkingText}
								chunkBytes, _ := json.Marshal(chunkData)
								fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
								fmt.Println("[CODEX] Sent complete reasoning chain to client")
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
								reasoningSent = true

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

							responseBuilder.WriteString(chunk.Delta)

							chunkData := map[string]string{"chunk": chunk.Delta}
							chunkBytes, _ := json.Marshal(chunkData)
							fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
						}
					}

					// Extract usage from final completed event
					if chunk.Type == "response.completed" {
						inputTokens = chunk.Response.Usage.InputTokens
						outputTokens = chunk.Response.Usage.OutputTokens
						reasoningTokens = chunk.Response.Usage.OutputTokensDetails.ReasoningTokens

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

						modelID, _ := reqBody["model"].(string)
						cost := estimateOpenAITextCost(modelID, inputTokens, outputTokens)

						doneData := map[string]interface{}{
							"done": true,
							"tokenUsage": map[string]int{
								"inputTokens":     inputTokens,
								"outputTokens":    outputTokens,
								"reasoningTokens": reasoningTokens,
								"totalTokens":     inputTokens + outputTokens,
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

	fmt.Printf("[CODEX] Stream complete: %d input tokens, %d output tokens (%d reasoning)\n", inputTokens, outputTokens, reasoningTokens)
	return nil
}

// setChatGPTCodexHeaders sets the required headers for ChatGPT backend API calls.
func setChatGPTCodexHeaders(req *http.Request, accessToken, accountID string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("chatgpt-account-id", accountID)
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("User-Agent", fmt.Sprintf("Glowbom (%s %s)", runtime.GOOS, runtime.GOARCH))
}
