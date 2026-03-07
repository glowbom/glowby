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

func normalizeOllamaModelName(modelName string) string {
	trimmed := strings.TrimSpace(modelName)
	normalized := strings.ToLower(trimmed)

	switch normalized {
	case "qwen3.5", "qwen3.5:latest", "qwen-3.5", "qwen-3.5:latest":
		return "qwen3.5"
	default:
		return trimmed
	}
}

func ollamaModelSupportsVision(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(normalizeOllamaModelName(modelName)))
	return strings.HasPrefix(normalized, "qwen3.5")
}

func ollamaModelWantsExplicitThinkingInstructions(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(normalizeOllamaModelName(modelName)))
	return strings.HasPrefix(normalized, "gpt-oss") ||
		strings.HasPrefix(normalized, "qwen3-coder") ||
		strings.HasPrefix(normalized, "qwen3.5")
}

// Some Ollama models may place the final answer inside the thinking field.
// If a "done thinking" marker is present with trailing text, split that out.
func splitThinkingAndAnswer(thinkingText string) (thinking string, answer string) {
	trimmed := strings.TrimSpace(thinkingText)
	if trimmed == "" {
		return "", ""
	}

	normalized := strings.ToLower(trimmed)
	marker := "done thinking"
	markerIndex := strings.LastIndex(normalized, marker)
	if markerIndex < 0 {
		return trimmed, ""
	}

	answerStart := markerIndex + len(marker)
	if answerStart >= len(trimmed) {
		return trimmed, ""
	}

	rawAnswer := strings.TrimSpace(trimmed[answerStart:])
	rawAnswer = strings.TrimLeft(rawAnswer, ".:;- \t\r\n")
	rawAnswer = strings.TrimSpace(rawAnswer)
	if rawAnswer == "" {
		return trimmed, ""
	}

	trimmedThinking := strings.TrimSpace(trimmed[:answerStart])
	return trimmedThinking, rawAnswer
}

func callR1DrawToCodeApiFull(
	imageBase64, userPrompt, systemPrompt, template, imageSource string,
) (*R1Response, error) {
	// 1) get desc
	desc, err := getImageDescription(imageBase64)
	if err != nil {
		return nil, fmt.Errorf("failed describing image: %v", err)
	}

	detailedTaskDescription := buildDetailedTaskDescription(template, imageSource, userPrompt)

	// 3) Combine lines, similar to TS
	lines := []string{
		systemPrompt + " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework.",
		fmt.Sprintf("<｜User｜>Image description: %s", desc),
		fmt.Sprintf("<｜User｜>Detailed Task Description: %s", detailedTaskDescription),
		fmt.Sprintf("<｜User｜>Human: %s", userPrompt),
	}
	finalPrompt := strings.Join(lines, "\n") + "\n<｜Assistant｜>"

	fmt.Println("finalPrompt: ", finalPrompt)

	// 4) call R1
	respData, err := callR1Model(finalPrompt, "deepseek-r1:32b", 4096)
	if err != nil {
		return nil, err
	}
	return respData, nil
}

func callR1ApiGo(prevMsgs []ChatMessage, newMsg string) (*R1Response, error) {
	systemMessage := defaultSystemPrompt
	var userLines []string

	for _, pm := range prevMsgs {
		if pm.Role == "system" {
			systemMessage = pm.Content
		} else if pm.Role == "assistant" {
			userLines = append(userLines, "\n<｜Assistant｜>"+pm.Content)
		} else {
			userLines = append(userLines, "\n<｜User｜>"+pm.Content)
		}
	}
	userLines = append(userLines, "\n<｜User｜>"+newMsg)
	prompt := systemMessage + "\n" + strings.Join(userLines, "") + "\n<｜Assistant｜>"

	fmt.Println("prompt: ", prompt)

	respData, err := callR1Model(prompt, "deepseek-r1:7b", 500)
	if err != nil {
		return nil, err
	}

	fmt.Println("respData: ", respData)
	return respData, nil
}

