package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/formatter"
	"tui-workflow/internal/git"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/snippet"
	"tui-workflow/internal/terminal"
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

func (m *Model) handleGoToSymbolKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeGoToSymbol()
		return m, nil
	case "up":
		if m.GoToSymbolSelected > 0 {
			m.GoToSymbolSelected--
			if m.GoToSymbolSelected < m.GoToSymbolScroll {
				m.GoToSymbolScroll = m.GoToSymbolSelected
			}
		}
		return m, nil
	case "down":
		if m.GoToSymbolSelected < len(m.GoToSymbolResults)-1 {
			m.GoToSymbolSelected++
			maxRows := 12
			if m.GoToSymbolSelected >= m.GoToSymbolScroll+maxRows {
				m.GoToSymbolScroll = m.GoToSymbolSelected - maxRows + 1
			}
		}
		return m, nil
	case "enter":
		if len(m.GoToSymbolResults) > 0 && m.GoToSymbolSelected >= 0 && m.GoToSymbolSelected < len(m.GoToSymbolResults) {
			sel := m.GoToSymbolResults[m.GoToSymbolSelected]
			m.closeGoToSymbol()
			m.openFileAt(sel.FilePath, sel.Line, sel.Col)
			m.Focus = FocusEditor
			m.syncStatusBarWithActiveTab()
		} else {
			m.closeGoToSymbol()
		}
		return m, nil
	case "backspace":
		if len(m.GoToSymbolQuery) > 0 {
			r := []rune(m.GoToSymbolQuery)
			m.GoToSymbolQuery = string(r[:len(r)-1])
			m.updateGoToSymbolResults()
		}
		return m, nil
	}
	if len(msg.Runes) > 0 && !msg.Alt {
		m.GoToSymbolQuery += string(msg.Runes)
		m.updateGoToSymbolResults()
		return m, nil
	}
	if msg.String() == "space" {
		m.GoToSymbolQuery += " "
		m.updateGoToSymbolResults()
		return m, nil
	}
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+K chord (fold all/unfold all) - task requires Ctrl+K Ctrl+0 / Ctrl+K Ctrl+J
	if m.ctrlKPending {
		m.ctrlKPending = false
		switch msg.String() {
		case "ctrl+0", "0":
			if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				tab.Buffer.FoldAll()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Fold all", false, 1500*time.Millisecond)
			}
			return m, nil
		case "ctrl+j", "j", "ctrl+J":
			if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				tab.Buffer.UnfoldAll()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Unfold all", false, 1500*time.Millisecond)
			}
			return m, nil
		case "ctrl+k":
			m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)
			return m, nil
		case "esc":
			m.StatusBar.ShowToast("Ctrl+K chord dibatalkan", false, 1*time.Second)
			return m, nil
		default:
			m.StatusBar.ShowToast("Ctrl+K chord: use Ctrl+0/Ctrl+J (or Ctrl+K to open folder)", true, 2*time.Second)
			return m, nil
		}
	}
	// Go To Symbol modal handling (Ctrl+Shift+O)
	if m.GoToSymbolActive {
		return m.handleGoToSymbolKey(msg)
	}

	// 1. Context Menu Handling if active
	if m.ContextMenu != nil && m.ContextMenu.Active {
		switch msg.String() {
		case "esc":
			m.ContextMenu.Close()
			return m, nil
		case "up", "k":
			m.ContextMenu.Prev()
			return m, nil
		case "down", "j":
			m.ContextMenu.Next()
			return m, nil
		case "enter":
			item := m.ContextMenu.SelectedItem()
			m.ContextMenu.Close()
			if item != nil && !item.Disabled {
				cmd := m.handleContextMenuAction(item.ID)
				return m, cmd
			}
			return m, nil
		}
		return m, nil
	}

	// 0. Find Bar Handling if active
	if m.FindBar != nil && m.FindBar.Active {
		tab := m.TabManager.ActiveTab()
		action, closed, cmd := m.FindBar.Update(msg)
		if closed {
			if tab != nil && tab.Buffer != nil {
				tab.Buffer.ClearSearch()
			}
		} else if tab != nil && tab.Buffer != nil {
			switch action {
			case "change":
				tab.Buffer.SetSearchQuery(m.FindBar.Input.Value())
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "next":
				tab.Buffer.FindNext()
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "prev":
				tab.Buffer.FindPrev()
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "replace_one":
				if tab.Buffer.ReplaceCurrent(m.FindBar.ReplaceInput.Value()) {
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("1 match replaced", false, 1*time.Second)
				}
			case "replace_all":
				count := tab.Buffer.ReplaceAll(m.FindBar.Input.Value(), m.FindBar.ReplaceInput.Value())
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast(fmt.Sprintf("%d matches replaced", count), false, 2*time.Second)
			}
		}
		return m, cmd
	}

	// 0.5. Quick Open Handling if active
	if m.QuickOpen != nil && m.QuickOpen.Active {
		targetTabIdx, targetPath, closed, cmd := m.QuickOpen.Update(msg)
		if closed {
			if targetTabIdx >= 0 && targetTabIdx < m.TabManager.Count() {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.TabManager.ActiveIndex = targetTabIdx
				m.syncStatusBarWithActiveTab()
			} else if targetPath != "" {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.openFile(targetPath)
				m.Focus = FocusEditor
			}
		}
		return m, cmd
	}

	// 0.6. Command Palette Handling if active
	if m.CommandPalette != nil && m.CommandPalette.Active {
		actionID, closed, cmd := m.CommandPalette.Update(msg)
		if closed && actionID != "" {
			menuCmd := m.handleMenuAction(actionID)
			return m, tea.Batch(cmd, menuCmd)
		}
		return m, cmd
	}

	// 0.7. Global Search Handling if active
	if m.GlobalSearch != nil && m.GlobalSearch.Active {
		filePath, line, col, closed, cmd := m.GlobalSearch.Update(msg)
		if closed && filePath != "" {
			m.openFileAt(filePath, line, col)
			m.Focus = FocusEditor
		}
		return m, cmd
	}

	// 0.8. Branch Switcher Handling if active
	if m.BranchSwitcher != nil && m.BranchSwitcher.Active {
		selectedBranch, closed, cmd := m.BranchSwitcher.Update(msg)
		if closed {
			if selectedBranch != "" && m.GitRepo != nil {
				if err := m.GitRepo.CheckoutBranch(selectedBranch); err != nil {
					m.StatusBar.ShowToast("Failed to switch branch: "+err.Error(), true, 3*time.Second)
				} else {
					m.StatusBar.Branch = selectedBranch
					if m.GitView != nil {
						m.GitView.Refresh()
					}
					m.StatusBar.ShowToast("Beralih ke branch: "+selectedBranch, false, 2*time.Second)
				}
			}
		}
		return m, cmd
	}

	// 0.85. AI Settings Dialog Handling if active
	if m.AISettingsDialog != nil && m.AISettingsDialog.Active {
		m.AISettingsDialog.SetScreenSize(m.Width, m.Height)
		saved, canceled, newCfg, cmd := m.AISettingsDialog.Update(msg)
		if saved {
			m.Config.AI = newCfg
			_ = config.SaveConfig(m.Config)
			m.StatusBar.ShowToast("AI settings saved successfully", false, 3*time.Second)
		} else if canceled {
			// Dialog closed
		}
		return m, cmd
	}

	// 1. Buffer Switcher Modal Handling
	if m.Switcher.Active {
		targetIdx, closed, cmd := m.Switcher.Update(msg)
		if closed {
			if targetIdx >= 0 && targetIdx < m.TabManager.Count() {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.TabManager.ActiveIndex = targetIdx
				m.syncStatusBarWithActiveTab()
			}
		}
		return m, cmd
	}

	// 1.5. File Picker Handling if active
	if m.FilePicker != nil && m.FilePicker.Active {
		confirmed, canceled, selectedPath, cmd := m.FilePicker.Update(msg)
		if confirmed {
			m.handleFilePickerConfirmed(selectedPath)
		} else if canceled {
			m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
		}
		return m, cmd
	}

	// 2. Input Dialog Handling if active
	if m.InputDialog.Active {
		confirmed, canceled, cmd := m.InputDialog.Update(msg)
		if confirmed {
			m.handleInputConfirmed()
		} else if canceled {
			m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
		}
		return m, cmd
	}

	// 3. Confirm Dialog Handling if active
	if m.ConfirmDialog.Active {
		confirmed, canceled, discard := m.ConfirmDialog.Update(msg)
		if confirmed {
			cmd := m.handleConfirmConfirmed()
			if cmd != nil {
				return m, cmd
			}
		} else if discard {
			cmd := m.handleDiscardConfirmed()
			if cmd != nil {
				return m, cmd
			}
		} else if canceled {
			if m.ConfirmDialog.Type == dialog.ConfirmAlert {
				return m, nil
			}
			if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
				m.StatusBar.ShowToast("External changes ignored (local preserved)", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
			}
			m.ClosingTabIdx = -1
		}
		return m, nil
	}

	// 4. Menu Bar Key Handling
	if action, handled := m.MenuBar.HandleKey(msg); handled {
		if action != "" {
			cmd := m.handleMenuAction(action)
			return m, cmd
		}
		return m, nil
	}

	// 5. Modal escape
	if m.ShowHelp && m.HelpDialog != nil && m.HelpDialog.Active {
		closed := m.HelpDialog.Update(msg)
		if closed {
			m.ShowHelp = false
		}
		return m, nil
	}
	if msg.String() == "esc" {
		if m.ShowHelp {
			m.ShowHelp = false
			return m, nil
		}
		if m.ShowAbout {
			m.ShowAbout = false
			return m, nil
		}
	}

	// 5.5. Active Terminal Tab Focus & Raw Keystroke Interception
	if m.Focus == FocusEditor {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			switch msg.String() {
			case "ctrl+\\", "ctrl+`", "f6":
				m.Focus = FocusSidebar
				m.StatusBar.ShowToast("Focus: Sidebar (press Ctrl+\\ to return to Terminal)", false, 2*time.Second)
				return m, nil
			case "ctrl+shift+\\", "ctrl+t":
				var cmd tea.Cmd
				m, cmd = m.openNewTerminalTab()
				return m, cmd
			case "ctrl+shift+w", "alt+w":
				m.TabManager.CloseTab(m.TabManager.ActiveIndex)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Terminal ditutup", false, 1500*time.Millisecond)
				return m, nil
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, m.requestSessionPersist()
			case "ctrl+shift+p", "ctrl+P", "f1":
				m.openCommandPalette()
				return m, nil
			default:
				bytes := terminal.KeyToBytes(msg)
				if len(bytes) > 0 {
					_, _ = tab.TerminalView.Write(bytes)
				}
				return m, nil
			}
		}
	}

	// 6. Global Shortcuts
	switch msg.String() {
	case "ctrl+shift+\\", "ctrl+t":
		var cmd tea.Cmd
		m, cmd = m.openNewTerminalTab()
		return m, cmd
	case "ctrl+\\", "ctrl+`":
		m.toggleFocus()
		return m, nil
	case "ctrl+q":
		if m.hasDirtyTabs() && !m.AutoSave {
			m.ConfirmDialog.Open(
				dialog.ConfirmUnsavedChanges,
				"Unsaved Changes",
				"There are unsaved buffers.\nSave before exiting?",
				"app.exit",
			)
			return m, nil
		}
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		// Reset cursor shape before exiting
		fmt.Print("\x1b]22;default\x1b\\")
		return m, tea.Quit

	case "ctrl+b":
		m.SidebarVisible = !m.SidebarVisible
		return m, nil

	case "ctrl+e", "f6":
		m.toggleFocus()
		return m, nil

	case "alt+p", "alt+P":
		if m.TabManager.Count() > 0 && m.TabManager.ActiveIndex >= 0 {
			tab := m.TabManager.ActiveTab()
			wasPinned := tab != nil && tab.IsPinned
			m.TabManager.TogglePinTab(m.TabManager.ActiveIndex)
			if wasPinned {
				m.StatusBar.ShowToast("Tab unpinned", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Tab pinned", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		}
		return m, nil
	case "alt+s":
		m.AutoSave = !m.AutoSave
		m.StatusBar.AutoSave = m.AutoSave
		if m.AutoSave {
			m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
		}
		return m, nil
	case "alt+z", "alt+Z":
		m.toggleSoftWrap()
		return m, nil

	case "alt+[":
		m.adjustSidebarWidth(-2)
		return m, nil

	case "alt+]":
		m.adjustSidebarWidth(2)
		return m, nil

	case "alt+1":
		m.SidebarView = SidebarFiles
		return m, nil

	case "alt+2":
		m.SidebarView = SidebarGit
		if m.GitView != nil {
			if time.Since(m.GitView.LastRefresh) > 2*time.Second && !m.GitView.IsLoading {
				return m, m.triggerGitRefresh()
			}
		}
		return m, nil

	case "alt+3":
		m.SidebarView = SidebarSearch
		if m.SearchView != nil {
			m.SearchView.DoSearch()
		}
		return m, nil
	case "alt+b":
		m.openBranchSwitcher()
		return m, nil

	case "alt+g":
		return m.triggerAICommit()

	case "alt+i":
		m.openAISettings()
		return m, nil

	case "ctrl+enter":
		return m.triggerGitCommit()

	case "ctrl+shift+o", "ctrl+O":
		m.openGoToSymbol()
		return m, nil
	case "ctrl+o":
		m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)
		return m, nil

	case "ctrl+k":
		m.ctrlKPending = true
		m.StatusBar.ShowToast("Ctrl+K: Ctrl+0 fold all, Ctrl+J unfold all, Ctrl+K open folder", false, 2*time.Second)
		return m, nil

	case "ctrl+p":
		m.openQuickOpen()
		return m, nil

	case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
		return m.triggerFormatCurrentBuffer()

	case "ctrl+shift+p", "ctrl+P", "f1":
		m.openCommandPalette()
		return m, nil

	case "ctrl+shift+f", "ctrl+F", "f4":
		m.openGlobalSearch()
		return m, nil

	case "ctrl+f":
		m.openFindBar()
		return m, nil

	case "ctrl+h":
		m.openReplaceBar()
		return m, nil

	case "ctrl+g":
		m.openGotoLine()
		return m, nil

	case "ctrl+m", "alt+m":
		if m.TabManager.ToggleActiveMarkdown() {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.MarkdownPreview {
				m.StatusBar.ShowToast("Mode: Markdown Preview (Glamour)", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Mode: Markdown Editor (Raw)", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		} else {
			m.StatusBar.ShowToast("Active tab is not a Markdown file", true, 2*time.Second)
		}
		return m, nil

	case "?":
		// Allow typing '?' in active editor buffer
		if m.Focus == FocusEditor {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				break
			}
		}
		m.ShowHelp = !m.ShowHelp
		if m.ShowHelp && m.HelpDialog != nil {
			m.HelpDialog.Open(m.Height)
		}
		return m, nil
	}

	// 7. Context-Specific: Sidebar (Files or Git)
	if m.Focus == FocusSidebar && m.SidebarVisible {
		if m.SidebarView == SidebarGit && m.GitView != nil {
			if msg.String() == "d" || msg.String() == "x" {
				if m.GitView.FocusArea == git.GitFocusChanges {
					sel := m.GitView.SelectedChangesFile()
					if sel != nil {
						m.ConfirmDialog.Open(
							dialog.ConfirmDiscardGit,
							"Discard Changes",
							fmt.Sprintf("Discard all local changes in '%s'? This action cannot be undone.", sel.Name),
							sel.Path,
						)
						return m, nil
					}
				}
			}
			cmd, fileToOpen := m.GitView.Update(msg)
			if fileToOpen != nil {
				switch fileToOpen.Path {
				case "__hunk_stage__":
					if f := m.GitView.SelectedChangesFile(); f != nil {
						if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
							if err := m.GitRepo.StageHunk(f.Path, hIdx); err != nil {
								m.StatusBar.ShowToast("Failed to stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
							} else {
								m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
							}
							if m.GitView != nil {
								m.GitView.Refresh()
							}
							m.refreshDiffTabsForPath(f.Path)
						} else {
							m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
						}
					}
				case "__hunk_unstage__":
					if f := m.GitView.SelectedStagedFile(); f != nil {
						if hIdx, ok := m.GitView.SelectedHunkIndex(true); ok {
							if err := m.GitRepo.UnstageHunk(f.Path, hIdx); err != nil {
								m.StatusBar.ShowToast("Failed to unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
							} else {
								m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
							}
							if m.GitView != nil {
								m.GitView.Refresh()
							}
							m.refreshDiffTabsForPath(f.Path)
						} else {
							m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
						}
					}
				case "__hunk_discard__":
					if f := m.GitView.SelectedChangesFile(); f != nil {
						if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
							m.pendingGitHunkPath = f.Path
							m.pendingGitHunkIndex = hIdx
							m.ConfirmDialog.Open(
								dialog.ConfirmDiscardGitHunk,
								"Buang Hunk?",
								fmt.Sprintf("Buang hunk #%d pada '%s'?\nPerubahan worktree untuk hunk ini akan hilang permanen.", hIdx+1, f.Path),
								f.Path,
							)
							return m, cmd
						}
						m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
					}
				default:
					if fileToOpen.Status == "?" || fileToOpen.Status == "U" {
						fullPath := filepath.Join(m.RootDir, fileToOpen.Path)
						m.openFile(fullPath)
						m.Focus = FocusEditor
					} else {
						m.openDiff(fileToOpen.Path, fileToOpen.IsStaged)
					}
				}
			}
			return m, cmd
		}

		if m.SidebarView == SidebarSearch && m.SearchView != nil {
			filePath, line, col, cmd := m.SearchView.Update(msg)
			if filePath != "" {
				m.openFileAt(filePath, line, col)
				m.Focus = FocusEditor
			}
			return m, cmd
		}

		if m.SidebarView == SidebarFiles {
			switch msg.String() {
			case "j", "down":
				m.Explorer.MoveDown()
				return m, nil

			case "k", "up":
				m.Explorer.MoveUp()
				return m, nil

			case "enter", "l", "right":
				node := m.Explorer.SelectedNode()
				if node != nil {
					if node.IsDir {
						m.Explorer.ToggleExpand()
					} else {
						m.openFile(node.Path)
						m.Focus = FocusEditor
					}
				}
				return m, nil

			case "h", "left":
				m.Explorer.CollapseOrParent()
				return m, nil

			case ".":
				m.Explorer.ToggleHidden()
				return m, nil

			case "r":
				m.Explorer.Refresh()
				m.StatusBar.ShowToast("Pohon file diperbarui", false, 2*time.Second)
				return m, nil
			case "/":
				m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file:", m.Explorer.FilterQuery)
				return m, nil
			case "esc":
				if m.Explorer.FilterQuery != "" {
					m.Explorer.SetFilter("")
					m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
					return m, nil
				}

			case "a":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFile, "New File", parentDir, "")
				return m, nil

			case "A":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFolder, "New Folder", parentDir, "")
				return m, nil

			case "F2":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.InputDialog.Open(dialog.InputRename, "Rename: "+node.Name, node.Path, node.Name)
				}
				return m, nil

			case "d":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.ConfirmDialog.Open(
						dialog.ConfirmDelete,
						"Confirm Delete",
						"Are you sure you want to delete:\n"+node.Path+"?",
						node.Path,
					)
				}
				return m, nil
			}
		}
	}

	// 8. Context-Specific: Modeless Text Editor Buffer or Diff View
	if m.Focus == FocusEditor {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.IsDiff && tab.DiffView != nil {
			switch msg.String() {
			case "ctrl+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, m.requestSessionPersist()
			case "up", "k":
				tab.DiffView.ScrollUp(1)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "down", "j":
				tab.DiffView.ScrollDown(1, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgup":
				tab.DiffView.ScrollUp(10)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgdown":
				tab.DiffView.ScrollDown(10, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "home":
				tab.DiffView.ScrollOffset = 0
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "n", "]":
				tab.DiffView.NextHunk(m.Height - 3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "N", "[":
				tab.DiffView.PrevHunk(m.Height - 3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "s", "+":
				return m, m.applyDiffHunkAction(tab, "stage")
			case "u", "-":
				return m, m.applyDiffHunkAction(tab, "unstage")
			case "x":
				return m, m.applyDiffHunkAction(tab, "discard")
			}
		} else if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
			switch msg.String() {
			case "ctrl+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, m.requestSessionPersist()
			case "up", "k":
				tab.MarkdownViewer.ScrollUp(1)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "down", "j":
				tab.MarkdownViewer.ScrollDown(1, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgup":
				tab.MarkdownViewer.ScrollUp(10)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgdown":
				tab.MarkdownViewer.ScrollDown(10, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "home":
				tab.MarkdownViewer.ScrollHome()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+c":
				text := tab.MarkdownViewer.GetSelectedText()
				if text == "" {
					text = tab.MarkdownViewer.GetAllCleanText()
				}
				if text != "" {
					_ = clipboard.WriteAll(text)
					m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
				}
				return m, nil

			case "ctrl+a":
				tab.MarkdownViewer.SelectAll()
				m.StatusBar.ShowToast("All preview text selected", false, 1*time.Second)
				return m, nil

			case "ctrl+v":
				text, err := clipboard.ReadAll()
				if err == nil && text != "" && tab.Buffer != nil {
					tab.Buffer.InsertString(text)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Ditempel ke dokumen markdown", false, 2*time.Second)
				}
				return m, nil

			case "esc", "escape":
				if tab.MarkdownViewer.HasSelection {
					tab.MarkdownViewer.ClearSelection()
					return m, nil
				}
			}
		} else if tab != nil && tab.Buffer != nil {
			buf := tab.Buffer

			// Intercept terminal-specific KeyType signals for autocomplete and tab navigation
			if msg.Type == tea.KeyCtrlAt || msg.Type == tea.KeyNull {
				return m.handleCtrlSpace()
			}
			if msg.Type == tea.KeyShiftTab {
				return m.handleShiftTab()
			}

			switch msg.String() {
			case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
				return m.triggerFormatCurrentBuffer()

			case "ctrl+s":
				if m.Config != nil && m.Config.FormatOnSave {
					req := formatter.FormatRequest{
						Content:         tab.Buffer.Text(),
						FilePath:        tab.Path,
						WorkspaceDir:    m.RootDir,
						CustomOverrides: m.Config.Formatters,
					}
					res, err := formatter.Format(req)
					if err == nil && res.Formatted != tab.Buffer.Text() {
						tab.Buffer.ReplaceContentWithUndo(res.Formatted)
					}
				}
				if err := m.TabManager.SaveActive(); err != nil {
					m.StatusBar.ShowToast("Save error: "+err.Error(), true, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Saved: "+tab.Name, false, 2*time.Second)
					if m.SymbolIndex != nil {
						m.SymbolIndex.UpdateFile(tab.Path)
					}
					m.syncStatusBarWithActiveTab()
					return m, linter.RunLinterCmd(m.RootDir, tab.Path)
				}
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil

			case "ctrl+n":
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, m.requestSessionPersist()

			case "ctrl+c":
				text := buf.GetSelectedText()
				if text != "" {
					_ = clipboard.WriteAll(text)
					m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
				}
				return m, nil

			case "ctrl+v":
				text, err := clipboard.ReadAll()
				if err == nil && text != "" {
					buf.InsertString(text)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
				}
				return m, nil

			case "ctrl+x":
				text := buf.GetSelectedText()
				if text != "" {
					_ = clipboard.WriteAll(text)
					buf.DeleteSelection()
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
				}
				return m, nil

			case "ctrl+a":
				buf.SelectAll()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+z":
				if buf.Undo() {
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Undo", false, 1*time.Second)
				} else {
					m.StatusBar.ShowToast("Already at earliest change", false, 1*time.Second)
				}
				return m, nil

			case "ctrl+y", "ctrl+shift+z":
				if buf.Redo() {
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Redo", false, 1*time.Second)
				} else {
					m.StatusBar.ShowToast("Already at latest change", false, 1*time.Second)
				}
				return m, nil

			case "enter":
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
				buf.InsertNewline()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "backspace":
				buf.Backspace()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "alt+backspace", "ctrl+backspace":
				buf.DeleteWordLeft()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "delete":
				buf.Delete()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "tab":
				return m.handleTab()

			case "shift+tab", "backtab":
				return m.handleShiftTab()

			case "ctrl+/", "ctrl+_":
				buf.ToggleLineComment(tab.CommentPrefix())
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Komentar dialihkan", false, 1*time.Second)
				return m, nil

			case "ctrl+space", "ctrl+@", "ctrl+ ", "c-space":
				return m.handleCtrlSpace()

			case "up":
				if m.CompletionPopup != nil && m.CompletionPopup.Active {
					m.CompletionPopup.MoveUp()
					return m, nil
				}
				buf.ClearSelection()
				buf.MoveUp()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+up":
				buf.SelectUp()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "down":
				if m.CompletionPopup != nil && m.CompletionPopup.Active {
					m.CompletionPopup.MoveDown()
					return m, nil
				}
				buf.ClearSelection()
				buf.MoveDown()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+down":
				buf.SelectDown()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "left":
				buf.ClearSelection()
				buf.MoveLeft()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+left":
				buf.SelectLeft()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+left", "alt+left":
				buf.ClearSelection()
				buf.MoveWordLeft()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+alt+left", "alt+shift+left", "shift+ctrl+left", "ctrl+shift+left":
				buf.SelectWordLeft()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "right":
				buf.ClearSelection()
				buf.MoveRight()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+right":
				buf.SelectRight()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+right", "alt+right":
				buf.ClearSelection()
				buf.MoveWordRight()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+alt+right", "alt+shift+right", "shift+ctrl+right", "ctrl+shift+right":
				buf.SelectWordRight()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "home":
				buf.ClearSelection()
				buf.MoveHome()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "end":
				buf.ClearSelection()
				buf.MoveEnd()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "pgup":
				buf.ClearSelection()
				buf.MovePageUp(10)
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "pgdown":
				buf.ClearSelection()
				buf.MovePageDown(10)
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "f12":
				m.gotoDefinition()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "shift+f12", "Shift+F12":
				m.findReferences()
				return m, nil

			case "ctrl+d", "ctrl+D":
				m.addNextOccurrence()
				return m, nil

			case "ctrl+alt+down":
				buf.AddCursorBelow()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+alt+up":
				buf.AddCursorAbove()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+shift+[":
				if buf.ToggleFold(buf.CursorRow) {
					// if not on foldable, fold innermost enclosing? try fold at cursor line's block start by toggling current behavior: ToggleFold folds/unfolds at row or if folded region exists
				}
				// Better: fold current or containing block
				if !buf.IsFoldedAt(buf.CursorRow) {
					// try to fold block starting at/containing cursor by walking up
					row := buf.CursorRow
					folded := false
					for r := row; r >= 0; r-- {
						if end, ok := buf.FoldableAt(r); ok && r <= row && row <= end {
							if !buf.IsFoldedAt(r) {
								buf.ToggleFold(r)
							}
							folded = true
							break
						}
					}
					if !folded {
						m.StatusBar.ShowToast("No foldable blocks found", true, 1500*time.Millisecond)
					}
				} else {
					// already folded at cursor anchor -> keep folded (no-op) or refold; ToggleFold would unfold
				}
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "ctrl+shift+]":
				// unfold current/containing fold
				unfolded := false
				for _, fr := range append([]editor.FoldRegion(nil), buf.Folded...) {
					if buf.CursorRow >= fr.Start && buf.CursorRow <= fr.End {
						// unfold this region
						idx := buf.FoldIndexAt(fr.Start)
						if idx >= 0 {
							buf.Folded = append(buf.Folded[:idx], buf.Folded[idx+1:]...)
						}
						unfolded = true
						break
					}
				}
				if !unfolded && buf.IsFoldedAt(buf.CursorRow) {
					buf.ToggleFold(buf.CursorRow)
					unfolded = true
				}
				if !unfolded {
					m.StatusBar.ShowToast("No folded block at cursor", true, 1500*time.Millisecond)
				}
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "esc", "escape":
				if m.CompletionPopup != nil && m.CompletionPopup.Active {
					m.CompletionPopup.Close()
					return m, nil
				}
				if m.SnippetSession != nil && m.SnippetSession.Active {
					m.SnippetSession.Cancel()
					m.StatusBar.ShowToast("Snippet session selesai", false, 1*time.Second)
					return m, nil
				}
				if buf.CursorCount() > 1 {
					buf.CollapseCursors()
					m.StatusBar.ShowToast("Cursor digabung", false, 1*time.Second)
				} else if buf.HasSelection {
					buf.ClearSelection()
				}
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "za":
				if buf.ToggleFold(buf.CursorRow) {
					m.syncStatusBarWithActiveTab()
				} else {
					m.StatusBar.ShowToast("No foldable block on this line", true, 2*time.Second)
				}
				return m, nil

			case "zM":
				buf.FoldAll()
				m.syncStatusBarWithActiveTab()
				return m, nil

			case "zR":
				buf.UnfoldAll()
				m.syncStatusBarWithActiveTab()
				return m, nil

			default:
				// Insert printable characters
				if len(msg.Runes) > 0 && !msg.Alt {
					for _, r := range msg.Runes {
						buf.InsertRune(r)
						if r == '>' && editor.IsAutoCloseTagSupported(tab.Path) {
							if buf.CursorRow >= 0 && buf.CursorRow < buf.LineCount() {
								line := string(buf.Lines[buf.CursorRow])
								if buf.CursorCol > 1 && buf.CursorCol <= len(line) {
									lineBeforeGT := line[:buf.CursorCol-1]
									closeTag := editor.DetectAutoCloseTag(lineBeforeGT)
									if closeTag != "" {
										buf.InsertString(closeTag)
										buf.CursorCol -= len(closeTag)
									}
								}
							}
						}
					}
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
			}
		}
	}

	return m, nil
}
