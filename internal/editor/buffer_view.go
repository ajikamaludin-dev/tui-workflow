package editor

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
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
	if b.CenterCursorRequested {
		b.CenterCursorRequested = false
		b.ScrollRow = visualCursorRow - (height / 2)
	} else if visualCursorRow < b.ScrollRow {
		b.ScrollRow = visualCursorRow
	} else if visualCursorRow >= b.ScrollRow+height {
		b.ScrollRow = visualCursorRow - height + 1
	}

	if b.ScrollRow < 0 {
		b.ScrollRow = 0
	}
	if b.ScrollRow >= len(vLines) && len(vLines) > 0 {
		b.ScrollRow = len(vLines) - 1
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

		leadingSpaces := 0
		if b.IndentRainbow && !vl.IsContinuation && physRow >= 0 && physRow < len(b.Lines) {
			leadingSpaces, _ = CountLeadingSpaces(b.Lines[physRow])
		}

		lineVisualCol := 0
		if vl.StartCol > 0 && physRow >= 0 && physRow < len(b.Lines) {
			pLine := b.Lines[physRow]
			for i := 0; i < vl.StartCol && i < len(pLine); i++ {
				lineVisualCol += runeVisualWidth(pLine[i], lineVisualCol)
			}
		}

		for cIdx, r := range lineRunes {
			physCol := vl.StartCol + cIdx
			isCursor := vIdx == visualCursorRow && cIdx == visualCursorCol && focused
			isExtraCursor := !isCursor && focused && !b.HasSelection && extraCursorHere(physCol)
			isSelected := b.IsSelected(physRow, physCol)
			isMatch, isActiveMatch := b.MatchAt(physRow, physCol)

			rw := runeVisualWidth(r, lineVisualCol)
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
				if b.IndentRainbow && lineVisualCol < leadingSpaces && (r == ' ' || r == '\t') {
					if activeStyle != "" {
						textPart.WriteString("\x1b[0m")
						activeStyle = ""
					}
					renderedIndent, isIndent := RenderIndentRune(lineVisualCol, leadingSpaces, r)
					if isIndent {
						textPart.WriteString(renderedIndent)
						curVisualCol += rw
						lineVisualCol += rw
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
			lineVisualCol += rw
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
