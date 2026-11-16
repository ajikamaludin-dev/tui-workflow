package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
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

// ScrollbackLimit caps how many lines of scrollback history are kept per terminal.
const ScrollbackLimit = 1000

// scrollbackCaptureDepth is how many rows are deep-copied before every write
// so lines scrolled off the top can be attributed. Normal output scrolls one
// line per LF sub-chunk, but newline-free chunks (auto-wrap) can scroll
// several rows at once.
const scrollbackCaptureDepth = 4

const (
	fnvOffset64 = uint64(14695981039346656037)
	fnvPrime64  = uint64(1099511628211)
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
	// CursorHidden is toggled by the app's blink ticker (true = blink-off phase).
	CursorHidden bool

	// Scrollback state. vt10x keeps only the visible grid and drops lines that
	// scroll off the top, so history is captured here by diffing the grid around
	// every write (see feedLocked).
	scrollback   [][]vt10x.Glyph // rows that scrolled off the top, oldest first
	scrollOffset int             // lines the viewport is scrolled up from the live bottom; 0 = live
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
		Title:      "Terminal",
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
				term.feedLocked(buf[:n])
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

// WriteToVT writes ANSI escape sequences directly into the internal vt10x virtual terminal emulator.
func (t *TerminalView) WriteToVT(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.vt == nil {
		return 0, nil
	}
	t.feedLocked(p)
	return len(p), nil
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

// chunkHasScrollTrigger reports whether the chunk contains bytes that can
// scroll the grid (LF, VT, FF). Only LF occurs in typical output, but the
// full grid is deep-copied anyway when one is present.
func chunkHasScrollTrigger(p []byte) bool {
	for i := 0; i < len(p); i++ {
		if p[i] == '\n' || p[i] == '\v' || p[i] == '\f' {
			return true
		}
	}
	return false
}

// glyphHash folds a cell into an FNV-1a accumulator. All vt10x Glyph fields
// are hashed so any attribute change is detected.
func glyphHash(h uint64, g vt10x.Glyph) uint64 {
	h ^= uint64(g.Char)
	h *= fnvPrime64
	h ^= uint64(g.FG)
	h *= fnvPrime64
	h ^= uint64(g.BG)
	h *= fnvPrime64
	h ^= uint64(g.Mode)
	h *= fnvPrime64
	return h
}

func hashRow(row []vt10x.Glyph) uint64 {
	h := fnvOffset64
	for i := range row {
		h = glyphHash(h, row[i])
	}
	return h
}

// preWriteSnap holds the state needed to attribute the lines a write displaces.
// Rows are deep-copied only when the chunk can scroll multiple rows at once;
// otherwise only per-row hashes are kept.
type preWriteSnap struct {
	hashes []uint64        // per-row FNV-1a hashes of the grid before the write
	rows   [][]vt10x.Glyph // deep copies of the top rows, or the whole grid for trigger chunks
}

// preWriteSnapshotLocked captures the grid before a vt10x write. Caller must
// hold t.mu. When copyRows is false only whole-grid hashes are recorded.
func (t *TerminalView) preWriteSnapshotLocked(copyRows bool) *preWriteSnap {
	if t.vt == nil {
		return nil
	}
	depth := scrollbackCaptureDepth
	if copyRows || depth > t.rows {
		depth = t.rows
	}
	snap := &preWriteSnap{
		hashes: make([]uint64, t.rows),
		rows:   make([][]vt10x.Glyph, 0, depth),
	}
	for y := 0; y < t.rows; y++ {
		if y < depth {
			row := make([]vt10x.Glyph, t.cols)
			for x := 0; x < t.cols; x++ {
				c := t.vt.Cell(x, y)
				row[x] = c
			}
			snap.rows = append(snap.rows, row)
			snap.hashes[y] = hashRow(row)
			continue
		}
		h := fnvOffset64
		for x := 0; x < t.cols; x++ {
			h = glyphHash(h, t.vt.Cell(x, y))
		}
		snap.hashes[y] = h
	}
	return snap
}

// detectHashShift reports how many lines scrolled off the top between two
// per-row hash snapshots. A scroll is signaled by rows DISAPPEARING, so the
// shift must explain more rows than a no-scroll edit does: the smallest k in
// [1, rows-1] whose aligned rows hash-equal more often than the identity
// alignment wins. This rejects in-place edits and cursor redraws (their
// identity alignment dominates) while matching LF-at-bottom scrolls (rows
// moved, identity alignment explains almost nothing). Blank rows hash alike,
// so a grid with no visible change reports no scroll. k == rows is excluded
// (imposes no constraints); newline-free chunks that scrolled multiple rows
// (auto-wrap) under-detect by one shift and are captured best-effort.
func detectHashShift(oldHashes, newHashes []uint64) int {
	rows := len(newHashes)
	if rows == 0 || len(oldHashes) != rows {
		return 0
	}
	base := 0
	for y := 0; y < rows; y++ {
		if oldHashes[y] == newHashes[y] {
			base++
		}
	}
	if base == rows {
		return 0
	}
	for k := 1; k < rows; k++ {
		m := 0
		for y := 0; y < rows-k; y++ {
			if newHashes[y] == oldHashes[y+k] {
				m++
			}
		}
		if m > base {
			return k
		}
	}
	return 0
}

// captureScrollbackLocked captures displaced lines after a vt10x write.
// pre is the snapshot taken before the write; caller must hold t.mu.
// Alternate-screen output is never captured and snaps the viewport to live.
func (t *TerminalView) captureScrollbackLocked(pre *preWriteSnap) {
	if t.vt == nil {
		return
	}
	if (t.vt.Mode() & vt10x.ModeAltScreen) != 0 {
		t.scrollOffset = 0
		return
	}
	if pre == nil || len(pre.hashes) != t.rows {
		return
	}
	newHashes := make([]uint64, t.rows)
	for y := 0; y < t.rows; y++ {
		h := fnvOffset64
		for x := 0; x < t.cols; x++ {
			h = glyphHash(h, t.vt.Cell(x, y))
		}
		newHashes[y] = h
	}
	k := detectHashShift(pre.hashes, newHashes)
	if k == 0 {
		return
	}
	nCap := k
	if nCap > len(pre.rows) {
		nCap = len(pre.rows)
	}
	for i := 0; i < nCap; i++ {
		row := make([]vt10x.Glyph, len(pre.rows[i]))
		copy(row, pre.rows[i])
		t.scrollback = append(t.scrollback, row)
	}
	if overflow := len(t.scrollback) - ScrollbackLimit; overflow > 0 {
		t.scrollback = t.scrollback[overflow:]
	}
	if t.scrollOffset > 0 {
		// Anchor the viewport to the same lines as new history arrives.
		t.scrollOffset += nCap
		if t.scrollOffset > len(t.scrollback) {
			t.scrollOffset = len(t.scrollback)
		}
	}
}

// feedLocked writes raw bytes into vt10x in newline-aligned sub-chunks and
// captures displaced lines after each one. Splitting at 0x0A/0x0B/0x0C never
// splits a UTF-8 rune (continuation bytes are ≥ 0x80), which a raw arbitrary
// split would — vt10x drops truncated trailing runes whole. Caller holds t.mu.
func (t *TerminalView) feedLocked(p []byte) {
	for len(p) > 0 {
		end := -1
		for i := 0; i < len(p); i++ {
			if p[i] == '\n' || p[i] == '\v' || p[i] == '\f' {
				end = i + 1
				break
			}
		}
		chunk := p
		if end >= 0 {
			chunk = p[:end]
			p = p[end:]
		} else {
			p = nil
		}
		if (t.vt.Mode() & vt10x.ModeAltScreen) != 0 {
			// Alternate screen (vim/htop/less): no scrollback exists here.
			_, _ = t.vt.Write(chunk)
			t.captureScrollbackLocked(nil) // snaps viewport to live
			continue
		}
		trigger := chunkHasScrollTrigger(chunk)
		// Deep-copy the whole grid only when the chunk is long enough to have
		// wrapped several rows (each wrap may scroll). Short LF lines need the
		// top scrollbackCaptureDepth rows only.
		pre := t.preWriteSnapshotLocked(trigger && len(chunk) > t.cols*2)
		_, _ = t.vt.Write(chunk)
		t.captureScrollbackLocked(pre)
	}
}

// ScrollBack scrolls the viewport up into the scrollback history by n lines.
func (t *TerminalView) ScrollBack(lines int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scrollOffset += lines
	if t.scrollOffset > len(t.scrollback) {
		t.scrollOffset = len(t.scrollback)
	}
}

// ScrollForward scrolls the viewport down toward the live output by n lines.
func (t *TerminalView) ScrollForward(lines int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scrollOffset -= lines
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}

// ScrollToBottom snaps the viewport back to the live output.
func (t *TerminalView) ScrollToBottom() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scrollOffset = 0
}

// IsScrolled reports whether the viewport is currently showing scrollback history.
func (t *TerminalView) IsScrolled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.scrollOffset > 0
}

