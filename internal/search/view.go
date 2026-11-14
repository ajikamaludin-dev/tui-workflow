package search

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tui-workflow/internal/ui"
)

// Row chrome: exact layout used by Render and HandleMouseClick.
// Every input is a rounded-border box (like the git commit box): focus color
// is the theme accent, idle color is the dim border color.
const (
	rowFindTop      = 0
	rowFindBottom   = 3 // Find box spans rows 0–3 (border + 2 lines + border)
	rowToggle       = 4
	rowReplaceTop   = 5
	rowReplaceBottom = 8
	rowScopeTop     = 9  // Path/Exclude box spans rows 9–12
	rowPath         = 10
	rowExclude      = 11
	rowScopeBottom  = 12
	rowButtons      = 13
	rowListStart    = 14
)

const debounceDuration = 300 * time.Millisecond

// SearchViewFocus defines which element is focused inside SearchView.
type SearchViewFocus int

const (
	FocusFindInput SearchViewFocus = iota
	FocusReplaceInput
	FocusPathInput
	FocusExcludeInput
	FocusMatchList
)

// ReplaceScope identifies which set of matches a replace request targets.
type ReplaceScope string

const (
	ReplaceScopeMatch ReplaceScope = "match"
	ReplaceScopeFile  ReplaceScope = "file"
	ReplaceScopeAll   ReplaceScope = "all"
)

// ReplaceRequest is emitted by the view when the user asks for a replace.
// The app layer turns it into a ReplaceJob (task 6) and a confirmation.
type ReplaceRequest struct {
	Scope      ReplaceScope
	FilePath   string
	LineNumber int // 1-based, valid for ScopeMatch
	ColNumber  int // 1-based, valid for ScopeMatch
}

// MouseAction describes a click inside the search view.
type MouseAction struct {
	Kind       string // "ignore", "focus-*", "find", "replace-all", "replace-file", "open", "select-group", "toggle-case", "toggle-word", "toggle-regex"
	FilePath   string
	LineNumber int
	ColNumber  int
}

// ResultsMsg carries async walk results back into the update loop.
type ResultsMsg struct {
	Generation int
	Matches    []SearchMatch
	Truncated  bool
	Err        error
}

// DebounceMsg is emitted after the typing debounce window.
type DebounceMsg struct{ generation int }

type matchGroup struct {
	RelPath  string
	FilePath string
	Indices  []int // indices into Matches
}

type rowItem struct {
	isHeader bool
	group    int // group index when isHeader
	matchIdx int // index into Matches when !isHeader
}

// SearchView provides an integrated search and replace sidebar panel.
type SearchView struct {
	RootDir      string
	FindInput    textarea.Model
	ReplaceInput textarea.Model
	PathInput    textarea.Model
	ExcludeInput textarea.Model
	FocusField   SearchViewFocus

	Matches    []SearchMatch
	Selected   int
	ScrollTop  int
	Width      int
	Height     int
	TotalFound int
	Truncated  bool
	ErrMsg     string

	CaseSensitive bool
	WholeWord     bool
	Regex         bool

	generation int
	groups     []*matchGroup
}

// NewSearchView creates a new SearchView instance.
func NewSearchView(rootDir string) *SearchView {
	mk := func(placeholder string, height int) textarea.Model {
		ta := textarea.New()
		ta.Prompt = ""
		ta.Placeholder = placeholder
		ta.CharLimit = 256
		ta.ShowLineNumbers = false
		ta.SetHeight(height)
		taStyles := ta.Styles()
		taStyles.Focused.CursorLine = lipgloss.NewStyle()
		taStyles.Blurred.CursorLine = lipgloss.NewStyle()
		taStyles.Focused.CursorLineNumber = lipgloss.NewStyle()
		taStyles.Blurred.CursorLineNumber = lipgloss.NewStyle()
		taStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
		taStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
		taStyles.Focused.Text = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgPrimary)
		taStyles.Blurred.Text = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgPrimary)
		ta.SetStyles(taStyles)
		return ta
	}

	find := mk("Find in files...", 2)
	replace := mk("Replacement...", 2)
	path := mk("Path (opt.: internal/)", 1)
	exclude := mk("Exclude (opt.: vendor/)", 1)

	sv := &SearchView{
		RootDir:      rootDir,
		FindInput:    find,
		ReplaceInput: replace,
		PathInput:    path,
		ExcludeInput: exclude,
		FocusField:   FocusFindInput,
		Matches:      make([]SearchMatch, 0),
		Selected:     0,
		ScrollTop:    0,
		Width:        28,
		Height:       20,
		TotalFound:   0,
	}
	sv.syncFocus()
	return sv
}

