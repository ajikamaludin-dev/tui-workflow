package dialog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// HelpDialog represents an interactive, scrollable keyboard shortcut cheatsheet modal.
type HelpDialog struct {
	Active    bool
	ScrollTop int
	Width     int
	MaxRows   int
	Lines     []string
}

// NewHelpDialog initializes a HelpDialog with structured shortcut reference lines.
func NewHelpDialog() *HelpDialog {
	rawContent := []string{
		"Global & Navigation:",
		" • Ctrl+Q : Exit application (save session; confirm if dirty)",
		" • Ctrl+E / F6 : Switch Focus Sidebar / Editor",
		" • Ctrl+B : Toggle Sidebar Visibility",
		" • Alt+[ / Alt+] : Narrow / Widen Sidebar Width",
		" • Alt+1 / 2 / 3 : Switch Sidebar Tab (Files / Git / Search)",
		" • Alt+F / Alt+E : Open Top Bar Menu File / Edit",
		" • Alt+R / Alt+T : Open Top Bar Menu Search / Settings",
		" • Alt+A : Open About Dialog",
		" • Alt+S : Toggle Auto-Save on Focus Change",
		" • Alt+Z : Toggle Soft-Wrap in Editor",
		" • Alt+I : AI Model Settings (OpenAI API Format)",
		" • ? / Esc : Open Help / Close Pop-up Dialog",
		"",
		"Search & Code Navigation:",
		" • F1 / Ctrl+Shift+P : Open Command Palette",
		" • Ctrl+P : Quick Open File (Workspace)",
		" • Ctrl+Shift+O : Go to Symbol (Workspace Symbol Index)",
		" • F12 : Go to Definition (Symbol Under Cursor)",
		" • Shift+F12 : Find References (Workspace)",
		" • Ctrl+F : Find in Editor Buffer (Find Bar)",
		" • Ctrl+H : Find & Replace in Buffer",
		" • Ctrl+G : Go to Line (and Column)",
		" • F4 : Global Search Across Workspace",
		" • Ctrl+O : Open File from Disk (File Picker)",
		" • Ctrl+K, Ctrl+K : Open Folder / Directory in Workspace",
		"",
		"Editor & Buffer:",
		" • Ctrl+Z / Ctrl+Y : Undo / Redo Text Changes",
		" • Tab / Shift+Tab : Expand Snippet / Next Tab-Stop / Indent",
		" • Ctrl+Space : Trigger LSP Completion & Auto-Import",
		" • Ctrl+/ : Toggle Line Comment",
		" • Shift+Alt+Arrow : Select Text by Word (Word Jump Block)",
		" • Shift+Arrow : Select Text by Character / Line",
		" • Alt/Ctrl+Arrow : Jump Word (Left / Right)",
		" • Alt/Ctrl+Bcksp : Delete Word Left (or Delete Selection)",
		" • Ctrl+Shift+D : Add Cursor at Next Occurrence",
		" • Ctrl+Alt+Up/Down : Add Cursor Above / Below",
		" • Esc : Merge Multi-Cursors / Clear Selection",
		" • Ctrl+Shift+[ / ] : Fold / Unfold Block at Cursor",
		" • Ctrl+K Ctrl+0/J : Fold All / Unfold All (Ctrl+K, Ctrl+K: open folder)",
		" • Click Gutter Fold : Toggle Fold Block (Marker ▸/▾)",
		" • Ctrl+S : Save Active Buffer to Disk",
		" • Ctrl+W : Close Active Tab / Diff Tab",
		" • Middle-Click Tab : Close Clicked Tab",
		" • Ctrl+Shift+I : Format Document (CLI Formatter)",
		" • Ctrl+M / Alt+M : Toggle Markdown Preview (Glamour) / Raw",
		" • Ctrl+A / C / V : Select All / Copy / Paste Text",
		" • Right-Click Editor : Open Context Menu (Copy/Paste/Cut)",
		"",
		"File Explorer:",
		" • j / k / Arrow : Highlight Next / Previous Node",
		" • Enter / l : Open File / Expand Folder",
		" • h / Left : Collapse Folder or Jump to Parent",
		" • a / A : New File / New Folder",
		" • F2 : Rename File or Folder",
		" • d : Delete Selected File or Folder",
		" • / : Filter File Tree (Enter empty: clear)",
		" • . : Toggle Hidden Files (Dotfiles)",
		" • Right-Click Node : Open Explorer Context Menu (CRUD & Copy Path)",
		"",
		"Git Source Control (Sidebar):",
		" • Alt+B : Switch / Checkout Git Branch",
		" • Alt+G / [✨ AI] : Generate Commit Message via AI",
		" • + / - : Stage / Unstage All File Changes",
		" • [ / ] : Navigate Active Hunk (Selected File)",
		" • s / u : Stage / Unstage Active Hunk (Changes/Staged)",
		" • x : Discard Active Hunk (Unstaged, with confirmation)",
		" • Enter : Open Diff Tab for Selected File",
		" • Tab / ↑↓ / j k : Switch Focus Section & Navigate Items",
		" • Ctrl+Enter : Commit Git Changes",
		" • Click Push : Push Changes to Remote Repository",
		"",
		"Diff Tab (Side-by-Side, Git Hunk):",
		" • n / ] / N / [ : Jump to Next / Previous Hunk",
		" • s / + : Stage Active Hunk (Unstaged Diff)",
		" • u / - : Unstage Active Hunk (Staged Diff)",
		" • x : Discard Active Hunk (Unstaged, with confirmation)",
		" • ↑↓ / j k / PgUpDn : Scroll Diff View",
		" • Ctrl+W : Close Diff Tab",
		"",
		"Terminal & Shell (PTY):",
		" • Ctrl+Shift+\\ / Ctrl+T : Open New Terminal Tab",
		" • Ctrl+\\ / Ctrl+` / F6 : Switch Focus Terminal / Editor",
		" • Alt+W / Ctrl+Shift+W : Close Active Terminal Tab",
		" • Alt/Ctrl+Arrow : Jump Word in Shell CLI (Readline)",
		" • Alt/Ctrl+Bcksp : Delete Word Left in Shell CLI",
		" • Mouse Wheel : Scroll Terminal History",
	}

	return &HelpDialog{
		Active:    false,
		ScrollTop: 0,
		Width:     68,
		MaxRows:   14,
		Lines:     rawContent,
	}
}

