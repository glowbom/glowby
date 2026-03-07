package main

// callGeminiApiGoWithAttachment builds a Gemini chat request that includes inline_data attachment.
func callGeminiApiGoWithAttachment(prevMsgs []ChatMessage, newMsg, attachmentBase64, mimeType, apiKey, geminiModel string) (*R1Response, error) {
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

	// Add new user message with inline attachment
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []interface{}{
			map[string]interface{}{"text": newMsg},
			map[string]interface{}{
				"inline_data": map[string]interface{}{
					"mime_type": mimeType,
					"data":      attachmentBase64,
				},
			},
		},
	})

	// Call Gemini API
	resolvedModel := normalizeGeminiModelID(geminiModel)
	respText, inputTokens, outputTokens, err := callGeminiAPIWithModel(contents, apiKey, 12000, "", resolvedModel)
	if err != nil {
		return nil, err
	}

	// Calculate cost
	cost := (float64(inputTokens) / 1_000_000.0 * 1.25) + (float64(outputTokens) / 1_000_000.0 * 5.0)

	return &R1Response{
		AIResponse: respText,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
		},
		Cost: cost,
	}, nil
}
