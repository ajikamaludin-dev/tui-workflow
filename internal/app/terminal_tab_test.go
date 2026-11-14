package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui/dialog"
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

	// 4. Test Toggle Focus (F6)
	updM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF6})
	m = updM.(Model)
	if m.Focus != FocusSidebar {
		t.Errorf("expected FocusSidebar after F6, got %v", m.Focus)
	}

	// Toggle back
	updM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF6})
	m = updM.(Model)
	if m.Focus != FocusEditor {
		t.Errorf("expected FocusEditor after second F6, got %v", m.Focus)
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

func TestTerminalCloseConfirmationWithRunningProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true

	newM, cmd := m.openNewTerminalTab()
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from openNewTerminalTab")
	}
	m = newM
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil || activeTab.TerminalView == nil {
		t.Fatal("expected active terminal tab")
	}
	defer activeTab.TerminalView.Close()

	// Write command with child process
	_, err := activeTab.TerminalView.Write([]byte("sleep 2\n"))
	if err != nil {
		t.Fatalf("failed to write sleep command: %v", err)
	}

	// Wait up to 500ms for child process
	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		if activeTab.TerminalView.HasRunningProcess() {
			break
		}
	}

	if activeTab.TerminalView.HasRunningProcess() {
		// Attempt close tab
		m.attemptCloseTab(0)
		if !m.ConfirmDialog.Active || m.ConfirmDialog.Type != dialog.ConfirmCloseTerminal {
			t.Errorf("expected ConfirmCloseTerminal dialog active, got active=%v, type=%v", m.ConfirmDialog.Active, m.ConfirmDialog.Type)
		}
	}
}

func TestOpenTerminalKeybinding(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping pty test on windows")
	}

	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true

	// Send ctrl+\ key msg
	resM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	m = resM.(Model)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from ctrl+\\")
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Type != editor.TabTerminal {
		t.Fatalf("expected TabTerminal active after ctrl+\\, got %v", tab)
	}
	if tab.TerminalView != nil {
		defer tab.TerminalView.Close()
	}
}

func TestQuickOpenSwitchToCommandPalette(t *testing.T) {
	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true

	// Open Quick Open
	m.openQuickOpen()
	if m.QuickOpen == nil || !m.QuickOpen.Active {
		t.Fatal("expected QuickOpen to be active")
	}

	// Press '>'
	resM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	m = resM.(Model)

	if m.QuickOpen.Active {
		t.Errorf("expected QuickOpen to be closed after typing '>'")
	}
	if m.CommandPalette == nil || !m.CommandPalette.Active {
		t.Errorf("expected CommandPalette to be active after typing '>'")
	}
}

func TestTerminalMouseDoesNotHijackTabBarAndMenu(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	_ = os.WriteFile(testFile, []byte("package main\nfunc main() {}\n"), 0644)

	m, _ := NewModelWithPath(dir, false)
	m.Width = 100
	m.Height = 30
	m.Ready = true
	m.SidebarVisible = true
	m.SidebarWidth = 25

	// Tab 0: Regular file tab
	m.openFile(testFile)
	if m.TabManager.Count() != 1 {
		t.Fatalf("expected 1 tab, got %d", m.TabManager.Count())
	}

	// Tab 1: Terminal tab
	newM, cmd := m.openNewTerminalTab()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from openNewTerminalTab")
	}
	m = newM
	if m.TabManager.Count() != 2 {
		t.Fatalf("expected 2 tabs, got %d", m.TabManager.Count())
	}

	termTab := m.TabManager.Tabs[1]
	if termTab.TerminalView == nil {
		t.Fatal("expected TerminalView")
	}
	defer termTab.TerminalView.Close()

	// Enable inner mouse reporting on the virtual terminal (simulate top/btop)
	_, _ = termTab.TerminalView.WriteToVT([]byte("\x1b[?1000h\x1b[?1006h"))
	if !termTab.TerminalView.WantsMouse() {
		t.Fatal("expected WantsMouse to be true after VT mouse mode sequence")
	}

	m.TabManager.ActiveIndex = 1
	m.Focus = FocusEditor
	_ = m.View()

	if len(m.TabManager.ClickRegions) < 2 {
		t.Fatalf("expected at nil least 2 tab click regions, got %d", len(m.TabManager.ClickRegions))
	}

	// 1. Click on Tab 0 (Row 1) in Tab Bar
	tab0Region := m.TabManager.ClickRegions[0]
	clickX := tab0Region.StartX + 1 + m.SidebarWidth
	clickY := 1

	resM, _ := m.Update(tea.MouseMsg{X: clickX, Y: clickY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = resM.(Model)
	resM, _ = m.Update(tea.MouseMsg{X: clickX, Y: clickY, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	m = resM.(Model)

	if m.TabManager.ActiveIndex != 0 {
		t.Errorf("expected ActiveIndex to be 0 after clicking Tab 0, got %d", m.TabManager.ActiveIndex)
	}

	// Switch back to Terminal tab
	m.TabManager.ActiveIndex = 1
	_ = m.View()

	// 2. Click on Menu Bar (Row 0)
	cat0 := m.MenuBar.Categories[0]
	menuClickX := cat0.StartX + 1
	resM, _ = m.Update(tea.MouseMsg{X: menuClickX, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = resM.(Model)

	if !m.MenuBar.IsOpen() {
		t.Errorf("expected MenuBar to be open after clicking category %q", cat0.Title)
	}

	// 3. Click on another Menu Bar category (Row 0, e.g. Category 1)
	cat1 := m.MenuBar.Categories[1]
	menuClickX1 := cat1.StartX + 1
	resM, _ = m.Update(tea.MouseMsg{X: menuClickX1, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = resM.(Model)

	if !m.MenuBar.IsOpen() || m.MenuBar.ActiveMenu != 1 {
		t.Errorf("expected MenuBar category 1 to be active, got open=%v active=%d", m.MenuBar.IsOpen(), m.MenuBar.ActiveMenu)
	}

	// Close menu
	m.MenuBar.Close()

	// 4. Test Scroll Wheel on Tab Bar (Row 1) does not get swallowed by Terminal
	m.TabManager.ActiveIndex = 1
	resM, _ = m.Update(tea.MouseMsg{X: clickX, Y: 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = resM.(Model)
	// Still valid state, no panics or corruptions

	// 5. Test Click inside Terminal Area (Row 5, Col 30) IS forwarded to PTY/Terminal
	resM, _ = m.Update(tea.MouseMsg{X: 30, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = resM.(Model)
	resM, _ = m.Update(tea.MouseMsg{X: 30, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	m = resM.(Model)
	// Active tab remains terminal
	if m.TabManager.ActiveIndex != 1 {
		t.Errorf("expected ActiveIndex to remain 1 after clicking inside terminal area, got %d", m.TabManager.ActiveIndex)
	}
}

