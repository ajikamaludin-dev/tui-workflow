package editor

import (
	"strings"
	"unicode"
)

func (b *Buffer) MoveLeft() {
	b.endTypingGroup()
	b.CollapseCursors()
	if b.CursorCol > 0 {
		b.CursorCol--
	} else if b.CursorRow > 0 {
		b.CursorRow--
		b.CursorCol = len(b.Lines[b.CursorRow])
	}
}

func (b *Buffer) MoveRight() {
	b.endTypingGroup()
	b.CollapseCursors()
	if b.CursorRow < len(b.Lines) {
		if b.CursorCol < len(b.Lines[b.CursorRow]) {
			b.CursorCol++
		} else if b.CursorRow < len(b.Lines)-1 {
			b.CursorRow++
			b.CursorCol = 0
		}
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (b *Buffer) MoveWordLeft() {
	b.endTypingGroup()
	b.CollapseCursors()
	if b.CursorRow < 0 || b.CursorRow >= len(b.Lines) {
		return
	}
	if b.CursorCol == 0 {
		if b.CursorRow > 0 {
			b.CursorRow--
			b.CursorCol = len(b.Lines[b.CursorRow])
		}
		return
	}
	line := b.Lines[b.CursorRow]
	col := b.CursorCol
	if col > len(line) {
		col = len(line)
	}

	// 1. Skip whitespace to the left
	for col > 0 && unicode.IsSpace(line[col-1]) {
		col--
	}
	if col == 0 {
		b.CursorCol = 0
		return
	}

	// 2. Determine if word char vs punctuation
	if isWordChar(line[col-1]) {
		for col > 0 && isWordChar(line[col-1]) {
			col--
		}
	} else {
		for col > 0 && !isWordChar(line[col-1]) && !unicode.IsSpace(line[col-1]) {
			col--
		}
	}
	b.CursorCol = col
}

func (b *Buffer) MoveWordRight() {
	b.endTypingGroup()
	b.CollapseCursors()
	if b.CursorRow < 0 || b.CursorRow >= len(b.Lines) {
		return
	}
	line := b.Lines[b.CursorRow]
	if b.CursorCol >= len(line) {
		if b.CursorRow < len(b.Lines)-1 {
			b.CursorRow++
			b.CursorCol = 0
		}
		return
	}
	col := b.CursorCol
	if col < 0 {
		col = 0
	}

	// 1. Skip leading whitespace if cursor is on whitespace
	for col < len(line) && unicode.IsSpace(line[col]) {
		col++
	}
	if col >= len(line) {
		b.CursorCol = len(line)
		return
	}

	// 2. Skip word or punctuation
	if isWordChar(line[col]) {
		for col < len(line) && isWordChar(line[col]) {
			col++
		}
	} else {
		for col < len(line) && !isWordChar(line[col]) && !unicode.IsSpace(line[col]) {
			col++
		}
	}

	// 3. Skip trailing whitespace to place cursor at start of next word (VS Code style)
	for col < len(line) && unicode.IsSpace(line[col]) {
		col++
	}
	b.CursorCol = col
}

func (b *Buffer) MoveUp() {
	b.endTypingGroup()
	b.CollapseCursors()
	for b.CursorRow > 0 {
		b.CursorRow--
		if !b.IsHiddenLine(b.CursorRow) {
			break
		}
	}
	b.clampCol()
}

func (b *Buffer) MoveDown() {
	b.endTypingGroup()
	b.CollapseCursors()
	for b.CursorRow < len(b.Lines)-1 {
		b.CursorRow++
		if !b.IsHiddenLine(b.CursorRow) {
			break
		}
	}
	b.clampCol()
}

func (b *Buffer) MoveHome() {
	b.endTypingGroup()
	b.CollapseCursors()
	b.CursorCol = 0
}

func (b *Buffer) MoveEnd() {
	b.endTypingGroup()
	b.CollapseCursors()
	if b.CursorRow >= 0 && b.CursorRow < len(b.Lines) {
		b.CursorCol = len(b.Lines[b.CursorRow])
	}
}

func (b *Buffer) MovePageUp(n int) {
	b.endTypingGroup()
	b.CollapseCursors()
	b.CursorRow -= n
	if b.CursorRow < 0 {
		b.CursorRow = 0
	}
	b.clampCol()
}

func (b *Buffer) MovePageDown(n int) {
	b.endTypingGroup()
	b.CollapseCursors()
	b.CursorRow += n
	if b.CursorRow >= len(b.Lines) {
		b.CursorRow = len(b.Lines) - 1
	}
	b.clampCol()
}

func (b *Buffer) clampCursor() {
	if b.CursorRow < 0 {
		b.CursorRow = 0
	}
	if b.CursorRow >= len(b.Lines) {
		b.CursorRow = len(b.Lines) - 1
	}
	b.clampCol()
	b.clampExtraCursors()
}

func (b *Buffer) clampCol() {
	if b.CursorRow >= 0 && b.CursorRow < len(b.Lines) {
		lineLen := len(b.Lines[b.CursorRow])
		if b.CursorCol > lineLen {
			b.CursorCol = lineLen
		}
		if b.CursorCol < 0 {
			b.CursorCol = 0
		}
	}
}

// ClampCursor ensures CursorRow and CursorCol are within valid buffer bounds.
func (b *Buffer) ClampCursor() {
	b.clampCursor()
}

func (b *Buffer) SetCursor(row, col int) {
	b.endTypingGroup()
	b.CursorRow = row
	b.CursorCol = col
	b.clampCursor()
}

// NormalizedSelection returns (startRow, startCol, endRow, endCol) with start <= end.
func (b *Buffer) NormalizedSelection() (int, int, int, int) {
	if !b.HasSelection {
		return b.CursorRow, b.CursorCol, b.CursorRow, b.CursorCol
	}
	if b.SelectAnchorRow < b.CursorRow || (b.SelectAnchorRow == b.CursorRow && b.SelectAnchorCol <= b.CursorCol) {
		return b.SelectAnchorRow, b.SelectAnchorCol, b.CursorRow, b.CursorCol
	}
	return b.CursorRow, b.CursorCol, b.SelectAnchorRow, b.SelectAnchorCol
}

// IsSelected returns true if position (row, col) is within current selection.
func (b *Buffer) IsSelected(row, col int) bool {
	if !b.HasSelection {
		return false
	}
	sRow, sCol, eRow, eCol := b.NormalizedSelection()
	if sRow == eRow && sCol == eCol {
		return false
	}
	if row < sRow || row > eRow {
		return false
	}
	if row > sRow && row < eRow {
		return true
	}
	if sRow == eRow {
		return col >= sCol && col < eCol
	}
	if row == sRow {
		return col >= sCol
	}
	if row == eRow {
		return col < eCol
	}
	return false
}

// StartSelection initiates selection anchor at (row, col).
func (b *Buffer) StartSelection(row, col int) {
	b.SelectAnchorRow = row
	b.SelectAnchorCol = col
	b.CursorRow = row
	b.CursorCol = col
	b.clampCursor()
	b.HasSelection = false
}

// UpdateSelection extends selection to (row, col).
func (b *Buffer) UpdateSelection(row, col int) {
	b.CursorRow = row
	b.CursorCol = col
	b.clampCursor()
	if b.CursorRow != b.SelectAnchorRow || b.CursorCol != b.SelectAnchorCol {
		b.HasSelection = true
	}
}

// ClearSelection removes any active selection.
func (b *Buffer) ClearSelection() {
	b.HasSelection = false
}

func (b *Buffer) ensureSelectionAnchor() {
	if !b.HasSelection {
		b.SelectAnchorRow = b.CursorRow
		b.SelectAnchorCol = b.CursorCol
	}
}

func (b *Buffer) updateSelectionAfterMove() {
	b.clampCursor()
	if b.CursorRow == b.SelectAnchorRow && b.CursorCol == b.SelectAnchorCol {
		b.HasSelection = false
	} else {
		b.HasSelection = true
	}
}

// SelectLeft moves cursor left while extending selection.
func (b *Buffer) SelectLeft() {
	b.ensureSelectionAnchor()
	b.MoveLeft()
	b.updateSelectionAfterMove()
}

// SelectRight moves cursor right while extending selection.
func (b *Buffer) SelectRight() {
	b.ensureSelectionAnchor()
	b.MoveRight()
	b.updateSelectionAfterMove()
}

// SelectUp moves cursor up while extending selection.
func (b *Buffer) SelectUp() {
	b.ensureSelectionAnchor()
	b.MoveUp()
	b.updateSelectionAfterMove()
}

// SelectDown moves cursor down while extending selection.
func (b *Buffer) SelectDown() {
	b.ensureSelectionAnchor()
	b.MoveDown()
	b.updateSelectionAfterMove()
}

// SelectWordLeft moves cursor word left while extending selection.
func (b *Buffer) SelectWordLeft() {
	b.ensureSelectionAnchor()
	b.MoveWordLeft()
	b.updateSelectionAfterMove()
}

// SelectWordRight moves cursor word right while extending selection.
func (b *Buffer) SelectWordRight() {
	b.ensureSelectionAnchor()
	b.MoveWordRight()
	b.updateSelectionAfterMove()
}

// SelectAll selects entire buffer content.
func (b *Buffer) SelectAll() {
	if len(b.Lines) == 0 {
		return
	}
	b.SelectAnchorRow = 0
	b.SelectAnchorCol = 0
	b.CursorRow = len(b.Lines) - 1
	b.CursorCol = len(b.Lines[b.CursorRow])
	b.HasSelection = true
}

// GetSelectedText returns currently selected text.
func (b *Buffer) GetSelectedText() string {
	if !b.HasSelection {
		return ""
	}
	sRow, sCol, eRow, eCol := b.NormalizedSelection()
	if sRow == eRow {
		if sRow >= len(b.Lines) {
			return ""
		}
		line := b.Lines[sRow]
		if sCol >= len(line) || sCol >= eCol {
			return ""
		}
		if eCol > len(line) {
			eCol = len(line)
		}
		return string(line[sCol:eCol])
	}

	var sb strings.Builder
	for r := sRow; r <= eRow && r < len(b.Lines); r++ {
		line := b.Lines[r]
		if r == sRow {
			if sCol < len(line) {
				sb.WriteString(string(line[sCol:]))
			}
			sb.WriteByte('\n')
		} else if r == eRow {
			if eCol > len(line) {
				eCol = len(line)
			}
			if eCol > 0 {
				sb.WriteString(string(line[:eCol]))
			}
		} else {
			sb.WriteString(string(line))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
