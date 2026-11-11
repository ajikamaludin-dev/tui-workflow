package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/app"
)

func main() {
	targetPath := "."
	if len(os.Args) > 1 {
		targetPath = os.Args[1]
	}

	model, err := app.NewModelWithPath(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening path %q: %v\n", targetPath, err)
		os.Exit(1)
	}

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
