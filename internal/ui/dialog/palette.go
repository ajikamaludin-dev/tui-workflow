package dialog

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// CommandPaletteItem represents an executable action in the command palette.
type CommandPaletteItem struct {
	ID       string
	Title    string
	Category string
	Shortcut string
	Score    int
}

// CommandPalette provides a VS Code style Command Palette (Ctrl+Shift+P / F1).
type CommandPalette struct {
	Active    bool
	Items     []CommandPaletteItem
	Filtered  []CommandPaletteItem
	Selected  int
	ScrollTop int
	Filter    textinput.Model
	Width     int
	MaxRows   int
}

// NewCommandPalette creates a new CommandPalette instance.
func NewCommandPalette() *CommandPalette {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 64
	ti.Width = 48
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &CommandPalette{
		Active:    false,
		Items:     make([]CommandPaletteItem, 0),
		Filtered:  make([]CommandPaletteItem, 0),
		Selected:  0,
		ScrollTop: 0,
		Filter:    ti,
		Width:     64,
		MaxRows:   10,
	}
}

// Open opens the palette with available commands.
func (cp *CommandPalette) Open(items []CommandPaletteItem) {
	cp.Active = true
	cp.Items = items
	cp.Filter.SetValue("")
	cp.Filter.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	cp.Filter.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	cp.Filter.Focus()
	cp.applyFilter()
	cp.Selected = 0
	cp.ScrollTop = 0
}

// Close closes the palette.
func (cp *CommandPalette) Close() {
	cp.Active = false
	cp.Filter.Blur()
}

// SetWidth updates modal width and inner filter width.
func (cp *CommandPalette) SetWidth(w int) {
	if w < 40 {
		w = 40
	}
	cp.Width = w
	cp.Filter.Width = w - 8
}

// ApplyFilter updates filtered commands based on current input.
func (cp *CommandPalette) ApplyFilter() {
	cp.applyFilter()
}

func (cp *CommandPalette) applyFilter() {
	query := strings.TrimSpace(cp.Filter.Value())
	if query == "" {
		cp.Filtered = make([]CommandPaletteItem, len(cp.Items))
		copy(cp.Filtered, cp.Items)
	} else {
		cp.Filtered = make([]CommandPaletteItem, 0)
		for _, it := range cp.Items {
			matchedTitle, scoreTitle := fuzzyMatch(query, it.Title)
			matchedCat, scoreCat := fuzzyMatch(query, it.Category)
			if matchedTitle || matchedCat {
				bestScore := scoreTitle
				if scoreCat > bestScore {
					bestScore = scoreCat
				}
				it.Score = bestScore
				cp.Filtered = append(cp.Filtered, it)
			}
		}

		sort.Slice(cp.Filtered, func(i, j int) bool {
			if cp.Filtered[i].Score != cp.Filtered[j].Score {
				return cp.Filtered[i].Score > cp.Filtered[j].Score
			}
			return cp.Filtered[i].Title < cp.Filtered[j].Title
		})
	}

	if cp.Selected >= len(cp.Filtered) {
		cp.Selected = len(cp.Filtered) - 1
		if cp.Selected < 0 {
			cp.Selected = 0
		}
	}
	cp.adjustScroll()
}

func (cp *CommandPalette) adjustScroll() {
	if cp.Selected < cp.ScrollTop {
		cp.ScrollTop = cp.Selected
	}
	if cp.Selected >= cp.ScrollTop+cp.MaxRows {
		cp.ScrollTop = cp.Selected - cp.MaxRows + 1
	}
	if cp.ScrollTop < 0 {
		cp.ScrollTop = 0
	}
}

// Update handles keyboard navigation in the Command Palette.
func (cp *CommandPalette) Update(msg tea.Msg) (actionID string, closed bool, cmd tea.Cmd) {
	if !cp.Active {
		return "", false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			cp.Close()
			return "", true, nil
		case "up":
			if cp.Selected > 0 {
				cp.Selected--
			} else if len(cp.Filtered) > 0 {
				cp.Selected = len(cp.Filtered) - 1
			}
			cp.adjustScroll()
			return "", false, nil
		case "down":
			if cp.Selected < len(cp.Filtered)-1 {
				cp.Selected++
			} else {
				cp.Selected = 0
			}
			cp.adjustScroll()
			return "", false, nil
		case "enter":
			if len(cp.Filtered) > 0 && cp.Selected >= 0 && cp.Selected < len(cp.Filtered) {
				action := cp.Filtered[cp.Selected].ID
				cp.Close()
				return action, true, nil
			}
			cp.Close()
			return "", true, nil
		}
	}

	oldVal := cp.Filter.Value()
	var tiCmd tea.Cmd
	cp.Filter, tiCmd = cp.Filter.Update(msg)
	if cp.Filter.Value() != oldVal {
		cp.applyFilter()
	}

	return "", false, tiCmd
}

// Render renders the Command Palette box.
func (cp *CommandPalette) Render() string {
	if !cp.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("⌘ Command Palette (Ctrl+Shift+P / F1)")
	renderedInput := cp.Filter.View()

	var listLines []string
	if len(cp.Filtered) == 0 {
		listLines = append(listLines, "  (Tidak ada perintah yang cocok)")
	} else {
		endIdx := cp.ScrollTop + cp.MaxRows
		if endIdx > len(cp.Filtered) {
			endIdx = len(cp.Filtered)
		}

		for i := cp.ScrollTop; i < endIdx; i++ {
			it := cp.Filtered[i]
			prefix := "  "
			if i == cp.Selected {
				prefix = "❯ "
			}

			catBadge := ""
			if it.Category != "" {
				catBadge = it.Category + ": "
			}

			shortcutStr := ""
			if it.Shortcut != "" {
				shortcutStr = "  [" + it.Shortcut + "]"
			}

			lineText := prefix + catBadge + it.Title + shortcutStr
			usableW := cp.Width - 8
			if usableW > 20 && ansi.StringWidth(lineText) > usableW {
				lineText = ansi.Truncate(lineText, usableW, "...")
			}
			if i == cp.Selected {
				lineText = ui.CurrentTheme.TopMenuActiveItem.Render(lineText)
			} else {
				lineText = ui.CurrentTheme.DropdownItemStyle.Render(lineText)
			}
			listLines = append(listLines, lineText)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		renderedInput,
		"\n",
		strings.Join(listLines, "\n"),
	)

	boxStyle := lipgloss.NewStyle().
		Width(cp.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
