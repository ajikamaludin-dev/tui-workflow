package dialog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// GlobalSearchMatch represents one matched line in workspace files.
type GlobalSearchMatch struct {
	FilePath   string
	RelPath    string
	LineNumber int // 1-based
	ColNumber  int // 1-based
	LineText   string
}

// GlobalSearch provides workspace-wide text search (Ctrl+Shift+F).
type GlobalSearch struct {
	Active     bool
	RootDir    string
	Filter     textinput.Model
	Matches    []GlobalSearchMatch
	Selected   int
	ScrollTop  int
	Width      int
	MaxRows    int
	TotalFound int
}

// NewGlobalSearch creates a new GlobalSearch instance.
func NewGlobalSearch() *GlobalSearch {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 64
	ti.Width = 52
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &GlobalSearch{
		Active:    false,
		Filter:    ti,
		Matches:   make([]GlobalSearchMatch, 0),
		Selected:  0,
		ScrollTop: 0,
		Width:     70,
		MaxRows:   12,
	}
}

// Open opens the global search dialog.
func (gs *GlobalSearch) Open(rootDir string) {
	gs.Active = true
	gs.RootDir = rootDir
	gs.Matches = make([]GlobalSearchMatch, 0)
	gs.Selected = 0
	gs.ScrollTop = 0
	gs.TotalFound = 0
	gs.Filter.SetValue("")
	gs.Filter.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	gs.Filter.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)
	gs.Filter.Focus()
}

// SetWidth updates modal width and inner filter width.
func (gs *GlobalSearch) SetWidth(w int) {
	if w < 40 {
		w = 40
	}
	gs.Width = w
	gs.Filter.Width = w - 8
}

// Close closes the global search dialog.
func (gs *GlobalSearch) Close() {
	gs.Active = false
	gs.Filter.Blur()
}

// isBinaryContent checks for null byte in the first 512 bytes.
func isBinaryContent(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	return bytes.IndexByte(data[:limit], 0x00) != -1
}

// PerformSearch executes search over all text files in RootDir.
func (gs *GlobalSearch) PerformSearch() {
	query := strings.TrimSpace(gs.Filter.Value())
	gs.Matches = make([]GlobalSearchMatch, 0)
	gs.Selected = 0
	gs.ScrollTop = 0
	gs.TotalFound = 0

	if query == "" || gs.RootDir == "" {
		return
	}

	qLower := strings.ToLower(query)
	maxMatches := 500
	matchCount := 0

	_ = filepath.WalkDir(gs.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || matchCount >= maxMatches {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if (strings.HasPrefix(name, ".") && name != ".") ||
				name == "node_modules" ||
				name == "vendor" ||
				name == "dist" ||
				name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Stat file for size check
		fi, err := d.Info()
		if err != nil || fi.Size() > 10*1024*1024 { // Skip files > 10MB
			return nil
		}

		// Open file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		// Read header for binary check
		headBuf := make([]byte, 512)
		n, _ := file.Read(headBuf)
		if n > 0 && isBinaryContent(headBuf[:n]) {
			return nil
		}
		_, _ = file.Seek(0, io.SeekStart)

		rel, relErr := filepath.Rel(gs.RootDir, path)
		if relErr != nil {
			rel = path
		}

		scanner := bufio.NewScanner(file)
		lineNum := 1
		for scanner.Scan() {
			line := scanner.Text()
			idx := strings.Index(strings.ToLower(line), qLower)
			if idx != -1 {
				matchCount++
				cleanLine := strings.TrimSpace(line)
				if len(cleanLine) > 80 {
					cleanLine = cleanLine[:80] + "..."
				}
				gs.Matches = append(gs.Matches, GlobalSearchMatch{
					FilePath:   path,
					RelPath:    rel,
					LineNumber: lineNum,
					ColNumber:  idx + 1,
					LineText:   cleanLine,
				})
				if matchCount >= maxMatches {
					break
				}
			}
			lineNum++
		}

		return nil
	})

	gs.TotalFound = len(gs.Matches)
	gs.adjustScroll()
}

func (gs *GlobalSearch) adjustScroll() {
	if gs.Selected < gs.ScrollTop {
		gs.ScrollTop = gs.Selected
	}
	if gs.Selected >= gs.ScrollTop+gs.MaxRows {
		gs.ScrollTop = gs.Selected - gs.MaxRows + 1
	}
	if gs.ScrollTop < 0 {
		gs.ScrollTop = 0
	}
}

