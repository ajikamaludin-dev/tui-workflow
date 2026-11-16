package git

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

func TestLineChangesFromSplitDiff(t *testing.T) {
	orig := "same line\ndelete me\nmodify me\nalso same\n"
	mod := "same line\nmodify ME\nalso same\nbrand new\n"

	rows := ComputeSplitDiff("", orig, mod).Rows
	changes := LineChangesFromSplitDiff(rows)

	// LCS pairs "delete me" with the added "modify ME" → line 2 = Modified;
	// the remaining pure-delete row marks the next worktree line (3) Deleted.
	if changes[2] != LineModified {
		t.Errorf("expected line 2 = LineModified, got %v (all changes: %v)", changes[2], changes)
	}
	if changes[3] != LineDeleted {
		t.Errorf("expected line 3 = LineDeleted (marker after deletion block), got %v (all changes: %v)", changes[3], changes)
	}
	// "brand new" pure add → line 4
	if changes[4] != LineAdded {
		t.Errorf("expected line 4 = LineAdded, got %v (all changes: %v)", changes[4], changes)
	}
	// line 1 unchanged
	if changes[1] != LineUnchanged {
		t.Errorf("expected line 1 = LineUnchanged, got %v", changes[1])
	}
}

func TestLineChangesDeletionAtEOF(t *testing.T) {
	orig := "keep\ngone 1\ngone 2\n"
	mod := "keep\n"

	changes := ComputeLineChanges(orig, mod)
	// deletion of trailing lines: marker on the last worktree line (line 1)
	if changes[1] != LineDeleted {
		t.Errorf("expected line 1 = LineDeleted for EOF deletion, got %v (changes: %v)", changes[1], changes)
	}
}

func TestLineChangesMiddleDeletion(t *testing.T) {
	orig := "a\nb\nc\n"
	mod := "a\nc\n"

	changes := ComputeLineChanges(orig, mod)
	// middle deletion: marker on the line after the deleted block (line 2 = "c")
	if changes[2] != LineDeleted {
		t.Errorf("expected line 2 = LineDeleted for middle deletion, got %v (changes: %v)", changes[2], changes)
	}
	if changes[1] != LineUnchanged {
		t.Errorf("expected line 1 = LineUnchanged, got %v", changes[1])
	}
}

func TestComputeLineChangesUntracked(t *testing.T) {
	changes := ComputeLineChanges("", "x\ny\nz")
	for i := 1; i <= 3; i++ {
		if changes[i] != LineAdded {
			t.Errorf("expected untracked line %d = LineAdded, got %v", i, changes[i])
		}
	}
}

func TestLineChangesIdentical(t *testing.T) {
	changes := ComputeLineChanges("a\nb\nc\n", "a\nb\nc\n")
	if len(changes) != 0 {
		t.Errorf("expected no changes for identical content, got %v", changes)
	}
}

func TestWrapDiffText(t *testing.T) {
	segs := wrapDiffText("abcde", 2)
	if len(segs) != 3 || segs[0] != "ab" || segs[1] != "cd" || segs[2] != "e" {
		t.Errorf("wrapDiffText basic wrap failed: %v", segs)
	}
	if got := wrapDiffText("abc", 10); len(got) != 1 || got[0] != "abc" {
		t.Errorf("wrapDiffText short line failed: %v", got)
	}
	if got := wrapDiffText("a\tb", 10); got[0] != "a    b" {
		t.Errorf("wrapDiffText tab expansion failed: %v", got)
	}
}

func TestDiffViewSoftWrap(t *testing.T) {
	longLine := "this is an extremely long line that definitely exceeds the narrow test viewport width of about fifty text columns or so yes it does"
	orig := longLine + "\nkeep\n"
	mod := longLine + " modified\nkeep\n"

	dv := NewDiffView("test.txt", orig, mod)
	cache := dv.visualCache(10) // narrow text width forces multiple segments
	if len(cache.Lines) <= len(dv.Result.Rows) {
		t.Errorf("expected visual lines (%d) > rows (%d) when wrapping", len(cache.Lines), len(dv.Result.Rows))
	}
	// RowToVisual must be non-decreasing and within range
	for i := 1; i < len(cache.RowToVisual); i++ {
		if cache.RowToVisual[i] < cache.RowToVisual[i-1] {
			t.Errorf("RowToVisual not monotonic at %d", i)
		}
	}

	// Every rendered line must fit the requested width (ANSI-aware).
	rendered := dv.Render(40, 10, true)
	for _, line := range strings.Split(rendered, "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("rendered line exceeds width 40 (got %d): %q", w, line)
		}
	}

	// ActiveLine falls back to right-side line numbers.
	if ln := dv.ActiveLine(); ln < 1 {
		t.Errorf("ActiveLine should be >= 1, got %d", ln)
	}
}
