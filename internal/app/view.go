package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) View() string {
	if !m.Ready {
		return "Loading tui-workflow..."
	}

	if IsTooSmall(m.Width, m.Height) {
		return RenderTooSmallWarning(m.Width, m.Height)
	}

	// 1. Top Menu Bar (Height = 2)
	if m.Width > 0 && m.MenuBar != nil && m.MenuBar.Width != m.Width {
		m.MenuBar.SetWidth(m.Width)
	}
	menuView := m.MenuBar.Render()

	// 2. Middle Content (Height = Height - 3: 2 for Menu, 1 for Status Bar)
	contentHeight := m.Height - 3
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

	// Overlay 2: Modals (QuickOpen, CommandPalette, GlobalSearch, GoToSymbol, Switcher, FilePicker, Input Dialog, Confirm Dialog, Help, or About)
	if m.QuickOpen != nil && m.QuickOpen.Active {
		modal := m.QuickOpen.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.CommandPalette != nil && m.CommandPalette.Active {
		modal := m.CommandPalette.Render()
		baseView = overlayTopCenter(baseView, modal, m.Width, m.Height)
	} else if m.GlobalSearch != nil && m.GlobalSearch.Active {
		modal := m.GlobalSearch.Render()
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
		baseView = overlayAt(baseView, barStr, findX, 2, m.Width, m.Height)
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
	for i, l := range lines {
		if ansi.StringWidth(l) > m.Width {
			lines[i] = ansi.Truncate(l, m.Width, "")
		}
	}
	if len(lines) > m.Height {
		lines = lines[:m.Height]
	}
	baseView = strings.Join(lines, "\n")

	return baseView
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

	// 3. Bottom Tabs Footer (Height = 2, 2x size)
	tabActive := ui.CurrentTheme.SidebarTabActive.Padding(0, 0)
	tabInactive := ui.CurrentTheme.SidebarTabInactive.Padding(0, 0)

	var tabFiles, tabGit, tabSearch string
	var indFiles, indGit, indSearch string

	activeIndStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgSidebar)

	inactiveIndStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.BorderColor).
		Background(ui.CurrentTheme.BgTopMenu)

	filesW := lipgloss.Width(" 1:Files ")
	gitW := lipgloss.Width(" 2:Git ")
	searchW := lipgloss.Width(" 3:Search ")

	if m.SidebarView == SidebarFiles {
		tabFiles = tabActive.Render(" 1:Files ")
		indFiles = activeIndStyle.Render(strings.Repeat("━", filesW))
	} else {
		tabFiles = tabInactive.Render(" 1:Files ")
		indFiles = inactiveIndStyle.Render(strings.Repeat(" ", filesW))
	}
	if m.SidebarView == SidebarGit {
		tabGit = tabActive.Render(" 2:Git ")
		indGit = activeIndStyle.Render(strings.Repeat("━", gitW))
	} else {
		tabGit = tabInactive.Render(" 2:Git ")
		indGit = inactiveIndStyle.Render(strings.Repeat(" ", gitW))
	}
	if m.SidebarView == SidebarSearch {
		tabSearch = tabActive.Render(" 3:Search ")
		indSearch = activeIndStyle.Render(strings.Repeat("━", searchW))
	} else {
		tabSearch = tabInactive.Render(" 3:Search ")
		indSearch = inactiveIndStyle.Render(strings.Repeat(" ", searchW))
	}

	footerRow1 := tabFiles + tabGit + tabSearch
	footerRow2 := indFiles + indGit + indSearch

	row1Len := lipgloss.Width(footerRow1)
	if row1Len < width-1 {
		footerRow1 += strings.Repeat(" ", width-1-row1Len)
	}
	row2Len := lipgloss.Width(footerRow2)
	if row2Len < width-1 {
		footerRow2 += strings.Repeat(" ", width-1-row2Len)
	}

	footerRow1 = ui.CurrentTheme.SidebarHeaderStyle.Width(width - 1).MaxHeight(1).Render(footerRow1)
	footerRow1 = ansi.Truncate(footerRow1, width-1, "")

	footerRow2 = ui.CurrentTheme.SidebarHeaderStyle.Width(width - 1).MaxHeight(1).Render(footerRow2)
	footerRow2 = ansi.Truncate(footerRow2, width-1, "")

	footerTabs := footerRow1 + "\n" + footerRow2

	bodyHeight := height - 4
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
		Width(width - 1).
		Height(height)

	if m.Focus == FocusSidebar {
		borderStyle = borderStyle.BorderForeground(ui.CurrentTheme.Accent)
	}

	return borderStyle.Render(colContent)
}

