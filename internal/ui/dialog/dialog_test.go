package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputDialog(t *testing.T) {
	d := NewInputDialog()
	if d.Active {
		t.Fatalf("Expected inactive initially")
	}

	d.Open(InputNewFile, "Create File", "/tmp", "init.txt")
	if !d.Active || d.Value() != "init.txt" {
		t.Fatalf("Expected active with value 'init.txt'")
	}

	rendered := d.Render()
	if !strings.Contains(rendered, "Create File") {
		t.Errorf("Rendered view missing title: %s", rendered)
	}

	// Tab navigates buttons
	d.Update(tea.KeyMsg{Type: tea.KeyTab})
	if d.FocusedBtn != 1 {
		t.Errorf("Expected FocusedBtn=1 after Tab, got %d", d.FocusedBtn)
	}

	// Esc cancels
	confirmed, canceled, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if confirmed || !canceled || d.Active {
		t.Errorf("Expected dialog canceled and inactive after Esc")
	}
}

func TestNormalizeDialogBackground(t *testing.T) {
	d := NewInputDialog()
	d.Open(InputNewFile, "New File", "/tmp", "")
	rendered := d.Render()

	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines, got %d", len(lines))
	}

	// For all interior lines, verify that there are no raw unstyled spaces after resets
	for i := 1; i < len(lines)-1; i++ {
		line := lines[i]
		if strings.Contains(line, "\x1b[0m ") {
			t.Errorf("Line %d contains unstyled space immediately after ANSI reset: %q", i, line)
		}
	}
}

func TestConfirmDialog(t *testing.T) {
	d := NewConfirmDialog()
	if d.Active {
		t.Fatalf("Expected inactive initially")
	}

	d.Open(ConfirmDelete, "Delete File", "Delete this file?", "/tmp/file.txt")
	if !d.Active || d.Selected != 1 {
		t.Fatalf("Expected active with default cancel button")
	}

	rendered := d.Render()
	if !strings.Contains(rendered, "Delete File") || !strings.Contains(rendered, "Delete") {
		t.Errorf("Rendered view missing elements: %s", rendered)
	}

	// Enter on default selected Cancel (1)
	confirmed, canceled, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || !canceled || d.Active {
		t.Errorf("Expected canceled after pressing Enter on Cancel button")
	}

	// Test Unsaved Changes
	d.Open(ConfirmUnsavedChanges, "Save Changes?", "Unsaved changes", "")
	// Navigate right to Discard (1)
	d.Selected = 1
	var discard bool
	confirmed, canceled, discard = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || canceled || !discard {
		t.Errorf("Expected discard=true, got confirmed=%v, canceled=%v, discard=%v", confirmed, canceled, discard)
	}

	// Test External Change Conflict
	d.Open(ConfirmExternalChange, "File Conflict", "File modified externally", "/tmp/conflict.txt")
	rendered = d.Render()
	if !strings.Contains(rendered, "Reload") || !strings.Contains(rendered, "Merge (Diff)") || !strings.Contains(rendered, "Cancel") {
		t.Errorf("Rendered view missing external change buttons: %s", rendered)
	}

	// Select 0: Reload
	d.Selected = 0
	confirmed, canceled, discard = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !confirmed || canceled || discard {
		t.Errorf("Expected confirmed=true for Reload, got confirmed=%v, canceled=%v, discard=%v", confirmed, canceled, discard)
	}

	// Select 1: Merge
	d.Open(ConfirmExternalChange, "File Conflict", "File modified externally", "/tmp/conflict.txt")
	d.Selected = 1
	confirmed, canceled, discard = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || canceled || !discard {
		t.Errorf("Expected discard=true for Merge, got confirmed=%v, canceled=%v, discard=%v", confirmed, canceled, discard)
	}
}

func TestConfirmDialog_Alert(t *testing.T) {
	d := NewConfirmDialog()
	d.Open(ConfirmAlert, "Git Identity Required", "Please configure your git user name and email", "")
	if !d.Active || d.Selected != 0 {
		t.Fatalf("Expected active with selected=0 for alert, got active=%v selected=%d", d.Active, d.Selected)
	}

	rendered := d.Render()
	if !strings.Contains(rendered, "Git Identity Required") || !strings.Contains(rendered, "OK") {
		t.Errorf("Rendered view missing elements: %s", rendered)
	}

	// Pressing Enter confirms and closes
	confirmed, canceled, discard := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !confirmed || canceled || discard || d.Active {
		t.Errorf("Expected confirmed=true on Enter for alert, got confirmed=%v canceled=%v discard=%v active=%v", confirmed, canceled, discard, d.Active)
	}

	// Reopen and test space
	d.Open(ConfirmAlert, "Notice", "Space to close", "")
	confirmed, canceled, discard = d.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !confirmed || canceled || discard || d.Active {
		t.Errorf("Expected confirmed=true on Space for alert")
	}

	// Reopen and test Esc
	d.Open(ConfirmAlert, "Notice", "Esc to close", "")
	confirmed, canceled, discard = d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if confirmed || !canceled || discard || d.Active {
		t.Errorf("Expected canceled=true on Esc for alert")
	}
}

