package dialog

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"tui-workflow/internal/ignore"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// QuickOpenItem represents a file or open tab in Quick Open.
type QuickOpenItem struct {
	TabIndex int // -1 if not an open tab
	Title    string
	RelPath  string
	FullPath string
	IsDirty  bool
	IsOpen   bool
	Score    int
}

// QuickOpen provides a VS Code style Ctrl+P file switcher for both tabs and workspace files.
type QuickOpen struct {
	Active       bool
	AllItems     []QuickOpenItem
	Filtered     []QuickOpenItem
	Selected     int
	ScrollTop    int
	Filter       textinput.Model
	Width        int
	MaxRows      int
	RootDir      string
	openTabPaths map[string]int

	cachedFiles   []QuickOpenItem
	cachedRootDir string
	matcher       *ignore.Matcher
}

// NewQuickOpen creates a new QuickOpen instance.
func NewQuickOpen() *QuickOpen {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 64
	ti.SetWidth(48)
	applyTextInputStyles(&ti, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))

	return &QuickOpen{
		Active:       false,
		AllItems:     make([]QuickOpenItem, 0),
		Filtered:     make([]QuickOpenItem, 0),
		Selected:     0,
		ScrollTop:    0,
		Filter:       ti,
		Width:        64,
		MaxRows:      10,
		openTabPaths: make(map[string]int),
	}
}

// InvalidateCache clears the cached workspace files list.
func (q *QuickOpen) InvalidateCache() {
	q.cachedFiles = nil
	q.cachedRootDir = ""
}

// Warmup pre-populates the workspace file cache in background.
func (q *QuickOpen) Warmup(rootDir string) {
	if rootDir == "" {
		return
	}
	q.scanWorkspaceFiles(rootDir)
}

func (q *QuickOpen) scanWorkspaceFiles(rootDir string) []QuickOpenItem {
	if q.cachedRootDir == rootDir && q.cachedFiles != nil {
		return q.cachedFiles
	}

	cleanRoot := filepath.Clean(rootDir)
	matcher := ignore.NewMatcher(cleanRoot)
	q.matcher = matcher
	q.cachedRootDir = cleanRoot

	var items []QuickOpenItem
	count := 0
	maxFiles := 10000

	_ = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || count >= maxFiles {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != cleanRoot {
				if strings.HasPrefix(name, ".") || matcher.Matches(path, true) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if strings.HasPrefix(name, ".") || matcher.Matches(path, false) {
			return nil
		}

		rel, relErr := filepath.Rel(cleanRoot, path)
		if relErr != nil {
			rel = path
		}

		items = append(items, QuickOpenItem{
			TabIndex: -1,
			Title:    name,
			RelPath:  rel,
			FullPath: filepath.Clean(path),
			IsDirty:  false,
			IsOpen:   false,
		})
		count++
		return nil
	})

	q.cachedFiles = items
	return items
}

// Open scans workspace files and populates open tabs.
func (q *QuickOpen) Open(openTabs []SwitcherItem, activeIdx int, rootDir string) {
	q.Active = true
	q.RootDir = rootDir
	q.openTabPaths = make(map[string]int)
	q.AllItems = make([]QuickOpenItem, 0)

	// 1. Add currently open tabs
	for _, t := range openTabs {
		relPath := t.Path
		if filepath.IsAbs(t.Path) && rootDir != "" {
			if rel, err := filepath.Rel(rootDir, t.Path); err == nil {
				relPath = rel
			}
		}
		cleanPath := filepath.Clean(t.Path)
		q.openTabPaths[cleanPath] = t.Index
		q.AllItems = append(q.AllItems, QuickOpenItem{
			TabIndex: t.Index,
			Title:    t.Title,
			RelPath:  relPath,
			FullPath: cleanPath,
			IsDirty:  t.IsDirty,
			IsOpen:   true,
		})
	}

	// 2. Add cached/scanned workspace files
	if rootDir != "" {
		workspaceFiles := q.scanWorkspaceFiles(rootDir)
		for _, item := range workspaceFiles {
			if _, exists := q.openTabPaths[item.FullPath]; exists {
				continue
			}
			q.AllItems = append(q.AllItems, item)
		}
	}

	q.Filter.SetValue("")
	applyTextInputStyles(&q.Filter, lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary), lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent))
	q.Filter.Focus()
	q.applyFilter()
	q.Selected = 0
	q.ScrollTop = 0

	// If active tab exists, select it initially
	for i, it := range q.Filtered {
		if it.TabIndex == activeIdx && it.IsOpen {
			q.Selected = i
			break
		}
	}
}

