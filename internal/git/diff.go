package git

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// DiffHunk represents a unified-diff hunk derived from split diff rows.
type DiffHunk struct {
	Index     int
	StartRow  int
	EndRow    int // exclusive in Rows
	OrigStart int
	OrigCount int
	ModStart  int
	ModCount  int
	Header    string
	Lines     []string
}

// SplitDiffHasChanges reports whether diff result contains any added/deleted rows.
func SplitDiffHasChanges(diffResult SplitDiffResult) bool {
	for _, row := range diffResult.Rows {
		if row.LeftType == DiffDeleted || row.RightType == DiffAdded {
			return true
		}
	}
	return false
}

// SplitDiffHunks groups changed rows into hunks with context and returns unified hunk metadata.
func SplitDiffHunks(diffResult SplitDiffResult, contextLines int) []DiffHunk {
	n := len(diffResult.Rows)
	if n == 0 || contextLines < 0 {
		return nil
	}
	include := make([]bool, n)
	hasChange := false
	for i, r := range diffResult.Rows {
		if r.LeftType == DiffDeleted || r.RightType == DiffAdded {
			hasChange = true
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= n {
				end = n - 1
			}
			for k := start; k <= end; k++ {
				include[k] = true
			}
		}
	}
	if !hasChange {
		return nil
	}

	var hunks []DiffHunk
	idx := 0
	for idx < n {
		if !include[idx] {
			idx++
			continue
		}
		chunkStart := idx
		for idx < n && include[idx] {
			idx++
		}
		chunkEnd := idx

		origStart := 1
		foundOrig := false
		for k := chunkStart; k < chunkEnd; k++ {
			if diffResult.Rows[k].LeftNum > 0 {
				origStart = diffResult.Rows[k].LeftNum
				foundOrig = true
				break
			}
		}
		if !foundOrig && chunkStart > 0 {
			// insertion at end/start fallback: approximate from previous row
			prev := diffResult.Rows[chunkStart-1]
			if prev.LeftNum > 0 {
				origStart = prev.LeftNum + 1
			}
		}
		modStart := 1
		foundMod := false
		for k := chunkStart; k < chunkEnd; k++ {
			if diffResult.Rows[k].RightNum > 0 {
				modStart = diffResult.Rows[k].RightNum
				foundMod = true
				break
			}
		}
		if !foundMod && chunkStart > 0 {
			prev := diffResult.Rows[chunkStart-1]
			if prev.RightNum > 0 {
				modStart = prev.RightNum + 1
			}
		}

		origCount := 0
		modCount := 0
		var lines []string
		for k := chunkStart; k < chunkEnd; k++ {
			row := diffResult.Rows[k]
			if row.LeftType != DiffEmpty {
				origCount++
			}
			if row.RightType != DiffEmpty {
				modCount++
			}
			switch {
			case row.LeftType == DiffDeleted && row.RightType == DiffAdded:
				lines = append(lines, "-"+row.LeftText)
				lines = append(lines, "+"+row.RightText)
			case row.LeftType == DiffDeleted:
				lines = append(lines, "-"+row.LeftText)
			case row.RightType == DiffAdded:
				lines = append(lines, "+"+row.RightText)
			case row.LeftType == DiffUnchanged:
				lines = append(lines, " "+row.LeftText)
			}
		}
		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", origStart, origCount, modStart, modCount)
		hunks = append(hunks, DiffHunk{
			Index:     len(hunks),
			StartRow:  chunkStart,
			EndRow:    chunkEnd,
			OrigStart: origStart,
			OrigCount: origCount,
			ModStart:  modStart,
			ModCount:  modCount,
			Header:    header,
			Lines:     append([]string(nil), lines...),
		})
	}
	return hunks
}