// Generic local Ollama chat using a specified model (e.g., gpt-oss:20b)
func callOllamaApiGeneric(prevMsgs []ChatMessage, newMsg string, modelName string) (*R1Response, error) {
	systemMessage := defaultSystemPrompt
	// Encourage models that don't emit thoughts by default (e.g., gpt-oss)
	// to include a short, clearly delimited thinking block we can parse.
	if ollamaModelWantsExplicitThinkingInstructions(modelName) {
		systemMessage += "\nBefore answering, include a brief thinking section to yourself. Start it with the exact line 'Thinking...' and end it with the exact line '...done thinking.' Then write the final answer after that. Keep the thinking short."
	}
	var userLines []string

	for _, pm := range prevMsgs {
		if pm.Role == "system" {
			systemMessage = pm.Content
		} else if pm.Role == "assistant" {
			userLines = append(userLines, "\n<｜Assistant｜>"+pm.Content)
		} else {
			userLines = append(userLines, "\n<｜User｜>"+pm.Content)
		}
	}
	userLines = append(userLines, "\n<｜User｜>"+newMsg)
	prompt := systemMessage + "\n" + strings.Join(userLines, "") + "\n<｜Assistant｜>"

	respData, err := callR1Model(prompt, modelName, 500)
	if err != nil {
		return nil, err
	}
	return respData, nil
}

// Generic local draw-to-code using a specified model (e.g., gpt-oss:20b)
func callOllamaDrawToCodeApiFullWithModel(
	imageBase64, userPrompt, template, imageSource, modelName string,
) (*R1Response, error) {
	// 1) Describe image using local vision model
	desc, err := getImageDescription(imageBase64)
	if err != nil {
		return nil, fmt.Errorf("failed describing image: %v", err)
	}

	systemPrompt := getSystemPrompt(template, imageSource)
	if ollamaModelWantsExplicitThinkingInstructions(modelName) {
		systemPrompt += "\nBefore answering, include a brief thinking section to yourself. Start it with the exact line 'Thinking...' and end it with the exact line '...done thinking.' Then write the final answer (single HTML file) after that. Keep the thinking short."
	}
	detailedTaskDescription := buildDetailedTaskDescription(template, imageSource, userPrompt)

	lines := []string{
		systemPrompt + " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework.",
		fmt.Sprintf("<｜User｜>Image description: %s", desc),
		fmt.Sprintf("<｜User｜>Detailed Task Description: %s", detailedTaskDescription),
		fmt.Sprintf("<｜User｜>Human: %s", userPrompt),
	}
	finalPrompt := strings.Join(lines, "\n") + "\n<｜Assistant｜>"

	respData, err := callR1Model(finalPrompt, modelName, 4096)
	if err != nil {
		return nil, err
	}
	return respData, nil
}

