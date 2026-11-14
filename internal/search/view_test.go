package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchViewSearchAndReplace(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "sample.txt")
	err := os.WriteFile(file1, []byte("hello world\nalpha beta\nhello tui"), 0644)
	if err != nil {
		t.Fatalf("Failed to create sample file: %v", err)
	}

	sv := NewSearchView(tempDir)
	sv.FindInput.SetValue("hello")
	sv.DoSearch()

	if sv.TotalFound != 2 {
		t.Errorf("Expected 2 matches for 'hello', got %d", sv.TotalFound)
	}
	if len(sv.Matches) != 2 {
		t.Fatalf("Expected 2 matches in slice, got %d", len(sv.Matches))
	}

	if sv.Matches[0].LineNumber != 1 || sv.Matches[1].LineNumber != 3 {
		t.Errorf("Expected line numbers 1 and 3, got %d and %d", sv.Matches[0].LineNumber, sv.Matches[1].LineNumber)
	}

	// Test Render
	rendered := sv.Render(30, 10, true)
	if !strings.Contains(rendered, "[Find]") || !strings.Contains(rendered, "[Replace All]") {
		t.Errorf("Expected action buttons in render output: %s", rendered)
	}
	if !strings.Contains(rendered, "sample.txt:1") {
		t.Errorf("Expected match row in render output: %s", rendered)
	}

	// Test Replace All
	sv.ReplaceInput.SetValue("greeting")
	replacedCount, err := sv.DoReplaceAll()
	if err != nil {
		t.Fatalf("DoReplaceAll failed: %v", err)
	}
	if replacedCount != 1 {
		t.Errorf("Expected 1 file modified, got %d", replacedCount)
	}

	// Verify content
	updated, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "greeting world") || !strings.Contains(string(updated), "greeting tui") {
		t.Errorf("Expected replacement in file content: %s", string(updated))
	}
}

func TestSearchViewNavigation(t *testing.T) {
	tempDir := t.TempDir()
	sv := NewSearchView(tempDir)

	// Tab navigates FocusField
	if sv.FocusField != FocusFindInput {
		t.Fatalf("Expected initial focus FocusFindInput")
	}

	sv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sv.FocusField != FocusReplaceInput {
		t.Errorf("Expected FocusReplaceInput after Tab, got %v", sv.FocusField)
	}

	sv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sv.FocusField != FocusMatchList {
		t.Errorf("Expected FocusMatchList after Tab, got %v", sv.FocusField)
	}

	sv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sv.FocusField != FocusFindInput {
		t.Errorf("Expected wrap to FocusFindInput after Tab, got %v", sv.FocusField)
	}
}
