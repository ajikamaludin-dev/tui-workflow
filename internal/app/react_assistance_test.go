package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/lsp"
)

func TestReactAutoCloseTag(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	testFile := filepath.Join(tempDir, "Component.jsx")
	_ = os.WriteFile(testFile, []byte(""), 0644)
	m.openFile(testFile)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("no active tab")
	}

	// 1. User types "<div>"
	for _, r := range "<div>" {
		res, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = res.(Model)
	}

	tab = m.TabManager.ActiveTab()
	text := tab.Buffer.Text()
	if !strings.Contains(text, "<div></div>") {
		t.Errorf("Expected auto-closed '<div></div>', got %q", text)
	}

	// Cursor should be at col 5 (between <div> and </div>)
	if tab.Buffer.CursorCol != 5 {
		t.Errorf("Expected cursor at col 5, got %d", tab.Buffer.CursorCol)
	}

	// 2. User types "<Button className=\"btn\">"
	res, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)

	for _, r := range "<Button className=\"btn\">" {
		res, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = res.(Model)
	}

	tab = m.TabManager.ActiveTab()
	line1 := string(tab.Buffer.Lines[1])
	if !strings.Contains(line1, "<Button className=\"btn\"></Button>") {
		t.Errorf("Expected auto-closed Button tag, got %q", line1)
	}
}

func TestReactAutoImportOnCompletion(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	testFile := filepath.Join(tempDir, "App.jsx")
	_ = os.WriteFile(testFile, []byte("export const App = () => {\n  \n};\n"), 0644)
	m.openFile(testFile)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("no active tab")
	}

	// Place cursor inside function on line 1
	tab.Buffer.CursorRow = 1
	tab.Buffer.CursorCol = 2

	// Type "useSt"
	for _, r := range "useSt" {
		res, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = res.(Model)
	}

	// Trigger Ctrl+Space autocomplete
	res, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	m = res.(Model)

	if m.CompletionPopup == nil || !m.CompletionPopup.Active {
		t.Fatalf("Expected completion popup to be active")
	}

	// Selected item should be useState
	selected := m.CompletionPopup.Selected()
	if selected == nil || selected.Label != "useState" {
		t.Fatalf("Expected 'useState' to be selected in completion popup, got %+v", selected)
	}

	// Press Enter to accept completion
	res, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)

	tab = m.TabManager.ActiveTab()
	text := tab.Buffer.Text()

	// Verify that useState was inserted AND auto-import was added to top
	if !strings.Contains(text, "import { useState } from 'react';") {
		t.Errorf("Expected top import 'import { useState } from 'react';', got:\n%s", text)
	}

	if !strings.Contains(text, "useState") {
		t.Errorf("Expected 'useState' to be inserted in buffer, got:\n%s", text)
	}
}

func TestComponentAlreadyImportedNoDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	// User's exact scenario: Button already imported via multi-component barrel import
	initialContent := `import { Button, Checkbox, FileDropzone, Label, SelectObject, TextareaInput } from '@/components/index';

export const MyPage = () => {
  return <Btn
};
`
	testFile := filepath.Join(tempDir, "MyPage.jsx")
	_ = os.WriteFile(testFile, []byte(initialContent), 0644)
	m.openFile(testFile)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("no active tab")
	}

	// Place cursor at line 3, col 13 (after "<Btn")
	tab.Buffer.CursorRow = 3
	tab.Buffer.CursorCol = 13

	// Inject a completion popup item for Button that has AdditionalTextEdits to import from ui/button
	m.CompletionPopup.SetItems([]lsp.CompletionItem{
		{
			Label:      "Button",
			Kind:       lsp.CompletionKindClass,
			InsertText: "Button",
			AdditionalTextEdits: []lsp.TextEdit{
				{
					Range:   lsp.Range{Start: lsp.Position{Line: 0, Character: 0}},
					NewText: "import { Button } from '../../components/ui/button';\n",
				},
			},
		},
	}, 10, 10, "Btn")

	// Press Enter to accept completion
	res, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)

	tab = m.TabManager.ActiveTab()
	text := tab.Buffer.Text()

	// 1. Verify "Button" replaced "Btn"
	if !strings.Contains(text, "<Button") {
		t.Errorf("Expected '<Button' in buffer, got:\n%s", text)
	}

	// 2. CRITICAL: Verify '../../components/ui/button' was NOT inserted because Button is already imported!
	if strings.Contains(text, "../../components/ui/button") {
		t.Errorf("CRITICAL BUG: Duplicate import was added despite Button already being imported from '@/components/index'!\n%s", text)
	}

	// 3. Verify exactly 1 import statement with Button exists
	count := strings.Count(text, "Button")
	// Count should be: 1 in import statement, 1 in JSX <Button
	expectedButtonMentions := 2
	if count != expectedButtonMentions {
		t.Errorf("Expected %d mentions of Button (1 in import, 1 in JSX), got %d mentions in:\n%s",
			expectedButtonMentions, count, text)
	}
}
