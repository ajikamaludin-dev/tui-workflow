package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// TabWidthPicker provides a modal dialog for choosing the global tab/indent
// width (1-4 spaces, default 4). The width applies to tab rendering, the
// Tab key, indent/outdent operations, and Indent Rainbow (level, offset,
// and the misaligned-indent error guard).
type TabWidthPicker struct {
	Active   bool
	Selected int
	Options  []int
	Current  int
	Width    int
}

func NewTabWidthPicker() *TabWidthPicker {
	return &TabWidthPicker{
		Options: []int{1, 2, 3, 4},
		Width:   44,
	}
}

// Open activates the picker, highlighting the option matching current.
func (p *TabWidthPicker) Open(current int) {
	p.Active = true
	p.Current = current
	p.Selected = 0
	for i, o := range p.Options {
		if o == current {
			p.Selected = i
			break
		}
	}
}

func (p *TabWidthPicker) Close() {
	p.Active = false
}

func (p *TabWidthPicker) chosen() int {
	return p.Options[p.Selected]
}

// Update handles keyboard events. Returns the chosen width (>= 1) when the
// user confirms, or 0 when cancelled/in progress.
func (p *TabWidthPicker) Update(msg tea.Msg) (chosen int, closed bool, cmd tea.Cmd) {
	if !p.Active {
		return 0, false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch ui.KeyName(msg) {
		case "esc":
			p.Close()
			return 0, true, nil
		case "up", "k", "ctrl+p":
			if p.Selected > 0 {
				p.Selected--
			}
			return 0, false, nil
		case "down", "j", "ctrl+n":
			if p.Selected < len(p.Options)-1 {
				p.Selected++
			}
			return 0, false, nil
		case "1", "2", "3", "4":
			name := ui.KeyName(msg)
			n := int(name[0] - '0')
			for i, o := range p.Options {
				if o == n {
					p.Selected = i
					break
				}
			}
			chosen := p.chosen()
			p.Close()
			return chosen, true, nil
		case "enter":
			chosen := p.chosen()
			p.Close()
			return chosen, true, nil
		}
	}
	return 0, false, nil
}

// HandleMouse processes mouse clicks and wheel scrolling inside or outside the picker.
func (p *TabWidthPicker) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (chosen int, closed bool, handled bool) {
	if !p.Active {
		return 0, false, false
	}

	rendered := p.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := p.Width

	// Outside click: dismiss without applying
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		p.Close()
		return 0, true, true
	}

	if isWheelUp {
		if p.Selected > 0 {
			p.Selected--
		}
		return 0, false, true
	}

	if isWheelDown {
		if p.Selected < len(p.Options)-1 {
			p.Selected++
		}
		return 0, false, true
	}

	// Layout mapping:
	// Border top: relY = 0
	// Header:    relY = 1
	// Divider ─: relY = 2
	// Rows:      relY = 3 ... 3+len(Options)-1
	listStartY := 3
	if relY >= listStartY && relY < listStartY+len(p.Options) {
		idx := relY - listStartY
		p.Selected = idx
		chosen := p.chosen()
		p.Close()
		return chosen, true, true
	}

	return 0, false, true
}

// Render renders the tab width picker modal.
func (p *TabWidthPicker) Render() string {
	if !p.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)
	header := titleStyle.Render("📐 Tab Width (1-4)")

	optionLabels := map[int]string{
		1: "1 Space",
		2: "2 Spaces (Flutter / JS)",
		3: "3 Spaces",
		4: "4 Spaces (Default)",
	}

	var rows []string
	for i, opt := range p.Options {
		isCurrent := opt == p.Current
		isSelected := i == p.Selected

		prefix := "  "
		if isCurrent {
			prefix = "✓ "
		}

		label := prefix + optionLabels[opt]
		availWidth := p.Width - 4
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

	hintStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(0, 1)
	hint := hintStyle.Render(" [↑/↓:Select  1-4:Pick  Enter:Apply  Esc:Cancel] ")

	dividerStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.BorderColor).
		Background(ui.CurrentTheme.BgTopMenu)
	divider := dividerStyle.Render(strings.Repeat("─", p.Width))

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		divider,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		divider,
		hint,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu)

	return NormalizeDialogBackground(boxStyle.Render(body), ui.CurrentTheme.BgTopMenu)
}
