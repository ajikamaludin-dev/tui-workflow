package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleClickFallbackMouse(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int, isClick bool) (tea.Model, tea.Cmd, bool) {
	if !isClick || mouse.Button != tea.MouseLeft {
		return m, nil, false
	}
	if nm, cmd, handled := m.handleSidebarRow1Click(msg, mouse); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleTabBarRow1Click(msg, mouse, editorStartX, isClick); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleSidebarRow2Click(msg, mouse, isClick); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleSidebarBottomTabsClick(msg, mouse, isClick); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleSidebarBodyClick(msg, mouse, isClick); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleEditorBufferClick(msg, mouse, editorStartX); handled {
		return nm, cmd, true
	}
	if nm, cmd, handled := m.handleStatusBarClick(msg, mouse); handled {
		return nm, cmd, true
	}
	return m, nil, false
}
