package search

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"tui-workflow/internal/ignore"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// SearchMatch represents a single search match in a workspace file.
type SearchMatch struct {
	FilePath   string
	RelPath    string
	LineNumber int // 1-based
	ColNumber  int // 1-based
	LineText   string
}

// SearchViewFocus defines which element is focused inside SearchView.
type SearchViewFocus int

const (
	FocusFindInput SearchViewFocus = iota
	FocusReplaceInput
	FocusMatchList
)

// SearchView provides an integrated search and replace sidebar panel.
type SearchView struct {
	RootDir      string
	FindInput    textinput.Model
	ReplaceInput textinput.Model
	FocusField   SearchViewFocus
	Matches      []SearchMatch
	Selected     int
	ScrollTop    int
	Width        int
	Height       int
	TotalFound   int
	matcher      *ignore.Matcher
}

// NewSearchView creates a new SearchView instance.
func NewSearchView(rootDir string) *SearchView {
	fi := textinput.New()
	fi.Prompt = " Find: "
	fi.CharLimit = 64
	fi.Width = 20
	fi.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	fi.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	fi.Focus()

	ri := textinput.New()
	ri.Prompt = " Replace: "
	ri.CharLimit = 64
	ri.Width = 20
	ri.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ri.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &SearchView{
		RootDir:      rootDir,
		FindInput:    fi,
		ReplaceInput: ri,
		FocusField:   FocusFindInput,
		Matches:      make([]SearchMatch, 0),
		Selected:     0,
		ScrollTop:    0,
		Width:        28,
		Height:       20,
		TotalFound:   0,
		matcher:      ignore.NewMatcher(rootDir),
	}
}

// SetRootDir updates root workspace path.
func (sv *SearchView) SetRootDir(dir string) {
	sv.RootDir = dir
	sv.matcher = ignore.NewMatcher(dir)
	sv.DoSearch()
}

// DoSearch performs workspace-wide text search based on FindInput value.
func (sv *SearchView) DoSearch() {
	query := strings.TrimSpace(sv.FindInput.Value())
	if query == "" || sv.RootDir == "" {
		sv.Matches = make([]SearchMatch, 0)
		sv.TotalFound = 0
		sv.Selected = 0
		sv.ScrollTop = 0
		return
	}

	lowerQuery := strings.ToLower(query)
	var matches []SearchMatch

	_ = filepath.WalkDir(sv.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != sv.RootDir {
				if strings.HasPrefix(name, ".") || (sv.matcher != nil && sv.matcher.Matches(path, true)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if strings.HasPrefix(name, ".") || (sv.matcher != nil && sv.matcher.Matches(path, false)) {
			return nil
		}

		// File safety guards: skip > 10MB
		info, err := d.Info()
		if err != nil || info.Size() > 10*1024*1024 || info.Size() == 0 {
			return nil
		}

		// Check first 512 bytes for null byte (binary guard)
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		buf := make([]byte, 512)
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return nil
		}
		if bytes.IndexByte(buf[:n], 0) != -1 {
			return nil
		}

		_, _ = f.Seek(0, 0)
		scanner := bufio.NewScanner(f)
		lineNum := 1
		relPath, _ := filepath.Rel(sv.RootDir, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		for scanner.Scan() {
			lineText := scanner.Text()
			idx := strings.Index(strings.ToLower(lineText), lowerQuery)
			if idx != -1 {
				matches = append(matches, SearchMatch{
					FilePath:   path,
					RelPath:    relPath,
					LineNumber: lineNum,
					ColNumber:  idx + 1,
					LineText:   strings.TrimSpace(lineText),
				})
				if len(matches) >= 500 {
					return fmt.Errorf("limit reached")
				}
			}
			lineNum++
		}
		return nil
	})

	sv.Matches = matches
	sv.TotalFound = len(matches)
	sv.Selected = 0
	sv.ScrollTop = 0
}

// DoReplaceAll replaces search query with replacement text in all matched files.
func (sv *SearchView) DoReplaceAll() (count int, err error) {
	query := strings.TrimSpace(sv.FindInput.Value())
	if query == "" || len(sv.Matches) == 0 {
		return 0, nil
	}

	replaceText := sv.ReplaceInput.Value()
	replacedFiles := make(map[string]bool)

	for _, m := range sv.Matches {
		if replacedFiles[m.FilePath] {
			continue
		}
		content, err := os.ReadFile(m.FilePath)
		if err != nil {
			continue
		}
		// Case-insensitive or literal replace
		newContent := strings.ReplaceAll(string(content), query, replaceText)
		if newContent != string(content) {
			if err := os.WriteFile(m.FilePath, []byte(newContent), 0644); err == nil {
				replacedFiles[m.FilePath] = true
				count++
			}
		}
	}

	sv.DoSearch()
	return count, nil
}

// Update handles keyboard navigation within SearchView.
func (sv *SearchView) Update(msg tea.Msg) (filePath string, line, col int, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			sv.FocusField = (sv.FocusField + 1) % 3
			sv.syncFocus()
			return "", 0, 0, nil
		case "shift+tab":
			sv.FocusField = (sv.FocusField + 2) % 3
			sv.syncFocus()
			return "", 0, 0, nil
		case "down", "ctrl+n":
			if sv.FocusField == FocusMatchList {
				if sv.Selected < len(sv.Matches)-1 {
					sv.Selected++
				}
				return "", 0, 0, nil
			}
			sv.FocusField = (sv.FocusField + 1) % 3
			sv.syncFocus()
			return "", 0, 0, nil
		case "up", "ctrl+p":
			if sv.FocusField == FocusMatchList {
				if sv.Selected > 0 {
					sv.Selected--
				}
				return "", 0, 0, nil
			}
			sv.FocusField = (sv.FocusField + 2) % 3
			sv.syncFocus()
			return "", 0, 0, nil
		case "enter":
			if sv.FocusField == FocusFindInput {
				sv.DoSearch()
				if len(sv.Matches) > 0 {
					sv.FocusField = FocusMatchList
					sv.syncFocus()
				}
				return "", 0, 0, nil
			} else if sv.FocusField == FocusReplaceInput {
				_, _ = sv.DoReplaceAll()
				return "", 0, 0, nil
			} else if sv.FocusField == FocusMatchList {
				if len(sv.Matches) > 0 && sv.Selected >= 0 && sv.Selected < len(sv.Matches) {
					m := sv.Matches[sv.Selected]
					return m.FilePath, m.LineNumber, m.ColNumber, nil
				}
			}
		}
	}

	var tiCmd tea.Cmd
	if sv.FocusField == FocusFindInput {
		oldVal := sv.FindInput.Value()
		sv.FindInput, tiCmd = sv.FindInput.Update(msg)
		if sv.FindInput.Value() != oldVal {
			sv.DoSearch()
		}
	} else if sv.FocusField == FocusReplaceInput {
		sv.ReplaceInput, tiCmd = sv.ReplaceInput.Update(msg)
	}
	return "", 0, 0, tiCmd
}

