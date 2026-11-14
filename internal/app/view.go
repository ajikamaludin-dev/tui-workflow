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
		baseView = overlayAt(baseView, barStr, findX, 1, m.Width, m.Height)
	}

	// Overlay 3: Context Menu
	if m.ContextMenu != nil && m.ContextMenu.Active {
		menuStr := m.ContextMenu.Render()
		baseView = overlayAt(baseView, menuStr, m.ContextMenu.X, m.ContextMenu.Y, m.Width, m.Height)
	}

	lines := strings.Split(baseView, "\n")
	if len(lines) > m.Height {
		baseView = strings.Join(lines[:m.Height], "\n")
	}

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
				Render(ansi.Truncate(" 🔍 "+m.Explorer.FilterQuery+" (Esc:Batal)", width-1, ""))
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
			body = "  Bukan repositori Git"
		}
	} else if m.SidebarView == SidebarSearch {
		if m.SearchView != nil {
			body = m.SearchView.Render(width-1, bodyHeight, m.Focus == FocusSidebar)
		} else {
			body = "  Search view belum siap"
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
		bodyContent := "\n  Selamat Datang di tui-workflow (v0.1.1)\n\n" +
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
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("📖 BANTUAN & PANDUAN PINTASAN KEYBOARD")

	content := `Global & Navigasi:
  • Ctrl+Q          : Keluar dari aplikasi
  • Ctrl+E / F6     : Ganti Fokus Sidebar / Editor
  • Ctrl+B          : Toggle Sembunyikan / Tampilkan Sidebar
  • Alt+[ / Alt+]   : Persempit / Perlebar Lebar Kolom Sidebar
  • Alt+1 / 2 / 3   : Beralih Tab Sidebar (Files / Git / Search)
  • Alt+F / Alt+E   : Buka Menu File / Edit
  • Alt+R / Alt+T   : Buka Menu Search / Setting
  • Alt+A           : Buka Dialog About
  • Alt+S / Alt+Z   : Toggle Auto-Save / Soft-Wrap
  • Alt+I           : Pengaturan AI Model (OpenAI API Format)
  • ? / Esc         : Buka Bantuan / Tutup Dialog Pop-up

Pencarian & Navigasi Kode:
  • F1 / Ctrl+Shift+P : Buka Command Palette Lengkap
  • Ctrl+P          : Buka Cepat File Workspace (Quick Open)
  • Ctrl+Shift+O    : Go to Symbol (workspace)
  • F12 / Shift+F12 : Go to Definition / Find References
  • Ctrl+F / Ctrl+H : Cari / Cari & Ganti Teks di Buffer
  • Ctrl+G          : Lompat ke Nomor Baris (Go to Line)
  • F4              : Cari Teks Global di Seluruh Workspace
  • Ctrl+O          : Buka File Baru dari Disk (File Picker)
  • Ctrl+K, Ctrl+K  : Buka Direktori / Folder Baru di Workspace

Editor & Buffer:
  • Ctrl+Z / Ctrl+Y : Undo / Redo Perubahan Teks
  • Tab / Shift+Tab : Indent / Outdent 4 Karakter Spasi
  • Ctrl+/          : Toggle Komentar Baris Otomatis
  • Ctrl+Shift+D    : Tambah kursor pada occurrence berikutnya
  • Ctrl+Alt+Up/Dn  : Tambah kursor di baris atas / bawah
  • Ctrl+Shift+[ / ]: Fold / Unfold blok di kursor
  • Ctrl+K Ctrl+0/J : Fold All / Unfold All
  • Klik Gutter Fold: Toggle lipat blok (marker ▸/▾)
  • Ctrl+S / Ctrl+W : Simpan Buffer / Tutup Tab Aktif
  • Ctrl+Shift+I    : Format Dokumen Otomatis (via CLI)
  • Ctrl+M / Alt+M  : Toggle Pratinjau Markdown (Glamour)
  • Ctrl+A / C / V  : Pilih Semua / Salin / Tempel Teks

Terminal & Shell (PTY):
  • Ctrl+Shift+\    : Buka Tab Terminal Shell Baru
  • Ctrl+\          : Alihkan Fokus Terminal / TUI Editor
  • Ctrl+Shift+W    : Tutup Tab Terminal Shell Aktif

Git Source Control:
  • Alt+B           : Ganti / Checkout Branch Git
  • Alt+G           : Buat Pesan Commit Otomatis via AI
  • d / [x]         : Buang Perubahan File (Discard Changes)
  • + / -           : Stage / Unstage Perubahan File
  • Ctrl+Enter      : Lakukan Commit Pesan Git`

	closeBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Padding(0, 2).
		MarginTop(1).
		Render("[ Tutup (Esc / ?) ]")

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

	renderedTitle := titleStyle.Render("ℹ TENTANG tui-workflow")

	content := `tui-workflow v0.1.1
Modern, High-Performance Terminal User Interface Workflow Environment.

Fitur Inti & Teknologi:
  • Bahasa Pemrograman : Go 1.24+ (Goroutines, Channels)
  • Terminal Engine    : Charmbracelet Bubble Tea & Lipgloss
  • Syntax Tokenizer   : Chroma v2 Highlighting Cache
  • Markdown Renderer  : Glamour ANSI Stylesheet
  • Git Integration    : In-Memory Pure-Go (go-git/v5)
  • PTY Terminal       : Embedded Pseudoterminal Subshell

Dirancang dengan standar 60 FPS, zero flickering, modeless text
editing, dan mouse support penuh di lingkungan terminal.`

	closeBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		Background(ui.CurrentTheme.Accent).
		Padding(0, 2).
		MarginTop(1).
		Render("[ Tutup (Esc) ]")

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
		body := ui.CurrentTheme.SidebarStyle.Render("  (tidak ada simbol)")
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
