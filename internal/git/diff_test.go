package git

import (
	"testing"
)

func TestComputeSplitDiff(t *testing.T) {
	orig := "line 1\nline 2\nline 3"
	mod := "line 1\nline 2 (modified)\nline 3\nline 4 (new)"

	res := ComputeSplitDiff("test.txt", orig, mod)
	if len(res.Rows) == 0 {
		t.Fatalf("Expected non-empty split diff rows")
	}

	// Line 1 should be unchanged
	if res.Rows[0].LeftType != DiffUnchanged || res.Rows[0].RightType != DiffUnchanged {
		t.Errorf("Row 0 should be unchanged, got Left: %v, Right: %v", res.Rows[0].LeftType, res.Rows[0].RightType)
	}

	// Should have added or modified rows
	hasAdded := false
	for _, row := range res.Rows {
		if row.RightType == DiffAdded {
			hasAdded = true
			break
		}
	}
	if !hasAdded {
		t.Errorf("Expected at least one DiffAdded row")
	}

	// Test render
	dv := NewDiffView("test.txt", orig, mod)
	rendered := dv.Render(80, 10, true)
	if len(rendered) == 0 {
		t.Errorf("Rendered diff view should not be empty")
	}
}
