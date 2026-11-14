package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type MarkdownViewer struct {
	ScrollOffset int
	cachedKey    string
	cachedLines  []string
	renderer     *glamour.TermRenderer
	renderWidth  int

	SelectionStartRow int
	SelectionStartCol int
	SelectionEndRow   int
	SelectionEndCol   int
	HasSelection      bool
}

func NewMarkdownViewer() *MarkdownViewer {
	return &MarkdownViewer{
		ScrollOffset: 0,
		cachedLines:  make([]string, 0),
	}
}

func (mv *MarkdownViewer) ScrollUp(n int) {
	mv.ScrollOffset -= n
	if mv.ScrollOffset < 0 {
		mv.ScrollOffset = 0
	}
}

func (mv *MarkdownViewer) ScrollDown(n int, viewHeight int) {
	maxOffset := len(mv.cachedLines) - viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	mv.ScrollOffset += n
	if mv.ScrollOffset > maxOffset {
		mv.ScrollOffset = maxOffset
	}
}

func (mv *MarkdownViewer) ScrollHome() {
	mv.ScrollOffset = 0
}

func (mv *MarkdownViewer) ClearSelection() {
	mv.HasSelection = false
	mv.SelectionStartRow = 0
	mv.SelectionStartCol = 0
	mv.SelectionEndRow = 0
	mv.SelectionEndCol = 0
}

func (mv *MarkdownViewer) SelectAll() {
	if len(mv.cachedLines) == 0 {
		return
	}
	mv.HasSelection = true
	mv.SelectionStartRow = 0
	mv.SelectionStartCol = 0
	mv.SelectionEndRow = len(mv.cachedLines) - 1
	lastLine := mv.cachedLines[len(mv.cachedLines)-1]
	mv.SelectionEndCol = lipgloss.Width(ansi.Strip(lastLine))
}

func (mv *MarkdownViewer) StartSelection(row, col int) {
	if row < 0 {
		row = 0
	}
	if len(mv.cachedLines) > 0 && row >= len(mv.cachedLines) {
		row = len(mv.cachedLines) - 1
	}
	mv.HasSelection = true
	mv.SelectionStartRow = row
	mv.SelectionStartCol = col
	mv.SelectionEndRow = row
	mv.SelectionEndCol = col
}

func (mv *MarkdownViewer) UpdateSelection(row, col int) {
	if !mv.HasSelection {
		mv.StartSelection(row, col)
		return
	}
	if row < 0 {
		row = 0
	}
	if len(mv.cachedLines) > 0 && row >= len(mv.cachedLines) {
		row = len(mv.cachedLines) - 1
	}
	mv.SelectionEndRow = row
	mv.SelectionEndCol = col
}

func (mv *MarkdownViewer) NormalizedSelection() (int, int, int, int) {
	sRow, sCol := mv.SelectionStartRow, mv.SelectionStartCol
	eRow, eCol := mv.SelectionEndRow, mv.SelectionEndCol
	if sRow > eRow || (sRow == eRow && sCol > eCol) {
		return eRow, eCol, sRow, sCol
	}
	return sRow, sCol, eRow, eCol
}

func (mv *MarkdownViewer) GetSelectedText() string {
	if !mv.HasSelection || len(mv.cachedLines) == 0 {
		return ""
	}
	sRow, sCol, eRow, eCol := mv.NormalizedSelection()
	if sRow >= len(mv.cachedLines) {
		return ""
	}
	if eRow >= len(mv.cachedLines) {
		eRow = len(mv.cachedLines) - 1
	}

	var selectedLines []string
	for r := sRow; r <= eRow; r++ {
		clean := ansi.Strip(mv.cachedLines[r])
		runes := []rune(clean)
		if r == sRow && r == eRow {
			start := sCol
			end := eCol
			if start > len(runes) {
				start = len(runes)
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start < end {
				selectedLines = append(selectedLines, string(runes[start:end]))
			}
		} else if r == sRow {
			start := sCol
			if start > len(runes) {
				start = len(runes)
			}
			selectedLines = append(selectedLines, string(runes[start:]))
		} else if r == eRow {
			end := eCol
			if end > len(runes) {
				end = len(runes)
			}
			selectedLines = append(selectedLines, string(runes[:end]))
		} else {
			selectedLines = append(selectedLines, clean)
		}
	}
	return strings.Join(selectedLines, "\n")
}

func (mv *MarkdownViewer) GetAllCleanText() string {
	var lines []string
	for _, l := range mv.cachedLines {
		lines = append(lines, ansi.Strip(l))
	}
	return strings.Join(lines, "\n")
}

func (mv *MarkdownViewer) Render(rawText string, width, height int) string {
	if width < 10 || height < 1 {
		return ""
	}

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	// Compute cache key: width + hash of content
	h := sha256.Sum256([]byte(rawText))
	hashStr := hex.EncodeToString(h[:8])
	key := fmt.Sprintf("%d:%s", wrapWidth, hashStr)

	if key != mv.cachedKey || mv.renderer == nil || mv.renderWidth != wrapWidth {
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(wrapWidth),
		)
		if err == nil {
			out, err := r.Render(rawText)
			if err == nil {
				out = strings.ReplaceAll(out, "\r\n", "\n")
				out = strings.ReplaceAll(out, "\r", "\n")
				lines := strings.Split(out, "\n")
				mv.cachedLines = lines
				mv.cachedKey = key
				mv.renderer = r
				mv.renderWidth = wrapWidth
			}
		}
	}

	// Viewport slice
	selStyle := lipgloss.NewStyle().Background(lipgloss.Color("#45475a")).Foreground(lipgloss.Color("#ffffff"))
	sRow, _, eRow, _ := mv.NormalizedSelection()

	var viewLines []string
	for i := 0; i < height; i++ {
		rowIdx := mv.ScrollOffset + i
		if rowIdx < len(mv.cachedLines) {
			line := mv.cachedLines[rowIdx]
			if mv.HasSelection && rowIdx >= sRow && rowIdx <= eRow {
				clean := ansi.Strip(line)
				line = selStyle.Render(clean)
			}
			// Ensure it does not overflow width
			if lipgloss.Width(line) > width {
				line = truncateAnsi(line, width)
			}
			viewLines = append(viewLines, line)
		} else {
			viewLines = append(viewLines, "")
		}
	}

	return strings.Join(viewLines, "\n")
}

func truncateAnsi(s string, width int) string {
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}
