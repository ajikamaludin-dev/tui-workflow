package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestE2EStartupAndResize(t *testing.T) {
	m := NewModel()

	// Initial unready state
	view := m.View()
	if !strings.Contains(view, "Memuat tui-workflow") {
		t.Errorf("Expected loading message on unready model, got: %s", view)
	}

	// 1. Send normal terminal size 100x30
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	if !m.Ready || m.Width != 100 || m.Height != 30 {
		t.Fatalf("Model not ready after WindowSizeMsg")
	}

	view = m.View()
	if !strings.Contains(view, "File") || !strings.Contains(view, "Setting") {
		t.Errorf("View missing menu bar: %s", view)
	}

	// 2. Send small terminal size 70x20 -> should show warning
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	m = newM.(Model)

	view = m.View()
	if !strings.Contains(view, "Ukuran Terminal Terlalu Kecil") {
		t.Errorf("Expected too small warning on 70x20 terminal")
	}
}

func TestE2ENavigationAndShortcuts(t *testing.T) {
	m := NewModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	// Toggle Sidebar via Ctrl+B
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = newM.(Model)
	if m.SidebarVisible {
		t.Errorf("Sidebar should be hidden after Ctrl+B")
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = newM.(Model)
	if !m.SidebarVisible {
		t.Errorf("Sidebar should be visible after second Ctrl+B")
	}

	// Toggle Focus via Ctrl+E
	initFocus := m.Focus
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = newM.(Model)
	if m.Focus == initFocus {
		t.Errorf("Focus should toggle after Ctrl+E")
	}

	// Switch Sidebar to Git via Alt+2
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	m = newM.(Model)
	if m.SidebarView != SidebarGit {
		t.Errorf("SidebarView should be SidebarGit after Alt+2")
	}

	// Switch Sidebar to Files via Alt+1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	m = newM.(Model)
	if m.SidebarView != SidebarFiles {
		t.Errorf("SidebarView should be SidebarFiles after Alt+1")
	}

	// Help modal via '?'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(Model)
	if !m.ShowHelp {
		t.Errorf("Help modal should be active after '?'")
	}

	// Close help modal via Esc
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)
	if m.ShowHelp {
		t.Errorf("Help modal should close after Esc")
	}
}

func TestE2EEditorWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := filepath.Join(tempDir, "sample.md")
	_ = os.WriteFile(mdPath, []byte("# Test Header\n\nInitial content."), 0644)

	m := NewModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	// Open file in editor
	m.openFile(mdPath)
	if m.TabManager.Count() == 0 {
		t.Fatalf("Expected open tab")
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Path != mdPath {
		t.Fatalf("Expected active tab to be sample.md")
	}

	// Make sure focus is on Editor
	m.Focus = FocusEditor

	// Toggle Markdown Preview via Alt+M / Ctrl+M
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true})
	m = newM.(Model)
	if !m.TabManager.ActiveTab().MarkdownPreview {
		t.Errorf("Markdown preview should be active after Alt+M")
	}

	// Toggle back to raw editor via Alt+M / Ctrl+M
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true})
	m = newM.(Model)
	if m.TabManager.ActiveTab().MarkdownPreview {
		t.Errorf("Markdown preview should be inactive after second Alt+M")
	}

	// Type text into buffer: " Extra"
	for _, r := range " Extra" {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(Model)
	}

	// Save via Ctrl+S
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(Model)

	// Verify file content on disk
	diskContent, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(diskContent), "Extra") {
		t.Errorf("Expected disk content to contain 'Extra', got: %s", string(diskContent))
	}
}

func TestE2EModalConfirmations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-modal-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	p := filepath.Join(tempDir, "unsaved.txt")
	_ = os.WriteFile(p, []byte("line 1"), 0644)

	m := NewModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	m.openFile(p)

	// Turn off Auto-Save
	m.AutoSave = false
	m.Focus = FocusEditor

	// Make tab dirty
	m.TabManager.ActiveTab().Buffer.InsertRune('X')

	// Attempt close tab via Ctrl+W
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = newM.(Model)

	// Unsaved changes confirm dialog should be active!
	if !m.ConfirmDialog.Active {
		t.Errorf("Expected ConfirmDialog to be active when closing dirty tab without autosave")
	}

	// Cancel dialog via Esc
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)

	if m.ConfirmDialog.Active {
		t.Errorf("ConfirmDialog should be inactive after Esc")
	}
	if m.TabManager.Count() == 0 {
		t.Errorf("Tab should not be closed after cancelling dialog")
	}
}
