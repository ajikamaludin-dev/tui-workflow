package git

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type LineDiffType int

const (
	DiffUnchanged LineDiffType = iota
	DiffAdded
	DiffDeleted
	DiffEmpty
)

type DiffRow struct {
	LeftNum   int
	LeftText  string
	LeftType  LineDiffType
	RightNum  int
	RightText string
	RightType LineDiffType
}

type SplitDiffResult struct {
	FilePath string
	Rows     []DiffRow
}

// ComputeSplitDiff calculates a side-by-side split diff using LCS
func ComputeSplitDiff(filePath, original, modified string) SplitDiffResult {
	origLines := splitLines(original)
	modLines := splitLines(modified)

	// If one is empty
	if len(origLines) == 0 && len(modLines) == 0 {
		return SplitDiffResult{FilePath: filePath}
	}

	// Compute LCS matrix
	m := len(origLines)
	n := len(modLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if origLines[i] == modLines[j] {
				dp[i+1][j+1] = dp[i][j] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i+1][j+1] = dp[i+1][j]
			} else {
				dp[i+1][j+1] = dp[i][j+1]
			}
		}
	}

	// Backtrack to build diff ops
	type op struct {
		typ     LineDiffType
		origIdx int
		modIdx  int
	}
	var ops []op
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && origLines[i-1] == modLines[j-1] {
			ops = append(ops, op{typ: DiffUnchanged, origIdx: i - 1, modIdx: j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, op{typ: DiffAdded, modIdx: j - 1})
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			ops = append(ops, op{typ: DiffDeleted, origIdx: i - 1})
			i--
		}
	}

	// Reverse ops
	for k := 0; k < len(ops)/2; k++ {
		ops[k], ops[len(ops)-1-k] = ops[len(ops)-1-k], ops[k]
	}

	// Align into side-by-side rows
	var rows []DiffRow
	idx := 0
	for idx < len(ops) {
		current := ops[idx]
		if current.typ == DiffUnchanged {
			rows = append(rows, DiffRow{
				LeftNum:   current.origIdx + 1,
				LeftText:  origLines[current.origIdx],
				LeftType:  DiffUnchanged,
				RightNum:  current.modIdx + 1,
				RightText: modLines[current.modIdx],
				RightType: DiffUnchanged,
			})
			idx++
		} else {
			// Collect contiguous deleted and added blocks
			var deleted []op
			var added []op
			for idx < len(ops) && (ops[idx].typ == DiffDeleted || ops[idx].typ == DiffAdded) {
				if ops[idx].typ == DiffDeleted {
					deleted = append(deleted, ops[idx])
				} else {
					added = append(added, ops[idx])
				}
				idx++
			}

			maxLen := len(deleted)
			if len(added) > maxLen {
				maxLen = len(added)
			}

			for k := 0; k < maxLen; k++ {
				var row DiffRow
				if k < len(deleted) {
					row.LeftNum = deleted[k].origIdx + 1
					row.LeftText = origLines[deleted[k].origIdx]
					row.LeftType = DiffDeleted
				} else {
					row.LeftType = DiffEmpty
				}

				if k < len(added) {
					row.RightNum = added[k].modIdx + 1
					row.RightText = modLines[added[k].modIdx]
					row.RightType = DiffAdded
				} else {
					row.RightType = DiffEmpty
				}

				rows = append(rows, row)
			}
		}
	}

	return SplitDiffResult{
		FilePath: filePath,
		Rows:     rows,
	}
}

func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	// Trim trailing single empty line if text ended with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type DiffView struct {
	Result       SplitDiffResult
	ScrollOffset int
}

func NewDiffView(filePath, original, modified string) *DiffView {
	res := ComputeSplitDiff(filePath, original, modified)
	return &DiffView{
		Result: res,
	}
}

func (dv *DiffView) ScrollUp(n int) {
	dv.ScrollOffset -= n
	if dv.ScrollOffset < 0 {
		dv.ScrollOffset = 0
	}
}

