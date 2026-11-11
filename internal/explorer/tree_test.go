package explorer

import (
	"os"
	"testing"
)

func TestTreeNavigation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-tree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup directory structure
	_, _ = CreateFile(tempDir, "file1.txt")
	_, _ = CreateFile(tempDir, "file2.go")
	_, _ = CreateFile(tempDir, ".hidden")
	subDir, _ := CreateFolder(tempDir, "sub")
	_, _ = CreateFile(subDir, "nested.md")

	tree := NewTree(tempDir)
	if len(tree.FlatNodes) == 0 {
		t.Fatalf("Tree should have nodes")
	}

	// Default ShowHidden is false, so .hidden should not be in FlatNodes
	for _, n := range tree.FlatNodes {
		if n.Name == ".hidden" {
			t.Errorf(".hidden should not be visible when ShowHidden is false")
		}
	}

	// Toggle hidden
	tree.ToggleHidden()
	foundHidden := false
	for _, n := range tree.FlatNodes {
		if n.Name == ".hidden" {
			foundHidden = true
			break
		}
	}
	if !foundHidden {
		t.Errorf(".hidden should be visible after ToggleHidden")
	}

	// Navigation
	tree.SelectedIndex = 0
	tree.MoveDown()
	if tree.SelectedIndex != 1 {
		t.Errorf("Expected SelectedIndex 1, got %d", tree.SelectedIndex)
	}

	tree.MoveUp()
	if tree.SelectedIndex != 0 {
		t.Errorf("Expected SelectedIndex 0, got %d", tree.SelectedIndex)
	}
}
