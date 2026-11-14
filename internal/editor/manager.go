package editor

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	DraggingIndex     int
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
		DraggingIndex:     -1,
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
		return nil, fmt.Errorf("failed to read file info: %w", err)
	}

	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("file size (%d MB) exceeds 10MB limit", info.Size()/(1024*1024))
	}

	// 3. Binary guard (check null byte in first 512 bytes)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	header := make([]byte, 512)
	n, _ := f.Read(header)
	if bytes.IndexByte(header[:n], 0x00) != -1 {
		return nil, fmt.Errorf("binary file is not supported")
	}

	// 4. Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
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

func (tm *TabManager) OpenDiff(path, original, modified string, staged bool, title string) *Tab {
	for i, t := range tm.Tabs {
		if t.Type == TabDiff && t.Path == path && t.DiffStaged == staged {
			t.DiffView = git.NewDiffView(path, original, modified)
			if title != "" {
				t.Name = title
			}
			t.DiffHunkPath = path
			tm.ActiveIndex = i
			tm.EnsureActiveTabVisible()
			return t
		}
	}
	tab := NewDiffTab(path, original, modified)
	tab.DiffStaged = staged
	tab.DiffHunkPath = path
	if title != "" {
		tab.Name = title
		tab.ID = fmt.Sprintf("diff:%s:%t", path, staged)
	} else {
		tab.ID = fmt.Sprintf("diff:%s:%t", path, staged)
	}
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
		return fmt.Errorf("failed to save file: %w", err)
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
		return fmt.Errorf("failed to save file '%s': %w", tab.Name, err)
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

// CloseOtherTabs closes all unpinned tabs except specified index.
func (tm *TabManager) CloseOtherTabs(keepIdx int) {
	if keepIdx < 0 || keepIdx >= len(tm.Tabs) {
		return
	}
	target := tm.Tabs[keepIdx]
	var newTabs []*Tab
	for i, tab := range tm.Tabs {
		if i == keepIdx || tab.IsPinned {
			newTabs = append(newTabs, tab)
		} else if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = newTabs
	newActive := 0
	for i, tab := range tm.Tabs {
		if tab == target {
			newActive = i
			break
		}
	}
	tm.ActiveIndex = newActive
	if tm.TabScrollOffset >= len(tm.Tabs) {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseTabsToRight closes all unpinned tabs to right of target index.
func (tm *TabManager) CloseTabsToRight(idx int) {
	if idx < 0 || idx >= len(tm.Tabs) {
		return
	}
	var newTabs []*Tab
	for i, tab := range tm.Tabs {
		if i <= idx || tab.IsPinned {
			newTabs = append(newTabs, tab)
		} else if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = newTabs
	if tm.ActiveIndex >= len(tm.Tabs) {
		tm.ActiveIndex = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset >= len(tm.Tabs) {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseTabsToLeft closes all unpinned tabs to left of target index.
func (tm *TabManager) CloseTabsToLeft(idx int) {
	if idx <= 0 || idx >= len(tm.Tabs) {
		return
	}
	target := tm.Tabs[idx]
	var newTabs []*Tab
	for i, tab := range tm.Tabs {
		if i >= idx || tab.IsPinned {
			newTabs = append(newTabs, tab)
		} else if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = newTabs
	newActive := 0
	for i, tab := range tm.Tabs {
		if tab == target {
			newActive = i
			break
		}
	}
	tm.ActiveIndex = newActive
	if tm.TabScrollOffset >= len(tm.Tabs) {
		tm.TabScrollOffset = len(tm.Tabs) - 1
	}
	if tm.TabScrollOffset < 0 {
		tm.TabScrollOffset = 0
	}
}

// CloseAllTabs closes all unpinned tabs.
func (tm *TabManager) CloseAllTabs() {
	var newTabs []*Tab
	for _, tab := range tm.Tabs {
		if tab.IsPinned {
			newTabs = append(newTabs, tab)
		} else if tab.Type == TabTerminal && tab.TerminalView != nil {
			_ = tab.TerminalView.Close()
		}
	}
	tm.Tabs = newTabs
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

// PinnedCount returns number of pinned tabs.
func (tm *TabManager) PinnedCount() int {
	count := 0
	for _, t := range tm.Tabs {
		if t.IsPinned {
			count++
		}
	}
	return count
}

// IsTabPinned returns whether tab at idx is pinned.
func (tm *TabManager) IsTabPinned(idx int) bool {
	if idx >= 0 && idx < len(tm.Tabs) {
		return tm.Tabs[idx].IsPinned
	}
	return false
}

func (tm *TabManager) moveRawTab(from, to int) {
	if from == to || from < 0 || from >= len(tm.Tabs) || to < 0 || to >= len(tm.Tabs) {
		return
	}
	tab := tm.Tabs[from]
	tm.Tabs = append(tm.Tabs[:from], tm.Tabs[from+1:]...)
	tm.Tabs = append(tm.Tabs[:to], append([]*Tab{tab}, tm.Tabs[to:]...)...)

	if tm.ActiveIndex == from {
		tm.ActiveIndex = to
	} else if from < tm.ActiveIndex && to >= tm.ActiveIndex {
		tm.ActiveIndex--
	} else if from > tm.ActiveIndex && to <= tm.ActiveIndex {
		tm.ActiveIndex++
	}
}

// MoveTab moves tab from from index to to index, respecting pinned grouping.
func (tm *TabManager) MoveTab(from, to int) bool {
	if from < 0 || from >= len(tm.Tabs) || to < 0 || to >= len(tm.Tabs) || from == to {
		return false
	}
	fromTab := tm.Tabs[from]
	if fromTab.IsPinned {
		lastPinned := 0
		for i, t := range tm.Tabs {
			if t.IsPinned {
				lastPinned = i
			}
		}
		if to > lastPinned {
			to = lastPinned
		}
	} else {
		firstUnpinned := len(tm.Tabs) - 1
		for i, t := range tm.Tabs {
			if !t.IsPinned {
				firstUnpinned = i
				break
			}
		}
		if to < firstUnpinned {
			to = firstUnpinned
		}
	}
	if from == to {
		return false
	}
	tm.moveRawTab(from, to)
	return true
}

// TogglePinTab toggles the pinned status of tab at idx and keeps pinned tabs grouped on the left.
func (tm *TabManager) TogglePinTab(idx int) bool {
	if idx < 0 || idx >= len(tm.Tabs) {
		return false
	}
	tab := tm.Tabs[idx]
	tab.IsPinned = !tab.IsPinned

	numPinned := tm.PinnedCount()
	var targetIdx int
	if tab.IsPinned {
		targetIdx = numPinned - 1
	} else {
		targetIdx = numPinned
	}

	if targetIdx != idx && targetIdx >= 0 && targetIdx < len(tm.Tabs) {
		tm.moveRawTab(idx, targetIdx)
	}
	tm.EnsureActiveTabVisible()
	return true
}

func (tm *TabManager) SetActiveIndex(idx int) bool {
	if idx < 0 || idx >= len(tm.Tabs) {
		return false
	}
	tm.ActiveIndex = idx
	tm.EnsureActiveTabVisible()
	return true
}


func (tm *TabManager) FindTabByDiffFile(path string, staged bool) int {
	for i, t := range tm.Tabs {
		if t.Type == TabDiff && t.Path == path && t.DiffStaged == staged {
			return i
		}
	}
	return -1
}

func (tm *TabManager) OpenFileAt(path string, line, col int) (*Tab, error) {
	tab, err := tm.OpenFile(path)
	if err != nil || tab == nil || tab.Buffer == nil {
		return tab, err
	}
	if line > 0 {
		tab.Buffer.CursorRow = line - 1
	}
	if col > 0 {
		tab.Buffer.CursorCol = col - 1
	}
	tab.Buffer.ClampCursor()
	return tab, nil
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
			if reg.CloseX >= 0 && clickX >= reg.CloseX && clickX <= reg.EndX {
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
		emptyRow1 := ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render(" (No open files)")
		emptyRow2 := ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render("")
		return emptyRow1 + "\n" + emptyRow2
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
		var titleContent string
		if t.IsPinned {
			titleContent = "📌 " + rawTitle
		} else {
			titleContent = rawTitle + " ×"
		}

		var r string
		if i == tm.DraggingIndex {
			r = ui.CurrentTheme.TabActiveStyle.Underline(true).Render(titleContent)
		} else if i == tm.ActiveIndex {
			r = ui.CurrentTheme.TabActiveStyle.Render(titleContent)
		} else {
			r = ui.CurrentTheme.TabInactiveStyle.Render(titleContent)
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
	var sb2 strings.Builder
	currX := 0

	if startIdx > 0 {
		tm.ScrollLeftStartX = currX
		tm.ScrollLeftEndX = currX + 1
		sb.WriteString("< ")
		sb2.WriteString("  ")
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
		closeX := -1
		if !tm.Tabs[rt.Index].IsPinned {
			closeX = endX - 2 // Position of '×'
		}

		tm.ClickRegions = append(tm.ClickRegions, TabClickRegion{
			TabIdx: rt.Index,
			StartX: startX,
			EndX:   endX,
			CloseX: closeX,
		})

		sb.WriteString(rt.Rendered)
		if rt.Index == tm.ActiveIndex {
			activeIndicator := strings.Repeat("━", rt.Width)
			activeIndicatorStyle := lipgloss.NewStyle().
				Foreground(ui.CurrentTheme.Accent).
				Background(ui.CurrentTheme.BgPrimary)
			sb2.WriteString(activeIndicatorStyle.Render(activeIndicator))
		} else {
			sb2.WriteString(ui.CurrentTheme.TabBarContainer.Render(strings.Repeat(" ", rt.Width)))
		}
		currX += rt.Width
	}

	if hasMoreRight {
		tm.ScrollRightStartX = currX
		tm.ScrollRightEndX = currX + 1
		sb.WriteString(" >")
		sb2.WriteString("  ")
		currX += 2
	}

	barContent := sb.String()
	barLen := ansi.StringWidth(barContent)
	barContent2 := sb2.String()
	barLen2 := ansi.StringWidth(barContent2)

	if barLen < availWidth {
		barContent += strings.Repeat(" ", availWidth-barLen)
	}
	if barLen2 < availWidth {
		barContent2 += strings.Repeat(" ", availWidth-barLen2)
	}

	if rightButton != "" {
		tm.MarkdownBtnStartX = availWidth
		tm.MarkdownBtnEndX = availWidth + rightButtonWidth - 1
		barContent += rightButton
		barContent2 += strings.Repeat(" ", rightButtonWidth)
	}

	if ansi.StringWidth(barContent) > width {
		barContent = ansi.Truncate(barContent, width, "")
	}
	if ansi.StringWidth(barContent2) > width {
		barContent2 = ansi.Truncate(barContent2, width, "")
	}

	row1 := ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render(barContent)
	row2 := ui.CurrentTheme.TabBarContainer.Width(width).MaxHeight(1).Render(barContent2)
	return row1 + "\n" + row2
}
