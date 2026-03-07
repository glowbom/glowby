package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type openAIResponsesRequest struct {
	Model        string                   `json:"model"`
	Instructions string                   `json:"instructions,omitempty"`
	Input        []openAIInputMessage     `json:"input"`
	Tools        []map[string]interface{} `json:"tools,omitempty"`
}

type openAIInputMessage struct {
	Role    string                   `json:"role"`
	Content []openAIInputContentItem `json:"content"`
}

type openAIInputContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIResponsesOutput struct {
	Output []struct {
		Type    string          `json:"type"`
		Status  string          `json:"status"`
		Content []openAIContent `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIContent struct {
	Type        string             `json:"type"`
	Text        string             `json:"text"`
	Annotations []openAIAnnotation `json:"annotations,omitempty"`
}

type openAIAnnotation struct {
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
}

type chatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []chatCompletionMessage `json:"messages"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callWebSearchMini(prevMsgs []ChatMessage, userQuery, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no OpenAI API key provided")
	}
	trimmed := strings.TrimSpace(userQuery)
	if trimmed == "" {
		return "", fmt.Errorf("empty search query")
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Question: " + trimmed + "\n\n")
	promptBuilder.WriteString("Provide up-to-date bullet points with inline citations like [1] and finish with a Sources section listing [n] Title — URL.")

	reqPayload := chatCompletionRequest{
		Model: "gpt-4o-mini-search-preview",
		Messages: []chatCompletionMessage{
			{
				Role:    "system",
				Content: "You are a research assistant with built-in web search. Always cite sources and end with a Sources list.",
			},
			{
				Role:    "user",
				Content: promptBuilder.String(),
			},
		},
	}

	jsonBytes, _ := json.Marshal(reqPayload)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := openAIHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mini search error (%d): %s", resp.StatusCode, string(body))
	}

	var respPayload chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", err
	}

	if len(respPayload.Choices) == 0 {
		return "", fmt.Errorf("mini search returned no choices")
	}

	content := strings.TrimSpace(respPayload.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("mini search returned empty content")
	}

	return content, nil
}

func shouldRetrySearch(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded") {
		return true
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "503") || strings.Contains(msg, "500") {
		return true
	}
	return false
}

var openAIHTTPClient = &http.Client{
	Transport: &http.Transport{
		ResponseHeaderTimeout: 120 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          128,
	},
	Timeout: 0,
}

func callGPT5Search(prevMsgs []ChatMessage, userQuery, apiKey string) (*R1Response, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenAI API key provided")
	}
	trimmed := strings.TrimSpace(userQuery)
	if trimmed == "" {
		return nil, fmt.Errorf("empty search query")
	}

	systemPrompt := defaultSystemPrompt + `

You can perform live web searches. When you use external information, cite sources inline with [n] and list a "Sources" section containing the URLs.`

	var historyBuilder strings.Builder
	maxHistory := 4
	var historySnippets []string
	for i := len(prevMsgs) - 1; i >= 0 && maxHistory > 0; i-- {
		m := prevMsgs[i]
		var snippet string
		switch m.Role {
		case "user":
			snippet = "User: " + m.Content + "\n\n"
		case "assistant":
			snippet = "Assistant: " + m.Content + "\n\n"
		default:
			continue
		}
		historySnippets = append(historySnippets, snippet)
		maxHistory--
	}
	for i := len(historySnippets) - 1; i >= 0; i-- {
		historyBuilder.WriteString(historySnippets[i])
	}

	var promptBuilder strings.Builder
	if historyBuilder.Len() > 0 {
		promptBuilder.WriteString("Recent conversation:\n")
		promptBuilder.WriteString(historyBuilder.String())
		promptBuilder.WriteString("\n")
	}
	promptBuilder.WriteString("User question: " + trimmed + "\n\n")
	promptBuilder.WriteString("Please answer with the latest information available. If no relevant information can be found, say so.")

	reqPayload := openAIResponsesRequest{
		Model:        defaultOpenAIModelID,
		Instructions: systemPrompt,
		Input: []openAIInputMessage{
			{
				Role: "user",
				Content: []openAIInputContentItem{
					{
						Type: "input_text",
						Text: promptBuilder.String(),
					},
				},
			},
		},
		Tools: []map[string]interface{}{
			{
				"type": "web_search",
			},
		},
	}

	jsonBytes, _ := json.Marshal(reqPayload)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/responses", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	req.Header.Set("OpenAI-Beta", "web-search=v1")
	resp, err := openAIHTTPClient.Do(req)
	if err != nil {
		fmt.Printf("[SEARCH] OpenAI responses request failed for query %q: %v\n", truncateForLog(trimmed), err)
		return nil, fmt.Errorf("openai responses request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		bodyText := string(body)
		fmt.Printf("[SEARCH] OpenAI responses returned status %d for query %q. Body: %s\n", resp.StatusCode, truncateForLog(trimmed), bodyText)
		if isMissingResponsesWriteScope(bodyText) {
			fmt.Printf("[SEARCH] Missing responses scope; attempting mini search fallback for query %q\n", truncateForLog(trimmed))
			if miniContext, miniErr := callWebSearchMini(prevMsgs, trimmed, apiKey); miniErr == nil && strings.TrimSpace(miniContext) != "" {
				return &R1Response{
					AIResponse: miniContext,
					TokenUsage: nil,
					Cost:       0,
				}, nil
			}
			return nil, fmt.Errorf("openai credential is missing required scope api.responses.write for GPT-5 web search")
		}
		return nil, fmt.Errorf("openai responses error (%d): %s", resp.StatusCode, bodyText)
	}

	var respPayload openAIResponsesOutput
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		fmt.Printf("[SEARCH] OpenAI responses decode failed for query %q: %v\n", truncateForLog(trimmed), err)
		return nil, fmt.Errorf("openai responses decode failed: %w", err)
	}

	var builder strings.Builder
	type sourceInfo struct {
		Title string
		URL   string
	}
	var sources []sourceInfo
	seenSources := make(map[string]bool)

	for _, item := range respPayload.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type != "output_text" {
				continue
			}
			builder.WriteString(content.Text)
			if !strings.HasSuffix(content.Text, "\n") {
				builder.WriteString("\n")
			}
			for _, ann := range content.Annotations {
				if ann.Type == "url_citation" && ann.URL != "" {
					if !seenSources[ann.URL] {
						seenSources[ann.URL] = true
						title := ann.Title
						if strings.TrimSpace(title) == "" {
							title = ann.URL
						}
						sources = append(sources, sourceInfo{Title: title, URL: ann.URL})
					}
				}
			}
		}
	}

	answer := builder.String()
	if strings.TrimSpace(answer) == "" {
		answer = "I could not retrieve any useful information from the web search."
	} else if len(sources) > 0 {
		answer = strings.TrimSpace(answer) + "\n\nSources:\n"
		for i, src := range sources {
			answer += fmt.Sprintf("[%d] %s — %s\n", i+1, src.Title, src.URL)
		}
	}

	inputTokens := respPayload.Usage.InputTokens
	outputTokens := respPayload.Usage.OutputTokens

	cost := estimateOpenAITextCost(defaultOpenAIModelID, inputTokens, outputTokens)

	return &R1Response{
		AIResponse: answer,
		TokenUsage: map[string]int{
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
		},
		Cost: cost,
	}, nil
}

