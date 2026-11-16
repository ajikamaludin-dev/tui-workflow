package git

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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

// LineChange classifies how a working-tree line differs from the index
// (the VS Code "Changes" gutter semantics).
type LineChange int

const (
	LineUnchanged LineChange = iota
	LineAdded
	LineModified
	LineDeleted
)

// ComputeLineChanges maps an LCS split diff of (original → modified) to
// per-modified-line markers keyed by 1-based modified line number.
func ComputeLineChanges(original, modified string) map[int]LineChange {
	res := ComputeSplitDiff("", original, modified)
	return LineChangesFromSplitDiff(res.Rows)
}

// LineChangesFromSplitDiff converts split-diff rows into per-modified-line
// change markers. Rules:
//   - paired delete+add row → LineModified
//   - pure add row → LineAdded
//   - a pure delete block marks the next modified line LineDeleted (gutter
//     marker on the line after the deletion); if the deletion is at EOF and
//     no following modified line exists, the last modified line is marked.
//   - a modified/added line is never overwritten by a pending delete marker.
func LineChangesFromSplitDiff(rows []DiffRow) map[int]LineChange {
	changes := make(map[int]LineChange)
	pendingDel := false
	lastModifiedLine := 0
	for _, row := range rows {
		hasRight := row.RightType != DiffEmpty && row.RightNum > 0
		if row.RightType == DiffAdded {
			if row.LeftType == DiffDeleted {
				changes[row.RightNum] = LineModified
			} else if _, ok := changes[row.RightNum]; !ok {
				changes[row.RightNum] = LineAdded
			}
		}
		if hasRight {
			lastModifiedLine = row.RightNum
			if pendingDel {
				if _, ok := changes[row.RightNum]; !ok {
					changes[row.RightNum] = LineDeleted
				}
				pendingDel = false
			}
			continue
		}
		if row.LeftType == DiffDeleted {
			pendingDel = true
		}
	}
	if pendingDel && lastModifiedLine > 0 {
		if _, ok := changes[lastModifiedLine]; !ok {
			changes[lastModifiedLine] = LineDeleted
		}
	}
	return changes
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

	vlCache       *diffVisualCache
	lastTextWidth int
}

// DiffVisualLine maps one rendered visual line back to its logical diff row.
// Long cell texts are soft-wrapped into multiple segments; a row occupies
// max(leftSegs, rightSegs) visual lines.
type DiffVisualLine struct {
	RowIdx  int
	Segment int
}

type diffVisualCache struct {
	Lines       []DiffVisualLine
	RowToVisual []int      // first visual index per row
	LeftSegs    [][]string // per row, wrapped left-cell segments (nil if DiffEmpty)
	RightSegs   [][]string // per row, wrapped right-cell segments (nil if DiffEmpty)
	Width       int        // textWidth used to build the cache
	RowCount    int        // len(Result.Rows) used to build the cache
}

// wrapDiffText expands tabs and splits s into segments of at most width visual
// columns. ANSI-unaware by design: cell styling is applied after wrapping and
// raw diff texts never carry escape sequences.
func wrapDiffText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	s = strings.ReplaceAll(s, "\t", "    ")
	if ansi.StringWidth(s) <= width {
		return []string{s}
	}
	var segs []string
	var seg strings.Builder
	segWidth := 0
	for _, r := range s {
		rw := ansi.StringWidth(string(r))
		if rw == 0 {
			seg.WriteRune(r)
			continue
		}
		if segWidth+rw > width && seg.Len() > 0 {
			segs = append(segs, seg.String())
			seg.Reset()
			segWidth = 0
		}
		seg.WriteRune(r)
		segWidth += rw
	}
	if seg.Len() > 0 || len(segs) == 0 {
		segs = append(segs, seg.String())
	}
	return segs
}

func (dv *DiffView) visualCache(textWidth int) *diffVisualCache {
	if dv.vlCache != nil && dv.vlCache.Width == textWidth && dv.vlCache.RowCount == len(dv.Result.Rows) {
		return dv.vlCache
	}
	cache := &diffVisualCache{
		RowToVisual: make([]int, len(dv.Result.Rows)),
		LeftSegs:    make([][]string, len(dv.Result.Rows)),
		RightSegs:   make([][]string, len(dv.Result.Rows)),
		Width:       textWidth,
		RowCount:    len(dv.Result.Rows),
	}
	for i := range dv.Result.Rows {
		row := &dv.Result.Rows[i]
		leftSegs, rightSegs := 0, 0
		if row.LeftType != DiffEmpty {
			cache.LeftSegs[i] = wrapDiffText(row.LeftText, textWidth-1)
			leftSegs = len(cache.LeftSegs[i])
		}
		if row.RightType != DiffEmpty {
			cache.RightSegs[i] = wrapDiffText(row.RightText, textWidth-1)
			rightSegs = len(cache.RightSegs[i])
		}
		h := leftSegs
		if rightSegs > h {
			h = rightSegs
		}
		if h < 1 {
			h = 1
		}
		cache.RowToVisual[i] = len(cache.Lines)
		for s := 0; s < h; s++ {
			cache.Lines = append(cache.Lines, DiffVisualLine{RowIdx: i, Segment: s})
		}
	}
	dv.vlCache = cache
	return cache
}

func (dv *DiffView) textWidth() int {
	if dv.lastTextWidth > 0 {
		return dv.lastTextWidth
	}
	return 60
}

func (dv *DiffView) maxScrollOffset(viewHeight int) int {
	contentHeight := viewHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}
	total := len(dv.visualCache(dv.textWidth()).Lines)
	maxOffset := total - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	return maxOffset
}

