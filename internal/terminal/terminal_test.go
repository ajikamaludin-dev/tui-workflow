package terminal

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyMsg
		expected []byte
	}{
		{
			name:     "Runes 'a'",
			msg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			expected: []byte("a"),
		},
		{
			name:     "Alt+a",
			msg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true},
			expected: []byte("\x1ba"),
		},
		{
			name:     "Enter",
			msg:      tea.KeyMsg{Type: tea.KeyEnter},
			expected: []byte{'\r'},
		},
		{
			name:     "Backspace",
			msg:      tea.KeyMsg{Type: tea.KeyBackspace},
			expected: []byte{0x7f},
		},
		{
			name:     "Tab",
			msg:      tea.KeyMsg{Type: tea.KeyTab},
			expected: []byte{'\t'},
		},
		{
			name:     "Ctrl+C",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: []byte{0x03},
		},
		{
			name:     "Ctrl+D",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlD},
			expected: []byte{0x04},
		},
		{
			name:     "Arrow Up",
			msg:      tea.KeyMsg{Type: tea.KeyUp},
			expected: []byte("\x1b[A"),
		},
		{
			name:     "Arrow Down",
			msg:      tea.KeyMsg{Type: tea.KeyDown},
			expected: []byte("\x1b[B"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := KeyToBytes(tc.msg)
			if string(res) != string(tc.expected) {
				t.Fatalf("expected %q, got %q", string(tc.expected), string(res))
			}
		})
	}
}

func TestTerminalViewLifecycle(t *testing.T) {
	term, err := NewTerminalView("term:test", 60, 15, "/bin/sh", "")
	if err != nil {
		t.Fatalf("NewTerminalView failed: %v", err)
	}
	defer term.Close()

	if term.ID != "term:test" {
		t.Errorf("expected ID term:test, got %s", term.ID)
	}

	// Write echo command
	cmd := []byte("echo HELLO_PTY_TEST\n")
	_, err = term.Write(cmd)
	if err != nil {
		t.Fatalf("term.Write failed: %v", err)
	}

	// Wait for output on channel
	select {
	case <-term.OutputChan():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal output")
	}

	// Give a bit of time for vt10x buffer update
	time.Sleep(100 * time.Millisecond)

	rendered := term.Render(60, 15, true)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 15 {
		t.Fatalf("expected 15 lines rendered, got %d", len(lines))
	}

	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w > 60 {
			t.Errorf("line %d visual width %d exceeds max 60", i, w)
		}
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, "HELLO_PTY_TEST") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Rendered output:\n%s", rendered)
	}

	// Test Resize
	term.Resize(70, 20)
	renderedResized := term.Render(70, 20, false)
	linesResized := strings.Split(renderedResized, "\n")
	if len(linesResized) != 20 {
		t.Fatalf("expected 20 lines after resize, got %d", len(linesResized))
	}

	// Test Close
	err = term.Close()
	if err != nil {
		t.Errorf("term.Close failed: %v", err)
	}

	// Multiple Close calls should be idempotent
	err = term.Close()
	if err != nil {
		t.Errorf("second term.Close failed: %v", err)
	}
}

func TestTerminalChurnMemory(t *testing.T) {
	runtime.GC()
	var mBaseline runtime.MemStats
	runtime.ReadMemStats(&mBaseline)

	for i := 0; i < 10; i++ {
		term, err := NewTerminalView(fmt.Sprintf("term:%d", i), 80, 24, "/bin/sh", "")
		if err != nil {
			t.Fatalf("failed to spawn terminal %d: %v", i, err)
		}
		_, _ = term.Write([]byte("echo hi\n"))
		time.Sleep(20 * time.Millisecond)
		_ = term.Close()
	}

	runtime.GC()
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)

	const maxAllowedBytes = 35 * 1024 * 1024
	if mAfter.Alloc > maxAllowedBytes {
		t.Errorf("Memory leak in terminal churn: Alloc %d bytes exceeds limit (%d)", mAfter.Alloc, maxAllowedBytes)
	}
}

func TestTerminalTrueColorAnd256ColorRender(t *testing.T) {
	term, err := NewTerminalView("term:color-test", 80, 10, "/bin/sh", "")
	if err != nil {
		t.Fatalf("failed to create terminal view: %v", err)
	}
	defer term.Close()

	// Feed 24-bit TrueColor, 256-color, and italic text directly to virtual terminal
	seq := []byte("\x1b[38;2;255;128;64m\x1b[48;2;10;20;30mTrueColor\x1b[0m " +
		"\x1b[38;5;196m256Color\x1b[0m " +
		"\x1b[3mItalicText\x1b[0m")

	term.mu.Lock()
	if term.vt != nil {
		_, _ = term.vt.Write(seq)
	}
	term.mu.Unlock()

	rendered := term.Render(80, 10, false)

	if !strings.Contains(rendered, "\x1b[38;2;255;128;64m") {
		t.Errorf("expected TrueColor foreground \\x1b[38;2;255;128;64m in rendered output, got: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[48;2;10;20;30m") {
		t.Errorf("expected TrueColor background \\x1b[48;2;10;20;30m in rendered output, got: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[38;5;196m") {
		t.Errorf("expected 256-color foreground \\x1b[38;5;196m in rendered output, got: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[3m") {
		t.Errorf("expected italic sequence \\x1b[3m in rendered output, got: %q", rendered)
	}

	// Verify all lines visual width <= 80
	for i, line := range strings.Split(rendered, "\n") {
		w := ansi.StringWidth(line)
		if w > 80 {
			t.Errorf("line %d visual width %d exceeds max 80", i, w)
		}
	}
}

