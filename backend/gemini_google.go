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

func isGeminiModelNotFound(statusCode int, body string) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "models/") && strings.Contains(lower, "not found")
}

// callGeminiDrawToCodeApiFull handles draw-to-code using Gemini with native vision
// Supports optional attachment (audio/file) inline_data in addition to the image.
func callGeminiDrawToCodeApiFull(imageBase64, userPrompt, template, imageSource, apiKey string, attachmentBase64 string, attachmentMime string, geminiModel string) (*R1Response, error) {
	if apiKey == "" {
		return &R1Response{
			AIResponse: "No Gemini API key provided. Please add your key in Settings.",
			TokenUsage: map[string]int{"inputTokens": 0, "outputTokens": 0, "totalTokens": 0},
			Cost:       0,
		}, nil
	}

	// 1) Build system prompt
	systemPrompt := getSystemPrompt(template, imageSource)
	systemPrompt += " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."

	// 2) Build detailed task
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	// 3) Construct Gemini request with vision
	// Gemini uses "contents" array with "role" and "parts"
	contents := []map[string]interface{}{
		{
			"role": "user",
			"parts": func() []interface{} {
				parts := []interface{}{
					map[string]interface{}{
						"text": systemPrompt + "\n\n" + detailedTask,
					},
				}
				if strings.TrimSpace(imageBase64) != "" {
					parts = append(parts, map[string]interface{}{
						"inline_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"data":      imageBase64,
						},
					})
				}
				if strings.TrimSpace(attachmentBase64) != "" && strings.TrimSpace(attachmentMime) != "" {
					parts = append(parts, map[string]interface{}{
						"inline_data": map[string]interface{}{
							"mime_type": attachmentMime,
							"data":      attachmentBase64,
						},
					})
				}
				return parts
			}(),
		},
	}

	// 4) Call Gemini API with a larger output budget to avoid truncated long-form code generations.
	aiResp, inputTokens, outputTokens, err := callGeminiAPI(contents, apiKey, 32768, "", geminiModel)
	if err != nil {
		return nil, err
	}

	// 5) Calculate cost (Gemini 3.1 Pro pricing)
	// Input: $1.25 per 1M tokens, Output: $5 per 1M tokens
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

// callGeminiDrawToCodeStreaming handles draw-to-code with streaming for Gemini.
func callGeminiDrawToCodeStreaming(w http.ResponseWriter, imageBase64, userPrompt, template, imageSource, apiKey, attachmentBase64, attachmentMime, geminiModel string) error {
	if apiKey == "" {
		return fmt.Errorf("no Gemini API key provided")
	}

	// DEBUG: Log all input parameters for translation debugging
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("[GEMINI DEBUG] callGeminiDrawToCodeStreaming called")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("[GEMINI DEBUG] template: %q\n", template)
	fmt.Printf("[GEMINI DEBUG] imageSource: %q\n", imageSource)
	fmt.Printf("[GEMINI DEBUG] imageBase64 length: %d bytes (not printed)\n", len(imageBase64))
	fmt.Printf("[GEMINI DEBUG] attachmentBase64 length: %d bytes (not printed)\n", len(attachmentBase64))
	fmt.Printf("[GEMINI DEBUG] attachmentMime: %q\n", attachmentMime)
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("[GEMINI DEBUG] userPrompt (truncated to 2000 chars):")
	if len(userPrompt) > 2000 {
		fmt.Println(userPrompt[:2000] + "\n... [TRUNCATED, total length: " + fmt.Sprintf("%d", len(userPrompt)) + " chars]")
	} else {
		fmt.Println(userPrompt)
	}
	fmt.Println(strings.Repeat("-", 80))

	// Build system prompt
	systemPrompt := getSystemPrompt(template, imageSource)
	systemPrompt += " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."

	// Build detailed task
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	// DEBUG: Log the constructed prompts (truncated)
	fmt.Println("[GEMINI DEBUG] systemPrompt length:", len(systemPrompt), "chars")
	fmt.Println("[GEMINI DEBUG] detailedTask length:", len(detailedTask), "chars")
	fmt.Println(strings.Repeat("-", 80))
	finalPrompt := systemPrompt + "\n\n" + detailedTask
	fmt.Println("[GEMINI DEBUG] FINAL PROMPT TO GEMINI (truncated to 3000 chars):")
	if len(finalPrompt) > 3000 {
		fmt.Println(finalPrompt[:3000] + "\n... [TRUNCATED, total length: " + fmt.Sprintf("%d", len(finalPrompt)) + " chars]")
	} else {
		fmt.Println(finalPrompt)
	}
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Construct Gemini request with vision
	contents := []map[string]interface{}{
		{
			"role": "user",
			"parts": func() []interface{} {
				parts := []interface{}{
					map[string]interface{}{
						"text": systemPrompt + "\n\n" + detailedTask,
					},
				}
				if strings.TrimSpace(imageBase64) != "" {
					parts = append(parts, map[string]interface{}{
						"inline_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"data":      imageBase64,
						},
					})
				}
				if strings.TrimSpace(attachmentBase64) != "" && strings.TrimSpace(attachmentMime) != "" {
					parts = append(parts, map[string]interface{}{
						"inline_data": map[string]interface{}{
							"mime_type": attachmentMime,
							"data":      attachmentBase64,
						},
					})
				}
				return parts
			}(),
		},
	}

	// Call existing streaming function
	// Match non-streaming output budget to reduce partial/truncated translated files.
	return callGeminiAPIStreaming(w, contents, apiKey, 32768, "", geminiModel)
}

