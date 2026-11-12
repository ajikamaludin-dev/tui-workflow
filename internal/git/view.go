package git

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	CommitInput textarea.Model

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
	TotalLines   int
	LastWidth    int
	LastHeight   int

	UnpushedCount int
	IsLoading     bool
	LoadingMsg    string
	SpinnerFrame  int

	AIButtonStartX int

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
	ClickButtonRow
	ClickStageAllAction
	ClickUnstageAllAction
	ClickAICommitAction
)

type GitClickTarget struct {
	Type  GitClickType
	Index int
	Path  string
}

func NewGitView(repo *Repo) *GitView {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = "Message (Ctrl+Enter to commit)"
	ta.CharLimit = 500
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgSecondary)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgPrimary)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgPrimary)

	gv := &GitView{
		Repo:         repo,
		CommitInput:  ta,
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

	var prevStagedPath, prevChangesPath string
	if gv.StagedIdx >= 0 && gv.StagedIdx < len(gv.Staged) {
		prevStagedPath = gv.Staged[gv.StagedIdx].Path
	}
	if gv.ChangesIdx >= 0 && gv.ChangesIdx < len(gv.Changes) {
		prevChangesPath = gv.Changes[gv.ChangesIdx].Path
	}

	staged, unstaged, _ := gv.Repo.GetStatus()
	gv.Staged = staged
	gv.Changes = unstaged

	if prevStagedPath != "" {
		for i, f := range gv.Staged {
			if f.Path == prevStagedPath {
				gv.StagedIdx = i
				break
			}
		}
	}
	if prevChangesPath != "" {
		for i, f := range gv.Changes {
			if f.Path == prevChangesPath {
				gv.ChangesIdx = i
				break
			}
		}
	}

	history, _ := gv.Repo.GetHistory(10)
	gv.History = history

	unpushed, _ := gv.Repo.GetUnpushedCount()
	gv.UnpushedCount = unpushed

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

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (gv *GitView) AdvanceSpinner() {
	gv.SpinnerFrame = (gv.SpinnerFrame + 1) % len(spinnerFrames)
}

func (gv *GitView) StageAll() error {
	if gv.Repo == nil {
		return fmt.Errorf("repositori git tidak ditemukan")
	}
	err := gv.Repo.StageAll()
	if err != nil {
		return err
	}
	gv.Refresh()
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

func (gv *GitView) UnstageAll() error {
	if gv.Repo == nil {
		return fmt.Errorf("repositori git tidak ditemukan")
	}
	err := gv.Repo.UnstageAll()
	if err != nil {
		return err
	}
	gv.Refresh()
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

func (gv *GitView) Push() error {
	if gv.Repo == nil {
		return fmt.Errorf("repositori git tidak ditemukan")
	}
	err := gv.Repo.Push()
	if err != nil {
		return err
	}
	gv.Refresh()
	return nil
}

func (gv *GitView) ScrollUp(delta int) {
	gv.ScrollOffset -= delta
	if gv.ScrollOffset < 0 {
		gv.ScrollOffset = 0
	}
}

func (gv *GitView) ScrollDown(delta int, height int) {
	maxScroll := gv.TotalLines - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	gv.ScrollOffset += delta
	if gv.ScrollOffset > maxScroll {
		gv.ScrollOffset = maxScroll
	}
}

func (gv *GitView) ensureVisible() {
	height := gv.LastHeight
	if height <= 0 {
		height = 20
	}
	targetLine := -1
	switch gv.FocusArea {
	case GitFocusCommitInput:
		targetLine = 0
	case GitFocusStaged:
		for line, tgt := range gv.ItemClickMap {
			if tgt.Type == ClickItemStaged && tgt.Index == gv.StagedIdx {
				targetLine = line
				break
			}
		}
	case GitFocusChanges:
		for line, tgt := range gv.ItemClickMap {
			if tgt.Type == ClickItemChanges && tgt.Index == gv.ChangesIdx {
				targetLine = line
				break
			}
		}
	case GitFocusHistory:
		for line, tgt := range gv.ItemClickMap {
			if tgt.Type == ClickItemHistory && tgt.Index == gv.HistoryIdx {
				targetLine = line
				break
			}
		}
	}

	if targetLine < 0 {
		return
	}
	if targetLine < gv.ScrollOffset {
		gv.ScrollOffset = targetLine
	} else if targetLine >= gv.ScrollOffset+height {
		gv.ScrollOffset = targetLine - height + 1
	}
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
				if gv.CommitInput.Line() > 0 || gv.CommitInput.LineInfo().RowOffset > 0 {
					var cmd tea.Cmd
					gv.CommitInput, cmd = gv.CommitInput.Update(msg)
					return cmd, nil
				}
				// stay at top row
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
				isLastLine := gv.CommitInput.Line() >= gv.CommitInput.LineCount()-1
				lineInfo := gv.CommitInput.LineInfo()
				isLastRow := lineInfo.RowOffset >= lineInfo.Height-1
				if isLastLine && isLastRow {
					gv.FocusArea = GitFocusStaged
					gv.CommitInput.Blur()
				} else {
					var cmd tea.Cmd
					gv.CommitInput, cmd = gv.CommitInput.Update(msg)
					return cmd, nil
				}
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

	gv.ensureVisible()
	return nil, fileToOpen
}

func (gv *GitView) HandleMouseClick(relX, relY int) (*GitFileStatus, string, bool) {
	actualLine := gv.ScrollOffset + relY
	target, ok := gv.ItemClickMap[actualLine]
	if !ok {
		return nil, "", false
	}

	switch target.Type {
	case ClickCommitInput:
		if gv.AIButtonStartX > 0 && relX >= gv.AIButtonStartX {
			if gv.IsLoading {
				return nil, "", true
			}
			return nil, "ai_commit", true
		}
		gv.FocusArea = GitFocusCommitInput
		gv.CommitInput.Focus()
		return nil, "", true

	case ClickButtonRow:
		gv.CommitInput.Blur()
		if gv.IsLoading {
			return nil, "", true
		}
		hasChanges := len(gv.Staged) > 0 || len(gv.Changes) > 0
		if hasChanges {
			return nil, "commit", true
		} else if gv.UnpushedCount > 0 {
			return nil, "push", true
		}
		return nil, "", true

	case ClickAICommitAction:
		gv.CommitInput.Blur()
		if gv.IsLoading {
			return nil, "", true
		}
		return nil, "ai_commit", true

	case ClickSectionStaged:
		gv.CommitInput.Blur()
		btnCol := (gv.LastWidth - 1) - 5
		if btnCol < 10 {
			btnCol = 10
		}
		if len(gv.Staged) > 0 && relX >= btnCol {
			_ = gv.UnstageAll()
			return nil, "", true
		}
		gv.StagedFolded = !gv.StagedFolded
		return nil, "", true

	case ClickSectionChanges:
		gv.CommitInput.Blur()
		btnCol := (gv.LastWidth - 1) - 5
		if btnCol < 10 {
			btnCol = 10
		}
		if len(gv.Changes) > 0 && relX >= btnCol {
			_ = gv.StageAll()
			return nil, "", true
		}
		gv.ChangesFolded = !gv.ChangesFolded
		return nil, "", true

	case ClickSectionHistory:
		gv.HistoryFolded = !gv.HistoryFolded
		return nil, "", true

	case ClickItemStaged:
		gv.FocusArea = GitFocusStaged
		gv.StagedIdx = target.Index
		gv.CommitInput.Blur()
		// Clicking [-] button (columns 2..6) unstages the file
		if relX >= 2 && relX <= 6 {
			_ = gv.Repo.UnstageFile(target.Path)
			gv.Refresh()
			return nil, "", true
		}
		if target.Index >= 0 && target.Index < len(gv.Staged) {
			return &gv.Staged[target.Index], "", true
		}

	case ClickItemChanges:
		gv.FocusArea = GitFocusChanges
		gv.ChangesIdx = target.Index
		gv.CommitInput.Blur()
		// Clicking [+] button (columns 2..5) stages the file
		if relX >= 2 && relX <= 5 {
			_ = gv.Repo.StageFile(target.Path)
			gv.Refresh()
			return nil, "", true
		}
		// Clicking [x] button (columns 6..10) requests discard
		if relX >= 6 && relX <= 10 {
			if target.Index >= 0 && target.Index < len(gv.Changes) {
				return &gv.Changes[target.Index], "discard", true
			}
		}
		if target.Index >= 0 && target.Index < len(gv.Changes) {
			return &gv.Changes[target.Index], "", true
		}
	case ClickStageAction:
		_ = gv.Repo.StageFile(target.Path)
		gv.Refresh()
		return nil, "", true

	case ClickUnstageAction:
		_ = gv.Repo.UnstageFile(target.Path)
		gv.Refresh()
		return nil, "", true

	case ClickStageAllAction:
		_ = gv.StageAll()
		return nil, "", true

	case ClickUnstageAllAction:
		_ = gv.UnstageAll()
		return nil, "", true
	}

	return nil, "", false
}

func (gv *GitView) Render(width, height int, isFocused bool) string {
	gv.ItemClickMap = make(map[int]GitClickTarget)
	gv.LastWidth = width
	gv.LastHeight = height

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

	var lines []string
	currentY := 0

	// 1. Commit Box with AI suggest button at top-right corner
	aiBtnStr := "✨"
	if width >= 32 {
		aiBtnStr = "✨ AI"
	}
	aiBtnStyled := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.Yellow).
		Bold(true).
		Render(aiBtnStr)

	aiBtnWidth := ansi.StringWidth(aiBtnStr)
	gv.AIButtonStartX = (width - 2) - aiBtnWidth - 2

	inputWidth := width - 2
	if inputWidth < 8 {
		inputWidth = 8
	}
	gv.CommitInput.SetWidth(inputWidth)
	gv.CommitInput.SetHeight(3)

	borderColor := ui.CurrentTheme.BorderColor
	if gv.FocusArea == GitFocusCommitInput && isFocused {
		borderColor = ui.CurrentTheme.Accent
	}

	inputBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(inputWidth)

	commitRender := inputBorder.Render(gv.CommitInput.View())
	commitLines := strings.Split(commitRender, "\n")

	// Inject AI suggest button into top border
	if len(commitLines) > 0 {
		start := gv.AIButtonStartX
		if start-2 >= 1 && width-start-aiBtnWidth-2 >= 0 {
			borderStyle := lipgloss.NewStyle().Foreground(borderColor)
			leftDashes := strings.Repeat("─", start-2)
			rightDashes := strings.Repeat("─", width-start-aiBtnWidth-2)
			commitLines[0] = borderStyle.Render("╭"+leftDashes) + " " + aiBtnStyled + " " + borderStyle.Render(rightDashes+"╮")
		}
	}

	for i, cLine := range commitLines {
		gv.ItemClickMap[currentY+i] = GitClickTarget{Type: ClickCommitInput}
		lines = append(lines, cLine)
	}
	currentY += len(commitLines)

	// Buttons: Dynamic fullwidth button (Commit vs Push vs Loading)
	btnW := width - 2
	if btnW < 10 {
		btnW = 10
	}
	btnStyle := lipgloss.NewStyle().
		Bold(true).
		Background(ui.CurrentTheme.Accent).
		Foreground(ui.CurrentTheme.FgHighlight).
		Width(btnW).
		Align(lipgloss.Center)

	hasChanges := len(gv.Staged) > 0 || len(gv.Changes) > 0
	if gv.IsLoading {
		txt := gv.LoadingMsg
		if txt == "" {
			txt = "Loading..."
		}
		frame := spinnerFrames[gv.SpinnerFrame%len(spinnerFrames)]
		lines = append(lines, " "+btnStyle.Render(fmt.Sprintf("%s %s", frame, txt)))
		gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickButtonRow}
		currentY++
	} else if hasChanges {
		lines = append(lines, " "+btnStyle.Render("Commit"))
		gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickButtonRow}
		currentY++
	} else if gv.UnpushedCount > 0 {
		pushTxt := fmt.Sprintf("Push (%d)", gv.UnpushedCount)
		lines = append(lines, " "+btnStyle.Render(pushTxt))
		gv.ItemClickMap[currentY] = GitClickTarget{Type: ClickButtonRow}
		currentY++
	}

	// Separator
	lines = append(lines, "")
	currentY++

	// 2. Section Staged Changes
	stagedFoldIcon := "▼"
	if gv.StagedFolded {
		stagedFoldIcon = "▶"
	}
	stagedTitle := fmt.Sprintf(" %s Staged Changes (%d)", stagedFoldIcon, len(gv.Staged))
	stagedBtn := ""
	if len(gv.Staged) > 0 {
		stagedBtn = "[-] "
	}
	availSpace := (width - 1) - ansi.StringWidth(stagedTitle) - ansi.StringWidth(stagedBtn)
	if availSpace < 0 {
		availSpace = 0
	}
	stagedHeader := stagedTitle + strings.Repeat(" ", availSpace) + stagedBtn
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
				prefix := fmt.Sprintf(" %s %s ", badge, btn)
				availWidth := (width - 1) - ansi.StringWidth(prefix)
				if availWidth < 0 {
					availWidth = 0
				}
				nameTruncated := ansi.Truncate(item.Name, availWidth, "…")
				lineText := prefix + nameTruncated

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
	changesTitle := fmt.Sprintf(" %s Changes (%d)", changesFoldIcon, len(gv.Changes))
	changesBtn := ""
	if len(gv.Changes) > 0 {
		changesBtn = "[+] "
	}
	availSpace = (width - 1) - ansi.StringWidth(changesTitle) - ansi.StringWidth(changesBtn)
	if availSpace < 0 {
		availSpace = 0
	}
	changesHeader := changesTitle + strings.Repeat(" ", availSpace) + changesBtn
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
				btn := "[+] [x]"
				prefix := fmt.Sprintf(" %s %s ", badge, btn)
				availWidth := (width - 1) - ansi.StringWidth(prefix)
				if availWidth < 0 {
					availWidth = 0
				}
				nameTruncated := ansi.Truncate(item.Name, availWidth, "…")
				lineText := prefix + nameTruncated

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
				prefix := fmt.Sprintf(" %s ", hashBadge)
				availWidth := (width - 1) - ansi.StringWidth(prefix)
				if availWidth < 0 {
					availWidth = 0
				}
				msgTruncated := ansi.Truncate(commit.Message, availWidth, "…")
				lineText := prefix + msgTruncated

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

	// Viewport scrolling
	gv.TotalLines = len(lines)
	maxScroll := gv.TotalLines - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if gv.ScrollOffset > maxScroll {
		gv.ScrollOffset = maxScroll
	}
	if gv.ScrollOffset < 0 {
		gv.ScrollOffset = 0
	}

	var visibleLines []string
	end := gv.ScrollOffset + height
	if end > gv.TotalLines {
		end = gv.TotalLines
	}
	if gv.ScrollOffset < gv.TotalLines {
		visibleLines = lines[gv.ScrollOffset:end]
	}

	for len(visibleLines) < height {
		visibleLines = append(visibleLines, "")
	}
	if len(visibleLines) > height {
		visibleLines = visibleLines[:height]
	}
	return strings.Join(visibleLines, "\n")
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
