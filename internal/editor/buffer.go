package editor

import (
	"strings"
	"unicode"

	"tui-workflow/internal/search"
	"tui-workflow/internal/syntax"
)

type BufferSnapshot struct {
	Lines        [][]rune
	CursorRow    int
	CursorCol    int
	ExtraCursors []extraCursor
}

// extraCursor adalah cursor tambahan (multi-cursor) dengan selection sendiri.
// Cursor utama (primary) tetap di Buffer.CursorRow/CursorCol + HasSelection.
type extraCursor struct {
	Row, Col     int
	SelAnchorRow int
	SelAnchorCol int
	HasSelection bool
}

// FoldRegion menyembunyikan baris (Start, End]; baris Start tetap terlihat
// dan diberi marker ▸. Fold berbasis indentasi, per-tab, tidak dipersistenkan.
type FoldRegion struct {
	Start int
	End   int
}

type Buffer struct {
	Lines           [][]rune
	CursorRow       int
	CursorCol       int
	ScrollRow       int
	ScrollCol       int
	SoftWrap        bool
	IsDirty         bool
	Highlighter     *syntax.Highlighter
	cachedHigh      []string
	cachedStyles    [][]string
	cachedRawLines  []string
	cachedPath      string
	cachedDirty     bool
	cachedText      string
	cachedTextDirty bool

	// Selection state
	HasSelection    bool
	SelectAnchorRow int
	SelectAnchorCol int

	// Search state
	SearchQuery           string
	SearchMatches         []MatchPos
	ActiveMatchIdx        int
	CenterCursorRequested bool
	matchByLine           map[int][]int // row → indices into SearchMatches
	matcher               *search.Matcher
	lastQuery             search.Query

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
