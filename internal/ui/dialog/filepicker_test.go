package dialog

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilePickerOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-test-picker-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "subfolder")
	_ = os.Mkdir(subDir, 0755)
	fileA := filepath.Join(tempDir, "alpha.go")
	_ = os.WriteFile(fileA, []byte("package main"), 0644)
	fileB := filepath.Join(tempDir, "beta.md")
	_ = os.WriteFile(fileB, []byte("# Markdown"), 0644)

	picker := NewFilePicker()

	// 1. Open in Folder mode
	picker.Open(PickerModeFolder, tempDir, true)
	if !picker.Active {
		t.Fatalf("Expected picker to be active")
	}
	// In folder mode, files should be omitted from list (only .. and subfolder)
	for _, it := range picker.Items {
		if !it.IsDir {
			t.Errorf("Unexpected non-dir item in PickerModeFolder: %s", it.Name)
		}
	}

	// 2. Open in File mode
	picker.Open(PickerModeFile, tempDir, true)
	var foundAlpha bool
	for _, it := range picker.Items {
		if it.Name == "alpha.go" {
			foundAlpha = true
			break
		}
	}
	if !foundAlpha {
		t.Fatalf("Expected alpha.go to be present in PickerModeFile")
	}

	// 3. Search Filter
	picker.FilterInput.SetValue("alpha")
	picker.applyFilter()
	if len(picker.Filtered) < 1 {
		t.Fatalf("Expected at least 1 filtered item for 'alpha'")
	}

	// 4. Keyboard Navigation (Down / Up)
	picker.FilterInput.SetValue("")
	picker.applyFilter()
	picker.FocusArea = 1 // List focus
	_, _, _, _ = picker.Update(tea.KeyMsg{Type: tea.KeyDown})
	if picker.SelectedIdx != 1 {
		t.Errorf("Expected SelectedIdx 1, got %d", picker.SelectedIdx)
	}
	_, _, _, _ = picker.Update(tea.KeyMsg{Type: tea.KeyUp})
	if picker.SelectedIdx != 0 {
		t.Errorf("Expected SelectedIdx 0, got %d", picker.SelectedIdx)
	}

	// 5. Render should not panic and produce non-empty string
	rendered := picker.Render()
	if rendered == "" {
		t.Errorf("Render() returned empty string")
	}

	// 6. Mouse click on item
	confirmed, canceled, _, handled := picker.HandleMouse(10, 6, false, false)
	if !handled {
		t.Errorf("Expected HandleMouse to handle inside-box click")
	}
	if confirmed || canceled {
		t.Errorf("First click should only select, not confirm or cancel")
	}

	// 7. Mouse click on Cancel button
	boxH := picker.Height
	confirmed, canceled, _, handled = picker.HandleMouse(40, boxH-3, false, false)
	if !handled || !canceled || confirmed {
		t.Errorf("Expected click on cancel button to cancel")
	}
}
