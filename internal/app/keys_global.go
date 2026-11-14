package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if nm, cmd, handled := m.handleCtrlKPending(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleGoToSymbolPending(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleContextMenuKey(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleSearchKeys(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleModalKeys(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleTerminalKeys(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleSidebarKeys(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleEditorKeys(msg); handled {
		return nm, cmd
	}
	return m, nil
}
