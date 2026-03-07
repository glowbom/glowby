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

// callGPT5ResponsesAPIStreaming uses the Responses API for GPT-5 with reasoning support
func callGPT5ResponsesAPIStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg, apiKey, openAIModel string) error {
	if apiKey == "" {
		return fmt.Errorf("no OpenAI API key provided")
	}
	resolvedModel := normalizeOpenAIModelID(openAIModel)

	// Build conversation history for Responses API
	// Responses API uses "input" instead of "messages"

	// Start with default system prompt, override if one exists in prevMsgs
	systemMsg := defaultSystemPrompt
	for _, m := range prevMsgs {
		if m.Role == "system" {
			systemMsg = m.Content
			break
		}
	}

	var conversationText strings.Builder

	// Add system prompt at the beginning
	conversationText.WriteString(fmt.Sprintf("System: %s\n\n", systemMsg))

	// Add conversation history (skip system messages as we already added it)
	for _, m := range prevMsgs {
		if m.Role == "user" {
			conversationText.WriteString(fmt.Sprintf("User: %s\n\n", m.Content))
		} else if m.Role == "assistant" {
			conversationText.WriteString(fmt.Sprintf("Assistant: %s\n\n", m.Content))
		}
	}
	conversationText.WriteString(fmt.Sprintf("User: %s\n\nAssistant:", newMsg))

	// Build Responses API request
	reqBody := map[string]interface{}{
		"model":             resolvedModel,
		"input":             conversationText.String(),
		"max_output_tokens": 10000,
		"reasoning": map[string]interface{}{
			"effort":  "medium", // "minimal", "medium", or "high"
			"summary": "auto",   // Request reasoning summaries
		},
		"stream": true,
	}

	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	fmt.Printf("[DEBUG] Calling %s Responses API with streaming...\n", resolvedModel)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		bodyText := string(b)
		if isMissingResponsesWriteScope(bodyText) {
			fmt.Println("[GPT-5] Responses scope missing, falling back to Chat Completions streaming")
			return callGPT5ChatStreaming(w, prevMsgs, newMsg, apiKey, resolvedModel)
		}
		if isMissingModelRequestScope(bodyText) {
			return fmt.Errorf("%s", openAIModelScopeHelpMessage())
		}
		return fmt.Errorf("OpenAI Responses API error (%d): %s", resp.StatusCode, bodyText)
	}

	// Stream response chunks to client
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

				// Responses API streaming format - events have delta/text at top level
				var chunk struct {
					Type           string `json:"type"`
					SequenceNumber int    `json:"sequence_number"`
					SummaryIndex   int    `json:"summary_index"`

					// For delta events (reasoning and output text)
					Delta string `json:"delta"`
					Text  string `json:"text"`

					// For completed event with usage
					Response struct {
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
					// Handle reasoning delta events (accumulate ALL summary sections)
					if chunk.Type == "response.reasoning_summary_text.delta" {
						if reasoningStart.IsZero() {
							reasoningStart = time.Now()
						}
						if chunk.Delta != "" {
							// When we move to a new summary section
							if chunk.SummaryIndex != lastSummaryIndex {
								// Add spacing between different summary sections in the full reasoning
								if lastSummaryIndex != -1 {
									reasoningBuilder.WriteString("\n\n")
								}

								// Reset for the new thinking step
								currentThinkingStep.Reset()
								currentStepTitleSent = false
								lastSummaryIndex = chunk.SummaryIndex
							}

							// Accumulate the reasoning text
							reasoningBuilder.WriteString(chunk.Delta)
							currentThinkingStep.WriteString(chunk.Delta)

							// Try to extract and send the title once we have a complete title
							if !currentStepTitleSent {
								text := currentThinkingStep.String()
								// Check if we have a complete title:
								// Either: both opening and closing ** (complete markdown title)
								// Or: text followed by at least 2 newlines (title + blank line before body)
								hasCompleteMarkdown := strings.HasPrefix(text, "**") && strings.Count(text, "**") >= 2
								hasNewlineSeparation := strings.Count(text, "\n") >= 2

								if hasCompleteMarkdown || hasNewlineSeparation {
									stepTitle := extractThinkingStepTitle(text)
									if stepTitle != "" {
										thinkingStepData := map[string]string{"thinkingStep": stepTitle}
										thinkingStepBytes, _ := json.Marshal(thinkingStepData)
										fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
										fmt.Printf("[GPT-5] Sending thinking step: %s\n", stepTitle)
										if f, ok := w.(http.Flusher); ok {
											f.Flush()
										}
										currentStepTitleSent = true
									}
								}
							}
						}
					}

					// Handle output text delta events (actual response content)
					if chunk.Type == "response.output_text.delta" {
						if chunk.Delta != "" {
							// Send complete reasoning before first content chunk
							if !reasoningSent && reasoningBuilder.Len() > 0 {
								// Send the last thinking step if we haven't sent its title yet
								if !currentStepTitleSent && currentThinkingStep.Len() > 0 {
									stepTitle := extractThinkingStepTitle(currentThinkingStep.String())
									if stepTitle != "" {
										thinkingStepData := map[string]string{"thinkingStep": stepTitle}
										thinkingStepBytes, _ := json.Marshal(thinkingStepData)
										fmt.Fprintf(w, "data: %s\n\n", thinkingStepBytes)
										fmt.Printf("[GPT-5] Sending final thinking step: %s\n", stepTitle)
										if f, ok := w.(http.Flusher); ok {
											f.Flush()
										}
										currentStepTitleSent = true
									}
								}

								thinkingText := "<think>" + reasoningBuilder.String() + "</think>"
								chunkData := map[string]string{"chunk": thinkingText}
								chunkBytes, _ := json.Marshal(chunkData)
								fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
								fmt.Println("[GPT-5] Sent complete reasoning chain to client")
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
								reasoningSent = true

								// Send thinking duration
								duration := time.Since(reasoningStart).Seconds()
								duration = math.Round(duration*10) / 10
								metaData := map[string]any{
									"meta": map[string]any{
										"thinkingSeconds": duration,
									},
								}
								metaBytes, _ := json.Marshal(metaData)
								fmt.Fprintf(w, "data: %s\n\n", metaBytes)
								fmt.Printf("[GPT-5] Sent thinking duration: %.1fs\n", duration)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
								reasoningDurationSent = true
							}

							responseBuilder.WriteString(chunk.Delta)

							// Send content chunk to client
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
							fmt.Printf("[GPT-5] Sent thinking duration: %.1fs\n", duration)
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

	fmt.Printf("[GPT-5] Stream complete: %d input tokens, %d output tokens (%d reasoning)\n", inputTokens, outputTokens, reasoningTokens)
	return nil
}

// extractThinkingStepTitle extracts the title from a thinking step
// Looks for text between ** markers (e.g., "**Title**") or up to the first newline
func extractThinkingStepTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// Look for text between ** markers
	if strings.HasPrefix(text, "**") {
		// Find the closing **
		rest := text[2:]
		if endIdx := strings.Index(rest, "**"); endIdx != -1 {
			return strings.TrimSpace(rest[:endIdx])
		}
	}

	// Fallback: take everything up to the first newline
	if newlineIdx := strings.Index(text, "\n"); newlineIdx != -1 {
		return strings.TrimSpace(text[:newlineIdx])
	}

	// If no newline and no ** markers, return the whole text (shouldn't happen in practice)
	return text
}
