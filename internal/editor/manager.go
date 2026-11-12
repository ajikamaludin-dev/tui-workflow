package editor

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/git"
	"tui-workflow/internal/terminal"
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
	TabScrollOffset   int
	DefaultSoftWrap   bool
	ClickRegions      []TabClickRegion
	MarkdownBtnStartX int
	MarkdownBtnEndX   int
	ScrollLeftStartX  int
	ScrollLeftEndX    int
	ScrollRightStartX int
	ScrollRightEndX   int
}

func NewTabManager() *TabManager {
	return &TabManager{
		Tabs:              make([]*Tab, 0),
		ActiveIndex:       -1,
		TabScrollOffset:   0,
		DefaultSoftWrap:   true,
		ClickRegions:      make([]TabClickRegion, 0),
		MarkdownBtnStartX: -1,
		MarkdownBtnEndX:   -1,
		ScrollLeftStartX:  -1,
		ScrollLeftEndX:    -1,
		ScrollRightStartX: -1,
		ScrollRightEndX:   -1,
	}
}

// SetSoftWrap updates the default soft-wrap mode and updates all open file tabs.
func (tm *TabManager) SetSoftWrap(enabled bool) {
	tm.DefaultSoftWrap = enabled
	for _, tab := range tm.Tabs {
		if tab.Buffer != nil {
			tab.Buffer.SoftWrap = enabled
		}
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
			tm.EnsureActiveTabVisible()
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
	tab.LastModTime = info.ModTime()
	if tab.Buffer != nil {
		tab.Buffer.SoftWrap = tm.DefaultSoftWrap
	}
	tm.Tabs = append(tm.Tabs, tab)
	tm.ActiveIndex = len(tm.Tabs) - 1
	tm.EnsureActiveTabVisible()
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
	tm.EnsureActiveTabVisible()
	return tab
}

// OpenTerminal adds and activates a new terminal tab.
func (tm *TabManager) OpenTerminal(id, name string, term *terminal.TerminalView) *Tab {
	tab := NewTerminalTab(id, name, term)
	tm.Tabs = append(tm.Tabs, tab)
	tm.ActiveIndex = len(tm.Tabs) - 1
	tm.EnsureActiveTabVisible()
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

	if info, errStat := os.Stat(tab.Path); errStat == nil {
		tab.LastModTime = info.ModTime()
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

	if info, errStat := os.Stat(tab.Path); errStat == nil {
		tab.LastModTime = info.ModTime()
	}

	tab.Buffer.IsDirty = false
	return nil
}

func (tm *TabManager) CloseTab(idx int) {
	if idx < 0 || idx >= len(tm.Tabs) {
		return
	}
	tabToClose := tm.Tabs[idx]
	if tabToClose.Type == TabTerminal && tabToClose.TerminalView != nil {
		_ = tabToClose.TerminalView.Close()
	}

	tm.Tabs = append(tm.Tabs[:idx], tm.Tabs[idx+1:]...)
	if tm.ActiveIndex >= len(tm.Tabs) {
		tm.ActiveIndex = len(tm.Tabs) - 1
	}
	if tm.ActiveIndex < 0 && len(tm.Tabs) > 0 {
		tm.ActiveIndex = 0
	}
	if tm.TabScrollOffset >= len(tm.Tabs) && len(tm.Tabs) > 0 {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	} else if len(tm.Tabs) == 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseOtherTabs closes all tabs except specified index.
func (tm *TabManager) CloseOtherTabs(keepIdx int) {
	if keepIdx < 0 || keepIdx >= len(tm.Tabs) {
		return
	}
	for i, tab := range tm.Tabs {
		if i != keepIdx && tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	target := tm.Tabs[keepIdx]
	tm.Tabs = []*Tab{target}
	tm.ActiveIndex = 0
	tm.TabScrollOffset = 0
}

// CloseTabsToRight closes all tabs right target index.
func (tm *TabManager) CloseTabsToRight(idx int) {
	if idx < 0 || idx >= len(tm.Tabs) {
		return
	}
	for i := idx + 1; i < len(tm.Tabs); i++ {
		tab := tm.Tabs[i]
		if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = tm.Tabs[:idx+1]
	if tm.ActiveIndex > idx {
		tm.ActiveIndex = idx
	}
	if tm.TabScrollOffset >= len(tm.Tabs) {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseTabsToLeft closes all tabs left target index.
func (tm *TabManager) CloseTabsToLeft(idx int) {
	if idx <= 0 || idx >= len(tm.Tabs) {
		return
	}
	for i := 0; i < idx; i++ {
		tab := tm.Tabs[i]
		if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = append([]*Tab(nil), tm.Tabs[idx:]...)
	if tm.ActiveIndex < idx {
		tm.ActiveIndex = 0
	} else {
		tm.ActiveIndex -= idx
	}
	if tm.TabScrollOffset >= len(tm.Tabs) {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseAllTabs closes all tabs.
func (tm *TabManager) CloseAllTabs() {
	for _, tab := range tm.Tabs {
		if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = make([]*Tab, 0)
	tm.ActiveIndex = 0
	tm.TabScrollOffset = 0
}

// FindTabAt returns tab index at clickX, or -1 if not on any tab.
func (tm *TabManager) FindTabAt(clickX int) int {
	for _, reg := range tm.ClickRegions {
		if clickX >= reg.StartX && clickX <= reg.EndX {
			return reg.TabIdx
		}
	}
	return -1
}

func (tm *TabManager) ScrollTabsLeft() {
	if tm.TabScrollOffset > 0 {
		tm.TabScrollOffset--
	}
}

func (tm *TabManager) ScrollTabsRight() {
	if tm.TabScrollOffset < len(tm.Tabs)-1 {
		tm.TabScrollOffset++
	}
}

func (tm *TabManager) EnsureActiveTabVisible() {
	if tm.ActiveIndex < 0 || tm.ActiveIndex >= len(tm.Tabs) {
		return
	}
	if tm.TabScrollOffset > tm.ActiveIndex {
		tm.TabScrollOffset = tm.ActiveIndex
	}
}

func (tm *TabManager) NextTab() {
	if len(tm.Tabs) > 1 {
		tm.ActiveIndex = (tm.ActiveIndex + 1) % len(tm.Tabs)
		tm.EnsureActiveTabVisible()
	}
}

func (tm *TabManager) PrevTab() {
	if len(tm.Tabs) > 1 {
		tm.ActiveIndex--
		if tm.ActiveIndex < 0 {
			tm.ActiveIndex = len(tm.Tabs) - 1
		}
		tm.EnsureActiveTabVisible()
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
	if tm.ScrollLeftStartX >= 0 && clickX >= tm.ScrollLeftStartX && clickX <= tm.ScrollLeftEndX {
		return "scroll_left", -1
	}
	if tm.ScrollRightStartX >= 0 && clickX >= tm.ScrollRightStartX && clickX <= tm.ScrollRightEndX {
		return "scroll_right", -1
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
	tm.ScrollLeftStartX = -1
	tm.ScrollLeftEndX = -1
	tm.ScrollRightStartX = -1
	tm.ScrollRightEndX = -1

	if len(tm.Tabs) == 0 {
		return ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render(" (Tidak ada file terbuka)")
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
	rightButtonWidth := ansi.StringWidth(rightButton)

	availWidth := width - rightButtonWidth
	if availWidth < 10 {
		availWidth = 10
	}

	// 1. Fixed max title length per tab (clean limit, truncate with .. if longer)
	const maxTabTitleLen = 16

	// 2. Pre-render tabs and record widths
	type renderedTabInfo struct {
		Index    int
		Rendered string
		Width    int
	}
	renderedTabs := make([]renderedTabInfo, len(tm.Tabs))
	for i, t := range tm.Tabs {
		rawTitle := t.Title()
		if ansi.StringWidth(rawTitle) > maxTabTitleLen {
			rawTitle = ansi.Truncate(rawTitle, maxTabTitleLen, "..")
		}
		titleWithClose := rawTitle + " ×"
		var r string
		if i == tm.ActiveIndex {
			r = ui.CurrentTheme.TabActiveStyle.Render(titleWithClose)
		} else {
			r = ui.CurrentTheme.TabInactiveStyle.Render(titleWithClose)
		}
		renderedTabs[i] = renderedTabInfo{
			Index:    i,
			Rendered: r,
			Width:    ansi.StringWidth(r),
		}
	}

	// 3. Ensure TabScrollOffset is within valid bounds
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
	if len(renderedTabs) > 0 && tm.TabScrollOffset >= len(renderedTabs) {
		tm.TabScrollOffset = len(renderedTabs) - 1
	}

	startIdx := tm.TabScrollOffset

	var sb strings.Builder
	currX := 0

	if startIdx > 0 {
		tm.ScrollLeftStartX = currX
		tm.ScrollLeftEndX = currX + 1
		sb.WriteString("< ")
		currX += 2
	}

	hasMoreRight := false
	for i := startIdx; i < len(renderedTabs); i++ {
		rt := renderedTabs[i]
		reservedRight := 0
		if i < len(renderedTabs)-1 {
			reservedRight = 2 // " >"
		}
		if currX+rt.Width > availWidth-reservedRight {
			hasMoreRight = true
			break
		}

		startX := currX
		endX := startX + rt.Width - 1
		closeX := endX - 2 // Position of '×'

		tm.ClickRegions = append(tm.ClickRegions, TabClickRegion{
			TabIdx: rt.Index,
			StartX: startX,
			EndX:   endX,
			CloseX: closeX,
		})

		sb.WriteString(rt.Rendered)
		currX += rt.Width
	}

	if hasMoreRight {
		tm.ScrollRightStartX = currX
		tm.ScrollRightEndX = currX + 1
		sb.WriteString(" >")
		currX += 2
	}

	barContent := sb.String()
	barLen := ansi.StringWidth(barContent)
	if barLen < availWidth {
		barContent += strings.Repeat(" ", availWidth-barLen)
	}

	if rightButton != "" {
		tm.MarkdownBtnStartX = availWidth
		tm.MarkdownBtnEndX = availWidth + rightButtonWidth - 1
		barContent += rightButton
	}

	if ansi.StringWidth(barContent) > width {
		barContent = ansi.Truncate(barContent, width, "")
	}

	return ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render(barContent)
}
