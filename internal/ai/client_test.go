package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tui-workflow/internal/config"
)

func TestBuildEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/chat/completions"},
		{"https://openrouter.ai/api/v1/chat/completions", "https://openrouter.ai/api/v1/chat/completions"},
		{"http://localhost:11434/v1", "http://localhost:11434/v1/chat/completions"},
	}

	for _, tc := range tests {
		got := BuildEndpoint(tc.input)
		if got != tc.expected {
			t.Errorf("BuildEndpoint(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feat: add ai client", "feat: add ai client"},
		{"\"feat: add ai client\"", "feat: add ai client"},
		{"'fix: resolve race condition'", "fix: resolve race condition"},
		{"`refactor: clean models`", "refactor: clean models"},
		{"```\nfeat: multiline commit\n\nwith body\n```", "feat: multiline commit\n\nwith body"},
		{"```git\nchore: bump version\n```", "chore: bump version"},
	}

	for _, tc := range tests {
		got := CleanCommitMessage(tc.input)
		if got != tc.expected {
			t.Errorf("CleanCommitMessage(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGenerateCommitMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", auth)
		}

		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode error: %v", err)
		}

		if req.Model != "gpt-4o-mini" {
			t.Errorf("expected model gpt-4o-mini, got %s", req.Model)
		}
		if req.Stream != false {
			t.Errorf("expected stream false, got %v", req.Stream)
		}

		resp := ChatCompletionResponse{
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "feat(git): add commit generator via ai",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	cfg := config.AIConfig{
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		DefaultModel: "gpt-4o-mini",
	}

	msg, err := client.GenerateCommitMessage(context.Background(), cfg, "diff --git a/test b/test\n+new line")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "feat(git): add commit generator via ai" {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestGenerateCommitMessage_SSEFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"feat: sse \"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"commit chunk\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	cfg := config.AIConfig{
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		DefaultModel: "gpt-4o-mini",
	}

	msg, err := client.GenerateCommitMessage(context.Background(), cfg, "+some change")
	if err != nil {
		t.Fatalf("unexpected error parsing SSE: %v", err)
	}
	if msg != "feat: sse commit chunk" {
		t.Errorf("unexpected SSE message: %s", msg)
	}
}

func TestGenerateCommitMessage_InvalidJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("default backend - 404"))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	cfg := config.AIConfig{
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		DefaultModel: "gpt-4o-mini",
	}

	_, err := client.GenerateCommitMessage(context.Background(), cfg, "+some change")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "respon: \"default backend - 404\"") {
		t.Errorf("expected error to include raw body snippet, got: %v", err)
	}
}

func TestGenerateCommitMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := ChatCompletionResponse{
			Error: &APIError{
				Message: "Incorrect API key provided",
				Type:    "invalid_request_error",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	cfg := config.AIConfig{
		ProviderURL:  server.URL,
		APIKey:       "bad-key",
		DefaultModel: "gpt-4o-mini",
	}

	_, err := client.GenerateCommitMessage(context.Background(), cfg, "+some change")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Incorrect API key provided") {
		t.Errorf("expected API key error message, got %v", err)
	}
}

func TestGenerateCommitMessage_EmptyDiff(t *testing.T) {
	client := NewClient()
	cfg := config.AIConfig{}
	_, err := client.GenerateCommitMessage(context.Background(), cfg, "   ")
	if err == nil {
		t.Fatal("expected error for empty diff, got nil")
	}
}
