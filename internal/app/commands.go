package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/ai"
	"tui-workflow/internal/config"
	"tui-workflow/internal/formatter"
	"tui-workflow/internal/git"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/symbol"
	"tui-workflow/internal/terminal"
	"tui-workflow/internal/ui/dialog"
)

type MarqueeTickMsg time.Time

func marqueeTickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return MarqueeTickMsg(t)
	})
}

type gitLoadingTickMsg struct{}

func gitLoadingTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return gitLoadingTickMsg{}
	})
}

type symbolIndexReadyMsg struct {
	count int
	err   error
}

func buildSymbolIndexCmd(ix *symbol.Index) tea.Cmd {
	return func() tea.Msg {
		if ix == nil {
			return symbolIndexReadyMsg{count: 0, err: nil}
		}
		err := ix.Build()
		return symbolIndexReadyMsg{count: ix.Count(), err: err}
	}
}

type gitStatusResultMsg struct {
	data git.RefreshData
}

func gitRefreshCmd(repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitStatusResultMsg{data: git.RefreshData{}}
		}
		data := git.FetchRefreshData(repo)
		return gitStatusResultMsg{data: data}
	}
}

type quickOpenWarmupMsg struct{}

func warmupQuickOpenCmd(qo *dialog.QuickOpen, rootDir string) tea.Cmd {
	return func() tea.Msg {
		if qo != nil && rootDir != "" {
			qo.Warmup(rootDir)
		}
		return quickOpenWarmupMsg{}
	}
}

type lspCompletionResultMsg struct {
	Items  []lsp.CompletionItem
	X, Y   int
	Prefix string
}

func (m Model) triggerLSPCompletionCmd() tea.Cmd {
	_, cmd := m.handleCtrlSpace()
	return cmd
}

func (m *Model) triggerGitRefresh() tea.Cmd {
	if m.GitView == nil || m.GitRepo == nil {
		return nil
	}
	if m.GitView.IsLoading {
		return gitLoadingTickCmd()
	}
	m.GitView.IsLoading = true
	m.GitView.LoadingMsg = "Memeriksa status git..."
	return tea.Batch(gitRefreshCmd(m.GitRepo), gitLoadingTickCmd())
}

type gitPushResultMsg struct {
	err error
}

func gitPushCmd(repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitPushResultMsg{err: fmt.Errorf("git repository not found")}
		}
		err := repo.Push()
		return gitPushResultMsg{err: err}
	}
}

type gitCommitResultMsg struct {
	hash string
	err  error
}

func (m *Model) triggerGitCommit() (Model, tea.Cmd) {
	if m.GitView == nil {
		return *m, nil
	}
	if m.GitView.IsLoading {
		return *m, nil
	}
	if len(m.GitView.Staged) == 0 {
		m.StatusBar.ShowToast("No changes in staging area", true, 3*time.Second)
		return *m, nil
	}
	if m.GitRepo == nil {
		m.StatusBar.ShowToast("Git repository not found", true, 3*time.Second)
		return *m, nil
	}

	// Proactively check git user identity on host
	if _, err := m.GitRepo.GetUserSignature(); err != nil {
		var userErr *git.ErrGitUserNotConfigured
		if errors.As(err, &userErr) {
			m.openGitUserNotConfiguredAlert()
			m.StatusBar.ShowToast("Commit failed: Git user identity not configured", true, 4*time.Second)
			return *m, nil
		}
	}

	commitMsg := m.GitView.CommitInput.Value()
	m.GitView.IsLoading = true
	m.GitView.LoadingMsg = "Committing..."
	m.GitView.SpinnerFrame = 0
	m.StatusBar.ShowToast("Melakukan commit...", false, 3*time.Second)
	return *m, tea.Batch(gitCommitCmd(m.GitRepo, commitMsg), gitLoadingTickCmd())
}

func (m *Model) openGitUserNotConfiguredAlert() {
	title := "Git Identity Required"
	msg := "Git user name and email are not configured on this host.\n\n" +
		"Please configure your Git identity using terminal:\n" +
		"  git config --global user.name \"Your Name\"\n" +
		"  git config --global user.email \"you@example.com\"\n\n" +
		"Or configure for this repository only:\n" +
		"  git config user.name \"Your Name\"\n" +
		"  git config user.email \"you@example.com\""

	m.ConfirmDialog.Open(dialog.ConfirmAlert, title, msg, "")
}

func gitCommitCmd(repo *git.Repo, commitMsg string) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitCommitResultMsg{err: fmt.Errorf("git repository not found")}
		}
		hash, err := repo.Commit(commitMsg)
		return gitCommitResultMsg{hash: hash, err: err}
	}
}

