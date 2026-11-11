package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

const (
	MinWidth  = 80
	MinHeight = 24
)

func IsTooSmall(w, h int) bool {
	return w < MinWidth || h < MinHeight
}

func RenderTooSmallWarning(w, h int) string {
	msg := fmt.Sprintf(
		"⚠️  Ukuran Terminal Terlalu Kecil\n\n"+
			"Minimum Diperlukan: %d × %d\n"+
			"Ukuran Saat Ini   : %d × %d\n\n"+
			"Silakan perbesar jendela terminal Anda untuk melanjutkan.",
		MinWidth, MinHeight, w, h,
	)

	box := ui.CurrentTheme.WarningBoxStyle.Render(msg)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