// callGeminiApiGo handles normal chat using Gemini.
func callGeminiApiGo(prevMsgs []ChatMessage, newMsg, apiKey, geminiModel string) (*R1Response, error) {
	if apiKey == "" {
		return &R1Response{
			AIResponse: "No Gemini API key provided. Please add your key in Settings.",
			TokenUsage: map[string]int{},
			Cost:       0,
		}, nil
	}

	// Gather system message and convert to Gemini format
	systemMsg := defaultSystemPrompt
	var contents []map[string]interface{}

	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}

	// Add system message as first user message (Gemini doesn't have separate system role)
	hasUserMsg := false
	for _, m := range prevMsgs {
		if m.Role == "user" || m.Role == "assistant" {
			hasUserMsg = true
			break
		}
	}

	if hasUserMsg {
		// Add system prompt as first user turn
		contents = append(contents, map[string]interface{}{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"text": systemMsg},
			},
		})
		// Add a model response acknowledging the system message
		contents = append(contents, map[string]interface{}{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"text": "Understood. I'll follow these instructions."},
			},
		})
	}

	// Add conversation history (convert "assistant" to "model" for Gemini)
	for _, m := range prevMsgs {
		if m.Role == "user" {
			contents = append(contents, map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": m.Content},
				},
			})
		} else if m.Role == "assistant" {
			contents = append(contents, map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"text": m.Content},
				},
			})
		}
	}

	// Add new user message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []interface{}{
			map[string]interface{}{"text": newMsg},
		},
	})

	// Call Gemini API
	aiResp, inputTokens, outputTokens, err := callGeminiAPI(contents, apiKey, 12000, "", geminiModel)
	if err != nil {
		return nil, err
	}

	// Calculate cost
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

