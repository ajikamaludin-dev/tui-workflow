package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTabManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-tab-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Normal text file
	filePath := filepath.Join(tempDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("line 1\nline 2"), 0644)

	tm := NewTabManager()
	tab, err := tm.OpenFile(filePath)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	if tab == nil || tm.Count() != 1 {
		t.Fatalf("Expected 1 open tab")
	}

	// Binary file guard
	binPath := filepath.Join(tempDir, "binary.bin")
	binContent := []byte{0x7F, 'E', 'L', 'F', 0x00, 0x01}
	_ = os.WriteFile(binPath, binContent, 0644)

	_, err = tm.OpenFile(binPath)
	if err == nil {
		t.Errorf("Expected error opening binary file, got nil")
	}

	// Auto-save
	tab.Buffer.InsertRune('X')
	if !tab.IsDirty() {
		t.Errorf("Tab should be dirty after insert")
	}

	saved, err := TriggerAutoSave(tm)
	if err != nil || saved != 1 {
		t.Errorf("Expected 1 file saved, got %d, err: %v", saved, err)
	}
	if tab.IsDirty() {
		t.Errorf("Tab should not be dirty after save")
	}
}

func TestTabBarSingleLineGuarantee(t *testing.T) {
	tm := NewTabManager()
	for i := 0; i < 20; i++ {
		tm.Tabs = append(tm.Tabs, &Tab{
			Path:   fmt.Sprintf("file_%d.txt", i),
			Buffer: NewBuffer("test"),
		})
	}
	tm.ActiveIndex = 3

	for _, w := range []int{40, 60, 80, 100, 120} {
		rendered := tm.RenderTabBar(w)
		lines := strings.Split(rendered, "\n")
		if len(lines) != 1 {
			t.Errorf("RenderTabBar at width %d produced %d lines (expected 1): %q", w, len(lines), rendered)
		}
	}
}

func TestTabBarScrollingAndTitleLimit(t *testing.T) {
	tm := NewTabManager()

	// 1. Title limit test
	longPath := "very_long_file_name_exceeding_sixteen_chars.go"
	tm.Tabs = append(tm.Tabs, &Tab{
		Path:   longPath,
		Name:   longPath,
		Buffer: NewBuffer("package main"),
	})
	tm.ActiveIndex = 0

	rendered := tm.RenderTabBar(60)
	if !strings.Contains(rendered, "..") {
		t.Errorf("Expected long tab title to be truncated with '..', got: %s", rendered)
	}

	// 2. Multi-tab scrolling
	for i := 1; i < 15; i++ {
		name := fmt.Sprintf("file_%02d.txt", i)
		tm.Tabs = append(tm.Tabs, &Tab{
			Path:   name,
			Name:   name,
			Buffer: NewBuffer("test"),
		})
	}

	// Render at narrow width (e.g. 40)
	_ = tm.RenderTabBar(40)
	if tm.ScrollRightStartX < 0 {
		t.Fatalf("Expected ScrollRightStartX to be active when tabs overflow")
	}

	// Clicking '>' scrolls right
	action, _ := tm.HandleTabClick(tm.ScrollRightStartX)
	if action != "scroll_right" {
		t.Errorf("Expected action 'scroll_right', got %q", action)
	}

	initialOffset := tm.TabScrollOffset
	tm.ScrollTabsRight()
	if tm.TabScrollOffset <= initialOffset {
		t.Errorf("Expected TabScrollOffset to increase after ScrollTabsRight()")
	}

	// Re-render, '<' button should now be visible
	_ = tm.RenderTabBar(40)
	if tm.ScrollLeftStartX < 0 {
		t.Fatalf("Expected ScrollLeftStartX to be active after scrolling right")
	}

	// Clicking '<' scrolls left
	action, _ = tm.HandleTabClick(tm.ScrollLeftStartX)
	if action != "scroll_left" {
		t.Errorf("Expected action 'scroll_left', got %q", action)
	}

	tm.ScrollTabsLeft()
	if tm.TabScrollOffset != initialOffset {
		t.Errorf("Expected TabScrollOffset to return to %d, got %d", initialOffset, tm.TabScrollOffset)
	}
}
