package terminal

import (
	"strings"
)

// terminalSelection holds local mouse selection state for the visible VT grid.
// It is directional-agnostic: use selectionRange() to obtain normalized bounds.
type terminalSelection struct {
	active           bool
	anchorX, anchorY int
	cursorX, cursorY int
}

func (t *TerminalView) clampGrid(x, y int) (int, int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= t.cols {
		x = t.cols - 1
	}
	if y >= t.rows {
		y = t.rows - 1
	}
	return x, y
}

// selectionRangeLocked returns normalized selection bounds.
// Caller must hold t.mu.
func (t *TerminalView) selectionRangeLocked() (sx, sy, ex, ey int, ok bool) {
	if !t.selection.active {
		return 0, 0, 0, 0, false
	}
	ax, ay := t.selection.anchorX, t.selection.anchorY
	cx, cy := t.selection.cursorX, t.selection.cursorY
	if ay > cy || (ay == cy && ax > cx) {
		return cx, cy, ax, ay, true
	}
	return ax, ay, cx, cy, true
}

// selectionContains reports whether cell (x, y) lies inside the linear selection.
func selectionContains(sx, sy, ex, ey, x, y int) bool {
	if y < sy || y > ey {
		return false
	}
	if sy == ey {
		return x >= sx && x <= ex
	}
	switch y {
	case sy:
		return x >= sx
	case ey:
		return x <= ex
	default:
		return true
	}
}

// StartSelection begins a local (visible-grid) selection at (x, y).
func (t *TerminalView) StartSelection(x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	x, y = t.clampGrid(x, y)
	t.selection = terminalSelection{active: true, anchorX: x, anchorY: y, cursorX: x, cursorY: y}
}

// UpdateSelection moves the selection cursor to (x, y).
func (t *TerminalView) UpdateSelection(x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.selection.active {
		return
	}
	x, y = t.clampGrid(x, y)
	t.selection.cursorX, t.selection.cursorY = x, y
}

// ClearSelection drops any local selection state.
func (t *TerminalView) ClearSelection() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.selection = terminalSelection{}
}

// HasSelection reports whether a non-zero-area selection exists.
func (t *TerminalView) HasSelection() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sx, sy, ex, ey, ok := t.selectionRangeLocked()
	if !ok {
		return false
	}
	return sx != ex || sy != ey
}

// SelectAll selects the entire visible grid.
func (t *TerminalView) SelectAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cols <= 0 || t.rows <= 0 {
		return
	}
	t.selection = terminalSelection{
		active:  true,
		anchorX: 0, anchorY: 0,
		cursorX: t.cols - 1, cursorY: t.rows - 1,
	}
}

// SelectedText returns the selected text reconstructed from the visible grid.
// Lines are trimmed of trailing spaces and joined with "\n". Returns "" when
// there is no selection (vt10x has no scrollback, so only visible rows are selectable).
func (t *TerminalView) SelectedText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	sx, sy, ex, ey, ok := t.selectionRangeLocked()
	if !ok || (sx == ex && sy == ey) {
		return ""
	}
	var sb strings.Builder
	for y := sy; y <= ey; y++ {
		from, to := 0, t.cols-1
		if y == sy {
			from = sx
		}
		if y == ey {
			to = ex
		}
		var line strings.Builder
		for x := from; x <= to; x++ {
			ch := ' '
			if t.vt != nil && y < t.rows && x < t.cols {
				ch = t.vt.Cell(x, y).Char
				if ch == 0 {
					ch = ' '
				}
			}
			line.WriteRune(ch)
		}
		if y > sy {
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.TrimRight(line.String(), " "))
	}
	return sb.String()
}