func (dv *DiffView) ScrollDown(n int, viewHeight int) {
	maxOffset := len(dv.Result.Rows) - viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	dv.ScrollOffset += n
	if dv.ScrollOffset > maxOffset {
		dv.ScrollOffset = maxOffset
	}
}

func (dv *DiffView) Render(width, height int, isFocused bool) string {
	if width < 20 || height < 2 {
		return ""
	}

	// Calculate half widths for side-by-side
	gutterWidth := 5 // e.g. " 123 "
	colWidth := (width - 1) / 2
	textWidth := colWidth - gutterWidth
	if textWidth < 4 {
		textWidth = 4
	}

	// Styles
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Background(lipgloss.Color("#3E1A1A"))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Background(lipgloss.Color("#132A1C"))
	emptyStyle := lipgloss.NewStyle().Background(lipgloss.Color("#1B1B1B"))
	gutterDelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Background(lipgloss.Color("#2D1212"))
	gutterAddStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Background(lipgloss.Color("#0E1F14"))
	gutterNormal := ui.CurrentTheme.LineNumberStyle

	var outLines []string
	headerLeft := lipgloss.NewStyle().Bold(true).Width(colWidth).Background(lipgloss.Color("#21262D")).Render(" HEAD / Original (-)")
	headerRight := lipgloss.NewStyle().Bold(true).Width(colWidth).Background(lipgloss.Color("#21262D")).Render(" Working Tree / Modified (+)")
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerLeft, "│", headerRight)
	outLines = append(outLines, headerRow)

	contentHeight := height - 1
	for v := 0; v < contentHeight; v++ {
		rowIdx := dv.ScrollOffset + v
		if rowIdx >= len(dv.Result.Rows) {
			emptyLeft := strings.Repeat(" ", colWidth)
			emptyRight := strings.Repeat(" ", colWidth)
			outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, emptyLeft, "│", emptyRight))
			continue
		}

		row := dv.Result.Rows[rowIdx]

		// Left Column
		var leftGutter, leftContent string
		switch row.LeftType {
		case DiffDeleted:
			leftGutter = gutterDelStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.LeftNum))
			txt := truncateOrPad(row.LeftText, textWidth)
			leftContent = delStyle.Width(textWidth).Render(txt)
		case DiffEmpty:
			leftGutter = emptyStyle.Width(gutterWidth).Render(strings.Repeat(" ", gutterWidth))
			leftContent = emptyStyle.Width(textWidth).Render(strings.Repeat(" ", textWidth))
		default:
			leftGutter = gutterNormal.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.LeftNum))
			txt := truncateOrPad(row.LeftText, textWidth)
			leftContent = lipgloss.NewStyle().Width(textWidth).Render(txt)
		}
		leftCol := leftGutter + leftContent

		// Right Column
		var rightGutter, rightContent string
		switch row.RightType {
		case DiffAdded:
			rightGutter = gutterAddStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.RightNum))
			txt := truncateOrPad(row.RightText, textWidth)
			rightContent = addStyle.Width(textWidth).Render(txt)
		case DiffEmpty:
			rightGutter = emptyStyle.Width(gutterWidth).Render(strings.Repeat(" ", gutterWidth))
			rightContent = emptyStyle.Width(textWidth).Render(strings.Repeat(" ", textWidth))
		default:
			rightGutter = gutterNormal.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.RightNum))
			txt := truncateOrPad(row.RightText, textWidth)
			rightContent = lipgloss.NewStyle().Width(textWidth).Render(txt)
		}
		rightCol := rightGutter + rightContent

		outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "│", rightCol))
	}

	return strings.Join(outLines, "\n")
}

func truncateOrPad(s string, width int) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	w := lipgloss.Width(s)
	if w > width {
		// Truncate
		runes := []rune(s)
		curW := 0
		for i, r := range runes {
			rw := lipgloss.Width(string(r))
			if curW+rw > width {
				return string(runes[:i])
			}
			curW += rw
		}
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
