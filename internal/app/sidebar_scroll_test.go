package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSidebarMouseWheelScroll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-wheel-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create many files so explorer and git tree have plenty of lines
	for i := 0; i < 30; i++ {
		p := filepath.Join(tempDir, filepath.Clean(string(rune('a'+(i%26)))+string(rune('0'+(i/26)))+".txt"))
		_ = os.WriteFile(p, []byte("test"), 0644)
	}

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Width = 100
	m.Height = 24
	m.SidebarVisible = true
	m.SidebarWidth = 28
	m.SidebarView = SidebarFiles

	// 1. Test Wheel Down on Files view
	for i := 0; i < 15; i++ {
		m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	}
	_ = m.View()

	if m.Explorer.ScrollOffset <= 0 {
		t.Errorf("Expected Explorer.ScrollOffset > 0 after multiple WheelDown, got %d", m.Explorer.ScrollOffset)
	}

	// Test Wheel Up on Files view all the way back to top
	for i := 0; i < 20; i++ {
		m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelUp})
	}
	_ = m.View()

	if m.Explorer.ScrollOffset != 0 {
		t.Errorf("Expected Explorer.ScrollOffset == 0 after scrolling back up, got %d", m.Explorer.ScrollOffset)
	}

	// 2. Test Wheel on Git view
	m.SidebarView = SidebarGit
	if m.GitView != nil {
		m.GitView.TotalLines = 50
		m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})

		if m.GitView.ScrollOffset <= 0 {
			t.Errorf("Expected GitView.ScrollOffset > 0 after WheelDown, got %d", m.GitView.ScrollOffset)
		}

		m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelUp})
	}
}
