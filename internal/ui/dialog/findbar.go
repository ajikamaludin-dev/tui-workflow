package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type FindBarFocus int

const (
	FindFocusInput FindBarFocus = iota
	FindFocusReplace
)

// FindBar represents an in-editor floating search and replace bar.
type FindBar struct {
	Active       bool
	IsReplace    bool
	FocusField   FindBarFocus
	Input        textinput.Model
	ReplaceInput textinput.Model
	MatchCurrent int
	MatchTotal   int
	Width        int
}

// NewFindBar initializes a new FindBar.
func NewFindBar() *FindBar {
	ti := textinput.New()
	ti.Prompt = " Cari: "
	ti.CharLimit = 64
	ti.Width = 24
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	ri := textinput.New()
	ri.Prompt = " Ganti: "
	ri.CharLimit = 64
	ri.Width = 24
	ri.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ri.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &FindBar{
		Active:       false,
		IsReplace:    false,
		FocusField:   FindFocusInput,
		Input:        ti,
		ReplaceInput: ri,
		MatchCurrent: 0,
		MatchTotal:   0,
		Width:        42,
	}
}

// Open activates find bar in find-only mode.
func (f *FindBar) Open(initialVal string) {
	f.Active = true
	f.IsReplace = false
	f.FocusField = FindFocusInput
	f.Input.SetValue(initialVal)
	f.Input.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	f.Input.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	f.ReplaceInput.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	f.ReplaceInput.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	f.Input.CursorEnd()
	f.Input.Focus()
	f.ReplaceInput.Blur()
}

// OpenReplace activates find bar in find & replace mode.
func (f *FindBar) OpenReplace(initialVal string) {
	f.Active = true
	f.IsReplace = true
	f.FocusField = FindFocusInput
	f.Input.SetValue(initialVal)
	f.Input.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	f.Input.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	f.ReplaceInput.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	f.ReplaceInput.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	f.Input.CursorEnd()
	f.Input.Focus()
	f.ReplaceInput.Blur()
}

// Close deactivates find bar.
func (f *FindBar) Close() {
	f.Active = false
	f.Input.Blur()
	f.ReplaceInput.Blur()
}

// SetMatches updates match counter.
func (f *FindBar) SetMatches(current, total int) {
	f.MatchCurrent = current
	f.MatchTotal = total
}

// Update handles keyboard navigation within find bar.
func (f *FindBar) Update(msg tea.Msg) (action string, closed bool, cmd tea.Cmd) {
	if !f.Active {
		return "", false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			f.Close()
			return "close", true, nil
		case "tab":
			if f.IsReplace {
				if f.FocusField == FindFocusInput {
					f.FocusField = FindFocusReplace
					f.Input.Blur()
					f.ReplaceInput.Focus()
				} else {
					f.FocusField = FindFocusInput
					f.ReplaceInput.Blur()
					f.Input.Focus()
				}
				return "", false, nil
			}
		case "shift+tab":
			if f.IsReplace {
				if f.FocusField == FindFocusReplace {
					f.FocusField = FindFocusInput
					f.ReplaceInput.Blur()
					f.Input.Focus()
				} else {
					f.FocusField = FindFocusReplace
					f.Input.Blur()
					f.ReplaceInput.Focus()
				}
				return "", false, nil
			}
		case "ctrl+h":
			f.IsReplace = !f.IsReplace
			if f.IsReplace {
				f.FocusField = FindFocusReplace
				f.Input.Blur()
				f.ReplaceInput.Focus()
			} else {
				f.FocusField = FindFocusInput
				f.ReplaceInput.Blur()
				f.Input.Focus()
			}
			return "", false, nil
		case "alt+r", "ctrl+r":
			if f.IsReplace {
				return "replace_one", false, nil
			}
		case "alt+enter", "ctrl+alt+enter":
			if f.IsReplace {
				return "replace_all", false, nil
			}
		case "enter":
			if f.IsReplace && f.FocusField == FindFocusReplace {
				return "replace_one", false, nil
			}
			return "next", false, nil
		case "down", "f3":
			return "next", false, nil
		case "shift+enter", "up", "shift+f3":
			return "prev", false, nil
		}
	}

	var tiCmd tea.Cmd
	if f.FocusField == FindFocusReplace {
		f.ReplaceInput, tiCmd = f.ReplaceInput.Update(msg)
	} else {
		oldVal := f.Input.Value()
		f.Input, tiCmd = f.Input.Update(msg)
		newVal := f.Input.Value()
		if oldVal != newVal {
			return "change", false, tiCmd
		}
	}
	return "", false, tiCmd
}

// Render renders the floating find bar box.
func (f *FindBar) Render() string {
	if !f.Active {
		return ""
	}

	var matchStr string
	if f.MatchTotal > 0 {
		matchStr = fmt.Sprintf(" [%d/%d] ", f.MatchCurrent, f.MatchTotal)
	} else if strings.TrimSpace(f.Input.Value()) != "" {
		matchStr = " [0/0] "
	} else {
		matchStr = " "
	}

	matchStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.FgSecondary)

	navHintStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.FgSecondary)

	navHints := " [Enter:↓ Shift+Enter:↑ Esc:×] "
	if f.IsReplace {
		navHints = " [Tab:Fokus Enter:↓ Esc:×] "
	}

	row1 := lipgloss.JoinHorizontal(
		lipgloss.Center,
		f.Input.View(),
		matchStyle.Render(matchStr),
		navHintStyle.Render(navHints),
	)

	content := row1
	if f.IsReplace {
		repHints := " [Enter:Ganti Alt+Enter:Semua] "
		row2 := lipgloss.JoinHorizontal(
			lipgloss.Center,
			f.ReplaceInput.View(),
			navHintStyle.Render(repHints),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
