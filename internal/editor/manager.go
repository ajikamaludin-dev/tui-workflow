package editor

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui"
)

const (
	MaxFileSize = 10 * 1024 * 1024 // 10 MB
)

type TabClickRegion struct {
	TabIdx int
	StartX int
	EndX   int
	CloseX int // X coordinate where [x] button is located
}

type TabManager struct {
	Tabs              []*Tab
	ActiveIndex       int
	ClickRegions      []TabClickRegion
	MarkdownBtnStartX int
	MarkdownBtnEndX   int
}

func NewTabManager() *TabManager {
	return &TabManager{
		Tabs:              make([]*Tab, 0),
		ActiveIndex:       -1,
		ClickRegions:      make([]TabClickRegion, 0),
		MarkdownBtnStartX: -1,
		MarkdownBtnEndX:   -1,
	}
}

func (tm *TabManager) Count() int {
	return len(tm.Tabs)
}

func (tm *TabManager) ActiveTab() *Tab {
	if tm.ActiveIndex >= 0 && tm.ActiveIndex < len(tm.Tabs) {
		return tm.Tabs[tm.ActiveIndex]
	}
	return nil
}

func (tm *TabManager) OpenFile(path string) (*Tab, error) {
	// 1. Check if already open
	for i, t := range tm.Tabs {
		if t.Path == path && !t.IsDiff {
			tm.ActiveIndex = i
			return t, nil
		}
	}

	// 2. Stat file & check size limit (10MB)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca info file: %w", err)
	}

	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("ukuran file (%d MB) melebihi batas 10MB", info.Size()/(1024*1024))
	}

	// 3. Binary guard (check null byte in first 512 bytes)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file: %w", err)
	}
	defer f.Close()

	header := make([]byte, 512)
	n, _ := f.Read(header)
	if bytes.IndexByte(header[:n], 0x00) != -1 {
		return nil, fmt.Errorf("file biner tidak didukung")
	}

	// 4. Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca isi file: %w", err)
	}

	tab := NewFileTab(path, string(content))
	tm.Tabs = append(tm.Tabs, tab)
	tm.ActiveIndex = len(tm.Tabs) - 1
	return tab, nil
}

func (tm *TabManager) OpenDiff(path, original, modified string) *Tab {
	diffID := "diff:" + path
	for i, t := range tm.Tabs {
		if t.ID == diffID {
			t.DiffView = git.NewDiffView(path, original, modified)
			tm.ActiveIndex = i
			return t
		}
	}

	tab := NewDiffTab(path, original, modified)
	tm.Tabs = append(tm.Tabs, tab)
	tm.ActiveIndex = len(tm.Tabs) - 1
	return tab
}

func (tm *TabManager) SaveActive() error {
	tab := tm.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return nil
	}

	err := os.WriteFile(tab.Path, []byte(tab.Buffer.Text()), 0644)
	if err != nil {
		return fmt.Errorf("gagal menyimpan file: %w", err)
	}

	tab.Buffer.IsDirty = false
	return nil
}

func (tm *TabManager) SaveTab(idx int) error {
	if idx < 0 || idx >= len(tm.Tabs) {
		return nil
	}
	tab := tm.Tabs[idx]
	if tab.Buffer == nil {
		return nil
	}

	err := os.WriteFile(tab.Path, []byte(tab.Buffer.Text()), 0644)
	if err != nil {
		return fmt.Errorf("gagal menyimpan file '%s': %w", tab.Name, err)
	}

	tab.Buffer.IsDirty = false
	return nil
}

func (tm *TabManager) CloseTab(idx int) {
	if idx < 0 || idx >= len(tm.Tabs) {
		return
	}

	tm.Tabs = append(tm.Tabs[:idx], tm.Tabs[idx+1:]...)
	if tm.ActiveIndex >= len(tm.Tabs) {
		tm.ActiveIndex = len(tm.Tabs) - 1
	}
	if tm.ActiveIndex < 0 && len(tm.Tabs) > 0 {
		tm.ActiveIndex = 0
	}
}

