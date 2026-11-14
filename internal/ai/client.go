package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tui-workflow/internal/config"
	"tui-workflow/internal/logger"
)

const (
	MaxDiffLength  = 30000
	DefaultTimeout = 45 * time.Second
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type ChatCompletionChoice struct {
	Index   int         `json:"index"`
	Message ChatMessage `json:"message"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices"`
	Error   *APIError              `json:"error,omitempty"`
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
}

type Client struct {
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// BuildEndpoint constructs the full /chat/completions URL from provider URL.
func BuildEndpoint(providerURL string) string {
	clean := strings.TrimRight(strings.TrimSpace(providerURL), "/")
	if clean == "" {
		clean = "https://api.openai.com/v1"
	}
	if strings.HasSuffix(clean, "/chat/completions") {
		return clean
	}
	return clean + "/chat/completions"
}

// CleanCommitMessage cleans raw model output code blocks, quotes, or trailing notes.
func CleanCommitMessage(raw string) string {
	res := strings.TrimSpace(raw)

	// Remove markdown code fences
	if strings.HasPrefix(res, "```") {
		lines := strings.Split(res, "\n")
		var filtered []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				continue
			}
			filtered = append(filtered, line)
		}
		res = strings.TrimSpace(strings.Join(filtered, "\n"))
	}

	// Strip surrounding double/single quotes if single-line
	if len(res) >= 2 {
		if (strings.HasPrefix(res, "\"") && strings.HasSuffix(res, "\"")) ||
			(strings.HasPrefix(res, "'") && strings.HasSuffix(res, "'")) ||
			(strings.HasPrefix(res, "`") && strings.HasSuffix(res, "`")) {
			res = strings.TrimSpace(res[1 : len(res)-1])
		}
	}
	return res
}

func parseSSEStream(body []byte) (string, error) {
	lines := strings.Split(string(body), "\n")
	var sb strings.Builder
	foundData := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		foundData = true
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk sseChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					sb.WriteString(choice.Delta.Content)
				} else if choice.Message.Content != "" {
					sb.WriteString(choice.Message.Content)
				} else if choice.Text != "" {
					sb.WriteString(choice.Text)
				}
			}
		}
	}

	if !foundData {
		return "", fmt.Errorf("not an SSE stream format")
	}
	result := sb.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("SSE stream did not produce text content")
	}
	return result, nil
}

func maskKey(k string) string {
	if len(k) <= 6 {
		return "***"
	}
	return k[:3] + "..." + k[len(k)-3:]
}

func (c *Client) GenerateCommitMessage(ctx context.Context, cfg config.AIConfig, diffText string) (string, error) {
	trimmedDiff := strings.TrimSpace(diffText)
	if trimmedDiff == "" {
		return "", fmt.Errorf("empty diff, no staged changes to analyze")
	}

	if len(trimmedDiff) > MaxDiffLength {
		trimmedDiff = trimmedDiff[:MaxDiffLength] + "\n\n... [diff dipotong karena melebihi batas ukuran]"
	}

	model := strings.TrimSpace(cfg.DefaultModel)
	if model == "" {
		model = "gpt-4o-mini"
	}

	endpoint := BuildEndpoint(cfg.ProviderURL)
	reqPayload := ChatCompletionRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are an expert developer. Generate a concise, conventional git commit message (format: <type>: <summary>, e.g., 'feat: add ai commit message generator') based on the provided git diff. Write ONLY the commit message text. Do NOT wrap in markdown fences, backticks, or quotes. Do NOT add extra explanation or pleasantries.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Git Diff:\n%s", trimmedDiff),
			},
		},
		Temperature: 0.2,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode request JSON: %w", err)
	}

	logger.Debugf("AI Request: POST %s | Model: %s | APIKey: %s", endpoint, model, maskKey(cfg.APIKey))
	logger.Debugf("AI Request Payload: %s", string(bodyBytes))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		logger.Debugf("AI Connection Error: %v", err)
		return "", fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debugf("AI Read Error: %v", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Debugf("AI Response: HTTP %d | Headers: %v", resp.StatusCode, resp.Header)
	logger.Debugf("AI Response Body (%d bytes): %s", len(respBody), string(respBody))

	trimmedBody := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		bodySnippet := trimmedBody
		if len(bodySnippet) > 120 {
			bodySnippet = bodySnippet[:120] + "..."
		}
		return "", fmt.Errorf("API HTTP %d: %s", resp.StatusCode, bodySnippet)
	}

	// 1. Check if response is SSE stream (starts with data:)
	if strings.HasPrefix(trimmedBody, "data:") {
		logger.Debugf("AI Response is SSE format, parsing chunks...")
		sseMsg, sseErr := parseSSEStream(respBody)
		if sseErr == nil && strings.TrimSpace(sseMsg) != "" {
			cleaned := CleanCommitMessage(sseMsg)
			logger.Debugf("AI Commit parsed from SSE successfully: %q", cleaned)
			return cleaned, nil
		}
		logger.Debugf("AI SSE parse error: %v", sseErr)
	}

	// 2. Standard JSON Response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		bodySnippet := trimmedBody
		if len(bodySnippet) > 120 {
			bodySnippet = bodySnippet[:120] + "..."
		}
		logger.Debugf("AI JSON Unmarshal Error: %v | Raw snippet: %s", err, bodySnippet)
		return "", fmt.Errorf("failed to decode response JSON: %w (response: %q)", err, bodySnippet)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("API returned no choices for commit message")
	}

	msgContent := chatResp.Choices[0].Message.Content
	cleaned := CleanCommitMessage(msgContent)
	logger.Debugf("AI Generated Commit message: %q", cleaned)
	return cleaned, nil
}