func (sv *SearchView) syncFocus() {
	if sv.FocusField == FocusFindInput {
		sv.FindInput.Focus()
		sv.ReplaceInput.Blur()
	} else if sv.FocusField == FocusReplaceInput {
		sv.FindInput.Blur()
		sv.ReplaceInput.Focus()
	} else {
		sv.FindInput.Blur()
		sv.ReplaceInput.Blur()
	}
}

// HandleMouseClick processes mouse click in SearchView.
func (sv *SearchView) HandleMouseClick(relX, relY int) (filePath string, line, col int, handled bool) {
	// Line 0: Find input
	if relY == 0 {
		sv.FocusField = FocusFindInput
		sv.syncFocus()
		return "", 0, 0, true
	}
	// Line 1: Replace input
	if relY == 1 {
		sv.FocusField = FocusReplaceInput
		sv.syncFocus()
		return "", 0, 0, true
	}
	// Line 2: Action buttons row [Cari] [Ganti Semua]
	if relY == 2 {
		if relX < 10 {
			sv.DoSearch()
		} else {
			_, _ = sv.DoReplaceAll()
		}
		return "", 0, 0, true
	}
	// Line 3: Separator
	// Line 4+: Matches list
	matchIdx := sv.ScrollTop + (relY - 4)
	if matchIdx >= 0 && matchIdx < len(sv.Matches) {
		sv.Selected = matchIdx
		sv.FocusField = FocusMatchList
		sv.syncFocus()
		m := sv.Matches[matchIdx]
		return m.FilePath, m.LineNumber, m.ColNumber, true
	}

	return "", 0, 0, false
}

