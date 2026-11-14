package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
)

func (m Model) View() tea.View {
	if !m.Ready {
		return tea.NewView("Loading tui-workflow...")
	}

	if IsTooSmall(m.Width, m.Height) {
		return tea.NewView(RenderTooSmallWarning(m.Width, m.Height))
	}

	// 1. Top Menu Bar (Height = 1)
	if m.Width > 0 && m.MenuBar != nil && m.MenuBar.Width != m.Width {
		m.MenuBar.SetWidth(m.Width)
	}
	menuView := m.MenuBar.Render()

	// 2. Middle Content (Height = Height - 2)
	contentHeight := m.Height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	var middleView string
	if m.SidebarVisible {
		sidebarWidth := m.SidebarWidth
		editorWidth := m.Width - sidebarWidth
		if editorWidth < 10 {
			editorWidth = 10
		}

		sidebarCol := m.renderSidebar(sidebarWidth, contentHeight)
		editorCol := m.renderEditor(editorWidth, contentHeight)

		middleView = lipgloss.JoinHorizontal(lipgloss.Top, sidebarCol, editorCol)
	} else {
		middleView = m.renderEditor(m.Width, contentHeight)
	}

	// 3. Status Bar (Height = 1)
	statusView := m.StatusBar.Render()

	// Base layout joined vertically
	baseView := lipgloss.JoinVertical(lipgloss.Left, menuView, middleView, statusView)

	// Overlay 1: Dropdown Menu
	if m.MenuBar.IsOpen() {
		dropBox, dropX, dropY := m.MenuBar.RenderDropdown()
		baseView = overlayAt(baseView, dropBox, dropX, dropY, m.Width, m.Height)
	}

	// Overlay 2: Modals (QuickOpen, CommandPalette, GoToSymbol, Switcher, FilePicker, Input Dialog, Confirm Dialog, Help, or About)
	if m.QuickOpen != nil && m.QuickOpen.Active {
		modal := m.QuickOpen.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.CommandPalette != nil && m.CommandPalette.Active {
		modal := m.CommandPalette.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.GoToSymbolActive {
		modal := m.renderGoToSymbol()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.Switcher.Active {
		modal := m.Switcher.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.BranchSwitcher != nil && m.BranchSwitcher.Active {
		modal := m.BranchSwitcher.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.FilePicker != nil && m.FilePicker.Active {
		modal := m.FilePicker.Render()
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	} else if m.InputDialog.Active {
		modal := m.InputDialog.Render()
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	} else if m.ConfirmDialog.Active {
		modal := m.ConfirmDialog.Render()
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	} else if m.AISettingsDialog != nil && m.AISettingsDialog.Active {
		modal := m.AISettingsDialog.Render()
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	} else if m.ShowHelp {
		modal := ""
		if m.HelpDialog != nil {
			modal = m.HelpDialog.Render(m.Height)
		} else {
			modal = renderHelpModal()
		}
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	} else if m.ShowAbout {
		modal := renderAboutModal()
		baseView = overlayCenter(baseView, modal, m.Width, m.Height)
	}

	// Overlay 2.5: Find Bar (Floating in Editor Pane top-right)
	if m.FindBar != nil && m.FindBar.Active {
		barStr := m.FindBar.Render()
		findWidth := lipgloss.Width(barStr)
		findX := m.Width - findWidth - 2
		editorStartX := 0
		if m.SidebarVisible {
			editorStartX = m.SidebarWidth
		}
		if findX < editorStartX+1 {
			findX = editorStartX + 1
		}
		baseView = overlayAt(baseView, barStr, findX, 1, m.Width, m.Height)
	}

	// Overlay 3: Context Menu
	if m.ContextMenu != nil && m.ContextMenu.Active {
		menuStr := m.ContextMenu.Render()
		baseView = overlayAt(baseView, menuStr, m.ContextMenu.X, m.ContextMenu.Y, m.Width, m.Height)
	}

	// Overlay 4: Autocomplete Popup
	if m.CompletionPopup != nil && m.CompletionPopup.Active {
		popupStr := m.CompletionPopup.Render(m.Width, m.Height)
		baseView = overlayAt(baseView, popupStr, m.CompletionPopup.X, m.CompletionPopup.Y, m.Width, m.Height)
	}

	lines := strings.Split(baseView, "\n")
	if len(lines) > m.Height {
		baseView = strings.Join(lines[:m.Height], "\n")
	}

	v := tea.NewView(baseView)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderSidebar(width, height int) string {
	// 1. Top Title Header (Row 1 in terminal)
	var title string
	switch m.SidebarView {
	case SidebarFiles:
		title = " 📁 EXPLORER"
	case SidebarGit:
		branchName := ""
		if m.GitRepo != nil {
			branchName, _ = m.GitRepo.GetBranchName()
		}
		if branchName == "" {
			branchName = "HEAD"
		}
		title = " 🌿 SOURCE CONTROL (" + branchName + ")"
	case SidebarSearch:
		title = " 🔍 SEARCH & REPLACE"
	}
	titleLen := lipgloss.Width(title)
	if titleLen < width-1 {
		title += strings.Repeat(" ", width-1-titleLen)
	}
	headerRow := ui.CurrentTheme.SidebarHeaderStyle.Width(width - 1).MaxHeight(1).Render(title)
	headerRow = ansi.Truncate(headerRow, width-1, "")

	// 2. Refresh button row (Row 2 in terminal)
	refreshBtn := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.Accent).
		Bold(true).
		Render(" [⟳ Refresh]")
	refreshRow := ui.CurrentTheme.SidebarHeaderStyle.
		Width(width - 1).
		MaxHeight(1).
		Render(refreshBtn)
	refreshRow = ansi.Truncate(refreshRow, width-1, "")

	// 3. Bottom Tabs Footer (Row Height-2 in terminal)
	tabActive := ui.CurrentTheme.SidebarTabActive.Padding(0, 0)
	tabInactive := ui.CurrentTheme.SidebarTabInactive.Padding(0, 0)

	var tabFiles, tabGit, tabSearch string
	if m.SidebarView == SidebarFiles {
		tabFiles = tabActive.Render(" 1:Files ")
	} else {
		tabFiles = tabInactive.Render(" 1:Files ")
	}
	if m.SidebarView == SidebarGit {
		tabGit = tabActive.Render(" 2:Git ")
	} else {
		tabGit = tabInactive.Render(" 2:Git ")
	}
	if m.SidebarView == SidebarSearch {
		tabSearch = tabActive.Render(" 3:Search ")
	} else {
		tabSearch = tabInactive.Render(" 3:Search ")
	}

	footerTabs := tabFiles + tabGit + tabSearch
	footerLen := lipgloss.Width(footerTabs)
	if footerLen < width-1 {
		footerTabs += strings.Repeat(" ", width-1-footerLen)
	}
	footerTabs = ui.CurrentTheme.SidebarHeaderStyle.Width(width - 1).MaxHeight(1).Render(footerTabs)
	footerTabs = ansi.Truncate(footerTabs, width-1, "")

	bodyHeight := height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	if m.SidebarView == SidebarFiles {
		if m.Explorer.FilterQuery != "" {
			filterBanner := lipgloss.NewStyle().
				Foreground(ui.CurrentTheme.Accent).
				Background(ui.CurrentTheme.BgTopMenu).
				Width(width - 1).
				MaxHeight(1).
				Render(ansi.Truncate(" 🔍 "+m.Explorer.FilterQuery+" (Esc:Cancel)", width-1, ""))
			bodyH := bodyHeight - 1
			if bodyH < 1 {
				bodyH = 1
			}
			treeView := m.Explorer.Render(width-1, bodyH, m.Focus == FocusSidebar)
			body = filterBanner + "\n" + treeView
		} else {
			body = m.Explorer.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
		}
	} else if m.SidebarView == SidebarGit {
		if m.GitView != nil {
			body = m.GitView.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
		} else {
			body = "  Not a Git repository"
		}
	} else if m.SidebarView == SidebarSearch {
		if m.SearchView != nil {
			body = m.SearchView.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
		} else {
			body = "  Search view not ready"
		}
	}

	// Pad or truncate body to exactly bodyHeight lines so footerTabs is always pinned to the bottom row
	bodyLines := strings.Split(body, "\n")
	if len(bodyLines) > bodyHeight {
		bodyLines = bodyLines[:bodyHeight]
	} else {
		for len(bodyLines) < bodyHeight {
			bodyLines = append(bodyLines, strings.Repeat(" ", width-1))
		}
	}
	body = strings.Join(bodyLines, "\n")

	colContent := headerRow + "\n" + refreshRow + "\n" + body + "\n" + footerTabs

	borderStyle := ui.CurrentTheme.SidebarStyle.
		Width(width).
		Height(height)

	if m.Focus == FocusSidebar {
		borderStyle = borderStyle.BorderForeground(ui.CurrentTheme.Accent)
	}

	return borderStyle.Render(colContent)
}

func (m Model) renderEditor(width, height int) string {
	// 1. Tab Bar
	tabBar := m.TabManager.RenderTabBar(width)

	// 2. Editor Body
	bodyHeight := height - 1
	var body string

	activeTab := m.TabManager.ActiveTab()
	if activeTab != nil {
		if activeTab.Type == editor.TabTerminal && activeTab.TerminalView != nil {
			body = activeTab.TerminalView.Render(width, bodyHeight, m.Focus == FocusEditor)
		} else if activeTab.IsDiff && activeTab.DiffView != nil {
			body = activeTab.DiffView.Render(width, bodyHeight, m.Focus == FocusEditor)
		} else if activeTab.Type == editor.TabMarkdown && activeTab.MarkdownPreview && activeTab.MarkdownViewer != nil {
			body = activeTab.MarkdownViewer.Render(activeTab.Buffer.Text(), width, bodyHeight)
		} else if activeTab.Buffer != nil {
			body = activeTab.Buffer.Render(width, bodyHeight, activeTab.Path, m.Focus == FocusEditor)
		}
	} else {
		bodyContent := "\n  Welcome to tui-workflow (v0.2.0)\n\n" +
			"  No file is currently open.\n\n" +
			"  • Open file from File Explorer on the left (Enter or Click)\n" +
			"  • Create new file with 'a' in Explorer\n" +
			"  • Quick buffer switcher via Ctrl+P\n" +
			"  • Press ? for keyboard shortcuts & help\n"
		lines := strings.Split(bodyContent, "\n")
		for len(lines) < bodyHeight {
			lines = append(lines, "")
		}
		if len(lines) > bodyHeight {
			lines = lines[:bodyHeight]
		}
		body = strings.Join(lines, "\n")
	}

	editorContent := tabBar + "\n" + body

	editorStyle := ui.CurrentTheme.EditorStyle.
		Width(width).
		Height(height)

	return editorStyle.Render(editorContent)
}
