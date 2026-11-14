package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleRightClickMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isClick bool) (tea.Model, tea.Cmd, bool) {
	if !isClick || mouse.Button != tea.MouseRight {
		return m, nil, false
	}
	if nm, cmd, handled := m.handleTerminalRightClick(msg, mouse, editorStartX); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleTabBarRightClick(msg, mouse, editorStartX); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleSidebarRightClick(msg, mouse, editorStartX); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleEditorRightClick(msg, mouse, editorStartX); handled {
		return nm, cmd, true
	}
	return m, nil, false
}

// handleReleaseMouse routes release events (sidebar resize end, tab drag end, terminal/editor selection finalization).
func (m Model) handleReleaseMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isRelease bool) (tea.Model, tea.Cmd, bool) {
	if !isRelease || (mouse.Button != tea.MouseLeft && mouse.Button != tea.MouseRight) {
		return m, nil, false
	}
	if mouse.Button == tea.MouseLeft {
		if nm, cmd, handled := m.handleReleaseResizer(msg, mouse); handled {
			return nm, cmd, true
		}
		if nm, cmd, handled := m.handleReleaseDragTab(msg, mouse); handled {
			return nm, cmd, true
		}
	}
	return m.handleReleaseEditor(msg, mouse, editorStartX)
}