func (m Model) renderEditor(width, height int) string {
	// 1. Tab Bar (Height = 2)
	tabBar := m.TabManager.RenderTabBar(width)

	// 2. Editor Body (Remaining height)
	bodyHeight := height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
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
		bodyContent := "\n  Welcome to tui-workflow (v0.1.1)\n\n" +
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

func renderHelpModal() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("📖 KEYBOARD SHORTCUTS & HELP")

	content := `Global & Navigation:
 • Ctrl+Q : Exit application
 • Ctrl+E / F6 : Switch Focus Sidebar / Editor
 • Ctrl+B : Toggle Sidebar Visibility
 • Alt+[ / Alt+] : Narrow / Widen Sidebar Width
 • Alt+1 / 2 / 3 : Switch Sidebar Tab (Files / Git / Search)
 • Alt+F / Alt+E : Open Menu File / Edit
 • Alt+R / Alt+T : Open Menu Search / Settings
 • Alt+A : Open About Dialog
 • Alt+S / Alt+Z : Toggle Auto-Save / Soft-Wrap
 • Alt+I : AI Model Settings (OpenAI API Format)
 • ? / Esc : Open Help / Close Pop-up Dialog

Search & Code Navigation:
 • F1 / Ctrl+Shift+P : Open Command Palette
 • Ctrl+P : Quick Open File (Workspace)
 • Ctrl+Shift+O : Go to Symbol (workspace)
 • F12 / Shift+F12 : Go to Definition / Find References
 • Ctrl+F / Ctrl+H : Find / Find & Replace in Buffer
 • Ctrl+G : Go to Line Number
 • F4 : Global Search Across Workspace
 • Ctrl+O : Open File from Disk (File Picker)
 • Ctrl+K, Ctrl+K : Open Directory / Folder in Workspace

Editor & Buffer:
 • Ctrl+Z / Ctrl+Y : Undo / Redo Text Changes
 • Tab / Shift+Tab : Expand Snippet / Next Tab-Stop / Indent
 • Ctrl+Space : Trigger LSP Completion & Auto-Import
 • Ctrl+/ : Toggle Line Comment
 • Shift+Alt+Arrow : Select Text by Word (Word Jump Block)
 • Shift+Arrow : Select Text by Character / Line
 • Alt/Ctrl+Arrow : Jump Word (Left / Right)
 • Alt/Ctrl+Bcksp : Delete Word Left (or Delete Selection)
 • Ctrl+Shift+D : Add Cursor at Next Occurrence
 • Ctrl+Alt+Up/Dn : Add Cursor Above / Below
 • Ctrl+Shift+[ / ]: Fold / Unfold Block at Cursor
 • Ctrl+K Ctrl+0/J : Fold All / Unfold All
 • Click Gutter Fold: Toggle Fold Block (Marker ▸/▾)
 • Alt+P : Pin / Unpin Tab
 • Drag Tab Mouse : Drag & Drop Reorder Tabs
 • Ctrl+S / Ctrl+W : Save Buffer / Close Active Tab
 • Ctrl+Shift+I : Format Document (via CLI)
 • Ctrl+M / Alt+M : Toggle Markdown Preview (Glamour)
 • Ctrl+A / C / V : Select All / Copy / Paste Text

Terminal & Shell (PTY):
 • Ctrl+Shift+\ : Open New Terminal Shell Tab
 • Ctrl+\ : Switch Focus Terminal / TUI Editor
 • Ctrl+Shift+W : Close Active Terminal Shell Tab
 • Alt/Ctrl+Arrow : Jump Word in Shell CLI (Readline)
 • Alt/Ctrl+Bcksp : Delete Word Left in Shell CLI

Git Source Control:
 • Alt+B : Switch / Checkout Git Branch
 • Alt+G : Generate Commit Message via AI
 • d / [x] : Discard File Changes
 • + / - : Stage / Unstage File Changes
 • Ctrl+Enter : Commit Git Changes`

	closeBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Padding(0, 2).
		MarginTop(1).
		Render("[ Close (Esc / ?) ]")

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		content,
		closeBtn,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2).
		Width(60)

	return dialog.NormalizeDialogBackground(boxStyle.Render(body), ui.CurrentTheme.BgTopMenu)
}

