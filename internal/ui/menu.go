package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MenuItem struct {
	Label    string
	Shortcut string
	ActionID string
}

type MenuCategory struct {
	Title    string
	Shortcut string
	Items    []MenuItem
	StartX   int
	EndX     int
}

type MenuBar struct {
	Categories   []MenuCategory
	ActiveMenu   int // -1 if closed
	SelectedItem int
	Width        int
}

func NewMenuBar() *MenuBar {
	mb := &MenuBar{
		ActiveMenu:   -1,
		SelectedItem: 0,
		Categories: []MenuCategory{
			{
				Title:    "File",
				Shortcut: "Alt+F",
				Items: []MenuItem{
					{Label: "New File", Shortcut: "a", ActionID: "file.new"},
					{Label: "Save File", Shortcut: "Ctrl+S", ActionID: "file.save"},
					{Label: "Close Tab", Shortcut: "Ctrl+W", ActionID: "file.close"},
					{Label: "Exit", Shortcut: "Ctrl+Q", ActionID: "app.exit"},
				},
			},
			{
				Title:    "Setting",
				Shortcut: "Alt+T",
				Items: []MenuItem{
					{Label: "Toggle Sidebar", Shortcut: "Ctrl+B", ActionID: "setting.sidebar"},
					{Label: "Toggle Auto-Save", Shortcut: "Alt+S", ActionID: "setting.autosave"},
					{Label: "Toggle Hidden Files", Shortcut: ".", ActionID: "setting.hidden"},
					{Label: "Keyboard Shortcuts", Shortcut: "?", ActionID: "setting.shortcuts"},
				},
			},
			{
				Title:    "About",
				Shortcut: "Alt+A",
				Items: []MenuItem{
					{Label: "About tui-workflow", Shortcut: "", ActionID: "about.open"},
				},
			},
		},
	}
	mb.updatePositions()
	return mb
}

func (m *MenuBar) updatePositions() {
	currX := 1
	for i := range m.Categories {
		cat := &m.Categories[i]
		cat.StartX = currX
		titleText := " " + cat.Title + " "
		renderedItem := CurrentTheme.TopMenuItemStyle.Render(titleText)
		currX += lipgloss.Width(renderedItem)
		cat.EndX = currX - 1
		currX++
	}
}

func (m *MenuBar) SetWidth(w int) {
	m.Width = w
}

func (m *MenuBar) Toggle(index int) {
	if m.ActiveMenu == index {
		m.ActiveMenu = -1
		m.SelectedItem = 0
	} else {
		m.ActiveMenu = index
		m.SelectedItem = 0
	}
}

func (m *MenuBar) Close() {
	m.ActiveMenu = -1
	m.SelectedItem = 0
}

func (m *MenuBar) IsOpen() bool {
	return m.ActiveMenu >= 0 && m.ActiveMenu < len(m.Categories)
}

func (m *MenuBar) HandleKey(msg tea.KeyMsg) (action string, handled bool) {
	if !m.IsOpen() {
		switch msg.String() {
		case "alt+f", "alt+F":
			m.Toggle(0)
			return "", true
		case "alt+t", "alt+T":
			m.Toggle(1)
			return "", true
		case "alt+a", "alt+A":
			m.Toggle(2)
			return "", true
		}
		return "", false
	}

	switch msg.String() {
	case "esc":
		m.Close()
		return "", true
	case "left":
		m.ActiveMenu--
		if m.ActiveMenu < 0 {
			m.ActiveMenu = len(m.Categories) - 1
		}
		m.SelectedItem = 0
		return "", true
	case "right":
		m.ActiveMenu++
		if m.ActiveMenu >= len(m.Categories) {
			m.ActiveMenu = 0
		}
		m.SelectedItem = 0
		return "", true
	case "up":
		curCat := m.Categories[m.ActiveMenu]
		m.SelectedItem--
		if m.SelectedItem < 0 {
			m.SelectedItem = len(curCat.Items) - 1
		}
		return "", true
	case "down":
		curCat := m.Categories[m.ActiveMenu]
		m.SelectedItem++
		if m.SelectedItem >= len(curCat.Items) {
			m.SelectedItem = 0
		}
		return "", true
	case "enter":
		curCat := m.Categories[m.ActiveMenu]
		if m.SelectedItem >= 0 && m.SelectedItem < len(curCat.Items) {
			actionID := curCat.Items[m.SelectedItem].ActionID
			m.Close()
			return actionID, true
		}
		m.Close()
		return "", true
	}

	return "", false
}

