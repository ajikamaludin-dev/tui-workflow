package dialog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type ConfirmDialogType int

const (
	ConfirmDelete ConfirmDialogType = iota
	ConfirmUnsavedChanges
	ConfirmExternalChange
	ConfirmDiscardGit
	ConfirmDiscardGitHunk
	ConfirmAlert
	ConfirmCloseTerminal
)

type ConfirmDialog struct {
	Active     bool
	Type       ConfirmDialogType
	Title      string
	Message    string
	TargetPath string
	Selected   int // 0: Confirm, 1: Cancel, 2: Discard (for unsaved)
	Width      int
}

func NewConfirmDialog() *ConfirmDialog {
	return &ConfirmDialog{
		Active:   false,
		Selected: 1, // Default Cancel for safety
		Width:    48,
	}
}

func (d *ConfirmDialog) Open(t ConfirmDialogType, title, message, targetPath string) {
	d.Active = true
	d.Type = t
	d.Title = title
	d.Message = message
	d.TargetPath = targetPath
	if t == ConfirmAlert {
		d.Selected = 0
		d.Width = 56
	} else if t == ConfirmExternalChange {
		d.Selected = 1
		d.Width = 54
	} else if t == ConfirmCloseTerminal {
		d.Selected = 1
		d.Width = 52
	} else {
		d.Selected = 1
		d.Width = 48
	}
}

func (d *ConfirmDialog) Close() {
	d.Active = false
}

func (d *ConfirmDialog) Update(msg tea.Msg) (confirmed bool, canceled bool, discard bool) {
	if !d.Active {
		return false, false, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			d.Close()
			return false, true, false

		case "space", " ":
			if d.Type == ConfirmAlert {
				d.Close()
				return true, false, false
			}
			return false, false, false

		case "left", "h":
			if d.Type == ConfirmAlert {
				return false, false, false
			}
			d.Selected--
			if d.Selected < 0 {
				if d.Type == ConfirmUnsavedChanges || d.Type == ConfirmExternalChange {
					d.Selected = 2
				} else {
					d.Selected = 1
				}
			}
			return false, false, false

		case "right", "l", "tab":
			if d.Type == ConfirmAlert {
				return false, false, false
			}
			maxIdx := 1
			if d.Type == ConfirmUnsavedChanges || d.Type == ConfirmExternalChange {
				maxIdx = 2
			}
			d.Selected = (d.Selected + 1) % (maxIdx + 1)
			return false, false, false

		case "enter":
			d.Close()
			if d.Type == ConfirmAlert {
				return true, false, false
			}
			if d.Type == ConfirmUnsavedChanges || d.Type == ConfirmExternalChange {
				if d.Selected == 0 {
					return true, false, false // Save & close OR Reload from disk
				} else if d.Selected == 1 {
					return false, false, true // Discard & close OR Merge (Split Diff)
				} else {
					return false, true, false // Cancel
				}
			} else if d.Type == ConfirmDiscardGitHunk {
				if d.Selected == 0 {
					return true, false, false // Discard hunk
				}
				return false, true, false
			} else if d.Type == ConfirmCloseTerminal {
				if d.Selected == 0 {
					return true, false, false // Terminate & close
				}
				return false, true, false
			} else {
				if d.Selected == 0 {
					return true, false, false // Confirm delete
				}
				return false, true, false // Cancel
			}
		}
	}

	return false, false, false
}

// HandleMouse handles mouse clicks on confirm dialog buttons.
func (d *ConfirmDialog) HandleMouse(relX, relY int) (confirmed bool, canceled bool, discard bool, handled bool) {
	if !d.Active {
		return false, false, false, false
	}

	rendered := d.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := d.Width

	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		return false, false, false, true
	}

	buttonY := boxH - 3
	if relY == buttonY || relY == buttonY+1 {
		d.Close()
		if d.Type == ConfirmAlert {
			return true, false, false, true
		}
		if d.Type == ConfirmUnsavedChanges {
			if relX >= 2 && relX <= 12 {
				return true, false, false, true // Save
			} else if relX >= 13 && relX <= 24 {
				return false, false, true, true // Discard
			} else {
				return false, true, false, true // Cancel
			}
		} else if d.Type == ConfirmExternalChange {
			if relX >= 2 && relX <= 14 {
				return true, false, false, true // Reload
			} else if relX >= 15 && relX <= 33 {
				return false, false, true, true // Merge (Diff)
			} else {
				return false, true, false, true // Cancel / Keep local
			}
		} else if d.Type == ConfirmDiscardGitHunk {
			if relX >= 2 && relX <= 20 {
				return true, false, false, true // Discard hunk
			} else {
				return false, true, false, true // Cancel
			}
		} else if d.Type == ConfirmCloseTerminal {
			if relX >= 2 && relX <= 18 {
				return true, false, false, true // Terminate
			} else {
				return false, true, false, true // Cancel
			}
		} else {
			if relX >= 2 && relX <= 16 {
				return true, false, false, true // Delete
			} else {
				return false, true, false, true // Cancel
			}
		}
	}

	return false, false, false, true
}