func renderAboutModal() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("ℹ ABOUT tui-workflow")

	content := `tui-workflow v0.1.1
Modern, High-Performance Terminal User Interface Workflow Environment.

Core Features & Technologies:
 • Programming Language: Go 1.24+ (Goroutines, Channels)
 • Terminal Engine     : Charmbracelet Bubble Tea & Lipgloss
 • Syntax Tokenizer    : Chroma v2 Highlighting Cache
 • Markdown Renderer   : Glamour ANSI Stylesheet
 • Git Integration     : In-Memory Pure-Go (go-git/v5)
 • PTY Terminal        : Embedded Pseudoterminal Subshell

Engineered for 60 FPS, zero flickering, modeless text
editing, and full mouse support in terminal environments.`

	closeBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Padding(0, 2).
		MarginTop(1).
		Render("[ Close (Esc) ]")

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		content,
		closeBtn,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2).
		Width(64)

	return dialog.NormalizeDialogBackground(boxStyle.Render(body), ui.CurrentTheme.BgTopMenu)
}

func overlayAt(base, overlay string, atX, atY, totalW, totalH int) string {
	baseLines := strings.Split(base, "\n")
	overLines := strings.Split(overlay, "\n")

	for i, oLine := range overLines {
		targetY := atY + i
		if targetY >= len(baseLines) {
			break
		}

		bLine := baseLines[targetY]
		baseWidth := ansi.StringWidth(bLine)
		oWidth := ansi.StringWidth(oLine)

		var left, right string
		if atX > 0 {
			left = ansi.Cut(bLine, 0, atX)
			leftW := ansi.StringWidth(left)
			if leftW < atX {
				left += strings.Repeat(" ", atX-leftW)
			}
		}

		rightStart := atX + oWidth
		if rightStart < baseWidth {
			right = ansi.Cut(bLine, rightStart, baseWidth)
		}

		baseLines[targetY] = left + oLine + right
	}

	return strings.Join(baseLines, "\n")
}

func overlayCenter(base, modal string, totalW, totalH int) string {
	mWidth := lipgloss.Width(modal)
	mHeight := lipgloss.Height(modal)
	atX := (totalW - mWidth) / 2
	atY := (totalH - mHeight) / 2
	if atX < 0 {
		atX = 0
	}
	if atY < 1 {
		atY = 1
	}
	return overlayAt(base, modal, atX, atY, totalW, totalH)
}

func overlayTopCenter(base, modal string, totalW, totalH int) string {
	mWidth := lipgloss.Width(modal)
	mHeight := lipgloss.Height(modal)
	atX := (totalW - mWidth) / 2
	atY := (totalH - mHeight) / 3
	if atX < 0 {
		atX = 0
	}
	if atY < 1 {
		atY = 1
	}
	return overlayAt(base, modal, atX, atY, totalW, totalH)
}

func placeCenter(base, modal string, totalW, totalH int) string {
	return overlayCenter(base, modal, totalW, totalH)
}


func (m Model) renderGoToSymbol() string {
	if !m.GoToSymbolActive {
		return ""
	}
	width := 72
	if m.Width > 0 && m.Width-8 < width {
		width = m.Width - 8
	}
	if width < 40 {
		width = 40
	}
	title := ui.CurrentTheme.TopMenuStyle.Bold(true).Render(" Go to Symbol (Ctrl+Shift+O) ")
	queryLine := ui.CurrentTheme.StatusBarItemStyle.Render("❯ " + m.GoToSymbolQuery + "▌")
	maxRows := 12
	if len(m.GoToSymbolResults) == 0 {
		body := ui.CurrentTheme.SidebarStyle.Render("  (no symbols)")
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.CurrentTheme.Accent).Padding(0, 1).Width(width).Render(title + "\n" + queryLine + "\n" + body)
		return box
	}
	start := m.GoToSymbolScroll
	if start < 0 { start = 0 }
	end := start + maxRows
	if end > len(m.GoToSymbolResults) { end = len(m.GoToSymbolResults) }
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		s := m.GoToSymbolResults[i]
		rel := s.FilePath
		if rp, err := filepath.Rel(m.RootDir, s.FilePath); err == nil {
			rel = rp
		}
		label := fmt.Sprintf("%s  %s:%d", s.Name, rel, s.Line)
		label = ansi.Truncate(label, width-4, "…")
		if i == m.GoToSymbolSelected {
			lines = append(lines, ui.CurrentTheme.TopMenuActiveItem.Render(" "+label+" "))
		} else {
			lines = append(lines, " "+label)
		}
	}
	body := strings.Join(lines, "\n")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.CurrentTheme.Accent).Padding(0, 1).Width(width).Render(title + "\n" + queryLine + "\n" + body)
	return box
}
