package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleMenuBarMouse(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	// Forward mouse to menu bar
	if action, handled := m.MenuBar.HandleMouse(msg); handled {
		if action != "" {
			cmd := m.handleMenuAction(action)
			return m, cmd, true
		}
		return m, nil, true
	}
	return m, nil, false
}
