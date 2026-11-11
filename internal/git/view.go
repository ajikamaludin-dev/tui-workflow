package git

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ui"
)

type GitFocusArea int

const (
	GitFocusCommitInput GitFocusArea = iota
	GitFocusStaged
	GitFocusChanges
	GitFocusHistory
)

type GitView struct {
	Repo        *Repo
	CommitInput textinput.Model

	Staged   []GitFileStatus
	Changes  []GitFileStatus
	History  []CommitInfo

	FocusArea    GitFocusArea
	StagedIdx    int
	ChangesIdx   int
	HistoryIdx   int

	StagedFolded  bool
	ChangesFolded bool
	HistoryFolded bool

	ScrollOffset int

	// Last click coordinate mapping for mouse handling
	ItemClickMap map[int]GitClickTarget
}

type GitClickType int

const (
	ClickNone GitClickType = iota
	ClickCommitInput
	ClickSectionStaged
	ClickSectionChanges
	ClickSectionHistory
	ClickItemStaged
	ClickItemChanges
	ClickItemHistory
	ClickStageAction
	ClickUnstageAction
)

type GitClickTarget struct {
	Type  GitClickType
	Index int
	Path  string
}

func NewGitView(repo *Repo) *GitView {
	ti := textinput.New()
	ti.Placeholder = "Message (Ctrl+Enter to commit)"
	ti.CharLimit = 200
	ti.Width = 24

	gv := &GitView{
		Repo:         repo,
		CommitInput:  ti,
		FocusArea:    GitFocusCommitInput,
		ItemClickMap: make(map[int]GitClickTarget),
	}
	gv.CommitInput.Focus()
	gv.Refresh()
	return gv
}

func (gv *GitView) Refresh() {
	if gv.Repo == nil {
		return
	}
	staged, unstaged, _ := gv.Repo.GetStatus()
	gv.Staged = staged
	gv.Changes = unstaged

	history, _ := gv.Repo.GetHistory(10)
	gv.History = history

	if gv.StagedIdx >= len(gv.Staged) {
		gv.StagedIdx = len(gv.Staged) - 1
	}
	if gv.StagedIdx < 0 {
		gv.StagedIdx = 0
	}

	if gv.ChangesIdx >= len(gv.Changes) {
		gv.ChangesIdx = len(gv.Changes) - 1
	}
	if gv.ChangesIdx < 0 {
		gv.ChangesIdx = 0
	}

	if gv.HistoryIdx >= len(gv.History) {
		gv.HistoryIdx = len(gv.History) - 1
	}
	if gv.HistoryIdx < 0 {
		gv.HistoryIdx = 0
	}
}

func (gv *GitView) SelectedStagedFile() *GitFileStatus {
	if len(gv.Staged) > 0 && gv.StagedIdx < len(gv.Staged) {
		return &gv.Staged[gv.StagedIdx]
	}
	return nil
}

func (gv *GitView) SelectedChangesFile() *GitFileStatus {
	if len(gv.Changes) > 0 && gv.ChangesIdx < len(gv.Changes) {
		return &gv.Changes[gv.ChangesIdx]
	}
	return nil
}

func (gv *GitView) StageSelected() error {
	if file := gv.SelectedChangesFile(); file != nil {
		err := gv.Repo.StageFile(file.Path)
		if err != nil {
			return err
		}
		gv.Refresh()
	}
	return nil
}

func (gv *GitView) UnstageSelected() error {
	if file := gv.SelectedStagedFile(); file != nil {
		err := gv.Repo.UnstageFile(file.Path)
		if err != nil {
			return err
		}
		gv.Refresh()
	}
	return nil
}

func (gv *GitView) DoCommit() (string, error) {
	if gv.Repo == nil {
		return "", fmt.Errorf("repositori git tidak ditemukan")
	}
	if len(gv.Staged) == 0 {
		return "", fmt.Errorf("tidak ada perubahan di staging area")
	}
	msg := gv.CommitInput.Value()
	hash, err := gv.Repo.Commit(msg)
	if err != nil {
		return "", err
	}
	gv.CommitInput.SetValue("")
	gv.Refresh()
	return hash, nil
}

