package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/editor"
)

func TestTerminalTabIntegration(t *testing.T) {
	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true
	m.SidebarVisible = true
	m.SidebarWidth = 25

	// 1. Open new terminal tab via openNewTerminalTab
	var cmd tea.Cmd
	newM, cmd := m.openNewTerminalTab()
	m = newM
	if cmd == nil {
		t.Fatal("expected waitForTerminalOutput cmd from openNewTerminalTab")
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Type != editor.TabTerminal {
		t.Fatalf("expected active tab to be TabTerminal, got %v", tab)
	}
	if tab.TerminalView == nil {
		t.Fatal("expected tab.TerminalView to be non-nil")
	}
	defer tab.TerminalView.Close()

	if m.Focus != FocusEditor {
		t.Errorf("expected FocusEditor, got %v", m.Focus)
	}

	// 2. Status bar should show Terminal info
	m.syncStatusBarWithActiveTab()
	if m.StatusBar.FileSyntax != "Terminal (PTY)" {
		t.Errorf("expected FileSyntax 'Terminal (PTY)', got %q", m.StatusBar.FileSyntax)
	}

	// 3. Test typing into terminal via Update
	for _, r := range "echo TEST_OK" {
		updM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updM.(Model)
	}
	updM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updM.(Model)

	// Wait for terminal output
	time.Sleep(150 * time.Millisecond)

	// Render view and check zero overflow
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > m.Height {
		t.Fatalf("rendered view height %d exceeds terminal height %d", len(lines), m.Height)
	}

	// 4. Test Toggle Focus (Ctrl+\)
	updM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	m = updM.(Model)
	if m.Focus != FocusSidebar {
		t.Errorf("expected FocusSidebar after Ctrl+\\, got %v", m.Focus)
	}

	// Toggle back
	updM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	m = updM.(Model)
	if m.Focus != FocusEditor {
		t.Errorf("expected FocusEditor after second Ctrl+\\, got %v", m.Focus)
	}

	// 5. Test Close Terminal Tab (Alt+W)
	updM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	m = updM.(Model)
	if m.TabManager.Count() != 0 {
		t.Errorf("expected 0 tabs after Alt+W, got %d", m.TabManager.Count())
	}
}

func TestTerminalMenuAndCommandPalette(t *testing.T) {
	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true

	// Check menu item exists in File menu
	var foundMenu bool
	for _, cat := range m.MenuBar.Categories {
		if cat.Title == "File" {
			for _, item := range cat.Items {
				if item.ActionID == "terminal.new" {
					foundMenu = true
					break
				}
			}
		}
	}
	if !foundMenu {
		t.Error("expected 'terminal.new' in File menu items")
	}

	// Execute menu action
	cmd := m.handleMenuAction("terminal.new")
	if cmd == nil {
		t.Fatal("expected cmd from handleMenuAction('terminal.new')")
	}
	if m.TabManager.Count() != 1 {
		t.Fatalf("expected 1 tab opened, got %d", m.TabManager.Count())
	}
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil || activeTab.Type != editor.TabTerminal {
		t.Fatalf("expected TabTerminal, got %v", activeTab)
	}
	defer activeTab.TerminalView.Close()

	// Check help modal includes terminal shortcuts
	help := renderHelpModal()
	if !strings.Contains(help, "Terminal & Shell (PTY)") {
		t.Error("expected Help Modal to contain 'Terminal & Shell (PTY)'")
	}
}