type FormatCompleteMsg struct {
	TabID    string
	NewText  string
	ToolName string
	Err      error
}

func formatBufferCmd(tabID string, content string, filePath string, workspaceDir string, overrides map[string][]string) tea.Cmd {
	return func() tea.Msg {
		req := formatter.FormatRequest{
			Content:         content,
			FilePath:        filePath,
			WorkspaceDir:    workspaceDir,
			CustomOverrides: overrides,
		}
		res, err := formatter.Format(req)
		if err != nil {
			return FormatCompleteMsg{
				TabID: tabID,
				Err:   err,
			}
		}
		return FormatCompleteMsg{
			TabID:    tabID,
			NewText:  res.Formatted,
			ToolName: res.ToolName,
		}
	}
}

func (m *Model) triggerFormatCurrentBuffer() (Model, tea.Cmd) {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil || tab.IsDiff {
		m.StatusBar.ShowToast("No active file to format", true, 3*time.Second)
		return *m, nil
	}

	content := tab.Buffer.Text()
	filePath := tab.Path
	tabID := tab.ID
	workspaceDir := m.RootDir
	var overrides map[string][]string
	if m.Config != nil {
		overrides = m.Config.Formatters
	}

	m.StatusBar.ShowToast("Memformat dokumen...", false, 2*time.Second)
	return *m, formatBufferCmd(tabID, content, filePath, workspaceDir, overrides)
}

type TerminalOutputMsg struct {
	TabID string
}

type TerminalExitMsg struct {
	TabID string
	Err   error
}

func waitForTerminalOutput(term *terminal.TerminalView) tea.Cmd {
	if term == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-term.OutputChan()
		if !ok {
			return TerminalExitMsg{TabID: term.ID}
		}
		return TerminalOutputMsg{TabID: term.ID}
	}
}

func (m *Model) openNewTerminalTab() (Model, tea.Cmd) {
	contentWidth := m.editorContentWidth()
	bodyHeight := m.editorBodyHeight()
	if contentWidth < 20 {
		contentWidth = 80
	}
	if bodyHeight < 5 {
		bodyHeight = 24
	}

	termID := fmt.Sprintf("term:%d", time.Now().UnixNano())
	term, err := terminal.NewTerminalView(termID, contentWidth, bodyHeight, "", m.RootDir)
	if err != nil {
		m.StatusBar.ShowToast(fmt.Sprintf("Failed to open terminal: %v", err), true, 3*time.Second)
		return *m, nil
	}

	m.TabManager.OpenTerminal(termID, term.Title, term)
	m.Focus = FocusEditor
	m.syncStatusBarWithActiveTab()
	m.StatusBar.ShowToast("Terminal dibuka", false, 2*time.Second)

	return *m, waitForTerminalOutput(term)
}

type aiCommitGeneratedMsg struct {
	message string
}

type aiCommitErrorMsg struct {
	err error
}

func aiGenerateCommitCmd(client *ai.Client, cfg config.AIConfig, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return aiCommitErrorMsg{err: fmt.Errorf("git repository not found")}
		}
		diff, err := repo.GetStagedDiff()
		if err != nil {
			return aiCommitErrorMsg{err: err}
		}
		if strings.TrimSpace(diff) == "" {
			return aiCommitErrorMsg{err: fmt.Errorf("staging area kosong, stage file terlebih dahulu")}
		}
		msg, err := client.GenerateCommitMessage(context.Background(), cfg, diff)
		if err != nil {
			return aiCommitErrorMsg{err: err}
		}
		return aiCommitGeneratedMsg{message: msg}
	}
}

func (m *Model) triggerAICommit() (Model, tea.Cmd) {
	if m.GitRepo == nil || m.GitView == nil {
		m.StatusBar.ShowToast("Not a git repository", true, 3*time.Second)
		return *m, nil
	}
	if m.GitView.IsLoading {
		return *m, nil
	}
	if len(m.GitView.Staged) == 0 {
		m.StatusBar.ShowToast("No changes in staging area (+ to stage)", true, 3*time.Second)
		return *m, nil
	}
	m.GitView.IsLoading = true
	m.GitView.LoadingMsg = "AI Generating..."
	m.GitView.SpinnerFrame = 0
	m.StatusBar.ShowToast("🤖 Menghasilkan commit message via AI...", false, 4*time.Second)
	return *m, tea.Batch(aiGenerateCommitCmd(m.AIClient, m.Config.AI, m.GitRepo), gitLoadingTickCmd())
}

type FileWatchTickMsg time.Time

func fileWatchTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return FileWatchTickMsg(t)
	})
}
