package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/terminal"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleTerminalKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// 5.5. Active Terminal Tab Focus & Raw Keystroke Interception
	if m.Focus == FocusEditor {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			switch ui.KeyName(msg) {
			case "ctrl+`", "f6":
				m.Focus = FocusSidebar
				m.StatusBar.ShowToast("Focus: Sidebar (press Ctrl+` to return to Terminal)", false, 2*time.Second)
				return m, nil, true
			case "ctrl+shift+\\", "ctrl+\\", "ctrl+|", "ctrl+t":
				var cmd tea.Cmd
				m, cmd = m.openNewTerminalTab()
				return m, cmd, true
			case "ctrl+shift+w", "alt+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil, true
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, m.requestSessionPersist(), true
			case "ctrl+shift+p", "ctrl+P", "f1":
				m.openCommandPalette()
				return m, nil, true
			default:
				bytes := terminal.KeyToBytes(msg)
				if len(bytes) > 0 {
					_, _ = tab.TerminalView.Write(bytes)
				}
				return m, nil, true
			}
		}
	}

	// 6. Global Shortcuts
	switch ui.KeyName(msg) {
	case "ctrl+shift+\\", "ctrl+\\", "ctrl+|", "ctrl+t":
		var cmd tea.Cmd
		m, cmd = m.openNewTerminalTab()
		return m, cmd, true
	case "ctrl+`":
		m.toggleFocus()
		return m, nil, true
	case "ctrl+q":
		if m.hasDirtyTabs() && !m.AutoSave {
			m.ConfirmDialog.Open(
				dialog.ConfirmUnsavedChanges,
				"Unsaved Changes",
				"There are unsaved buffers.\nSave before exiting?",
				"app.exit",
			)
			return m, nil, true
		}
		if runningTerm := m.hasRunningTerminalProcesses(); runningTerm != nil {
			m.ConfirmDialog.Open(
				dialog.ConfirmCloseTerminal,
				"Active Terminal Process",
				fmt.Sprintf("Terminal '%s' still has active processes running.\nTerminate process and exit?", runningTerm.Name),
				"app.exit",
			)
			return m, nil, true
		}
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		// Reset cursor shape before exiting
		fmt.Print("\x1b]22;default\x1b\\")
		return m, tea.Quit, true

	case "ctrl+b":
		m.SidebarVisible = !m.SidebarVisible
		return m, nil, true

	case "ctrl+e", "f6":
		m.toggleFocus()
		return m, nil, true

	case "alt+p", "alt+P":
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
		return m, nil, true
	case "alt+s":
		m.AutoSave = !m.AutoSave
		m.StatusBar.AutoSave = m.AutoSave
		if m.AutoSave {
			m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
		}
		return m, nil, true
	case "alt+z", "alt+Z":
		m.toggleSoftWrap()
		return m, nil, true

	case "alt+[":
		m.adjustSidebarWidth(-2)
		return m, nil, true

	case "alt+]":
		m.adjustSidebarWidth(2)
		return m, nil, true

	case "alt+1":
		m.SidebarView = SidebarFiles
		return m, nil, true

	case "alt+2":
		m.SidebarView = SidebarGit
		if m.GitView != nil {
			if time.Since(m.GitView.LastRefresh) > 2*time.Second && !m.GitView.IsLoading {
				return m, m.triggerGitRefresh(), true
			}
		}
		return m, nil, true

	case "alt+3":
		m.SidebarView = SidebarSearch
		if m.SearchView != nil {
			m.SearchView.DoSearch()
		}
		return m, nil, true
	case "alt+b":
		m.openBranchSwitcher()
		return m, nil, true

	case "alt+g":
		nm, cmd := m.triggerAICommit()
		return nm, cmd, true

	case "alt+i":
		m.openAISettings()
		return m, nil, true

	case "ctrl+enter":
		nm, cmd := m.triggerGitCommit()
		return nm, cmd, true

	case "ctrl+shift+o", "ctrl+O":
		m.openGoToSymbol()
		return m, nil, true
	case "ctrl+o":
		m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)
		return m, nil, true

	case "ctrl+k":
		m.ctrlKPending = true
		m.StatusBar.ShowToast("Ctrl+K: Ctrl+0 fold all, Ctrl+J unfold all, Ctrl+K open folder", false, 2*time.Second)
		return m, nil, true

	case "ctrl+p":
		m.openQuickOpen()
		return m, nil, true

	case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
		nm, cmd := m.triggerFormatCurrentBuffer()
		return nm, cmd, true

	case "ctrl+shift+p", "ctrl+P", "f1":
		m.openCommandPalette()
		return m, nil, true

	case "ctrl+shift+f", "ctrl+F", "f4":
		m.openGlobalSearch()
		return m, nil, true

	case "ctrl+f":
		m.openFindBar()
		return m, nil, true

	case "ctrl+h":
		m.openReplaceBar()
		return m, nil, true

	case "ctrl+g":
		m.openGotoLine()
		return m, nil, true

	case "ctrl+m", "alt+m":
		if m.TabManager.ToggleActiveMarkdown() {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.MarkdownPreview {
				m.StatusBar.ShowToast("Mode: Markdown Preview (Glamour)", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Mode: Markdown Editor (Raw)", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		} else {
			m.StatusBar.ShowToast("Active tab is not a Markdown file", true, 2*time.Second)
		}
		return m, nil, true

	case "?":
		// Allow typing '?' in active editor buffer
		if m.Focus == FocusEditor {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
				break
			}
		}
		m.ShowHelp = !m.ShowHelp
		if m.ShowHelp && m.HelpDialog != nil {
			m.HelpDialog.Open(m.Height)
		}
		return m, nil, true
	}
	return m, nil, false
}