// callGeminiAPI makes the actual HTTP request to Gemini API (non-streaming).
func callGeminiAPI(contents []map[string]interface{}, apiKey string, maxTokens int, thinkingBudget string, geminiModel string) (string, int, int, error) {
	// Build generation config with thinking config
	thinkingCfg := map[string]interface{}{
		"thinkingBudget":  -1,   // -1 means dynamic budget (default)
		"includeThoughts": true, // Include thought summaries in response
	}

	// Set thinking budget if provided
	if thinkingBudget != "" {
		thinkingCfg["thinkingBudget"] = thinkingBudget
	}

	generationConfig := map[string]interface{}{
		"maxOutputTokens": maxTokens,
		"temperature":     1.0,
		"thinkingConfig":  thinkingCfg,
	}

	reqBody := map[string]interface{}{
		"contents":         contents,
		"generationConfig": generationConfig,
	}

	jsonBytes, _ := json.Marshal(reqBody)
	resolvedModel := normalizeGeminiModelID(geminiModel)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", resolvedModel, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("[DEBUG] Calling Gemini API model=%s...\n", resolvedModel)
	// Request body not printed (contains large base64 images)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[Gemini ERROR] HTTP request failed: %v\n", err)
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	fmt.Printf("[DEBUG] Gemini API response status: %d\n", resp.StatusCode)

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		if isGeminiModelNotFound(resp.StatusCode, bodyText) && resolvedModel != "gemini-3.1-pro-preview" {
			fmt.Printf("[Gemini WARN] Model %s not available, retrying with gemini-3.1-pro-preview\n", resolvedModel)
			return callGeminiAPI(contents, apiKey, maxTokens, thinkingBudget, "gemini-3.1-pro-preview")
		}
		errMsg := fmt.Sprintf("Gemini API error (%d): %s", resp.StatusCode, bodyText)
		fmt.Println("[Gemini ERROR]", errMsg)
		return "", 0, 0, fmt.Errorf("%s", errMsg)
	}

	// Parse response
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
	fmt.Printf("[DEBUG] Gemini raw response: %s\n", string(bodyBytes))

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", 0, 0, fmt.Errorf("no candidates in Gemini response")
	}

	// Extract text and thoughts
	// Gemini 3: thought can be a boolean (true) or string
	// When thought is true, the text field contains thinking content
	// When thought is false/absent, the text field contains the actual response
	var responseText strings.Builder
	var thinking strings.Builder

	for _, part := range geminiResp.Candidates[0].Content.Parts {
		// Check if this part is a thought
		// Gemini 3: thought can be boolean true or a string
		// When thought is true, the text field contains thinking content
		isThought := false
		if thoughtVal, hasThought := part["thought"]; hasThought {
			if thoughtBool, ok := thoughtVal.(bool); ok && thoughtBool {
				isThought = true
			} else if thoughtStr, ok := thoughtVal.(string); ok && thoughtStr != "" {
				// Legacy: if thought is a non-empty string, treat as thought
				isThought = true
			}
		}

		// Extract text field
		if textVal, hasText := part["text"]; hasText {
			if textStr, ok := textVal.(string); ok && textStr != "" {
				if isThought {
					// In Gemini 3, when thought is true, text contains the thinking
					thinking.WriteString(textStr)
				} else {
					// Otherwise, text contains the actual response
					responseText.WriteString(textStr)
				}
			}
		}
	}

	// Prepend thinking wrapped in <think> tags if present
	fullResponse := responseText.String()
	if thinking.Len() > 0 {
		fullResponse = "<think>" + thinking.String() + "</think>" + fullResponse
	}

	inputTokens := geminiResp.UsageMetadata.PromptTokenCount
	outputTokens := geminiResp.UsageMetadata.CandidatesTokenCount

	fmt.Printf("[DEBUG] Gemini response: %d input tokens, %d output tokens\n", inputTokens, outputTokens)

	return fullResponse, inputTokens, outputTokens, nil
}

// getGeminiMagicEditTool returns the tool definition for magic_edit in Gemini format
func getGeminiMagicEditTool() map[string]interface{} {
	return map[string]interface{}{
		"function_declarations": []map[string]interface{}{
			{
				"name":        "magic_edit",
				"description": "Trigger a visual code edit on the currently loaded project. Use this when the user asks you to make visual or code changes to their project (like changing colors, adding elements, modifying layouts, etc.).",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"edit_description": map[string]interface{}{
							"type":        "string",
							"description": "A clear, specific description of what changes to make to the code/design. Be detailed about colors, positions, sizes, etc.",
						},
					},
					"required": []string{"edit_description"},
				},
			},
		},
	}
}

// callGeminiChatStreaming streams Gemini response via SSE to the writer (for chat)
func callGeminiChatStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, apiKey string, attachmentBase64 string, attachmentMime string, geminiModel string) error {
	return callGeminiChatStreamingWithTools(w, prevMsgs, newMsg, apiKey, attachmentBase64, attachmentMime, nil, geminiModel)
}

// callGeminiChatStreamingWithTools streams Gemini response with optional tools
func callGeminiChatStreamingWithTools(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, apiKey string, attachmentBase64 string, attachmentMime string, tools []map[string]interface{}, geminiModel string) error {
	if apiKey == "" {
		return fmt.Errorf("no Gemini API key provided")
	}

	// Gather system message and convert to Gemini format
	systemMsg := defaultSystemPrompt
	var contents []map[string]interface{}

	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}

	// Add system message as first user message
	hasUserMsg := false
	for _, m := range prevMsgs {
		if m.Role == "user" || m.Role == "assistant" {
			hasUserMsg = true
			break
		}
	}

	if hasUserMsg {
		contents = append(contents, map[string]interface{}{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"text": systemMsg},
			},
		})
		contents = append(contents, map[string]interface{}{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"text": "Understood. I'll follow these instructions."},
			},
		})
	}

	// Add conversation history
	for _, m := range prevMsgs {
		if m.Role == "user" {
			contents = append(contents, map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": m.Content},
				},
			})
		} else if m.Role == "assistant" {
			contents = append(contents, map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"text": m.Content},
				},
			})
		}
	}

	// Add new user message
	parts := []interface{}{
		map[string]interface{}{"text": newMsg},
	}
	if strings.TrimSpace(attachmentBase64) != "" && strings.TrimSpace(attachmentMime) != "" {
		parts = append(parts, map[string]interface{}{
			"inline_data": map[string]interface{}{
				"mime_type": attachmentMime,
				"data":      attachmentBase64,
			},
		})
	}
	contents = append(contents, map[string]interface{}{
		"role":  "user",
		"parts": parts,
	})

	// Call streaming Gemini API with optional tools
	return callGeminiAPIStreamingWithTools(w, contents, apiKey, 12000, "", tools, geminiModel)
}