func (d *ConfirmDialog) Render() string {
	if !d.Active {
		return ""
	}

	titleColor := ui.CurrentTheme.Red
	if d.Type == ConfirmUnsavedChanges || d.Type == ConfirmExternalChange || d.Type == ConfirmAlert {
		titleColor = ui.CurrentTheme.Yellow
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		Background(ui.CurrentTheme.BgTopMenu).
		MarginBottom(1)

	msgStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu).
		MarginBottom(1)

	renderedTitle := titleStyle.Render(d.Title)
	renderedMsg := msgStyle.Render(d.Message)

	btnStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	btnSep := lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Render(" ")

	var buttons string
	if d.Type == ConfirmAlert {
		btnOK := btnStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		buttons = lipgloss.JoinHorizontal(lipgloss.Center, btnOK.Render("OK"))
	} else if d.Type == ConfirmDiscardGitHunk {
		btnDiscard := btnStyle
		btnCancel := btnStyle
		if d.Selected == 0 {
			btnDiscard = btnDiscard.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else {
			btnCancel = btnCancel.Background(ui.CurrentTheme.BorderColor).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		}
		buttons = lipgloss.JoinHorizontal(lipgloss.Center, btnDiscard.Render("Discard Hunk"), btnSep, btnCancel.Render("Cancel"))
	} else if d.Type == ConfirmUnsavedChanges {
		btnSave := btnStyle
		btnDiscard := btnStyle
		btnCancel := btnStyle

		if d.Selected == 0 {
			btnSave = btnSave.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 1 {
			btnDiscard = btnDiscard.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 2 {
			btnCancel = btnCancel.Background(ui.CurrentTheme.BorderColor).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		}

		buttons = lipgloss.JoinHorizontal(
			lipgloss.Center,
			btnSave.Render("Save"),
			btnSep,
			btnDiscard.Render("Discard"),
			btnSep,
			btnCancel.Render("Cancel"),
		)
	} else if d.Type == ConfirmExternalChange {
		btnReload := btnStyle
		btnMerge := btnStyle
		btnCancel := btnStyle

		if d.Selected == 0 {
			btnReload = btnReload.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 1 {
			btnMerge = btnMerge.Background(ui.CurrentTheme.Green).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 2 {
			btnCancel = btnCancel.Background(ui.CurrentTheme.BorderColor).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		}

		buttons = lipgloss.JoinHorizontal(
			lipgloss.Center,
			btnReload.Render("Reload"),
			btnSep,
			btnMerge.Render("Merge (Diff)"),
			btnSep,
			btnCancel.Render("Cancel"),
		)
	} else if d.Type == ConfirmCloseTerminal {
		btnTerminate := btnStyle
		btnCancel := btnStyle
		if d.Selected == 0 {
			btnTerminate = btnTerminate.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 1 {
			btnCancel = btnCancel.Background(ui.CurrentTheme.BorderColor).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		}

		buttons = lipgloss.JoinHorizontal(
			lipgloss.Center,
			btnTerminate.Render("Terminate"),
			btnSep,
			btnCancel.Render("Cancel"),
		)
	} else {
		btnConfirm := btnStyle
		btnCancel := btnStyle

		if d.Selected == 0 {
			btnConfirm = btnConfirm.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		} else if d.Selected == 1 {
			btnCancel = btnCancel.Background(ui.CurrentTheme.BorderColor).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
		}

		buttons = lipgloss.JoinHorizontal(
			lipgloss.Center,
			btnConfirm.Render("Delete"),
			btnSep,
			btnCancel.Render("Cancel"),
		)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		renderedMsg,
		buttons,
	)

	boxStyle := lipgloss.NewStyle().
		Width(d.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