// SetWidth updates modal width and inner filter width.
func (q *QuickOpen) SetWidth(w int) {
	if w < 40 {
		w = 40
	}
	q.Width = w
	q.Filter.SetWidth(w - 8)
}

// Close closes the quick open dialog.
func (q *QuickOpen) Close() {
	q.Active = false
	q.Filter.Blur()
}

// fuzzyMatch calculates simple score for matching query against target.
// Returns matched bool and score (higher is better).
func fuzzyMatch(query, target string) (bool, int) {
	if query == "" {
		return true, 0
	}
	qLower := strings.ToLower(query)
	tLower := strings.ToLower(target)

	// 1. Exact match bonus
	if tLower == qLower {
		return true, 1000
	}
	// 2. Exact substring match bonus
	if idx := strings.Index(tLower, qLower); idx != -1 {
		score := 500 - idx
		if idx == 0 {
			score += 100
		}
		return true, score
	}

	// 3. Subsequence match
	qRunes := []rune(qLower)
	tRunes := []rune(tLower)
	qIdx := 0
	score := 0
	lastMatchIdx := -2

	for tIdx, tr := range tRunes {
		if qIdx < len(qRunes) && tr == qRunes[qIdx] {
			// Consecutive match bonus
			if tIdx == lastMatchIdx+1 {
				score += 20
			}
			// Word start bonus (after slash, dot, underscore, dash)
			if tIdx == 0 || tRunes[tIdx-1] == '/' || tRunes[tIdx-1] == '.' || tRunes[tIdx-1] == '_' || tRunes[tIdx-1] == '-' {
				score += 30
			}
			score += 10
			lastMatchIdx = tIdx
			qIdx++
		}
	}

	if qIdx == len(qRunes) {
		return true, score
	}

	return false, 0
}

// ApplyFilter updates filtered items based on current input.
func (q *QuickOpen) ApplyFilter() {
	q.applyFilter()
}

func (q *QuickOpen) applyFilter() {
	query := strings.TrimSpace(q.Filter.Value())
	if query == "" {
		q.Filtered = make([]QuickOpenItem, len(q.AllItems))
		copy(q.Filtered, q.AllItems)
	} else {
		q.Filtered = make([]QuickOpenItem, 0)
		for _, it := range q.AllItems {
			// Match against title and relative path
			matchedTitle, scoreTitle := fuzzyMatch(query, it.Title)
			matchedPath, scorePath := fuzzyMatch(query, it.RelPath)
			if matchedTitle || matchedPath {
				bestScore := scoreTitle
				if scorePath > bestScore {
					bestScore = scorePath
				}
				if it.IsOpen {
					bestScore += 50 // Open tabs prioritized
				}
				it.Score = bestScore
				q.Filtered = append(q.Filtered, it)
			}
		}

		sort.Slice(q.Filtered, func(i, j int) bool {
			if q.Filtered[i].Score != q.Filtered[j].Score {
				return q.Filtered[i].Score > q.Filtered[j].Score
			}
			return strings.ToLower(q.Filtered[i].RelPath) < strings.ToLower(q.Filtered[j].RelPath)
		})
	}

	if q.Selected >= len(q.Filtered) {
		q.Selected = len(q.Filtered) - 1
		if q.Selected < 0 {
			q.Selected = 0
		}
	}
	q.adjustScroll()
}

func (q *QuickOpen) adjustScroll() {
	if q.Selected < q.ScrollTop {
		q.ScrollTop = q.Selected
	}
	if q.Selected >= q.ScrollTop+q.MaxRows {
		q.ScrollTop = q.Selected - q.MaxRows + 1
	}
	if q.ScrollTop < 0 {
		q.ScrollTop = 0
	}
}

