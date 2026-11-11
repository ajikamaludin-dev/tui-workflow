package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type SwitcherItem struct {
	Index   int
	Title   string
	Path    string
	IsDirty bool
}

type BufferSwitcher struct {
	Active     bool
	Items      []SwitcherItem
	Filtered   []SwitcherItem
	Selected   int
	Filter     textinput.Model
	Width      int
}

func NewBufferSwitcher() *BufferSwitcher {
	ti := textinput.New()
	ti.Prompt = "Cari Tab: "
	ti.CharLimit = 64
	ti.Width = 38
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &BufferSwitcher{
		Active:   false,
		Items:    make([]SwitcherItem, 0),
		Filtered: make([]SwitcherItem, 0),
		Selected: 0,
		Filter:   ti,
		Width:    50,
	}
}

func (s *BufferSwitcher) Open(items []SwitcherItem, activeIdx int) {
	s.Active = true
	s.Items = items
	s.Filter.SetValue("")
	s.Filter.Focus()
	s.applyFilter()

	s.Selected = 0
	for i, it := range s.Filtered {
		if it.Index == activeIdx {
			s.Selected = i
			break
		}
	}
}

func (s *BufferSwitcher) Close() {
	s.Active = false
	s.Filter.Blur()
}

func (s *BufferSwitcher) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.Filter.Value()))
	if query == "" {
		s.Filtered = make([]SwitcherItem, len(s.Items))
		copy(s.Filtered, s.Items)
	} else {
		s.Filtered = make([]SwitcherItem, 0)
		for _, it := range s.Items {
			if strings.Contains(strings.ToLower(it.Title), query) || strings.Contains(strings.ToLower(it.Path), query) {
				s.Filtered = append(s.Filtered, it)
			}
		}
	}

	if s.Selected >= len(s.Filtered) {
		s.Selected = len(s.Filtered) - 1
	}
	if s.Selected < 0 {
		s.Selected = 0
	}
}

func (s *BufferSwitcher) Update(msg tea.Msg) (targetIdx int, closed bool, cmd tea.Cmd) {
	if !s.Active {
		return -1, false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.Close()
			return -1, true, nil

		case "up":
			if s.Selected > 0 {
				s.Selected--
			} else if len(s.Filtered) > 0 {
				s.Selected = len(s.Filtered) - 1
			}
			return -1, false, nil

		case "down":
			if s.Selected < len(s.Filtered)-1 {
				s.Selected++
			} else {
				s.Selected = 0
			}
			return -1, false, nil

		case "enter":
			if len(s.Filtered) > 0 && s.Selected >= 0 && s.Selected < len(s.Filtered) {
				idx := s.Filtered[s.Selected].Index
				s.Close()
				return idx, true, nil
			}
			s.Close()
			return -1, true, nil
		}
	}

	var tiCmd tea.Cmd
	s.Filter, tiCmd = s.Filter.Update(msg)
	s.applyFilter()
	return -1, false, tiCmd
}

func (s *BufferSwitcher) Render() string {
	if !s.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("⚡ Beralih Buffer Tab (Ctrl+P)")
	renderedInput := s.Filter.View()

	var listLines []string
	if len(s.Filtered) == 0 {
		listLines = append(listLines, "  (Tidak ada tab yang cocok)")
	} else {
		for i, it := range s.Filtered {
			prefix := "  "
			if i == s.Selected {
				prefix = "❯ "
			}

			title := it.Title
			if it.IsDirty {
				title += "*"
			}

			lineText := prefix + title + "  (" + it.Path + ")"
			if i == s.Selected {
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
		Width(s.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
