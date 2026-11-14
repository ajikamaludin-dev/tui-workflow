package app

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"path/filepath"
	"strings"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

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
 • F1 / Ctrl+Shift+P : Open Command Palette (or type '>' in Ctrl+P)
 • Ctrl+P : Quick Open File (Workspace)
 • Ctrl+Shift+O : Go to Symbol (workspace)
 • F12 / Shift+F12 : Go to Definition / Find References
 • Ctrl+F / Ctrl+H : Find / Find & Replace in Buffer
 • Ctrl+G : Go to Line Number
 • F4 : Search & Replace Sidebar (focus Find input)
 • Alt+C / Alt+W / Alt+X : Toggle Case / Whole-Word / Regex (Find Bar & Search Sidebar)
 • Alt+Enter / Alt+Shift+Enter : Replace selected match / all matches in file (Search Sidebar)
 • Alt+3 → Tab : Cycle Find → Replace → Path → Exclude → results list
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
 • Ctrl+\ / Ctrl+T : Open New Terminal Shell Tab
 • Ctrl+Backtick / F6 : Switch Focus Terminal / TUI Editor
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

	content := `tui-workflow v0.2.0
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
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > len(m.GoToSymbolResults) {
		end = len(m.GoToSymbolResults)
	}
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
