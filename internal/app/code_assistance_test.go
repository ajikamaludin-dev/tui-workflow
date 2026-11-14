package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/snippet"
)

func TestCodeAssistanceIntegration(t *testing.T) {
	tempDir := t.TempDir()
	m, _ := NewModelWithPath(tempDir, false)
	m.Init()
	m.Width = 120
	m.Height = 40

	// Open a mock JSX file
	testFile := filepath.Join(tempDir, "TestComp.jsx")
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	m.openFile(testFile)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("expected active buffer for %s", testFile)
	}

	// 1. Test Indent Rainbow toggle via Command Palette / Menu
	tab.Buffer.IndentRainbow = false
	cmd := m.handleMenuAction("edit.indent_rainbow")
	if cmd != nil {
		t.Errorf("expected nil cmd for indent rainbow toggle")
	}
	if !tab.Buffer.IndentRainbow {
		t.Errorf("expected IndentRainbow to be true after toggle")
	}

	// 2. Test Snippet Expansion: type 'rafce' then press Tab
	tab.Buffer.SetText("")
	tab.Buffer.InsertString("rafce")
	if word, _ := tab.Buffer.WordBeforeCursor(); word != "rafce" {
		t.Fatalf("expected word before cursor to be 'rafce', got '%s'", word)
	}

	// Verify catalog has snippet
	if s := snippet.FindSnippet("rafce", tab.Path); s == nil {
		t.Fatalf("expected 'rafce' snippet to exist in catalog")
	}

	// Simulate pressing Tab
	resModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = resModel.(Model)

	tab = m.TabManager.ActiveTab()
	bufText := tab.Buffer.Text()
	if m.SnippetSession == nil || !m.SnippetSession.Active {
		t.Errorf("expected SnippetSession to be active after expanding 'rafce'")
	}
	if len(bufText) == 0 || bufText == "rafce" {
		t.Errorf("expected buffer text to contain expanded React component, got: %s", bufText)
	}

	// Cancel snippet session with Esc
	resModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = resModel.(Model)
	if m.SnippetSession != nil && m.SnippetSession.Active {
		t.Errorf("expected SnippetSession to be inactive after Esc")
	}

	// 3. Test Linter Diagnostic Update and Toast Integration
	m.LinterStore = linter.NewStore()
	m.LinterStore.Set(tab.Path, []linter.Diagnostic{
		{
			FilePath: tab.Path,
			Line:     1,
			Col:      1,
			Message:  "React hook rules violation",
			Severity: linter.SeverityError,
			Source:   "eslint",
			RuleID:   "react-hooks/rules-of-hooks",
		},
	})

	// Dispatch LintFinishedMsg
	resModel, _ = m.Update(linter.LintFinishedMsg{
		FilePath: tab.Path,
		Diagnostics: []linter.Diagnostic{
			{
				FilePath: tab.Path,
				Line:     1,
				Col:      1,
				Message:  "React hook rules violation",
				Severity: linter.SeverityError,
				Source:   "eslint",
				RuleID:   "react-hooks/rules-of-hooks",
			},
		},
	})
	m = resModel.(Model)
	tab = m.TabManager.ActiveTab()

	// Check if buffer has diagnostic on line 0 (1-based line 1)
	if marker, ok := tab.Buffer.LineDiagnostics[0]; !ok || !strings.Contains(marker, "●") {
		t.Errorf("expected error marker '●' on line 0, got %q (ok: %v)", marker, ok)
	}

	// 4. Test LSP Completion Popup trigger and insertion
	m.CompletionPopup = lsp.NewCompletionPopup()
	m.CompletionPopup.SetItems([]lsp.CompletionItem{
		{
			Label:      "useEffect",
			Kind:       lsp.CompletionKindFunction,
			Detail:     "(effect: EffectCallback, deps?: DependencyList) => void",
			InsertText: "useEffect",
			AdditionalTextEdits: []lsp.TextEdit{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
					NewText: "import { useEffect } from 'react';\n",
				},
			},
		},
	}, 10, 10, "")

	if !m.CompletionPopup.Active {
		t.Errorf("expected completion popup to be active")
	}

	// Press Enter to confirm completion
	resModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = resModel.(Model)
	tab = m.TabManager.ActiveTab()

	if m.CompletionPopup.Active {
		t.Errorf("expected completion popup to close after Enter")
	}

	// Verify top import was added/merged
	firstLine := string(tab.Buffer.Lines[0])
	if firstLine != "import React, { useEffect } from 'react';" && firstLine != "import { useEffect } from 'react';" {
		t.Errorf("expected auto-import on line 0, got %q", firstLine)
	}
}