// callGeminiAPIStreaming streams Gemini responses via SSE directly to the HTTP response writer
func callGeminiAPIStreaming(w http.ResponseWriter, contents []map[string]interface{}, apiKey string, maxTokens int, thinkingBudget string, geminiModel string) error {
	return callGeminiAPIStreamingWithTools(w, contents, apiKey, maxTokens, thinkingBudget, nil, geminiModel)
}

// callGeminiAPIStreamingWithTools streams Gemini responses with optional tools support
func callGeminiAPIStreamingWithTools(w http.ResponseWriter, contents []map[string]interface{}, apiKey string, maxTokens int, thinkingBudget string, tools []map[string]interface{}, geminiModel string) error {
	// Build generation config with thinking config
	thinkingCfg := map[string]interface{}{
		"thinkingBudget":  -1,   // -1 means dynamic budget (default)
		"includeThoughts": true, // Include thought summaries in response
	}

	// Set thinking budget if provided
	if thinkingBudget != "" {
		thinkingCfg["thinkingBudget"] = thinkingBudget
	}

	generationConfig := map[string]interface{}{
		"maxOutputTokens": maxTokens,
		"temperature":     1.0,
		"thinkingConfig":  thinkingCfg,
	}

	reqBody := map[string]interface{}{
		"contents":         contents,
		"generationConfig": generationConfig,
	}

	// Add tools if provided
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	jsonBytes, _ := json.Marshal(reqBody)
	resolvedModel := normalizeGeminiModelID(geminiModel)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", resolvedModel, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		fmt.Printf("[Gemini ERROR] Creating request: %v\n", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("[DEBUG] Calling Gemini API with streaming (SSE) model=%s...\n", resolvedModel)
	// Request body not printed (contains large base64 images)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[Gemini ERROR] HTTP request failed: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("[DEBUG] Gemini API response status: %d\n", resp.StatusCode)

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		if isGeminiModelNotFound(resp.StatusCode, bodyText) && resolvedModel != "gemini-3.1-pro-preview" {
			fmt.Printf("[Gemini WARN] Model %s not available for streaming, retrying with gemini-3.1-pro-preview\n", resolvedModel)
			return callGeminiAPIStreamingWithTools(w, contents, apiKey, maxTokens, thinkingBudget, tools, "gemini-3.1-pro-preview")
		}
		errMsg := fmt.Sprintf("Gemini API error (%d): %s", resp.StatusCode, bodyText)
		fmt.Println("[Gemini ERROR]", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Stream response chunks to client
	var responseBuilder strings.Builder
	var thinkingBuilder strings.Builder
	var thinkingSent bool
	var thinkingStart time.Time
	var thinkingDurationSent bool
	var inputTokens, outputTokens int

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				dataStr := strings.TrimSpace(line[5:])

				// Skip empty data lines
				if dataStr == "" {
					continue
				}

				var chunk struct {
					Candidates []struct {
						Content struct {
							Parts []map[string]interface{} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
					UsageMetadata struct {
						PromptTokenCount     int `json:"promptTokenCount"`
						CandidatesTokenCount int `json:"candidatesTokenCount"`
					} `json:"usageMetadata"`
				}

				if e := json.Unmarshal([]byte(dataStr), &chunk); e == nil {
					// Debug: print the raw chunk to see structure
					fmt.Printf("[Gemini DEBUG] Raw chunk: %s\n", dataStr)

					if len(chunk.Candidates) > 0 {
						for _, part := range chunk.Candidates[0].Content.Parts {
							// Check if this part is a thought (has "thought" key and it's true/string)
							if thoughtVal, hasThought := part["thought"]; hasThought {
								// Could be a boolean flag or a string content
								var thoughtText string

								// If thought is true (boolean), use text field
								if thoughtBool, ok := thoughtVal.(bool); ok && thoughtBool {
									if textVal, hasText := part["text"]; hasText {
										thoughtText = textVal.(string)
									}
								} else if thoughtStr, ok := thoughtVal.(string); ok {
									// If thought is a string, use it directly
									thoughtText = thoughtStr
								}

								if thoughtText != "" {
									if thinkingStart.IsZero() {
										thinkingStart = time.Now()
									}
									fmt.Printf("[Gemini DEBUG] Found thought: %s\n", thoughtText)
									cleanThought := strings.Trim(thoughtText, "\n")
									firstLineSource := strings.TrimSpace(cleanThought)
									if firstLineSource != "" {
										// Preserve the full thought (including newlines) for the final <think> block
										if thinkingBuilder.Len() > 0 {
											thinkingBuilder.WriteString("\n\n")
										}
										thinkingBuilder.WriteString(cleanThought)

										// Send only the first line (e.g., "Framing the Response") as the visible thinking step
										firstLine := firstLineSource
										if idx := strings.Index(firstLine, "\n"); idx != -1 {
											firstLine = firstLine[:idx]
										}
										firstLine = strings.TrimSpace(firstLine)
										firstLine = strings.Trim(firstLine, "* ")
										if firstLine != "" {
											thinkingStepData := map[string]string{"thinkingStep": firstLine}
											thinkingStepBytes, _ := json.Marshal(thinkingStepData)
											fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
											fmt.Println("[Gemini] Sending thinking step:", firstLine)
											if f, ok := w.(http.Flusher); ok {
												f.Flush()
											}
										}
									}
								}
							}

							// Handle function call (tool use)
							if funcCall, hasFuncCall := part["functionCall"]; hasFuncCall {
								if fcMap, ok := funcCall.(map[string]interface{}); ok {
									funcName, _ := fcMap["name"].(string)
									funcArgs, _ := fcMap["args"].(map[string]interface{})

									fmt.Printf("[Gemini] Function call detected: %s, args: %v\n", funcName, funcArgs)

									// Convert args to JSON string for consistency with Claude
									argsJSON, _ := json.Marshal(funcArgs)

									// Send tool_use event to client (same format as Claude)
									toolUseData := map[string]interface{}{
										"tool_use": map[string]interface{}{
											"id":    fmt.Sprintf("gemini-%d", time.Now().UnixNano()), // Generate unique ID
											"name":  funcName,
											"input": string(argsJSON),
										},
									}
									toolUseBytes, _ := json.Marshal(toolUseData)
									fmt.Fprintf(w, "data: %s\n\n", toolUseBytes)
									if f, ok := w.(http.Flusher); ok {
										f.Flush()
									}
								}
							}

							// Handle text content (only if not a thought)
							if textVal, hasText := part["text"]; hasText {
								// Skip if this is a thought part (thought is true boolean or non-empty string)
								isThoughtPart := false
								if thoughtVal, hasThought := part["thought"]; hasThought {
									if thoughtBool, ok := thoughtVal.(bool); ok && thoughtBool {
										isThoughtPart = true
									} else if thoughtStr, ok := thoughtVal.(string); ok && thoughtStr != "" {
										isThoughtPart = true
									}
								}
								if isThoughtPart {
									continue
								}
								content := textVal.(string)
								responseBuilder.WriteString(content)

								// Send accumulated thinking before first content chunk
								if !thinkingSent && thinkingBuilder.Len() > 0 {
									thinkingText := "<think>" + thinkingBuilder.String() + "</think>"
									chunkData := map[string]string{"chunk": thinkingText}
									chunkBytes, _ := json.Marshal(chunkData)
									fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
									fmt.Println("[Gemini] Sending thinking:", thinkingBuilder.String())
									if f, ok := w.(http.Flusher); ok {
										f.Flush()
									}
									thinkingSent = true
								}

								// Send content chunk
								chunkData := map[string]string{"chunk": content}
								chunkBytes, _ := json.Marshal(chunkData)
								fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
								fmt.Println("[Gemini] Sending chunk:", content)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}

								// Send thinking duration after first content
								if !thinkingDurationSent && !thinkingStart.IsZero() {
									duration := time.Since(thinkingStart).Seconds()
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
									thinkingDurationSent = true
								}
							}
						}
					}

					// Extract usage metadata (just store it, don't send done yet)
					if chunk.UsageMetadata.PromptTokenCount > 0 {
						inputTokens = chunk.UsageMetadata.PromptTokenCount
						outputTokens = chunk.UsageMetadata.CandidatesTokenCount
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

	// Send thinking duration if not sent yet
	if !thinkingDurationSent && !thinkingStart.IsZero() {
		duration := time.Since(thinkingStart).Seconds()
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
	}

	// Calculate cost (Gemini 3.1 Pro: $1.25 per 1M input, $5 per 1M output)
	cost := (float64(inputTokens) / 1_000_000.0 * 1.25) + (float64(outputTokens) / 1_000_000.0 * 5.0)

	// Send done event with usage after all chunks are sent
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

	fmt.Printf("[Gemini] Stream complete: %d input tokens, %d output tokens\n", inputTokens, outputTokens)
	return nil
}
