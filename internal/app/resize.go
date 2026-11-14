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
		"⚠️  Terminal Size Too Small\n\n"+
			"Minimum Required: %d × %d\n"+
			"Current Size    : %d × %d\n\n"+
			"Please resize your terminal window to continue.",
		MinWidth, MinHeight, w, h,
	)

	box := ui.CurrentTheme.WarningBoxStyle.Render(msg)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
