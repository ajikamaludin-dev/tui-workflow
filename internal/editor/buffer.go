package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
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

type Buffer struct {
	Lines       [][]rune
	CursorRow   int
	CursorCol   int
	ScrollRow   int
	ScrollCol   int
	SoftWrap    bool
	IsDirty     bool
	Highlighter *syntax.Highlighter
	cachedHigh  []string
	cachedDirty bool

	// Selection state
	HasSelection    bool
	SelectAnchorRow int
	SelectAnchorCol int

	// Search state
	SearchQuery    string
	SearchMatches  []MatchPos
	ActiveMatchIdx int
}

func NewBuffer(content string) *Buffer {
	b := &Buffer{
		Lines:       make([][]rune, 0),
		CursorRow:   0,
		CursorCol:   0,
		ScrollRow:   0,
		ScrollCol:   0,
		SoftWrap:    true,
		IsDirty:     false,
		Highlighter: syntax.NewHighlighter(),
		cachedDirty: true,
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

func (b *Buffer) Text() string {
	var sb strings.Builder
	for i, l := range b.Lines {
		sb.WriteString(string(l))
		if i < len(b.Lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
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
	b.cachedDirty = true
	b.cachedHigh = nil
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

func (b *Buffer) InsertRune(r rune) {
	if b.HasSelection {
		b.DeleteSelection()
	}
	b.clampCursor()
	line := b.Lines[b.CursorRow]

	// Insert into rune slice
	newLine := make([]rune, 0, len(line)+1)
	newLine = append(newLine, line[:b.CursorCol]...)
	newLine = append(newLine, r)
	newLine = append(newLine, line[b.CursorCol:]...)

	b.Lines[b.CursorRow] = newLine
	b.CursorCol++
	b.IsDirty = true
	b.cachedDirty = true
}

func (b *Buffer) InsertString(s string) {
	for _, r := range s {
		if r == '\n' {
			b.InsertNewline()
		} else {
			b.InsertRune(r)
		}
	}
}

func (b *Buffer) InsertNewline() {
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
	b.cachedDirty = true
}

func (b *Buffer) Backspace() {
	if b.HasSelection {
		b.DeleteSelection()
		return
	}
	b.clampCursor()

	if b.CursorCol > 0 {
		line := b.Lines[b.CursorRow]
		newLine := make([]rune, 0, len(line)-1)
		newLine = append(newLine, line[:b.CursorCol-1]...)
		newLine = append(newLine, line[b.CursorCol:]...)

		b.Lines[b.CursorRow] = newLine
		b.CursorCol--
		b.IsDirty = true
		b.cachedDirty = true
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
		b.cachedDirty = true
	}
}

func (b *Buffer) Delete() {
	if b.HasSelection {
		b.DeleteSelection()
		return
	}
	b.clampCursor()
	line := b.Lines[b.CursorRow]

	if b.CursorCol < len(line) {
		newLine := make([]rune, 0, len(line)-1)
		newLine = append(newLine, line[:b.CursorCol]...)
		newLine = append(newLine, line[b.CursorCol+1:]...)

		b.Lines[b.CursorRow] = newLine
		b.IsDirty = true
		b.cachedDirty = true
		return
	}

	// At end of line, merge next line
	if b.CursorRow < len(b.Lines)-1 {
		nextLine := b.Lines[b.CursorRow+1]
		merged := append(line, nextLine...)
		b.Lines[b.CursorRow] = merged

		b.Lines = append(b.Lines[:b.CursorRow+1], b.Lines[b.CursorRow+2:]...)
		b.IsDirty = true
		b.cachedDirty = true
	}
}

func (b *Buffer) MoveLeft() {
	if b.CursorCol > 0 {
		b.CursorCol--
	} else if b.CursorRow > 0 {
		b.CursorRow--
		b.CursorCol = len(b.Lines[b.CursorRow])
	}
}

func (b *Buffer) MoveRight() {
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
	if b.CursorRow > 0 {
		b.CursorRow--
		b.clampCol()
	}
}

func (b *Buffer) MoveDown() {
	if b.CursorRow < len(b.Lines)-1 {
		b.CursorRow++
		b.clampCol()
	}
}

func (b *Buffer) MoveHome() {
	b.CursorCol = 0
}

func (b *Buffer) MoveEnd() {
	if b.CursorRow >= 0 && b.CursorRow < len(b.Lines) {
		b.CursorCol = len(b.Lines[b.CursorRow])
	}
}

func (b *Buffer) MovePageUp(n int) {
	b.CursorRow -= n
	if b.CursorRow < 0 {
		b.CursorRow = 0
	}
	b.clampCol()
}

func (b *Buffer) MovePageDown(n int) {
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
	b.cachedDirty = true
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

// RowColFromVisual converts a visual row offset (from top of viewport) and visual column
// offset into the corresponding physical line and column.
func (b *Buffer) RowColFromVisual(relY, textX, textWidth int) (int, int) {
	if textX < 0 {
		textX = 0
	}
	if !b.SoftWrap {
		vIdx := b.ScrollRow + relY
		if vIdx < 0 {
			vIdx = 0
		}
		if vIdx >= len(b.Lines) {
			if len(b.Lines) == 0 {
				return 0, 0
			}
			lastRow := len(b.Lines) - 1
			return lastRow, len(b.Lines[lastRow])
		}
		physRow := vIdx
		physCol := b.ScrollCol + textX
		if physCol > len(b.Lines[physRow]) {
			physCol = len(b.Lines[physRow])
		}
		if physCol < 0 {
			physCol = 0
		}
		return physRow, physCol
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
	physCol := vl.StartCol + textX
	if physRow < len(b.Lines) {
		if physCol > len(b.Lines[physRow]) {
			physCol = len(b.Lines[physRow])
		}
	}
	if physCol > vl.EndCol {
		physCol = vl.EndCol
	}
	if physCol < vl.StartCol {
		physCol = vl.StartCol
	}
	return physRow, physCol
}

// ComputeVisualLines generates visual lines, handling soft-wrap or horizontal window slicing.
func (b *Buffer) ComputeVisualLines(textWidth int) []VisualLine {
	if textWidth < 10 {
		textWidth = 10
	}
	var vLines []VisualLine
	for pIdx, pLine := range b.Lines {
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
			end := b.ScrollCol + textWidth
			if end > len(pLine) {
				end = len(pLine)
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
			end := start + textWidth
			if end > len(pLine) {
				end = len(pLine)
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
	if !b.SoftWrap {
		visualCursorRow = b.CursorRow
		visualCursorCol = b.CursorCol - b.ScrollCol
	} else {
		for vIdx, vl := range vLines {
			if vl.PhysicalLineIdx == b.CursorRow && b.CursorCol >= vl.StartCol && b.CursorCol <= vl.EndCol {
				visualCursorRow = vIdx
				visualCursorCol = b.CursorCol - vl.StartCol
				break
			}
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

	// 5. Highlighting
	if b.cachedDirty {
		b.cachedHigh = b.Highlighter.HighlightCode(b.Text(), path)
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
			if vl.PhysicalLineIdx == b.CursorRow && focused {
				gutterText = gutterActiveStyle.Render(fmt.Sprintf(" %*d │ ", gutterDigits, lineNum))
			} else {
				gutterText = gutterStyle.Render(fmt.Sprintf(" %*d │ ", gutterDigits, lineNum))
			}
		} else {
			gutterText = gutterStyle.Render(fmt.Sprintf(" %*s ┊ ", gutterDigits, " "))
		}

		// Text part
		lineRunes := vl.Runes
		var textPart strings.Builder

		physRow := vl.PhysicalLineIdx
		for cIdx, r := range lineRunes {
			physCol := vl.StartCol + cIdx
			isCursor := vIdx == visualCursorRow && cIdx == visualCursorCol && focused
			isSelected := b.IsSelected(physRow, physCol)
			isMatch, isActiveMatch := b.MatchAt(physRow, physCol)

			if isCursor && !b.HasSelection {
				textPart.WriteString(cursorIBeamStyle.Render(string(r)))
			} else if isSelected {
				textPart.WriteString(selectionStyle.Render(string(r)))
			} else if isActiveMatch {
				textPart.WriteString(ui.CurrentTheme.SearchActiveMatch.Render(string(r)))
			} else if isMatch {
				textPart.WriteString(ui.CurrentTheme.SearchMatchStyle.Render(string(r)))
			} else {
				textPart.WriteRune(r)
			}
		}

		if vIdx == visualCursorRow && focused && !b.HasSelection && visualCursorCol >= len(lineRunes) {
			textPart.WriteString(cursorIBeamStyle.Render("│"))
		}

		plainTextLen := lipgloss.Width(textPart.String())
		padding := textWidth - plainTextLen
		if padding < 0 {
			padding = 0
		}

		renderedRow := gutterText + textPart.String() + strings.Repeat(" ", padding)
		renderedRows = append(renderedRows, renderedRow)
	}

	// Fill remaining height
	for len(renderedRows) < height {
		emptyGutter := gutterStyle.Render(fmt.Sprintf(" %*s │ ", gutterDigits, "~"))
		renderedRows = append(renderedRows, emptyGutter+strings.Repeat(" ", textWidth))
	}

	return strings.Join(renderedRows, "\n")
}
