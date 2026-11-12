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

func TestBufferWordJump(t *testing.T) {
	// Text: "hello world.foo_bar  baz"
	// Indices:
	// 'h'=0, 'w'=6, '.'=11, 'f'=12, '_'=15, ' '=20, 'b'=21
	buf := NewBuffer("hello world.foo_bar  baz\nsecond line")

	// Jump right from start
	buf.SetCursor(0, 0)
	buf.MoveWordRight() // after "hello " -> should land at "world" (col 6)
	if buf.CursorCol != 6 {
		t.Errorf("Expected cursor at col 6 ('world'), got %d", buf.CursorCol)
	}

	buf.MoveWordRight() // after "world" -> lands at "." (col 11)
	if buf.CursorCol != 11 {
		t.Errorf("Expected cursor at col 11 ('.'), got %d", buf.CursorCol)
	}

	buf.MoveWordRight() // after "." -> lands at "foo_bar" (col 12)
	if buf.CursorCol != 12 {
		t.Errorf("Expected cursor at col 12 ('foo_bar'), got %d", buf.CursorCol)
	}

	buf.MoveWordRight() // after "foo_bar  " -> lands at "baz" (col 21)
	if buf.CursorCol != 21 {
		t.Errorf("Expected cursor at col 21 ('baz'), got %d", buf.CursorCol)
	}

	buf.MoveWordRight() // end of line (col 24)
	if buf.CursorCol != 24 {
		t.Errorf("Expected cursor at col 24 (end of line), got %d", buf.CursorCol)
	}

	buf.MoveWordRight() // wraps to next line start (row 1, col 0)
	if buf.CursorRow != 1 || buf.CursorCol != 0 {
		t.Errorf("Expected cursor at (1, 0), got (%d, %d)", buf.CursorRow, buf.CursorCol)
	}

	// Jump left back
	buf.MoveWordLeft() // wraps to end of line 0 (row 0, col 24)
	if buf.CursorRow != 0 || buf.CursorCol != 24 {
		t.Errorf("Expected cursor at (0, 24), got (%d, %d)", buf.CursorRow, buf.CursorCol)
	}

	buf.MoveWordLeft() // back before "baz" -> col 21
	if buf.CursorCol != 21 {
		t.Errorf("Expected cursor at col 21, got %d", buf.CursorCol)
	}

	buf.MoveWordLeft() // back before "foo_bar" -> col 12
	if buf.CursorCol != 12 {
		t.Errorf("Expected cursor at col 12, got %d", buf.CursorCol)
	}

	buf.MoveWordLeft() // back before "." -> col 11
	if buf.CursorCol != 11 {
		t.Errorf("Expected cursor at col 11, got %d", buf.CursorCol)
	}

	buf.MoveWordLeft() // back before "world" -> col 6
	if buf.CursorCol != 6 {
		t.Errorf("Expected cursor at col 6, got %d", buf.CursorCol)
	}

	buf.MoveWordLeft() // back before "hello" -> col 0
	if buf.CursorCol != 0 {
		t.Errorf("Expected cursor at col 0, got %d", buf.CursorCol)
	}
}

func TestBufferGutterWidth(t *testing.T) {
	buf := NewBuffer("one\ntwo")
	if buf.GutterDigits() != 3 {
		t.Errorf("Expected min 3 digits, got %d", buf.GutterDigits())
	}
	if buf.GutterWidth() != 7 {
		t.Errorf("Expected gutter width 7 (3 digits + 4), got %d", buf.GutterWidth())
	}

	// 1000 lines
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString("line\n")
	}
	bufBig := NewBuffer(sb.String())
	if bufBig.GutterDigits() < 4 {
		t.Errorf("Expected at least 4 digits for 1000 lines, got %d", bufBig.GutterDigits())
	}
	if bufBig.GutterWidth() != bufBig.GutterDigits()+4 {
		t.Errorf("GutterWidth mismatch with digits")
	}
}

func TestBufferRowColPrecisionWrappedAndDisabled(t *testing.T) {
	// Test soft-wrapped precision: click past the end of first visual line
	// Line is 30 chars, wrap width is 10.
	// Visual Line 0: cols 0..10
	// Visual Line 1: cols 10..20
	// Visual Line 2: cols 20..30
	buf := NewBuffer("012345678901234567890123456789")
	buf.SoftWrap = true

	// Click on visual line 0 (relY=0) with textX=25 (far past col 10)
	row, col := buf.RowColFromVisual(0, 25, 10)
	if row != 0 {
		t.Errorf("Expected row 0, got %d", row)
	}
	// Must be clamped to EndCol (10), NOT 25!
	if col != 10 {
		t.Errorf("Expected clamped col 10 on wrapped line, got %d", col)
	}

	// Test with SoftWrap = false
	buf.SoftWrap = false
	buf.ScrollCol = 5
	// Click on line 0 with textX=4 -> physCol should be ScrollCol + textX = 5 + 4 = 9
	row, col = buf.RowColFromVisual(0, 4, 10)
	if row != 0 || col != 9 {
		t.Errorf("Expected (0, 9) when SoftWrap is false, got (%d, %d)", row, col)
	}

	// Click past end of line
	row, col = buf.RowColFromVisual(0, 50, 10)
	if col != 30 {
		t.Errorf("Expected clamped to line length 30, got %d", col)
	}
}

