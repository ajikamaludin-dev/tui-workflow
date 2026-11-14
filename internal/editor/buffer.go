package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/syntax"
	"tui-workflow/internal/ui"
)

type VisualLine struct {
	PhysicalLineIdx int
	IsContinuation  bool
	Runes           []rune
	StartCol        int
	EndCol          int
}

type BufferSnapshot struct {
	Lines        [][]rune
	CursorRow    int
	CursorCol    int
	ExtraCursors []extraCursor
}

// extraCursor adalah cursor tambahan (multi-cursor) dengan selection sendiri.
// Cursor utama (primary) tetap di Buffer.CursorRow/CursorCol + HasSelection.
type extraCursor struct {
	Row, Col         int
	SelAnchorRow     int
	SelAnchorCol     int
	HasSelection     bool
}

// FoldRegion menyembunyikan baris (Start, End]; baris Start tetap terlihat
// dan diberi marker ▸. Fold berbasis indentasi, per-tab, tidak dipersistenkan.
type FoldRegion struct {
	Start int
	End   int
}

type Buffer struct {
	Lines        [][]rune
	CursorRow    int
	CursorCol    int
	ScrollRow    int
	ScrollCol    int
	SoftWrap     bool
	IsDirty      bool
	Highlighter  *syntax.Highlighter
	cachedHigh   []string
	cachedStyles   [][]string
	cachedRawLines []string
	cachedPath   string
	cachedDirty  bool
	cachedText   string
	cachedTextDirty bool

	// Selection state
	HasSelection    bool
	SelectAnchorRow int
	SelectAnchorCol int

	// Search state
	SearchQuery    string
	SearchMatches  []MatchPos
	ActiveMatchIdx int

	// Undo / Redo state
	undoStack    []BufferSnapshot
	redoStack    []BufferSnapshot
	maxUndoStack int
	typingGroup  bool
	inUndoRedo   bool

	// Multi-cursor state (primary selalu ada di CursorRow/CursorCol)
	ExtraCursors []extraCursor

	// Fold state (indent-based, hidden lines dilewati renderer)
	Folded []FoldRegion

	// Indent Rainbow state & diagnostics
	IndentRainbow   bool
	LineDiagnostics map[int]string
}

