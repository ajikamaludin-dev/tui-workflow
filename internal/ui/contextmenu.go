package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type ContextMenuItem struct {
	ID       string
	Label    string
	Shortcut string
	Disabled bool
}

type ContextMenu struct {
	Active      bool
	X           int
	Y           int
	SelectedIdx int
	Items       []ContextMenuItem
	Width       int
}

func NewContextMenu() *ContextMenu {
	return &ContextMenu{
		Active: false,
		Width:  24,
	}
}

func (m *ContextMenu) Open(x, y int, hasSelection bool, hasClipboard bool, screenW, screenH int) {
	items := []ContextMenuItem{
		{ID: "copy", Label: "Salin", Shortcut: "Ctrl+C", Disabled: !hasSelection},
		{ID: "paste", Label: "Tempel", Shortcut: "Ctrl+V", Disabled: !hasClipboard},
		{ID: "cut", Label: "Potong", Shortcut: "Ctrl+X", Disabled: !hasSelection},
		{ID: "select_all", Label: "Pilih Semua", Shortcut: "Ctrl+A", Disabled: false},
	}
	m.OpenCustom(x, y, items, screenW, screenH)
	if !hasSelection && hasClipboard {
		m.SelectedIdx = 1 // Default to Paste if no selection but clipboard exists
	}
}

func (m *ContextMenu) OpenCustom(x, y int, items []ContextMenuItem, screenW, screenH int) {
	m.Active = true
	m.SelectedIdx = 0
	m.Items = items

	maxW := 24
	for _, it := range items {
		w := ansi.StringWidth(it.Label) + ansi.StringWidth(it.Shortcut) + 6
		if w > maxW {
			maxW = w
		}
	}
	m.Width = maxW

	// Clamp X and Y within screen boundary
	if screenW > 0 && x+m.Width > screenW {
		x = screenW - m.Width
	}
	if x < 0 {
		x = 0
	}
	totalH := len(m.Items) + 2
	if screenH > 0 && y+totalH > screenH-1 {
		y = screenH - 1 - totalH
	}
	if y < 1 {
		y = 1
	}
	m.X = x
	m.Y = y
}
func (m *ContextMenu) Close() {
	m.Active = false
}

func (m *ContextMenu) Next() {
	if len(m.Items) == 0 {
		return
	}
	for i := 0; i < len(m.Items); i++ {
		m.SelectedIdx = (m.SelectedIdx + 1) % len(m.Items)
		if !m.Items[m.SelectedIdx].Disabled {
			break
		}
	}
}

func (m *ContextMenu) Prev() {
	if len(m.Items) == 0 {
		return
	}
	for i := 0; i < len(m.Items); i++ {
		m.SelectedIdx--
		if m.SelectedIdx < 0 {
			m.SelectedIdx = len(m.Items) - 1
		}
		if !m.Items[m.SelectedIdx].Disabled {
			break
		}
	}
}

func (m *ContextMenu) SelectedItem() *ContextMenuItem {
	if !m.Active || m.SelectedIdx < 0 || m.SelectedIdx >= len(m.Items) {
		return nil
	}
	return &m.Items[m.SelectedIdx]
}

// HandleMouse handles mouse events on context menu.
func (m *ContextMenu) HandleMouse(msg tea.MouseMsg) (action string, handled bool) {
	if !m.Active {
		return "", false
	}

	relX := msg.X - m.X
	relY := msg.Y - m.Y
	totalH := len(m.Items) + 2
	isInside := relX >= 0 && relX < m.Width && relY >= 0 && relY < totalH
	isOnItem := relX >= 1 && relX < m.Width-1 && relY >= 1 && relY <= len(m.Items)

	switch msg.Action {
	case tea.MouseActionMotion:
		if isOnItem {
			idx := relY - 1
			if idx >= 0 && idx < len(m.Items) && !m.Items[idx].Disabled {
				m.SelectedIdx = idx
			}
			return "", true
		}
		if isInside {
			return "", true
		}
		// Mouse motion outside does NOT close menu
		return "", false

	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			if isOnItem {
				idx := relY - 1
				item := m.Items[idx]
				m.Close()
				if !item.Disabled {
					return item.ID, true
				}
				return "", true
			}
			// Left click outside or on border closes menu
			m.Close()
			return "", true
		}
		if msg.Button == tea.MouseButtonRight {
			// Right click re-handled by caller to open at new position
			m.Close()
			return "", false
		}
		return "", true

	case tea.MouseActionRelease:
		// Consume release event if inside to prevent underlying controls from receiving it
		if isInside {
			return "", true
		}
		return "", false
	}

	return "", false
}

func (m *ContextMenu) Render() string {
	if !m.Active {
		return ""
	}

	innerW := m.Width - 2
	var renderedItems []string

	for i, item := range m.Items {
		isSelected := i == m.SelectedIdx && !item.Disabled
		leftPart := " " + item.Label
		rightPart := item.Shortcut + " "
		spaceLen := innerW - ansi.StringWidth(leftPart) - ansi.StringWidth(rightPart)
		if spaceLen < 1 {
			spaceLen = 1
		}
		rowText := leftPart + strings.Repeat(" ", spaceLen) + rightPart
		if ansi.StringWidth(rowText) > innerW {
			rowText = ansi.Truncate(rowText, innerW, "")
		} else if ansi.StringWidth(rowText) < innerW {
			rowText += strings.Repeat(" ", innerW-ansi.StringWidth(rowText))
		}

		var rowStyle lipgloss.Style
		if isSelected {
			rowStyle = lipgloss.NewStyle().
				Background(CurrentTheme.Accent).
				Foreground(CurrentTheme.FgHighlight).
				Bold(true)
		} else if item.Disabled {
			rowStyle = lipgloss.NewStyle().
				Background(CurrentTheme.BgTopMenu).
				Foreground(CurrentTheme.FgSecondary)
		} else {
			rowStyle = lipgloss.NewStyle().
				Background(CurrentTheme.BgTopMenu).
				Foreground(CurrentTheme.FgPrimary)
		}

		renderedItems = append(renderedItems, rowStyle.Render(rowText))
	}

	content := strings.Join(renderedItems, "\n")
	boxStyle := lipgloss.NewStyle().
		Width(m.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CurrentTheme.BorderColor).
		Background(CurrentTheme.BgTopMenu)

	// Normalize dialog background
	sample := lipgloss.NewStyle().Background(CurrentTheme.BgTopMenu).Render("X")
	idx := strings.Index(sample, "X")
	var bgSeq string
	if idx != -1 {
		bgSeq = sample[:idx]
	}

	rendered := boxStyle.Render(content)
	lines := strings.Split(rendered, "\n")
	for i := 1; i < len(lines)-1; i++ {
		line := lines[i]
		lastReset := strings.LastIndex(line, "\x1b[0m")
		if lastReset != -1 {
			prefix := line[:lastReset]
			suffix := line[lastReset:]
			lines[i] = strings.ReplaceAll(prefix, "\x1b[0m", "\x1b[0m"+bgSeq) + suffix
		}
	}

	return strings.Join(lines, "\n")
}
