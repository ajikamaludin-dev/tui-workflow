package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type StatusBar struct {
	AutoSave   bool
	Branch     string
	FilePath   string
	Line       int
	Col        int
	FileSyntax string
	Encoding   string
	ToastMsg   string
	ToastIsErr bool
	ToastUntil time.Time
	Width      int
}

func NewStatusBar() *StatusBar {
	return &StatusBar{
		AutoSave:   true,
		FilePath:   "Welcome to tui-workflow",
		Line:       1,
		Col:        1,
		FileSyntax: "Plain Text",
		Encoding:   "UTF-8",
		Width:      80,
	}
}

func (s *StatusBar) SetWidth(w int) {
	s.Width = w
}

func (s *StatusBar) ShowToast(msg string, isErr bool, duration time.Duration) {
	s.ToastMsg = msg
	s.ToastIsErr = isErr
	s.ToastUntil = time.Now().Add(duration)
}

// HandleMouseClick determines which status bar component was clicked based on column X.
func (s *StatusBar) HandleMouseClick(colX int) string {
	autoSaveLen := len("[AutoSave: ON]") + 2 // padding (0, 1) = 2
	if !s.AutoSave {
		autoSaveLen = len("[AutoSave: OFF]") + 2
	}

	// 1. AutoSave badge click
	if colX >= 0 && colX < autoSaveLen {
		return "toggle_autosave"
	}

	curX := autoSaveLen

	// If toast is displayed, ignore middle elements
	if s.ToastMsg != "" {
		// Only check right-hand side for help
	} else {
		// 2. Branch badge click
		if s.Branch != "" {
			branchStr := " git: " + s.Branch + " "
			branchLen := ansi.StringWidth(branchStr)
			if colX >= curX && colX < curX+branchLen {
				return "branch_switcher"
			}
			curX += branchLen
		}

		// 3. File info
		fileStr := " " + s.FilePath + " "
		fileLen := ansi.StringWidth(fileStr)
		curX += fileLen

		// 4. Line / Col info click
		posStr := fmt.Sprintf(" Ln %d, Col %d ", s.Line, s.Col)
		posLen := ansi.StringWidth(posStr)
		if colX >= curX && colX < curX+posLen {
			return "goto_line"
		}
		curX += posLen
	}

	// 5. Right side: check if user clicked on help "?: Help"
	// Help item is the last item on the right
	if colX >= s.Width-12 && colX <= s.Width {
		return "help"
	}

	return ""
}

func (s *StatusBar) Render() string {
	// Check toast expiration
	if s.ToastMsg != "" && time.Now().After(s.ToastUntil) {
		s.ToastMsg = ""
	}

	var leftPart, rightPart string

	// AutoSave Badge
	autoSaveText := "[AutoSave: ON]"
	if !s.AutoSave {
		autoSaveText = "[AutoSave: OFF]"
	}
	autoSaveBadge := CurrentTheme.StatusBarBadgeStyle.Render(autoSaveText)

	branchBadge := ""
	if s.Branch != "" {
		branchBadge = CurrentTheme.StatusBarItemStyle.Render("git: " + s.Branch)
	}

	if s.ToastMsg != "" {
		toastStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Background(CurrentTheme.Red).
			Foreground(CurrentTheme.FgHighlight)
		if !s.ToastIsErr {
			toastStyle = toastStyle.Background(CurrentTheme.Green)
		}
		leftPart = autoSaveBadge + " " + toastStyle.Render(s.ToastMsg)
	} else {
		fileInfo := CurrentTheme.StatusBarItemStyle.Render(s.FilePath)
		posInfo := CurrentTheme.StatusBarItemStyle.Render(fmt.Sprintf("Ln %d, Col %d", s.Line, s.Col))
		leftPart = autoSaveBadge + branchBadge + fileInfo + posInfo
	}

	// Dynamic rightPart depending on available width
	helpTipsText := "Alt+F: Menu | Ctrl+E: Focus | ?: Help"
	if s.Width < 120 {
		helpTipsText = "Alt+F: Menu | ?: Help"
	}
	if s.Width < 90 {
		helpTipsText = "?: Help"
	}
	if s.Width < 70 {
		helpTipsText = ""
	}

	var rightItems []string
	if s.FileSyntax != "" && s.Width >= 60 {
		rightItems = append(rightItems, CurrentTheme.StatusBarItemStyle.Render(s.FileSyntax))
	}
	if s.Encoding != "" && s.Width >= 80 {
		rightItems = append(rightItems, CurrentTheme.StatusBarItemStyle.Render(s.Encoding))
	}
	if helpTipsText != "" {
		rightItems = append(rightItems, CurrentTheme.StatusBarItemStyle.Render(helpTipsText))
	}
	rightPart = strings.Join(rightItems, "")

	leftWidth := ansi.StringWidth(leftPart)
	rightWidth := ansi.StringWidth(rightPart)

	// If total width exceeds s.Width, truncate leftPart (file info)
	if leftWidth+rightWidth > s.Width {
		availableLeft := s.Width - rightWidth - 1
		if availableLeft > len(autoSaveText)+4 {
			leftPart = ansi.Truncate(leftPart, availableLeft, "…")
			leftWidth = ansi.StringWidth(leftPart)
		} else {
			leftPart = autoSaveBadge
			leftWidth = ansi.StringWidth(leftPart)
		}
	}

	gap := s.Width - leftWidth - rightWidth
	if gap < 0 {
		gap = 0
	}

	barContent := leftPart + strings.Repeat(" ", gap) + rightPart
	if ansi.StringWidth(barContent) > s.Width {
		barContent = ansi.Truncate(barContent, s.Width, "")
	}

	return CurrentTheme.StatusBarStyle.Width(s.Width).MaxHeight(1).Render(barContent)
}
