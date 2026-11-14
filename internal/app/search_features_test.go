package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tui-workflow/internal/search"
)

func TestSearchFeaturesIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "alpha.go")
	fileB := filepath.Join(tmpDir, "beta.go")

	_ = os.WriteFile(fileA, []byte("package main\n\nfunc Alpha() string {\n\treturn \"find_me_token\"\n}\n"), 0644)
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc Beta() {\n\t// find_me_token here too\n}\n"), 0644)

	m, err := NewModelWithPath(tmpDir, false)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30

	// 1. Test Quick Open (Ctrl+P)
	m.openQuickOpen()
	if m.QuickOpen == nil || !m.QuickOpen.Active {
		t.Fatalf("expected QuickOpen to be active")
	}

	// Filter for beta
	m.QuickOpen.Filter.SetValue("beta")
	m.QuickOpen.ApplyFilter()
	newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newM.(Model)

	if m.QuickOpen.Active {
		t.Errorf("expected QuickOpen to be closed after enter")
	}
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil || activeTab.Name != "beta.go" {
		t.Fatalf("expected beta.go open in active tab, got %+v", activeTab)
	}

	// 2. Test In-Editor Find (Ctrl+F)
	m.openFindBar()
	if m.FindBar == nil || !m.FindBar.Active {
		t.Fatalf("expected FindBar to be active")
	}

	// Simulate typing search query into FindBar
	for _, r := range "find_me" {
		newM, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = newM.(Model)
	}

	cur, tot := activeTab.Buffer.MatchInfo()
	if tot != 1 || cur != 1 {
		t.Errorf("expected 1 match in beta.go, got %d (active: %d)", tot, cur)
	}

	// Close FindBar with Esc
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = newM.(Model)
	if m.FindBar.Active {
		t.Errorf("expected FindBar to be closed on Esc")
	}
	if activeTab.Buffer.SearchQuery != "" {
		t.Errorf("expected buffer search query cleared")
	}

	// 3. Test Sidebar Search (F4) — replaces removed Global Search modal
	m.openSearchSidebar()
	if m.SidebarView != SidebarSearch {
		t.Fatalf("expected sidebar to switch to Search")
	}
	if m.SearchView == nil {
		t.Fatalf("expected SearchView to exist")
	}

	m.SearchView.FindInput.SetValue("find_me_token")
	m.SearchView.DoSearch()
	if len(m.SearchView.Matches) != 2 {
		t.Fatalf("expected 2 sidebar search matches, got %d", len(m.SearchView.Matches))
	}

	m.SearchView.FocusField = search.FocusMatchList
	m.SearchView.Selected = 0
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newM.(Model)
	activeTab = m.TabManager.ActiveTab()
	if activeTab == nil {
		t.Fatalf("expected active tab after search enter")
	}
	if activeTab.Buffer.CursorRow <= 0 {
		t.Errorf("expected cursor row to jump to match, got %d", activeTab.Buffer.CursorRow)
	}

	// 4. Test Command Palette
	m.openCommandPalette()
	if m.CommandPalette == nil || !m.CommandPalette.Active {
		t.Fatalf("expected CommandPalette to be active")
	}

	m.CommandPalette.Filter.SetValue("Toggle Sidebar")
	m.CommandPalette.ApplyFilter()
	wasSidebarVisible := m.SidebarVisible
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newM.(Model)

	if m.CommandPalette.Active {
		t.Errorf("expected CommandPalette to be closed after enter")
	}
	if m.SidebarVisible == wasSidebarVisible {
		t.Errorf("expected sidebar visibility toggled by command palette")
	}

	// 5. Test Floating Modal Overlay & 80% Width
	m.Width = 100
	m.Height = 30
	m.openQuickOpen()
	expectedWidth := int(float64(m.Width) * 0.8)
	if m.QuickOpen.Width != expectedWidth {
		t.Errorf("expected QuickOpen width to be 80%% (%d), got %d", expectedWidth, m.QuickOpen.Width)
	}

	viewStr := m.View().Content
	// Menu bar at top should still be preserved in the floating overlay
	if !strings.Contains(viewStr, "File") {
		t.Errorf("expected floating view to preserve top menu File in background")
	}
	if !strings.Contains(viewStr, "Search") {
		t.Errorf("expected floating view to preserve top menu Search in background")
	}

	// Close QuickOpen
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = newM.(Model)

	// 6. Test F4 opens Search sidebar
	m.SidebarView = SidebarFiles
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF4})
	m = newM.(Model)
	if m.SidebarView != SidebarSearch {
		t.Errorf("expected F4 to open Search sidebar, got %v", m.SidebarView)
	}
}
