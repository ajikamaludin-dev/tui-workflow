package app

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func overlayAt(base, overlay string, atX, atY, totalW, totalH int) string {
	baseLines := strings.Split(base, "\n")
	overLines := strings.Split(overlay, "\n")

	for i, oLine := range overLines {
		targetY := atY + i
		if targetY >= len(baseLines) {
			break
		}

		bLine := baseLines[targetY]
		baseWidth := ansi.StringWidth(bLine)
		oWidth := ansi.StringWidth(oLine)

		var left, right string
		if atX > 0 {
			left = ansi.Cut(bLine, 0, atX)
			leftW := ansi.StringWidth(left)
			if leftW < atX {
				left += strings.Repeat(" ", atX-leftW)
			}
		}

		rightStart := atX + oWidth
		if rightStart < baseWidth {
			right = ansi.Cut(bLine, rightStart, baseWidth)
		}

		baseLines[targetY] = left + oLine + right
	}

	return strings.Join(baseLines, "\n")
}

func overlayCenter(base, modal string, totalW, totalH int) string {
	mWidth := lipgloss.Width(modal)
	mHeight := lipgloss.Height(modal)
	atX := (totalW - mWidth) / 2
	atY := (totalH - mHeight) / 2
	if atX < 0 {
		atX = 0
	}
	if atY < 1 {
		atY = 1
	}
	return overlayAt(base, modal, atX, atY, totalW, totalH)
}

func overlayTopCenter(base, modal string, totalW, totalH int) string {
	mWidth := lipgloss.Width(modal)
	mHeight := lipgloss.Height(modal)
	atX := (totalW - mWidth) / 2
	atY := (totalH - mHeight) / 3
	if atX < 0 {
		atX = 0
	}
	if atY < 1 {
		atY = 1
	}
	return overlayAt(base, modal, atX, atY, totalW, totalH)
}

func placeCenter(base, modal string, totalW, totalH int) string {
	return overlayCenter(base, modal, totalW, totalH)
}
