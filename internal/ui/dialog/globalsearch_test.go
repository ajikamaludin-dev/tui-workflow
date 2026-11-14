package dialog

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGlobalSearch(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("alpha\nbeta gamma\ndelta"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("epsilon\nzeta beta\ntheta"), 0644)

	gs := NewGlobalSearch()
	gs.Open(tmpDir)

	gs.Filter.SetValue("beta")
	gs.PerformSearch()

	if len(gs.Matches) != 2 {
		t.Fatalf("expected 2 matches for 'beta', got %d", len(gs.Matches))
	}

	if gs.Matches[0].LineNumber != 2 || gs.Matches[1].LineNumber != 2 {
		t.Errorf("expected match on line 2, got %d and %d", gs.Matches[0].LineNumber, gs.Matches[1].LineNumber)
	}

	// Navigate and Enter
	path, line, col, closed, _ := gs.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !closed {
		t.Errorf("expected closed=true")
	}
	if path == "" || line != 2 || col <= 0 {
		t.Errorf("expected valid match selected, got path=%s, line=%d, col=%d", path, line, col)
	}
}
