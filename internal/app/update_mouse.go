package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Forward mouse to context menu
	if m.ContextMenu != nil && m.ContextMenu.Active {
		action, handled := m.ContextMenu.HandleMouse(msg)
		if handled {
			if action != "" {
				cmd := m.handleContextMenuAction(action)
				return m, cmd
			}
			return m, nil
		}
	}

	// Intercept mouse clicks for active modals
	if m.FindBar != nil && m.FindBar.Active {
		barStr := m.FindBar.Render()
		findWidth := lipgloss.Width(barStr)
		findX := m.Width - findWidth - 2
		editorStartX := 0
		if m.SidebarVisible {
			editorStartX = m.SidebarWidth
		}
		if findX < editorStartX+1 {
			findX = editorStartX + 1
		}
		relX := msg.X - findX
		relY := msg.Y - 1 // Floating at row 1
		action, closed, handled := m.FindBar.HandleMouse(relX, relY)
		if handled {
			tab := m.TabManager.ActiveTab()
			if closed {
				if tab != nil && tab.Buffer != nil {
					tab.Buffer.ClearSearch()
				}
				return m, nil
			}
			if tab != nil && tab.Buffer != nil {
				switch action {
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
			return m, nil
		}
	}

	if m.CommandPalette != nil && m.CommandPalette.Active {
		modalStr := m.CommandPalette.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 3
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			actionID, closed, handled := m.CommandPalette.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && actionID != "" {
					menuCmd := m.handleMenuAction(actionID)
					return m, menuCmd
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.QuickOpen != nil && m.QuickOpen.Active {
		modalStr := m.QuickOpen.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 3
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			tabIdx, relPath, closed, handled := m.QuickOpen.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed {
					if tabIdx >= 0 {
						m.TabManager.ActiveIndex = tabIdx
						m.Focus = FocusEditor
						m.syncStatusBarWithActiveTab()
					} else if relPath != "" {
						fullPath := filepath.Join(m.RootDir, relPath)
						m.openFile(fullPath)
					}
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.GlobalSearch != nil && m.GlobalSearch.Active {
		modalStr := m.GlobalSearch.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 3
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			relPath, lineNum, colNum, closed, handled := m.GlobalSearch.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && relPath != "" {
					fullPath := filepath.Join(m.RootDir, relPath)
					m.openFileAt(fullPath, lineNum, colNum)
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.BranchSwitcher != nil && m.BranchSwitcher.Active {
		modalStr := m.BranchSwitcher.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 3
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			targetBranch, closed, handled := m.BranchSwitcher.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && targetBranch != "" && m.GitRepo != nil {
					if err := m.GitRepo.CheckoutBranch(targetBranch); err != nil {
						m.StatusBar.ShowToast("Failed to switch branch: "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.Branch = targetBranch
						if m.GitView != nil {
							m.GitView.Refresh()
						}
						m.StatusBar.ShowToast("Beralih ke branch: "+targetBranch, false, 2*time.Second)
					}
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.AISettingsDialog != nil && m.AISettingsDialog.Active {
		modalStr := m.AISettingsDialog.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 2
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			confirmed, canceled, newCfg, handled := m.AISettingsDialog.HandleMouse(relX, relY)
			if handled {
				if confirmed {
					m.Config.AI = newCfg
					_ = config.SaveConfig(m.Config)
					m.StatusBar.ShowToast("AI configuration saved successfully", false, 3*time.Second)
				} else if canceled {
					m.StatusBar.ShowToast("Pengaturan AI dibatalkan", false, 2*time.Second)
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.FilePicker != nil && m.FilePicker.Active {
		modalStr := m.FilePicker.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 2
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			confirmed, canceled, selectedPath, handled := m.FilePicker.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if confirmed {
				m.handleFilePickerConfirmed(selectedPath)
			} else if canceled {
				m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
			}
			if handled {
				return m, nil
			}
		}
		return m, nil
	}

	if m.InputDialog.Active {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			modalStr := m.InputDialog.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := msg.X - startX
			relY := msg.Y - startY
			confirmed, canceled, handled := m.InputDialog.HandleMouse(relX, relY)
			if confirmed {
				m.handleInputConfirmed()
			} else if canceled {
				m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
			}
			if handled {
				return m, nil
			}
		}
		return m, nil
	}

	if m.ConfirmDialog.Active {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			modalStr := m.ConfirmDialog.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := msg.X - startX
			relY := msg.Y - startY
			confirmed, canceled, discard, handled := m.ConfirmDialog.HandleMouse(relX, relY)
			if confirmed {
				return m, m.handleConfirmConfirmed()
			} else if discard {
				return m, m.handleDiscardConfirmed()
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
			if handled {
				return m, nil
			}
		}
		return m, nil
	}

	if m.Switcher.Active {
		modalStr := m.Switcher.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 3
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
			targetIdx, closed, handled := m.Switcher.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && targetIdx >= 0 {
					m.TabManager.ActiveIndex = targetIdx
					m.Focus = FocusEditor
					m.syncStatusBarWithActiveTab()
				}
				return m, nil
			}
		}
		return m, nil
	}

	// Resizing Sidebar Handling
	if m.SidebarVisible {
		isAtBorder := msg.X == m.SidebarWidth-1 && msg.Y >= 1 && msg.Y < m.Height-1
		if isAtBorder {
			m.setMousePointer("col-resize")
		}

		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && isAtBorder {
			m.ResizingSidebar = true
			return m, nil
		}
	}

	if m.ResizingSidebar {
		if msg.Action == tea.MouseActionMotion {
			m.setSidebarWidth(msg.X)
			return m, nil
		}
		if msg.Action == tea.MouseActionRelease {
			m.ResizingSidebar = false
			return m, nil
		}
	}

	if m.DraggingTabIdx >= 0 {
		if msg.Action == tea.MouseActionMotion {
			diffX := msg.X - m.DragTabStartX
			if diffX < 0 {
				diffX = -diffX
			}
			if diffX >= 1 {
				m.IsDraggingTab = true
				m.setMousePointer("grab")
				editorStartX := 0
				if m.SidebarVisible {
					editorStartX = m.SidebarWidth
				}
				if msg.Y == 1 && msg.X >= editorStartX {
					relX := msg.X - editorStartX
					targetTab := m.TabManager.FindTabAt(relX)
					if targetTab >= 0 && targetTab != m.DraggingTabIdx {
						if m.TabManager.MoveTab(m.DraggingTabIdx, targetTab) {
							m.DraggingTabIdx = targetTab
							m.TabManager.DraggingIndex = targetTab
							m.TabManager.ActiveIndex = targetTab
							editorWidth := m.Width - editorStartX
							_ = m.TabManager.RenderTabBar(editorWidth)
							m.syncStatusBarWithActiveTab()
						}
					}
				}
			}
			return m, nil
		}
		if msg.Action == tea.MouseActionRelease {
			m.DraggingTabIdx = -1
			m.IsDraggingTab = false
			m.TabManager.DraggingIndex = -1
			m.setMousePointer("default")
			return m, nil
		}
	}

	editorStartX := 0
	if m.SidebarVisible {
		editorStartX = m.SidebarWidth
	}

	// Update mouse pointer shape: col-resize at border, text over editor, default elsewhere
	if m.SidebarVisible && msg.X == m.SidebarWidth-1 && msg.Y >= 1 && msg.Y < m.Height-1 {
		m.setMousePointer("col-resize")
	} else if msg.X >= editorStartX && msg.Y >= 2 && msg.Y < m.Height-1 {
		m.setMousePointer("text")
	} else {
		m.setMousePointer("default")
	}

	// Right-click handling
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
		// Terminal tab with inner mouse reporting active → forward right press to PTY
		if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
			if tab := m.TabManager.ActiveTab(); tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				if tab.TerminalView.WantsMouse() {
					relX := msg.X - editorStartX
					relY := msg.Y - 2
					if relX < 0 {
						relX = 0
					}
					if relY < 0 {
						relY = 0
					}
					_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
					return m, nil
				}
			}
		}
		// Tab Bar right-click (Row 1, X >= editorStartX)
		if msg.Y == 1 && msg.X >= editorStartX {
			relX := msg.X - editorStartX
			clickedTabIdx := m.TabManager.FindTabAt(relX)
			if clickedTabIdx >= 0 && clickedTabIdx < m.TabManager.Count() {
				m.ContextMenuTabIdx = clickedTabIdx
				targetTab := m.TabManager.Tabs[clickedTabIdx]
				pinLabel := "Pin Tab"
				if targetTab.IsPinned {
					pinLabel = "Unpin Tab"
				}
				items := []ui.ContextMenuItem{
					{ID: "tab_toggle_pin", Label: pinLabel, Shortcut: "Alt+P"},
					{ID: "tab_close_this", Label: "Close", Shortcut: "Ctrl+W"},
					{ID: "tab_close_others", Label: "Close Others"},
					{ID: "tab_close_right", Label: "Close to the Right"},
					{ID: "tab_close_left", Label: "Close to the Left"},
					{ID: "tab_close_all", Label: "Close All"},
				}
				if targetTab.IsPinned {
					items[1].Disabled = true
				}
				if m.TabManager.Count() <= 1 {
					items[2].Disabled = true
					items[3].Disabled = true
					items[4].Disabled = true
				} else {
					if clickedTabIdx == m.TabManager.Count()-1 {
						items[3].Disabled = true
					}
					if clickedTabIdx == 0 {
						items[4].Disabled = true
					}
				}
				m.ContextMenu.OpenCustom(msg.X, msg.Y+1, items, m.Width, m.Height)
				return m, nil
			}
		}

		// Right-click inside File Explorer sidebar
		if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 3 && msg.Y < m.Height-2 && m.SidebarView == SidebarFiles {
			m.Focus = FocusSidebar
			filterActive := m.Explorer.FilterQuery != ""
			nodeBodyY := msg.Y
			if filterActive {
				nodeBodyY = msg.Y - 1
			}
			if filterActive && msg.Y == 3 {
				// Right-click on filter banner: treat as clear-filter shortcut via context menu with single entry
				items := []ui.ContextMenuItem{{ID: "explorer_clear_filter", Label: "Clear Filter", Shortcut: "Esc"}}
				m.ContextMenu.OpenCustom(msg.X, msg.Y+1, items, m.Width, m.Height)
				return m, nil
			}
			if nodeBodyY >= 3 {
				relLine := nodeBodyY - 3
				targetIdx := m.Explorer.ScrollOffset + relLine
				if targetIdx >= 0 && targetIdx < len(m.Explorer.FlatNodes) {
					m.Explorer.SelectedIndex = targetIdx
				}
			}
			m.ContextMenuTabIdx = -1
			m.openExplorerContextMenu(msg.X, msg.Y+1)
			return m, nil
		}

		// Right-click inside editor area -> Open Editor Context Menu
		if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
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
			m.ContextMenu.Open(msg.X, msg.Y, hasSelection, hasClipboard, m.Width, m.Height)
			return m, nil
		}
	}

	// Drag mouse inside editor buffer area to select text
	if msg.Action == tea.MouseActionMotion && msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
			relRow := msg.Y - 2 + tab.MarkdownViewer.ScrollOffset
			relCol := msg.X - editorStartX
			if relCol < 0 {
				relCol = 0
			}
			tab.MarkdownViewer.UpdateSelection(relRow, relCol)
			return m, nil
		} else if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			relX := msg.X - editorStartX
			if relX < 0 {
				relX = 0
			}
			relY := msg.Y - 2
			if relY < 0 {
				relY = 0
			}
			if tab.TerminalView.WantsMouse() {
				_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
			} else {
				tab.TerminalView.UpdateSelection(relX, relY)
			}
			return m, nil
		} else if tab != nil && tab.Buffer != nil {
			relY := msg.Y - 2
			gutterW := tab.Buffer.GutterWidth()
			textX := msg.X - editorStartX - gutterW
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
		return m, nil
	}

	// Release mouse inside editor area
	if msg.Action == tea.MouseActionRelease && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonRight) {
		if msg.Button == tea.MouseButtonLeft {
			if m.ResizingSidebar {
				m.ResizingSidebar = false
				return m, nil
			}
			if m.DraggingTabIdx >= 0 {
				m.DraggingTabIdx = -1
				m.IsDraggingTab = false
				m.TabManager.DraggingIndex = -1
				m.setMousePointer("default")
				return m, nil
			}
		}
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			relX := msg.X - editorStartX
			if relX < 0 {
				relX = 0
			}
			relY := msg.Y - 2
			if relY < 0 {
				relY = 0
			}
			if tab.TerminalView.WantsMouse() {
				_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
				return m, nil
			}
			if msg.Button == tea.MouseButtonLeft {
				tab.TerminalView.UpdateSelection(relX, relY)
				if !tab.TerminalView.HasSelection() {
					tab.TerminalView.ClearSelection()
				}
			}
			return m, nil
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
		return m, nil
	}

	// Forward mouse to menu bar
	if action, handled := m.MenuBar.HandleMouse(msg); handled {
		if action != "" {
			cmd := m.handleMenuAction(action)
			return m, cmd
		}
		return m, nil
	}

	// Mouse scroll wheel on Tab Bar (Row 1): scroll tabs horizontally
	if msg.Y == 1 && msg.X >= editorStartX {
		if msg.Button == tea.MouseButtonWheelUp {
			m.TabManager.ScrollTabsLeft()
			return m, nil
		} else if msg.Button == tea.MouseButtonWheelDown {
			m.TabManager.ScrollTabsRight()
			return m, nil
		}
	}

	// Mouse scroll wheel in sidebar (Row 2 to Height-2)
	if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 2 && msg.Y < m.Height-1 {
		sidebarH := m.Height - 4
		if sidebarH < 1 {
			sidebarH = 1
		}
		if msg.Button == tea.MouseButtonWheelUp {
			if m.SidebarView == SidebarFiles && m.Explorer != nil {
				m.Explorer.ScrollUp(3)
				return m, nil
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				m.GitView.ScrollUp(3)
				return m, nil
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				if m.SearchView.Selected > 0 {
					m.SearchView.Selected--
				}
				return m, nil
			}
		} else if msg.Button == tea.MouseButtonWheelDown {
			if m.SidebarView == SidebarFiles && m.Explorer != nil {
				m.Explorer.ScrollDown(3, sidebarH)
				return m, nil
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				m.GitView.ScrollDown(3, sidebarH)
				return m, nil
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				if m.SearchView.Selected < len(m.SearchView.Matches)-1 {
					m.SearchView.Selected++
				}
				return m, nil
			}
		}
	}

	// Mouse scroll wheel in editor
	if m.Focus == FocusEditor && m.TabManager.ActiveTab() != nil {
		tab := m.TabManager.ActiveTab()
		if msg.Action == tea.MouseActionPress {
			if tab.IsDiff && tab.DiffView != nil {
				if msg.Button == tea.MouseButtonWheelUp {
					tab.DiffView.ScrollUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				} else if msg.Button == tea.MouseButtonWheelDown {
					tab.DiffView.ScrollDown(3, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
			} else if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				if msg.Button == tea.MouseButtonWheelUp {
					tab.MarkdownViewer.ScrollUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				} else if msg.Button == tea.MouseButtonWheelDown {
					tab.MarkdownViewer.ScrollDown(3, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
			} else if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				if tab.TerminalView.WantsMouse() {
					relX := msg.X - editorStartX
					relY := msg.Y - 2
					if relX < 0 {
						relX = 0
					}
					if relY < 0 {
						relY = 0
					}
					_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
					return m, nil
				}
				if msg.Button == tea.MouseButtonWheelUp {
					tab.TerminalView.ScrollUp(3)
					return m, nil
				} else if msg.Button == tea.MouseButtonWheelDown {
					tab.TerminalView.ScrollDown(3)
					return m, nil
				}
			} else if tab.Buffer != nil {
				buf := tab.Buffer
				if msg.Button == tea.MouseButtonWheelUp {
					buf.MovePageUp(3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				} else if msg.Button == tea.MouseButtonWheelDown {
					buf.MovePageDown(3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
			}
		}
	}

	// ShowHelp / ShowAbout mouse handling
	if m.ShowHelp && m.HelpDialog != nil && m.HelpDialog.Active {
		modalStr := m.HelpDialog.Render(m.Height)
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 1 {
			startY = 1
		}
		relX := msg.X - startX
		relY := msg.Y - startY
		isWheelUp := msg.Button == tea.MouseButtonWheelUp
		isWheelDown := msg.Button == tea.MouseButtonWheelDown
		closed, _ := m.HelpDialog.HandleMouse(relX, relY, isWheelUp, isWheelDown)
		if closed {
			m.ShowHelp = false
		}
		return m, nil
	}
	if m.ShowHelp || m.ShowAbout {
		if msg.Action == tea.MouseActionPress {
			m.ShowHelp = false
			m.ShowAbout = false
		}
		return m, nil
	}

	// Mouse click handling
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Row 1: Sidebar Title / Branch click (Sidebar columns)
		if msg.Y == 1 && m.SidebarVisible && msg.X < m.SidebarWidth {
			m.Focus = FocusSidebar
			if m.SidebarView == SidebarGit {
				m.openBranchSwitcher()
				return m, nil
			}
		}

		// Row 1: Tab Bar click (Editor columns)
		if msg.Y == 1 && msg.X >= editorStartX {
			relX := msg.X - editorStartX
			// Middle-click closes the tab under the cursor (no activate/toggle side effects)
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonMiddle {
				idx := m.TabManager.FindTabAt(relX)
				if idx >= 0 {
					m.attemptCloseTab(idx)
					return m, nil
				}
				return m, nil
			}
			action, clickedTabIdx := m.TabManager.HandleTabClick(relX)
			if action == "scroll_left" {
				m.TabManager.ScrollTabsLeft()
				return m, nil
			}
			if action == "scroll_right" {
				m.TabManager.ScrollTabsRight()
				return m, nil
			}
			if action == "toggle_markdown" {
				m.TabManager.ToggleActiveMarkdown()
				m.syncStatusBarWithActiveTab()
				return m, nil
			}
			if action == "activate" {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.TabManager.ActiveIndex = clickedTabIdx
				m.Focus = FocusEditor
				m.syncStatusBarWithActiveTab()
				m.DraggingTabIdx = clickedTabIdx
				m.IsDraggingTab = false
				m.DragTabStartX = msg.X
				m.TabManager.DraggingIndex = clickedTabIdx
				return m, nil
			} else if action == "close" {
				m.attemptCloseTab(clickedTabIdx)
				return m, nil
			}
		}

		// Row 2: Sidebar refresh button row click (limited to the [⟳ Refresh] hit area)
		if msg.Y == 2 && m.SidebarVisible && msg.X >= 0 && msg.X < 15 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.Focus = FocusSidebar
			if m.SidebarView == SidebarFiles {
				m.Explorer.Refresh()
				m.StatusBar.ShowToast("File explorer reloaded", false, 2*time.Second)
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				return m, m.triggerGitRefresh()
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				m.SearchView.DoSearch()
				m.StatusBar.ShowToast("Search updated", false, 2*time.Second)
			}
			return m, nil
		}

		// Row Height-2: Sidebar bottom tabs click (📁 Files | 🌿 Git | 🔍 Search)
		if msg.Y == m.Height-2 && m.SidebarVisible && msg.X < m.SidebarWidth && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.Focus = FocusSidebar
			filesTabW := lipgloss.Width(" 1:Files ")
			gitTabW := lipgloss.Width(" 2:Git ")
			searchTabW := lipgloss.Width(" 3:Search ")
			if msg.X >= 0 && msg.X < filesTabW {
				m.SidebarView = SidebarFiles
			} else if msg.X >= filesTabW && msg.X < filesTabW+gitTabW {
				m.SidebarView = SidebarGit
				if m.GitView != nil && time.Since(m.GitView.LastRefresh) > 2*time.Second && !m.GitView.IsLoading {
					return m, m.triggerGitRefresh()
				}
			} else if msg.X >= filesTabW+gitTabW && msg.X < filesTabW+gitTabW+searchTabW {
				m.SidebarView = SidebarSearch
				if m.SearchView != nil {
					m.SearchView.DoSearch()
				}
			}
			return m, nil
		}

		// Sidebar body rows click (Row 3 to Height-3)
		if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 3 && msg.Y < m.Height-2 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.Focus != FocusSidebar && m.AutoSave {
				_, _ = editor.TriggerAutoSave(m.TabManager)
			}
			m.Focus = FocusSidebar
			if m.SidebarView == SidebarFiles {
				if m.Explorer == nil {
					return m, nil
				}
				filterActive := m.Explorer.FilterQuery != ""
				// Filter banner occupies row Y==3; node rows start at Y==4
				if filterActive && msg.Y == 3 {
					m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file (kosong = bersihkan):", m.Explorer.FilterQuery)
					return m, nil
				}
				relLine := msg.Y - 3
				if filterActive {
					relLine = msg.Y - 4
				}
				if relLine < 0 {
					return m, nil
				}
				action, node := m.Explorer.HandleClick(relLine)
				if action == "open" && node != nil {
					m.openFile(node.Path)
				}
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				relLine := msg.Y - 3
				targetFile, action, _ := m.GitView.HandleMouseClick(msg.X, relLine)
				if action == "commit" {
					return m.triggerGitCommit()
				} else if action == "push" {
					if m.GitView.IsLoading {
						return m, nil
					}
					if m.GitRepo == nil {
						m.StatusBar.ShowToast("Git repository not found", true, 3*time.Second)
						return m, nil
					}
					m.GitView.IsLoading = true
					m.GitView.LoadingMsg = "Pushing..."
					m.GitView.SpinnerFrame = 0
					m.StatusBar.ShowToast("Pushing ke remote repository...", false, 5*time.Second)
					return m, tea.Batch(gitPushCmd(m.GitRepo), gitLoadingTickCmd())
				} else if action == "ai_commit" {
					return m.triggerAICommit()
				}
				if action == "discard" && targetFile != nil {
					m.ConfirmDialog.Open(
						dialog.ConfirmDiscardGit,
						"Discard Changes",
						fmt.Sprintf("Discard all local changes in '%s'? This action cannot be undone.", targetFile.Name),
						targetFile.Path,
					)
					return m, nil
				}
				if targetFile != nil {
					if targetFile.Status == "?" || targetFile.Status == "U" {
						fullPath := filepath.Join(m.RootDir, targetFile.Path)
						m.openFile(fullPath)
						m.Focus = FocusEditor
					} else {
						m.openDiff(targetFile.Path, targetFile.IsStaged)
					}
				}
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				relLine := msg.Y - 3
				filePath, line, col, _ := m.SearchView.HandleMouseClick(msg.X, relLine)
				if filePath != "" {
					m.openFileAt(filePath, line, col)
					m.Focus = FocusEditor
				}
			}
			return m, nil
		}
		// Click inside editor buffer area (Row 2 to Height-2, X >= editorStartX)
		if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
			m.Focus = FocusEditor
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				relRow := msg.Y - 2 + tab.MarkdownViewer.ScrollOffset
				relCol := msg.X - editorStartX
				if relCol < 0 {
					relCol = 0
				}
				tab.MarkdownViewer.StartSelection(relRow, relCol)
				return m, nil
			} else if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				relX := msg.X - editorStartX
				if relX < 0 {
					relX = 0
				}
				relY := msg.Y - 2
				if relY < 0 {
					relY = 0
				}
				if tab.TerminalView.WantsMouse() {
					_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
					return m, nil
				}
				tab.TerminalView.StartSelection(relX, relY)
				return m, nil
			} else if tab != nil && tab.Buffer != nil {
				relY := msg.Y - 2
				gutterW := tab.Buffer.GutterWidth()
				relX := msg.X - editorStartX

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
						return m, nil
					}
					// Gutter baris non-foldable: pindahkan kursor ke awal baris itu
					tab.Buffer.CursorRow = physRow
					tab.Buffer.CursorCol = 0
					tab.Buffer.CollapseCursors()
					tab.Buffer.ClearSelection()
					m.syncStatusBarWithActiveTab()
					return m, nil
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
					return m, nil
				}

				switch {
				case msg.Ctrl && msg.Alt:
					tab.Buffer.AddCursor(physRow, physCol)
				case msg.Alt:
					tab.Buffer.CursorRow, tab.Buffer.CursorCol = physRow, physCol
					tab.Buffer.CollapseCursors()
					tab.Buffer.ClearSelection()
					m.gotoDefinition()
					m.syncStatusBarWithActiveTab()
					return m, nil
				default:
					tab.Buffer.StartSelection(physRow, physCol)
				}
				m.syncStatusBarWithActiveTab()
			}
			return m, nil
		}

		// Row Height-1: Status Bar click
		if msg.Y == m.Height-1 {
			action := m.StatusBar.HandleMouseClick(msg.X)
			switch action {
			case "toggle_autosave":
				m.AutoSave = !m.AutoSave
				m.StatusBar.AutoSave = m.AutoSave
				if m.AutoSave {
					m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
				}
				if m.Config != nil {
					m.Config.AutoSave = m.AutoSave
					_ = config.SaveConfig(m.Config)
				}
				return m, nil

			case "branch_switcher":
				m.openBranchSwitcher()
				return m, nil

			case "goto_line":
				curLine := 1
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.Buffer != nil {
					curLine = tab.Buffer.CursorRow + 1
				}
				m.InputDialog.Open(dialog.InputGotoLine, "Go to Line [line:col]", "", strconv.Itoa(curLine))
				return m, nil

			case "help":
				m.ShowHelp = true
				if m.HelpDialog != nil {
					m.HelpDialog.Open(m.Height)
				}
				return m, nil
			}
			return m, nil
		}
	}

	return m, nil
}
