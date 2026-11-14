package dialog

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// BranchSwitcher provides a modal dialog for switching git branches.
type BranchSwitcher struct {
	Active        bool
	Branches      []string
	Filtered      []string
	CurrentBranch string
	Selected      int
	ScrollTop     int
	Filter        textinput.Model
	Width         int
	MaxRows       int
}

// NewBranchSwitcher creates a new BranchSwitcher instance.
func NewBranchSwitcher() *BranchSwitcher {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 64
	ti.SetWidth(36)
	applyTextInputStyles(&ti, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))

	return &BranchSwitcher{
		Active:    false,
		Branches:  make([]string, 0),
		Filtered:  make([]string, 0),
		Selected:  0,
		ScrollTop: 0,
		Filter:    ti,
		Width:     48,
		MaxRows:   8,
	}
}

// Open activates the branch switcher dialog.
func (b *BranchSwitcher) Open(branches []string, currentBranch string) {
	b.Active = true
	b.Branches = branches
	b.CurrentBranch = currentBranch
	b.Selected = 0
	b.ScrollTop = 0
	b.Filter.SetValue("")
	applyTextInputStyles(&b.Filter, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
	b.Filter.CursorEnd()
	b.Filter.Focus()
	b.applyFilter()

	// Locate current branch in filtered list
	for i, br := range b.Filtered {
		if br == currentBranch {
			b.Selected = i
			break
		}
	}
}

// Close deactivates the branch switcher dialog.
func (b *BranchSwitcher) Close() {
	b.Active = false
	b.Filter.Blur()
}

func (b *BranchSwitcher) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(b.Filter.Value()))
	if q == "" {
		b.Filtered = make([]string, len(b.Branches))
		copy(b.Filtered, b.Branches)
	} else {
		b.Filtered = make([]string, 0)
		for _, br := range b.Branches {
			if strings.Contains(strings.ToLower(br), q) {
				b.Filtered = append(b.Filtered, br)
			}
		}
	}
	if b.Selected >= len(b.Filtered) {
		b.Selected = len(b.Filtered) - 1
	}
	if b.Selected < 0 {
		b.Selected = 0
	}
}

// Update handles keyboard events for BranchSwitcher.
func (b *BranchSwitcher) Update(msg tea.Msg) (selectedBranch string, closed bool, cmd tea.Cmd) {
	if !b.Active {
		return "", false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch ui.KeyName(msg) {
		case "esc":
			b.Close()
			return "", true, nil
		case "up", "ctrl+p":
			if b.Selected > 0 {
				b.Selected--
				if b.Selected < b.ScrollTop {
					b.ScrollTop = b.Selected
				}
			}
			return "", false, nil
		case "down", "ctrl+n":
			if b.Selected < len(b.Filtered)-1 {
				b.Selected++
				if b.Selected >= b.ScrollTop+b.MaxRows {
					b.ScrollTop = b.Selected - b.MaxRows + 1
				}
			}
			return "", false, nil
		case "enter":
			if len(b.Filtered) > 0 && b.Selected >= 0 && b.Selected < len(b.Filtered) {
				chosen := b.Filtered[b.Selected]
				b.Close()
				return chosen, true, nil
			}
			b.Close()
			return "", true, nil
		}
	}

	oldVal := b.Filter.Value()
	var tiCmd tea.Cmd
	b.Filter, tiCmd = b.Filter.Update(msg)
	if b.Filter.Value() != oldVal {
		b.applyFilter()
	}
	return "", false, tiCmd
}

// HandleMouse processes mouse clicks and scrolling inside or outside Branch Switcher.
func (b *BranchSwitcher) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (targetBranch string, closed bool, handled bool) {
	if !b.Active {
		return "", false, false
	}

	rendered := b.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := b.Width

	// Outside click: dismiss dialog
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		b.Close()
		return "", true, true
	}

	if isWheelUp {
		if b.Selected > 0 {
			b.Selected--
			if b.Selected < b.ScrollTop {
				b.ScrollTop = b.Selected
			}
		} else if b.ScrollTop > 0 {
			b.ScrollTop--
		}
		return "", false, true
	}

	if isWheelDown {
		if b.Selected < len(b.Filtered)-1 {
			b.Selected++
			if b.Selected >= b.ScrollTop+b.MaxRows {
				b.ScrollTop = b.Selected - b.MaxRows + 1
			}
		}
		return "", false, true
	}

	// Layout mapping:
	// Border top: relY = 0
	// Header: relY = 1
	// SearchBox: relY = 2
	// Divider ─: relY = 3
	// Rows list: relY = 4 ...
	if relY == 2 {
		b.Filter.Focus()
		return "", false, true
	}

	listStartY := 4
	if relY >= listStartY && relY < listStartY+b.MaxRows {
		clickedIdx := b.ScrollTop + (relY - listStartY)
		if clickedIdx >= 0 && clickedIdx < len(b.Filtered) {
			b.Selected = clickedIdx
			branch := b.Filtered[clickedIdx]
			b.Close()
			return branch, true, true
		}
	}

	return "", false, true
}

// Render renders the branch switcher modal.
func (b *BranchSwitcher) Render() string {
	if !b.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)

	header := titleStyle.Render("🌿 Switch Git Branch (Alt+B)")
	searchBox := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1).
		Render(b.Filter.View())

	var rows []string
	if len(b.Filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(ui.CurrentTheme.FgSecondary).
			Background(ui.CurrentTheme.BgTopMenu).
			Padding(0, 2)
		rows = append(rows, emptyStyle.Render("(No matching branches)"))
	} else {
		endIdx := b.ScrollTop + b.MaxRows
		if endIdx > len(b.Filtered) {
			endIdx = len(b.Filtered)
		}

		for i := b.ScrollTop; i < endIdx; i++ {
			branch := b.Filtered[i]
			isCurrent := branch == b.CurrentBranch
			isSelected := i == b.Selected

			prefix := "  "
			if isCurrent {
				prefix = "✓ "
			}

			label := prefix + branch
			availWidth := b.Width - 4
			if availWidth < 10 {
				availWidth = 10
			}
			label = ansi.Truncate(label, availWidth, "…")

			rowStyle := lipgloss.NewStyle().
				Background(ui.CurrentTheme.BgTopMenu).
				Foreground(ui.CurrentTheme.FgPrimary)

			if isSelected {
				rowStyle = rowStyle.
					Background(ui.CurrentTheme.Accent).
					Foreground(ui.CurrentTheme.FgHighlight).
					Bold(true)
			} else if isCurrent {
				rowStyle = rowStyle.Foreground(ui.CurrentTheme.Accent)
			}

			rows = append(rows, rowStyle.Render(" "+label+" "))
		}
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)
	hint := hintStyle.Render(" [↑/↓:Select Enter:Switch Esc:Cancel] ")

	dividerStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.BorderColor).
		Background(ui.CurrentTheme.BgTopMenu)
	divider := dividerStyle.Render(strings.Repeat("─", b.Width))

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		searchBox,
		divider,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		divider,
		hint,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)

	return NormalizeDialogBackground(boxStyle.Render(body), ui.CurrentTheme.BgTopMenu)
}
