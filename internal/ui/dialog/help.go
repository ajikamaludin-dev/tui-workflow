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
		"Global & Navigasi:",
		"  • Ctrl+Q            : Keluar dari aplikasi (simpan session; konfirmasi jika dirty)",
		"  • Ctrl+E / F6       : Ganti Fokus Sidebar / Editor",
		"  • Ctrl+B            : Toggle Sembunyikan / Tampilkan Sidebar",
		"  • Alt+[ / Alt+]     : Persempit / Perlebar Lebar Kolom Sidebar",
		"  • Alt+1 / 2 / 3     : Beralih Tab Sidebar (Files / Git / Search)",
		"  • Alt+F / Alt+E     : Buka Menu Top Bar File / Edit",
		"  • Alt+R / Alt+T     : Buka Menu Top Bar Search / Setting",
		"  • Alt+A             : Buka Dialog About",
		"  • Alt+S             : Toggle Auto-Save on Focus Change",
		"  • Alt+Z             : Toggle Soft-Wrap pada Editor",
		"  • Alt+I             : Pengaturan AI Model (OpenAI API Format)",
		"  • ? / Esc           : Buka Bantuan / Tutup Dialog Pop-up",
		"",
		"Pencarian & File:",
		"  • F1 / Ctrl+Shift+P : Buka Command Palette Lengkap",
		"  • Ctrl+P            : Buka Cepat File Workspace (Quick Open)",
		"  • Ctrl+F            : Cari Teks di Buffer Editor (Find Bar)",
		"  • Ctrl+H            : Cari & Ganti Teks di Buffer (Find & Replace)",
		"  • Ctrl+G            : Lompat ke Nomor Baris (Go to Line)",
		"  • F4                : Cari Teks Global di Seluruh Workspace",
		"  • Ctrl+O            : Buka File Baru dari Disk (File Picker)",
		"  • Ctrl+K            : Buka Direktori / Folder Baru di Workspace",
		"",
		"Editor & Buffer:",
		"  • Ctrl+Z / Ctrl+Y   : Undo / Redo Perubahan Teks",
		"  • Tab / Shift+Tab   : Indent / Outdent 4 Karakter Spasi",
		"  • Ctrl+/            : Toggle Komentar Baris Otomatis",
		"  • Ctrl+S            : Simpan Buffer Aktif ke Disk",
		"  • Ctrl+W            : Tutup Tab Aktif / Diff Tab",
		"  • Middle-Click Tab  : Tutup Tab yang Diklik",
		"  • Ctrl+Shift+I      : Format Dokumen Otomatis (CLI Formatter)",
		"  • Ctrl+M / Alt+M    : Toggle Pratinjau Markdown (Glamour) / Raw",
		"  • Ctrl+A / C / V    : Pilih Semua / Salin / Tempel Teks",
		"  • Klik Kanan Editor : Buka Context Menu Edit (Salin/Tempel/Potong)",
		"",
		"File Explorer:",
		"  • j / k / Panah     : Sorot Node Berikutnya / Sebelumnya",
		"  • Enter / l         : Buka File / Expand Folder",
		"  • h / Left          : Collapse Folder atau Lompat ke Parent",
		"  • a / A             : Buat File Baru / Buat Folder Baru",
		"  • F2                : Ubah Nama (Rename) File atau Folder",
		"  • d                 : Hapus File atau Folder Terpilih",
		"  • /                 : Filter Real-Time Pohon File (Enter kosong: bersihkan)",
		"  • .                 : Toggle Tampilkan / Sembunyikan Dotfiles",
		"  • Klik Kanan Node   : Buka Context Menu Explorer (CRUD & Copy Path)",
		"",
		"Git Source Control (Sidebar):",
		"  • Alt+B             : Ganti / Checkout Branch Git Lokal",
		"  • Alt+G / [✨ AI]   : Buat Pesan Commit Otomatis via AI",
		"  • + / -             : Stage / Unstage Semua Perubahan File",
		"  • [ / ]             : Navigasi Hunk Aktif (file terpilih)",
		"  • s / u             : Stage / Unstage Hunk Aktif (Changes/Staged)",
		"  • x                 : Discard Hunk Aktif (Unstaged, konfirmasi)",
		"  • Enter             : Buka Diff Tab File Terpilih",
		"  • Tab / ↑↓ / j k    : Pindah Fokus Section & Navigasi Item",
		"  • Ctrl+Enter        : Lakukan Commit Pesan Git",
		"  • Klik Push         : Push Perubahan ke Remote Repository",
		"",
		"Diff Tab (Side-by-Side, Git Hunk):",
		"  • n / ] / N / [     : Lompat Hunk Berikutnya / Sebelumnya",
		"  • s / +             : Stage Hunk Aktif (pada Diff Unstaged)",
		"  • u / -             : Unstage Hunk Aktif (pada Diff Staged)",
		"  • x                 : Discard Hunk Aktif (Unstaged, konfirmasi)",
		"  • ↑↓ / j k / PgUpDn : Scroll Tampilan Diff",
		"  • Ctrl+W            : Tutup Tab Diff",
		"",
		"Terminal & Shell (PTY):",
		"  • Ctrl+Shift+\\ / Ctrl+T : Buka Tab Terminal Shell Baru",
		"  • Ctrl+\\ / Ctrl+` / F6  : Alihkan Fokus Terminal / Sidebar",
		"  • Alt+W / Ctrl+Shift+W  : Tutup Tab Terminal Aktif",
		"  • Mouse Wheel            : Scroll Riwayat Terminal",
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
	renderedTitle := titleStyle.Render("📖 BANTUAN & PANDUAN PINTASAN KEYBOARD")

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
		Render("[ Tutup (Esc / ?) ]")

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
