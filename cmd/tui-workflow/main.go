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

	// Set cursor shape to steady I-beam (\x1b[6 q) and restore default at exit
	fmt.Print("\x1b[6 q")
	defer fmt.Print("\x1b[0 q\x1b]22;default\x1b\\")

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