func NewBuffer(content string) *Buffer {
	b := &Buffer{
		Lines:           make([][]rune, 0),
		CursorRow:       0,
		CursorCol:       0,
		ScrollRow:       0,
		ScrollCol:       0,
		SoftWrap:        true,
		IsDirty:         false,
		Highlighter:     syntax.NewHighlighter(),
		cachedDirty:     true,
		cachedTextDirty: true,
		maxUndoStack:    100,
		IndentRainbow:   true,
		LineDiagnostics: make(map[int]string),
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")

	for _, rl := range rawLines {
		b.Lines = append(b.Lines, []rune(rl))
	}

	if len(b.Lines) == 0 {
		b.Lines = append(b.Lines, []rune{})
	}

	return b
}

func (b *Buffer) markContentChanged() {
	b.cachedDirty = true
	b.cachedTextDirty = true
}

// InsertTopImport prepends an import line at the top of the buffer.
func (b *Buffer) InsertTopImport(importLine string) {
	importLine = strings.TrimRight(importLine, "\r\n")
	b.Lines = append([][]rune{[]rune(importLine)}, b.Lines...)
	b.CursorRow++
	b.markContentChanged()
}

func (b *Buffer) Text() string {
	if !b.cachedTextDirty && b.cachedText != "" && len(b.Lines) > 0 {
		return b.cachedText
	}
	var sb strings.Builder
	for i, l := range b.Lines {
		sb.WriteString(string(l))
		if i < len(b.Lines)-1 {
			sb.WriteByte('\n')
		}
	}
	b.cachedText = sb.String()
	b.cachedTextDirty = false
	return b.cachedText
}

// SetText replaces the buffer content with a new string.
func (b *Buffer) SetText(content string) {
	b.Lines = make([][]rune, 0)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")
	for _, rl := range rawLines {
		b.Lines = append(b.Lines, []rune(rl))
	}
	if len(b.Lines) == 0 {
		b.Lines = append(b.Lines, []rune{})
	}
	b.clampCursor()
	b.markContentChanged()
	b.cachedHigh = nil
	b.cachedStyles = nil
	b.cachedRawLines = nil
	b.undoStack = nil
	b.redoStack = nil
	b.typingGroup = false
}

// ReplaceContentWithUndo replaces buffer content while recording an undo snapshot.
// Allows operations like code formatting to be undone via Ctrl+Z.
func (b *Buffer) ReplaceContentWithUndo(content string) {
	b.endTypingGroup()
	b.pushUndo()

	b.Lines = make([][]rune, 0)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")
	for _, rl := range rawLines {
		b.Lines = append(b.Lines, []rune(rl))
	}
	if len(b.Lines) == 0 {
		b.Lines = append(b.Lines, []rune{})
	}

	b.clampCursor()
	b.markContentChanged()
	b.cachedHigh = nil
	b.cachedStyles = nil
	b.cachedRawLines = nil
	b.IsDirty = true
}

func (b *Buffer) LineCount() int {
	return len(b.Lines)
}

func (b *Buffer) CurrentLineLen() int {
	if b.CursorRow >= 0 && b.CursorRow < len(b.Lines) {
		return len(b.Lines[b.CursorRow])
	}
	return 0
}

func cloneRuneLines(lines [][]rune) [][]rune {
	res := make([][]rune, len(lines))
	for i, l := range lines {
		cp := make([]rune, len(l))
		copy(cp, l)
		res[i] = cp
	}
	return res
}

func cloneExtraCursors(cs []extraCursor) []extraCursor {
	if len(cs) == 0 {
		return nil
	}
	out := make([]extraCursor, len(cs))
	copy(out, cs)
	return out
}

func (b *Buffer) endTypingGroup() {
	b.typingGroup = false
}

func (b *Buffer) pushUndo() {
	if b.inUndoRedo {
		return
	}
	if b.maxUndoStack <= 0 {
		b.maxUndoStack = 100
	}
	snap := BufferSnapshot{
		Lines:        cloneRuneLines(b.Lines),
		CursorRow:    b.CursorRow,
		CursorCol:    b.CursorCol,
		ExtraCursors: cloneExtraCursors(b.ExtraCursors),
	}
	b.undoStack = append(b.undoStack, snap)
	if len(b.undoStack) > b.maxUndoStack {
		b.undoStack = b.undoStack[1:]
	}
	b.redoStack = nil
}

func (b *Buffer) Undo() bool {
	b.endTypingGroup()
	if len(b.undoStack) == 0 {
		return false
	}
	currSnap := BufferSnapshot{
		Lines:        cloneRuneLines(b.Lines),
		CursorRow:    b.CursorRow,
		CursorCol:    b.CursorCol,
		ExtraCursors: cloneExtraCursors(b.ExtraCursors),
	}
	b.redoStack = append(b.redoStack, currSnap)
	if len(b.redoStack) > b.maxUndoStack {
		b.redoStack = b.redoStack[1:]
	}

	lastIdx := len(b.undoStack) - 1
	snap := b.undoStack[lastIdx]
	b.undoStack = b.undoStack[:lastIdx]

	b.inUndoRedo = true
	b.Lines = cloneRuneLines(snap.Lines)
	b.CursorRow = snap.CursorRow
	b.CursorCol = snap.CursorCol
	b.ExtraCursors = cloneExtraCursors(snap.ExtraCursors)
	b.clampCursor()
	b.clampExtraCursors()
	b.ClearSelection()
	b.IsDirty = true
	b.markContentChanged()
	b.inUndoRedo = false
	return true
}

func (b *Buffer) Redo() bool {
	b.endTypingGroup()
	if len(b.redoStack) == 0 {
		return false
	}
	currSnap := BufferSnapshot{
		Lines:        cloneRuneLines(b.Lines),
		CursorRow:    b.CursorRow,
		CursorCol:    b.CursorCol,
		ExtraCursors: cloneExtraCursors(b.ExtraCursors),
	}
	b.undoStack = append(b.undoStack, currSnap)
	if len(b.undoStack) > b.maxUndoStack {
		b.undoStack = b.undoStack[1:]
	}

	lastIdx := len(b.redoStack) - 1
	snap := b.redoStack[lastIdx]
	b.redoStack = b.redoStack[:lastIdx]

	b.inUndoRedo = true
	b.Lines = cloneRuneLines(snap.Lines)
	b.CursorRow = snap.CursorRow
	b.CursorCol = snap.CursorCol
	b.ExtraCursors = cloneExtraCursors(snap.ExtraCursors)
	b.clampCursor()
	b.clampExtraCursors()
	b.ClearSelection()
	b.IsDirty = true
	b.markContentChanged()
	b.inUndoRedo = false
	return true
}

func (b *Buffer) CanUndo() bool {
	return len(b.undoStack) > 0
}

func (b *Buffer) CanRedo() bool {
	return len(b.redoStack) > 0
}

func (b *Buffer) insertRuneInternal(r rune) {
	b.clampCursor()
	line := b.Lines[b.CursorRow]
	newLine := make([]rune, 0, len(line)+1)
	newLine = append(newLine, line[:b.CursorCol]...)
	newLine = append(newLine, r)
	newLine = append(newLine, line[b.CursorCol:]...)
	b.Lines[b.CursorRow] = newLine
	b.CursorCol++
	b.IsDirty = true
	b.markContentChanged()
}

func (b *Buffer) InsertRune(r rune) {
	if b.HasMultipleCursors() {
		b.endTypingGroup()
		b.pushUndo()
		b.applyToAllCursors(func() { b.insertRuneInternal(r) })
		if unicode.IsSpace(r) {
			b.endTypingGroup()
		}
		return
	}
	if b.HasSelection {
		b.endTypingGroup()
		b.pushUndo()
		b.deleteSelectionInternal()
	} else if !b.typingGroup {
		b.pushUndo()
		b.typingGroup = true
	}
	b.insertRuneInternal(r)
	if unicode.IsSpace(r) {
		b.endTypingGroup()
	}
}

func (b *Buffer) insertNewlineInternal() {
	b.clampCursor()
	line := b.Lines[b.CursorRow]
	left := line[:b.CursorCol]
	right := line[b.CursorCol:]

	b.Lines[b.CursorRow] = left

	// Insert new row after CursorRow
	newLines := make([][]rune, 0, len(b.Lines)+1)
	newLines = append(newLines, b.Lines[:b.CursorRow+1]...)
	newLines = append(newLines, right)
	if b.CursorRow+1 < len(b.Lines) {
		newLines = append(newLines, b.Lines[b.CursorRow+1:]...)
	}

	b.Lines = newLines
	b.CursorRow++
	b.CursorCol = 0
	b.IsDirty = true
	b.markContentChanged()
}

func (b *Buffer) InsertString(s string) {
	b.endTypingGroup()
	b.pushUndo()
	if b.HasMultipleCursors() {
		b.applyToAllCursors(func() {
			for _, r := range s {
				if r == '\n' {
					b.insertNewlineInternal()
				} else {
					b.insertRuneInternal(r)
				}
			}
		})
		return
	}
	if b.HasSelection {
		b.deleteSelectionInternal()
	}
	for _, r := range s {
		if r == '\n' {
			b.insertNewlineInternal()
		} else {
			b.insertRuneInternal(r)
		}
	}
}

func (b *Buffer) InsertNewline() {
	b.endTypingGroup()
	b.pushUndo()
	if b.HasMultipleCursors() {
		b.applyToAllCursors(b.insertNewlineInternal)
		return
	}
	if b.HasSelection {
		b.deleteSelectionInternal()
	}
	b.insertNewlineInternal()
}

func (b *Buffer) Backspace() {
	b.endTypingGroup()
	b.pushUndo()
	if b.HasMultipleCursors() {
		b.applyToAllCursors(b.backspaceInternal)
		return
	}
	if b.HasSelection {
		b.deleteSelectionInternal()
		return
	}
	b.backspaceInternal()
}

// DeleteWordLeft deletes word or whitespace to the left of the cursor, or active selection.
func (b *Buffer) DeleteWordLeft() {
	b.endTypingGroup()
	b.pushUndo()
	if b.HasMultipleCursors() {
		b.applyToAllCursors(b.deleteWordLeftInternal)
		return
	}
	if b.HasSelection {
		b.deleteSelectionInternal()
		return
	}
	b.deleteWordLeftInternal()
}

func (b *Buffer) deleteWordLeftInternal() {
	b.clampCursor()
	if b.CursorRow < 0 || b.CursorRow >= len(b.Lines) {
		return
	}
	if b.CursorCol == 0 {
		if b.CursorRow > 0 {
			prevLine := b.Lines[b.CursorRow-1]
			currLine := b.Lines[b.CursorRow]
			prevLen := len(prevLine)
			merged := append(prevLine, currLine...)
			b.Lines[b.CursorRow-1] = merged
			b.Lines = append(b.Lines[:b.CursorRow], b.Lines[b.CursorRow+1:]...)
			b.CursorRow--
			b.CursorCol = prevLen
			b.IsDirty = true
			b.markContentChanged()
		}
		return
	}
	line := b.Lines[b.CursorRow]
	col := b.CursorCol
	if col > len(line) {
		col = len(line)
	}
	targetCol := col
	// 1. Skip whitespace left
	for targetCol > 0 && unicode.IsSpace(line[targetCol-1]) {
		targetCol--
	}
	if targetCol > 0 {
		if isWordChar(line[targetCol-1]) {
			for targetCol > 0 && isWordChar(line[targetCol-1]) {
				targetCol--
			}
		} else {
			for targetCol > 0 && !isWordChar(line[targetCol-1]) && !unicode.IsSpace(line[targetCol-1]) {
				targetCol--
			}
		}
	}
	newLine := make([]rune, 0, len(line)-(col-targetCol))
	newLine = append(newLine, line[:targetCol]...)
	newLine = append(newLine, line[col:]...)
	b.Lines[b.CursorRow] = newLine
	b.CursorCol = targetCol
	b.IsDirty = true
	b.markContentChanged()
}

func (b *Buffer) backspaceInternal() {
	b.clampCursor()

	if b.CursorCol > 0 {
		line := b.Lines[b.CursorRow]
		newLine := make([]rune, 0, len(line)-1)
		newLine = append(newLine, line[:b.CursorCol-1]...)
		newLine = append(newLine, line[b.CursorCol:]...)

		b.Lines[b.CursorRow] = newLine
		b.CursorCol--
		b.IsDirty = true
		b.markContentChanged()
		return
	}

	// At start of line, merge with previous line
	if b.CursorRow > 0 {
		prevLine := b.Lines[b.CursorRow-1]
		currLine := b.Lines[b.CursorRow]
		prevLen := len(prevLine)

		merged := append(prevLine, currLine...)
		b.Lines[b.CursorRow-1] = merged

		// Remove current line
		b.Lines = append(b.Lines[:b.CursorRow], b.Lines[b.CursorRow+1:]...)
		b.CursorRow--
		b.CursorCol = prevLen
		b.IsDirty = true
		b.markContentChanged()
	}
}

func (b *Buffer) Delete() {
	b.endTypingGroup()
	b.pushUndo()
	if b.HasMultipleCursors() {
		b.applyToAllCursors(b.deleteInternal)
		return
	}
	if b.HasSelection {
		b.deleteSelectionInternal()
		return
	}
	b.deleteInternal()
}

func (b *Buffer) deleteInternal() {
	b.clampCursor()

	line := b.Lines[b.CursorRow]
	if b.CursorCol < len(line) {
		newLine := make([]rune, 0, len(line)-1)
		newLine = append(newLine, line[:b.CursorCol]...)
		newLine = append(newLine, line[b.CursorCol+1:]...)

		b.Lines[b.CursorRow] = newLine
		b.IsDirty = true
		b.markContentChanged()
		return
	}

	// At end of line, merge with next line
	if b.CursorRow < len(b.Lines)-1 {
		nextLine := b.Lines[b.CursorRow+1]
		merged := append(line, nextLine...)
		b.Lines[b.CursorRow] = merged
		b.Lines = append(b.Lines[:b.CursorRow+1], b.Lines[b.CursorRow+2:]...)
		b.IsDirty = true
		b.markContentChanged()
	}
}

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

	// 2. Determine word char vs punctuation
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

// IsSelected returns true if the position (row, col) is within the current selection.
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

// StartSelection initiates a selection anchor at (row, col).
func (b *Buffer) StartSelection(row, col int) {
	b.SelectAnchorRow = row
	b.SelectAnchorCol = col
	b.CursorRow = row
	b.CursorCol = col
	b.clampCursor()
	b.HasSelection = false
}

// UpdateSelection extends the selection to (row, col).
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

// SelectAll selects the entire buffer content.
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

// GetSelectedText returns the currently selected text.
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

// DeleteSelection removes selected text and places cursor at start of selection.
func (b *Buffer) DeleteSelection() bool {
	b.endTypingGroup()
	b.pushUndo()
	return b.deleteSelectionInternal()
}

func (b *Buffer) deleteSelectionInternal() bool {
	if !b.HasSelection {
		return false
	}
	sRow, sCol, eRow, eCol := b.NormalizedSelection()
	if sRow == eRow && sCol == eCol {
		b.HasSelection = false
		return false
	}
	if sRow >= len(b.Lines) {
		b.HasSelection = false
		return false
	}

	if sRow == eRow {
		line := b.Lines[sRow]
		if sCol < len(line) {
			if eCol > len(line) {
				eCol = len(line)
			}
			b.Lines[sRow] = append(line[:sCol], line[eCol:]...)
		}
	} else {
		firstPart := b.Lines[sRow][:sCol]
		var lastPart []rune
		if eRow < len(b.Lines) {
			if eCol < len(b.Lines[eRow]) {
				lastPart = b.Lines[eRow][eCol:]
			}
		}
		merged := append(firstPart, lastPart...)
		newLines := make([][]rune, 0, len(b.Lines)-(eRow-sRow))
		newLines = append(newLines, b.Lines[:sRow]...)
		newLines = append(newLines, merged)
		if eRow+1 < len(b.Lines) {
			newLines = append(newLines, b.Lines[eRow+1:]...)
		}
		b.Lines = newLines
	}

	b.CursorRow = sRow
	b.CursorCol = sCol
	b.HasSelection = false
	b.IsDirty = true
	b.markContentChanged()
	b.clampCursor()
	return true
}

// GutterDigits returns the number of digits needed for line numbers (minimum 3).
func (b *Buffer) GutterDigits() int {
	lineCount := len(b.Lines)
	digits := len(strconv.Itoa(lineCount))
	if digits < 3 {
		digits = 3
	}
	return digits
}

// GutterWidth returns the total gutter column width including margins and separator.
func (b *Buffer) GutterWidth() int {
	return b.GutterDigits() + 4
}

// TabWidth defines visual spacing for tab characters in the editor.
const TabWidth = 4

// runeVisualWidth returns the visual cell width of rune r at the given visual column.
func runeVisualWidth(r rune, curVisualCol int) int {
	if r == '\t' {
		rem := curVisualCol % TabWidth
		return TabWidth - rem
	}
	w := ansi.StringWidth(string(r))
	if w <= 0 {
		return 1
	}
	return w
}

// RowColFromVisual converts a visual row offset (from top of viewport) and visual column
// offset into the corresponding physical line and column.
func (b *Buffer) RowColFromVisual(relY, textX, textWidth int) (int, int) {
	if textX < 0 {
		textX = 0
	}
	if !b.SoftWrap {
		visible := make([]int, 0, len(b.Lines))
		for i := range b.Lines {
			if !b.IsHiddenLine(i) {
				visible = append(visible, i)
			}
		}
		vIdx := b.ScrollRow + relY
		if vIdx < 0 {
			vIdx = 0
		}
		if vIdx >= len(visible) {
			if len(visible) == 0 {
				return 0, 0
			}
			lastRow := visible[len(visible)-1]
			return lastRow, len(b.Lines[lastRow])
		}
		physRow := visible[vIdx]
		pLine := b.Lines[physRow]
		curVis := 0
		for c, r := range pLine {
			rw := runeVisualWidth(r, curVis)
			if curVis+rw > b.ScrollCol+textX {
				return physRow, c
			}
			curVis += rw
		}
		return physRow, len(pLine)
	}

	vLines := b.ComputeVisualLines(textWidth)
	vIdx := b.ScrollRow + relY
	if vIdx < 0 {
		vIdx = 0
	}
	if vIdx >= len(vLines) {
		if len(b.Lines) == 0 {
			return 0, 0
		}
		lastRow := len(b.Lines) - 1
		return lastRow, len(b.Lines[lastRow])
	}
	vl := vLines[vIdx]
	physRow := vl.PhysicalLineIdx
	curVis := 0
	for c, r := range vl.Runes {
		rw := runeVisualWidth(r, curVis)
		if curVis+rw > textX {
			return physRow, vl.StartCol + c
		}
		curVis += rw
	}
	return physRow, vl.EndCol
}

// ComputeVisualLines

// ComputeVisualLines generates visual lines, handling soft-wrap or horizontal window slicing.
func (b *Buffer) ComputeVisualLines(textWidth int) []VisualLine {
	if textWidth < 10 {
		textWidth = 10
	}

	var vLines []VisualLine
	for pIdx, pLine := range b.Lines {
		if b.IsHiddenLine(pIdx) {
			continue
		}
		if len(pLine) == 0 {
			vLines = append(vLines, VisualLine{
				PhysicalLineIdx: pIdx,
				IsContinuation:  false,
				Runes:           []rune{},
				StartCol:        0,
				EndCol:          0,
			})
			continue
		}

		if !b.SoftWrap {
			start := b.ScrollCol
			if start > len(pLine) {
				start = len(pLine)
			}
			end := start
			visWidth := 0
			for end < len(pLine) {
				rw := runeVisualWidth(pLine[end], visWidth)
				if visWidth+rw > textWidth && end > start {
					break
				}
				visWidth += rw
				end++
			}
			var runes []rune
			if start < end {
				runes = pLine[start:end]
			} else {
				runes = []rune{}
			}
			vLines = append(vLines, VisualLine{
				PhysicalLineIdx: pIdx,
				IsContinuation:  false,
				Runes:           runes,
				StartCol:        start,
				EndCol:          end,
			})
			continue
		}

		start := 0
		for start < len(pLine) {
			end := start
			visWidth := 0
			for end < len(pLine) {
				rw := runeVisualWidth(pLine[end], visWidth)
				if visWidth+rw > textWidth && end > start {
					break
				}
				visWidth += rw
				end++
			}
			if end == start && end < len(pLine) {
				end = start + 1
			}

			vLines = append(vLines, VisualLine{
				PhysicalLineIdx: pIdx,
				IsContinuation:  start > 0,
				Runes:           pLine[start:end],
				StartCol:        start,
				EndCol:          end,
			})
			start = end
		}
	}
	return vLines
}

func (b *Buffer) Render(width, height int, path string, focused bool) string {
	b.clampCursor()

	// 1. Calculate gutter width
	gutterDigits := b.GutterDigits()
	gutterWidth := b.GutterWidth()
	textWidth := width - gutterWidth
	if textWidth < 10 {
		textWidth = 10
	}

	// 2. Horizontal scroll adjustment when soft-wrap is disabled
	if !b.SoftWrap {
		if b.CursorCol < b.ScrollCol {
			b.ScrollCol = b.CursorCol
		} else if b.CursorCol >= b.ScrollCol+textWidth {
			b.ScrollCol = b.CursorCol - textWidth + 1
		}
		if b.ScrollCol < 0 {
			b.ScrollCol = 0
		}
	} else {
		b.ScrollCol = 0
	}

	// 3. Compute visual lines
	vLines := b.ComputeVisualLines(textWidth)

	// 4. Find visual cursor index
	visualCursorRow := 0
	visualCursorCol := 0
	for vIdx, vl := range vLines {
		if vl.PhysicalLineIdx == b.CursorRow && b.CursorCol >= vl.StartCol && b.CursorCol <= vl.EndCol {
			visualCursorRow = vIdx
			visualCursorCol = b.CursorCol - vl.StartCol
			break
		}
	}

	// 4. Adjust viewport scrolling
	if visualCursorRow < b.ScrollRow {
		b.ScrollRow = visualCursorRow
	} else if visualCursorRow >= b.ScrollRow+height {
		b.ScrollRow = visualCursorRow - height + 1
	}

	if b.ScrollRow < 0 {
		b.ScrollRow = 0
	}

	// 5. Highlighting (Lazy Viewport-Bounded Line Parsing)
	if b.cachedDirty || b.cachedPath != path {
		if b.Highlighter != nil {
			b.cachedRawLines = b.Highlighter.HighlightCodeLines(b.Text(), path)
			b.cachedStyles = make([][]string, len(b.cachedRawLines))
		} else {
			b.cachedRawLines = nil
			b.cachedStyles = nil
		}
		b.cachedPath = path
		b.cachedDirty = false
	}

	gutterStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary).
		Background(ui.CurrentTheme.BgPrimary)

	gutterActiveStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.Accent).
		Bold(true).
		Background(ui.CurrentTheme.BgPrimary)

	selectionStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.Accent).
		Foreground(ui.CurrentTheme.FgHighlight)

	cursorIBeamStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Bold(true)

	var renderedRows []string
	endV := b.ScrollRow + height
	if endV > len(vLines) {
		endV = len(vLines)
	}

	for vIdx := b.ScrollRow; vIdx < endV; vIdx++ {
		vl := vLines[vIdx]

		// Format gutter
		var gutterText string
		if !vl.IsContinuation {
			lineNum := vl.PhysicalLineIdx + 1
			marker := " "
			if diag, hasDiag := b.LineDiagnostics[vl.PhysicalLineIdx]; hasDiag && diag != "" {
				marker = diag
			} else if end, ok := b.FoldableAt(vl.PhysicalLineIdx); ok {
				if b.IsFoldedAt(vl.PhysicalLineIdx) {
					marker = "▸"
				} else {
					marker = "▾"
					_ = end
				}
			}
			if vl.PhysicalLineIdx == b.CursorRow && focused {
				gutterText = gutterActiveStyle.Render(fmt.Sprintf("%s%*d │ ", marker, gutterDigits, lineNum))
			} else {
				gutterText = gutterStyle.Render(fmt.Sprintf("%s%*d │ ", marker, gutterDigits, lineNum))
			}
		} else {
			gutterText = gutterStyle.Render(fmt.Sprintf(" %*s ┊ ", gutterDigits, " "))
		}

		// Text part
		lineRunes := vl.Runes
		var textPart strings.Builder
		physRow := vl.PhysicalLineIdx
		activeStyle := ""

		curVisualCol := 0
		secondaryCursorStyle := lipgloss.NewStyle().
			Foreground(ui.CurrentTheme.FgHighlight).
			Background(ui.CurrentTheme.Accent)
		extraCursorHere := func(physCol int) bool {
			if physRow < 0 || physRow >= len(b.Lines) {
				return false
			}
			_ = physRow
			for _, e := range b.ExtraCursors {
				if e.Row == physRow && e.Col == physCol {
					return true
				}
			}
			return false
		}
		for cIdx, r := range lineRunes {
			physCol := vl.StartCol + cIdx
			isCursor := vIdx == visualCursorRow && cIdx == visualCursorCol && focused
			isExtraCursor := !isCursor && focused && !b.HasSelection && extraCursorHere(physCol)
			isSelected := b.IsSelected(physRow, physCol)
			isMatch, isActiveMatch := b.MatchAt(physRow, physCol)

			rw := runeVisualWidth(r, curVisualCol)
			var rStr string
			if r == '\t' {
				rStr = strings.Repeat(" ", rw)
			} else {
				rStr = string(r)
			}

			if isCursor && !b.HasSelection {
				if activeStyle != "" {
					textPart.WriteString("\x1b[0m")
					activeStyle = ""
				}
				if r == '\t' {
					textPart.WriteString(cursorIBeamStyle.Render(" "))
					if rw > 1 {
						textPart.WriteString(strings.Repeat(" ", rw-1))
					}
				} else {
					textPart.WriteString(cursorIBeamStyle.Render(rStr))
				}
			} else if isExtraCursor {
				if activeStyle != "" {
					textPart.WriteString("\x1b[0m")
					activeStyle = ""
				}
				if r == '\t' {
					textPart.WriteString(secondaryCursorStyle.Render(" "))
					if rw > 1 {
						textPart.WriteString(strings.Repeat(" ", rw-1))
					}
				} else {
					textPart.WriteString(secondaryCursorStyle.Render(rStr))
				}
			} else if isSelected {
				if activeStyle != "" {
					textPart.WriteString("\x1b[0m")
					activeStyle = ""
				}
				textPart.WriteString(selectionStyle.Render(rStr))
			} else if isActiveMatch {
				if activeStyle != "" {
					textPart.WriteString("\x1b[0m")
					activeStyle = ""
				}
				textPart.WriteString(ui.CurrentTheme.SearchActiveMatch.Render(rStr))
			} else if isMatch {
				if activeStyle != "" {
					textPart.WriteString("\x1b[0m")
					activeStyle = ""
				}
				textPart.WriteString(ui.CurrentTheme.SearchMatchStyle.Render(rStr))
			} else {
				// Indent Rainbow for leading indentation spaces
				leadingSpaces := 0
				if b.IndentRainbow && !vl.IsContinuation && physRow >= 0 && physRow < len(b.Lines) {
					leadingSpaces, _ = CountLeadingSpaces(b.Lines[physRow])
				}

				if b.IndentRainbow && physCol < leadingSpaces && (r == ' ' || r == '\t') {
					if activeStyle != "" {
						textPart.WriteString("\x1b[0m")
						activeStyle = ""
					}
					renderedIndent, isIndent := RenderIndentRune(physCol, leadingSpaces, r)
					if isIndent {
						textPart.WriteString(renderedIndent)
						curVisualCol += rw
						continue
					}
				}

				syntaxStyle := ""
				if physRow < len(b.cachedStyles) {
					if b.cachedStyles[physRow] == nil && physRow < len(b.cachedRawLines) {
						b.cachedStyles[physRow] = syntax.ParseLineStyles(b.cachedRawLines[physRow])
					}
					if physCol < len(b.cachedStyles[physRow]) {
						syntaxStyle = b.cachedStyles[physRow][physCol]
					}
				}

				if syntaxStyle != activeStyle {
					if activeStyle != "" {
						textPart.WriteString("\x1b[0m")
					}
					if syntaxStyle != "" {
						textPart.WriteString(syntaxStyle)
					}
					activeStyle = syntaxStyle
				}
				textPart.WriteString(rStr)
			}

			curVisualCol += rw
		}

		if activeStyle != "" {
			textPart.WriteString("\x1b[0m")
			activeStyle = ""
		}

		if vIdx == visualCursorRow && focused && !b.HasSelection && visualCursorCol >= len(lineRunes) {
			textPart.WriteString(cursorIBeamStyle.Render("│"))
		}
		if focused && !b.HasSelection {
			for _, e := range b.ExtraCursors {
				if e.Row == physRow && e.Col >= vl.StartCol+len(lineRunes) && e.Col <= vl.EndCol {
					textPart.WriteString(secondaryCursorStyle.Render("│"))
					break
				}
			}
		}

		plainTextLen := ansi.StringWidth(textPart.String())
		padding := textWidth - plainTextLen
		if padding < 0 {
			padding = 0
		}
		renderedRow := gutterText + textPart.String() + strings.Repeat(" ", padding)
		if ansi.StringWidth(renderedRow) > width {
			renderedRow = ansi.Truncate(renderedRow, width, "")
		} else if ansi.StringWidth(renderedRow) < width {
			renderedRow += strings.Repeat(" ", width-ansi.StringWidth(renderedRow))
		}
		renderedRows = append(renderedRows, renderedRow)
	}

	// Fill remaining height
	for len(renderedRows) < height {
		emptyGutter := gutterStyle.Render(fmt.Sprintf(" %*s │ ", gutterDigits, "~"))
		renderedRows = append(renderedRows, emptyGutter+strings.Repeat(" ", textWidth))
	}

	return strings.Join(renderedRows, "\n")
}

