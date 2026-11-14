package app

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"fmt"
	"path/filepath"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/snippet"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleCtrlSpace() (tea.Model, tea.Cmd) {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return m, nil
	}
	buf := tab.Buffer
	word, _ := buf.WordBeforeCursor()

	gutterWidth := buf.GutterWidth()
	sidebarW := 0
	if m.SidebarVisible {
		sidebarW = m.SidebarWidth
	}
	curX := sidebarW + gutterWidth + buf.CursorCol - buf.ScrollCol
	curY := 2 + (buf.CursorRow - buf.ScrollRow) + 1
	if curX < 0 {
		curX = 0
	}
	if curX > m.Width-40 {
		curX = m.Width - 40
	}
	if curY > m.Height-10 {
		curY = curY - 10
	}
	if curY < 2 {
		curY = 2
	}

	// 1. Build local completions immediately (<0.1ms)
	localItems := lsp.BuildLocalCompletions(buf, tab.Path, m.SymbolIndex)
	filtered := lsp.FilterCompletions(localItems, word)
	if len(filtered) > 0 && m.CompletionPopup != nil {
		m.CompletionPopup.SetItems(filtered, curX, curY, word)
	}

	// 2. Query LSP server asynchronously if available
	ext := filepath.Ext(tab.Path)
	if m.LSPManager != nil {
		client, langID, err := m.LSPManager.GetClient(ext)
		if client != nil && err == nil {
			row := buf.CursorRow
			col := buf.CursorCol
			path := tab.Path
			content := buf.Text()
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = client.DidOpen(path, langID, content)
				items, err := client.Completion(ctx, path, row, col)
				if err != nil || len(items) == 0 {
					return nil
				}
				return lspCompletionResultMsg{
					Items:  items,
					X:      curX,
					Y:      curY,
					Prefix: word,
				}
			}
		} else {
			serverBin, _, _ := lsp.DetectLanguageServer(m.RootDir, ext)
			if serverBin == "" {
				m.StatusBar.ShowToast("Local autocomplete & snippets active (LSP server not found)", false, 2*time.Second)
			}
		}
	}

	if len(filtered) == 0 {
		m.StatusBar.ShowToast("No autocomplete suggestions", false, 1*time.Second)
	}

	return m, nil
}

func (m Model) handleTab() (tea.Model, tea.Cmd) {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return m, nil
	}
	buf := tab.Buffer

	// 1. If completion popup is active, accept current selection
	if m.CompletionPopup != nil && m.CompletionPopup.Active {
		selected := m.CompletionPopup.Selected()
		m.CompletionPopup.Close()
		if selected != nil {
			_, startCol := buf.WordBeforeCursor()
			if selected.Kind == lsp.CompletionKindSnippet {
				if s := snippet.FindSnippet(selected.Label, tab.Path); s != nil {
					startRow := buf.CursorRow
					expanded := snippet.Expand(s, tab.Path)
					buf.ReplaceWordBeforeCursor(startCol, expanded.Text)
					if editor.ReactKnownExports[s.Prefix] {
						if editor.EnsureReactImport(buf, s.Prefix) {
							startRow++
						}
					}
					m.SnippetSession = snippet.NewSession(startRow, startCol, expanded.TabStops)
					if row, col, length, ok := m.SnippetSession.Current(); ok {
						buf.CursorRow = row
						buf.CursorCol = col
						buf.CollapseCursors()
						if length > 0 {
							buf.SelectAnchorRow = row
							buf.SelectAnchorCol = col
							buf.CursorCol = col + length
							buf.HasSelection = true
						} else {
							buf.ClearSelection()
						}
					}
					idx, total := m.SnippetSession.Progress()
					m.StatusBar.ShowToast(fmt.Sprintf("Snippet %s (%d/%d) | Tab/Shift+Tab | Esc: Selesai", s.Prefix, idx, total), false, 3*time.Second)
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
			}

			insertText := selected.InsertText
			if insertText == "" {
				insertText = selected.Label
			}
			buf.ReplaceWordBeforeCursor(startCol, insertText)

			// Smart Auto-Import
			if len(selected.AdditionalTextEdits) > 0 {
				if editor.ReactKnownExports[selected.Label] {
					if editor.EnsureReactImport(buf, selected.Label) {
						m.StatusBar.ShowToast("Auto-import '"+selected.Label+"' dari 'react' ditambahkan", false, 2*time.Second)
					}
				} else {
					for _, edit := range selected.AdditionalTextEdits {
						if editor.EnsureTextEditImport(buf, selected.Label, edit.NewText) {
							m.StatusBar.ShowToast("Auto-import '"+selected.Label+"' ditambahkan", false, 2*time.Second)
						}
					}
				}
			}
			m.syncStatusBarWithActiveTab()
			return m, nil
		}
	}

	// 2. If snippet session is active, advance to next tab-stop
	if m.SnippetSession != nil && m.SnippetSession.Active {
		if row, col, length, ok := m.SnippetSession.Next(); ok {
			buf.CursorRow = row
			buf.CursorCol = col
			buf.CollapseCursors()
			if length > 0 {
				buf.SelectAnchorRow = row
				buf.SelectAnchorCol = col
				buf.CursorCol = col + length
				buf.HasSelection = true
			} else {
				buf.ClearSelection()
			}
			idx, total := m.SnippetSession.Progress()
			m.StatusBar.ShowToast(fmt.Sprintf("Tab-stop %d/%d (Tab / Shift+Tab | Esc: Selesai)", idx, total), false, 2*time.Second)
			m.syncStatusBarWithActiveTab()
			return m, nil
		}
	}

	// 3. If buffer has selection, indent selection
	if buf.HasSelection {
		buf.IndentSelection()
		m.syncStatusBarWithActiveTab()
		return m, nil
	}

	// 4. Check if word before cursor is a snippet trigger
	word, startCol := buf.WordBeforeCursor()
	if word != "" {
		if s := snippet.FindSnippet(word, tab.Path); s != nil {
			startRow := buf.CursorRow
			expanded := snippet.Expand(s, tab.Path)
			buf.ReplaceWordBeforeCursor(startCol, expanded.Text)
			if editor.ReactKnownExports[s.Prefix] {
				if editor.EnsureReactImport(buf, s.Prefix) {
					startRow++
				}
			}
			m.SnippetSession = snippet.NewSession(startRow, startCol, expanded.TabStops)
			if row, col, length, ok := m.SnippetSession.Current(); ok {
				buf.CursorRow = row
				buf.CursorCol = col
				buf.CollapseCursors()
				if length > 0 {
					buf.SelectAnchorRow = row
					buf.SelectAnchorCol = col
					buf.CursorCol = col + length
					buf.HasSelection = true
				} else {
					buf.ClearSelection()
				}
			}
			idx, total := m.SnippetSession.Progress()
			m.StatusBar.ShowToast(fmt.Sprintf("Snippet %s (%d/%d) | Tab/Shift+Tab | Esc: Selesai", s.Prefix, idx, total), false, 3*time.Second)
			m.syncStatusBarWithActiveTab()
			return m, nil
		}
	}

	// 5. Default: insert 4 spaces
	buf.InsertString("    ")
	m.syncStatusBarWithActiveTab()
	return m, nil
}

