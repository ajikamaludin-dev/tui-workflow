package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Indent Rainbow colors (VSCode-like ANSI palette)
var (
	indentRainbowStyles = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")), // Gold / Yellow
		lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")), // Blue
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD")), // Purple
		lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")), // Green
		lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2")), // Cyan
		lipgloss.NewStyle().Foreground(lipgloss.Color("#D19A66")), // Orange
	}

	indentErrorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#E06C75")).
				Foreground(lipgloss.Color("#FFFFFF"))
)

// CountLeadingSpaces counts the number of leading space/tab characters in a rune slice.
func CountLeadingSpaces(runes []rune) (int, bool) {
	count := 0
	hasTab := false
	for _, r := range runes {
		if r == ' ' {
			count++
		} else if r == '\t' {
			count += TabWidth
			hasTab = true
		} else {
			break
		}
	}
	return count, hasTab
}

// RenderIndentRune renders a rune within the leading whitespace indentation zone.
// If the indentation is misaligned (leadingSpaces % TabWidth != 0), it renders with error highlighting.
// Otherwise it renders a color-coded vertical guide bar or space.
func RenderIndentRune(visualCol, leadingSpaces int, r rune) (string, bool) {
	if r != ' ' && r != '\t' {
		return "", false
	}
	rw := 1
	if r == '\t' {
		rem := visualCol % TabWidth
		rw = TabWidth - rem
	}

	// Indentation error guard: misaligned spaces
	if leadingSpaces%TabWidth != 0 {
		return indentErrorStyle.Render(strings.Repeat(" ", rw)), true
	}

	level := visualCol / TabWidth
	offset := visualCol % TabWidth
	style := indentRainbowStyles[level%len(indentRainbowStyles)]

	if offset == 0 {
		if rw > 1 {
			return style.Render("│") + strings.Repeat(" ", rw-1), true
		}
		return style.Render("│"), true
	}

	return strings.Repeat(" ", rw), true
}

// SetDiagnostic adds or clears a diagnostic marker for a physical line (0-based).
func (b *Buffer) SetDiagnostic(lineIdx int, marker string) {
	if b.LineDiagnostics == nil {
		b.LineDiagnostics = make(map[int]string)
	}
	if marker == "" {
		delete(b.LineDiagnostics, lineIdx)
	} else {
		b.LineDiagnostics[lineIdx] = marker
	}
}

// ClearDiagnostics clears all diagnostics for this buffer.
func (b *Buffer) ClearDiagnostics() {
	b.LineDiagnostics = make(map[int]string)
}