// ClampCursor ensures CursorRow and CursorCol are within valid buffer bounds.
func (b *Buffer) ClampCursor() {
	b.clampCursor()
}

// IndentCurrentLine adds 4 spaces to the beginning of the current line.
func (b *Buffer) IndentCurrentLine() {
	b.endTypingGroup()
	b.pushUndo()
	if b.CursorRow < 0 || b.CursorRow >= len(b.Lines) {
		return
	}
	line := b.Lines[b.CursorRow]
	newLine := make([]rune, 0, len(line)+4)
	newLine = append(newLine, []rune("    ")...)
	newLine = append(newLine, line...)
	b.Lines[b.CursorRow] = newLine
	b.CursorCol += 4
	b.IsDirty = true
	b.markContentChanged()
	b.clampCursor()
}

// IndentSelection adds 4 spaces to each line touched by current selection, or indents current line if no selection.
func (b *Buffer) IndentSelection() {
	if !b.HasSelection {
		b.IndentCurrentLine()
		return
	}
	b.endTypingGroup()
	b.pushUndo()
	sRow, _, eRow, eCol := b.NormalizedSelection()
	if eRow > sRow && eCol == 0 {
		eRow--
	}
	for r := sRow; r <= eRow && r < len(b.Lines); r++ {
		line := b.Lines[r]
		newLine := make([]rune, 0, len(line)+4)
		newLine = append(newLine, []rune("    ")...)
		newLine = append(newLine, line...)
		b.Lines[r] = newLine
	}
	b.SelectAnchorCol += 4
	b.CursorCol += 4
	b.IsDirty = true
	b.markContentChanged()
	b.clampCursor()
}

