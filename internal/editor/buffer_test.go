package editor

import (
	"strings"
	"testing"
)

func TestBufferOperations(t *testing.T) {
	buf := NewBuffer("hello\nworld")

	if buf.LineCount() != 2 {
		t.Fatalf("Expected 2 lines, got %d", buf.LineCount())
	}

	// Insert rune at (0, 5) -> "hello!"
	buf.SetCursor(0, 5)
	buf.InsertRune('!')
	if string(buf.Lines[0]) != "hello!" {
		t.Errorf("Expected 'hello!', got '%s'", string(buf.Lines[0]))
	}

	// Backspace at (0, 6) -> removes '!'
	buf.Backspace()
	if string(buf.Lines[0]) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", string(buf.Lines[0]))
	}

	// Insert newline at (0, 5) -> splits into "hello" and ""
	buf.InsertNewline()
	if buf.LineCount() != 3 {
		t.Errorf("Expected 3 lines after newline, got %d", buf.LineCount())
	}

	// Backspace at start of line (1, 0) -> merges back
	buf.Backspace()
	if buf.LineCount() != 2 {
		t.Errorf("Expected 2 lines after merge, got %d", buf.LineCount())
	}
}

func TestBufferSoftWrap(t *testing.T) {
	longLine := "12345678901234567890" // 20 chars
	buf := NewBuffer(longLine)

	// Wrap with width = 10 -> should produce 2 visual lines
	vLines := buf.ComputeVisualLines(10)
	if len(vLines) != 2 {
		t.Fatalf("Expected 2 visual lines, got %d", len(vLines))
	}

	if string(vLines[0].Runes) != "1234567890" {
		t.Errorf("Expected '1234567890', got '%s'", string(vLines[0].Runes))
	}
	if string(vLines[1].Runes) != "1234567890" {
		t.Errorf("Expected '1234567890', got '%s'", string(vLines[1].Runes))
	}
	if !vLines[1].IsContinuation {
		t.Errorf("Second visual line should be marked as continuation")
	}
}

func TestBufferSelection(t *testing.T) {
	buf := NewBuffer("hello world\nsecond line")

	// Select "world" (0, 6 to 0, 11)
	buf.StartSelection(0, 6)
	buf.UpdateSelection(0, 11)
	if !buf.HasSelection {
		t.Fatalf("Expected selection active")
	}

	selected := buf.GetSelectedText()
	if selected != "world" {
		t.Errorf("Expected 'world', got %q", selected)
	}

	// Delete selection
	buf.DeleteSelection()
	if string(buf.Lines[0]) != "hello " {
		t.Errorf("Expected 'hello ', got %q", string(buf.Lines[0]))
	}

	// Multiline selection
	buf = NewBuffer("line 1\nline 2\nline 3")
	buf.StartSelection(0, 5)  // "1\n"
	buf.UpdateSelection(2, 4) // "line"
	selected = buf.GetSelectedText()
	expected := "1\nline 2\nline"
	if selected != expected {
		t.Errorf("Expected multiline %q, got %q", expected, selected)
	}

	// SelectAll
	buf.SelectAll()
	if !buf.HasSelection {
		t.Errorf("Expected HasSelection true after SelectAll")
	}
	allText := buf.GetSelectedText()
	if allText != "line 1\nline 2\nline 3" {
		t.Errorf("Expected full text, got %q", allText)
	}
}

func TestBufferIBeamCursor(t *testing.T) {
	buf := NewBuffer("hi")
	buf.SetCursor(0, 2) // at end of line
	rendered := buf.Render(20, 5, "test.txt", true)
	if !strings.Contains(rendered, "│") {
		t.Errorf("Expected I-beam vertical bar at end of line in rendered output")
	}
}
