package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/atotto/clipboard"
	"strconv"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/formatter"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/snippet"
	"tui-workflow/internal/ui"
)

func (m Model) handleBufferKeys(msg tea.KeyPressMsg, tab *editor.Tab) (tea.Model, tea.Cmd, bool) {
	if tab != nil && tab.Buffer != nil {
		buf := tab.Buffer

		// Intercept terminal-specific KeyType signals for autocomplete and tab navigation
		if msg.Code == tea.KeySpace && msg.Mod.Contains(tea.ModCtrl) {
			nm, cmd := m.handleCtrlSpace()
			return nm, cmd, true
		}
		if msg.Code == tea.KeyTab && msg.Mod.Contains(tea.ModShift) {
			nm, cmd := m.handleShiftTab()
			return nm, cmd, true
		}

		switch ui.KeyName(msg) {
		case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
			nm, cmd := m.triggerFormatCurrentBuffer()
			return nm, cmd, true

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
				return m, linter.RunLinterCmd(m.RootDir, tab.Path), true
			}
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+w":
			m.attemptCloseTab(m.TabManager.ActiveIndex)
			return m, nil, true

		case "ctrl+n":
			if m.AutoSave {
				_, _ = editor.TriggerAutoSave(m.TabManager)
			}
			m.TabManager.NextTab()
			m.syncStatusBarWithActiveTab()
			return m, m.requestSessionPersist(), true

		case "ctrl+c":
			text := buf.GetSelectedText()
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
			return m, nil, true

		case "ctrl+v":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" {
				buf.InsertString(text)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
			}
			return m, nil, true

		case "ctrl+x":
			text := buf.GetSelectedText()
			if text != "" {
				_ = clipboard.WriteAll(text)
				buf.DeleteSelection()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
			}
			return m, nil, true

		case "ctrl+a":
			buf.SelectAll()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+z":
			if buf.Undo() {
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Undo", false, 1*time.Second)
			} else {
				m.StatusBar.ShowToast("Already at earliest change", false, 1*time.Second)
			}
			return m, nil, true

		case "ctrl+y", "ctrl+shift+z":
			if buf.Redo() {
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Redo", false, 1*time.Second)
			} else {
				m.StatusBar.ShowToast("Already at latest change", false, 1*time.Second)
			}
			return m, nil, true

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
							return m, nil, true
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
					return m, nil, true
				}
			}
			buf.InsertNewline()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "backspace":
			buf.Backspace()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "alt+backspace", "ctrl+backspace":
			buf.DeleteWordLeft()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "delete":
			buf.Delete()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "tab":
			nm, cmd := m.handleTab()
			return nm, cmd, true

		case "shift+tab", "backtab":
			nm, cmd := m.handleShiftTab()
			return nm, cmd, true

		case "ctrl+/", "ctrl+_":
			buf.ToggleLineComment(tab.CommentPrefix())
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Komentar dialihkan", false, 1*time.Second)
			return m, nil, true

		case "ctrl+space", "ctrl+@", "ctrl+ ", "c-space":
			nm, cmd := m.handleCtrlSpace()
			return nm, cmd, true

		case "up":
			if m.CompletionPopup != nil && m.CompletionPopup.Active {
				m.CompletionPopup.MoveUp()
				return m, nil, true
			}
			buf.ClearSelection()
			buf.MoveUp()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+up":
			buf.SelectUp()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "down":
			if m.CompletionPopup != nil && m.CompletionPopup.Active {
				m.CompletionPopup.MoveDown()
				return m, nil, true
			}
			buf.ClearSelection()
			buf.MoveDown()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+down":
			buf.SelectDown()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "left":
			buf.ClearSelection()
			buf.MoveLeft()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+left":
			buf.SelectLeft()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+left", "alt+left":
			buf.ClearSelection()
			buf.MoveWordLeft()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+alt+left", "alt+shift+left", "shift+ctrl+left", "ctrl+shift+left":
			buf.SelectWordLeft()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "right":
			buf.ClearSelection()
			buf.MoveRight()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+right":
			buf.SelectRight()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+right", "alt+right":
			buf.ClearSelection()
			buf.MoveWordRight()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+alt+right", "alt+shift+right", "shift+ctrl+right", "ctrl+shift+right":
			buf.SelectWordRight()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "home":
			buf.ClearSelection()
			buf.MoveHome()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "end":
			buf.ClearSelection()
			buf.MoveEnd()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "pgup":
			buf.ClearSelection()
			buf.MovePageUp(10)
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "pgdown":
			buf.ClearSelection()
			buf.MovePageDown(10)
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "f12":
			m.gotoDefinition()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "shift+f12", "Shift+F12":
			m.findReferences()
			return m, nil, true

		case "ctrl+d", "ctrl+D":
			m.addNextOccurrence()
			return m, nil, true

		case "ctrl+alt+down":
			buf.AddCursorBelow()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+alt+up":
			buf.AddCursorAbove()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

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
			return m, nil, true

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
			return m, nil, true

		case "esc", "escape":
			if m.CompletionPopup != nil && m.CompletionPopup.Active {
				m.CompletionPopup.Close()
				return m, nil, true
			}
			if m.SnippetSession != nil && m.SnippetSession.Active {
				m.SnippetSession.Cancel()
				m.StatusBar.ShowToast("Snippet session selesai", false, 1*time.Second)
				return m, nil, true
			}
			if buf.CursorCount() > 1 {
				buf.CollapseCursors()
				m.StatusBar.ShowToast("Cursor digabung", false, 1*time.Second)
			} else if buf.HasSelection {
				buf.ClearSelection()
			}
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "za":
			if buf.ToggleFold(buf.CursorRow) {
				m.syncStatusBarWithActiveTab()
			} else {
				m.StatusBar.ShowToast("No foldable block on this line", true, 2*time.Second)
			}
			return m, nil, true

		case "zM":
			buf.FoldAll()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "zR":
			buf.UnfoldAll()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		default:
			// Insert printable characters (v2: msg.Text holds the characters)
			if len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModAlt) && !msg.Mod.Contains(tea.ModCtrl) {
				for _, r := range msg.Text {
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
				return m, nil, true
			}
		}
	}
	return m, nil, false
}