// FormatUnifiedDiff formats a SplitDiffResult into standard unified diff text with context lines.
func FormatUnifiedDiff(diffResult SplitDiffResult, contextLines int) string {
	hunks := SplitDiffHunks(diffResult, contextLines)
	if len(hunks) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, h := range hunks {
		sb.WriteString(h.Header)
		sb.WriteString("\n")
		for _, line := range h.Lines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// FormatUnifiedDiffHunk formats a single hunk as unified diff body (header + lines).
func FormatUnifiedDiffHunk(h DiffHunk) string {
	var sb strings.Builder
	sb.WriteString(h.Header)
	sb.WriteString("\n")
	for _, line := range h.Lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ApplyUnifiedHunkToText applies one unified hunk to base text on target side semantics.
// targetSide: "orig" applies relative to original line numbers, "mod" relative to modified line numbers.
func ApplyUnifiedHunkToText(base string, h DiffHunk, targetSide string) (string, error) {
	baseHadFinalNL := strings.HasSuffix(base, "\n") || strings.HasSuffix(base, "\r\n")
	lines := splitLines(base)

	start := h.OrigStart
	if targetSide == "mod" {
		start = h.ModStart
	}
	if start < 1 {
		start = 1
	}
	idx := start - 1
	if idx > len(lines) {
		idx = len(lines)
	}

	var expected []string
	var replacement []string
	for _, hl := range h.Lines {
		if hl == "" {
			continue
		}
		prefix := hl[0]
		content := ""
		if len(hl) > 1 {
			content = hl[1:]
		}
		switch prefix {
		case ' ':
			expected = append(expected, content)
			replacement = append(replacement, content)
		case '-':
			expected = append(expected, content)
		case '+':
			replacement = append(replacement, content)
		case '\\':
			// no-newline marker
		default:
			return "", fmt.Errorf("invalid hunk line prefix: %q", prefix)
		}
	}

	if idx < 0 || idx+len(expected) > len(lines) {
		return "", fmt.Errorf("hunk out of range")
	}
	for i, exp := range expected {
		if lines[idx+i] != exp {
			return "", fmt.Errorf("hunk context mismatch")
		}
	}

	newLines := make([]string, 0, len(lines)-len(expected)+len(replacement))
	newLines = append(newLines, lines[:idx]...)
	newLines = append(newLines, replacement...)
	newLines = append(newLines, lines[idx+len(expected):]...)

	out := strings.Join(newLines, "\n")
	if base == "" {
		if len(newLines) == 0 {
			return "", nil
		}
		return out + "\n", nil
	}
	if baseHadFinalNL {
		out += "\n"
	}
	return out, nil
}

// ComputeSplitDiff calculates side-by-side split diff using LCS
func ComputeSplitDiff(filePath, original, modified string) SplitDiffResult {
	origLines := splitLines(original)
	modLines := splitLines(modified)

	if len(origLines) == 0 && len(modLines) == 0 {
		return SplitDiffResult{FilePath: filePath}
	}

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
	for k := 0; k < len(ops)/2; k++ {
		ops[k], ops[len(ops)-1-k] = ops[len(ops)-1-k], ops[k]
	}

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

	return SplitDiffResult{FilePath: filePath, Rows: rows}
}

func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type DiffView struct {
	Result       SplitDiffResult
	ScrollOffset int
	Hunks        []DiffHunk
	ActiveHunk   int
}

func NewDiffView(filePath, original, modified string) *DiffView {
	res := ComputeSplitDiff(filePath, original, modified)
	dv := &DiffView{Result: res}
	dv.Hunks = SplitDiffHunks(res, 3)
	return dv
}

func (dv *DiffView) RecomputeHunks() {
	dv.Hunks = SplitDiffHunks(dv.Result, 3)
	if len(dv.Hunks) == 0 {
		dv.ActiveHunk = 0
		return
	}
	if dv.ActiveHunk < 0 {
		dv.ActiveHunk = 0
	}
	if dv.ActiveHunk >= len(dv.Hunks) {
		dv.ActiveHunk = len(dv.Hunks) - 1
	}
}

func (dv *DiffView) NextHunk(viewHeight int) {
	if len(dv.Hunks) == 0 {
		return
	}
	dv.ActiveHunk = (dv.ActiveHunk + 1) % len(dv.Hunks)
	dv.ensureActiveHunkVisible(viewHeight)
}

func (dv *DiffView) PrevHunk(viewHeight int) {
	if len(dv.Hunks) == 0 {
		return
	}
	dv.ActiveHunk--
	if dv.ActiveHunk < 0 {
		dv.ActiveHunk = len(dv.Hunks) - 1
	}
	dv.ensureActiveHunkVisible(viewHeight)
}

func (dv *DiffView) ActiveHunkData() (DiffHunk, bool) {
	if len(dv.Hunks) == 0 || dv.ActiveHunk < 0 || dv.ActiveHunk >= len(dv.Hunks) {
		return DiffHunk{}, false
	}
	return dv.Hunks[dv.ActiveHunk], true
}

func (dv *DiffView) ensureActiveHunkVisible(viewHeight int) {
	h, ok := dv.ActiveHunkData()
	if !ok {
		return
	}
	contentHeight := viewHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}
	target := h.StartRow
	if target < dv.ScrollOffset {
		dv.ScrollOffset = target
	}
	if target >= dv.ScrollOffset+contentHeight {
		dv.ScrollOffset = target - contentHeight + 1
		if dv.ScrollOffset < 0 {
			dv.ScrollOffset = 0
		}
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
	gutterWidth := 5
	colWidth := (width - 1) / 2
	textWidth := colWidth - gutterWidth
	if textWidth < 4 {
		textWidth = 4
	}

	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Background(lipgloss.Color("#3E1A1A"))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Background(lipgloss.Color("#132A1C"))
	emptyStyle := lipgloss.NewStyle().Background(lipgloss.Color("#1B1B1B"))
	gutterDelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Background(lipgloss.Color("#2D1212"))
	gutterAddStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Background(lipgloss.Color("#0E1F14"))
	gutterNormal := ui.CurrentTheme.LineNumberStyle

	hunkInfo := ""
	if len(dv.Hunks) > 0 {
		hunkInfo = fmt.Sprintf("  Hunk %d/%d", dv.ActiveHunk+1, len(dv.Hunks))
	}
	var outLines []string
	headerLeft := lipgloss.NewStyle().Bold(true).Width(colWidth).Background(lipgloss.Color("#21262D")).Render(" HEAD / Original (-)")
	headerRightText := " Working Tree / Modified (+)" + hunkInfo
	headerRight := lipgloss.NewStyle().Bold(true).Width(colWidth).Background(lipgloss.Color("#21262D")).Render(headerRightText)
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerLeft, "│", headerRight)
	outLines = append(outLines, headerRow)

	contentHeight := height - 1
	activeHunk, hasActive := dv.ActiveHunkData()
	for v := 0; v < contentHeight; v++ {
		rowIdx := dv.ScrollOffset + v
		if rowIdx >= len(dv.Result.Rows) {
			emptyLeft := strings.Repeat(" ", colWidth)
			emptyRight := strings.Repeat(" ", colWidth)
			outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, emptyLeft, "│", emptyRight))
			continue
		}
		row := dv.Result.Rows[rowIdx]
		inActiveHunk := hasActive && rowIdx >= activeHunk.StartRow && rowIdx < activeHunk.EndRow
		rowMarker := " "
		if inActiveHunk {
			rowMarker = "▌"
		}

		var leftGutter, leftContent string
		switch row.LeftType {
		case DiffDeleted:
			leftGutter = gutterDelStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.LeftNum))
			txt := truncateOrPad(row.LeftText, textWidth-1)
			leftContent = delStyle.Width(textWidth).Render(rowMarker + txt)
		case DiffEmpty:
			leftGutter = emptyStyle.Width(gutterWidth).Render(strings.Repeat(" ", gutterWidth))
			leftContent = emptyStyle.Width(textWidth).Render(strings.Repeat(" ", textWidth))
		default:
			leftGutter = gutterNormal.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.LeftNum))
			txt := truncateOrPad(row.LeftText, textWidth-1)
			style := lipgloss.NewStyle().Width(textWidth)
			if inActiveHunk {
				style = style.Background(lipgloss.Color("#1F2A33"))
			}
			leftContent = style.Render(rowMarker + txt)
		}
		leftCol := leftGutter + leftContent

		var rightGutter, rightContent string
		switch row.RightType {
		case DiffAdded:
			rightGutter = gutterAddStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.RightNum))
			txt := truncateOrPad(row.RightText, textWidth-1)
			rightContent = addStyle.Width(textWidth).Render(rowMarker + txt)
		case DiffEmpty:
			rightGutter = emptyStyle.Width(gutterWidth).Render(strings.Repeat(" ", gutterWidth))
			rightContent = emptyStyle.Width(textWidth).Render(strings.Repeat(" ", textWidth))
		default:
			rightGutter = gutterNormal.Width(gutterWidth).Render(fmt.Sprintf("%4d ", row.RightNum))
			txt := truncateOrPad(row.RightText, textWidth-1)
			style := lipgloss.NewStyle().Width(textWidth)
			if inActiveHunk {
				style = style.Background(lipgloss.Color("#1F2A33"))
			}
			rightContent = style.Render(rowMarker + txt)
		}
		rightCol := rightGutter + rightContent
		outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "│", rightCol))
	}
	return strings.Join(outLines, "\n")
}

func truncateOrPad(s string, width int) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	w := ansi.StringWidth(s)
	if w > width {
		return ansi.Truncate(s, width, "")
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
