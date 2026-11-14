package editor

import (
	"testing"
)

func TestFoldableAt(t *testing.T) {
	src := `func main() {
    line1
    line2
}
func other() {
}
`
	buf := NewBuffer(src)

	// Line 0 ("func main() {") has indented lines following it
	end, ok := buf.FoldableAt(0)
	if !ok {
		t.Fatal("expected line 0 to be foldable")
	}
	if end != 2 {
		t.Fatalf("expected fold end to be line 2, got %d", end)
	}

	// Line 1 is inside block, not a header for smaller block
	_, ok = buf.FoldableAt(1)
	if ok {
		t.Fatal("expected line 1 not to be foldable")
	}

	// Line 4 ("func other() {") has no indented lines following it before closing
	_, ok = buf.FoldableAt(4)
	if ok {
		t.Fatal("expected line 4 without body not to be foldable")
	}
}

func TestToggleFold(t *testing.T) {
	src := `func calculate() {
    a := 1
    b := 2
    return a + b
}
line6`
	buf := NewBuffer(src)
	buf.CursorRow = 2 // on "b := 2"

	// Fold block at line 0
	ok := buf.ToggleFold(0)
	if !ok {
		t.Fatal("expected ToggleFold(0) to succeed")
	}
	if !buf.IsFoldedAt(0) {
		t.Fatal("expected IsFoldedAt(0) to be true")
	}

	// Lines 1, 2, 3 should be hidden; line 0 should NOT be hidden
	if buf.IsHiddenLine(0) {
		t.Fatal("anchor line 0 should not be hidden")
	}
	for r := 1; r <= 3; r++ {
		if !buf.IsHiddenLine(r) {
			t.Fatalf("line %d should be hidden", r)
		}
	}
	if buf.IsHiddenLine(4) {
		t.Fatal("line 4 should not be hidden")
	}

	// Cursor was at row 2 (which is hidden) -> should be pinned to row 0
	if buf.CursorRow != 0 {
		t.Fatalf("cursor expected to be pinned to 0, got %d", buf.CursorRow)
	}

	// Visible physical lines should be [0, 4, 5]
	vis := buf.VisiblePhysicalLines()
	expectedVis := []int{0, 4, 5}
	if len(vis) != len(expectedVis) {
		t.Fatalf("expected %d visible lines, got %d: %v", len(expectedVis), len(vis), vis)
	}
	for i, v := range expectedVis {
		if vis[i] != v {
			t.Errorf("visible line [%d]: expected %d, got %d", i, v, vis[i])
		}
	}

	// Toggle fold again to unfold
	ok = buf.ToggleFold(0)
	if !ok || buf.IsFoldedAt(0) {
		t.Fatal("expected ToggleFold(0) to unfold")
	}
	if buf.IsHiddenLine(1) {
		t.Fatal("line 1 should no longer be hidden")
	}
}

func TestFoldAllAndUnfoldAll(t *testing.T) {
	src := `block1 {
    child1
}
block2 {
    child2
}
`
	buf := NewBuffer(src)

	buf.FoldAll()
	if len(buf.Folded) != 2 {
		t.Fatalf("expected 2 folded regions, got %d", len(buf.Folded))
	}

	vis := buf.VisiblePhysicalLines()
	// Line 0 ("block1"), Line 2 ("}"), Line 3 ("block2"), Line 5 ("}")
	if len(vis) >= len(buf.Lines) {
		t.Fatalf("expected hidden lines in fold all, got %v", vis)
	}

	buf.UnfoldAll()
	if len(buf.Folded) != 0 {
		t.Fatalf("expected 0 folded regions after UnfoldAll, got %d", len(buf.Folded))
	}
}
