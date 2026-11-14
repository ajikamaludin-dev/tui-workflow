package app

import (
	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/editor"
)

func (m Model) handleTerminalRightClick(msg tea.MouseMsg, mouse tea.Mouse, editorStartX int) (tea.Model, tea.Cmd, bool) {
	// Terminal tab with inner mouse reporting active → forward right press to PTY
	if mouse.Y >= 2 && mouse.Y < m.Height-1 && mouse.X >= editorStartX {
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			if tab.TerminalView.WantsMouse() {
				relX := mouse.X - editorStartX
				relY := mouse.Y - 2
				if relX < 0 {
					relX = 0
				}
				if relY < 0 {
					relY = 0
				}
				_ = tab.TerminalView.ForwardMouse(msg, relX, relY)
				return m, nil, true
			}
		}
	}
	return m, nil, false
}