func performWebSearch(prevMsgs []ChatMessage, userQuery, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no OpenAI API key provided")
	}
	trimmed := strings.TrimSpace(userQuery)
	if trimmed == "" {
		return "", fmt.Errorf("empty search query")
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		context, err := callWebSearchMini(prevMsgs, trimmed, apiKey)
		if err == nil && strings.TrimSpace(context) != "" {
			fmt.Printf("[SEARCH] performWebSearch succeeded via mini for query %q (attempt %d)\n", truncateForLog(trimmed), attempt)
			return context, nil
		}
		if err != nil {
			fmt.Printf("[SEARCH] mini search attempt %d failed for %q: %v\n", attempt, truncateForLog(trimmed), err)
			lastErr = err
			if !shouldRetrySearch(err) {
				break
			}
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
		}
	}

	resp, err := callGPT5Search(prevMsgs, trimmed, apiKey)
	if err != nil {
		fmt.Printf("[SEARCH] performWebSearch fallback GPT-5 failed for query %q: %v\n", truncateForLog(trimmed), err)
		if lastErr != nil {
			return "", lastErr
		}
		return "", err
	}
	fmt.Printf("[SEARCH] performWebSearch fallback GPT-5 succeeded for query %q\n", truncateForLog(trimmed))
	return resp.AIResponse, nil
}

func truncateForLog(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 80 {
		return trimmed
	}
	return trimmed[:80] + "…"
}

func isMissingResponsesWriteScope(message string) bool {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "api.responses.write") {
		return true
	}
	return strings.Contains(lower, "missing scopes") && strings.Contains(lower, "responses")
}

func isMissingModelRequestScope(message string) bool {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "model.request") {
		return true
	}
	return strings.Contains(lower, "missing scopes") && strings.Contains(lower, "model")
}

func isOpenAIMissingModelScope(message string) bool {
	return isMissingResponsesWriteScope(message) || isMissingModelRequestScope(message)
}

func openAIModelScopeHelpMessage() string {
	return "Imported ChatGPT/Codex login does not grant OpenAI API model scopes (`model.request`, `api.responses.write`) needed for GPT-5 API calls. Add an OpenAI API key in API Keys or switch to a local/non-OpenAI model."
}

func injectSearchContext(prevMsgs *[]ChatMessage, searchContext string) {
	searchContext = strings.TrimSpace(searchContext)
	if searchContext == "" {
		return
	}
	formatted := "Web search context:\n" + searchContext
	*prevMsgs = append(*prevMsgs, ChatMessage{
		Role:    "system",
		Content: formatted,
	})
}