// Render renders the SearchView inside the sidebar.
func (sv *SearchView) Render(width, height int, focused bool) string {
	sv.Width = width
	sv.Height = height

	sv.FindInput.Width = width - 10
	if sv.FindInput.Width < 8 {
		sv.FindInput.Width = 8
	}
	sv.ReplaceInput.Width = width - 10
	if sv.ReplaceInput.Width < 8 {
		sv.ReplaceInput.Width = 8
	}

	inputRow1 := sv.FindInput.View()
	inputRow2 := sv.ReplaceInput.View()

	btnStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.Accent).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)

	var countStr string
	if sv.TotalFound > 0 {
		countStr = fmt.Sprintf(" (%d)", sv.TotalFound)
	}
	actionRow := " " + btnStyle.Render("[Find]") + " " + btnStyle.Render("[Replace All]") + countStyle.Render(countStr)
	sep := strings.Repeat("─", width-1)

	// Available height for matches
	listHeight := height - 4
	if listHeight < 1 {
		listHeight = 1
	}

	// Adjust scroll
	if sv.Selected < sv.ScrollTop {
		sv.ScrollTop = sv.Selected
	} else if sv.Selected >= sv.ScrollTop+listHeight {
		sv.ScrollTop = sv.Selected - listHeight + 1
	}

	var rows []string
	if len(sv.Matches) == 0 {
		emptyMsg := " (Type text & press Enter)"
		if strings.TrimSpace(sv.FindInput.Value()) != "" {
			emptyMsg = " (No matches found)"
		}
		rows = append(rows, countStyle.Render(emptyMsg))
	} else {
		endIdx := sv.ScrollTop + listHeight
		if endIdx > len(sv.Matches) {
			endIdx = len(sv.Matches)
		}

		for i := sv.ScrollTop; i < endIdx; i++ {
			m := sv.Matches[i]
			isSelected := focused && sv.FocusField == FocusMatchList && i == sv.Selected

			prefix := fmt.Sprintf(" %s:%d ", filepath.Base(m.RelPath), m.LineNumber)
			avail := width - ansi.StringWidth(prefix) - 2
			if avail < 5 {
				avail = 5
			}
			textPart := ansi.Truncate(m.LineText, avail, "…")
			lineText := prefix + textPart

			rowStyle := lipgloss.NewStyle().Width(width).MaxHeight(1)
			if isSelected {
				rowStyle = rowStyle.
					Background(ui.CurrentTheme.Accent).
					Foreground(ui.CurrentTheme.FgHighlight).
					Bold(true)
			} else {
				rowStyle = rowStyle.Foreground(ui.CurrentTheme.FgPrimary)
			}
			rows = append(rows, rowStyle.Render(ansi.Truncate(lineText, width, "")))
		}
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		inputRow1,
		inputRow2,
		actionRow,
		sep,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	return body
}

// DoSearchWord mengisi Matches dengan semua kemunculan kata utuh (word-boundary,
// case-insensitive). Dipakai untuk "Find References" (Shift+F12).
func (sv *SearchView) DoSearchWord(word string) {
	sv.FindInput.SetValue(word)
	word = strings.TrimSpace(word)
	if word == "" || sv.RootDir == "" {
		sv.Matches = make([]SearchMatch, 0)
		sv.TotalFound = 0
		sv.Selected = 0
		sv.ScrollTop = 0
		return
	}

	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	var matches []SearchMatch

	_ = filepath.WalkDir(sv.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != sv.RootDir {
				if strings.HasPrefix(name, ".") || (sv.matcher != nil && sv.matcher.Matches(path, true)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if strings.HasPrefix(name, ".") || (sv.matcher != nil && sv.matcher.Matches(path, false)) {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > 10*1024*1024 || info.Size() == 0 {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		buf := make([]byte, 512)
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return nil
		}
		if bytes.IndexByte(buf[:n], 0) != -1 {
			return nil
		}

		_, _ = f.Seek(0, 0)
		scanner := bufio.NewScanner(f)
		lineNum := 1
		relPath, _ := filepath.Rel(sv.RootDir, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		for scanner.Scan() {
			lineText := scanner.Text()
			loc := re.FindStringIndex(lineText)
			if loc != nil {
				matches = append(matches, SearchMatch{
					FilePath:   path,
					RelPath:    relPath,
					LineNumber: lineNum,
					ColNumber:  loc[0] + 1,
					LineText:   strings.TrimSpace(lineText),
				})
				if len(matches) >= 500 {
					return fmt.Errorf("limit reached")
				}
			}
			lineNum++
		}
		return nil
	})

	sv.Matches = matches
	sv.TotalFound = len(matches)
	sv.Selected = 0
	sv.ScrollTop = 0
}
