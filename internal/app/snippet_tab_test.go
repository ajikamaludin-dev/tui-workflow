package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSnippetTabAndShiftTabNavigation(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	testFile := filepath.Join(tempDir, "App.jsx")
	_ = os.WriteFile(testFile, []byte(""), 0644)
	m.openFile(testFile)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("no active tab")
	}

	// 1. User types 'rafce'
	for _, r := range "rafce" {
		res, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = res.(Model)
	}

	// 2. User presses Tab -> expands snippet (Tab-stop 1: ComponentName)
	res, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = res.(Model)

	tab = m.TabManager.ActiveTab()
	if m.SnippetSession == nil || !m.SnippetSession.Active {
		t.Fatalf("Snippet session should be active after expansion")
	}
	if tab.Buffer.CursorRow != 2 {
		t.Errorf("Expected Tab-stop 1 on row 2, got %d", tab.Buffer.CursorRow)
	}

	// 3. User presses Tab -> advances to Tab-stop 2: content
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = res.(Model)

	tab = m.TabManager.ActiveTab()
	if tab.Buffer.CursorRow != 5 {
		t.Errorf("Expected Tab-stop 2 on row 5, got %d", tab.Buffer.CursorRow)
	}

	// 4. User presses Shift+Tab -> goes back to Tab-stop 1: ComponentName
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = res.(Model)

	tab = m.TabManager.ActiveTab()
	if tab.Buffer.CursorRow != 2 {
		t.Errorf("Expected Shift+Tab to jump back to row 2, got %d", tab.Buffer.CursorRow)
	}

	// 5. User presses Esc -> exits snippet session cleanly
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = res.(Model)

	if m.SnippetSession != nil && m.SnippetSession.Active {
		t.Errorf("Snippet session should be inactive after Esc")
	}
}

func TestCtrlSpaceLocalAutocomplete(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	testFile := filepath.Join(tempDir, "Button.jsx")
	_ = os.WriteFile(testFile, []byte("const isCustomActive = true;\n"), 0644)
	m.openFile(testFile)

	// Move cursor to bottom
	res, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)

	// Trigger Ctrl+Space (both tea.KeyCtrlAt and "ctrl+space")
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	m = res.(Model)

	if m.CompletionPopup == nil || !m.CompletionPopup.Active {
		t.Fatalf("CompletionPopup should be active on Ctrl+Space fallback")
	}

	if len(m.CompletionPopup.Items) == 0 {
		t.Fatalf("CompletionPopup items should not be empty")
	}

	// Verify snippets like rafce and buffer words like isCustomActive are available
	hasRafce := false
	hasCustomWord := false
	for _, it := range m.CompletionPopup.Items {
		if it.Label == "rafce" {
			hasRafce = true
		}
		if it.Label == "isCustomActive" {
			hasCustomWord = true
		}
	}

	if !hasRafce {
		t.Errorf("Expected 'rafce' snippet in autocomplete popup")
	}
	if !hasCustomWord {
		t.Errorf("Expected 'isCustomActive' in autocomplete popup")
	}

	// Close popup with Esc
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = res.(Model)
	if m.CompletionPopup.Active {
		t.Errorf("CompletionPopup should close on Esc")
	}
}
