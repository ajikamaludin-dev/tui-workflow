package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/app"
	"tui-workflow/internal/logger"
)

func main() {
	targetPath := "."
	explicitArg := false
	debugMode := os.Getenv("DEBUG") == "1" || os.Getenv("TUI_DEBUG") == "1"

	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			debugMode = true
		} else if targetPath == "." {
			targetPath = arg
			explicitArg = true
		}
	}

	if debugMode {
		if err := logger.Init(true, "debug.log"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize debug logger: %v\n", err)
		} else {
			defer logger.Close()
			logger.Debugf("tui-workflow started with path: %s (debug mode enabled)", targetPath)
		}
	}

	model, err := app.NewModelWithPath(targetPath, explicitArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error path %q: %v\n", targetPath, err)
		os.Exit(1)
	}

	// Set cursor shape steady I-beam (\x1b[6 q) and restore default exit
	fmt.Print("\x1b[6 q")
	fmt.Print("\x1b[0 q\x1b]22;default\x1b\\")

	// Program options moved to declarative tea.View fields in Bubble Tea v2
	// (see Model.View: AltScreen + MouseModeCellMotion).
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error tui-workflow: %v\n", err)
		os.Exit(1)
	}
}
