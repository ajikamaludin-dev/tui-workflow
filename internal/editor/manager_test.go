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

func TestTabManagerCloseBatch(t *testing.T) {
	createManager := func() *TabManager {
		tm := NewTabManager()
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("file_%d.txt", i)
			tm.Tabs = append(tm.Tabs, &Tab{
				Path:   name,
				Name:   name,
				Buffer: NewBuffer("content"),
			})
		}
		tm.ActiveIndex = 2
		return tm
	}

	// 1. CloseTabsToRight from index 2 -> tabs 0, 1, 2 remain
	tm := createManager()
	tm.CloseTabsToRight(2)
	if len(tm.Tabs) != 3 {
		t.Fatalf("Expected 3 tabs after CloseTabsToRight(2), got %d", len(tm.Tabs))
	}
	if tm.Tabs[2].Name != "file_2.txt" {
		t.Errorf("Expected last tab to be file_2.txt, got %s", tm.Tabs[2].Name)
	}

	// 2. CloseTabsToLeft from index 2 -> tabs 2, 3, 4 remain (3 tabs)
	tm = createManager()
	tm.CloseTabsToLeft(2)
	if len(tm.Tabs) != 3 {
		t.Fatalf("Expected 3 tabs after CloseTabsToLeft(2), got %d", len(tm.Tabs))
	}
	if tm.Tabs[0].Name != "file_2.txt" {
		t.Errorf("Expected first tab to be file_2.txt, got %s", tm.Tabs[0].Name)
	}
	if tm.ActiveIndex != 0 {
		t.Errorf("Expected ActiveIndex 0, got %d", tm.ActiveIndex)
	}

	// 3. FindTabAt
	tm = createManager()
	_ = tm.RenderTabBar(100)
	if len(tm.ClickRegions) > 0 {
		firstTab := tm.ClickRegions[0]
		found := tm.FindTabAt(firstTab.StartX)
		if found != firstTab.TabIdx {
			t.Errorf("Expected FindTabAt to return %d, got %d", firstTab.TabIdx, found)
		}
		outOfBounds := tm.FindTabAt(999)
		if outOfBounds != -1 {
			t.Errorf("Expected FindTabAt(999) to return -1, got %d", outOfBounds)
		}
	}
}


func TestTabManager_MoveAndPin(t *testing.T) {
	tm := NewTabManager()
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("file_%d.txt", i)
		tm.Tabs = append(tm.Tabs, &Tab{
			Path:   name,
			Name:   name,
			Buffer: NewBuffer("content"),
		})
	}
	tm.ActiveIndex = 2 // file_2.txt

	// 1. Pin tab 2 (file_2.txt) -> moves to index 0 (first pinned tab)
	if !tm.TogglePinTab(2) {
		t.Fatalf("TogglePinTab(2) failed")
	}
	if tm.PinnedCount() != 1 {
		t.Errorf("Expected 1 pinned tab, got %d", tm.PinnedCount())
	}
	if !tm.Tabs[0].IsPinned || tm.Tabs[0].Name != "file_2.txt" {
		t.Errorf("Expected tab 0 to be pinned file_2.txt, got %s (pinned=%v)", tm.Tabs[0].Name, tm.Tabs[0].IsPinned)
	}
	if tm.ActiveIndex != 0 {
		t.Errorf("Expected ActiveIndex to follow moved tab to 0, got %d", tm.ActiveIndex)
	}

	// 2. RenderTabBar checks: pinned tab has pin icon and CloseX == -1
	_ = tm.RenderTabBar(100)
	if len(tm.ClickRegions) < 4 {
		t.Fatalf("Expected at least 4 click regions, got %d", len(tm.ClickRegions))
	}
	reg0 := tm.ClickRegions[0]
	if reg0.CloseX != -1 {
		t.Errorf("Expected pinned tab CloseX to be -1, got %d", reg0.CloseX)
	}
	action, tabIdx := tm.HandleTabClick(reg0.StartX + 1)
	if action != "activate" || tabIdx != 0 {
		t.Errorf("Expected 'activate' for pinned tab click, got action=%q, tabIdx=%d", action, tabIdx)
	}

	// 3. MoveTab within unpinned group (indices 1, 2, 3: file_0, file_1, file_3)
	// Move tab at 3 (file_3) to 1 -> order becomes [file_2(pinned), file_3, file_0, file_1]
	if !tm.MoveTab(3, 1) {
		t.Fatalf("MoveTab(3, 1) failed")
	}
	if tm.Tabs[1].Name != "file_3.txt" {
		t.Errorf("Expected index 1 to be file_3.txt, got %s", tm.Tabs[1].Name)
	}

	// 4. MoveTab boundary protection: moving unpinned tab (index 2) to 0 clamps to 1
	_ = tm.MoveTab(2, 0)
	if tm.Tabs[0].Name != "file_2.txt" || !tm.Tabs[0].IsPinned {
		t.Errorf("Pinned tab at 0 should remain intact after unpinned move attempt")
	}

	// 5. Batch close protection: CloseOtherTabs from index 2 (unpinned) must keep pinned tab 0!
	tm.CloseOtherTabs(2)
	if len(tm.Tabs) != 2 {
		t.Fatalf("Expected 2 tabs (pinned tab + kept tab), got %d", len(tm.Tabs))
	}
	if !tm.Tabs[0].IsPinned {
		t.Errorf("Tab 0 should still be pinned")
	}

	// 6. CloseAllTabs must preserve pinned tab!
	tm.CloseAllTabs()
	if len(tm.Tabs) != 1 || !tm.Tabs[0].IsPinned {
		t.Errorf("CloseAllTabs should preserve pinned tab, remaining tabs=%d", len(tm.Tabs))
	}

	// 7. Unpin tab 0 -> becomes unpinned
	if !tm.TogglePinTab(0) {
		t.Fatalf("TogglePinTab(0) unpin failed")
	}
	if tm.PinnedCount() != 0 {
		t.Errorf("Expected 0 pinned tabs after unpin, got %d", tm.PinnedCount())
	}
	tm.CloseAllTabs()
	if len(tm.Tabs) != 0 {
		t.Errorf("Expected 0 tabs after closing all when unpinned, got %d", len(tm.Tabs))
	}
}
