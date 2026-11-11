package dialog

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NormalizeDialogBackground ensures that all padding spaces and inner elements
// inside a dialog box have the specified background color, preventing visual "holes"
// caused by child ANSI resets (\x1b[0m).
func NormalizeDialogBackground(rendered string, bg lipgloss.Color) string {
	if rendered == "" {
		return ""
	}

	// Extract the raw ANSI background sequence for the specified color.
	bgStyle := lipgloss.NewStyle().Background(bg)
	sample := bgStyle.Render("X")
	idx := strings.Index(sample, "X")
	if idx == -1 {
		return rendered
	}
	bgSeq := sample[:idx]

	lines := strings.Split(rendered, "\n")
	if len(lines) <= 2 {
		return rendered
	}

	for i := 1; i < len(lines)-1; i++ {
		line := lines[i]
		lastReset := strings.LastIndex(line, "\x1b[0m")
		if lastReset != -1 {
			prefix := line[:lastReset]
			suffix := line[lastReset:]
			fixedPrefix := strings.ReplaceAll(prefix, "\x1b[0m", "\x1b[0m"+bgSeq)
			lines[i] = fixedPrefix + suffix
		}
	}

	return strings.Join(lines, "\n")
}
