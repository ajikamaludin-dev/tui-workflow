package dialog

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type ConfirmDialogType int

const (
	ConfirmDelete ConfirmDialogType = iota
	ConfirmUnsavedChanges
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
	d.Selected = 1 // default to Cancel
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

		case "left", "h":
			d.Selected--
			if d.Selected < 0 {
				if d.Type == ConfirmUnsavedChanges {
					d.Selected = 2
				} else {
					d.Selected = 1
				}
			}
			return false, false, false

		case "right", "l", "tab":
			maxIdx := 1
			if d.Type == ConfirmUnsavedChanges {
				maxIdx = 2
			}
			d.Selected = (d.Selected + 1) % (maxIdx + 1)
			return false, false, false

		case "enter":
			d.Close()
			if d.Type == ConfirmUnsavedChanges {
				if d.Selected == 0 {
					return true, false, false // Save & close
				} else if d.Selected == 1 {
					return false, false, true // Discard & close
				} else {
					return false, true, false // Cancel
				}
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

func (d *ConfirmDialog) Render() string {
	if !d.Active {
		return ""
	}

	titleColor := ui.CurrentTheme.Red
	if d.Type == ConfirmUnsavedChanges {
		titleColor = ui.CurrentTheme.Yellow
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		MarginBottom(1)

	msgStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgPrimary).
		MarginBottom(1)

	renderedTitle := titleStyle.Render(d.Title)
	renderedMsg := msgStyle.Render(d.Message)

	btnStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	var buttons string
	if d.Type == ConfirmUnsavedChanges {
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
			btnDiscard.Render("Discard"),
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

	return boxStyle.Render(content)
}