func (m *MenuBar) HandleMouse(msg tea.MouseMsg) (action string, handled bool) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return "", false
	}

	// Click on Row 0 (Menu bar items)
	if msg.Y == 0 {
		for i, cat := range m.Categories {
			if msg.X >= cat.StartX && msg.X <= cat.EndX {
				m.Toggle(i)
				return "", true
			}
		}
		if m.IsOpen() {
			m.Close()
			return "", true
		}
		return "", false
	}

	// Click inside dropdown if open
	if m.IsOpen() {
		cat := m.Categories[m.ActiveMenu]
		dropStartY := 1
		dropEndY := dropStartY + len(cat.Items)
		dropStartX := cat.StartX

		maxItemLen := 0
		for _, it := range cat.Items {
			l := len(it.Label) + len(it.Shortcut) + 4
			if l > maxItemLen {
				maxItemLen = l
			}
		}
		dropEndX := dropStartX + maxItemLen + 2

		if msg.Y >= dropStartY && msg.Y <= dropEndY && msg.X >= dropStartX && msg.X <= dropEndX {
			clickedItem := msg.Y - dropStartY
			if clickedItem >= 0 && clickedItem < len(cat.Items) {
				actionID := cat.Items[clickedItem].ActionID
				m.Close()
				return actionID, true
			}
		}

		// Clicked outside dropdown
		m.Close()
		return "", true
	}

	return "", false
}

func (m *MenuBar) Render() string {
	var sb strings.Builder
	currX := 1
	sb.WriteString(" ")

	for i := range m.Categories {
		cat := &m.Categories[i]
		cat.StartX = currX

		titleText := " " + cat.Title + " "
		var renderedItem string
		if m.ActiveMenu == i {
			renderedItem = CurrentTheme.TopMenuActiveItem.Render(titleText)
		} else {
			renderedItem = CurrentTheme.TopMenuItemStyle.Render(titleText)
		}
		sb.WriteString(renderedItem)
		currX += lipgloss.Width(renderedItem)
		cat.EndX = currX - 1
		sb.WriteString(" ")
		currX++
	}

	renderedBar := sb.String()
	barLen := lipgloss.Width(renderedBar)
	if barLen < m.Width {
		renderedBar += strings.Repeat(" ", m.Width-barLen)
	}

	return CurrentTheme.TopMenuStyle.Width(m.Width).Render(renderedBar)
}

func (m *MenuBar) RenderDropdown() (string, int, int) {
	if !m.IsOpen() {
		return "", 0, 0
	}

	cat := m.Categories[m.ActiveMenu]
	var lines []string

	maxLabelLen := 0
	for _, it := range cat.Items {
		if len(it.Label) > maxLabelLen {
			maxLabelLen = len(it.Label)
		}
	}

	for i, it := range cat.Items {
		padding := strings.Repeat(" ", maxLabelLen-len(it.Label)+3)
		itemStr := it.Label + padding + it.Shortcut
		if i == m.SelectedItem {
			lines = append(lines, CurrentTheme.TopMenuActiveItem.Render(" "+itemStr+" "))
		} else {
			lines = append(lines, CurrentTheme.DropdownItemStyle.Render(" "+itemStr+" "))
		}
	}

	content := strings.Join(lines, "\n")
	box := CurrentTheme.DropdownStyle.Render(content)
	return box, cat.StartX, 1
}
