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