func TestBufferDisabledSoftWrapRenderingAndScroll(t *testing.T) {
	longLine := "abcdefghijklmnopqrstuvwxyz1234567890" // 36 chars
	buf := NewBuffer(longLine)
	buf.SoftWrap = false

	// Render with width = 20 (gutter 7, textWidth 13)
	buf.SetCursor(0, 25)
	rendered := buf.Render(20, 5, "test.txt", true)

	// Since cursor is at col 25, ScrollCol must have scrolled right so col 25 is visible
	if buf.ScrollCol == 0 {
		t.Errorf("Expected ScrollCol > 0 when cursor is at col 25 and width is narrow")
	}
	if !strings.Contains(rendered, "│") {
		t.Errorf("Expected cursor rendered in viewport")
	}
}

func TestBufferUndoRedo(t *testing.T) {
	buf := NewBuffer("hello")
	buf.CursorCol = 5 // move cursor to end of "hello"
	if buf.CanUndo() {
		t.Errorf("Newly created buffer should not have undo")
	}
	if buf.CanRedo() {
		t.Errorf("Newly created buffer should not have redo")
	}

	// 1. Insert text
	buf.InsertRune(' ')
	buf.InsertRune('w')
	buf.InsertRune('o')
	buf.InsertRune('r')
	buf.InsertRune('l')
	buf.InsertRune('d')

	if buf.Text() != "hello world" {
		t.Errorf("Expected 'hello world', got %q", buf.Text())
	}
	if !buf.CanUndo() {
		t.Errorf("Expected CanUndo to be true after editing")
	}

	// 2. Undo step 1 (undo word "world")
	if !buf.Undo() {
		t.Fatalf("Undo failed")
	}
	if buf.Text() != "hello " {
		t.Errorf("Expected 'hello ' after first undo, got %q", buf.Text())
	}
	// Undo step 2 (undo space)
	if !buf.Undo() {
		t.Fatalf("Undo 2 failed")
	}
	if buf.Text() != "hello" {
		t.Errorf("Expected 'hello' after second undo, got %q", buf.Text())
	}
	if !buf.CanRedo() {
		t.Errorf("Expected CanRedo to be true after undo")
	}

	// 3. Redo both
	if !buf.Redo() {
		t.Fatalf("Redo 1 failed")
	}
	if buf.Text() != "hello " {
		t.Errorf("Expected 'hello ' after redo 1, got %q", buf.Text())
	}
	if !buf.Redo() {
		t.Fatalf("Redo 2 failed")
	}
	if buf.Text() != "hello world" {
		t.Errorf("Expected 'hello world' after redo 2, got %q", buf.Text())
	}

	// 4. Test stack capping (max 100)
	for i := 0; i < 120; i++ {
		buf.InsertRune('!')
	}
	if len(buf.undoStack) > 100 {
		t.Errorf("Expected undo stack capped at 100, got %d", len(buf.undoStack))
	}
}

func TestBufferIndentOutdent(t *testing.T) {
	// Single line indent
	buf := NewBuffer("func main() {\nprintln(\"hi\")\n}")
	buf.SetCursor(1, 0)
	buf.IndentSelection()
	if string(buf.Lines[1]) != "    println(\"hi\")" {
		t.Errorf("Expected 4 spaces indent, got %q", string(buf.Lines[1]))
	}

	// Single line outdent
	buf.OutdentSelection()
	if string(buf.Lines[1]) != "println(\"hi\")" {
		t.Errorf("Expected outdent to remove 4 spaces, got %q", string(buf.Lines[1]))
	}

	// Multiline selection indent
	buf.StartSelection(0, 0)
	buf.UpdateSelection(2, 1)

	buf.IndentSelection()
	if string(buf.Lines[0]) != "    func main() {" || string(buf.Lines[1]) != "    println(\"hi\")" || string(buf.Lines[2]) != "    }" {
		t.Errorf("Multiline indent failed: %v", buf.Lines)
	}

	// Multiline selection outdent
	buf.OutdentSelection()
	if string(buf.Lines[0]) != "func main() {" || string(buf.Lines[1]) != "println(\"hi\")" || string(buf.Lines[2]) != "}" {
		t.Errorf("Multiline outdent failed: %v", buf.Lines)
	}
}
