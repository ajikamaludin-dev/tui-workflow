package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/app"
)

func main() {
	model := app.NewModel()
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running tui-workflow: %v\n", err)
		os.Exit(1)
	}
}