// Update handles keyboard events.
// Returns (filePath, line, col, closed, cmd).
func (gs *GlobalSearch) Update(msg tea.Msg) (filePath string, line int, col int, closed bool, cmd tea.Cmd) {
	if !gs.Active {
		return "", 0, 0, false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			gs.Close()
			return "", 0, 0, true, nil
		case "up":
			if gs.Selected > 0 {
				gs.Selected--
			} else if len(gs.Matches) > 0 {
				gs.Selected = len(gs.Matches) - 1
			}
			gs.adjustScroll()
			return "", 0, 0, false, nil
		case "down":
			if gs.Selected < len(gs.Matches)-1 {
				gs.Selected++
			} else {
				gs.Selected = 0
			}
			gs.adjustScroll()
			return "", 0, 0, false, nil
		case "enter":
			if len(gs.Matches) > 0 && gs.Selected >= 0 && gs.Selected < len(gs.Matches) {
				m := gs.Matches[gs.Selected]
				gs.Close()
				return m.FilePath, m.LineNumber, m.ColNumber, true, nil
			}
			// If no match selected yet or pressing enter in search input, trigger search
			gs.PerformSearch()
			return "", 0, 0, false, nil
		}
	}

	oldVal := gs.Filter.Value()
	var tiCmd tea.Cmd
	gs.Filter, tiCmd = gs.Filter.Update(msg)
	if gs.Filter.Value() != oldVal {
		gs.PerformSearch()
	}

	return "", 0, 0, false, tiCmd
}

// HandleMouse processes mouse clicks and scrolling inside or outside Global Search.
func (gs *GlobalSearch) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (relPath string, lineNum, colNum int, closed bool, handled bool) {
	if !gs.Active {
		return "", 0, 0, false, false
	}

	rendered := gs.Render()
	lines := strings.Split(rendered, "\n")
	boxH := len(lines)
	boxW := gs.Width

	// Outside click: dismiss dialog
	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		gs.Close()
		return "", 0, 0, true, true
	}

	if isWheelUp {
		if gs.Selected > 0 {
			gs.Selected--
		} else if gs.ScrollTop > 0 {
			gs.ScrollTop--
		}
		gs.adjustScroll()
		return "", 0, 0, false, true
	}

	if isWheelDown {
		if gs.Selected < len(gs.Matches)-1 {
			gs.Selected++
		}
		gs.adjustScroll()
		return "", 0, 0, false, true
	}

	// Layout mapping:
	// Border top: relY = 0
	// Padding top: relY = 1
	// Title: relY = 2 (MarginBottom 1 makes relY = 3 blank)
	// Input: relY = 4
	// Blank: relY = 5
	// Matches list: relY = 6 ...
	if relY == 4 {
		gs.Filter.Focus()
		return "", 0, 0, false, true
	}

	listStartY := 6
	if relY >= listStartY && relY < listStartY+gs.MaxRows {
		clickedIdx := gs.ScrollTop + (relY - listStartY)
		if clickedIdx >= 0 && clickedIdx < len(gs.Matches) {
			gs.Selected = clickedIdx
			m := gs.Matches[clickedIdx]
			gs.Close()
			return m.RelPath, m.LineNumber, m.ColNumber, true, true
		}
	}

	return "", 0, 0, false, true
}

// Render renders the Global Search dialog.
func (gs *GlobalSearch) Render() string {
	if !gs.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	headerText := "🔍 Cari Teks di Seluruh Workspace (Ctrl+Shift+F)"
	if gs.TotalFound > 0 {
		headerText += fmt.Sprintf(" — %d hasil ditemukan", gs.TotalFound)
	}

	renderedTitle := titleStyle.Render(headerText)
	renderedInput := gs.Filter.View()

	var listLines []string
	if strings.TrimSpace(gs.Filter.Value()) == "" {
		listLines = append(listLines, "  Ketik kata kunci untuk mencari di seluruh file...")
	} else if len(gs.Matches) == 0 {
		listLines = append(listLines, "  (Tidak ada kecocokan ditemukan)")
	} else {
		endIdx := gs.ScrollTop + gs.MaxRows
		if endIdx > len(gs.Matches) {
			endIdx = len(gs.Matches)
		}

		matchNumStyle := lipgloss.NewStyle().
			Foreground(ui.CurrentTheme.Accent).
			Bold(true)

		for i := gs.ScrollTop; i < endIdx; i++ {
			m := gs.Matches[i]
			prefix := "  "
			if i == gs.Selected {
				prefix = "❯ "
			}

			locStr := fmt.Sprintf("%s:%d:%d", m.RelPath, m.LineNumber, m.ColNumber)
			lineText := prefix + matchNumStyle.Render(locStr) + "  " + m.LineText
			usableW := gs.Width - 8
			if usableW > 20 && ansi.StringWidth(lineText) > usableW {
				lineText = ansi.Truncate(lineText, usableW, "...")
			}

			if i == gs.Selected {
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
		Width(gs.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}
