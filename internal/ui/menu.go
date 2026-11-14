package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
					{Label: "New Terminal Tab", Shortcut: "Ctrl+\\", ActionID: "terminal.new"},
					{Label: "Open File", Shortcut: "Ctrl+O", ActionID: "file.open"},
					{Label: "Open Folder", Shortcut: "Ctrl+K Ctrl+K", ActionID: "file.open_folder"},
					{Label: "Save File", Shortcut: "Ctrl+S", ActionID: "file.save"},
					{Label: "Format Document", Shortcut: "Ctrl+Shift+I", ActionID: "file.format"},
					{Label: "Pin / Unpin Tab", Shortcut: "Alt+P", ActionID: "tab.toggle_pin"},
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
					{Label: "Trigger Completion", Shortcut: "Ctrl+Space", ActionID: "edit.lsp_completion"},
					{Label: "Find...", Shortcut: "Ctrl+F", ActionID: "search.find"},
					{Label: "Find & Replace...", Shortcut: "Ctrl+H", ActionID: "search.replace"},
					{Label: "Go to Line...", Shortcut: "Ctrl+G", ActionID: "edit.goto_line"},
					{Label: "Toggle Comment", Shortcut: "Ctrl+/", ActionID: "edit.toggle_comment"},
					{Label: "Delete Word Left", Shortcut: "Alt+Backspace", ActionID: "edit.delete_word_left"},
					{Label: "Add Cursor Above", Shortcut: "Ctrl+Alt+Up", ActionID: "edit.add_cursor_above"},
					{Label: "Add Cursor Below", Shortcut: "Ctrl+Alt+Down", ActionID: "edit.add_cursor_below"},
					{Label: "Add Next Occurrence", Shortcut: "Ctrl+Shift+D", ActionID: "edit.add_next_occurrence"},
					{Label: "Fold Block", Shortcut: "Ctrl+Shift+[", ActionID: "edit.fold"},
					{Label: "Unfold Block", Shortcut: "Ctrl+Shift+]", ActionID: "edit.unfold"},
					{Label: "Fold All", Shortcut: "Ctrl+K Ctrl+0", ActionID: "edit.fold_all"},
					{Label: "Unfold All", Shortcut: "Ctrl+K Ctrl+J", ActionID: "edit.unfold_all"},
				},
			},
			{
				Title:    "Search",
				Shortcut: "Alt+R",
				Items: []MenuItem{
					{Label: "Command Palette...", Shortcut: "F1", ActionID: "search.palette"},
					{Label: "Quick Open File...", Shortcut: "Ctrl+P", ActionID: "search.quick_open"},
					{Label: "Go to Symbol...", Shortcut: "Ctrl+Shift+O", ActionID: "search.goto_symbol"},
					{Label: "Go to Definition", Shortcut: "F12", ActionID: "search.goto_definition"},
					{Label: "Find References", Shortcut: "Shift+F12", ActionID: "search.find_references"},
					{Label: "Find in Current File...", Shortcut: "Ctrl+F", ActionID: "search.find"},
					{Label: "Find & Replace...", Shortcut: "Ctrl+H", ActionID: "search.replace"},
					{Label: "Find in Files (Global)...", Shortcut: "F4", ActionID: "search.global"},
					{Label: "Go to Line...", Shortcut: "Ctrl+G", ActionID: "edit.goto_line"},
				},
			},
			{
				Title:    "Git",
				Shortcut: "Alt+Gm",
				Items: []MenuItem{
					{Label: "Stage All Changes", Shortcut: "", ActionID: "git.stage_all"},
					{Label: "Unstage All Changes", Shortcut: "", ActionID: "git.unstage_all"},
					{Label: "Next Hunk", Shortcut: "n/]", ActionID: "git.hunk_next"},
					{Label: "Previous Hunk", Shortcut: "N/[", ActionID: "git.hunk_prev"},
					{Label: "Stage Hunk", Shortcut: "s", ActionID: "git.hunk_stage"},
					{Label: "Unstage Hunk", Shortcut: "u", ActionID: "git.hunk_unstage"},
					{Label: "Discard Hunk", Shortcut: "x", ActionID: "git.hunk_discard"},
					{Label: "Switch Branch...", Shortcut: "Alt+B", ActionID: "git.branch_switcher"},
					{Label: "AI Commit Message", Shortcut: "Alt+G", ActionID: "git.ai_commit"},
				},
			},
			{
				Title:    "Setting",
				Shortcut: "Alt+T",
				Items: []MenuItem{
					{Label: "Toggle Sidebar", Shortcut: "Ctrl+B", ActionID: "setting.sidebar"},
					{Label: "Toggle Soft Wrap", Shortcut: "Alt+Z", ActionID: "setting.softwrap"},
					{Label: "Toggle Auto-Save", Shortcut: "Alt+S", ActionID: "setting.autosave"},
					{Label: "Toggle Format on Save", Shortcut: "", ActionID: "setting.format_on_save"},
					{Label: "Toggle Indent Rainbow", Shortcut: "", ActionID: "edit.indent_rainbow"},
					{Label: "Toggle Hidden Files", Shortcut: ".", ActionID: "setting.hidden"},
					{Label: "Switch Git Branch...", Shortcut: "Alt+B", ActionID: "git.branch_switcher"},
					{Label: "AI Settings...", Shortcut: "Alt+I", ActionID: "setting.ai"},
				},
			},
			{
				Title:    "About",
				Shortcut: "Alt+A",
				Items: []MenuItem{
					{Label: "Keyboard Shortcuts", Shortcut: "?", ActionID: "setting.shortcuts"},
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

func (m *MenuBar) HandleKey(msg tea.KeyPressMsg) (action string, handled bool) {
	if !m.IsOpen() {
		switch KeyName(msg) {
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

	switch KeyName(msg) {
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
	click, isClick := msg.(tea.MouseClickMsg)
	if !isClick || click.Button != tea.MouseLeft {
		return "", false
	}

	// Click on Row 0 (Menu bar items)
	if click.Y == 0 {
		for i, cat := range m.Categories {
			if click.X >= cat.StartX && click.X <= cat.EndX {
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
		if click.X >= dropStartX && click.X < dropStartX+boxWidth &&
			click.Y >= dropStartY && click.Y < dropStartY+boxHeight {
			// Item rows are inside the border (Y = dropStartY + 1 up to dropStartY + len(cat.Items))
			itemY := click.Y - (dropStartY + 1)
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
	barLen := ansi.StringWidth(renderedBar)
	if m.Width > 0 {
		if barLen < m.Width {
			renderedBar += strings.Repeat(" ", m.Width-barLen)
		} else if barLen > m.Width {
			renderedBar = ansi.Truncate(renderedBar, m.Width, "")
		}
		return CurrentTheme.TopMenuStyle.Width(m.Width).MaxHeight(1).Render(renderedBar)
	}

	return CurrentTheme.TopMenuStyle.MaxHeight(1).Render(renderedBar)
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
