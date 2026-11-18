package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestDiffSeparatorDragChangesRatio(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-diff-drag-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.SidebarVisible = false

	m.TabManager.OpenDiff("drag.txt", "line 1\nline 2\n", "line 1\nline 2 changed\n", false, "Δ drag.txt")
	dv := m.activeDiffView()
	if dv == nil {
		t.Fatalf("expected active diff view")
	}
	if dv.SplitRatio != 0.5 {
		t.Fatalf("expected default 50/50 ratio, got %v", dv.SplitRatio)
	}

	sepX := m.diffSeparatorAbsX()
	if sepX < 0 {
		t.Fatalf("expected a valid separator column")
	}

	// Click on the separator starts a drag.
	nm, _ := m.Update(tea.MouseClickMsg{X: sepX, Y: 5, Button: tea.MouseLeft})
	m = nm.(Model)
	if !m.ResizingDiff {
		t.Fatalf("expected ResizingDiff after clicking separator at x=%d", sepX)
	}

	// Drag right to widen the left column.
	nm, _ = m.Update(tea.MouseMotionMsg{X: sepX + 20, Y: 5, Button: tea.MouseLeft})
	m = nm.(Model)
	if dv.SplitRatio <= 0.5 {
		t.Errorf("expected ratio > 0.5 after dragging right, got %v", dv.SplitRatio)
	}

	// Release ends the drag.
	nm, _ = m.Update(tea.MouseReleaseMsg{X: sepX + 20, Y: 5, Button: tea.MouseLeft})
	m = nm.(Model)
	if m.ResizingDiff {
		t.Errorf("expected ResizingDiff cleared after release")
	}
}

func TestDiffHitRequiresYInDiffBody(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, _ := os.MkdirTemp("", "tui-diff-hit-*")
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Width = 100
	m.Height = 30
	m.SidebarVisible = false
	m.TabManager.OpenDiff("hit.txt", "a\n", "b\n", false, "Δ hit.txt")

	sepX := m.diffSeparatorAbsX()
	if m.diffHit(sepX, 1) {
		t.Errorf("row 1 (tab bar) must not hit the diff separator")
	}
	if m.diffHit(sepX+1, 5) {
		t.Errorf("x away from the separator must not hit")
	}
	if !m.diffHit(sepX, 5) {
		t.Errorf("separator column in diff body should hit")
	}
}

func TestWatcherDirtyDeletedFileKeepsTabAndOpensDialogOnce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-dirty-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "dirty.txt")
	_ = os.WriteFile(filePath, []byte("isi awal\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	tab.Buffer.InsertString(" suntingan lokal")

	_ = os.Remove(filePath)
	nm, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = nm.(Model)

	if m.TabManager.Count() != 1 {
		t.Fatalf("dirty tab must stay open, got %d tabs", m.TabManager.Count())
	}
	if m.ConfirmDialog == nil || !m.ConfirmDialog.Active {
		t.Fatalf("expected save dialog for dirty deleted file")
	}
	if !tab.DeletedFromDisk {
		t.Errorf("expected DeletedFromDisk flag set")
	}
	if m.ClosingTabIdx < 0 {
		t.Errorf("expected ClosingTabIdx to reference the dirty tab")
	}

	// A subsequent tick while the dialog is open must not reconfigure it.
	selected := m.ConfirmDialog.Selected
	nm, _ = m.Update(FileWatchTickMsg(time.Now()))
	m = nm.(Model)
	if m.ConfirmDialog.Selected != selected {
		t.Errorf("dialog selection should not be reinitialized on repeated tick")
	}
}

func TestWatcherPinnedDeletedFileKeepsTab(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-pin-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "pinned.txt")
	_ = os.WriteFile(filePath, []byte("isi\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	tab.IsPinned = true

	_ = os.Remove(filePath)
	nm, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = nm.(Model)

	if m.TabManager.Count() != 1 {
		t.Fatalf("pinned tab must stay open even when file is deleted, got %d tabs", m.TabManager.Count())
	}
	if m.ConfirmDialog != nil && m.ConfirmDialog.Active {
		t.Errorf("pinned tab must not raise a save dialog")
	}
}
