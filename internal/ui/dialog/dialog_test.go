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

	d.Open(InputNewFile, "Buat File", "/tmp", "init.txt")
	if !d.Active || d.Value() != "init.txt" {
		t.Fatalf("Expected active with value 'init.txt'")
	}

	rendered := d.Render()
	if !strings.Contains(rendered, "Buat File") {
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

func TestConfirmDialog(t *testing.T) {
	d := NewConfirmDialog()
	if d.Active {
		t.Fatalf("Expected inactive initially")
	}

	d.Open(ConfirmDelete, "Hapus File", "Hapus file ini?", "/tmp/file.txt")
	if !d.Active || d.Selected != 1 {
		t.Fatalf("Expected active with default cancel button")
	}

	rendered := d.Render()
	if !strings.Contains(rendered, "Hapus File") || !strings.Contains(rendered, "Delete") {
		t.Errorf("Rendered view missing elements: %s", rendered)
	}

	// Enter on default selected Cancel (1)
	confirmed, canceled, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || !canceled || d.Active {
		t.Errorf("Expected canceled after pressing Enter on Cancel button")
	}

	// Test Unsaved Changes
	d.Open(ConfirmUnsavedChanges, "Simpan?", "Perubahan belum disimpan", "")
	// Navigate right to Discard (1)
	d.Selected = 1
	confirmed, canceled, discard := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || canceled || !discard {
		t.Errorf("Expected discard=true, got confirmed=%v, canceled=%v, discard=%v", confirmed, canceled, discard)
	}
}
