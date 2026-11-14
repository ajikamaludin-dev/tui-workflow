package dialog

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tui-workflow/internal/ui"
)

type InputDialogType int

const (
	InputNewFile InputDialogType = iota
	InputNewFolder
	InputRename
	InputOpenFile
	InputOpenFolder
	InputGotoLine
	InputFilterExplorer
)

type InputDialog struct {
	Active     bool
	Type       InputDialogType
	Title      string
	TargetPath string // target folder or existing file path
	TextInput  textinput.Model
	FocusedBtn int // 0: Input, 1: OK, 2: Cancel
	Width      int
}

func NewInputDialog() *InputDialog {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 128
	ti.SetWidth(36)
	applyTextInputStyles(&ti, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))

	return &InputDialog{
		Active:    false,
		TextInput: ti,
		Width:     46,
	}
}

func (d *InputDialog) Open(t InputDialogType, title, targetPath, initialVal string) {
	d.Active = true
	d.Type = t
	d.Title = title
	d.TargetPath = targetPath
	d.TextInput.SetValue(initialVal)
	applyTextInputStyles(&d.TextInput, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
	d.TextInput.CursorEnd()
	d.TextInput.Focus()
	d.FocusedBtn = 0
}

func (d *InputDialog) Close() {
	d.Active = false
	d.TextInput.Blur()
}

func (d *InputDialog) Value() string {
	return strings.TrimSpace(d.TextInput.Value())
}

func (d *InputDialog) Update(msg tea.Msg) (confirmed bool, canceled bool, cmd tea.Cmd) {
	if !d.Active {
		return false, false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch ui.KeyName(msg) {
		case "esc":
			d.Close()
			return false, true, nil

		case "tab":
			d.FocusedBtn = (d.FocusedBtn + 1) % 3
			if d.FocusedBtn == 0 {
				d.TextInput.Focus()
			} else {
				d.TextInput.Blur()
			}
			return false, false, nil

		case "enter":
			if d.FocusedBtn == 2 { // Cancel
				d.Close()
				return false, true, nil
			}
			val := d.Value()
			if val != "" || d.Type == InputFilterExplorer {
				d.Close()
				return true, false, nil
			}
			return false, false, nil
		}
	}

	if d.FocusedBtn == 0 {
		var textCmd tea.Cmd
		d.TextInput, textCmd = d.TextInput.Update(msg)
		return false, false, textCmd
	}

	return false, false, nil
}

// HandleMouse handles mouse clicks on input dialog buttons.
func (d *InputDialog) HandleMouse(relX, relY int) (confirmed bool, canceled bool, handled bool) {
	if !d.Active {
		return false, false, false
	}

	rendered := d.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := d.Width

	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		return false, false, true
	}

	// Click on input row (Line 4)
	if relY == 4 {
		d.FocusedBtn = 0
		d.TextInput.Focus()
		return false, false, true
	}

	// Click on buttons row (boxH - 3)
	buttonY := boxH - 3
	if relY == buttonY {
		// OK Button: col 3..18
		if relX >= 3 && relX <= 18 {
			val := d.Value()
			if val != "" || d.Type == InputFilterExplorer {
				d.Close()
				return true, false, true
			}
			return false, false, true
		}
		// Cancel Button: col 19..36
		if relX >= 19 && relX <= 36 {
			d.Close()
			return false, true, true
		}
	}

	return false, false, true
}

func (d *InputDialog) Render() string {
	if !d.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.BgTopMenu).
		MarginBottom(1)

	renderedTitle := titleStyle.Render(d.Title)
	renderedInput := d.TextInput.View()

	btnOKStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	btnCancelStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	if d.FocusedBtn == 1 {
		btnOKStyle = btnOKStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
	} else if d.FocusedBtn == 2 {
		btnCancelStyle = btnCancelStyle.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
	}

	btnSep := lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Render(" ")
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, btnOKStyle.Render("OK (Enter)"), btnSep, btnCancelStyle.Render("Cancel (Esc)"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		renderedInput,
		"\n",
		buttons,
	)

	boxStyle := lipgloss.NewStyle().
		Width(d.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