func (m Model) handleShiftTab() (tea.Model, tea.Cmd) {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return m, nil
	}
	buf := tab.Buffer

	// 1. If snippet session is active, go back to previous tab-stop
	if m.SnippetSession != nil && m.SnippetSession.Active {
		if row, col, length, ok := m.SnippetSession.Prev(); ok {
			buf.CursorRow = row
			buf.CursorCol = col
			buf.CollapseCursors()
			if length > 0 {
				buf.SelectAnchorRow = row
				buf.SelectAnchorCol = col
				buf.CursorCol = col + length
				buf.HasSelection = true
			} else {
				buf.ClearSelection()
			}
			idx, total := m.SnippetSession.Progress()
			m.StatusBar.ShowToast(fmt.Sprintf("Tab-stop %d/%d (Tab / Shift+Tab | Esc: Selesai)", idx, total), false, 2*time.Second)
			m.syncStatusBarWithActiveTab()
			return m, nil
		}
	}

	// 2. If buffer has selection, outdent selection
	if buf.HasSelection {
		buf.OutdentSelection()
	} else {
		buf.OutdentCurrentLine()
	}
	m.syncStatusBarWithActiveTab()
	return m, nil
}

func (m Model) handleCtrlKPending(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// Ctrl+K chord (fold all/unfold all) - task requires Ctrl+K Ctrl+0 / Ctrl+K Ctrl+J
	if m.ctrlKPending {
		m.ctrlKPending = false
		switch ui.KeyName(msg) {
		case "ctrl+0", "0":
			if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				tab.Buffer.FoldAll()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Fold all", false, 1500*time.Millisecond)
			}
			return m, nil, true
		case "ctrl+j", "j", "ctrl+J":
			if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				tab.Buffer.UnfoldAll()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Unfold all", false, 1500*time.Millisecond)
			}
			return m, nil, true
		case "ctrl+k":
			m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)
			return m, nil, true
		case "esc":
			m.StatusBar.ShowToast("Ctrl+K chord dibatalkan", false, 1*time.Second)
			return m, nil, true
		default:
			m.StatusBar.ShowToast("Ctrl+K chord: use Ctrl+0/Ctrl+J (or Ctrl+K to open folder)", true, 2*time.Second)
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleEditorKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if m.Focus != FocusEditor {
		return m, nil, false
	}
	tab := m.TabManager.ActiveTab()
	if tab != nil && tab.IsDiff && tab.DiffView != nil {
		return m.handleDiffKeys(msg, tab)
	} else if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
		return m.handleMarkdownPreviewKeys(msg, tab)
	} else if tab != nil && tab.Buffer != nil {
		return m.handleBufferKeys(msg, tab)
	}
	return m, nil, false
}