// SetRootDir updates root workspace path.
func (sv *SearchView) SetRootDir(dir string) {
	sv.RootDir = dir
	sv.DoSearch()
}

// FocusFind focuses the find input.
func (sv *SearchView) FocusFind() {
	sv.FocusField = FocusFindInput
	sv.syncFocus()
}

// CurrentQuery builds the engine query from the UI state.
func (sv *SearchView) CurrentQuery() Query {
	return Query{
		Pattern:       sv.FindInput.Value(),
		CaseSensitive: sv.CaseSensitive,
		WholeWord:     sv.WholeWord,
		Regex:         sv.Regex,
	}
}

func splitFields(v string) []string {
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
	filtered := fields[:0]
	for _, f := range fields {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

// DoSearch performs the workspace search synchronously (used for explicit
// refreshes and in tests). Typing goes through scheduleSearch/debounce instead.
func (sv *SearchView) DoSearch() {
	sv.generation++
	if strings.TrimSpace(sv.FindInput.Value()) == "" {
		sv.ErrMsg = ""
		sv.Matches = nil
		sv.TotalFound = 0
		sv.Truncated = false
		sv.groups = nil
		sv.Selected = 0
		sv.ScrollTop = 0
		return
	}
	req := WalkRequest{
		Root:       sv.RootDir,
		Include:    splitFields(sv.PathInput.Value()),
		Exclude:    splitFields(sv.ExcludeInput.Value()),
		Query:      sv.CurrentQuery(),
		MaxMatches: DefaultMaxMatches,
	}
	res := WalkSync(req)
	sv.applyResults(sv.generation, res, true)
}

// DoSearchWord fills Matches with all whole-word (case-insensitive) matches.
// Used for "Find References" (Shift+F12).
func (sv *SearchView) DoSearchWord(word string) {
	sv.FindInput.SetValue(word)
	sv.generation++
	res := WalkSync(WalkRequest{
		Root:       sv.RootDir,
		Include:    splitFields(sv.PathInput.Value()),
		Exclude:    splitFields(sv.ExcludeInput.Value()),
		Query:      Query{Pattern: word, WholeWord: true},
		MaxMatches: DefaultMaxMatches,
	})
	sv.applyResults(sv.generation, res, true)
}

func (sv *SearchView) scheduleSearch() tea.Cmd {
	sv.generation++
	g := sv.generation
	return tea.Tick(debounceDuration, func(time.Time) tea.Msg {
		return DebounceMsg{generation: g}
	})
}

// HandleResults applies async walk results; stale generations are dropped.
func (sv *SearchView) HandleResults(msg ResultsMsg) {
	if msg.Generation != sv.generation {
		return
	}
	sv.applyResults(msg.Generation, WalkResult{Matches: msg.Matches, Truncated: msg.Truncated, Err: msg.Err}, false)
}

func (sv *SearchView) applyResults(_ int, res WalkResult, _ bool) {
	sv.ErrMsg = ""
	if res.Err != nil {
		sv.ErrMsg = "invalid regex: " + res.Err.Error()
		sv.Matches = nil
		sv.TotalFound = 0
		sv.Truncated = false
		sv.groups = nil
		sv.Selected = 0
		sv.ScrollTop = 0
		return
	}
	sv.Matches = res.Matches
	sv.TotalFound = len(res.Matches)
	sv.Truncated = res.Truncated
	sv.Selected = 0
	sv.ScrollTop = 0

	groupMap := make(map[string]*matchGroup)
	var order []*matchGroup
	for i, m := range sv.Matches {
		g, ok := groupMap[m.RelPath]
		if !ok {
			g = &matchGroup{RelPath: m.RelPath, FilePath: m.FilePath}
			groupMap[m.RelPath] = g
			order = append(order, g)
		}
		g.Indices = append(g.Indices, i)
	}
	sv.groups = order
}

func (sv *SearchView) selectedMatch() (SearchMatch, bool) {
	if sv.Selected < 0 || sv.Selected >= len(sv.Matches) {
		return SearchMatch{}, false
	}
	return sv.Matches[sv.Selected], true
}

func (sv *SearchView) allRows() []rowItem {
	var rows []rowItem
	for gi, g := range sv.groups {
		rows = append(rows, rowItem{isHeader: true, group: gi})
		for _, idx := range g.Indices {
			rows = append(rows, rowItem{matchIdx: idx})
		}
	}
	return rows
}

func (sv *SearchView) selectedRowIndex() int {
	rows := sv.allRows()
	for i, r := range rows {
		if !r.isHeader && r.matchIdx == sv.Selected {
			return i
		}
	}
	return -1
}

func (sv *SearchView) listHeight() int {
	h := sv.Height - rowListStart - 1 // minus footer
	if h < 1 {
		h = 1
	}
	return h
}

func (sv *SearchView) adjustScroll() {
	listH := sv.listHeight()
	row := sv.selectedRowIndex()
	if row < 0 {
		// Header-only or empty; scroll to top.
		sv.ScrollTop = 0
		return
	}
	if row < sv.ScrollTop {
		sv.ScrollTop = row
	} else if row >= sv.ScrollTop+listH {
		sv.ScrollTop = row - listH + 1
	}
	if sv.ScrollTop < 0 {
		sv.ScrollTop = 0
	}
}

// ReplaceAllCounts returns (occurrences, files) across the current results.
func (sv *SearchView) ReplaceAllCounts() (int, int) {
	files := make(map[string]bool)
	for _, m := range sv.Matches {
		files[m.FilePath] = true
	}
	return len(sv.Matches), len(files)
}

// ReplaceFileCounts returns (occurrences, files=1) for the given absolute path.
func (sv *SearchView) ReplaceFileCounts(filePath string) (int, int) {
	n := 0
	for _, m := range sv.Matches {
		if m.FilePath == filePath {
			n++
		}
	}
	return n, 1
}

func (sv *SearchView) replaceCmd(req ReplaceRequest) tea.Cmd {
	return func() tea.Msg { return req }
}

// Update handles key/message input for the search view.
func (sv *SearchView) Update(msg tea.Msg) (filePath string, line, col int, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := ui.KeyName(msg)
		switch key {
		case "tab":
			sv.cycleFocus(1)
			return "", 0, 0, nil
		case "shift+tab":
			sv.cycleFocus(-1)
			return "", 0, 0, nil
		case "down", "ctrl+n":
			if sv.FocusField == FocusMatchList {
				if sv.Selected < len(sv.Matches)-1 {
					sv.Selected++
				}
				sv.adjustScroll()
				return "", 0, 0, nil
			}
			sv.cycleFocus(1)
			return "", 0, 0, nil
		case "up", "ctrl+p":
			if sv.FocusField == FocusMatchList {
				if sv.Selected > 0 {
					sv.Selected--
				}
				sv.adjustScroll()
				return "", 0, 0, nil
			}
			sv.cycleFocus(-1)
			return "", 0, 0, nil
		case "alt+c":
			sv.CaseSensitive = !sv.CaseSensitive
			return "", 0, 0, sv.scheduleSearch()
		case "alt+w":
			sv.WholeWord = !sv.WholeWord
			return "", 0, 0, sv.scheduleSearch()
		case "alt+x":
			sv.Regex = !sv.Regex
			return "", 0, 0, sv.scheduleSearch()
		case "alt+enter":
			if sv.FocusField == FocusMatchList {
				if m, ok := sv.selectedMatch(); ok {
					return "", 0, 0, sv.replaceCmd(ReplaceRequest{Scope: ReplaceScopeMatch, FilePath: m.FilePath, LineNumber: m.LineNumber, ColNumber: m.ColNumber})
				}
			}
			return "", 0, 0, nil
		case "alt+shift+enter":
			if sv.FocusField == FocusMatchList {
				if m, ok := sv.selectedMatch(); ok {
					return "", 0, 0, sv.replaceCmd(ReplaceRequest{Scope: ReplaceScopeFile, FilePath: m.FilePath})
				}
			}
			return "", 0, 0, nil
		case "enter":
			switch sv.FocusField {
			case FocusFindInput, FocusPathInput, FocusExcludeInput:
				sv.DoSearch()
				if len(sv.Matches) > 0 {
					sv.FocusField = FocusMatchList
					sv.syncFocus()
				}
				return "", 0, 0, nil
			case FocusReplaceInput:
				return "", 0, 0, sv.replaceCmd(ReplaceRequest{Scope: ReplaceScopeAll})
			case FocusMatchList:
				if m, ok := sv.selectedMatch(); ok {
					return m.FilePath, m.LineNumber, m.ColNumber, nil
				}
				return "", 0, 0, nil
			}
		}
	case DebounceMsg:
		if msg.generation != sv.generation {
			return "", 0, 0, nil
		}
		if strings.TrimSpace(sv.FindInput.Value()) == "" {
			sv.ErrMsg = ""
			sv.Matches = nil
			sv.TotalFound = 0
			sv.Truncated = false
			sv.groups = nil
			sv.Selected = 0
			sv.ScrollTop = 0
			return "", 0, 0, nil
		}
		req := WalkRequest{
			Root:       sv.RootDir,
			Include:    splitFields(sv.PathInput.Value()),
			Exclude:    splitFields(sv.ExcludeInput.Value()),
			Query:      sv.CurrentQuery(),
			MaxMatches: DefaultMaxMatches,
		}
		g := sv.generation
		return "", 0, 0, func() tea.Msg {
			res := WalkSync(req)
			return ResultsMsg{Generation: g, Matches: res.Matches, Truncated: res.Truncated, Err: res.Err}
		}
	case ResultsMsg:
		sv.HandleResults(msg)
		return "", 0, 0, nil
	}

	// Default: forward to the focused textarea.
	changed := false
	if sv.FocusField == FocusFindInput {
		old := sv.FindInput.Value()
		sv.FindInput, cmd = sv.FindInput.Update(msg)
		changed = sv.FindInput.Value() != old
	} else if sv.FocusField == FocusReplaceInput {
		sv.ReplaceInput, cmd = sv.ReplaceInput.Update(msg)
	} else if sv.FocusField == FocusPathInput {
		old := sv.PathInput.Value()
		sv.PathInput, cmd = sv.PathInput.Update(msg)
		changed = sv.PathInput.Value() != old
	} else if sv.FocusField == FocusExcludeInput {
		old := sv.ExcludeInput.Value()
		sv.ExcludeInput, cmd = sv.ExcludeInput.Update(msg)
		changed = sv.ExcludeInput.Value() != old
	}
	if changed {
		cmd = sv.scheduleSearch()
	}
	return "", 0, 0, cmd
}

func (sv *SearchView) cycleFocus(delta int) {
	sv.FocusField = (sv.FocusField + SearchViewFocus(delta) + 5) % 5
	sv.syncFocus()
}

func (sv *SearchView) syncFocus() {
	sv.FindInput.Blur()
	sv.ReplaceInput.Blur()
	sv.PathInput.Blur()
	sv.ExcludeInput.Blur()
	switch sv.FocusField {
	case FocusFindInput:
		sv.FindInput.Focus()
	case FocusReplaceInput:
		sv.ReplaceInput.Focus()
	case FocusPathInput:
		sv.PathInput.Focus()
	case FocusExcludeInput:
		sv.ExcludeInput.Focus()
	case FocusMatchList:
	}
}

// HandleMouseClick processes a click at the given sidebar-relative coordinates.
func (sv *SearchView) HandleMouseClick(relX, relY int) MouseAction {
	width := sv.Width
	switch {
	case relY >= rowFindTop && relY <= rowFindBottom:
		sv.FocusField = FocusFindInput
		sv.syncFocus()
		return MouseAction{Kind: "focus-find"}
	case relY == rowToggle:
		switch {
		case relX >= 1 && relX <= 5:
			sv.CaseSensitive = !sv.CaseSensitive
			sv.DoSearch()
			return MouseAction{Kind: "toggle-case"}
		case relX >= 6 && relX <= 11:
			sv.WholeWord = !sv.WholeWord
			sv.DoSearch()
			return MouseAction{Kind: "toggle-word"}
		case relX >= 12 && relX <= 16:
			sv.Regex = !sv.Regex
			sv.DoSearch()
			return MouseAction{Kind: "toggle-regex"}
		}
		return MouseAction{Kind: "ignore"}
	case relY >= rowReplaceTop && relY <= rowReplaceBottom:
		sv.FocusField = FocusReplaceInput
		sv.syncFocus()
		return MouseAction{Kind: "focus-replace"}
	case relY >= rowScopeTop && relY <= rowPath:
		sv.FocusField = FocusPathInput
		sv.syncFocus()
		return MouseAction{Kind: "focus-path"}
	case relY == rowExclude || relY == rowScopeBottom:
		sv.FocusField = FocusExcludeInput
		sv.syncFocus()
		return MouseAction{Kind: "focus-exclude"}
	case relY == rowButtons:
		if relX >= 1 && relX <= 6 {
			sv.DoSearch()
			return MouseAction{Kind: "find"}
		}
		if relX >= 8 && relX <= 21 {
			return MouseAction{Kind: "replace-all"}
		}
		return MouseAction{Kind: "ignore"}
	case relY >= rowListStart:
		rows := sv.allRows()
		listH := sv.listHeight()
		start := sv.ScrollTop
		end := start + listH
		idx := start + (relY - rowListStart)
		if idx >= len(rows) || idx >= end {
			return MouseAction{Kind: "ignore"}
		}
		row := rows[idx]
		if row.isHeader {
			g := sv.groups[row.group]
			focusList := func() {
				sv.FocusField = FocusMatchList
				sv.syncFocus()
				if len(g.Indices) > 0 {
					sv.Selected = g.Indices[0]
					sv.adjustScroll()
				}
			}
			if relX >= width-10 {
				return MouseAction{Kind: "replace-file", FilePath: g.FilePath}
			}
			focusList()
			return MouseAction{Kind: "select-group"}
		}
		m := sv.Matches[row.matchIdx]
		sv.Selected = row.matchIdx
		sv.FocusField = FocusMatchList
		sv.syncFocus()
		sv.adjustScroll()
		return MouseAction{Kind: "open", FilePath: m.FilePath, LineNumber: m.LineNumber, ColNumber: m.ColNumber}
	}
	return MouseAction{Kind: "ignore"}
}

// Render renders the SearchView inside the sidebar.
func (sv *SearchView) Render(width, height int, focused bool) string {
	sv.Width = width
	sv.Height = height

	setWidth := func(t *textarea.Model) {
		w := width - 1
		if w < 8 {
			w = 8
		}
		t.SetWidth(w)
	}
	setWidth(&sv.FindInput)
	setWidth(&sv.ReplaceInput)
	setWidth(&sv.PathInput)
	setWidth(&sv.ExcludeInput)

	rowFind := sv.FindInput.View()
	rowReplace := sv.ReplaceInput.View()
	rowPath := sv.PathInput.View()
	rowExclude := sv.ExcludeInput.View()

	// Toggle chips + right-aligned counter
	chip := func(label string, on bool) string {
		style := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
		if on {
			style = style.Foreground(ui.CurrentTheme.Accent).Bold(true)
		}
		return style.Render(label)
	}
	chips := " " + chip("[Aa]", sv.CaseSensitive) + " " + chip("[ab|]", sv.WholeWord) + " " + chip("[.*]", sv.Regex)
	var counter string
	if sv.ErrMsg != "" {
		counter = lipgloss.NewStyle().Foreground(ui.CurrentTheme.Red).Render("ERR")
	} else if sv.Truncated {
		counter = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary).Render("500+")
	} else if sv.TotalFound > 0 {
		counter = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary).Render(fmt.Sprintf("%d", sv.TotalFound))
	}
	chipsRow := chips
	if counter != "" {
		pad := width - 1 - ansi.StringWidth(chips) - ansi.StringWidth(counter)
		if pad < 1 {
			pad = 1
		}
		chipsRow = chips + strings.Repeat(" ", pad) + counter
	}

	btnStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.Accent).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
	buttonsRow := " " + btnStyle.Render("[Find]") + "  " + btnStyle.Render("[Replace All]")

	sep := strings.Repeat("─", width)

	listH := sv.listHeight()
	sv.adjustScroll()
	rows := sv.allRows()

	var sb strings.Builder
	sb.WriteString(rowFind)
	sb.WriteString("\n")
	sb.WriteString(chipsRow)
	sb.WriteString("\n")
	sb.WriteString(rowReplace)
	sb.WriteString("\n")
	sb.WriteString(rowPath)
	sb.WriteString("\n")
	sb.WriteString(rowExclude)
	sb.WriteString("\n")
	sb.WriteString(buttonsRow)
	sb.WriteString("\n")
	sb.WriteString(sep)
	sb.WriteString("\n")

	if sv.ErrMsg != "" {
		hint := lipgloss.NewStyle().Foreground(ui.CurrentTheme.Red).Render(" " + sv.ErrMsg)
		sb.WriteString(ansi.Truncate(hint, width, ""))
		for i := 1; i < listH; i++ {
			sb.WriteString("\n")
		}
	} else if strings.TrimSpace(sv.FindInput.Value()) == "" {
		sb.WriteString(countStyle.Render(" (Type text to search)"))
		for i := 1; i < listH; i++ {
			sb.WriteString("\n")
		}
	} else if len(sv.Matches) == 0 {
		sb.WriteString(countStyle.Render(" (No matches found)"))
		for i := 1; i < listH; i++ {
			sb.WriteString("\n")
		}
	} else {
		vRows := 0
		start := sv.ScrollTop
		end := start + listH
		if end > len(rows) {
			end = len(rows)
		}
		for i := start; i < end; i++ {
			row := rows[i]
			if vRows > 0 {
				sb.WriteString("\n")
			}
			vRows++
			if row.isHeader {
				g := sv.groups[row.group]
				left := fmt.Sprintf(" ▸ %s (%d)", g.RelPath, len(g.Indices))
				right := "[Replace]"
				left = ansi.Truncate(left, width-len(right)-2, "…")
				pad := width - ansi.StringWidth(left) - len(right) - 1
				if pad < 1 {
					pad = 1
				}
				sb.WriteString(ansi.Truncate(left, width, "") +
					strings.Repeat(" ", pad) +
					lipgloss.NewStyle().Foreground(ui.CurrentTheme.Accent).Bold(true).Render("[Replace]"))
				continue
			}
			m := sv.Matches[row.matchIdx]
			isSelected := focused && sv.FocusField == FocusMatchList && row.matchIdx == sv.Selected
			prefix := fmt.Sprintf("  %d: ", m.LineNumber)
			avail := width - ansi.StringWidth(prefix) - 1
			if avail < 5 {
				avail = 5
			}
			line := prefix + ansi.Truncate(m.LineText, avail, "…")
			if isSelected {
				style := lipgloss.NewStyle().
					Width(width).
					MaxHeight(1).
					Background(ui.CurrentTheme.Accent).
					Foreground(ui.CurrentTheme.FgHighlight).
					Bold(true)
				sb.WriteString(style.Render(ansi.Truncate(line, width, "")))
			} else {
				sb.WriteString(lipgloss.NewStyle().MaxHeight(1).Foreground(ui.CurrentTheme.FgPrimary).Render(ansi.Truncate(line, width, "")))
			}
		}
		for i := vRows; i < listH; i++ {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	var footer string
	if sv.Truncated {
		footer = " 500+ matches (first 500 shown)"
	} else if sv.TotalFound > 0 {
		footer = fmt.Sprintf(" %d files, %d matches", len(sv.groups), sv.TotalFound)
	} else {
		footer = " Search results"
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary).Render(ansi.Truncate(footer, width, "")))

	return sb.String()
}