// OutdentSelection removes up to 4 leading spaces from each line touched by current selection.
func (b *Buffer) OutdentSelection() {
	if !b.HasSelection {
		b.OutdentCurrentLine()
		return
	}
	b.endTypingGroup()
	b.pushUndo()
	sRow, _, eRow, eCol := b.NormalizedSelection()
	if eRow > sRow && eCol == 0 {
		eRow--
	}
	for r := sRow; r <= eRow && r < len(b.Lines); r++ {
		line := b.Lines[r]
		spaces := 0
		for spaces < 4 && spaces < len(line) && line[spaces] == ' ' {
			spaces++
		}
		if spaces > 0 {
			b.Lines[r] = line[spaces:]
		}
	}
	b.SelectAnchorCol -= 4
	if b.SelectAnchorCol < 0 {
		b.SelectAnchorCol = 0
	}
	b.CursorCol -= 4
	if b.CursorCol < 0 {
		b.CursorCol = 0
	}
	b.IsDirty = true
	b.markContentChanged()
	b.clampCursor()
}

// OutdentCurrentLine removes up to 4 leading spaces from current line.
func (b *Buffer) OutdentCurrentLine() {
	b.endTypingGroup()
	b.pushUndo()
	b.clampCursor()
	line := b.Lines[b.CursorRow]
	spaces := 0
	for spaces < 4 && spaces < len(line) && line[spaces] == ' ' {
		spaces++
	}
	if spaces > 0 {
		b.Lines[b.CursorRow] = line[spaces:]
		b.CursorCol -= spaces
		if b.CursorCol < 0 {
			b.CursorCol = 0
		}
		b.IsDirty = true
		b.markContentChanged()
		b.clampCursor()
	}
}

