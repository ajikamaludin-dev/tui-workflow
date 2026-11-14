package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"strings"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
)

func (m *Model) handleGoToSymbolKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch ui.KeyName(msg) {
	case "esc":
		m.closeGoToSymbol()
		return m, nil
	case "up":
		if m.GoToSymbolSelected > 0 {
			m.GoToSymbolSelected--
			if m.GoToSymbolSelected < m.GoToSymbolScroll {
				m.GoToSymbolScroll = m.GoToSymbolSelected
			}
		}
		return m, nil
	case "down":
		if m.GoToSymbolSelected < len(m.GoToSymbolResults)-1 {
			m.GoToSymbolSelected++
			maxRows := 12
			if m.GoToSymbolSelected >= m.GoToSymbolScroll+maxRows {
				m.GoToSymbolScroll = m.GoToSymbolSelected - maxRows + 1
			}
		}
		return m, nil
	case "enter":
		if len(m.GoToSymbolResults) > 0 && m.GoToSymbolSelected >= 0 && m.GoToSymbolSelected < len(m.GoToSymbolResults) {
			sel := m.GoToSymbolResults[m.GoToSymbolSelected]
			m.closeGoToSymbol()
			m.openFileAt(sel.FilePath, sel.Line, sel.Col)
			m.Focus = FocusEditor
			m.syncStatusBarWithActiveTab()
		} else {
			m.closeGoToSymbol()
		}
		return m, nil
	case "backspace":
		if len(m.GoToSymbolQuery) > 0 {
			r := []rune(m.GoToSymbolQuery)
			m.GoToSymbolQuery = string(r[:len(r)-1])
			m.updateGoToSymbolResults()
		}
		return m, nil
	}
	if len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModAlt) {
		// v2: Text is populated for printable characters, including Space (" ")
		m.GoToSymbolQuery += msg.Text
		m.updateGoToSymbolResults()
		return m, nil
	}
	return m, nil
}

func (m Model) handleGoToSymbolPending(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// Go To Symbol modal handling (Ctrl+Shift+O)
	if m.GoToSymbolActive {
		nm, cmd := m.handleGoToSymbolKey(msg)
		return nm, cmd, true
	}
	return m, nil, false
}

func (m Model) handleSearchKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// 0. Find Bar Handling if active
	if m.FindBar != nil && m.FindBar.Active {
		tab := m.TabManager.ActiveTab()
		action, closed, cmd := m.FindBar.Update(msg)
		if closed {
			if tab != nil && tab.Buffer != nil {
				tab.Buffer.ClearSearch()
			}
		} else if tab != nil && tab.Buffer != nil {
			switch action {
			case "change":
				tab.Buffer.SetSearchQuery(m.FindBar.Input.Value())
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "next":
				tab.Buffer.FindNext()
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "prev":
				tab.Buffer.FindPrev()
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
			case "replace_one":
				if tab.Buffer.ReplaceCurrent(m.FindBar.ReplaceInput.Value()) {
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("1 match replaced", false, 1*time.Second)
				}
			case "replace_all":
				count := tab.Buffer.ReplaceAll(m.FindBar.Input.Value(), m.FindBar.ReplaceInput.Value())
				cur, tot := tab.Buffer.MatchInfo()
				m.FindBar.SetMatches(cur, tot)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast(fmt.Sprintf("%d matches replaced", count), false, 2*time.Second)
			}
		}
		return m, cmd, true
	}
	// 0.5. Quick Open Handling if active
	if m.QuickOpen != nil && m.QuickOpen.Active {
		targetTabIdx, targetPath, closed, cmd := m.QuickOpen.Update(msg)
		if m.QuickOpen.Active && strings.HasPrefix(m.QuickOpen.Filter.Value(), ">") {
			initialQuery := strings.TrimPrefix(m.QuickOpen.Filter.Value(), ">")
			initialQuery = strings.TrimLeft(initialQuery, " ")
			m.QuickOpen.Close()
			m.openCommandPalette()
			if initialQuery != "" && m.CommandPalette != nil {
				m.CommandPalette.Filter.SetValue(initialQuery)
				m.CommandPalette.ApplyFilter()
			}
			return m, nil, true
		}
		if closed {
			if targetTabIdx >= 0 && targetTabIdx < m.TabManager.Count() {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.TabManager.ActiveIndex = targetTabIdx
				m.syncStatusBarWithActiveTab()
			} else if targetPath != "" {
				if m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.openFile(targetPath)
				m.Focus = FocusEditor
			}
		}
		return m, cmd, true
	}
	// 0.6. Command Palette Handling if active
	if m.CommandPalette != nil && m.CommandPalette.Active {
		actionID, closed, cmd := m.CommandPalette.Update(msg)
		if closed && actionID != "" {
			menuCmd := m.handleMenuAction(actionID)
			return m, tea.Batch(cmd, menuCmd), true
		}
		return m, cmd, true
	}
	// 0.7. Global Search Handling if active
	if m.GlobalSearch != nil && m.GlobalSearch.Active {
		filePath, line, col, closed, cmd := m.GlobalSearch.Update(msg)
		if closed && filePath != "" {
			m.openFileAt(filePath, line, col)
			m.Focus = FocusEditor
		}
		return m, cmd, true
	}
	return m, nil, false
}
