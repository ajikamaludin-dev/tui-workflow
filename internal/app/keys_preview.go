package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"strconv"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
)

func (m Model) handleDiffKeys(msg tea.KeyPressMsg, tab *editor.Tab) (tea.Model, tea.Cmd, bool) {
	if tab != nil && tab.IsDiff && tab.DiffView != nil {
		switch ui.KeyName(msg) {
		case "ctrl+w":
			m.attemptCloseTab(m.TabManager.ActiveIndex)
			return m, nil, true
		case "ctrl+n":
			m.TabManager.NextTab()
			m.syncStatusBarWithActiveTab()
			return m, m.requestSessionPersist(), true
		case "up", "k":
			tab.DiffView.ScrollUp(1)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "down", "j":
			tab.DiffView.ScrollDown(1, m.Height-3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "pgup":
			tab.DiffView.ScrollUp(10)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "pgdown":
			tab.DiffView.ScrollDown(10, m.Height-3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "home":
			tab.DiffView.ScrollOffset = 0
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "n", "]":
			tab.DiffView.NextHunk(m.Height - 3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "N", "[":
			tab.DiffView.PrevHunk(m.Height - 3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "s", "+":
			return m, m.applyDiffHunkAction(tab, "stage"), true
		case "u", "-":
			return m, m.applyDiffHunkAction(tab, "unstage"), true
		case "x":
			return m, m.applyDiffHunkAction(tab, "discard"), true
		}
	}
	return m, nil, false
}

func (m Model) handleMarkdownPreviewKeys(msg tea.KeyPressMsg, tab *editor.Tab) (tea.Model, tea.Cmd, bool) {
	if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
		switch ui.KeyName(msg) {
		case "ctrl+w":
			m.attemptCloseTab(m.TabManager.ActiveIndex)
			return m, nil, true
		case "ctrl+n":
			m.TabManager.NextTab()
			m.syncStatusBarWithActiveTab()
			return m, m.requestSessionPersist(), true
		case "up", "k":
			tab.MarkdownViewer.ScrollUp(1)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "down", "j":
			tab.MarkdownViewer.ScrollDown(1, m.Height-3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "pgup":
			tab.MarkdownViewer.ScrollUp(10)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "pgdown":
			tab.MarkdownViewer.ScrollDown(10, m.Height-3)
			m.syncStatusBarWithActiveTab()
			return m, nil, true
		case "home":
			tab.MarkdownViewer.ScrollHome()
			m.syncStatusBarWithActiveTab()
			return m, nil, true

		case "ctrl+c":
			text := tab.MarkdownViewer.GetSelectedText()
			if text == "" {
				text = tab.MarkdownViewer.GetAllCleanText()
			}
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
			return m, nil, true

		case "ctrl+a":
			tab.MarkdownViewer.SelectAll()
			m.StatusBar.ShowToast("All preview text selected", false, 1*time.Second)
			return m, nil, true

		case "ctrl+v":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" && tab.Buffer != nil {
				tab.Buffer.InsertString(text)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Ditempel ke dokumen markdown", false, 2*time.Second)
			}
			return m, nil, true

		case "esc", "escape":
			if tab.MarkdownViewer.HasSelection {
				tab.MarkdownViewer.ClearSelection()
				return m, nil, true
			}
		}
	}
	return m, nil, false
}
