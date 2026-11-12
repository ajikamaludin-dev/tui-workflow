package dialog

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQuickOpen(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0644)
	subDir := filepath.Join(tmpDir, "pkg")
	_ = os.Mkdir(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "helper.go"), []byte("package pkg"), 0644)

	openTabs := []SwitcherItem{
		{Index: 0, Title: "main.go", Path: filepath.Join(tmpDir, "main.go"), IsDirty: true},
	}

	qo := NewQuickOpen()
	qo.Open(openTabs, 0, tmpDir)

	if len(qo.AllItems) < 3 {
		t.Fatalf("expected at least 3 items in QuickOpen, got %d", len(qo.AllItems))
	}

	// Filter for helper
	qo.Filter.SetValue("helper")
	qo.applyFilter()
	if len(qo.Filtered) != 1 {
		t.Fatalf("expected 1 match for 'helper', got %d", len(qo.Filtered))
	}
	if qo.Filtered[0].Title != "helper.go" {
		t.Errorf("expected helper.go, got %s", qo.Filtered[0].Title)
	}

	// Select and Enter
	tabIdx, path, closed, _ := qo.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !closed {
		t.Errorf("expected closed=true on enter")
	}
	if tabIdx != -1 {
		t.Errorf("expected tabIdx=-1 for unopened file, got %d", tabIdx)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected abs path, got %s", path)
	}
}
