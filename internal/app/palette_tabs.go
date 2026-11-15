package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"tui-workflow/internal/editor"
)

// handleTabPaletteAction dispatches tab-related command palette / menu actions.
// Ctrl+Tab / Ctrl+Shift+Tab key handling lives in handleTerminalKeys; this is
// the shared logic for the palette, menu and context-menu entry points.
func (m *Model) handleTabPaletteAction(action string) tea.Cmd {
	switch action {
	case "tab.next", "tab.prev":
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		if action == "tab.next" {
			m.TabManager.NextTab()
		} else {
			m.TabManager.PrevTab()
		}
		m.syncStatusBarWithActiveTab()
		return m.requestSessionPersist()

	case "tab.toggle_pin":
		if m.TabManager.Count() > 0 && m.TabManager.ActiveIndex >= 0 {
			tab := m.TabManager.ActiveTab()
			wasPinned := tab != nil && tab.IsPinned
			m.TabManager.TogglePinTab(m.TabManager.ActiveIndex)
			if wasPinned {
				m.StatusBar.ShowToast("Tab unpinned", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Tab pinned", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		}
		return nil

	case "tab.close_others":
		if m.blockBatchCloseIfRunning("others", m.TabManager.ActiveIndex) {
			return nil
		}
		m.TabManager.CloseOtherTabs(m.TabManager.ActiveIndex)
		m.syncStatusBarWithActiveTab()
		return nil

	case "tab.close_all":
		if m.blockBatchCloseIfRunning("all", -1) {
			return nil
		}
		m.TabManager.CloseAllTabs()
		m.syncStatusBarWithActiveTab()
		return nil
	}
	return nil
}
