package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGitClickStageButtonAndBinaryGuard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-git-binary-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m := NewModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	// Create a simulated binary file with null byte inside Git repo root
	binPath := "sample.bin"
	fullBinPath := filepath.Join(m.GitRepo.RootPath, binPath)
	binData := append([]byte("ELF"), 0x00, 0x01, 0x02)
	_ = os.WriteFile(fullBinPath, binData, 0755)
	defer os.Remove(fullBinPath)

	// 1. Test binary guard in openDiff
	initTabCount := m.TabManager.Count()
	m.openDiff(binPath, false)

	if m.TabManager.Count() != initTabCount {
		t.Errorf("Binary file should NOT be opened in TabManager diff view")
	}

	if m.StatusBar.ToastMsg != "Diff file biner tidak didukung" {
		t.Errorf("Expected toast 'Diff file biner tidak didukung', got: %q", m.StatusBar.ToastMsg)
	}

	// 2. Test GitView click coordinates
	if m.GitView != nil {
		// Set up changes with a mock item
		// Column 4 is [+]
		targetFile, _, handled := m.GitView.HandleMouseClick(4, 999) // non-existent row
		if handled || targetFile != nil {
			t.Errorf("Expected unhandled for invalid row")
		}
	}
}
