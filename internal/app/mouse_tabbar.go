package app

import (
	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
)

func (m Model) handleTabDragMouse(msg tea.MouseMsg, mouse tea.Mouse, isMotion, isRelease bool) (tea.Model, tea.Cmd, bool) {
	if m.DraggingTabIdx >= 0 {
		if isMotion {
			diffX := mouse.X - m.DragTabStartX
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
				if mouse.Y == 1 && mouse.X >= editorStartX {
					relX := mouse.X - editorStartX
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
			return m, nil, true
		}
		if isRelease {
			m.DraggingTabIdx = -1
			m.IsDraggingTab = false
			m.TabManager.DraggingIndex = -1
			m.setMousePointer("default")
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleTabBarRightClick(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Tab Bar right-click (Row 1, X >= editorStartX)
	if mouse.Y == 1 && mouse.X >= editorStartX {
		relX := mouse.X - editorStartX
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
			m.ContextMenu.OpenCustom(mouse.X, mouse.Y+1, items, m.Width, m.Height)
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleReleaseDragTab(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	if m.DraggingTabIdx >= 0 {
		m.DraggingTabIdx = -1
		m.IsDraggingTab = false
		m.TabManager.DraggingIndex = -1
		m.setMousePointer("default")
		return m, nil, true
	}

	return m, nil, false
}

func (m Model) handleTabBarWheelMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Mouse scroll wheel on Tab Bar (Row 1): scroll tabs horizontally
	if mouse.Y == 1 && mouse.X >= editorStartX {
		if mouse.Button == tea.MouseWheelUp {
			m.TabManager.ScrollTabsLeft()
			return m, nil, true
		} else if mouse.Button == tea.MouseWheelDown {
			m.TabManager.ScrollTabsRight()
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) handleTabBarRow1Click(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isClick bool) (tea.Model, tea.Cmd, bool) {
	// Row 1: Tab Bar click (Editor columns)
	if mouse.Y == 1 && mouse.X >= editorStartX {
		relX := mouse.X - editorStartX
		// Middle-click closes the tab under the cursor (no activate/toggle side effects)
		if isClick && mouse.Button == tea.MouseMiddle {
			idx := m.TabManager.FindTabAt(relX)
			if idx >= 0 {
				m.attemptCloseTab(idx)
				return m, nil, true
			}
			return m, nil, true
		}
		action, clickedTabIdx := m.TabManager.HandleTabClick(relX)
		if action == "scroll_left" {
			m.TabManager.ScrollTabsLeft()
			return m, nil, true
		}
		if action == "scroll_right" {
			m.TabManager.ScrollTabsRight()
			return m, nil, true
		}
		if action == "toggle_markdown" {
			m.TabManager.ToggleActiveMarkdown()
			m.syncStatusBarWithActiveTab()
			return m, nil, true
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
			m.DragTabStartX = mouse.X
			m.TabManager.DraggingIndex = clickedTabIdx
			return m, nil, true
		} else if action == "close" {
			m.attemptCloseTab(clickedTabIdx)
			return m, nil, true
		}
	}
	return m, nil, false
}
