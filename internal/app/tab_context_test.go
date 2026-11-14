package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabContextMenu(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-tab-context-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Width = 100
	m.Height = 30
	m.SidebarWidth = 28
	m.SidebarVisible = true

	// Create 4 files and open them
	for i := 0; i < 4; i++ {
		p := filepath.Join(tempDir, string(rune('a'+i))+".txt")
		_ = os.WriteFile(p, []byte("content"), 0644)
		_, err := m.TabManager.OpenFile(p)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", p, err)
		}
	}
	if m.TabManager.Count() != 4 {
		t.Fatalf("Expected 4 tabs, got %d", m.TabManager.Count())
	}

	// Render tab bar to compute ClickRegions
	editorStartX := m.SidebarWidth
	editorWidth := m.Width - editorStartX
	_ = m.TabManager.RenderTabBar(editorWidth)

	if len(m.TabManager.ClickRegions) < 4 {
		t.Fatalf("Expected at least 4 click regions, got %d", len(m.TabManager.ClickRegions))
	}

	// 1. Right-click on Tab 1
	tab1Region := m.TabManager.ClickRegions[1]
	clickX := editorStartX + tab1Region.StartX + 1
	newM, _ := m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      1,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if m.ContextMenu == nil || !m.ContextMenu.Active {
		t.Fatalf("Expected context menu to be active after right-clicking tab")
	}
	if m.ContextMenuTabIdx != 1 {
		t.Errorf("Expected ContextMenuTabIdx to be 1, got %d", m.ContextMenuTabIdx)
	}
	if len(m.ContextMenu.Items) != 6 {
		t.Fatalf("Expected 6 context menu items for tab, got %d", len(m.ContextMenu.Items))
	}

	// Verify items: Close, Close Others, Close to the Right, Close to the Left, Close All
	renderedMenu := m.ContextMenu.Render()
	if !strings.Contains(renderedMenu, "Close to the Right") {
		t.Errorf("Rendered context menu missing 'Close to the Right': %s", renderedMenu)
	}

	// 2. Execute "tab_close_right" from tab 1 (index 1) -> tabs 2 and 3 should close, leaving 2 tabs (0 and 1)
	m.handleContextMenuAction("tab_close_right")
	if m.TabManager.Count() != 2 {
		t.Fatalf("Expected 2 tabs after tab_close_right, got %d", m.TabManager.Count())
	}
	if m.TabManager.Tabs[0].Name != "a.txt" || m.TabManager.Tabs[1].Name != "b.txt" {
		t.Errorf("Expected remaining tabs to be a.txt and b.txt, got %s and %s", m.TabManager.Tabs[0].Name, m.TabManager.Tabs[1].Name)
	}

	// 3. Test "tab_close_all"
	m.handleContextMenuAction("tab_close_all")
	if m.TabManager.Count() != 0 {
		t.Errorf("Expected 0 tabs after tab_close_all, got %d", m.TabManager.Count())
	}
}

func TestSidebarRefreshButton(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-refresh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Width = 100
	m.Height = 30
	m.SidebarWidth = 28
	m.SidebarVisible = true
	m.SidebarView = SidebarFiles

	// 1. Click refresh button in Row 2 when Files tab is active
	// Refresh button is located on Row 2 at the top inside the active tab
	refreshClickX := 5
	newM, _ := m.Update(tea.MouseMsg{
		X:      refreshClickX,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if !strings.Contains(m.StatusBar.ToastMsg, "File explorer reloaded") {
		t.Errorf("Expected toast 'File explorer reloaded', got %q", m.StatusBar.ToastMsg)
	}

	// 2. Click refresh button when Git tab is active
	m.SidebarView = SidebarGit
	newM, _ = m.Update(tea.MouseMsg{
		X:      refreshClickX,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	// Since tempDir is not a git repo, gitView is nil or it doesn't crash
	// If repo initialized, toast says "Git status diperbarui"
	// 3. Test Command Palette files.refresh action
	m.handleMenuAction("files.refresh")
	if !strings.Contains(m.StatusBar.ToastMsg, "File explorer reloaded") {
		t.Errorf("Expected toast 'File explorer reloaded' from command palette, got %q", m.StatusBar.ToastMsg)
	}
}

func TestEditorQuestionMarkTyping(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-qm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("Hello"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.openFile(filePath)
	m.Focus = FocusEditor
	m.ShowHelp = false

	// Type '?' in active editor buffer
	newM, _ := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'?'},
	})
	m = newM.(Model)

	// Help modal should NOT be shown
	if m.ShowHelp {
		t.Errorf("ShowHelp should be false when typing '?' in editor buffer")
	}

	// Active buffer should now contain '?'
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil || activeTab.Buffer == nil {
		t.Fatalf("Active tab buffer is nil")
	}
	lineContent := string(activeTab.Buffer.Lines[0])
	if !strings.Contains(lineContent, "?") {
		t.Errorf("Expected editor buffer to contain '?', got %q", lineContent)
	}

	// When focus is on Sidebar, '?' SHOULD toggle Help
	m.Focus = FocusSidebar
	newM, _ = m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'?'},
	})
	m = newM.(Model)
	if !m.ShowHelp {
		t.Errorf("Expected ShowHelp=true when pressing '?' outside editor")
	}
}