func (gv *GitView) Update(msg tea.Msg) (tea.Cmd, *GitFileStatus) {
	var fileToOpen *GitFileStatus

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			// Cycle focus
			switch gv.FocusArea {
			case GitFocusCommitInput:
				gv.FocusArea = GitFocusStaged
				gv.CommitInput.Blur()
			case GitFocusStaged:
				gv.FocusArea = GitFocusChanges
			case GitFocusChanges:
				gv.FocusArea = GitFocusHistory
			case GitFocusHistory:
				gv.FocusArea = GitFocusCommitInput
				gv.CommitInput.Focus()
			}
			return nil, nil

		case tea.KeyUp:
			if gv.FocusArea == GitFocusCommitInput {
				// stay
			} else if gv.FocusArea == GitFocusStaged {
				if gv.StagedIdx > 0 {
					gv.StagedIdx--
				} else {
					gv.FocusArea = GitFocusCommitInput
					gv.CommitInput.Focus()
				}
			} else if gv.FocusArea == GitFocusChanges {
				if gv.ChangesIdx > 0 {
					gv.ChangesIdx--
				} else {
					gv.FocusArea = GitFocusStaged
				}
			} else if gv.FocusArea == GitFocusHistory {
				if gv.HistoryIdx > 0 {
					gv.HistoryIdx--
				} else {
					gv.FocusArea = GitFocusChanges
				}
			}

		case tea.KeyDown:
			if gv.FocusArea == GitFocusCommitInput {
				gv.FocusArea = GitFocusStaged
				gv.CommitInput.Blur()
			} else if gv.FocusArea == GitFocusStaged {
				if gv.StagedIdx < len(gv.Staged)-1 {
					gv.StagedIdx++
				} else {
					gv.FocusArea = GitFocusChanges
				}
			} else if gv.FocusArea == GitFocusChanges {
				if gv.ChangesIdx < len(gv.Changes)-1 {
					gv.ChangesIdx++
				} else {
					gv.FocusArea = GitFocusHistory
				}
			} else if gv.FocusArea == GitFocusHistory {
				if gv.HistoryIdx < len(gv.History)-1 {
					gv.HistoryIdx++
				}
			}

		case tea.KeyEnter:
			if gv.FocusArea == GitFocusStaged {
				fileToOpen = gv.SelectedStagedFile()
			} else if gv.FocusArea == GitFocusChanges {
				fileToOpen = gv.SelectedChangesFile()
			}

		case tea.KeyRunes:
			str := string(msg.Runes)
			if gv.FocusArea == GitFocusCommitInput {
				var cmd tea.Cmd
				gv.CommitInput, cmd = gv.CommitInput.Update(msg)
				return cmd, nil
			}

			if str == "+" {
				_ = gv.StageSelected()
			} else if str == "-" {
				_ = gv.UnstageSelected()
			} else if str == "k" {
				// vim up
				if gv.FocusArea == GitFocusStaged && gv.StagedIdx > 0 {
					gv.StagedIdx--
				} else if gv.FocusArea == GitFocusChanges && gv.ChangesIdx > 0 {
					gv.ChangesIdx--
				}
			} else if str == "j" {
				// vim down
				if gv.FocusArea == GitFocusStaged && gv.StagedIdx < len(gv.Staged)-1 {
					gv.StagedIdx++
				} else if gv.FocusArea == GitFocusChanges && gv.ChangesIdx < len(gv.Changes)-1 {
					gv.ChangesIdx++
				}
			}
		}
	}

	if gv.FocusArea == GitFocusCommitInput {
		var cmd tea.Cmd
		gv.CommitInput, cmd = gv.CommitInput.Update(msg)
		return cmd, fileToOpen
	}

	return nil, fileToOpen
}

func (gv *GitView) HandleMouseClick(relY int) (*GitFileStatus, bool) {
	target, ok := gv.ItemClickMap[relY]
	if !ok {
		return nil, false
	}

	switch target.Type {
	case ClickCommitInput:
		gv.FocusArea = GitFocusCommitInput
		gv.CommitInput.Focus()
		return nil, true

	case ClickSectionStaged:
		gv.StagedFolded = !gv.StagedFolded
		return nil, true

	case ClickSectionChanges:
		gv.ChangesFolded = !gv.ChangesFolded
		return nil, true

	case ClickSectionHistory:
		gv.HistoryFolded = !gv.HistoryFolded
		return nil, true

	case ClickItemStaged:
		gv.FocusArea = GitFocusStaged
		gv.StagedIdx = target.Index
		gv.CommitInput.Blur()
		if target.Index < len(gv.Staged) {
			return &gv.Staged[target.Index], true
		}

	case ClickItemChanges:
		gv.FocusArea = GitFocusChanges
		gv.ChangesIdx = target.Index
		gv.CommitInput.Blur()
		if target.Index < len(gv.Changes) {
			return &gv.Changes[target.Index], true
		}

	case ClickStageAction:
		_ = gv.Repo.StageFile(target.Path)
		gv.Refresh()
		return nil, true

	case ClickUnstageAction:
		_ = gv.Repo.UnstageFile(target.Path)
		gv.Refresh()
		return nil, true
	}

	return nil, false
}

