package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

type PickerMode int

const (
	PickerModeFile PickerMode = iota
	PickerModeFolder
)

type PickerItem struct {
	Name     string
	Path     string
	IsDir    bool
	IsParent bool
	Size     int64
}

type FilePicker struct {
	Active      bool
	Mode        PickerMode
	Title       string
	CurrentDir  string
	Items       []PickerItem
	Filtered    []PickerItem
	SelectedIdx int
	ScrollTop   int
	FilterInput textinput.Model
	Width       int
	Height      int
	MaxRows     int
	FocusArea   int // 0: Filter, 1: List, 2: SelectBtn, 3: CancelBtn
	ShowHidden  bool
}

func NewFilePicker() *FilePicker {
	ti := textinput.New()
	ti.Prompt = "Search: "
	ti.CharLimit = 64
	ti.Width = 36
	ti.TextStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.FgPrimary)
	ti.PromptStyle = lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Foreground(ui.CurrentTheme.Accent)

	return &FilePicker{
		Active:      false,
		Mode:        PickerModeFolder,
		Title:       "Select Folder",
		CurrentDir:  ".",
		Items:       make([]PickerItem, 0),
		Filtered:    make([]PickerItem, 0),
		SelectedIdx: 0,
		ScrollTop:   0,
		FilterInput: ti,
		Width:       62,
		Height:      20,
		MaxRows:     10,
		FocusArea:   1,
		ShowHidden:  true,
	}
}

func (p *FilePicker) Open(mode PickerMode, startDir string, showHidden bool) {
	p.Active = true
	p.Mode = mode
	p.ShowHidden = showHidden
	p.FocusArea = 1

	absDir, err := filepath.Abs(startDir)
	if err != nil {
		absDir = startDir
	}
	fi, err := os.Stat(absDir)
	if err == nil && !fi.IsDir() {
		absDir = filepath.Dir(absDir)
	}
	p.CurrentDir = absDir

	if p.Mode == PickerModeFolder {
		p.Title = "📂 Open Folder (Select Working Directory)"
	} else {
		p.Title = "📄 Open File (Select File to Edit)"
	}

	p.FilterInput.SetValue("")
	p.FilterInput.Blur()
	p.loadDir()
}

func (p *FilePicker) Close() {
	p.Active = false
	p.FilterInput.Blur()
}

func (p *FilePicker) loadDir() {
	p.Items = make([]PickerItem, 0)

	// Add parent directory item if not at filesystem root
	parent := filepath.Dir(p.CurrentDir)
	if parent != p.CurrentDir {
		p.Items = append(p.Items, PickerItem{
			Name:     ".. (Up 1 Folder)",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
		})
	}

	entries, err := os.ReadDir(p.CurrentDir)
	if err == nil {
		var dirItems []PickerItem
		var fileItems []PickerItem

		for _, e := range entries {
			name := e.Name()
			if !p.ShowHidden && strings.HasPrefix(name, ".") {
				continue
			}

			fullPath := filepath.Join(p.CurrentDir, name)
			info, infoErr := e.Info()
			var size int64
			if infoErr == nil {
				size = info.Size()
			}

			if e.IsDir() {
				dirItems = append(dirItems, PickerItem{
					Name:  name + "/",
					Path:  fullPath,
					IsDir: true,
					Size:  size,
				})
			} else if p.Mode == PickerModeFile {
				fileItems = append(fileItems, PickerItem{
					Name:  name,
					Path:  fullPath,
					IsDir: false,
					Size:  size,
				})
			}
		}

		sort.Slice(dirItems, func(i, j int) bool {
			return strings.ToLower(dirItems[i].Name) < strings.ToLower(dirItems[j].Name)
		})
		sort.Slice(fileItems, func(i, j int) bool {
			return strings.ToLower(fileItems[i].Name) < strings.ToLower(fileItems[j].Name)
		})

		p.Items = append(p.Items, dirItems...)
		p.Items = append(p.Items, fileItems...)
	}

	p.applyFilter()
}

func (p *FilePicker) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(p.FilterInput.Value()))
	if query == "" {
		p.Filtered = make([]PickerItem, len(p.Items))
		copy(p.Filtered, p.Items)
	} else {
		p.Filtered = make([]PickerItem, 0)
		for _, it := range p.Items {
			if it.IsParent || strings.Contains(strings.ToLower(it.Name), query) {
				p.Filtered = append(p.Filtered, it)
			}
		}
	}

	if p.SelectedIdx >= len(p.Filtered) {
		p.SelectedIdx = len(p.Filtered) - 1
	}
	if p.SelectedIdx < 0 {
		p.SelectedIdx = 0
	}
	p.ScrollTop = 0
}

