package editor

import (
	"testing"
)

func TestMultiCursorBasic(t *testing.T) {
	buf := NewBuffer("hello\nworld\nfoo\nbar")
	if buf.CursorCount() != 1 {
		t.Fatalf("expected 1 cursor, got %d", buf.CursorCount())
	}
	if buf.HasMultipleCursors() {
		t.Fatal("expected no multiple cursors initially")
	}

	buf.AddCursor(1, 2)
	buf.AddCursor(2, 1)
	if buf.CursorCount() != 3 {
		t.Fatalf("expected 3 cursors, got %d", buf.CursorCount())
	}
	if !buf.HasMultipleCursors() {
		t.Fatal("expected HasMultipleCursors to be true")
	}

	// Duplicate add should be ignored
	buf.AddCursor(1, 2)
	if buf.CursorCount() != 3 {
		t.Fatalf("expected 3 cursors after adding duplicate, got %d", buf.CursorCount())
	}

	buf.CollapseCursors()
	if buf.CursorCount() != 1 || buf.HasMultipleCursors() {
		t.Fatal("expected collapse to return to 1 cursor")
	}
}

func TestMultiCursorAboveBelow(t *testing.T) {
	buf := NewBuffer("line 1\nline 2\nline 3\nline 4")
	buf.CursorRow = 1
	buf.CursorCol = 2

	buf.AddCursorBelow()
	if buf.CursorCount() != 2 {
		t.Fatalf("expected 2 cursors, got %d", buf.CursorCount())
	}
	if buf.ExtraCursors[0].Row != 2 || buf.ExtraCursors[0].Col != 2 {
		t.Fatalf("expected extra cursor at (2,2), got (%d,%d)", buf.ExtraCursors[0].Row, buf.ExtraCursors[0].Col)
	}

	buf.AddCursorAbove()
	if buf.CursorCount() != 3 {
		t.Fatalf("expected 3 cursors, got %d", buf.CursorCount())
	}
}

func TestMultiCursorTyping(t *testing.T) {
	buf := NewBuffer("apple\napple\napple")
	buf.CursorRow = 0
	buf.CursorCol = 5
	buf.AddCursor(1, 5)
	buf.AddCursor(2, 5)

	// Type 's' at end of each line: "apples"
	buf.InsertRune('s')

	expected := "apples\napples\napples"
	if buf.Text() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.Text())
	}

	// Backspace on all cursors
	buf.Backspace()
	expected = "apple\napple\napple"
	if buf.Text() != expected {
		t.Fatalf("expected %q after backspace, got %q", expected, buf.Text())
	}
}

func TestMultiCursorAddNextOccurrence(t *testing.T) {
	buf := NewBuffer("const val = 1;\nconst val = 2;\nconst val = 3;")
	// Put cursor inside first "val"
	buf.CursorRow = 0
	buf.CursorCol = 7 // inside "val"

	word := buf.WordAtCursor()
	if word != "val" {
		t.Fatalf("expected word 'val', got %q", word)
	}

	ok := buf.AddNextOccurrence()
	if !ok {
		t.Fatal("expected AddNextOccurrence to succeed")
	}
	if buf.CursorCount() != 2 {
		t.Fatalf("expected 2 cursors, got %d", buf.CursorCount())
	}

	ok = buf.AddNextOccurrence()
	if !ok {
		t.Fatal("expected 3rd AddNextOccurrence to succeed")
	}
	if buf.CursorCount() != 3 {
		t.Fatalf("expected 3 cursors, got %d", buf.CursorCount())
	}

	// Type replacement: "X" replaces all selected "val"
	buf.InsertRune('X')
	expected := "const X = 1;\nconst X = 2;\nconst X = 3;"
	if buf.Text() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.Text())
	}
}
