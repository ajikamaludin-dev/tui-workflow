package dialog

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Active      bool
	AllItems    []QuickOpenItem
	Filtered    []QuickOpenItem
	Selected    int
	ScrollTop   int
	Filter      textinput.Model
	Width       int
	MaxRows     int
	RootDir     string
	openTabPaths map[string]int
}

// NewQuickOpen creates a new QuickOpen instance.
func NewQuickOpen() *QuickOpen {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 64
	ti.Width = 48
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

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
		q.openTabPaths[filepath.Clean(t.Path)] = t.Index
		q.AllItems = append(q.AllItems, QuickOpenItem{
			TabIndex: t.Index,
			Title:    t.Title,
			RelPath:  relPath,
			FullPath: t.Path,
			IsDirty:  t.IsDirty,
			IsOpen:   true,
		})
	}

	// 2. Scan workspace files in background or fast walk
	if rootDir != "" {
		count := 0
		maxFiles := 5000
		_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || count >= maxFiles {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				// Ignore hidden dirs, git, vendor, node_modules
				if (strings.HasPrefix(name, ".") && name != ".") ||
					name == "node_modules" ||
					name == "vendor" ||
					name == "dist" ||
					name == "build" {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files
			if strings.HasPrefix(name, ".") {
				return nil
			}

			cleanPath := filepath.Clean(path)
			// Skip if already in open tabs
			if _, exists := q.openTabPaths[cleanPath]; exists {
				return nil
			}

			rel, relErr := filepath.Rel(rootDir, path)
			if relErr != nil {
				rel = path
			}

			q.AllItems = append(q.AllItems, QuickOpenItem{
				TabIndex: -1,
				Title:    name,
				RelPath:  rel,
				FullPath: cleanPath,
				IsDirty:  false,
				IsOpen:   false,
			})
			count++
			return nil
		})
	}

	q.Filter.SetValue("")
	q.Filter.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	q.Filter.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
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
	q.Filter.Width = w - 8
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
	case tea.KeyMsg:
		switch msg.String() {
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

// Render renders the QuickOpen modal box.
func (q *QuickOpen) Render() string {
	if !q.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	renderedTitle := titleStyle.Render("⚡ Buka Cepat File Workspace (Ctrl+P)")
	renderedInput := q.Filter.View()

	var listLines []string
	if len(q.Filtered) == 0 {
		listLines = append(listLines, "  (Tidak ada file yang cocok)")
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