func (tm *TabManager) NextTab() {
	if len(tm.Tabs) > 1 {
		tm.ActiveIndex = (tm.ActiveIndex + 1) % len(tm.Tabs)
	}
}

func (tm *TabManager) PrevTab() {
	if len(tm.Tabs) > 1 {
		tm.ActiveIndex--
		if tm.ActiveIndex < 0 {
			tm.ActiveIndex = len(tm.Tabs) - 1
		}
	}
}

func (tm *TabManager) ToggleActiveMarkdown() bool {
	tab := tm.ActiveTab()
	if tab != nil && tab.Type == TabMarkdown {
		tab.MarkdownPreview = !tab.MarkdownPreview
		return true
	}
	return false
}

func (tm *TabManager) HandleTabClick(clickX int) (action string, tabIdx int) {
	if tm.MarkdownBtnStartX >= 0 && clickX >= tm.MarkdownBtnStartX && clickX <= tm.MarkdownBtnEndX {
		return "toggle_markdown", -1
	}

	for _, reg := range tm.ClickRegions {
		if clickX >= reg.StartX && clickX <= reg.EndX {
			if clickX >= reg.CloseX && clickX <= reg.EndX {
				return "close", reg.TabIdx
			}
			return "activate", reg.TabIdx
		}
	}
	return "", -1
}

func (tm *TabManager) RenderTabBar(width int) string {
	tm.ClickRegions = make([]TabClickRegion, 0)
	tm.MarkdownBtnStartX = -1
	tm.MarkdownBtnEndX = -1

	if len(tm.Tabs) == 0 {
		return ui.CurrentTheme.TabBarContainer.Width(width).Render(" (Tidak ada file terbuka)")
	}

	var rightButton string
	activeTab := tm.ActiveTab()
	if activeTab != nil && activeTab.Type == TabMarkdown {
		if activeTab.MarkdownPreview {
			rightButton = ui.CurrentTheme.StatusBarBadgeStyle.Render(" [Raw] ")
		} else {
			rightButton = ui.CurrentTheme.StatusBarBadgeStyle.Render(" [Preview] ")
		}
	}
	rightButtonWidth := lipgloss.Width(rightButton)

	availWidth := width - rightButtonWidth
	if availWidth < 10 {
		availWidth = 10
	}

	maxTabTitleLen := 18
	if len(tm.Tabs)*14 > availWidth && len(tm.Tabs) > 0 {
		maxTabTitleLen = (availWidth / len(tm.Tabs)) - 6
		if maxTabTitleLen < 4 {
			maxTabTitleLen = 4
		}
	}

	var sb strings.Builder
	currX := 0

	for i, t := range tm.Tabs {
		rawTitle := t.Title()
		if len(rawTitle) > maxTabTitleLen {
			rawTitle = rawTitle[:maxTabTitleLen-2] + ".."
		}

		titleWithClose := rawTitle + " ×"
		startX := currX

		var renderedTab string
		if i == tm.ActiveIndex {
			renderedTab = ui.CurrentTheme.TabActiveStyle.Render(titleWithClose)
		} else {
			renderedTab = ui.CurrentTheme.TabInactiveStyle.Render(titleWithClose)
		}

		tabW := lipgloss.Width(renderedTab)
		endX := startX + tabW - 1
		closeX := endX - 2 // Position of '×'

		tm.ClickRegions = append(tm.ClickRegions, TabClickRegion{
			TabIdx: i,
			StartX: startX,
			EndX:   endX,
			CloseX: closeX,
		})

		sb.WriteString(renderedTab)
		currX = endX + 1
	}

	barContent := sb.String()
	barLen := lipgloss.Width(barContent)
	if barLen < availWidth {
		barContent += strings.Repeat(" ", availWidth-barLen)
	}

	if rightButton != "" {
		tm.MarkdownBtnStartX = availWidth
		tm.MarkdownBtnEndX = availWidth + rightButtonWidth - 1
		barContent += rightButton
	}

	return ui.CurrentTheme.TabBarContainer.Width(width).Render(barContent)
}
