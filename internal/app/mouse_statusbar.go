package app

import (
	tea "charm.land/bubbletea/v2"
	"strconv"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleStatusBarClick(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	// Row Height-1: Status Bar click
	if mouse.Y == m.Height-1 {
		action := m.StatusBar.HandleMouseClick(mouse.X)
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
			return m, nil, true

		case "branch_switcher":
			m.openBranchSwitcher()
			return m, nil, true

		case "goto_line":
			curLine := 1
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil {
				curLine = tab.Buffer.CursorRow + 1
			}
			m.InputDialog.Open(dialog.InputGotoLine, "Go to Line [line:col]", "", strconv.Itoa(curLine))
			return m, nil, true

		case "help":
			m.ShowHelp = true
			if m.HelpDialog != nil {
				m.HelpDialog.Open(m.Height)
			}
			return m, nil, true
		}
		return m, nil, true
	}

	return m, nil, false
}
