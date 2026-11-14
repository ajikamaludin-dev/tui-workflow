package editor

import (
	"strings"
	"testing"
)

func TestBufferSearch(t *testing.T) {
	content := "Hello world\nThis is a test\nWorld is big and hello again"
	b := NewBuffer(content)

	b.SetSearchQuery("world")
	if len(b.SearchMatches) != 2 {
		t.Fatalf("expected 2 matches for 'world', got %d", len(b.SearchMatches))
	}

	// First match at (0, 6)
	if b.SearchMatches[0].Row != 0 || b.SearchMatches[0].Col != 6 {
		t.Errorf("expected match 0 at (0, 6), got (%d, %d)", b.SearchMatches[0].Row, b.SearchMatches[0].Col)
	}
	// Second match at (2, 0)
	if b.SearchMatches[1].Row != 2 || b.SearchMatches[1].Col != 0 {
		t.Errorf("expected match 1 at (2, 0), got (%d, %d)", b.SearchMatches[1].Row, b.SearchMatches[1].Col)
	}

	cur, total := b.MatchInfo()
	if cur != 1 || total != 2 {
		t.Errorf("expected match info (1, 2), got (%d, %d)", cur, total)
	}

	// Test MatchAt
	isMatch, isActive := b.MatchAt(0, 6)
	if !isMatch || !isActive {
		t.Errorf("expected (true, true) at (0, 6), got (%v, %v)", isMatch, isActive)
	}

	isMatch, isActive = b.MatchAt(2, 0)
	if !isMatch || isActive {
		t.Errorf("expected (true, false) at (2, 0), got (%v, %v)", isMatch, isActive)
	}

	// FindNext
	if !b.FindNext() {
		t.Fatalf("expected FindNext to return true")
	}
	if b.CursorRow != 2 || b.CursorCol != 0 {
		t.Errorf("expected cursor at (2, 0) after FindNext, got (%d, %d)", b.CursorRow, b.CursorCol)
	}

	// FindPrev
	if !b.FindPrev() {
		t.Fatalf("expected FindPrev to return true")
	}
	if b.CursorRow != 0 || b.CursorCol != 6 {
		t.Errorf("expected cursor at (0, 6) after FindPrev, got (%d, %d)", b.CursorRow, b.CursorCol)
	}

	// ClearSearch
	b.ClearSearch()
	if len(b.SearchMatches) != 0 || b.SearchQuery != "" {
		t.Errorf("expected search cleared")
	}
}

func TestBufferSearchVerticalCentering(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line content"
	}
	lines[30] = "special keyword here"

	b := NewBuffer(strings.Join(lines, "\n"))
	b.SetSearchQuery("keyword")

	// Render with height 20
	_ = b.Render(80, 20, "test.txt", false)

	// Since match is on line 30, with height 20, ScrollRow should be centered around 30 - (20/2) = 20
	expectedScroll := 30 - (20 / 2)
	if b.ScrollRow != expectedScroll {
		t.Errorf("expected ScrollRow to be centered around %d, got %d", expectedScroll, b.ScrollRow)
	}
}