func (p *FilePicker) Update(msg tea.Msg) (confirmed bool, canceled bool, selectedPath string, cmd tea.Cmd) {
	if !p.Active {
		return false, false, "", nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			p.Close()
			return false, true, "", nil

		case "tab":
			p.FocusArea = (p.FocusArea + 1) % 4
			if p.FocusArea == 0 {
				p.FilterInput.Focus()
			} else {
				p.FilterInput.Blur()
			}
			return false, false, "", nil

		case "shift+tab":
			p.FocusArea = (p.FocusArea + 3) % 4
			if p.FocusArea == 0 {
				p.FilterInput.Focus()
			} else {
				p.FilterInput.Blur()
			}
			return false, false, "", nil

		case "up", "k":
			if p.FocusArea != 0 {
				if p.SelectedIdx > 0 {
					p.SelectedIdx--
					if p.SelectedIdx < p.ScrollTop {
						p.ScrollTop = p.SelectedIdx
					}
				}
				return false, false, "", nil
			}

		case "down", "j":
			if p.FocusArea != 0 {
				if p.SelectedIdx < len(p.Filtered)-1 {
					p.SelectedIdx++
					if p.SelectedIdx >= p.ScrollTop+p.MaxRows {
						p.ScrollTop = p.SelectedIdx - p.MaxRows + 1
					}
				}
				return false, false, "", nil
			}

		case "enter":
			if p.FocusArea == 3 { // Batal
				p.Close()
				return false, true, "", nil
			}

			if p.FocusArea == 2 { // Select Current Directory button
				if p.Mode == PickerModeFolder {
					p.Close()
					return true, false, p.CurrentDir, nil
				}
			}

			// In list
			if len(p.Filtered) > 0 && p.SelectedIdx >= 0 && p.SelectedIdx < len(p.Filtered) {
				sel := p.Filtered[p.SelectedIdx]
				if sel.IsDir {
					p.CurrentDir = sel.Path
					p.FilterInput.SetValue("")
					p.loadDir()
					return false, false, "", nil
				}
				if p.Mode == PickerModeFile && !sel.IsDir {
					p.Close()
					return true, false, sel.Path, nil
				}
			} else if p.Mode == PickerModeFolder {
				p.Close()
				return true, false, p.CurrentDir, nil
			}
			return false, false, "", nil
		}
	}

	if p.FocusArea == 0 {
		var textCmd tea.Cmd
		p.FilterInput, textCmd = p.FilterInput.Update(msg)
		p.applyFilter()
		return false, false, "", textCmd
	}

	return false, false, "", nil
}

// HandleMouse handles mouse clicks and scrolling relative to the modal origin.
func (p *FilePicker) HandleMouse(relX, relY int, isWheelUp, isWheelDown bool) (confirmed bool, canceled bool, selectedPath string, handled bool) {
	if !p.Active {
		return false, false, "", false
	}

	// Modal dimensions (measured from the rendered output, which includes border + padding)
	rendered := p.Render()
	boxW := lipgloss.Width(rendered)
	boxH := lipgloss.Height(rendered)

	if relX < 0 || relX >= boxW || relY < 0 || relY >= boxH {
		// Click outside modal
		return false, false, "", true // handled to prevent leaking to background
	}

	if isWheelUp {
		if p.ScrollTop > 0 {
			p.ScrollTop--
		}
		return false, false, "", true
	}
	if isWheelDown {
		if p.ScrollTop+p.MaxRows < len(p.Filtered) {
			p.ScrollTop++
		}
		return false, false, "", true
	}

	// Layout mapping inside box:
	// Border top: relY = 0
	// Padding: relY = 1
	// Title: relY = 2
	// Current Dir: relY = 3
	// Filter row: relY = 4
	// Empty line: relY = 5
	// List items: relY = 6 to 6 + MaxRows - 1 (relY 6..15)
	// Scroll info: relY = 16
	// Buttons: relY = 17
	// Padding: relY = 18
	// Border bottom: relY = 19

	// 1. Filter input click
	if relY == 4 {
		p.FocusArea = 0
		p.FilterInput.Focus()
		return false, false, "", true
	}

	// 2. List items click
	listStartY := 6
	if relY >= listStartY && relY < listStartY+p.MaxRows {
		clickedRow := relY - listStartY
		targetIdx := p.ScrollTop + clickedRow
		if targetIdx >= 0 && targetIdx < len(p.Filtered) {
			if p.SelectedIdx == targetIdx {
				// Second click on same item -> activate / enter
				sel := p.Filtered[targetIdx]
				if sel.IsDir {
					p.CurrentDir = sel.Path
					p.FilterInput.SetValue("")
					p.loadDir()
					return false, false, "", true
				}
				if p.Mode == PickerModeFile && !sel.IsDir {
					p.Close()
					return true, false, sel.Path, true
				}
			} else {
				p.SelectedIdx = targetIdx
				p.FocusArea = 1
				p.FilterInput.Blur()
			}
		}
		return false, false, "", true
	}

	// 3. Action Buttons click
	buttonY := boxH - 3
	if relY == buttonY || relY == buttonY+1 {
		// [ Pilih/Buka ] starts around relX = 4..30
		// [ Batal ] starts around relX = 32..50
		if relX >= 2 && relX <= 32 {
			if p.Mode == PickerModeFolder {
				p.Close()
				return true, false, p.CurrentDir, true
			}
			if p.Mode == PickerModeFile {
				if len(p.Filtered) > 0 && p.SelectedIdx >= 0 && p.SelectedIdx < len(p.Filtered) {
					sel := p.Filtered[p.SelectedIdx]
					if !sel.IsDir {
						p.Close()
						return true, false, sel.Path, true
					} else {
						p.CurrentDir = sel.Path
						p.FilterInput.SetValue("")
						p.loadDir()
						return false, false, "", true
					}
				}
			}
		} else if relX >= 34 && relX <= 55 {
			p.Close()
			return false, true, "", true
		}
	}

	return false, false, "", true
}

