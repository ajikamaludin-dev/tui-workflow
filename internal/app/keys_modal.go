package app

import (
	tea "charm.land/bubbletea/v2"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleContextMenuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// 1. Context Menu Handling if active
	if m.ContextMenu != nil && m.ContextMenu.Active {
		switch ui.KeyName(msg) {
		case "esc":
			m.ContextMenu.Close()
			return m, nil, true
		case "up", "k":
			m.ContextMenu.Prev()
			return m, nil, true
		case "down", "j":
			m.ContextMenu.Next()
			return m, nil, true
		case "enter":
			item := m.ContextMenu.SelectedItem()
			m.ContextMenu.Close()
			if item != nil && !item.Disabled {
				cmd := m.handleContextMenuAction(item.ID)
				return m, cmd, true
			}
			return m, nil, true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleModalKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
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
		return m, cmd, true
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
		return m, cmd, true
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
		return m, cmd, true
	}

	// 1.5. File Picker Handling if active
	if m.FilePicker != nil && m.FilePicker.Active {
		confirmed, canceled, selectedPath, cmd := m.FilePicker.Update(msg)
		if confirmed {
			m.handleFilePickerConfirmed(selectedPath)
		} else if canceled {
			m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
		}
		return m, cmd, true
	}

	// 2. Input Dialog Handling if active
	if m.InputDialog.Active {
		confirmed, canceled, cmd := m.InputDialog.Update(msg)
		if confirmed {
			m.handleInputConfirmed()
		} else if canceled {
			m.StatusBar.ShowToast("Operation cancelled", false, 2*time.Second)
		}
		return m, cmd, true
	}

	// 3. Confirm Dialog Handling if active
	if m.ConfirmDialog.Active {
		confirmed, canceled, discard := m.ConfirmDialog.Update(msg)
		if confirmed {
			cmd := m.handleConfirmConfirmed()
			if cmd != nil {
				return m, cmd, true
			}
		} else if discard {
			cmd := m.handleDiscardConfirmed()
			if cmd != nil {
				return m, cmd, true
			}
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
		return m, nil, true
	}

	// 4. Menu Bar Key Handling
	if action, handled := m.MenuBar.HandleKey(msg); handled {
		if action != "" {
			cmd := m.handleMenuAction(action)
			return m, cmd, true
		}
		return m, nil, true
	}

	// 5. Modal escape
	if m.ShowHelp && m.HelpDialog != nil && m.HelpDialog.Active {
		closed := m.HelpDialog.Update(msg)
		if closed {
			m.ShowHelp = false
		}
		return m, nil, true
	}
	if ui.KeyName(msg) == "esc" {
		if m.ShowHelp {
			m.ShowHelp = false
			return m, nil, true
		}
		if m.ShowAbout {
			m.ShowAbout = false
			return m, nil, true
		}
	}
	return m, nil, false
}
