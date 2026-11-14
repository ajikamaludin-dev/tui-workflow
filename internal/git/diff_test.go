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

	// Test FormatUnifiedDiff
	unified := FormatUnifiedDiff(res, 1)
	if len(unified) == 0 {
		t.Errorf("Expected non-empty unified diff")
	}
	if !contains(unified, "+line 2 (modified)") {
		t.Errorf("Expected unified diff to contain added line, got:\n%s", unified)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && (s[:len(substr)] == substr || (len(s) > len(substr) && (contains(s[1:], substr))))))
}

func TestDiffHunkVerticalCentering(t *testing.T) {
	origLines := make([]string, 50)
	modLines := make([]string, 50)
	for i := range origLines {
		origLines[i] = "common line"
		modLines[i] = "common line"
	}
	modLines[30] = "modified line 30"

	orig := ""
	mod := ""
	for i := range origLines {
		orig += origLines[i] + "\n"
		mod += modLines[i] + "\n"
	}

	dv := NewDiffView("test.txt", orig, mod)
	if len(dv.Hunks) == 0 {
		t.Fatalf("expected at least 1 hunk")
	}

	// Navigate to next hunk with viewHeight 20
	dv.NextHunk(20)

	h, ok := dv.ActiveHunkData()
	if !ok {
		t.Fatalf("expected active hunk data")
	}

	// Check that ScrollOffset is centered around target (target - contentHeight/2)
	contentHeight := 20 - 1
	expectedOffset := h.StartRow - (contentHeight / 2)
	if expectedOffset < 0 {
		expectedOffset = 0
	}
	if dv.ScrollOffset != expectedOffset {
		t.Errorf("expected ScrollOffset to be %d, got %d (hunk StartRow=%d)", expectedOffset, dv.ScrollOffset, h.StartRow)
	}
}
