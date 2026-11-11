package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type MarkdownViewer struct {
	ScrollOffset int
	cachedKey    string
	cachedLines  []string
	renderer     *glamour.TermRenderer
	renderWidth  int
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
	var viewLines []string
	for i := 0; i < height; i++ {
		rowIdx := mv.ScrollOffset + i
		if rowIdx < len(mv.cachedLines) {
			line := mv.cachedLines[rowIdx]
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
	// Simple rune truncation if lipgloss width exceeds
	if lipgloss.Width(s) <= width {
		return s
	}
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
