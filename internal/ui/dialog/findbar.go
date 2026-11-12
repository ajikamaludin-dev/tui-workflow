package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

// FindBar represents an in-editor floating search bar.
type FindBar struct {
	Active       bool
	Input        textinput.Model
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

	return &FindBar{
		Active:       false,
		Input:        ti,
		MatchCurrent: 0,
		MatchTotal:   0,
		Width:        42,
	}
}

// Open activates the find bar.
func (f *FindBar) Open(initialVal string) {
	f.Active = true
	f.Input.SetValue(initialVal)
	f.Input.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	f.Input.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	f.Input.CursorEnd()
	f.Input.Focus()
}

// Close deactivates the find bar.
func (f *FindBar) Close() {
	f.Active = false
	f.Input.Blur()
}

// SetMatches updates the match counter.
func (f *FindBar) SetMatches(current, total int) {
	f.MatchCurrent = current
	f.MatchTotal = total
}

// Update handles keyboard navigation within the find bar.
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
		case "enter", "down", "f3":
			return "next", false, nil
		case "shift+enter", "up", "shift+f3":
			return "prev", false, nil
		}
	}

	oldVal := f.Input.Value()
	var tiCmd tea.Cmd
	f.Input, tiCmd = f.Input.Update(msg)
	newVal := f.Input.Value()

	if oldVal != newVal {
		return "change", false, tiCmd
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
		matchStr = "       "
	}

	matchStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.FgSecondary)

	navHintStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.FgSecondary)

	navHints := " [Enter:↓ Shift+Enter:↑ Esc:×] "

	rowContent := lipgloss.JoinHorizontal(
		lipgloss.Center,
		f.Input.View(),
		matchStyle.Render(matchStr),
		navHintStyle.Render(navHints),
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)

	return NormalizeDialogBackground(boxStyle.Render(rowContent), ui.CurrentTheme.BgTopMenu)
}
