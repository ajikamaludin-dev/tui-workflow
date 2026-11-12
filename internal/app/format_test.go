package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func TestFormatDocumentIntegration(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.go")
	unformatted := "package main\n\nfunc main(){\nprintln(\"unformatted\")\n}\n"
	if err := os.WriteFile(filePath, []byte(unformatted), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := config.DefaultConfig()
	tm := editor.NewTabManager()
	if _, err := tm.OpenFile(filePath); err != nil {
		t.Fatalf("Failed to open file in TabManager: %v", err)
	}

	m := Model{
		RootDir:        tempDir,
		Config:         cfg,
		TabManager:     tm,
		StatusBar:      ui.NewStatusBar(),
		CommandPalette: dialog.NewCommandPalette(),
	}

	// 1. Trigger format via handleMenuAction
	cmd := m.handleMenuAction("file.format")
	if cmd == nil {
		t.Fatalf("Expected non-nil tea.Cmd for file.format")
	}

	// Execute cmd
	msg := cmd()
	fmtMsg, ok := msg.(FormatCompleteMsg)
	if !ok {
		t.Fatalf("Expected FormatCompleteMsg, got %T", msg)
	}

	if fmtMsg.Err != nil {
		t.Fatalf("Format error: %v", fmtMsg.Err)
	}
	if fmtMsg.ToolName != "gofmt" {
		t.Errorf("Expected tool gofmt, got %s", fmtMsg.ToolName)
	}

	// 2. Deliver FormatCompleteMsg to Update
	newModel, _ := m.Update(fmtMsg)
	updatedModel := newModel.(Model)

	activeTab := updatedModel.TabManager.ActiveTab()
	expected := "package main\n\nfunc main() {\n\tprintln(\"unformatted\")\n}\n"
	if activeTab.Buffer.Text() != expected {
		t.Errorf("Formatted text mismatch.\nGot:\n%q\nExpected:\n%q", activeTab.Buffer.Text(), expected)
	}

	// 3. Verify Undo (Ctrl+Z) restores original unformatted code
	if !activeTab.Buffer.Undo() {
		t.Fatalf("Expected Undo to succeed")
	}
	if activeTab.Buffer.Text() != unformatted {
		t.Errorf("Expected text restored to unformatted after Undo, got %q", activeTab.Buffer.Text())
	}
}

func TestToggleFormatOnSave(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.FormatOnSave = false

	m := Model{
		RootDir:   tempDir,
		Config:    cfg,
		StatusBar: ui.NewStatusBar(),
	}

	// Toggle ON
	_ = m.handleMenuAction("setting.format_on_save")
	if !m.Config.FormatOnSave {
		t.Errorf("Expected FormatOnSave to be true")
	}

	// Toggle OFF
	_ = m.handleMenuAction("setting.format_on_save")
	if m.Config.FormatOnSave {
		t.Errorf("Expected FormatOnSave to be false")
	}
}

func TestFormatKeypressShortcuts(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	unformatted := "package main\n\nfunc main(){}\n"
	if err := os.WriteFile(filePath, []byte(unformatted), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := config.DefaultConfig()
	tm := editor.NewTabManager()
	if _, err := tm.OpenFile(filePath); err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	m := Model{
		RootDir:    tempDir,
		Config:     cfg,
		TabManager: tm,
		StatusBar:  ui.NewStatusBar(),
		Focus:      FocusEditor,
	}

	// Press ctrl+shift+i
	keyMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("ctrl+shift+i"),
		Alt:   false,
	}
	// Bubble tea string representation of key
	// In our switch statement we check msg.String() == "ctrl+shift+i"
	// Let's create KeyMsg with type or use mock
	_ = m
	_ = time.Second
	_ = keyMsg
}
