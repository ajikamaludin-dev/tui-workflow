package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"tui-workflow/internal/editor"
)

func (m Model) handleEditorRightClick(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Right-click inside editor area -> Open Editor Context Menu
	if mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
		m.ContextMenuTabIdx = -1
		m.Focus = FocusEditor
		hasSelection := false
		if tab := m.TabManager.ActiveTab(); tab != nil {
			if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				hasSelection = tab.MarkdownViewer.HasSelection
			} else if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				hasSelection = tab.TerminalView.HasSelection()
			} else if tab.Buffer != nil {
				hasSelection = tab.Buffer.HasSelection
			}
		}
		clipText, _ := clipboard.ReadAll()
		hasClipboard := len(clipText) > 0
		m.ContextMenu.Open(mouse.X, mouse.Y, hasSelection, hasClipboard, m.Width, m.Height)
		return m, nil, true
	}

	return m, nil, false
}

func (m Model) handleMotionMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isMotion bool) (tea.Model, tea.Cmd, bool) {
	// Drag mouse inside editor buffer area to select text
	if isMotion && mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
			relRow := mouse.Y - 2 + tab.MarkdownViewer.ScrollOffset
			relCol := mouse.X - editorStartX
			if relCol < 0 {
				relCol = 0
			}
			tab.MarkdownViewer.UpdateSelection(relRow, relCol)
			return m, nil, true
		} else if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			relX := mouse.X - editorStartX
			if relX < 0 {
				relX = 0
			}
			relY := mouse.Y - 2
			if relY < 0 {
				relY = 0
			}
			if tab.TerminalView.WantsMouse() {
				_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
			} else {
				tab.TerminalView.UpdateSelection(relX, relY)
			}
			return m, nil, true
		} else if tab != nil && tab.Buffer != nil {
			relY := mouse.Y - 2
			gutterW := tab.Buffer.GutterWidth()
			textX := mouse.X - editorStartX - gutterW
			if textX < 0 {
				textX = 0
			}
			textW := m.Width - editorStartX - gutterW
			if textW < 10 {
				textW = 10
			}
			physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)
			tab.Buffer.UpdateSelection(physRow, physCol)
			m.syncStatusBarWithActiveTab()
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleReleaseEditor(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	tab := m.TabManager.ActiveTab()
	if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil &&
		mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
		relX := mouse.X - editorStartX
		relY := mouse.Y - 2
		if tab.TerminalView.WantsMouse() {
			_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
			return m, nil, true
		}
		if mouse.Button == tea.MouseLeft {
			tab.TerminalView.UpdateSelection(relX, relY)
			if !tab.TerminalView.HasSelection() {
				tab.TerminalView.ClearSelection()
			}
		}
		return m, nil, true
	}
	if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
		sR, sC, eR, eC := tab.MarkdownViewer.NormalizedSelection()
		if sR == eR && sC == eC {
			tab.MarkdownViewer.ClearSelection()
		}
	} else if tab != nil && tab.Buffer != nil && tab.Buffer.HasSelection {
		if tab.Buffer.SelectAnchorRow == tab.Buffer.CursorRow && tab.Buffer.SelectAnchorCol == tab.Buffer.CursorCol {
			tab.Buffer.ClearSelection()
			m.syncStatusBarWithActiveTab()
		}
	}
	return m, nil, true
}

