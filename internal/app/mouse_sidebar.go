package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"path/filepath"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleSidebarResizeStart(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	// Resizing Sidebar Handling
	if m.SidebarVisible {
		isAtBorder := mouse.X == m.SidebarWidth-1 && mouse.Y >= 1 && mouse.Y < m.Height-1
		if isAtBorder {
			m.setMousePointer("col-resize")
		}

		if isClick && mouse.Button == tea.MouseLeft && isAtBorder {
			m.ResizingSidebar = true
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleSidebarResizing(msg tea.MouseMsg, mouse tea.Mouse, isMotion, isRelease bool) (tea.Model, tea.Cmd, bool) {
	if m.ResizingSidebar {
		if isMotion {
			m.setSidebarWidth(mouse.X)
			return m, nil, true
		}
		if isRelease {
			m.ResizingSidebar = false
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleReleaseResizer(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	if m.ResizingSidebar {
		m.ResizingSidebar = false
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleSidebarRightClick(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Right-click inside File Explorer sidebar
	if m.SidebarVisible && mouse.X < m.SidebarWidth && mouse.Y >= 3 && mouse.Y < m.Height-2 && m.SidebarView == SidebarFiles {
		m.Focus = FocusSidebar
		filterActive := m.Explorer.FilterQuery != ""
		nodeBodyY := mouse.Y
		if filterActive {
			nodeBodyY = mouse.Y - 1
		}
		if filterActive && mouse.Y == 3 {
			// Right-click on filter banner: treat as clear-filter shortcut via context menu with single entry
			items := []ui.ContextMenuItem{{ID: "explorer_clear_filter", Label: "Clear Filter", Shortcut: "Esc"}}
			m.ContextMenu.OpenCustom(mouse.X, mouse.Y+1, items, m.Width, m.Height)
			return m, nil, true
		}
		if nodeBodyY >= 3 {
			relLine := nodeBodyY - 3
			targetIdx := m.Explorer.ScrollOffset + relLine
			if targetIdx >= 0 && targetIdx < len(m.Explorer.FlatNodes) {
				m.Explorer.SelectedIndex = targetIdx
			}
		}
		m.ContextMenuTabIdx = -1
		m.openExplorerContextMenu(mouse.X, mouse.Y+1)
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleSidebarWheelMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Mouse scroll wheel in sidebar (Row 2 to Height-2)
	if m.SidebarVisible && mouse.X < m.SidebarWidth && mouse.Y >= 2 && mouse.Y < m.Height-1 {
		sidebarH := m.Height - 4
		if sidebarH < 1 {
			sidebarH = 1
		}
		if mouse.Button == tea.MouseWheelUp {
			if m.SidebarView == SidebarFiles && m.Explorer != nil {
				m.Explorer.ScrollUp(3)
				return m, nil, true
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				m.GitView.ScrollUp(3)
				return m, nil, true
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				if m.SearchView.Selected > 0 {
					m.SearchView.Selected--
				}
				return m, nil, true
			}
		} else if mouse.Button == tea.MouseWheelDown {
			if m.SidebarView == SidebarFiles && m.Explorer != nil {
				m.Explorer.ScrollDown(3, sidebarH)
				return m, nil, true
			} else if m.SidebarView == SidebarGit && m.GitView != nil {
				m.GitView.ScrollDown(3, sidebarH)
				return m, nil, true
			} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
				if m.SearchView.Selected < len(m.SearchView.Matches)-1 {
					m.SearchView.Selected++
				}
				return m, nil, true
			}
		}
	}
	return m, nil, false
}

func (m Model) handleSidebarRow1Click(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	// Row 1: Sidebar Title / Branch click (Sidebar columns)
	if mouse.Y == 1 && m.SidebarVisible && mouse.X < m.SidebarWidth {
		m.Focus = FocusSidebar
		if m.SidebarView == SidebarGit {
			m.openBranchSwitcher()
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleSidebarRow2Click(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	// Row 2: Sidebar refresh button row click (limited to the [⟳ Refresh] hit area)
	if mouse.Y == 2 && m.SidebarVisible && mouse.X >= 0 && mouse.X < 15 && isClick && mouse.Button == tea.MouseLeft {
		m.Focus = FocusSidebar
		if m.SidebarView == SidebarFiles {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("File explorer reloaded", false, 2*time.Second)
		} else if m.SidebarView == SidebarGit && m.GitView != nil {
			return m, m.triggerGitRefresh(), true
		} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
			m.SearchView.DoSearch()
			m.StatusBar.ShowToast("Search updated", false, 2*time.Second)
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleSidebarBottomTabsClick(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	// Row Height-2: Sidebar bottom tabs click (📁 Files | 🌿 Git | 🔍 Search)
	if mouse.Y == m.Height-2 && m.SidebarVisible && mouse.X < m.SidebarWidth && isClick && mouse.Button == tea.MouseLeft {
		m.Focus = FocusSidebar
		filesTabW := lipgloss.Width(" 1:Files ")
		gitTabW := lipgloss.Width(" 2:Git ")
		searchTabW := lipgloss.Width(" 3:Search ")
		if mouse.X >= 0 && mouse.X < filesTabW {
			m.SidebarView = SidebarFiles
		} else if mouse.X >= filesTabW && mouse.X < filesTabW+gitTabW {
			m.SidebarView = SidebarGit
			if m.GitView != nil && time.Since(m.GitView.LastRefresh) > 2*time.Second && !m.GitView.IsLoading {
				return m, m.triggerGitRefresh(), true
			}
		} else if mouse.X >= filesTabW+gitTabW && mouse.X < filesTabW+gitTabW+searchTabW {
			m.SidebarView = SidebarSearch
			if m.SearchView != nil {
				m.SearchView.DoSearch()
			}
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleSidebarBodyClick(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	// Sidebar body rows click (Row 3 to Height-3)
	if m.SidebarVisible && mouse.X < m.SidebarWidth && mouse.Y >= 3 && mouse.Y < m.Height-2 && isClick && mouse.Button == tea.MouseLeft {
		if m.Focus != FocusSidebar && m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		m.Focus = FocusSidebar
		if m.SidebarView == SidebarFiles {
			if m.Explorer == nil {
				return m, nil, true
			}
			filterActive := m.Explorer.FilterQuery != ""
			// Filter banner occupies row Y==3; node rows start at Y==4
			if filterActive && mouse.Y == 3 {
				m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file (kosong = bersihkan):", m.Explorer.FilterQuery)
				return m, nil, true
			}
			relLine := mouse.Y - 3
			if filterActive {
				relLine = mouse.Y - 4
			}
			if relLine < 0 {
				return m, nil, true
			}
			action, node := m.Explorer.HandleClick(relLine)
			if action == "open" && node != nil {
				m.openFile(node.Path)
			}
		} else if m.SidebarView == SidebarGit && m.GitView != nil {
			relLine := mouse.Y - 3
			targetFile, action, _ := m.GitView.HandleMouseClick(mouse.X, relLine)
			if action == "commit" {
				nm, cmd := m.triggerGitCommit()
				return nm, cmd, true
			} else if action == "push" {
				if m.GitView.IsLoading {
					return m, nil, true
				}
				if m.GitRepo == nil {
					m.StatusBar.ShowToast("Git repository not found", true, 3*time.Second)
					return m, nil, true
				}
				m.GitView.IsLoading = true
				m.GitView.LoadingMsg = "Pushing..."
				m.GitView.SpinnerFrame = 0
				m.StatusBar.ShowToast("Pushing ke remote repository...", false, 5*time.Second)
				return m, tea.Batch(gitPushCmd(m.GitRepo), gitLoadingTickCmd()), true
			} else if action == "ai_commit" {
				nm, cmd := m.triggerAICommit()
				return nm, cmd, true
			}
			if action == "discard" && targetFile != nil {
				m.ConfirmDialog.Open(
					dialog.ConfirmDiscardGit,
					"Discard Changes",
					fmt.Sprintf("Discard all local changes in '%s'? This action cannot be undone.", targetFile.Name),
					targetFile.Path,
				)
				return m, nil, true
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
			relLine := mouse.Y - 3
			filePath, line, col, _ := m.SearchView.HandleMouseClick(mouse.X, relLine)
			if filePath != "" {
				m.openFileAt(filePath, line, col)
				m.Focus = FocusEditor
			}
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleSidebarResizeMouse(msg tea.MouseMsg, mouse tea.Mouse, isClick, isMotion, isRelease bool) (tea.Model, tea.Cmd, bool) {
	if nm, cmd, handled := m.handleSidebarResizeStart(msg, mouse, isClick); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleSidebarResizing(msg, mouse, isMotion, isRelease); handled {
		return nm, cmd, true
	}
	return m, nil, false
}