func (p *FilePicker) Render() string {
	if !p.Active {
		return ""
	}

	innerW := p.Width - 4

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight)
	renderedTitle := titleStyle.Render(p.Title)

	dirText := "📁 " + p.CurrentDir
	if ansi.StringWidth(dirText) > innerW {
		dirText = ansi.Truncate(dirText, innerW, "…")
	}
	dirStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary).
		Italic(true)
	renderedDir := dirStyle.Render(dirText)

	renderedFilter := p.FilterInput.View()

	// List items
	var listLines []string
	visibleCount := p.MaxRows
	for i := 0; i < visibleCount; i++ {
		idx := p.ScrollTop + i
		if idx < len(p.Filtered) {
			item := p.Filtered[idx]
			isSelected := idx == p.SelectedIdx

			prefix := "  "
			if isSelected {
				prefix = "❯ "
			}

			icon := "📄 "
			if item.IsParent {
				icon = "⤴ "
			} else if item.IsDir {
				icon = "📁 "
			}

			name := ansi.Truncate(item.Name, innerW-6, "…")
			lineText := prefix + icon + name
			padded := lineText + strings.Repeat(" ", max(0, innerW-ansi.StringWidth(lineText)))

			rowStyle := lipgloss.NewStyle()
			if isSelected {
				rowStyle = rowStyle.
					Background(ui.CurrentTheme.Accent).
					Foreground(ui.CurrentTheme.FgHighlight).
					Bold(true)
			} else if item.IsDir {
				rowStyle = rowStyle.Foreground(ui.CurrentTheme.Accent)
			} else {
				rowStyle = rowStyle.Foreground(ui.CurrentTheme.FgPrimary)
			}
			listLines = append(listLines, rowStyle.Render(padded))
		} else {
			listLines = append(listLines, strings.Repeat(" ", innerW))
		}
	}
	renderedList := strings.Join(listLines, "\n")

	// Scroll indicator info
	scrollInfo := fmt.Sprintf("[%d/%d]  ▲/▼ Scroll | Tab: Switch | Enter: Select", min(p.SelectedIdx+1, len(p.Filtered)), len(p.Filtered))
	scrollStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgSecondary)
	renderedScroll := scrollStyle.Render(scrollInfo)

	// Action buttons
	btnSelectText := "Open This Folder (Enter)"
	if p.Mode == PickerModeFile {
		btnSelectText = "Open File (Enter)"
	}

	btnSelectStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	btnCancelStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BgTopMenu)

	if p.FocusArea == 2 {
		btnSelectStyle = btnSelectStyle.
			Background(ui.CurrentTheme.Accent).
			Foreground(ui.CurrentTheme.FgHighlight).
			Bold(true)
	} else if p.FocusArea == 3 {
		btnCancelStyle = btnCancelStyle.
			Background(ui.CurrentTheme.Red).
			Foreground(ui.CurrentTheme.FgHighlight).
			Bold(true)
	}

	btnSep := lipgloss.NewStyle().Background(ui.CurrentTheme.BgTopMenu).Render("  ")
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		btnSelectStyle.Render(btnSelectText),
		btnSep,
		btnCancelStyle.Render("Cancel (Esc)"),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		renderedDir,
		renderedFilter,
		"\n",
		renderedList,
		renderedScroll,
		"\n",
		buttons,
	)

	boxStyle := lipgloss.NewStyle().
		Width(p.Width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.Accent).
		Background(ui.CurrentTheme.BgTopMenu).
		Padding(1, 2)

	return NormalizeDialogBackground(boxStyle.Render(content), ui.CurrentTheme.BgTopMenu)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