// Open activates the help modal and resets scroll position.
func (h *HelpDialog) Open(screenH int) {
	h.Active = true
	h.ScrollTop = 0
	h.calculateMaxRows(screenH)
}

// Close hides the help modal.
func (h *HelpDialog) Close() {
	h.Active = false
}

func (h *HelpDialog) calculateMaxRows(screenH int) {
	if screenH <= 0 {
		h.MaxRows = 14
		return
	}
	// Chrome inside box: border (2) + padding (2) + title (2) + scroll indicator (1) + close button (2) = 9
	avail := screenH - 9
	if avail < 6 {
		avail = 6
	} else if avail > 20 {
		avail = 20
	}
	h.MaxRows = avail
}

// Update handles keyboard navigation inside help modal.
func (h *HelpDialog) Update(msg tea.Msg) (closed bool) {
	if !h.Active {
		return false
	}

	maxScroll := len(h.Lines) - h.MaxRows
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			h.Close()
			return true
		case "up", "k":
			if h.ScrollTop > 0 {
				h.ScrollTop--
			}
			return false
		case "down", "j":
			if h.ScrollTop < maxScroll {
				h.ScrollTop++
			}
			return false
		case "pgup":
			h.ScrollTop -= h.MaxRows
			if h.ScrollTop < 0 {
				h.ScrollTop = 0
			}
			return false
		case "pgdown", "space":
			h.ScrollTop += h.MaxRows
			if h.ScrollTop > maxScroll {
				h.ScrollTop = maxScroll
			}
			return false
		case "home", "g":
			h.ScrollTop = 0
			return false
		case "end", "G":
			h.ScrollTop = maxScroll
			return false
		case "enter":
			h.Close()
			return true
		}
	}
	return false
}

// HandleMouse processes mouse clicks and wheel scrolling for the help modal.
func (h *HelpDialog) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (closed bool, handled bool) {
	if !h.Active {
		return false, false
	}

	rendered := h.Render(0)
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := 0
	for _, l := range lines {
		w := ansi.StringWidth(l)
		if w > boxW {
			boxW = w
		}
	}

	// Click outside modal dismisses it
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		h.Close()
		return true, true
	}

	maxScroll := len(h.Lines) - h.MaxRows
	if maxScroll < 0 {
		maxScroll = 0
	}

	if isWheelUp {
		h.ScrollTop -= 2
		if h.ScrollTop < 0 {
			h.ScrollTop = 0
		}
		return false, true
	}
	if isWheelDown {
		h.ScrollTop += 2
		if h.ScrollTop > maxScroll {
			h.ScrollTop = maxScroll
		}
		return false, true
	}

	// Click on close button (last 2 rows inside box)
	if relY >= boxH-3 && relY <= boxH-2 {
		h.Close()
		return true, true
	}

	return false, true
}

// Render renders the scrollable help modal with border, title, visible lines, and scroll hints.
func (h *HelpDialog) Render(screenH int) string {
	if !h.Active {
		return ""
	}
	if screenH > 0 {
		h.calculateMaxRows(screenH)
	}

	maxScroll := len(h.Lines) - h.MaxRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.ScrollTop > maxScroll {
		h.ScrollTop = maxScroll
	}
	if h.ScrollTop < 0 {
		h.ScrollTop = 0
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.BgTopMenu).
		MarginBottom(1)
	renderedTitle := titleStyle.Render("📖 KEYBOARD SHORTCUTS & HELP")

	endIdx := h.ScrollTop + h.MaxRows
	if endIdx > len(h.Lines) {
		endIdx = len(h.Lines)
	}

	var visibleRows []string
	headerCategoryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu)
	bodyTextStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	for i := h.ScrollTop; i < endIdx; i++ {
		line := h.Lines[i]
		if strings.HasSuffix(line, ":") {
			visibleRows = append(visibleRows, headerCategoryStyle.Render(line))
		} else {
			visibleRows = append(visibleRows, bodyTextStyle.Render(line))
		}
	}

	// Pad visible rows to h.MaxRows for stability
	for len(visibleRows) < h.MaxRows {
		visibleRows = append(visibleRows, bodyTextStyle.Render(""))
	}

	// Scroll indicator info
	scrollInfoStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary).
		Background(ui.CurrentTheme.BgTopMenu)

	scrollText := "↑↓ / Wheel: Scroll"
	if h.ScrollTop > 0 && h.ScrollTop < maxScroll {
		scrollText = "▲ More above | ▼ More below (" + scrollText + ")"
	} else if h.ScrollTop > 0 {
		scrollText = "▲ More above (" + scrollText + ")"
	} else if maxScroll > 0 {
		scrollText = "▼ More below (" + scrollText + ")"
	}
	renderedScrollInfo := scrollInfoStyle.Render(scrollText)

	closeBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Padding(0, 2).
		MarginTop(1).
		Render("[ Close (Esc / ?) ]")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		strings.Join(visibleRows, "\n"),
		renderedScrollInfo,
		closeBtn,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2).
		Width(h.Width)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
