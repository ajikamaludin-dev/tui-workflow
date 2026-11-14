package dialog

import (
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// lipgloss emits resets as \x1b[m, while some writers emit \x1b[0m.
// Match both exactly (not e.g. \x1b[7;37m).
var ansiResetRe = regexp.MustCompile(`\x1b\[0?m`)

// NormalizeDialogBackground ensures that all padding spaces and inner elements
// inside a dialog box have the specified background color, preventing visual "holes"
// caused by child ANSI resets and by unstyled trailing padding.
// It also pads every interior line to the box width so dialogs render as a
// perfect rectangle with no empty gaps at the end of a row.
func NormalizeDialogBackground(rendered string, bg color.Color) string {
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

	// Determine the full box width from the widest line (usually the border).
	maxW := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > maxW {
			maxW = w
		}
	}

	for i := 1; i < len(lines)-1; i++ {
		line := lines[i]

		// Re-apply the background after every child reset so inner styled
		// segments cannot punch holes through the dialog background.
		line = ansiResetRe.ReplaceAllStringFunc(line, func(s string) string {
			return s + bgSeq
		})

		// Trailing unstyled spaces (e.g. from JoinVertical alignment) leak the
		// terminal default background; recolor them with the dialog background.
		if trimmed := strings.TrimRight(line, " "); len(trimmed) != len(line) {
			line = trimmed + bgSeq + strings.Repeat(" ", len(line)-len(trimmed))
		}

		// Pad shorter interior lines to the full box width with background.
		if w := ansi.StringWidth(line); w < maxW {
			line = line + bgSeq + strings.Repeat(" ", maxW-w)
		}

		lines[i] = line
	}

	return strings.Join(lines, "\n")
}
