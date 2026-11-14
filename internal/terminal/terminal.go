package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

const (
	vtAttrReverse   = 1 << 0 // 1
	vtAttrUnderline = 1 << 1 // 2
	vtAttrBold      = 1 << 2 // 4
	vtAttrGfx       = 1 << 3 // 8
	vtAttrItalic    = 1 << 4 // 16
	vtAttrBlink     = 1 << 5 // 32
)

// TerminalView wraps an OS pseudo-terminal (PTY) and a VT100/Xterm virtual terminal emulator.
type TerminalView struct {
	ID         string
	Title      string
	ptyFile    *os.File
	cmd        *exec.Cmd
	vt         vt10x.Terminal
	cols       int
	rows       int
	outputChan chan struct{}
	done       chan struct{}
	doneOnce   sync.Once
	closeOnce  sync.Once
	mu         sync.RWMutex
	closed     bool
	exited     bool
	exitErr    error
	selection  terminalSelection
}

// NewTerminalView spawns a new shell process attached to a PTY.
func NewTerminalView(id string, cols, rows int, shell string, cwd string) (*TerminalView, error) {
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
			if _, err := os.Stat(shell); err != nil {
				shell = "/bin/sh"
			}
		}
	}

	cmd := exec.Command(shell)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	vt := vt10x.New(vt10x.WithSize(cols, rows))

	term := &TerminalView{
		ID:         id,
		Title:      ">_ " + getBaseName(shell),
		ptyFile:    ptyFile,
		cmd:        cmd,
		vt:         vt,
		cols:       cols,
		rows:       rows,
		outputChan: make(chan struct{}, 100),
		done:       make(chan struct{}),
	}

	// Reader goroutine
	go func() {
		defer func() {
			term.closeOnce.Do(func() {
				close(term.outputChan)
			})
		}()
		buf := make([]byte, 8192)
		for {
			select {
			case <-term.done:
				return
			default:
			}

			n, readErr := ptyFile.Read(buf)
			if n > 0 {
				term.mu.Lock()
				if term.vt != nil {
					_, _ = term.vt.Write(buf[:n])
				}
				term.mu.Unlock()

				select {
				case <-term.done:
					return
				case term.outputChan <- struct{}{}:
				default:
				}
			}
			if readErr != nil {
				term.mu.Lock()
				term.exited = true
				term.exitErr = readErr
				term.mu.Unlock()
				return
			}
		}
	}()

	return term, nil
}

func getBaseName(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return "terminal"
}

// OutputChan returns channel that receives notification when PTY outputs data.
func (t *TerminalView) OutputChan() <-chan struct{} {
	return t.outputChan
}

// Write writes bytes into PTY stdin.
func (t *TerminalView) Write(p []byte) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.ptyFile == nil {
		return 0, os.ErrClosed
	}
	return t.ptyFile.Write(p)
}

// Resize resizes the PTY and virtual terminal grid.
func (t *TerminalView) Resize(cols, rows int) {
	if cols < 10 || rows < 2 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cols == cols && t.rows == rows {
		return
	}
	t.cols = cols
	t.rows = rows

	if t.ptyFile != nil && !t.closed {
		_ = pty.Setsize(t.ptyFile, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
	if t.vt != nil {
		t.vt.Resize(cols, rows)
	}
}

// Close terminates process and cleans up PTY.
func (t *TerminalView) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()

	t.doneOnce.Do(func() {
		close(t.done)
	})

	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGHUP)
		go func(p *os.Process) {
			time.Sleep(150 * time.Millisecond)
			_ = p.Kill()
		}(t.cmd.Process)
	}

	var closeErr error
	if t.ptyFile != nil {
		closeErr = t.ptyFile.Close()
	}
	return closeErr
}

// IsExited returns whether underlying shell process has exited.
func (t *TerminalView) IsExited() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.exited
}

// HasRunningProcess checks if the terminal's shell has spawned active child processes.
func (t *TerminalView) HasRunningProcess() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.exited || t.cmd == nil || t.cmd.Process == nil {
		return false
	}
	return hasRunningProcess(t.ptyFile, t.cmd.Process.Pid)
}

// ScrollUp sends Up arrow escape sequences to the PTY.
func (t *TerminalView) ScrollUp(lines int) {
	for i := 0; i < lines; i++ {
		_, _ = t.Write([]byte("\x1b[A"))
	}
}

// ScrollDown sends Down arrow escape sequences to the PTY.
func (t *TerminalView) ScrollDown(lines int) {
	for i := 0; i < lines; i++ {
		_, _ = t.Write([]byte("\x1b[B"))
	}
}