// ActiveLine returns the 1-based working-tree (or HEAD) line number of the
// first visible visual line; used for the status bar "Ln" indicator.
func (dv *DiffView) ActiveLine() int {
	cache := dv.visualCache(dv.textWidth())
	if len(cache.Lines) == 0 {
		return 1
	}
	idx := dv.ScrollOffset
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cache.Lines) {
		idx = len(cache.Lines) - 1
	}
	row := dv.Result.Rows[cache.Lines[idx].RowIdx]
	if row.RightNum > 0 {
		return row.RightNum
	}
	if row.LeftNum > 0 {
		return row.LeftNum
	}
	return 1
}

func NewDiffView(filePath, original, modified string) *DiffView {
	res := ComputeSplitDiff(filePath, original, modified)
	dv := &DiffView{Result: res}
	dv.Hunks = SplitDiffHunks(res, 3)
	return dv
}

func (dv *DiffView) RecomputeHunks() {
	dv.Hunks = SplitDiffHunks(dv.Result, 3)
	dv.vlCache = nil
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
	cache := dv.visualCache(dv.textWidth())
	target := 0
	if h.StartRow >= 0 && h.StartRow < len(cache.RowToVisual) {
		target = cache.RowToVisual[h.StartRow]
	}
	// Position target at vertical center of viewport
	dv.ScrollOffset = target - (contentHeight / 2)
	if dv.ScrollOffset < 0 {
		dv.ScrollOffset = 0
	}
	if maxOffset := dv.maxScrollOffset(viewHeight); dv.ScrollOffset > maxOffset {
		dv.ScrollOffset = maxOffset
	}
}

func (dv *DiffView) ScrollUp(n int) {
	dv.ScrollOffset -= n
	if dv.ScrollOffset < 0 {
		dv.ScrollOffset = 0
	}
}

func (dv *DiffView) ScrollDown(n int, viewHeight int) {
	dv.ScrollOffset += n
	if maxOffset := dv.maxScrollOffset(viewHeight); dv.ScrollOffset > maxOffset {
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
	dv.lastTextWidth = textWidth
	if so := dv.maxScrollOffset(height); dv.ScrollOffset > so {
		dv.ScrollOffset = so
	}
	cache := dv.visualCache(textWidth)

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
	headerRight := lipgloss.NewStyle().Bold(true).Width(colWidth).Background(lipgloss.Color("#21262D")).Render(ansi.Truncate(headerRightText, colWidth, ""))
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerLeft, "│", headerRight)
	outLines = append(outLines, headerRow)

	padDiff := func(s string, w int) string {
		if sw := ansi.StringWidth(s); sw < w {
			return s + strings.Repeat(" ", w-sw)
		}
		return ansi.Truncate(s, w, "")
	}
	cell := func(num int, typ LineDiffType, segs []string, segIdx int, marker string, inActiveHunk bool) (string, string) {
		segText := ""
		if segIdx >= 0 && segIdx < len(segs) {
			segText = segs[segIdx]
		}
		continuation := segIdx > 0
		switch typ {
		case DiffDeleted:
			gutter := gutterDelStyle.Width(gutterWidth).Render(" ")
			if !continuation {
				gutter = gutterDelStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", num))
			}
			content := delStyle.Width(textWidth).Render(marker + padDiff(segText, textWidth-1))
			return gutter, content
		case DiffAdded:
			gutter := gutterAddStyle.Width(gutterWidth).Render(" ")
			if !continuation {
				gutter = gutterAddStyle.Width(gutterWidth).Render(fmt.Sprintf("%4d ", num))
			}
			content := addStyle.Width(textWidth).Render(marker + padDiff(segText, textWidth-1))
			return gutter, content
		case DiffEmpty:
			gutter := emptyStyle.Width(gutterWidth).Render(strings.Repeat(" ", gutterWidth))
			content := emptyStyle.Width(textWidth).Render(strings.Repeat(" ", textWidth))
			return gutter, content
		default: // DiffUnchanged
			gutter := gutterNormal.Width(gutterWidth).Render(" ")
			if !continuation {
				gutter = gutterNormal.Width(gutterWidth).Render(fmt.Sprintf("%4d ", num))
			}
			style := lipgloss.NewStyle().Width(textWidth)
			if inActiveHunk {
				style = style.Background(lipgloss.Color("#1F2A33"))
			}
			return gutter, style.Render(marker + padDiff(segText, textWidth-1))
		}
	}

	contentHeight := height - 1
	activeHunk, hasActive := dv.ActiveHunkData()
	emptyLeft := strings.Repeat(" ", colWidth)
	emptyRight := strings.Repeat(" ", colWidth)
	for v := 0; v < contentHeight; v++ {
		vlIdx := dv.ScrollOffset + v
		if vlIdx >= len(cache.Lines) {
			outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, emptyLeft, "│", emptyRight))
			continue
		}
		dvl := cache.Lines[vlIdx]
		row := dv.Result.Rows[dvl.RowIdx]
		inActiveHunk := hasActive && dvl.RowIdx >= activeHunk.StartRow && dvl.RowIdx < activeHunk.EndRow
		rowMarker := " "
		if inActiveHunk {
			rowMarker = "▌"
		}
		leftGutter, leftContent := cell(row.LeftNum, row.LeftType, cache.LeftSegs[dvl.RowIdx], dvl.Segment, rowMarker, inActiveHunk)
		rightGutter, rightContent := cell(row.RightNum, row.RightType, cache.RightSegs[dvl.RowIdx], dvl.Segment, rowMarker, inActiveHunk)
		outLines = append(outLines, lipgloss.JoinHorizontal(lipgloss.Top, leftGutter+leftContent, "│", rightGutter+rightContent))
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
