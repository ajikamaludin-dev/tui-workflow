package terminal

import (
	"strings"
	"testing"

	"github.com/hinshun/vt10x"
)

func newTestView(cols, rows int, screen string) *TerminalView {
	vt := vt10x.New(vt10x.WithSize(cols, rows))
	_, _ = vt.Write([]byte(screen))
	return &TerminalView{vt: vt, cols: cols, rows: rows}
}

func TestSelection_DragAndText(t *testing.T) {
	term := newTestView(10, 3, "hello")
	term.StartSelection(0, 0)
	term.UpdateSelection(4, 0)
	if !term.HasSelection() {
		t.Fatal("expected non-zero selection after drag")
	}
	if got := term.SelectedText(); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestSelection_ReverseDirection(t *testing.T) {
	term := newTestView(10, 3, "hello")
	term.StartSelection(4, 0)
	term.UpdateSelection(0, 0)
	if got := term.SelectedText(); got != "hello" {
		t.Fatalf("reverse selection got %q, want %q", got, "hello")
	}
}

func TestSelection_MultiLine(t *testing.T) {
	term := newTestView(10, 3, "hello\r\nworld")
	term.StartSelection(2, 0)
	term.UpdateSelection(2, 1)
	got := term.SelectedText()
	want := "llo\nwor"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSelection_OutOfBoundsClamped(t *testing.T) {
	term := newTestView(10, 3, "hello")
	term.StartSelection(-5, -5)
	term.UpdateSelection(999, 999)
	if !term.HasSelection() {
		t.Fatal("expected selection after clamped drag")
	}
	// must not panic; full grid selected
	if got := term.SelectedText(); got == "" {
		t.Fatal("expected non-empty SelectedText for clamped full-grid selection")
	}
}

func TestSelection_ClearAndSingleCell(t *testing.T) {
	term := newTestView(10, 3, "hello")
	term.StartSelection(2, 0)
	if term.HasSelection() {
		t.Fatal("single-cell (press without drag) must not count as selection")
	}
	term.ClearSelection()
	if term.HasSelection() {
		t.Fatal("expected cleared selection")
	}
}

func TestSelection_SelectAll(t *testing.T) {
	term := newTestView(5, 2, "ab\r\ncd")
	term.SelectAll()
	got := term.SelectedText()
	want := "ab\ncd"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSelection_NotRenderedWhenInactive(t *testing.T) {
	term := newTestView(5, 1, "abc")
	out := term.Render(5, 1, true)
	if got := term.HasSelection(); got {
		t.Fatal("fresh view must not have a selection")
	}
	if out == "" {
		t.Fatal("render must produce output")
	}
}

func TestSelection_RenderHighlightsSelectedCells(t *testing.T) {
	term := newTestView(5, 1, "abc")
	term.StartSelection(0, 0)
	term.UpdateSelection(1, 0)
	out := term.Render(5, 1, false) // focused=false: isolate selection from cursor inverse
	if !strings.Contains(out, "\x1b[7m") {
		t.Fatalf("expected inverse-video escape in render with active selection, got %q", out)
	}
	term.ClearSelection()
	out = term.Render(5, 1, false)
	if strings.Contains(out, "\x1b[7m") {
		t.Fatalf("expected no inverse-video escape after clearing selection, got %q", out)
	}
}