// Update handles keyboard navigation in QuickOpen.
// Returns (targetTabIndex, targetPath, closed, cmd).
func (q *QuickOpen) Update(msg tea.Msg) (targetTabIdx int, targetPath string, closed bool, cmd tea.Cmd) {
	if !q.Active {
		return -1, "", false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch ui.KeyName(msg) {
		case "esc":
			q.Close()
			return -1, "", true, nil
		case "up":
			if q.Selected > 0 {
				q.Selected--
			} else if len(q.Filtered) > 0 {
				q.Selected = len(q.Filtered) - 1
			}
			q.adjustScroll()
			return -1, "", false, nil
		case "down":
			if q.Selected < len(q.Filtered)-1 {
				q.Selected++
			} else {
				q.Selected = 0
			}
			q.adjustScroll()
			return -1, "", false, nil
		case "enter":
			if len(q.Filtered) > 0 && q.Selected >= 0 && q.Selected < len(q.Filtered) {
				item := q.Filtered[q.Selected]
				q.Close()
				return item.TabIndex, item.FullPath, true, nil
			}
			q.Close()
			return -1, "", true, nil
		}
	}

	oldVal := q.Filter.Value()
	var tiCmd tea.Cmd
	q.Filter, tiCmd = q.Filter.Update(msg)
	if q.Filter.Value() != oldVal {
		q.applyFilter()
	}

	return -1, "", false, tiCmd
}

// HandleMouse processes mouse clicks and scrolling inside or outside QuickOpen.
func (q *QuickOpen) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (tabIdx int, path string, closed bool, handled bool) {
	if !q.Active {
		return -1, "", false, false
	}

	rendered := q.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := q.Width

	// Outside click: dismiss dialog
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		q.Close()
		return -1, "", true, true
	}

	if isWheelUp {
		if q.Selected > 0 {
			q.Selected--
		} else if q.ScrollTop > 0 {
			q.ScrollTop--
		}
		q.adjustScroll()
		return -1, "", false, true
	}

	if isWheelDown {
		if q.Selected < len(q.Filtered)-1 {
			q.Selected++
		}
		q.adjustScroll()
		return -1, "", false, true
	}

	// Layout mapping:
	// Border top: relY = 0
	// Padding top: relY = 1
	// Title: relY = 2 (MarginBottom 1 makes relY = 3 blank)
	// Input: relY = 4
	// Blank: relY = 5
	// List items: relY = 6 ...
	if relY == 4 {
		q.Filter.Focus()
		return -1, "", false, true
	}

	listStartY := 6
	if relY >= listStartY && relY < listStartY+q.MaxRows {
		clickedIdx := q.ScrollTop + (relY - listStartY)
		if clickedIdx >= 0 && clickedIdx < len(q.Filtered) {
			q.Selected = clickedIdx
			it := q.Filtered[clickedIdx]
			q.Close()
			return it.TabIndex, it.RelPath, true, true
		}
	}

	return -1, "", false, true
}

// Render renders the QuickOpen modal box.
func (q *QuickOpen) Render() string {
	if !q.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("⚡ Quick Open File (Ctrl+P)")
	renderedInput := q.Filter.View()

	var listLines []string
	if len(q.Filtered) == 0 {
		listLines = append(listLines, "  (No matching files)")
	} else {
		endIdx := q.ScrollTop + q.MaxRows
		if endIdx > len(q.Filtered) {
			endIdx = len(q.Filtered)
		}

		badgeOpen := lipgloss.NewStyle().
			Foreground(ui.CurrentTheme.Accent).
			Render("[Tab] ")

		for i := q.ScrollTop; i < endIdx; i++ {
			it := q.Filtered[i]
			prefix := "  "
			if i == q.Selected {
				prefix = "❯ "
			}

			title := it.Title
			if it.IsDirty {
				title += "*"
			}

			tag := ""
			if it.IsOpen {
				tag = badgeOpen
			}

			lineText := prefix + tag + title + "  " + "(" + it.RelPath + ")"
			usableW := q.Width - 8
			if usableW > 20 && ansi.StringWidth(lineText) > usableW {
				lineText = ansi.Truncate(lineText, usableW, "...")
			}
			if i == q.Selected {
				lineText = ui.CurrentTheme.TopMenuActiveItem.Render(lineText)
			} else {
				lineText = ui.CurrentTheme.DropdownItemStyle.Render(lineText)
			}
			listLines = append(listLines, lineText)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		renderedInput,
		"\n",
		strings.Join(listLines, "\n"),
	)

	boxStyle := lipgloss.NewStyle().
		Width(q.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