func (gv *GitView) Render(width, height int, isFocused bool) string {
	gv.ItemClickMap = make(map[int]GitClickTarget)

	if gv.Repo == nil {
		lines := []string{
			" Bukan repositori Git",
			" Jalankan 'git init'",
		}
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}

	gv.CommitInput.Width = width - 4
	if gv.CommitInput.Width < 10 {
		gv.CommitInput.Width = 10
	}

	var lines []string
	currentY := 0

	// 1. Commit Box
	inputBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.BorderColor).
		Width(width - 2)

	if gv.FocusArea == GitFocusCommitInput && isFocused {
		inputBorder = inputBorder.BorderForeground(ui.CurrentTheme.Accent)
	}

	commitRender := inputBorder.Render(gv.CommitInput.View())
	commitLines := strings.Split(commitRender, "\n")
	for i, cLine := range commitLines {
		gv.ItemClickMap[currentY+i] = GitClickTarget{Type: ClickCommitInput}
		lines = append(lines, cLine)
	}
	currentY += len(commitLines)

	// Separator
	lines = append(lines, "")
	currentY++

	// 2. Section Staged Changes
	stagedFoldIcon := "▼"
	if gv.StagedFolded {
		stagedFoldIcon = "▶"
	}
	stagedHeader := fmt.Sprintf(" %s Staged Changes (%d)", stagedFoldIcon, len(gv.Staged))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.CurrentTheme.FgHighlight)
	lines = append(lines, headerStyle.Render(stagedHeader))
	gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickSectionStaged}
	currentY++

	if !gv.StagedFolded {
		if len(gv.Staged) == 0 {
			emptyStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
			lines = append(lines, emptyStyle.Render("   (No staged changes)"))
			currentY++
		} else {
			for i, item := range gv.Staged {
				isSelected := isFocused && gv.FocusArea == GitFocusStaged && gv.StagedIdx == i
				badge := formatBadge(item.Status)
				btn := "[-]"
				lineText := fmt.Sprintf(" %s %s %s", badge, btn, item.Name)

				rowStyle := lipgloss.NewStyle()
				if isSelected {
					rowStyle = rowStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
				}
				lines = append(lines, rowStyle.Render(truncateOrPad(lineText, width-1)))
				gv.ItemClickMap[currentY] = GitClickTarget{
					Type:  ClickItemStaged,
					Index: i,
					Path:  item.Path,
				}
				currentY++
			}
		}
	}

	// Separator
	lines = append(lines, "")
	currentY++

	// 3. Section Changes (Unstaged)
	changesFoldIcon := "▼"
	if gv.ChangesFolded {
		changesFoldIcon = "▶"
	}
	changesHeader := fmt.Sprintf(" %s Changes (%d)", changesFoldIcon, len(gv.Changes))
	lines = append(lines, headerStyle.Render(changesHeader))
	gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickSectionChanges}
	currentY++

	if !gv.ChangesFolded {
		if len(gv.Changes) == 0 {
			emptyStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
			lines = append(lines, emptyStyle.Render("   (Working tree clean)"))
			currentY++
		} else {
			for i, item := range gv.Changes {
				isSelected := isFocused && gv.FocusArea == GitFocusChanges && gv.ChangesIdx == i
				badge := formatBadge(item.Status)
				btn := "[+]"
				lineText := fmt.Sprintf(" %s %s %s", badge, btn, item.Name)

				rowStyle := lipgloss.NewStyle()
				if isSelected {
					rowStyle = rowStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
				}
				lines = append(lines, rowStyle.Render(truncateOrPad(lineText, width-1)))
				gv.ItemClickMap[currentY] = GitClickTarget{
					Type:  ClickItemChanges,
					Index: i,
					Path:  item.Path,
				}
				currentY++
			}
		}
	}

	// Separator
	lines = append(lines, "")
	currentY++

	// 4. Section Commit History
	histFoldIcon := "▼"
	if gv.HistoryFolded {
		histFoldIcon = "▶"
	}
	histHeader := fmt.Sprintf(" %s History (%d)", histFoldIcon, len(gv.History))
	lines = append(lines, headerStyle.Render(histHeader))
	gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickSectionHistory}
	currentY++

	if !gv.HistoryFolded {
		if len(gv.History) == 0 {
			emptyStyle := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
			lines = append(lines, emptyStyle.Render("   (No commit history)"))
			currentY++
		} else {
			for i, commit := range gv.History {
				isSelected := isFocused && gv.FocusArea == GitFocusHistory && gv.HistoryIdx == i
				hashBadge := lipgloss.NewStyle().Foreground(ui.CurrentTheme.Yellow).Render(commit.Hash)
				timeBadge := lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary).Render(commit.RelativeTime())
				lineText := fmt.Sprintf(" %s %s %s", hashBadge, commit.Message, timeBadge)

				rowStyle := lipgloss.NewStyle()
				if isSelected {
					rowStyle = rowStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
				}
				lines = append(lines, rowStyle.Render(truncateOrPad(lineText, width-1)))
				gv.ItemClickMap[currentY] = GitClickTarget{
					Type:  ClickItemHistory,
					Index: i,
				}
				currentY++
			}
		}
	}

	// Pad or truncate to height
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func formatBadge(status string) string {
	switch status {
	case "M":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#CCA700")).Bold(true).Render("M")
	case "A":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#89D185")).Bold(true).Render("A")
	case "D":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C")).Bold(true).Render("D")
	case "U", "?":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#73C991")).Bold(true).Render("?")
	default:
		return status
	}
}