// ToggleLineComment toggles line comment on current line or selection.
func (b *Buffer) ToggleLineComment(prefix string) {
	if prefix == "" {
		prefix = "// "
	}
	b.endTypingGroup()
	b.pushUndo()

	sRow := b.CursorRow
	eRow := b.CursorRow
	if b.HasSelection {
		var eCol int
		sRow, _, eRow, eCol = b.NormalizedSelection()
		if eRow > sRow && eCol == 0 {
			eRow--
		}
	}

	trimmedPref := strings.TrimRight(prefix, " ")

	// Check if all non-empty lines are already commented
	allCommented := true
	hasNonEmpty := false
	for r := sRow; r <= eRow && r < len(b.Lines); r++ {
		line := b.Lines[r]
		trimmed := strings.TrimLeft(string(line), " \t")
		if trimmed == "" {
			continue
		}
		hasNonEmpty = true
		if !strings.HasPrefix(trimmed, trimmedPref) {
			allCommented = false
			break
		}
	}
	if !hasNonEmpty {
		allCommented = false
	}

	for r := sRow; r <= eRow && r < len(b.Lines); r++ {
		line := b.Lines[r]
		str := string(line)
		leadSpaces := len(str) - len(strings.TrimLeft(str, " \t"))
		indent := str[:leadSpaces]
		rest := str[leadSpaces:]

		if allCommented {
			// Uncomment: remove prefix
			targetPref := prefix
			if !strings.HasPrefix(rest, targetPref) {
				targetPref = trimmedPref
			}
			if strings.HasPrefix(rest, targetPref) {
				uncommented := rest[len(targetPref):]
				b.Lines[r] = []rune(indent + uncommented)
			}
		} else {
			// Comment: add prefix after indent
			b.Lines[r] = []rune(indent + prefix + rest)
		}
	}

	b.IsDirty = true
	b.markContentChanged()
	b.clampCursor()
}