// Render formats the VT10X terminal screen buffer into an ANSI-formatted string.
// Exactly height lines are returned, and each line is clamped to width characters.
func (t *TerminalView) Render(width, height int, focused bool) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Auto-resize if dimensions changed
	if width != t.cols || height != t.rows {
		t.Resize(width, height)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vt == nil {
		return strings.Repeat(" \n", height)
	}

	selStartX, selStartY, selEndX, selEndY, selActive := t.selectionRangeLocked()

	cur := t.vt.Cursor()
	curVis := t.vt.CursorVisible()

	lines := make([]string, height)

	for y := 0; y < height; y++ {
		var sb strings.Builder
		lastFG := vt10x.DefaultFG
		lastBG := vt10x.DefaultBG
		lastInverse := false
		lastUnderline := false
		lastBold := false
		lastItalic := false

		for x := 0; x < width; x++ {
			var attr vt10x.Glyph
			if y < t.rows && x < t.cols {
				attr = t.vt.Cell(x, y)
			} else {
				attr = vt10x.Glyph{Char: ' ', FG: vt10x.DefaultFG, BG: vt10x.DefaultBG}
			}

			isCursor := focused && curVis && (x == cur.X && y == cur.Y)
			isSelected := selActive && selectionContains(selStartX, selStartY, selEndX, selEndY, x, y)
			isInverse := isCursor || isSelected || ((attr.Mode & vtAttrReverse) != 0)
			isUnderline := (attr.Mode & vtAttrUnderline) != 0
			isBold := (attr.Mode & vtAttrBold) != 0
			isItalic := (attr.Mode & vtAttrItalic) != 0

			styleChanged := (attr.FG != lastFG) || (attr.BG != lastBG) ||
				(isInverse != lastInverse) || (isUnderline != lastUnderline) ||
				(isBold != lastBold) || (isItalic != lastItalic)

			if styleChanged {
				// Reset then apply attributes
				sb.WriteString("\x1b[0m")
				if isBold {
					sb.WriteString("\x1b[1m")
				}
				if isItalic {
					sb.WriteString("\x1b[3m")
				}
				if isUnderline {
					sb.WriteString("\x1b[4m")
				}
				if isInverse {
					sb.WriteString("\x1b[7m")
				}

				// Foreground color: 256-color or 24-bit TrueColor (RGB)
				if attr.FG != vt10x.DefaultFG {
					if attr.FG < 256 {
						sb.WriteString(fmt.Sprintf("\x1b[38;5;%dm", attr.FG))
					} else if attr.FG < vt10x.DefaultFG {
						r := (attr.FG >> 16) & 0xFF
						g := (attr.FG >> 8) & 0xFF
						b := attr.FG & 0xFF
						sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b))
					}
				}

				// Background color: 256-color or 24-bit TrueColor (RGB)
				if attr.BG != vt10x.DefaultBG {
					if attr.BG < 256 {
						sb.WriteString(fmt.Sprintf("\x1b[48;5;%dm", attr.BG))
					} else if attr.BG < vt10x.DefaultFG {
						r := (attr.BG >> 16) & 0xFF
						g := (attr.BG >> 8) & 0xFF
						b := attr.BG & 0xFF
						sb.WriteString(fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b))
					}
				}

				lastFG = attr.FG
				lastBG = attr.BG
				lastInverse = isInverse
				lastUnderline = isUnderline
				lastBold = isBold
				lastItalic = isItalic
			}

			ch := attr.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}

		// Reset styling at end of line
		sb.WriteString("\x1b[0m")

		// Clamp line to exactly width visual cells to prevent terminal scroll overflow
		lineStr := ansi.Truncate(sb.String(), width, "")
		lines[y] = lineStr
	}

	return strings.Join(lines, "\n")
}

// KeyToBytes converts a Bubble Tea KeyMsg to raw bytes expected by the PTY.
func KeyToBytes(msg tea.KeyMsg) []byte {
	switch msg.String() {
	case "alt+left", "ctrl+left":
		return []byte("\x1bb")
	case "alt+right", "ctrl+right":
		return []byte("\x1bf")
	case "alt+backspace":
		return []byte("\x1b\x7f")
	case "ctrl+backspace":
		return []byte{0x17}
	}

	if msg.Type == tea.KeyRunes {
		if msg.Alt {
			return append([]byte{0x1b}, []byte(string(msg.Runes))...)
		}
		return []byte(string(msg.Runes))
	}

	switch msg.Type {
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		if msg.Alt {
			return []byte("\x1b\x7f")
		}
		return []byte{0x7f}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		if msg.Alt {
			return []byte("\x1bf")
		}
		return []byte("\x1b[C")
	case tea.KeyLeft:
		if msg.Alt {
			return []byte("\x1bb")
		}
		return []byte("\x1b[D")
	case tea.KeyCtrlLeft:
		return []byte("\x1bb")
	case tea.KeyCtrlRight:
		return []byte("\x1bf")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyInsert:
		return []byte("\x1b[2~")
	case tea.KeyShiftTab:
		return []byte("\x1b[Z")
	case tea.KeyF1:
		return []byte("\x1bOP")
	case tea.KeyF2:
		return []byte("\x1bOQ")
	case tea.KeyF3:
		return []byte("\x1bOR")
	case tea.KeyF4:
		return []byte("\x1bOS")
	case tea.KeyF5:
		return []byte("\x1b[15~")
	case tea.KeyF6:
		return []byte("\x1b[17~")
	case tea.KeyF7:
		return []byte("\x1b[18~")
	case tea.KeyF8:
		return []byte("\x1b[19~")
	case tea.KeyF9:
		return []byte("\x1b[20~")
	case tea.KeyF10:
		return []byte("\x1b[21~")
	case tea.KeyF11:
		return []byte("\x1b[23~")
	case tea.KeyF12:
		return []byte("\x1b[24~")
	}

	// Control keys
	if msg.Type >= tea.KeyCtrlAt && msg.Type <= tea.KeyCtrlUnderscore {
		return []byte{byte(msg.Type - tea.KeyCtrlAt)}
	}

	return nil
}
