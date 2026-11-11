package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
)

func (m Model) View() string {
	if !m.Ready {
		return "Memuat tui-workflow..."
	}

	if IsTooSmall(m.Width, m.Height) {
		return RenderTooSmallWarning(m.Width, m.Height)
	}

	// 1. Top Menu Bar (Height = 1)
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

	// Overlay 2: Modals (Switcher, Input Dialog, Confirm Dialog, Help, or About)
	if m.Switcher.Active {
		modal := m.Switcher.Render()
		baseView = placeCenter(baseView, modal, m.Width, m.Height)
	} else if m.InputDialog.Active {
		modal := m.InputDialog.Render()
		baseView = placeCenter(baseView, modal, m.Width, m.Height)
	} else if m.ConfirmDialog.Active {
		modal := m.ConfirmDialog.Render()
		baseView = placeCenter(baseView, modal, m.Width, m.Height)
	} else if m.ShowHelp {
		modal := renderHelpModal()
		baseView = placeCenter(baseView, modal, m.Width, m.Height)
	} else if m.ShowAbout {
		modal := renderAboutModal()
		baseView = placeCenter(baseView, modal, m.Width, m.Height)
	}

	lines := strings.Split(baseView, "\n")
	if len(lines) > m.Height {
		baseView = strings.Join(lines[:m.Height], "\n")
	}

	return baseView
}

func (m Model) renderSidebar(width, height int) string {
	var tabFiles, tabGit string
	if m.SidebarView == SidebarFiles {
		tabFiles = ui.CurrentTheme.SidebarTabActive.Render(" 1:Files ")
		tabGit = ui.CurrentTheme.SidebarTabInactive.Render(" 2:Git ")
	} else {
		tabFiles = ui.CurrentTheme.SidebarTabInactive.Render(" 1:Files ")
		tabGit = ui.CurrentTheme.SidebarTabActive.Render(" 2:Git ")
	}
	header := tabFiles + tabGit
	headerLen := lipgloss.Width(header)
	if headerLen < width {
		header += strings.Repeat(" ", width-headerLen)
	}
	header = ui.CurrentTheme.SidebarHeaderStyle.Width(width).Render(header)

	bodyHeight := height - 1
	var body string
	if m.SidebarView == SidebarFiles {
		body = m.Explorer.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
	} else if m.GitView != nil {
		body = m.GitView.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
	} else {
		body = " Bukan repositori Git"
	}

	colContent := header + "\n" + body

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
		if activeTab.IsDiff && activeTab.DiffView != nil {
			body = activeTab.DiffView.Render(width, bodyHeight, m.Focus == FocusEditor)
		} else if activeTab.Type == editor.TabMarkdown && activeTab.MarkdownPreview && activeTab.MarkdownViewer != nil {
			body = activeTab.MarkdownViewer.Render(activeTab.Buffer.Text(), width, bodyHeight)
		} else if activeTab.Buffer != nil {
			body = activeTab.Buffer.Render(width, bodyHeight, activeTab.Path, m.Focus == FocusEditor)
		}
	} else {
		bodyContent := "\n  Selamat Datang di tui-workflow (v1.1.0)\n\n" +
			"  Tidak ada file yang sedang dibuka.\n\n" +
			"  • Buka file dari File Explorer di sebelah kiri (Enter atau Klik)\n" +
			"  • Buat file baru dengan tombol 'a' di Explorer\n" +
			"  • Buka buffer switcher cepat via Ctrl+P\n" +
			"  • Tekan ? untuk panduan shortcut lengkap\n"

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
	content := "┌─ BANTUAN & SHORTCUT tui-workflow ────────────────┐\n" +
		"│                                                  │\n" +
		"│  Global:                                         │\n" +
		"│  • Ctrl+Q           : Keluar dari aplikasi       │\n" +
		"│  • Ctrl+E / F6      : Ganti Fokus Sidebar/Editor │\n" +
		"│  • Ctrl+B           : Toggle Sidebar             │\n" +
		"│  • Alt+F, Alt+T     : Buka Top Menu Bar          │\n" +
		"│  • Alt+1, Alt+2     : Switch Sidebar (Files/Git) │\n" +
		"│  • Alt+S            : Toggle Auto-Save           │\n" +
		"│  • Ctrl+P           : Quick Buffer Tab Switcher  │\n" +
		"│  • ?                : Buka/Tutup Bantuan Ini     │\n" +
		"│  • Esc              : Tutup Dialog / Menu        │\n" +
		"│                                                  │\n" +
		"│  Editor (Modeless):                              │\n" +
		"│  • Ketik langsung   : Teks disisipkan ke buffer  │\n" +
		"│  • Panah / PgUp/Dn  : Navigasi Kursor            │\n" +
		"│  • Ctrl+S           : Simpan Buffer Aktif        │\n" +
		"│  • Ctrl+W           : Tutup Tab Aktif            │\n" +
		"│  • Ctrl+N           : Pindah ke Tab Berikutnya   │\n" +
		"│  • Ctrl+M           : Toggle Markdown Preview    │\n" +
		"│  • Tab              : Indentasi 4 Spasi          │\n" +
		"│                                                  │\n" +
		"│  [Tekan Esc atau ? untuk menutup modal ini]      │\n" +
		"└──────────────────────────────────────────────────┘"

	return ui.CurrentTheme.WarningBoxStyle.
		BorderForeground(ui.CurrentTheme.Accent).
		Render(content)
}

func renderAboutModal() string {
	content := "┌─ ABOUT tui-workflow ─────────────────────────────┐\n" +
		"│                                                  │\n" +
		"│  tui-workflow v1.1.0                             │\n" +
		"│  Modern TUI Text Editor & Git Workflow           │\n" +
		"│                                                  │\n" +
		"│  Tech Stack:                                     │\n" +
		"│  • Go 1.24+                                      │\n" +
		"│  • Charmbracelet Bubble Tea & Lipgloss           │\n" +
		"│  • Chroma Syntax Highlighting                    │\n" +
		"│  • Glamour Markdown Rendering                    │\n" +
		"│  • Pure-Go Git (go-git/v5)                       │\n" +
		"│                                                  │\n" +
		"│  [Tekan Esc untuk menutup dialog ini]            │\n" +
		"└──────────────────────────────────────────────────┘"

	return ui.CurrentTheme.WarningBoxStyle.
		BorderForeground(ui.CurrentTheme.Accent).
		Render(content)
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

func placeCenter(base, modal string, totalW, totalH int) string {
	return lipgloss.Place(totalW, totalH, lipgloss.Center, lipgloss.Center, modal)
}