func TestSidebarFooterTabsPinnedToBottom(t *testing.T) {
	m := NewModel()
	m.RootDir = "."
	m.Width = 100
	m.Height = 30
	m.SidebarWidth = 28
	m.SidebarVisible = true

	// Test across all 3 sidebar views
	views := []SidebarView{SidebarFiles, SidebarGit, SidebarSearch}
	for _, v := range views {
		m.SidebarView = v
		sidebarOutput := m.renderSidebar(m.SidebarWidth, m.Height-2)
		lines := strings.Split(sidebarOutput, "\n")

		expectedHeight := m.Height - 2
		if len(lines) != expectedHeight {
			t.Errorf("Sidebar view %v: expected %d lines, got %d lines", v, expectedHeight, len(lines))
		}

		// The last line of the sidebar MUST contain the footer tabs
		lastLine := lines[len(lines)-1]
		if !strings.Contains(lastLine, "Files") || !strings.Contains(lastLine, "Git") || !strings.Contains(lastLine, "Search") {
			t.Errorf("Sidebar view %v: expected footer tabs on last line, got %q", v, lastLine)
		}
	}
}


func TestTabDragAndPin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-drag-pin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Width = 100
	m.Height = 30
	m.SidebarWidth = 28
	m.SidebarVisible = true

	// Open 3 files: a.txt, b.txt, c.txt
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(tempDir, name)
		_ = os.WriteFile(p, []byte("content"), 0644)
		_, _ = m.TabManager.OpenFile(p)
	}
	if m.TabManager.Count() != 3 {
		t.Fatalf("Expected 3 tabs, got %d", m.TabManager.Count())
	}

	// 1. Test Alt+P shortcut on active tab (c.txt, index 2)
	m.TabManager.ActiveIndex = 2
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true})
	m = newM.(Model)

	if !m.TabManager.Tabs[0].IsPinned || m.TabManager.Tabs[0].Name != "c.txt" {
		t.Errorf("Expected tab 0 to be pinned c.txt, got %s (pinned=%v)", m.TabManager.Tabs[0].Name, m.TabManager.Tabs[0].IsPinned)
	}
	if !strings.Contains(m.StatusBar.ToastMsg, "Tab pinned") {
		t.Errorf("Expected toast 'Tab pinned', got %q", m.StatusBar.ToastMsg)
	}

	// 2. Attempt to close pinned tab -> must be blocked
	m.attemptCloseTab(0)
	if m.TabManager.Count() != 3 {
		t.Errorf("Pinned tab should not be closed, remaining: %d", m.TabManager.Count())
	}
	if !strings.Contains(m.StatusBar.ToastMsg, "Tab is pinned") {
		t.Errorf("Expected warning toast for pinned tab close attempt, got %q", m.StatusBar.ToastMsg)
	}

	// 3. Render tab bar and perform mouse drag: drag unpinned tab 2 (b.txt) to tab 1 (a.txt)
	editorStartX := m.SidebarWidth
	editorWidth := m.Width - editorStartX
	_ = m.TabManager.RenderTabBar(editorWidth)

	if len(m.TabManager.ClickRegions) < 3 {
		t.Fatalf("Expected at least 3 click regions, got %d", len(m.TabManager.ClickRegions))
	}

	reg1 := m.TabManager.ClickRegions[1] // a.txt
	reg2 := m.TabManager.ClickRegions[2] // b.txt

	// Mouse Press on tab 2
	newM, _ = m.Update(tea.MouseMsg{
		X:      editorStartX + reg2.StartX + 1,
		Y:      1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)
	if m.DraggingTabIdx != 2 {
		t.Errorf("Expected DraggingTabIdx=2, got %d", m.DraggingTabIdx)
	}

	// Mouse Motion to tab 1
	newM, _ = m.Update(tea.MouseMsg{
		X:      editorStartX + reg1.StartX + 1,
		Y:      1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	m = newM.(Model)
	if m.DraggingTabIdx != 1 {
		t.Errorf("Expected DraggingTabIdx to become 1 after drag, got %d", m.DraggingTabIdx)
	}
	if m.TabManager.Tabs[1].Name != "b.txt" {
		t.Errorf("Expected tab 1 to be b.txt after reordering, got %s", m.TabManager.Tabs[1].Name)
	}

	// Mouse Release ends drag
	newM, _ = m.Update(tea.MouseMsg{
		X:      editorStartX + reg1.StartX + 1,
		Y:      1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	})
	m = newM.(Model)
	if m.DraggingTabIdx != -1 || m.IsDraggingTab {
		t.Errorf("Expected drag state reset after release, DraggingTabIdx=%d, IsDraggingTab=%v", m.DraggingTabIdx, m.IsDraggingTab)
	}

	// 4. Test unpinning via context menu
	m.ContextMenuTabIdx = 0
	m.handleContextMenuAction("tab_toggle_pin")
	if m.TabManager.Tabs[0].IsPinned {
		t.Errorf("Expected tab 0 to be unpinned after tab_toggle_pin")
	}
	if !strings.Contains(m.StatusBar.ToastMsg, "Tab unpinned") {
		t.Errorf("Expected toast 'Tab unpinned', got %q", m.StatusBar.ToastMsg)
	}
}
