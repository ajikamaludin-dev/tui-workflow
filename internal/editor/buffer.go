package editor

import (
	"fmt"
	"strings"

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
	Lines        [][]rune
	CursorRow    int
	CursorCol    int
	ScrollRow    int
	IsDirty      bool
	Highlighter  *syntax.Highlighter
	cachedHigh   []string
	cachedDirty  bool
}

func NewBuffer(content string) *Buffer {
	b := &Buffer{
		Lines:       make([][]rune, 0),
		CursorRow:   0,
		CursorCol:   0,
		ScrollRow:   0,
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

// ComputeVisualLines generates soft-wrapped visual lines for the given text width
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
	lineCount := len(b.Lines)
	gutterDigits := len(fmt.Sprintf("%d", lineCount))
	if gutterDigits < 3 {
		gutterDigits = 3
	}
	// Gutter format: " %3d │ " -> length is 1 + digits + 3 = digits + 4
	gutterWidth := gutterDigits + 4
	textWidth := width - gutterWidth
	if textWidth < 10 {
		textWidth = 10
	}

	// 2. Compute visual lines with soft-wrap
	vLines := b.ComputeVisualLines(textWidth)

	// 3. Find visual cursor index
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

	cursorStyle := lipgloss.NewStyle().
		Reverse(true).
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

		if vIdx == visualCursorRow && focused {
			// Render with cursor
			for cIdx, r := range lineRunes {
				if cIdx == visualCursorCol {
					textPart.WriteString(cursorStyle.Render(string(r)))
				} else {
					textPart.WriteRune(r)
				}
			}
			if visualCursorCol >= len(lineRunes) {
				textPart.WriteString(cursorStyle.Render(" "))
			}
		} else {
			textPart.WriteString(string(lineRunes))
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