func getImageDescription(imageBase64 string) (string, error) {
	/* // This section save image to disk (for testing purposes)
		// 1) Decode base64 and store the raw bytes
	    data, err := base64.StdEncoding.DecodeString(imageBase64)
	    if err != nil {
	        return "", fmt.Errorf("Error decoding base64: %v", err)
	    }

	    // 2) Create a unique filename, e.g. using time
	    filename := fmt.Sprintf("saved_images/%d.png", time.Now().UnixNano())

	    // 3) Write to disk
	    if err := os.WriteFile(filename, data, 0644); err != nil {
	        return "", fmt.Errorf("Error saving file: %v", err)
	    }*/

	payload := map[string]interface{}{
		"model": "llama3.2-vision",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Describe this image in human language. If something is in a foreign language, do not translate. Describe all elements thoroughly.",
				"images":  []string{imageBase64},
			},
		},
		"max_tokens": 500,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "http://localhost:11434/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("Vision API error: %s", string(b))
	}

	var desc strings.Builder
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}
			if e := json.Unmarshal([]byte(line), &chunk); e == nil {
				desc.WriteString(chunk.Message.Content)
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

func callR1Model(prompt, model string, maxTokens int) (*R1Response, error) {
	fmt.Printf("[OLLAMA] Calling generate model: %s\n", model)
	reqBody := map[string]interface{}{
		"model":       model,
		"prompt":      prompt,
		"max_tokens":  maxTokens,
		"temperature": 0.1,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	fmt.Println("req: ", reqBody)

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("R1 API error: %s", string(b))
	}

	var responseBuilder strings.Builder
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			var chunk struct {
				Response string `json:"response"`
			}
			if e := json.Unmarshal([]byte(line), &chunk); e == nil {
				responseBuilder.WriteString(chunk.Response)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	aiResp := responseBuilder.String()
	if aiResp == "" {
		return nil, fmt.Errorf("No valid response from R1 model")
	}

	inputLen := len(prompt)
	outputLen := len(aiResp)
	inputTokens := (inputLen + 3) / 4
	outputTokens := (outputLen + 3) / 4
	totalTokens := inputTokens + outputTokens
	cost := float64(totalTokens) / 1_000_000.0 * 9.5

	return &R1Response{
		AIResponse: aiResp,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  totalTokens,
		},
		Cost: cost,
	}, nil
}

// ====== Ollama Chat API helpers (messages streaming) ======
type ollamaChatMsg struct {
	Role     string   `json:"role"`
	Content  string   `json:"content"`
	Thinking string   `json:"thinking,omitempty"`
	Images   []string `json:"images,omitempty"`
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []ollamaChatMsg `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaChatChunk struct {
	Message ollamaChatMsg `json:"message"`
	Done    bool          `json:"done"`
}

func ollamaChatAttachmentImages(modelName, attachmentBase64, attachmentMime string) []string {
	trimmedAttachment := strings.TrimSpace(attachmentBase64)
	if trimmedAttachment == "" {
		return nil
	}
	if !ollamaModelSupportsVision(modelName) {
		return nil
	}
	trimmedMime := strings.ToLower(strings.TrimSpace(attachmentMime))
	if trimmedMime != "" && !strings.HasPrefix(trimmedMime, "image/") {
		return nil
	}
	return []string{trimmedAttachment}
}

func buildOllamaChatMessages(
	prevMsgs []ChatMessage,
	newMsg string,
	systemMessage string,
	modelName string,
	attachmentBase64 string,
	attachmentMime string,
) []ollamaChatMsg {
	var msgs []ollamaChatMsg
	if systemMessage != "" {
		msgs = append(msgs, ollamaChatMsg{Role: "system", Content: systemMessage})
	}
	for _, pm := range prevMsgs {
		role := pm.Role
		if role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, ollamaChatMsg{Role: role, Content: pm.Content})
	}
	lastUser := ollamaChatMsg{Role: "user", Content: newMsg}
	if images := ollamaChatAttachmentImages(modelName, attachmentBase64, attachmentMime); len(images) > 0 {
		lastUser.Images = images
	}
	msgs = append(msgs, lastUser)
	return msgs
}

// callOllamaChatWithModel uses /api/chat to encourage chain-of-thought from models like gpt-oss
func callOllamaChatWithModel(prevMsgs []ChatMessage, newMsg string, modelName, systemMessage string) (*R1Response, error) {
	return callOllamaChatWithModelAndAttachment(prevMsgs, newMsg, modelName, systemMessage, "", "")
}

func callOllamaChatWithModelAndAttachment(
	prevMsgs []ChatMessage,
	newMsg string,
	modelName string,
	systemMessage string,
	attachmentBase64 string,
	attachmentMime string,
) (*R1Response, error) {
	resolvedModel := normalizeOllamaModelName(modelName)
	fmt.Printf("[OLLAMA] Calling chat model: %s\n", resolvedModel)
	sys := systemMessage
	if sys == "" {
		sys = defaultSystemPrompt
	}
	// Models with native thinking support (gpt-oss, qwen3-coder, qwen3.5) will automatically use it.
	msgs := buildOllamaChatMessages(prevMsgs, newMsg, sys, resolvedModel, attachmentBase64, attachmentMime)

	reqBody := ollamaChatReq{Model: resolvedModel, Messages: msgs, Stream: true, Options: map[string]any{"temperature": 0.2}}
	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "http://localhost:11434/api/chat", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("Ollama chat error: %s", string(b))
	}
	var sb strings.Builder
	var thinking string
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var chunk ollamaChatChunk
			if e := json.Unmarshal(line, &chunk); e == nil {
				if chunk.Message.Thinking != "" {
					thinking = chunk.Message.Thinking
				}
				sb.WriteString(chunk.Message.Content)
				if chunk.Done {
					break
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	thinkingText := strings.TrimSpace(thinking)
	// Recover content when Ollama returned answer text only inside thinking.
	if strings.TrimSpace(sb.String()) == "" && thinkingText != "" {
		trimmedThinking, recoveredAnswer := splitThinkingAndAnswer(thinkingText)
		if recoveredAnswer != "" {
			sb.WriteString(recoveredAnswer)
			thinkingText = trimmedThinking
		}
	}
	// Prepend thinking wrapped in <think> tags if present
	text := sb.String()
	if thinkingText != "" {
		text = "<think>" + thinkingText + "</think>" + text
	}
	if text == "" {
		return nil, fmt.Errorf("empty chat response")
	}
	inputTokens := (len(newMsg) + 3) / 4
	outputTokens := (len(text) + 3) / 4
	return &R1Response{AIResponse: text, TokenUsage: map[string]int{"inputTokens": inputTokens, "outputTokens": outputTokens, "totalTokens": inputTokens + outputTokens}, Cost: 0}, nil
}

// callOllamaChatWithModelStreaming streams the response via SSE to the writer
func callOllamaChatWithModelStreaming(w http.ResponseWriter, prevMsgs []ChatMessage, newMsg string, modelName, systemMessage string) error {
	return callOllamaChatWithModelStreamingAndAttachment(w, prevMsgs, newMsg, modelName, systemMessage, "", "")
}

func callOllamaChatWithModelStreamingAndAttachment(
	w http.ResponseWriter,
	prevMsgs []ChatMessage,
	newMsg string,
	modelName string,
	systemMessage string,
	attachmentBase64 string,
	attachmentMime string,
) error {
	resolvedModel := normalizeOllamaModelName(modelName)
	fmt.Printf("[OLLAMA] Streaming chat model: %s\n", resolvedModel)
	sys := systemMessage
	if sys == "" {
		sys = defaultSystemPrompt
	}
	// Models with native thinking support (gpt-oss, qwen3-coder, qwen3.5) will automatically use it.
	msgs := buildOllamaChatMessages(prevMsgs, newMsg, sys, resolvedModel, attachmentBase64, attachmentMime)

	reqBody := ollamaChatReq{Model: resolvedModel, Messages: msgs, Stream: true, Options: map[string]any{"temperature": 0.2}}
	jsonBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "http://localhost:11434/api/chat", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("Ollama chat error: %s", string(b))
	}

	var sb strings.Builder
	var thinkingBuilder strings.Builder
	var thinkingSent bool
	var thinkingStart time.Time
	var thinkingDurationSent bool
	var contentSent bool
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var chunk ollamaChatChunk
			if e := json.Unmarshal(line, &chunk); e == nil {
				// Accumulate thinking content
				if chunk.Message.Thinking != "" {
					if thinkingStart.IsZero() {
						thinkingStart = time.Now()
					}
					thinkingBuilder.WriteString(chunk.Message.Thinking)
				}

				content := chunk.Message.Content
				sb.WriteString(content)

				// Send accumulated thinking before first content chunk.
				if !thinkingSent && thinkingBuilder.Len() > 0 && content != "" {
					thinkingText := "<think>" + thinkingBuilder.String() + "</think>"
					chunkData := map[string]string{"chunk": thinkingText}
					chunkBytes, _ := json.Marshal(chunkData)
					fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
					fmt.Println("Sending thinking:", thinkingBuilder.String())
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					thinkingSent = true
				}

				if content != "" {
					contentSent = true
					// Send SSE event
					chunkData := map[string]string{"chunk": content}
					chunkBytes, _ := json.Marshal(chunkData)
					fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
					fmt.Println("Sending chunk:", content)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}

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
				if chunk.Done {
					// Recover content when Ollama returned answer text only inside thinking.
					if !contentSent && thinkingBuilder.Len() > 0 {
						trimmedThinking, recoveredAnswer := splitThinkingAndAnswer(thinkingBuilder.String())
						if recoveredAnswer != "" {
							if !thinkingSent && strings.TrimSpace(trimmedThinking) != "" {
								thinkingText := "<think>" + trimmedThinking + "</think>"
								chunkData := map[string]string{"chunk": thinkingText}
								chunkBytes, _ := json.Marshal(chunkData)
								fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
								if f, ok := w.(http.Flusher); ok {
									f.Flush()
								}
								thinkingSent = true
							}
							chunkData := map[string]string{"chunk": recoveredAnswer}
							chunkBytes, _ := json.Marshal(chunkData)
							fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
							fmt.Printf("Sending recovered chunk from thinking (%d chars)\n", len(recoveredAnswer))
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
							sb.WriteString(recoveredAnswer)
							contentSent = true
						} else if !thinkingSent {
							thinkingText := "<think>" + thinkingBuilder.String() + "</think>"
							chunkData := map[string]string{"chunk": thinkingText}
							chunkBytes, _ := json.Marshal(chunkData)
							fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
							thinkingSent = true
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
						metaBytes, _ := json.Marshal(metaData)
						fmt.Fprintf(w, "data: %s\n\n", metaBytes)
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
						thinkingDurationSent = true
					}

					// Calculate usage
					inputTokens := (len(newMsg) + 3) / 4
					outputTokens := (len(sb.String()) + 3) / 4
					totalTokens := inputTokens + outputTokens
					doneData := map[string]interface{}{
						"done": true,
						"tokenUsage": map[string]int{
							"inputTokens":  inputTokens,
							"outputTokens": outputTokens,
							"totalTokens":  totalTokens,
						},
						"cost": 0.0,
					}
					doneBytes, _ := json.Marshal(doneData)
					fmt.Fprintf(w, "data: %s\n\n", doneBytes)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					break
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
	return nil
}

func callOllamaChatDrawToCodeWithModel(imageBase64, userPrompt, template, imageSource, modelName string) (*R1Response, error) {
	resolvedModel := normalizeOllamaModelName(modelName)
	systemPrompt := getSystemPrompt(template, imageSource) + " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."
	if ollamaModelWantsExplicitThinkingInstructions(resolvedModel) {
		systemPrompt += "\nBefore answering, include a brief thinking section to yourself. Start it with the exact line 'Thinking...' and end it with the exact line '...done thinking.' Then write the final answer (single HTML file) after that. Keep the thinking short."
	}
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	if ollamaModelSupportsVision(resolvedModel) {
		userContent := strings.Join([]string{
			fmt.Sprintf("Detailed Task Description: %s", detailedTask),
			fmt.Sprintf("Human: %s", userPrompt),
		}, "\n")
		return callOllamaChatWithModelAndAttachment(nil, userContent, resolvedModel, systemPrompt, imageBase64, "image/jpeg")
	}

	desc, err := getImageDescription(imageBase64)
	if err != nil {
		return nil, fmt.Errorf("failed describing image: %v", err)
	}
	userContent := strings.Join([]string{
		fmt.Sprintf("Image description: %s", desc),
		fmt.Sprintf("Detailed Task Description: %s", detailedTask),
		fmt.Sprintf("Human: %s", userPrompt),
	}, "\n")
	return callOllamaChatWithModel(nil, userContent, resolvedModel, systemPrompt)
}

// callOllamaDrawToCodeStreaming handles draw-to-code with streaming for Ollama models
func callOllamaDrawToCodeStreaming(w http.ResponseWriter, imageBase64, userPrompt, template, imageSource, modelName string) error {
	resolvedModel := normalizeOllamaModelName(modelName)

	// Build system prompt (reuse existing logic)
	systemPrompt := getSystemPrompt(template, imageSource) + " Replace @tailwind placeholders with the Tailwind CSS CDN link to load the real framework."
	if ollamaModelWantsExplicitThinkingInstructions(resolvedModel) {
		systemPrompt += "\nBefore answering, include a brief thinking section to yourself. Start it with the exact line 'Thinking...' and end it with the exact line '...done thinking.' Then write the final answer (single HTML file) after that. Keep the thinking short."
	}

	// Build detailed task
	detailedTask := buildDetailedTaskDescription(template, imageSource, userPrompt)

	if ollamaModelSupportsVision(resolvedModel) {
		userContent := strings.Join([]string{
			fmt.Sprintf("Detailed Task Description: %s", detailedTask),
			fmt.Sprintf("Human: %s", userPrompt),
		}, "\n")
		return callOllamaChatWithModelStreamingAndAttachment(w, nil, userContent, resolvedModel, systemPrompt, imageBase64, "image/jpeg")
	}

	// Get image description first (required for vision-less models)
	desc, err := getImageDescription(imageBase64)
	if err != nil {
		return fmt.Errorf("failed describing image: %v", err)
	}
	userContent := strings.Join([]string{
		fmt.Sprintf("Image description: %s", desc),
		fmt.Sprintf("Detailed Task Description: %s", detailedTask),
		fmt.Sprintf("Human: %s", userPrompt),
	}, "\n")

	// Call existing streaming function
	return callOllamaChatWithModelStreaming(w, nil, userContent, resolvedModel, systemPrompt)
}
