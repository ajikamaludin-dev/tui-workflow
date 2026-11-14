package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

type FindBarFocus int

const (
	FindFocusInput FindBarFocus = iota
	FindFocusReplace
)

// FindBar represents an in-editor floating search and replace bar.
type FindBar struct {
	Active         bool
	IsReplace      bool
	FocusField     FindBarFocus
	Input          textinput.Model
	ReplaceInput   textinput.Model
	MatchCurrent   int
	MatchTotal     int
	Width          int
	QueryCase      bool
	QueryWholeWord bool
	QueryRegex     bool
}

// NewFindBar initializes a new FindBar.
func NewFindBar() *FindBar {
	ti := textinput.New()
	ti.Prompt = " Find: "
	ti.CharLimit = 64
	ti.SetWidth(24)
	applyTextInputStyles(&ti, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))

	ri := textinput.New()
	ri.Prompt = " Replace: "
	ri.CharLimit = 64
	ri.SetWidth(22)
	applyTextInputStyles(&ri, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))

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
	applyTextInputStyles(&f.Input, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
	applyTextInputStyles(&f.ReplaceInput, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
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
	applyTextInputStyles(&f.Input, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
	applyTextInputStyles(&f.ReplaceInput, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
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
	case tea.KeyPressMsg:
		switch ui.KeyName(msg) {
		case "esc":
			f.Close()
			return "close", true, nil
		case "alt+c":
			f.QueryCase = !f.QueryCase
			return "change", false, nil
		case "alt+w":
			f.QueryWholeWord = !f.QueryWholeWord
			return "change", false, nil
		case "alt+x":
			f.QueryRegex = !f.QueryRegex
			return "change", false, nil
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

// HandleMouse processes mouse clicks inside the floating FindBar.
func (f *FindBar) HandleMouse(relX, relY int) (action string, closed bool, handled bool) {
	if !f.Active {
		return "", false, false
	}

	rendered := f.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := 0
	for _, l := range lines {
		w := ansi.StringWidth(l)
		if w > boxW {
			boxW = w
		}
	}

	// Outside click: dismiss find bar
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		f.Close()
		return "close", true, true
	}

	// Row 1 (relY == 1 inside border): Input field, match info, buttons [↓], [↑], [×]
	if relY == 1 {
		// Input field: col 1..26
		if relX >= 1 && relX <= 26 {
			f.FocusField = FindFocusInput
			f.ReplaceInput.Blur()
			f.Input.Focus()
			return "", false, true
		}
		// Tombol navigasi di sebelah kanan input & match
		// [↓] button: around col 35..38
		if relX >= 34 && relX <= 38 {
			return "next", false, true
		}
		// [↑] button: around col 39..43
		if relX >= 39 && relX <= 43 {
			return "prev", false, true
		}
		// [×] close button: around col 44..48
		if relX >= 44 {
			f.Close()
			return "close", true, true
		}
		return "", false, true
	}

	// Row 2 (relY == 2 inside border, when replace mode is active)
	if f.IsReplace && relY == 2 {
		// Replace input field: col 1..26
		if relX >= 1 && relX <= 26 {
			f.FocusField = FindFocusReplace
			f.Input.Blur()
			f.ReplaceInput.Focus()
			return "", false, true
		}
		// [Ganti] button: around col 28..37
		if relX >= 27 && relX <= 37 {
			return "replace_one", false, true
		}
		// [Semua] button: around col 38..48
		if relX >= 38 {
			return "replace_all", false, true
		}
		return "", false, true
	}

	return "", false, true
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

	btnStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.Accent).
		Bold(true)

	badgeOn := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.Accent).
		Bold(true)
	badgeOff := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.FgSecondary)

	closeBtnStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Foreground(ui.CurrentTheme.Red).
		Bold(true)

	// Interactive buttons in row 1: [↓] [↑] [×]
	btnNext := btnStyle.Render(" [↓] ")
	btnPrev := btnStyle.Render("[↑] ")
	btnClose := closeBtnStyle.Render("[×] ")

	renderBadge := func(label string, on bool) string {
		if on {
			return badgeOn.Render(label)
		}
		return badgeOff.Render(label)
	}
	badgeCase := renderBadge(" [Aa]", f.QueryCase)
	badgeWord := renderBadge(" [ab|]", f.QueryWholeWord)
	badgeRegex := renderBadge(" [.*] ", f.QueryRegex)

	row1 := lipgloss.JoinHorizontal(
		lipgloss.Center,
		f.Input.View(),
		badgeCase,
		badgeWord,
		badgeRegex,
		matchStyle.Render(matchStr),
		btnNext,
		btnPrev,
		btnClose,
	)

	content := row1
	if f.IsReplace {
		btnRepOne := btnStyle.Render(" [Replace] ")
		btnRepAll := btnStyle.Render("[All] ")

		row2 := lipgloss.JoinHorizontal(
			lipgloss.Center,
			f.ReplaceInput.View(),
			btnRepOne,
			btnRepAll,
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
