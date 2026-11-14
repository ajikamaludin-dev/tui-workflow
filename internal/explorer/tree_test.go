package explorer

import (
	"os"
	"path/filepath"
	"strings"
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

	// Default ShowHidden is true, so .hidden should be in FlatNodes
	foundHidden := false
	for _, n := range tree.FlatNodes {
		if n.Name == ".hidden" {
			foundHidden = true
			break
		}
	}
	if !foundHidden {
		t.Errorf(".hidden should be visible by default when ShowHidden is true")
	}

	// Toggle hidden to false
	tree.ToggleHidden()
	for _, n := range tree.FlatNodes {
		if n.Name == ".hidden" {
			t.Errorf(".hidden should not be visible after ToggleHidden to false")
		}
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

func TestTreeLongNameTruncationAndMarquee(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-tree-marquee-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	longName := "2026-11-11 - Initial Project Setup & v1 Implementation.md"
	_, _ = CreateFile(tempDir, longName)
	_, _ = CreateFile(tempDir, "short.go")

	tree := NewTree(tempDir)
	if len(tree.FlatNodes) < 2 {
		t.Fatalf("Tree should have nodes")
	}

	// Find index of longName node
	longIdx := -1
	for idx, n := range tree.FlatNodes {
		if n.Name == longName {
			longIdx = idx
			break
		}
	}
	if longIdx == -1 {
		t.Fatalf("Could not find node with longName")
	}

	// 1. When unselected (select other node), long node should be truncated with ...
	tree.SelectedIndex = (longIdx + 1) % len(tree.FlatNodes)
	rendered := tree.Render(28, 5, true)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 5 {
		t.Errorf("Expected exactly 5 lines (no line wrapping), got %d", len(lines))
	}
	if !strings.Contains(rendered, "...") {
		t.Errorf("Expected inactive long filename to be truncated with '...'")
	}

	// 2. When selected, marquee is active
	tree.SelectedIndex = longIdx
	if !tree.SelectedNameExceeds(28) {
		t.Errorf("Expected SelectedNameExceeds(28) to be true for long name")
	}

	rendered1 := tree.Render(28, 5, true)
	lines1 := strings.Split(rendered1, "\n")
	if len(lines1) != 5 {
		t.Errorf("Expected exactly 5 lines with marquee, got %d", len(lines1))
	}

	// Advance marquee offset
	tree.TickMarquee()
	tree.TickMarquee()
	rendered2 := tree.Render(28, 5, true)
	if rendered1 == rendered2 {
		t.Errorf("Expected marquee text to change after TickMarquee()")
	}

	// Reset marquee
	tree.ResetMarquee()
	renderedReset := tree.Render(28, 5, true)
	if renderedReset != rendered1 {
		t.Errorf("Expected marquee text to reset to initial frame after ResetMarquee()")
	}
}

func TestTreeFiltering(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tree_filter_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Hello"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "util.go"), []byte("package pkg"), 0644)

	tree := NewTree(tmpDir)
	if len(tree.FlatNodes) < 3 {
		t.Fatalf("Expected at least 3 nodes, got %d", len(tree.FlatNodes))
	}

	// Filter by ".go"
	tree.SetFilter(".go")
	for _, n := range tree.FlatNodes {
		if !n.IsDir && !strings.HasSuffix(n.Name, ".go") {
			t.Errorf("Unexpected non-go file in filtered tree: %s", n.Name)
		}
	}

	// Clear filter
	tree.SetFilter("")
	hasReadme := false
	for _, n := range tree.FlatNodes {
		if n.Name == "README.md" {
			hasReadme = true
			break
		}
	}
	if !hasReadme {
		t.Errorf("Expected README.md to be restored after clearing filter")
	}
}

func TestTreeScrollUpDown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tree_scroll_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 30 files
	for i := 0; i < 30; i++ {
		p := filepath.Join(tmpDir, filepath.Clean(string(rune('a'+(i%26)))+string(rune('0'+(i/26)))+".txt"))
		_ = os.WriteFile(p, []byte("test"), 0644)
	}

	tree := NewTree(tmpDir)
	if len(tree.FlatNodes) < 30 {
		t.Fatalf("Expected at least 30 nodes, got %d", len(tree.FlatNodes))
	}

	height := 10
	// 1. Initial render
	_ = tree.Render(28, height, true)
	if tree.ScrollOffset != 0 {
		t.Errorf("Initial ScrollOffset should be 0, got %d", tree.ScrollOffset)
	}

	// 2. Scroll all the way down
	for i := 0; i < 15; i++ {
		tree.ScrollDown(3, height)
		_ = tree.Render(28, height, true)
	}

	expectedMaxScroll := len(tree.FlatNodes) - height
	if tree.ScrollOffset != expectedMaxScroll {
		t.Errorf("Expected ScrollOffset at maxScroll %d, got %d", expectedMaxScroll, tree.ScrollOffset)
	}

	// 3. Scroll all the way back up
	for i := 0; i < 15; i++ {
		tree.ScrollUp(3)
		_ = tree.Render(28, height, true)
	}

	if tree.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset to be 0 after scrolling back up, got %d", tree.ScrollOffset)
	}

	// 4. Select bottom node, scroll up all the way to 0, ensuring bottom node selection does not clamp scroll
	tree.SelectedIndex = len(tree.FlatNodes) - 1
	tree.ScrollOffset = expectedMaxScroll
	for i := 0; i < 15; i++ {
		tree.ScrollUp(3)
		_ = tree.Render(28, height, true)
	}
	if tree.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset to be 0 even when bottom node is selected, got %d", tree.ScrollOffset)
	}

	// 5. Test keyboard navigation keeps selected node in view
	tree.SelectedIndex = 0
	tree.adjustScroll()
	if tree.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset to be 0 after selecting node 0, got %d", tree.ScrollOffset)
	}

	// Move down past viewport height
	for i := 0; i < 15; i++ {
		tree.MoveDown()
	}
	_ = tree.Render(28, height, true)
	if tree.SelectedIndex != 15 {
		t.Errorf("Expected SelectedIndex 15, got %d", tree.SelectedIndex)
	}
	if tree.ScrollOffset > tree.SelectedIndex || tree.SelectedIndex >= tree.ScrollOffset+height {
		t.Errorf("Expected SelectedIndex %d to be within viewport [%d, %d)", tree.SelectedIndex, tree.ScrollOffset, tree.ScrollOffset+height)
	}

	// Move up back to 0
	for i := 0; i < 15; i++ {
		tree.MoveUp()
	}
	_ = tree.Render(28, height, true)
	if tree.SelectedIndex != 0 {
		t.Errorf("Expected SelectedIndex 0, got %d", tree.SelectedIndex)
	}
	if tree.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset 0 after moving up to 0, got %d", tree.ScrollOffset)
	}
}

