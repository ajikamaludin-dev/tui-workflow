package dialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBranchSwitcherOpenAndFilter(t *testing.T) {
	bs := NewBranchSwitcher()
	branches := []string{"main", "feature/login", "feature/dashboard", "bugfix/issue-1"}
	current := "main"

	bs.Open(branches, current)
	if !bs.Active {
		t.Fatalf("Expected BranchSwitcher to be active")
	}
	if len(bs.Filtered) != 4 {
		t.Errorf("Expected 4 branches, got %d", len(bs.Filtered))
	}
	if bs.Selected != 0 {
		t.Errorf("Expected current branch 'main' to be selected (index 0), got %d", bs.Selected)
	}

	// Filter by "feat"
	bs.Filter.SetValue("feat")
	bs.applyFilter()
	if len(bs.Filtered) != 2 {
		t.Errorf("Expected 2 filtered branches for 'feat', got %d", len(bs.Filtered))
	}
	if bs.Filtered[0] != "feature/login" || bs.Filtered[1] != "feature/dashboard" {
		t.Errorf("Unexpected filtered branches: %v", bs.Filtered)
	}

	// Navigate down
	bs.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bs.Selected != 1 {
		t.Errorf("Expected Selected 1 after KeyDown, got %d", bs.Selected)
	}

	// Select branch via Enter
	chosen, closed, _ := bs.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !closed || chosen != "feature/dashboard" {
		t.Errorf("Expected chosen 'feature/dashboard', got %q (closed=%v)", chosen, closed)
	}
	if bs.Active {
		t.Errorf("Expected BranchSwitcher to be inactive after Enter")
	}
}

func TestBranchSwitcherCancel(t *testing.T) {
	bs := NewBranchSwitcher()
	bs.Open([]string{"main", "dev"}, "main")

	chosen, closed, _ := bs.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !closed || chosen != "" {
		t.Errorf("Expected empty chosen branch on Esc, got %q", chosen)
	}
	if bs.Active {
		t.Errorf("Expected BranchSwitcher to be closed")
	}
}
