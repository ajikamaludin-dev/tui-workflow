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
					{Label: "Open File", Shortcut: "Ctrl+O", ActionID: "file.open"},
					{Label: "Open Folder", Shortcut: "Ctrl+K", ActionID: "file.open_folder"},
					{Label: "Save File", Shortcut: "Ctrl+S", ActionID: "file.save"},
					{Label: "Close Tab", Shortcut: "Ctrl+W", ActionID: "file.close"},
					{Label: "Exit", Shortcut: "Ctrl+Q", ActionID: "app.exit"},
				},
			},
			{
				Title:    "Edit",
				Shortcut: "Alt+E",
				Items: []MenuItem{
					{Label: "Undo", Shortcut: "Ctrl+Z", ActionID: "edit.undo"},
					{Label: "Redo", Shortcut: "Ctrl+Y", ActionID: "edit.redo"},
					{Label: "Find...", Shortcut: "Ctrl+F", ActionID: "search.find"},
					{Label: "Find & Replace...", Shortcut: "Ctrl+H", ActionID: "search.replace"},
					{Label: "Go to Line...", Shortcut: "Ctrl+G", ActionID: "edit.goto_line"},
					{Label: "Toggle Comment", Shortcut: "Ctrl+/", ActionID: "edit.toggle_comment"},
				},
			},
			{
				Title:    "Search",
				Shortcut: "Alt+R",
				Items: []MenuItem{
					{Label: "Command Palette...", Shortcut: "F1", ActionID: "search.palette"},
					{Label: "Quick Open File...", Shortcut: "Ctrl+P", ActionID: "search.quick_open"},
					{Label: "Find in Current File...", Shortcut: "Ctrl+F", ActionID: "search.find"},
					{Label: "Find & Replace...", Shortcut: "Ctrl+H", ActionID: "search.replace"},
					{Label: "Find in Files (Global)...", Shortcut: "F4", ActionID: "search.global"},
					{Label: "Go to Line...", Shortcut: "Ctrl+G", ActionID: "edit.goto_line"},
				},
			},
			{
				Title:    "Setting",
				Shortcut: "Alt+T",
				Items: []MenuItem{
					{Label: "Toggle Sidebar", Shortcut: "Ctrl+B", ActionID: "setting.sidebar"},
					{Label: "Toggle Soft Wrap", Shortcut: "Alt+Z", ActionID: "setting.softwrap"},
					{Label: "Toggle Auto-Save", Shortcut: "Alt+S", ActionID: "setting.autosave"},
					{Label: "Toggle Hidden Files", Shortcut: ".", ActionID: "setting.hidden"},
					{Label: "Switch Git Branch...", Shortcut: "Alt+B", ActionID: "git.branch_switcher"},
					{Label: "AI Settings...", Shortcut: "Alt+I", ActionID: "setting.ai"},
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
		case "alt+e", "alt+E":
			m.Toggle(1)
			return "", true
		case "alt+r", "alt+R":
			m.Toggle(2)
			return "", true
		case "alt+t", "alt+T":
			m.Toggle(3)
			return "", true
		case "alt+a", "alt+A":
			m.Toggle(4)
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
		box, dropStartX, dropStartY := m.RenderDropdown()
		boxWidth := lipgloss.Width(box)
		boxHeight := lipgloss.Height(box)

		// Check if click is within the dropdown box boundaries
		if msg.X >= dropStartX && msg.X < dropStartX+boxWidth &&
			msg.Y >= dropStartY && msg.Y < dropStartY+boxHeight {
			// Item rows are inside the border (Y = dropStartY + 1 up to dropStartY + len(cat.Items))
			itemY := msg.Y - (dropStartY + 1)
			if itemY >= 0 && itemY < len(cat.Items) {
				actionID := cat.Items[itemY].ActionID
				m.Close()
				return actionID, true
			}
			// Clicked on border of dropdown box
			return "", true
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