// AltScreenActive reports whether the inner application is using the alternate
// screen (e.g. vim, htop, less), where scrollback history does not apply.
func (t *TerminalView) AltScreenActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.vt != nil && t.vt.Mode()&vt10x.ModeAltScreen != 0
}

// SendArrowUp writes n Up-arrow escape sequences to the PTY. Used for
// alternate-screen applications (where scrollback does not exist) so wheel
// input keeps their legacy "arrow keys scroll" behavior.
func (t *TerminalView) SendArrowUp(lines int) {
	for i := 0; i < lines; i++ {
		_, _ = t.Write([]byte("\x1b[A"))
	}
}

// SendArrowDown writes n Down-arrow escape sequences to the PTY.
func (t *TerminalView) SendArrowDown(lines int) {
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
	// Viewport is scrolled into scrollback history: render history rows and
	// suppress live-grid decorations (cursor and selection highlight).
	scrolled := t.scrollOffset > 0
	if scrolled {
		selActive = false
	}

	cur := t.vt.Cursor()
	curVis := t.vt.CursorVisible()

	// Build the rows to paint: a window into scrollback+screen when scrolled,
	// the live grid otherwise. Rows are deep-copied here so the render loop
	// does not hit vt10x while iterating.
	viewRows := make([][]vt10x.Glyph, 0, height)
	fillRow := func(y int) []vt10x.Glyph {
		row := make([]vt10x.Glyph, t.cols)
		for x := 0; x < t.cols; x++ {
			row[x] = t.vt.Cell(x, y)
		}
		return row
	}
	if scrolled {
		composite := make([][]vt10x.Glyph, 0, len(t.scrollback)+t.rows)
		composite = append(composite, t.scrollback...)
		for y := 0; y < t.rows; y++ {
			composite = append(composite, fillRow(y))
		}
		end := len(composite) - t.scrollOffset
		start := end - height
		if start < 0 {
			start = 0
		}
		viewRows = composite[start:end]
	} else {
		for y := 0; y < height; y++ {
			if y < t.rows {
				viewRows = append(viewRows, fillRow(y))
			} else {
				viewRows = append(viewRows, nil)
			}
		}
	}

	lines := make([]string, height)

	for y := 0; y < height; y++ {
		var sb strings.Builder
		if y >= len(viewRows) {
			// Pathological underflow (e.g. scrollOffset beyond available
			// history after a shrink): paint a blank row.
			viewRows = append(viewRows, nil)
		}
		lastFG := vt10x.DefaultFG
		lastBG := vt10x.DefaultBG
		lastInverse := false
		lastBlink := false
		lastUnderline := false
		lastBold := false
		lastItalic := false

		for x := 0; x < width; x++ {
			attr := vt10x.Glyph{Char: ' ', FG: vt10x.DefaultFG, BG: vt10x.DefaultBG}
			if row := viewRows[y]; row != nil && x < len(row) {
				attr = row[x]
			}

			isCursor := focused && !scrolled && curVis && !t.CursorHidden && (x == cur.X && y == cur.Y)
			// SGR blink attribute from the child application
			isBlink := (attr.Mode & vtAttrBlink) != 0
			isSelected := selActive && selectionContains(selStartX, selStartY, selEndX, selEndY, x, y)
			isInverse := isCursor || isSelected || ((attr.Mode & vtAttrReverse) != 0)
			isUnderline := (attr.Mode & vtAttrUnderline) != 0
			isBold := (attr.Mode & vtAttrBold) != 0
			isItalic := (attr.Mode & vtAttrItalic) != 0

			styleChanged := (attr.FG != lastFG) || (attr.BG != lastBG) ||
				(isInverse != lastInverse) || (isBlink != lastBlink) || (isUnderline != lastUnderline) ||
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
				if isBlink {
					sb.WriteString("\x1b[5m")
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
				lastBlink = isBlink
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

// KeyToBytes converts a Bubble Tea KeyPressMsg to raw bytes expected by the PTY.
func KeyToBytes(msg tea.KeyPressMsg) []byte {
	isAlt := msg.Mod.Contains(tea.ModAlt)
	isCtrl := msg.Mod.Contains(tea.ModCtrl)

	switch msg.Keystroke() {
	case "alt+left", "ctrl+left":
		return []byte("\x1bb")
	case "alt+right", "ctrl+right":
		return []byte("\x1bf")
	case "alt+backspace":
		return []byte("\x1b\x7f")
	case "ctrl+backspace":
		return []byte{0x17}
	case "shift+tab":
		return []byte("\x1b[Z")
	}

	// Printable text (Alt prefixes an ESC byte, as in legacy terminals)
	if msg.Text != "" {
		if isAlt {
			return append([]byte{0x1b}, msg.Text...)
		}
		return []byte(msg.Text)
	}

	switch msg.Code {
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		if isAlt {
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
		if isAlt {
			return []byte("\x1bf")
		}
		return []byte("\x1b[C")
	case tea.KeyLeft:
		if isAlt {
			return []byte("\x1bb")
		}
		return []byte("\x1b[D")
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

	// Ctrl combos: Ctrl+Space/@, Ctrl+A..Z, Ctrl+[\]^_ (and friends) → 0x00–0x1F
	if isCtrl {
		switch {
		case msg.Code == tea.KeySpace:
			return []byte{0x00}
		case msg.Code >= 'a' && msg.Code <= 'z':
			return []byte{byte(msg.Code - 'a' + 1)}
		case msg.Code >= '@' && msg.Code <= '_':
			return []byte{byte(msg.Code & 0x1f)}
		}
	}

	// Alt+printable fallback for legacy terminals (ESC + char)
	if isAlt && msg.Code >= 0x21 && msg.Code <= 0x7e {
		return append([]byte{0x1b}, string(msg.Code)...)
	}

	return nil
}
