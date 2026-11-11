package editor

import (
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
