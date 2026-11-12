package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/ai"
	"tui-workflow/internal/git"
)

func TestAI_SettingsDialogWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	m, err := NewModelWithPath(tempDir)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// 1. Trigger Alt+I to open AI settings dialog
	resModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}, Alt: true})
	m = resModel.(Model)
	if cmd != nil {
		t.Errorf("expected nil cmd on alt+i, got %v", cmd)
	}
	if m.AISettingsDialog == nil || !m.AISettingsDialog.Active {
		t.Fatalf("expected AISettingsDialog to be active after Alt+I")
	}

	// 2. Modify provider URL and save via Ctrl+S
	m.AISettingsDialog.ProviderInput.SetValue("https://test.ai/v1")
	m.AISettingsDialog.APIKeyInput.SetValue("sk-mysecret")
	m.AISettingsDialog.DefaultModelInput.SetValue("gpt-4o")

	resModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = resModel.(Model)

	if m.AISettingsDialog.Active {
		t.Errorf("expected AISettingsDialog to close after Ctrl+S")
	}
	if m.Config.AI.ProviderURL != "https://test.ai/v1" || m.Config.AI.APIKey != "sk-mysecret" || m.Config.AI.DefaultModel != "gpt-4o" {
		t.Errorf("expected updated AI config, got: %+v", m.Config.AI)
	}
}

func TestAI_CommitGeneratorWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ai.ChatCompletionResponse{
			Choices: []ai.ChatCompletionChoice{
				{
					Index: 0,
					Message: ai.ChatMessage{
						Role:    "assistant",
						Content: "feat: add mock commit",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	m, err := NewModelWithPath(tempDir)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	m.Config.AI.ProviderURL = mockServer.URL
	m.Config.AI.APIKey = "dummy-key"
	m.Config.AI.DefaultModel = "gpt-4o-mini"
	m.AIClient = &ai.Client{HTTPClient: mockServer.Client()}

	// Test when GitRepo is nil (not a git repo)
	m.GitRepo = nil
	m.GitView = nil
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}, Alt: true})
	if m.StatusBar.ToastMsg != "Bukan repositori git" {
		t.Errorf("expected toast 'Bukan repositori git', got %q", m.StatusBar.ToastMsg)
	}

	// Test message handler for aiCommitGeneratedMsg
	gv := git.NewGitView(nil)
	m.GitView = gv
	m.GitView.IsLoading = true

	resModel, _ := m.Update(aiCommitGeneratedMsg{message: "feat: generated commit"})
	m = resModel.(Model)

	if m.GitView.IsLoading {
		t.Errorf("expected IsLoading to be false")
	}
	if m.GitView.CommitInput.Value() != "feat: generated commit" {
		t.Errorf("expected commit message 'feat: generated commit', got %q", m.GitView.CommitInput.Value())
	}

	// Test message handler for aiCommitErrorMsg
	m.GitView.IsLoading = true
	resModel, _ = m.Update(aiCommitErrorMsg{err: context.DeadlineExceeded})
	m = resModel.(Model)

	if m.GitView.IsLoading {
		t.Errorf("expected IsLoading to be false after error")
	}
}
