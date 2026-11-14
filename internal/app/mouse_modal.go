package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"path/filepath"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleModalMouse(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	if m.ContextMenu != nil && m.ContextMenu.Active {
		action, handled := m.ContextMenu.HandleMouse(msg)
		if handled {
			if action != "" {
				cmd := m.handleContextMenuAction(action)
				return m, cmd, true
			}
			return m, nil, true
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
		relX := mouse.X - findX
		relY := mouse.Y - 1 // Floating at row 1
		action, closed, handled := m.FindBar.HandleMouse(relX, relY)
		if handled {
			tab := m.TabManager.ActiveTab()
			if closed {
				if tab != nil && tab.Buffer != nil {
					tab.Buffer.ClearSearch()
				}
				return m, nil, true
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
			return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
			actionID, closed, handled := m.CommandPalette.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && actionID != "" {
					menuCmd := m.handleMenuAction(actionID)
					return m, menuCmd, true
				}
				return m, nil, true
			}
		}
		return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
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
				return m, nil, true
			}
		}
		return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
			relPath, lineNum, colNum, closed, handled := m.GlobalSearch.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && relPath != "" {
					fullPath := filepath.Join(m.RootDir, relPath)
					m.openFileAt(fullPath, lineNum, colNum)
				}
				return m, nil, true
			}
		}
		return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
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
				return m, nil, true
			}
		}
		return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		if isClick && mouse.Button == tea.MouseLeft {
			confirmed, canceled, newCfg, handled := m.AISettingsDialog.HandleMouse(relX, relY)
			if handled {
				if confirmed {
					m.Config.AI = newCfg
					_ = config.SaveConfig(m.Config)
					m.StatusBar.ShowToast("AI configuration saved successfully", false, 3*time.Second)
				} else if canceled {
					m.StatusBar.ShowToast("Pengaturan AI dibatalkan", false, 2*time.Second)
				}
				return m, nil, true
			}
		}
		return m, nil, true
	}

	if m.FilePicker != nil && m.FilePicker.Active {
		modalStr := m.FilePicker.Render()
		modalW := lipgloss.Width(modalStr)
		modalH := lipgloss.Height(modalStr)
		startX := (m.Width - modalW) / 2
		startY := (m.Height - modalH) / 2
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
			confirmed, canceled, selectedPath, handled := m.FilePicker.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if confirmed {
				m.handleFilePickerConfirmed(selectedPath)
			} else if canceled {
				m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
			}
			if handled {
				return m, nil, true
			}
		}
		return m, nil, true
	}

	if m.InputDialog.Active {
		if isClick && mouse.Button == tea.MouseLeft {
			modalStr := m.InputDialog.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := mouse.X - startX
			relY := mouse.Y - startY
			confirmed, canceled, handled := m.InputDialog.HandleMouse(relX, relY)
			if confirmed {
				m.handleInputConfirmed()
			} else if canceled {
				m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
			}
			if handled {
				return m, nil, true
			}
		}
		return m, nil, true
	}

	if m.ConfirmDialog.Active {
		if isClick && mouse.Button == tea.MouseLeft {
			modalStr := m.ConfirmDialog.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := mouse.X - startX
			relY := mouse.Y - startY
			confirmed, canceled, discard, handled := m.ConfirmDialog.HandleMouse(relX, relY)
			if confirmed {
				return m, m.handleConfirmConfirmed(), true
			} else if discard {
				return m, m.handleDiscardConfirmed(), true
			} else if canceled {
				if m.ConfirmDialog.Type == dialog.ConfirmAlert {
					return m, nil, true
				}
				if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
					m.StatusBar.ShowToast("External changes ignored (local preserved)", false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
				}
				m.ClosingTabIdx = -1
			}
			if handled {
				return m, nil, true
			}
		}
		return m, nil, true
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		if isClick || isWheelUp || isWheelDown {
			targetIdx, closed, handled := m.Switcher.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if handled {
				if closed && targetIdx >= 0 {
					m.TabManager.ActiveIndex = targetIdx
					m.Focus = FocusEditor
					m.syncStatusBarWithActiveTab()
				}
				return m, nil, true
			}
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleHelpAboutMouse(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
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
		relX := mouse.X - startX
		relY := mouse.Y - startY
		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		closed, _ := m.HelpDialog.HandleMouse(relX, relY, isWheelUp, isWheelDown)
		if closed {
			m.ShowHelp = false
		}
		return m, nil, true
	}
	if m.ShowHelp || m.ShowAbout {
		if isClick {
			m.ShowHelp = false
			m.ShowAbout = false
		}
		return m, nil, true
	}
	return m, nil, false
}