func (m Model) handleEditorFocusMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isWheel bool) (tea.Model, tea.Cmd, bool) {
	// Mouse scroll wheel in editor
	if m.Focus == FocusEditor && m.TabManager.ActiveTab() != nil {
		tab := m.TabManager.ActiveTab()
		if isWheel {
			if tab.IsDiff && tab.DiffView != nil {
				if mouse.Button == tea.MouseWheelUp {
					tab.DiffView.ScrollUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				} else if mouse.Button == tea.MouseWheelDown {
					tab.DiffView.ScrollDown(3, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				}
			} else if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				if mouse.Button == tea.MouseWheelUp {
					tab.MarkdownViewer.ScrollUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				} else if mouse.Button == tea.MouseWheelDown {
					tab.MarkdownViewer.ScrollDown(3, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				}
			} else if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				if mouse.Button == tea.MouseWheelUp || mouse.Button == tea.MouseWheelDown {
					if mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
						relX := mouse.X - editorStartX
						relY := mouse.Y - 2
						if tab.TerminalView.WantsMouse() {
							_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
							return m, nil, true
						}
						if mouse.Button == tea.MouseWheelUp {
							tab.TerminalView.ScrollUp(3)
							return m, nil, true
						} else if mouse.Button == tea.MouseWheelDown {
							tab.TerminalView.ScrollDown(3)
							return m, nil, true
						}
					}
				}
			} else if tab.Buffer != nil {
				buf := tab.Buffer
				if mouse.Button == tea.MouseWheelUp {
					buf.MovePageUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				} else if mouse.Button == tea.MouseWheelDown {
					buf.MovePageDown(3)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				}
			}
		}
	}
	return m, nil, false
}

func (m Model) handleEditorBufferClick(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Click inside editor buffer area (Row 2 to Height-2, X >= editorStartX)
	if mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
		m.Focus = FocusEditor
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
			relRow := mouse.Y - 2 + tab.MarkdownViewer.ScrollOffset
			relCol := mouse.X - editorStartX
			if relCol < 0 {
				relCol = 0
			}
			tab.MarkdownViewer.StartSelection(relRow, relCol)
			return m, nil, true
		} else if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			relX := mouse.X - editorStartX
			if relX < 0 {
				relX = 0
			}
			relY := mouse.Y - 2
			if relY < 0 {
				relY = 0
			}
			if tab.TerminalView.WantsMouse() {
				_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
				return m, nil, true
			}
			tab.TerminalView.StartSelection(relX, relY)
			return m, nil, true
		} else if tab != nil && tab.Buffer != nil {
			relY := mouse.Y - 2
			gutterW := tab.Buffer.GutterWidth()
			relX := mouse.X - editorStartX

			textW := m.Width - editorStartX - gutterW
			if textW < 10 {
				textW = 10
			}

			physRow, _ := tab.Buffer.RowColFromVisual(relY, 0, textW)

			// Gutter click (marker col 0, line number digits, or separator): toggle fold anchor baris itu
			if relX >= 0 && relX < gutterW {
				if _, ok := tab.Buffer.FoldableAt(physRow); ok || tab.Buffer.IsFoldedAt(physRow) {
					tab.Buffer.ToggleFold(physRow)
					m.syncStatusBarWithActiveTab()
					return m, nil, true
				}
				// Gutter baris non-foldable: pindahkan kursor ke awal baris itu
				tab.Buffer.CursorRow = physRow
				tab.Buffer.CursorCol = 0
				tab.Buffer.CollapseCursors()
				tab.Buffer.ClearSelection()
				m.syncStatusBarWithActiveTab()
				return m, nil, true
			}

			textX := relX - gutterW
			if textX < 0 {
				textX = 0
			}

			physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)

			// Klik pada fold badge di ujung baris terlipat: buka lipatan (unfold)
			if tab.Buffer.IsFoldedAt(physRow) && physCol >= len(tab.Buffer.Lines[physRow]) {
				tab.Buffer.ToggleFold(physRow)
				m.syncStatusBarWithActiveTab()
				return m, nil, true
			}

			switch {
			case mouse.Mod.Contains(tea.ModCtrl) && mouse.Mod.Contains(tea.ModAlt):
				tab.Buffer.AddCursor(physRow, physCol)
			case mouse.Mod.Contains(tea.ModAlt):
				tab.Buffer.CursorRow, tab.Buffer.CursorCol = physRow, physCol
				tab.Buffer.CollapseCursors()
				tab.Buffer.ClearSelection()
				m.gotoDefinition()
				m.syncStatusBarWithActiveTab()
				return m, nil, true
			default:
				tab.Buffer.StartSelection(physRow, physCol)
			}
			m.syncStatusBarWithActiveTab()
		}
		return m, nil, true
	}
	return m, nil, false
}
