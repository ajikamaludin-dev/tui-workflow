package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/ai"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/explorer"
	"tui-workflow/internal/formatter"
	"tui-workflow/internal/git"
	"tui-workflow/internal/search"
	"tui-workflow/internal/terminal"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

type Model struct {
	RootDir           string
	Config            *config.Config
	Width             int
	Height            int
	Ready             bool
	Focus             FocusArea
	SidebarVisible    bool
	SidebarWidth      int
	SidebarView       SidebarView
	MenuBar           *ui.MenuBar
	StatusBar         *ui.StatusBar
	Explorer          *explorer.Tree
	GitRepo           *git.Repo
	GitView           *git.GitView
	TabManager        *editor.TabManager
	FilePicker        *dialog.FilePicker
	InputDialog       *dialog.InputDialog
	ConfirmDialog     *dialog.ConfirmDialog
	Switcher          *dialog.BufferSwitcher
	QuickOpen         *dialog.QuickOpen
	CommandPalette    *dialog.CommandPalette
	GlobalSearch      *dialog.GlobalSearch
	FindBar           *dialog.FindBar
	BranchSwitcher    *dialog.BranchSwitcher
	AISettingsDialog  *dialog.AISettingsDialog
	HelpDialog        *dialog.HelpDialog
	AIClient          *ai.Client
	SearchView        *search.SearchView
	ContextMenu       *ui.ContextMenu
	AutoSave          bool
	SoftWrap          bool
	ShowHelp          bool
	ShowAbout         bool
	ClosingTabIdx     int
	ContextMenuTabIdx int
	ResizingSidebar   bool
	lastPointerShape  string

	SessionExplicitArg bool

	// Pending git hunk discard confirmation
	pendingGitHunkPath  string
	pendingGitHunkIndex int
	SessionDirtySave   bool
}

func NewModelWithPath(targetPath string, explicitArg bool) (Model, error) {
	if targetPath == "" {
		targetPath = "."
	}
	cleanPath, err := filepath.Abs(targetPath)
	if err != nil {
		cleanPath = targetPath
	}
	fi, err := os.Stat(cleanPath)
	if err != nil {
		return Model{}, fmt.Errorf("path tidak ditemukan: %w", err)
	}

	var rootDir string
	var initialFile string
	openPRDByDefault := false

	if fi.IsDir() {
		rootDir = cleanPath
		prdPath := filepath.Join(rootDir, "PRD.md")
		if _, err := os.Stat(prdPath); err == nil {
			if explicitArg {
				initialFile = prdPath
			} else {
				openPRDByDefault = true
			}
		}
	} else {
		rootDir = filepath.Dir(cleanPath)
		initialFile = cleanPath
	}

	tm := editor.NewTabManager()
	if initialFile != "" {
		_, _ = tm.OpenFile(initialFile)
	}

	repo, _ := git.OpenRepo(rootDir)
	var gv *git.GitView
	if repo != nil {
		gv = git.NewGitView(repo)
	}

	cfg, _ := config.LoadConfig()

	initialSidebarWidth := 30
	if cfg.SidebarWidth > 0 {
		initialSidebarWidth = cfg.SidebarWidth
	}
	if initialSidebarWidth < 15 {
		initialSidebarWidth = 15
	}

	m := Model{
		RootDir:           rootDir,
		Config:            cfg,
		Focus:             FocusEditor,
		SidebarVisible:    true,
		SidebarWidth:      initialSidebarWidth,
		SidebarView:       SidebarFiles,
		MenuBar:           ui.NewMenuBar(),
		StatusBar:         ui.NewStatusBar(),
		Explorer:          explorer.NewTree(rootDir),
		GitRepo:           repo,
		GitView:           gv,
		TabManager:        tm,
		FilePicker:        dialog.NewFilePicker(),
		InputDialog:       dialog.NewInputDialog(),
		ConfirmDialog:     dialog.NewConfirmDialog(),
		Switcher:          dialog.NewBufferSwitcher(),
		QuickOpen:         dialog.NewQuickOpen(),
		CommandPalette:    dialog.NewCommandPalette(),
		GlobalSearch:      dialog.NewGlobalSearch(),
		FindBar:           dialog.NewFindBar(),
		BranchSwitcher:    dialog.NewBranchSwitcher(),
		AISettingsDialog:  dialog.NewAISettingsDialog(),
		HelpDialog:        dialog.NewHelpDialog(),
		AIClient:          ai.NewClient(),
		SearchView:        search.NewSearchView(rootDir),
		ContextMenu:       ui.NewContextMenu(),
		AutoSave:          cfg.AutoSave,
		SoftWrap:          cfg.SoftWrap,
		ShowHelp:          false,
		ShowAbout:         false,
		ClosingTabIdx:     -1,
		ContextMenuTabIdx: -1,
		SessionExplicitArg: explicitArg,
	}
	m.Explorer.ShowHidden = cfg.ShowHidden
	m.StatusBar.AutoSave = cfg.AutoSave
	m.TabManager.SetSoftWrap(cfg.SoftWrap)
	if repo != nil {
		if branch, err := repo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	}
	m.syncStatusBarWithActiveTab()

	if !explicitArg {
		if sess, err := config.LoadWorkspaceSession(m.canonicalRootDir()); err == nil && sess != nil && len(sess.OpenTabs) > 0 {
			restored, skipped := m.restoreWorkspaceSession(sess)
			if restored > 0 {
				msg := fmt.Sprintf("Session dipulihkan (%d tab)", restored)
				if len(skipped) > 0 {
					msg += fmt.Sprintf(", %d dilewati", len(skipped))
				}
				m.StatusBar.ShowToast(msg, false, 3*time.Second)
				m.syncStatusBarWithActiveTab()
			} else if openPRDByDefault {
				prdPath := filepath.Join(rootDir, "PRD.md")
				if _, err := os.Stat(prdPath); err == nil {
					_, _ = tm.OpenFile(prdPath)
					m.syncStatusBarWithActiveTab()
				}
			}
		} else if openPRDByDefault {
			prdPath := filepath.Join(rootDir, "PRD.md")
			if _, err := os.Stat(prdPath); err == nil {
				_, _ = tm.OpenFile(prdPath)
				m.syncStatusBarWithActiveTab()
			}
		}
	}

	return m, nil
}

func NewModel() Model {
	m, _ := NewModelWithPath(".", false)
	return m
}

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

type gitPushResultMsg struct {
	err error
}

func gitPushCmd(repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitPushResultMsg{err: fmt.Errorf("repositori git tidak ditemukan")}
		}
		err := repo.Push()
		return gitPushResultMsg{err: err}
	}
}

type gitCommitResultMsg struct {
	hash string
	err  error
}

func gitCommitCmd(repo *git.Repo, commitMsg string) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitCommitResultMsg{err: fmt.Errorf("repositori git tidak ditemukan")}
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
		m.StatusBar.ShowToast("Tidak ada file aktif untuk diformat", true, 3*time.Second)
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
	contentWidth := m.Width - m.SidebarWidth
	if !m.SidebarVisible {
		contentWidth = m.Width
	}
	bodyHeight := m.Height - 3 // Row 0: Menu, Row 1: Tab bar, Last row: Status bar
	if contentWidth < 20 {
		contentWidth = 80
	}
	if bodyHeight < 5 {
		bodyHeight = 24
	}

	termID := fmt.Sprintf("term:%d", time.Now().UnixNano())
	term, err := terminal.NewTerminalView(termID, contentWidth, bodyHeight, "", m.RootDir)
	if err != nil {
		m.StatusBar.ShowToast(fmt.Sprintf("Gagal membuka terminal: %v", err), true, 3*time.Second)
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
			return aiCommitErrorMsg{err: fmt.Errorf("repositori git tidak ditemukan")}
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
		m.StatusBar.ShowToast("Bukan repositori git", true, 3*time.Second)
		return *m, nil
	}
	if m.GitView.IsLoading {
		return *m, nil
	}
	if len(m.GitView.Staged) == 0 {
		m.StatusBar.ShowToast("Tidak ada perubahan di staging area (+ untuk stage)", true, 3*time.Second)
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(marqueeTickCmd(), fileWatchTickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionPersistMsg:
		// best-effort persistence; avoid noisy errors
		return m, nil
	case FileWatchTickMsg:
		m.checkWatchedFiles()
		return m, fileWatchTickCmd()

	case gitLoadingTickMsg:
		if m.GitView != nil && m.GitView.IsLoading {
			m.GitView.AdvanceSpinner()
			return m, gitLoadingTickCmd()
		}
		return m, nil

	case gitPushResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				m.StatusBar.ShowToast("Push gagal: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Push berhasil", false, 3*time.Second)
			}
		}
		return m, nil

	case gitCommitResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				m.StatusBar.ShowToast("Commit gagal: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.CommitInput.SetValue("")
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Commit sukses: "+msg.hash, false, 3*time.Second)
				if m.GitRepo != nil {
					if branch, err := m.GitRepo.GetBranchName(); err == nil {
						m.StatusBar.Branch = branch
					}
				}
			}
		}
		return m, nil

	case aiCommitGeneratedMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			m.GitView.CommitInput.SetValue(msg.message)
			m.GitView.CommitInput.Focus()
			m.GitView.FocusArea = git.GitFocusCommitInput
		}
		m.StatusBar.ShowToast("Saran commit message berhasil disarankan AI (Ctrl+Enter untuk commit)", false, 4*time.Second)
		return m, nil

	case aiCommitErrorMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
		}
		m.StatusBar.ShowToast(fmt.Sprintf("AI Error: %v", msg.err), true, 4*time.Second)
		return m, nil

	case FormatCompleteMsg:
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.ID == msg.TabID && tab.Buffer != nil {
			if msg.Err != nil {
				m.StatusBar.ShowToast("Format: "+msg.Err.Error(), true, 4*time.Second)
			} else {
				if msg.NewText != tab.Buffer.Text() {
					tab.Buffer.ReplaceContentWithUndo(msg.NewText)
					m.StatusBar.ShowToast("✓ Formatted with "+msg.ToolName, false, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Dokumen sudah rapi ("+msg.ToolName+")", false, 3*time.Second)
				}
			}
		}
		return m, nil

	case MarqueeTickMsg:
		if m.SidebarVisible && m.SidebarView == SidebarFiles && m.Focus == FocusSidebar && m.Explorer != nil {
			if m.Explorer.SelectedNameExceeds(m.SidebarWidth - 1) {
				m.Explorer.TickMarquee()
				return m, marqueeTickCmd()
			}
		}
		// When not needed or idle outside sidebar, check back in 1 second instead of 250ms
		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return MarqueeTickMsg(t)
		})

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.MenuBar.SetWidth(m.Width)
		m.StatusBar.SetWidth(m.Width)
		if m.QuickOpen != nil {
			m.QuickOpen.SetWidth(m.modalWidth())
		}
		if m.CommandPalette != nil {
			m.CommandPalette.SetWidth(m.modalWidth())
		}
		if m.GlobalSearch != nil {
			m.GlobalSearch.SetWidth(m.modalWidth())
		}
		contentWidth := m.Width - m.SidebarWidth
		if !m.SidebarVisible {
			contentWidth = m.Width
		}
		bodyHeight := m.Height - 3
		for _, tab := range m.TabManager.Tabs {
			if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				tab.TerminalView.Resize(contentWidth, bodyHeight)
			}
		}
		return m, nil

	case TerminalOutputMsg:
		for _, tab := range m.TabManager.Tabs {
			if tab.ID == msg.TabID && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				return m, waitForTerminalOutput(tab.TerminalView)
			}
		}
		return m, nil

	case TerminalExitMsg:
		for i, tab := range m.TabManager.Tabs {
			if tab.ID == msg.TabID && tab.Type == editor.TabTerminal {
				m.TabManager.CloseTab(i)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Terminal ditutup (exit)", false, 2*time.Second)
				break
			}
		}
		return m, nil

	case tea.KeyMsg:
		// 1. Context Menu Handling if active
		if m.ContextMenu != nil && m.ContextMenu.Active {
			switch msg.String() {
			case "esc":
				m.ContextMenu.Close()
				return m, nil
			case "up", "k":
				m.ContextMenu.Prev()
				return m, nil
			case "down", "j":
				m.ContextMenu.Next()
				return m, nil
			case "enter":
				item := m.ContextMenu.SelectedItem()
				m.ContextMenu.Close()
				if item != nil && !item.Disabled {
					cmd := m.handleContextMenuAction(item.ID)
					return m, cmd
				}
				return m, nil
			}
			return m, nil
		}

		// 0. Find Bar Handling if active
		if m.FindBar != nil && m.FindBar.Active {
			tab := m.TabManager.ActiveTab()
			action, closed, cmd := m.FindBar.Update(msg)
			if closed {
				if tab != nil && tab.Buffer != nil {
					tab.Buffer.ClearSearch()
				}
			} else if tab != nil && tab.Buffer != nil {
				switch action {
				case "change":
					tab.Buffer.SetSearchQuery(m.FindBar.Input.Value())
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
				case "next":
					tab.Buffer.FindNext()
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
				case "prev":
					tab.Buffer.FindPrev()
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
				case "replace_one":
					if tab.Buffer.ReplaceCurrent(m.FindBar.ReplaceInput.Value()) {
						cur, tot := tab.Buffer.MatchInfo()
						m.FindBar.SetMatches(cur, tot)
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("1 kecocokan diganti", false, 1*time.Second)
					}
				case "replace_all":
					count := tab.Buffer.ReplaceAll(m.FindBar.Input.Value(), m.FindBar.ReplaceInput.Value())
					cur, tot := tab.Buffer.MatchInfo()
					m.FindBar.SetMatches(cur, tot)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast(fmt.Sprintf("%d kecocokan diganti", count), false, 2*time.Second)
				}
			}
			return m, cmd
		}

		// 0.5. Quick Open Handling if active
		if m.QuickOpen != nil && m.QuickOpen.Active {
			targetTabIdx, targetPath, closed, cmd := m.QuickOpen.Update(msg)
			if closed {
				if targetTabIdx >= 0 && targetTabIdx < m.TabManager.Count() {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.ActiveIndex = targetTabIdx
					m.syncStatusBarWithActiveTab()
				} else if targetPath != "" {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.openFile(targetPath)
					m.Focus = FocusEditor
				}
			}
			return m, cmd
		}

		// 0.6. Command Palette Handling if active
		if m.CommandPalette != nil && m.CommandPalette.Active {
			actionID, closed, cmd := m.CommandPalette.Update(msg)
			if closed && actionID != "" {
				menuCmd := m.handleMenuAction(actionID)
				return m, tea.Batch(cmd, menuCmd)
			}
			return m, cmd
		}

		// 0.7. Global Search Handling if active
		if m.GlobalSearch != nil && m.GlobalSearch.Active {
			filePath, line, col, closed, cmd := m.GlobalSearch.Update(msg)
			if closed && filePath != "" {
				m.openFileAt(filePath, line, col)
				m.Focus = FocusEditor
			}
			return m, cmd
		}

		// 0.8. Branch Switcher Handling if active
		if m.BranchSwitcher != nil && m.BranchSwitcher.Active {
			selectedBranch, closed, cmd := m.BranchSwitcher.Update(msg)
			if closed {
				if selectedBranch != "" && m.GitRepo != nil {
					if err := m.GitRepo.CheckoutBranch(selectedBranch); err != nil {
						m.StatusBar.ShowToast("Gagal ganti branch: "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.Branch = selectedBranch
						if m.GitView != nil {
							m.GitView.Refresh()
						}
						m.StatusBar.ShowToast("Beralih ke branch: "+selectedBranch, false, 2*time.Second)
					}
				}
			}
			return m, cmd
		}

		// 0.85. AI Settings Dialog Handling if active
		if m.AISettingsDialog != nil && m.AISettingsDialog.Active {
			m.AISettingsDialog.SetScreenSize(m.Width, m.Height)
			saved, canceled, newCfg, cmd := m.AISettingsDialog.Update(msg)
			if saved {
				m.Config.AI = newCfg
				_ = config.SaveConfig(m.Config)
				m.StatusBar.ShowToast("Pengaturan AI berhasil disimpan", false, 3*time.Second)
			} else if canceled {
				// Dialog closed
			}
			return m, cmd
		}

		// 1. Buffer Switcher Modal Handling
		if m.Switcher.Active {
			targetIdx, closed, cmd := m.Switcher.Update(msg)
			if closed {
				if targetIdx >= 0 && targetIdx < m.TabManager.Count() {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.ActiveIndex = targetIdx
					m.syncStatusBarWithActiveTab()
				}
			}
			return m, cmd
		}

		// 1.5. File Picker Handling if active
		if m.FilePicker != nil && m.FilePicker.Active {
			confirmed, canceled, selectedPath, cmd := m.FilePicker.Update(msg)
			if confirmed {
				m.handleFilePickerConfirmed(selectedPath)
			} else if canceled {
				m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
			}
			return m, cmd
		}

		// 2. Input Dialog Handling if active
		if m.InputDialog.Active {
			confirmed, canceled, cmd := m.InputDialog.Update(msg)
			if confirmed {
				m.handleInputConfirmed()
			} else if canceled {
				m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
			}
			return m, cmd
		}

		// 3. Confirm Dialog Handling if active
		if m.ConfirmDialog.Active {
			confirmed, canceled, discard := m.ConfirmDialog.Update(msg)
			if confirmed {
				cmd := m.handleConfirmConfirmed()
				if cmd != nil {
					return m, cmd
				}
			} else if discard {
				cmd := m.handleDiscardConfirmed()
				if cmd != nil {
					return m, cmd
				}
			} else if canceled {
				if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
					m.StatusBar.ShowToast("Perubahan eksternal diabaikan (lokal dipertahankan)", false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				}
				m.ClosingTabIdx = -1
			}
			return m, nil
		}

		// 4. Menu Bar Key Handling
		if action, handled := m.MenuBar.HandleKey(msg); handled {
			if action != "" {
				cmd := m.handleMenuAction(action)
				return m, cmd
			}
			return m, nil
		}

		// 5. Modal escape
		if m.ShowHelp && m.HelpDialog != nil && m.HelpDialog.Active {
			closed := m.HelpDialog.Update(msg)
			if closed {
				m.ShowHelp = false
			}
			return m, nil
		}
		if msg.String() == "esc" {
			if m.ShowHelp {
				m.ShowHelp = false
				return m, nil
			}
			if m.ShowAbout {
				m.ShowAbout = false
				return m, nil
			}
		}

		// 5.5. Active Terminal Tab Focus & Raw Keystroke Interception
		if m.Focus == FocusEditor {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				switch msg.String() {
				case "ctrl+\\", "ctrl+`", "f6":
					m.Focus = FocusSidebar
					m.StatusBar.ShowToast("Fokus: Sidebar (tekan Ctrl+\\ untuk kembali ke Terminal)", false, 2*time.Second)
					return m, nil
				case "ctrl+shift+\\", "ctrl+t":
					var cmd tea.Cmd
					m, cmd = m.openNewTerminalTab()
					return m, cmd
				case "ctrl+shift+w", "alt+w":
					m.TabManager.CloseTab(m.TabManager.ActiveIndex)
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Terminal ditutup", false, 1500*time.Millisecond)
					return m, nil
				case "ctrl+n":
					m.TabManager.NextTab()
					m.syncStatusBarWithActiveTab()
					return m, m.requestSessionPersist()
				case "ctrl+shift+p", "ctrl+P", "f1":
					m.openCommandPalette()
					return m, nil
				default:
					bytes := terminal.KeyToBytes(msg)
					if len(bytes) > 0 {
						_, _ = tab.TerminalView.Write(bytes)
					}
					return m, nil
				}
			}
		}

		// 6. Global Shortcuts
		switch msg.String() {
		case "ctrl+shift+\\", "ctrl+t":
			var cmd tea.Cmd
			m, cmd = m.openNewTerminalTab()
			return m, cmd
		case "ctrl+\\", "ctrl+`":
			m.toggleFocus()
			return m, nil
		case "ctrl+q":
			if m.hasDirtyTabs() && !m.AutoSave {
				m.ConfirmDialog.Open(
					dialog.ConfirmUnsavedChanges,
					"Perubahan Belum Disimpan",
					"Terdapat buffer yang belum disimpan.\nSimpan sebelum keluar?",
					"app.exit",
				)
				return m, nil
			}
			if m.AutoSave {
				_, _ = editor.TriggerAutoSave(m.TabManager)
			}
			// Reset cursor shape before exiting
			fmt.Print("\x1b]22;default\x1b\\")
			return m, tea.Quit

		case "ctrl+b":
			m.SidebarVisible = !m.SidebarVisible
			return m, nil

		case "ctrl+e", "f6":
			m.toggleFocus()
			return m, nil

		case "alt+s":
			m.AutoSave = !m.AutoSave
			m.StatusBar.AutoSave = m.AutoSave
			if m.AutoSave {
				m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
			}
			return m, nil
		case "alt+z", "alt+Z":
			m.toggleSoftWrap()
			return m, nil

		case "alt+[":
			m.adjustSidebarWidth(-2)
			return m, nil

		case "alt+]":
			m.adjustSidebarWidth(2)
			return m, nil

		case "alt+1":
			m.SidebarView = SidebarFiles
			return m, nil

		case "alt+2":
			m.SidebarView = SidebarGit
			if m.GitView != nil {
				m.GitView.Refresh()
			}
			return m, nil

		case "alt+3":
			m.SidebarView = SidebarSearch
			if m.SearchView != nil {
				m.SearchView.DoSearch()
			}
			return m, nil
		case "alt+b":
			m.openBranchSwitcher()
			return m, nil

		case "alt+g":
			return m.triggerAICommit()

		case "alt+i":
			m.openAISettings()
			return m, nil

		case "ctrl+enter":
			if m.GitView != nil && m.GitRepo != nil {
				if m.GitView.IsLoading {
					return m, nil
				}
				if len(m.GitView.Staged) == 0 {
					m.StatusBar.ShowToast("Tidak ada perubahan di staging area", true, 3*time.Second)
					return m, nil
				}
				commitMsg := m.GitView.CommitInput.Value()
				m.GitView.IsLoading = true
				m.GitView.LoadingMsg = "Committing..."
				m.GitView.SpinnerFrame = 0
				m.StatusBar.ShowToast("Melakukan commit...", false, 3*time.Second)
				return m, tea.Batch(gitCommitCmd(m.GitRepo, commitMsg), gitLoadingTickCmd())
			}
			return m, nil

		case "ctrl+o":
			m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)
			return m, nil

		case "ctrl+k":
			m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)
			return m, nil

		case "ctrl+p":
			m.openQuickOpen()
			return m, nil

		case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
			return m.triggerFormatCurrentBuffer()

		case "ctrl+shift+p", "ctrl+P", "f1":
			m.openCommandPalette()
			return m, nil

		case "ctrl+shift+f", "ctrl+F", "f4":
			m.openGlobalSearch()
			return m, nil

		case "ctrl+f":
			m.openFindBar()
			return m, nil

		case "ctrl+h":
			m.openReplaceBar()
			return m, nil

		case "ctrl+g":
			m.openGotoLine()
			return m, nil

		case "ctrl+m", "alt+m":
			if m.TabManager.ToggleActiveMarkdown() {
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.MarkdownPreview {
					m.StatusBar.ShowToast("Mode: Markdown Preview (Glamour)", false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Mode: Markdown Editor (Raw)", false, 2*time.Second)
				}
				m.syncStatusBarWithActiveTab()
			} else {
				m.StatusBar.ShowToast("Tab aktif bukan file Markdown", true, 2*time.Second)
			}
			return m, nil

		case "?":
			// Allow typing '?' in active editor buffer
			if m.Focus == FocusEditor {
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.Buffer != nil && !tab.IsDiff && !(tab.Type == editor.TabMarkdown && tab.MarkdownPreview) {
					break
				}
			}
			m.ShowHelp = !m.ShowHelp
			if m.ShowHelp && m.HelpDialog != nil {
				m.HelpDialog.Open(m.Height)
			}
			return m, nil
		}

		// 7. Context-Specific: Sidebar (Files or Git)
		if m.Focus == FocusSidebar && m.SidebarVisible {
			if m.SidebarView == SidebarGit && m.GitView != nil {
				if msg.String() == "d" || msg.String() == "x" {
					if m.GitView.FocusArea == git.GitFocusChanges {
						sel := m.GitView.SelectedChangesFile()
						if sel != nil {
							m.ConfirmDialog.Open(
								dialog.ConfirmDiscardGit,
								"Buang Perubahan",
								fmt.Sprintf("Buang semua perubahan lokal pada '%s'? Tindakan ini tidak dapat dibatalkan.", sel.Name),
								sel.Path,
							)
							return m, nil
						}
					}
				}
				cmd, fileToOpen := m.GitView.Update(msg)
				if fileToOpen != nil {
					switch fileToOpen.Path {
					case "__hunk_stage__":
						if f := m.GitView.SelectedChangesFile(); f != nil {
							if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
								if err := m.GitRepo.StageHunk(f.Path, hIdx); err != nil {
									m.StatusBar.ShowToast("Gagal stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
								} else {
									m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
								}
								if m.GitView != nil {
									m.GitView.Refresh()
								}
								m.refreshDiffTabsForPath(f.Path)
							} else {
								m.StatusBar.ShowToast("Tidak ada hunk aktif", true, 2*time.Second)
							}
						}
					case "__hunk_unstage__":
						if f := m.GitView.SelectedStagedFile(); f != nil {
							if hIdx, ok := m.GitView.SelectedHunkIndex(true); ok {
								if err := m.GitRepo.UnstageHunk(f.Path, hIdx); err != nil {
									m.StatusBar.ShowToast("Gagal unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
								} else {
									m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
								}
								if m.GitView != nil {
									m.GitView.Refresh()
								}
								m.refreshDiffTabsForPath(f.Path)
							} else {
								m.StatusBar.ShowToast("Tidak ada hunk aktif", true, 2*time.Second)
							}
						}
					case "__hunk_discard__":
						if f := m.GitView.SelectedChangesFile(); f != nil {
							if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
								m.pendingGitHunkPath = f.Path
								m.pendingGitHunkIndex = hIdx
								m.ConfirmDialog.Open(
									dialog.ConfirmDiscardGitHunk,
									"Buang Hunk?",
									fmt.Sprintf("Buang hunk #%d pada '%s'?\nPerubahan worktree untuk hunk ini akan hilang permanen.", hIdx+1, f.Path),
									f.Path,
								)
								return m, cmd
							}
							m.StatusBar.ShowToast("Tidak ada hunk aktif", true, 2*time.Second)
						}
					default:
						if fileToOpen.Status == "?" || fileToOpen.Status == "U" {
							fullPath := filepath.Join(m.RootDir, fileToOpen.Path)
							m.openFile(fullPath)
							m.Focus = FocusEditor
						} else {
							m.openDiff(fileToOpen.Path, fileToOpen.IsStaged)
						}
					}
				}
				return m, cmd
			}

			if m.SidebarView == SidebarSearch && m.SearchView != nil {
				filePath, line, col, cmd := m.SearchView.Update(msg)
				if filePath != "" {
					m.openFileAt(filePath, line, col)
					m.Focus = FocusEditor
				}
				return m, cmd
			}

			if m.SidebarView == SidebarFiles {
				switch msg.String() {
				case "j", "down":
					m.Explorer.MoveDown()
					return m, nil

				case "k", "up":
					m.Explorer.MoveUp()
					return m, nil

				case "enter", "l", "right":
					node := m.Explorer.SelectedNode()
					if node != nil {
						if node.IsDir {
							m.Explorer.ToggleExpand()
						} else {
							m.openFile(node.Path)
							m.Focus = FocusEditor
						}
					}
					return m, nil

				case "h", "left":
					m.Explorer.CollapseOrParent()
					return m, nil

				case ".":
					m.Explorer.ToggleHidden()
					return m, nil

				case "r":
					m.Explorer.Refresh()
					m.StatusBar.ShowToast("Pohon file diperbarui", false, 2*time.Second)
					return m, nil
				case "/":
					m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file:", m.Explorer.FilterQuery)
					return m, nil
				case "esc":
					if m.Explorer.FilterQuery != "" {
						m.Explorer.SetFilter("")
						m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
						return m, nil
					}

				case "a":
					parentDir := m.getSelectedParentDir()
					m.InputDialog.Open(dialog.InputNewFile, "Buat File Baru", parentDir, "")
					return m, nil

				case "A":
					parentDir := m.getSelectedParentDir()
					m.InputDialog.Open(dialog.InputNewFolder, "Buat Folder Baru", parentDir, "")
					return m, nil

				case "F2":
					node := m.Explorer.SelectedNode()
					if node != nil {
						m.InputDialog.Open(dialog.InputRename, "Ubah Nama: "+node.Name, node.Path, node.Name)
					}
					return m, nil

				case "d":
					node := m.Explorer.SelectedNode()
					if node != nil {
						m.ConfirmDialog.Open(
							dialog.ConfirmDelete,
							"Konfirmasi Hapus",
							"Apakah Anda yakin ingin menghapus:\n"+node.Path+"?",
							node.Path,
						)
					}
					return m, nil
				}
			}
		}

		// 8. Context-Specific: Modeless Text Editor Buffer or Diff View
		if m.Focus == FocusEditor {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.IsDiff && tab.DiffView != nil {
				switch msg.String() {
				case "ctrl+w":
					m.attemptCloseTab(m.TabManager.ActiveIndex)
					return m, nil
				case "ctrl+n":
					m.TabManager.NextTab()
					m.syncStatusBarWithActiveTab()
					return m, m.requestSessionPersist()
				case "up", "k":
					tab.DiffView.ScrollUp(1)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "down", "j":
					tab.DiffView.ScrollDown(1, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "pgup":
					tab.DiffView.ScrollUp(10)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "pgdown":
					tab.DiffView.ScrollDown(10, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "home":
					tab.DiffView.ScrollOffset = 0
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "n", "]":
					tab.DiffView.NextHunk(m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "N", "[":
					tab.DiffView.PrevHunk(m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "s", "+":
					return m, m.applyDiffHunkAction(tab, "stage")
				case "u", "-":
					return m, m.applyDiffHunkAction(tab, "unstage")
				case "x":
					return m, m.applyDiffHunkAction(tab, "discard")
				}
			} else if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				switch msg.String() {
				case "ctrl+w":
					m.attemptCloseTab(m.TabManager.ActiveIndex)
					return m, nil
				case "ctrl+n":
					m.TabManager.NextTab()
					m.syncStatusBarWithActiveTab()
					return m, m.requestSessionPersist()
				case "up", "k":
					tab.MarkdownViewer.ScrollUp(1)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "down", "j":
					tab.MarkdownViewer.ScrollDown(1, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "pgup":
					tab.MarkdownViewer.ScrollUp(10)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "pgdown":
					tab.MarkdownViewer.ScrollDown(10, m.Height-3)
					m.syncStatusBarWithActiveTab()
					return m, nil
				case "home":
					tab.MarkdownViewer.ScrollHome()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+c":
					text := tab.MarkdownViewer.GetSelectedText()
					if text == "" {
						text = tab.MarkdownViewer.GetAllCleanText()
					}
					if text != "" {
						_ = clipboard.WriteAll(text)
						m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+a":
					tab.MarkdownViewer.SelectAll()
					m.StatusBar.ShowToast("Semua teks preview dipilih", false, 1*time.Second)
					return m, nil

				case "ctrl+v":
					text, err := clipboard.ReadAll()
					if err == nil && text != "" && tab.Buffer != nil {
						tab.Buffer.InsertString(text)
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Ditempel ke dokumen markdown", false, 2*time.Second)
					}
					return m, nil

				case "esc", "escape":
					if tab.MarkdownViewer.HasSelection {
						tab.MarkdownViewer.ClearSelection()
						return m, nil
					}
				}
			} else if tab != nil && tab.Buffer != nil {
				buf := tab.Buffer
				switch msg.String() {
				case "ctrl+shift+i", "ctrl+shift+I", "alt+shift+f", "alt+F":
					return m.triggerFormatCurrentBuffer()

				case "ctrl+s":
					if m.Config != nil && m.Config.FormatOnSave {
						req := formatter.FormatRequest{
							Content:         tab.Buffer.Text(),
							FilePath:        tab.Path,
							WorkspaceDir:    m.RootDir,
							CustomOverrides: m.Config.Formatters,
						}
						res, err := formatter.Format(req)
						if err == nil && res.Formatted != tab.Buffer.Text() {
							tab.Buffer.ReplaceContentWithUndo(res.Formatted)
						}
					}
					if err := m.TabManager.SaveActive(); err != nil {
						m.StatusBar.ShowToast("Error simpan: "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.ShowToast("Disimpan: "+tab.Name, false, 2*time.Second)
					}
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+w":
					m.attemptCloseTab(m.TabManager.ActiveIndex)
					return m, nil

				case "ctrl+n":
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.NextTab()
					m.syncStatusBarWithActiveTab()
					return m, m.requestSessionPersist()

				case "ctrl+c":
					text := buf.GetSelectedText()
					if text != "" {
						_ = clipboard.WriteAll(text)
						m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+v":
					text, err := clipboard.ReadAll()
					if err == nil && text != "" {
						buf.InsertString(text)
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+x":
					text := buf.GetSelectedText()
					if text != "" {
						_ = clipboard.WriteAll(text)
						buf.DeleteSelection()
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+a":
					buf.SelectAll()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+z":
					if buf.Undo() {
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Undo", false, 1*time.Second)
					} else {
						m.StatusBar.ShowToast("Sudah di perubahan paling awal", false, 1*time.Second)
					}
					return m, nil

				case "ctrl+y", "ctrl+shift+z":
					if buf.Redo() {
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Redo", false, 1*time.Second)
					} else {
						m.StatusBar.ShowToast("Sudah di perubahan paling akhir", false, 1*time.Second)
					}
					return m, nil

				case "enter":
					buf.InsertNewline()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "backspace":
					buf.Backspace()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "delete":
					buf.Delete()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "tab":
					if buf.HasSelection {
						buf.IndentSelection()
					} else {
						buf.InsertString("    ")
					}
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "shift+tab":
					if buf.HasSelection {
						buf.OutdentSelection()
					} else {
						buf.OutdentCurrentLine()
					}
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+/", "ctrl+_":
					buf.ToggleLineComment(tab.CommentPrefix())
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Komentar dialihkan", false, 1*time.Second)
					return m, nil

				case "up":
					buf.ClearSelection()
					buf.MoveUp()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "down":
					buf.ClearSelection()
					buf.MoveDown()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "left":
					buf.ClearSelection()
					buf.MoveLeft()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+left", "alt+left":
					buf.ClearSelection()
					buf.MoveWordLeft()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "right":
					buf.ClearSelection()
					buf.MoveRight()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+right", "alt+right":
					buf.ClearSelection()
					buf.MoveWordRight()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "home":
					buf.ClearSelection()
					buf.MoveHome()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "end":
					buf.ClearSelection()
					buf.MoveEnd()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgup":
					buf.ClearSelection()
					buf.MovePageUp(10)
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgdown":
					buf.ClearSelection()
					buf.MovePageDown(10)
					m.syncStatusBarWithActiveTab()
					return m, nil

				default:
					// Insert printable characters
					if len(msg.Runes) > 0 && !msg.Alt {
						for _, r := range msg.Runes {
							buf.InsertRune(r)
						}
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				}
			}
		}

	case tea.MouseMsg:
		// Forward mouse to context menu
		if m.ContextMenu != nil && m.ContextMenu.Active {
			action, handled := m.ContextMenu.HandleMouse(msg)
			if handled {
				if action != "" {
					cmd := m.handleContextMenuAction(action)
					return m, cmd
				}
				return m, nil
			}
		}

		// Intercept mouse clicks for active modals
		if m.FindBar != nil && m.FindBar.Active {
			barStr := m.FindBar.Render()
			findWidth := lipgloss.Width(barStr)
			findX := m.Width - findWidth - 2
			editorStartX := 0
			if m.SidebarVisible {
				editorStartX = m.SidebarWidth
			}
			if findX < editorStartX+1 {
				findX = editorStartX + 1
			}
			relX := msg.X - findX
			relY := msg.Y - 1 // Floating at row 1
			action, closed, handled := m.FindBar.HandleMouse(relX, relY)
			if handled {
				tab := m.TabManager.ActiveTab()
				if closed {
					if tab != nil && tab.Buffer != nil {
						tab.Buffer.ClearSearch()
					}
					return m, nil
				}
				if tab != nil && tab.Buffer != nil {
					switch action {
					case "next":
						tab.Buffer.FindNext()
						cur, tot := tab.Buffer.MatchInfo()
						m.FindBar.SetMatches(cur, tot)
						m.syncStatusBarWithActiveTab()
					case "prev":
						tab.Buffer.FindPrev()
						cur, tot := tab.Buffer.MatchInfo()
						m.FindBar.SetMatches(cur, tot)
						m.syncStatusBarWithActiveTab()
					case "replace_one":
						if tab.Buffer.ReplaceCurrent(m.FindBar.ReplaceInput.Value()) {
							cur, tot := tab.Buffer.MatchInfo()
							m.FindBar.SetMatches(cur, tot)
							m.syncStatusBarWithActiveTab()
							m.StatusBar.ShowToast("1 kecocokan diganti", false, 1*time.Second)
						}
					case "replace_all":
						count := tab.Buffer.ReplaceAll(m.FindBar.Input.Value(), m.FindBar.ReplaceInput.Value())
						cur, tot := tab.Buffer.MatchInfo()
						m.FindBar.SetMatches(cur, tot)
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast(fmt.Sprintf("%d kecocokan diganti", count), false, 2*time.Second)
					}
				}
				return m, nil
			}
		}

		if m.CommandPalette != nil && m.CommandPalette.Active {
			modalStr := m.CommandPalette.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 3
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				actionID, closed, handled := m.CommandPalette.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if handled {
					if closed && actionID != "" {
						menuCmd := m.handleMenuAction(actionID)
						return m, menuCmd
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.QuickOpen != nil && m.QuickOpen.Active {
			modalStr := m.QuickOpen.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 3
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				tabIdx, relPath, closed, handled := m.QuickOpen.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if handled {
					if closed {
						if tabIdx >= 0 {
							m.TabManager.ActiveIndex = tabIdx
							m.Focus = FocusEditor
							m.syncStatusBarWithActiveTab()
						} else if relPath != "" {
							fullPath := filepath.Join(m.RootDir, relPath)
							m.openFile(fullPath)
						}
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.GlobalSearch != nil && m.GlobalSearch.Active {
			modalStr := m.GlobalSearch.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 3
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				relPath, lineNum, colNum, closed, handled := m.GlobalSearch.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if handled {
					if closed && relPath != "" {
						fullPath := filepath.Join(m.RootDir, relPath)
						m.openFileAt(fullPath, lineNum, colNum)
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.BranchSwitcher != nil && m.BranchSwitcher.Active {
			modalStr := m.BranchSwitcher.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 3
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				targetBranch, closed, handled := m.BranchSwitcher.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if handled {
					if closed && targetBranch != "" && m.GitRepo != nil {
						if err := m.GitRepo.CheckoutBranch(targetBranch); err != nil {
							m.StatusBar.ShowToast("Gagal ganti branch: "+err.Error(), true, 3*time.Second)
						} else {
							m.StatusBar.Branch = targetBranch
							if m.GitView != nil {
								m.GitView.Refresh()
							}
							m.StatusBar.ShowToast("Beralih ke branch: "+targetBranch, false, 2*time.Second)
						}
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.AISettingsDialog != nil && m.AISettingsDialog.Active {
			modalStr := m.AISettingsDialog.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				confirmed, canceled, newCfg, handled := m.AISettingsDialog.HandleMouse(relX, relY)
				if handled {
					if confirmed {
						m.Config.AI = newCfg
						_ = config.SaveConfig(m.Config)
						m.StatusBar.ShowToast("Konfigurasi AI berhasil disimpan", false, 3*time.Second)
					} else if canceled {
						m.StatusBar.ShowToast("Pengaturan AI dibatalkan", false, 2*time.Second)
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.FilePicker != nil && m.FilePicker.Active {
			modalStr := m.FilePicker.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				confirmed, canceled, selectedPath, handled := m.FilePicker.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if confirmed {
					m.handleFilePickerConfirmed(selectedPath)
				} else if canceled {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.InputDialog.Active {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				modalStr := m.InputDialog.Render()
				modalW := lipgloss.Width(modalStr)
				modalH := lipgloss.Height(modalStr)
				startX := (m.Width - modalW) / 2
				startY := (m.Height - modalH) / 2
				relX := msg.X - startX
				relY := msg.Y - startY
				confirmed, canceled, handled := m.InputDialog.HandleMouse(relX, relY)
				if confirmed {
					m.handleInputConfirmed()
				} else if canceled {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.ConfirmDialog.Active {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				modalStr := m.ConfirmDialog.Render()
				modalW := lipgloss.Width(modalStr)
				modalH := lipgloss.Height(modalStr)
				startX := (m.Width - modalW) / 2
				startY := (m.Height - modalH) / 2
				relX := msg.X - startX
				relY := msg.Y - startY
				confirmed, canceled, discard, handled := m.ConfirmDialog.HandleMouse(relX, relY)
				if confirmed {
					return m, m.handleConfirmConfirmed()
				} else if discard {
					return m, m.handleDiscardConfirmed()
				} else if canceled {
					if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
						m.StatusBar.ShowToast("Perubahan eksternal diabaikan (lokal dipertahankan)", false, 2*time.Second)
					} else {
						m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
					}
					m.ClosingTabIdx = -1
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.Switcher.Active {
			modalStr := m.Switcher.Render()
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 3
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				targetIdx, closed, handled := m.Switcher.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if handled {
					if closed && targetIdx >= 0 {
						m.TabManager.ActiveIndex = targetIdx
						m.Focus = FocusEditor
						m.syncStatusBarWithActiveTab()
					}
					return m, nil
				}
			}
			return m, nil
		}

		// Resizing Sidebar Handling
		if m.SidebarVisible {
			isAtBorder := (msg.X == m.SidebarWidth-1 || msg.X == m.SidebarWidth) && msg.Y >= 1 && msg.Y < m.Height-1
			if isAtBorder {
				m.setMousePointer("col-resize")
			}

			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && isAtBorder {
				m.ResizingSidebar = true
				return m, nil
			}
		}

		if m.ResizingSidebar {
			if msg.Action == tea.MouseActionMotion {
				m.setSidebarWidth(msg.X)
				return m, nil
			}
			if msg.Action == tea.MouseActionRelease {
				m.ResizingSidebar = false
				return m, nil
			}
		}

		editorStartX := 0
		if m.SidebarVisible {
			editorStartX = m.SidebarWidth
		}

		// Update mouse pointer shape: col-resize at border, text over editor, default elsewhere
		if m.SidebarVisible && (msg.X == m.SidebarWidth-1 || msg.X == m.SidebarWidth) && msg.Y >= 1 && msg.Y < m.Height-1 {
			m.setMousePointer("col-resize")
		} else if msg.X >= editorStartX && msg.Y >= 2 && msg.Y < m.Height-1 {
			m.setMousePointer("text")
		} else {
			m.setMousePointer("default")
		}

		// Right-click handling
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
			// Tab Bar right-click (Row 1, X >= editorStartX)
			if msg.Y == 1 && msg.X >= editorStartX {
				relX := msg.X - editorStartX
				clickedTabIdx := m.TabManager.FindTabAt(relX)
				if clickedTabIdx >= 0 && clickedTabIdx < m.TabManager.Count() {
					m.ContextMenuTabIdx = clickedTabIdx
					items := []ui.ContextMenuItem{
						{ID: "tab_close_this", Label: "Tutup", Shortcut: "Ctrl+W"},
						{ID: "tab_close_others", Label: "Tutup Lainnya"},
						{ID: "tab_close_right", Label: "Tutup ke Kanan"},
						{ID: "tab_close_left", Label: "Tutup ke Kiri"},
						{ID: "tab_close_all", Label: "Tutup Semua"},
					}
					if m.TabManager.Count() <= 1 {
						items[1].Disabled = true
						items[2].Disabled = true
						items[3].Disabled = true
					} else {
						if clickedTabIdx == m.TabManager.Count()-1 {
							items[2].Disabled = true
						}
						if clickedTabIdx == 0 {
							items[3].Disabled = true
						}
					}
					m.ContextMenu.OpenCustom(msg.X, msg.Y+1, items, m.Width, m.Height)
					return m, nil
				}
			}

			// Right-click inside File Explorer sidebar
			if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 3 && msg.Y < m.Height-2 && m.SidebarView == SidebarFiles {
				m.Focus = FocusSidebar
				filterActive := m.Explorer.FilterQuery != ""
				nodeBodyY := msg.Y
				if filterActive {
					nodeBodyY = msg.Y - 1
				}
				if filterActive && msg.Y == 3 {
					// Right-click on filter banner: treat as clear-filter shortcut via context menu with single entry
					items := []ui.ContextMenuItem{{ID: "explorer_clear_filter", Label: "Clear Filter", Shortcut: "Esc"}}
					m.ContextMenu.OpenCustom(msg.X, msg.Y+1, items, m.Width, m.Height)
					return m, nil
				}
				if nodeBodyY >= 3 {
					relLine := nodeBodyY - 3
					targetIdx := m.Explorer.ScrollOffset + relLine
					if targetIdx >= 0 && targetIdx < len(m.Explorer.FlatNodes) {
						m.Explorer.SelectedIndex = targetIdx
					}
				}
				m.ContextMenuTabIdx = -1
				m.openExplorerContextMenu(msg.X, msg.Y+1)
				return m, nil
			}

			// Right-click inside editor area -> Open Editor Context Menu
			if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
				m.ContextMenuTabIdx = -1
				m.Focus = FocusEditor
				hasSelection := false
				if tab := m.TabManager.ActiveTab(); tab != nil {
					if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
						hasSelection = tab.MarkdownViewer.HasSelection
					} else if tab.Buffer != nil {
						hasSelection = tab.Buffer.HasSelection
					}
				}
				clipText, _ := clipboard.ReadAll()
				hasClipboard := len(clipText) > 0
				m.ContextMenu.Open(msg.X, msg.Y, hasSelection, hasClipboard, m.Width, m.Height)
				return m, nil
			}
		}

		// Drag mouse inside editor buffer area to select text
		if msg.Action == tea.MouseActionMotion && msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				relRow := msg.Y - 2 + tab.MarkdownViewer.ScrollOffset
				relCol := msg.X - editorStartX
				if relCol < 0 {
					relCol = 0
				}
				tab.MarkdownViewer.UpdateSelection(relRow, relCol)
				return m, nil
			} else if tab != nil && tab.Buffer != nil {
				relY := msg.Y - 2
				gutterW := tab.Buffer.GutterWidth()
				textX := msg.X - editorStartX - gutterW
				if textX < 0 {
					textX = 0
				}
				textW := m.Width - editorStartX - gutterW
				if textW < 10 {
					textW = 10
				}
				physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)
				tab.Buffer.UpdateSelection(physRow, physCol)
				m.syncStatusBarWithActiveTab()
			}
			return m, nil
		}

		// Release mouse inside editor area
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if m.ResizingSidebar {
				m.ResizingSidebar = false
				return m, nil
			}
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
				sR, sC, eR, eC := tab.MarkdownViewer.NormalizedSelection()
				if sR == eR && sC == eC {
					tab.MarkdownViewer.ClearSelection()
				}
			} else if tab != nil && tab.Buffer != nil && tab.Buffer.HasSelection {
				if tab.Buffer.SelectAnchorRow == tab.Buffer.CursorRow && tab.Buffer.SelectAnchorCol == tab.Buffer.CursorCol {
					tab.Buffer.ClearSelection()
					m.syncStatusBarWithActiveTab()
				}
			}
			return m, nil
		}

		// Forward mouse to menu bar
		if action, handled := m.MenuBar.HandleMouse(msg); handled {
			if action != "" {
				cmd := m.handleMenuAction(action)
				return m, cmd
			}
			return m, nil
		}

		// Mouse scroll wheel on Tab Bar (Row 1): scroll tabs horizontally
		if msg.Y == 1 && msg.X >= editorStartX {
			if msg.Button == tea.MouseButtonWheelUp {
				m.TabManager.ScrollTabsLeft()
				return m, nil
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.TabManager.ScrollTabsRight()
				return m, nil
			}
		}

		// Mouse scroll wheel in sidebar (Row 2 to Height-2)
		if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 2 && msg.Y < m.Height-1 {
			sidebarH := m.Height - 4
			if sidebarH < 1 {
				sidebarH = 1
			}
			if msg.Button == tea.MouseButtonWheelUp {
				if m.SidebarView == SidebarFiles && m.Explorer != nil {
					m.Explorer.ScrollUp(3)
					return m, nil
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					m.GitView.ScrollUp(3)
					return m, nil
				} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
					if m.SearchView.Selected > 0 {
						m.SearchView.Selected--
					}
					return m, nil
				}
			} else if msg.Button == tea.MouseButtonWheelDown {
				if m.SidebarView == SidebarFiles && m.Explorer != nil {
					m.Explorer.ScrollDown(3, sidebarH)
					return m, nil
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					m.GitView.ScrollDown(3, sidebarH)
					return m, nil
				} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
					if m.SearchView.Selected < len(m.SearchView.Matches)-1 {
						m.SearchView.Selected++
					}
					return m, nil
				}
			}
		}

		// Mouse scroll wheel in editor
		if m.Focus == FocusEditor && m.TabManager.ActiveTab() != nil {
			tab := m.TabManager.ActiveTab()
			if msg.Action == tea.MouseActionPress {
				if tab.IsDiff && tab.DiffView != nil {
					if msg.Button == tea.MouseButtonWheelUp {
						tab.DiffView.ScrollUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						tab.DiffView.ScrollDown(3, m.Height-3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				} else if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
					if msg.Button == tea.MouseButtonWheelUp {
						tab.MarkdownViewer.ScrollUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						tab.MarkdownViewer.ScrollDown(3, m.Height-3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				} else if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
					if msg.Button == tea.MouseButtonWheelUp {
						tab.TerminalView.ScrollUp(3)
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						tab.TerminalView.ScrollDown(3)
						return m, nil
					}
				} else if tab.Buffer != nil {
					buf := tab.Buffer
					if msg.Button == tea.MouseButtonWheelUp {
						buf.MovePageUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						buf.MovePageDown(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				}
			}
		}

		// ShowHelp / ShowAbout mouse handling
		if m.ShowHelp && m.HelpDialog != nil && m.HelpDialog.Active {
			modalStr := m.HelpDialog.Render(m.Height)
			modalW := lipgloss.Width(modalStr)
			modalH := lipgloss.Height(modalStr)
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			if startX < 0 {
				startX = 0
			}
			if startY < 1 {
				startY = 1
			}
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			closed, _ := m.HelpDialog.HandleMouse(relX, relY, isWheelUp, isWheelDown)
			if closed {
				m.ShowHelp = false
			}
			return m, nil
		}
		if m.ShowHelp || m.ShowAbout {
			if msg.Action == tea.MouseActionPress {
				m.ShowHelp = false
				m.ShowAbout = false
			}
			return m, nil
		}

		// Mouse click handling
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Row 1: Sidebar Title / Branch click (Sidebar columns)
			if msg.Y == 1 && m.SidebarVisible && msg.X < m.SidebarWidth {
				m.Focus = FocusSidebar
				if m.SidebarView == SidebarGit {
					m.openBranchSwitcher()
					return m, nil
				}
			}

			// Row 1: Tab Bar click (Editor columns)
			if msg.Y == 1 && msg.X >= editorStartX {
				relX := msg.X - editorStartX
				// Middle-click closes the tab under the cursor (no activate/toggle side effects)
				if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonMiddle {
					idx := m.TabManager.FindTabAt(relX)
					if idx >= 0 {
						m.attemptCloseTab(idx)
						return m, nil
					}
					return m, nil
				}
				action, clickedTabIdx := m.TabManager.HandleTabClick(relX)
				if action == "scroll_left" {
					m.TabManager.ScrollTabsLeft()
					return m, nil
				}
				if action == "scroll_right" {
					m.TabManager.ScrollTabsRight()
					return m, nil
				}
				if action == "toggle_markdown" {
					m.TabManager.ToggleActiveMarkdown()
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
				if action == "activate" {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.ActiveIndex = clickedTabIdx
					m.Focus = FocusEditor
					m.syncStatusBarWithActiveTab()
					return m, nil
				} else if action == "close" {
					m.attemptCloseTab(clickedTabIdx)
					return m, nil
				}
			}

			// Row 2: Sidebar refresh button row click (limited to the [⟳ Refresh] hit area)
			if msg.Y == 2 && m.SidebarVisible && msg.X >= 0 && msg.X < 15 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.Focus = FocusSidebar
				if m.SidebarView == SidebarFiles {
					m.Explorer.Refresh()
					m.StatusBar.ShowToast("File explorer dimuat ulang", false, 2*time.Second)
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					m.GitView.Refresh()
					m.StatusBar.ShowToast("Git status diperbarui", false, 2*time.Second)
				} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
					m.SearchView.DoSearch()
					m.StatusBar.ShowToast("Pencarian diperbarui", false, 2*time.Second)
				}
				return m, nil
			}

			// Row Height-2: Sidebar bottom tabs click (📁 Files | 🌿 Git | 🔍 Search)
			if msg.Y == m.Height-2 && m.SidebarVisible && msg.X < m.SidebarWidth && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.Focus = FocusSidebar
				filesTabW := lipgloss.Width(" 1:Files ")
				gitTabW := lipgloss.Width(" 2:Git ")
				searchTabW := lipgloss.Width(" 3:Search ")
				if msg.X >= 0 && msg.X < filesTabW {
					m.SidebarView = SidebarFiles
				} else if msg.X >= filesTabW && msg.X < filesTabW+gitTabW {
					m.SidebarView = SidebarGit
					if m.GitView != nil {
						m.GitView.Refresh()
					}
				} else if msg.X >= filesTabW+gitTabW && msg.X < filesTabW+gitTabW+searchTabW {
					m.SidebarView = SidebarSearch
					if m.SearchView != nil {
						m.SearchView.DoSearch()
					}
				}
				return m, nil
			}

			// Sidebar body rows click (Row 3 to Height-3)
			if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 3 && msg.Y < m.Height-2 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				if m.Focus != FocusSidebar && m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.Focus = FocusSidebar
				if m.SidebarView == SidebarFiles {
					if m.Explorer == nil {
						return m, nil
					}
					filterActive := m.Explorer.FilterQuery != ""
					// Filter banner occupies row Y==3; node rows start at Y==4
					if filterActive && msg.Y == 3 {
						m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file (kosong = bersihkan):", m.Explorer.FilterQuery)
						return m, nil
					}
					relLine := msg.Y - 3
					if filterActive {
						relLine = msg.Y - 4
					}
					if relLine < 0 {
						return m, nil
					}
					action, node := m.Explorer.HandleClick(relLine)
					if action == "open" && node != nil {
						m.openFile(node.Path)
					}
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					relLine := msg.Y - 3
					targetFile, action, _ := m.GitView.HandleMouseClick(msg.X, relLine)
					if action == "commit" {
						if m.GitView.IsLoading {
							return m, nil
						}
						if len(m.GitView.Staged) == 0 {
							m.StatusBar.ShowToast("Tidak ada perubahan di staging area", true, 3*time.Second)
							return m, nil
						}
						commitMsg := m.GitView.CommitInput.Value()
						m.GitView.IsLoading = true
						m.GitView.LoadingMsg = "Committing..."
						m.GitView.SpinnerFrame = 0
						m.StatusBar.ShowToast("Melakukan commit...", false, 3*time.Second)
						return m, tea.Batch(gitCommitCmd(m.GitRepo, commitMsg), gitLoadingTickCmd())
					} else if action == "push" {
						if m.GitView.IsLoading {
							return m, nil
						}
						if m.GitRepo == nil {
							m.StatusBar.ShowToast("Repositori git tidak ditemukan", true, 3*time.Second)
							return m, nil
						}
						m.GitView.IsLoading = true
						m.GitView.LoadingMsg = "Pushing..."
						m.GitView.SpinnerFrame = 0
						m.StatusBar.ShowToast("Pushing ke remote repository...", false, 5*time.Second)
						return m, tea.Batch(gitPushCmd(m.GitRepo), gitLoadingTickCmd())
					} else if action == "ai_commit" {
						return m.triggerAICommit()
					}
					if action == "discard" && targetFile != nil {
						m.ConfirmDialog.Open(
							dialog.ConfirmDiscardGit,
							"Buang Perubahan",
							fmt.Sprintf("Buang semua perubahan lokal pada '%s'? Tindakan ini tidak dapat dibatalkan.", targetFile.Name),
							targetFile.Path,
						)
						return m, nil
					}
					if targetFile != nil {
						if targetFile.Status == "?" || targetFile.Status == "U" {
							fullPath := filepath.Join(m.RootDir, targetFile.Path)
							m.openFile(fullPath)
							m.Focus = FocusEditor
						} else {
							m.openDiff(targetFile.Path, targetFile.IsStaged)
						}
					}
				} else if m.SidebarView == SidebarSearch && m.SearchView != nil {
					relLine := msg.Y - 3
					filePath, line, col, _ := m.SearchView.HandleMouseClick(msg.X, relLine)
					if filePath != "" {
						m.openFileAt(filePath, line, col)
						m.Focus = FocusEditor
					}
				}
				return m, nil
			}
			// Click inside editor buffer area (Row 2 to Height-2, X >= editorStartX)
			if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
				m.Focus = FocusEditor
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
					relRow := msg.Y - 2 + tab.MarkdownViewer.ScrollOffset
					relCol := msg.X - editorStartX
					if relCol < 0 {
						relCol = 0
					}
					tab.MarkdownViewer.StartSelection(relRow, relCol)
					return m, nil
				} else if tab != nil && tab.Buffer != nil {
					relY := msg.Y - 2
					gutterW := tab.Buffer.GutterWidth()
					textX := msg.X - editorStartX - gutterW
					if textX < 0 {
						textX = 0
					}
					textW := m.Width - editorStartX - gutterW
					if textW < 10 {
						textW = 10
					}
					physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)
					tab.Buffer.StartSelection(physRow, physCol)
					m.syncStatusBarWithActiveTab()
				}
				return m, nil
			}

			// Row Height-1: Status Bar click
			if msg.Y == m.Height-1 {
				action := m.StatusBar.HandleMouseClick(msg.X)
				switch action {
				case "toggle_autosave":
					m.AutoSave = !m.AutoSave
					m.StatusBar.AutoSave = m.AutoSave
					if m.AutoSave {
						m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
					} else {
						m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
					}
					if m.Config != nil {
						m.Config.AutoSave = m.AutoSave
						_ = config.SaveConfig(m.Config)
					}
					return m, nil

				case "branch_switcher":
					m.openBranchSwitcher()
					return m, nil

				case "goto_line":
					curLine := 1
					tab := m.TabManager.ActiveTab()
					if tab != nil && tab.Buffer != nil {
						curLine = tab.Buffer.CursorRow + 1
					}
					m.InputDialog.Open(dialog.InputGotoLine, "Lompat ke Baris [baris:kolom]", "", strconv.Itoa(curLine))
					return m, nil

				case "help":
					m.ShowHelp = true
					if m.HelpDialog != nil {
						m.HelpDialog.Open(m.Height)
					}
					return m, nil
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *Model) toggleSoftWrap() {
	m.SoftWrap = !m.SoftWrap
	m.TabManager.SetSoftWrap(m.SoftWrap)
	if m.SoftWrap {
		m.StatusBar.ShowToast("Soft-Wrap Diaktifkan", false, 2*time.Second)
	} else {
		m.StatusBar.ShowToast("Soft-Wrap Dimatikan", true, 2*time.Second)
	}
	if m.Config != nil {
		m.Config.SoftWrap = m.SoftWrap
		_ = config.SaveConfig(m.Config)
	}
}

func (m *Model) adjustSidebarWidth(delta int) {
	if !m.SidebarVisible {
		return
	}
	m.setSidebarWidth(m.SidebarWidth + delta)
}

func (m *Model) setSidebarWidth(width int) {
	minWidth := 15
	maxWidth := m.Width - 20
	if maxWidth < minWidth {
		maxWidth = minWidth
	}
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if width != m.SidebarWidth {
		m.SidebarWidth = width
		if m.Config != nil {
			m.Config.SidebarWidth = width
			_ = config.SaveConfig(m.Config)
		}
	}
}

func (m *Model) setMousePointer(shape string) {
	if m.lastPointerShape != shape {
		m.lastPointerShape = shape
		fmt.Print("\x1b]22;" + shape + "\x1b\\")
	}
}

func (m *Model) checkWatchedFiles() {
	if m.TabManager == nil {
		return
	}
	for _, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Path == "" || tab.Type == editor.TabDiff || tab.Buffer == nil {
			continue
		}
		info, err := os.Stat(tab.Path)
		if err != nil {
			if os.IsNotExist(err) && !tab.LastModTime.IsZero() {
				m.StatusBar.ShowToast(fmt.Sprintf("File '%s' telah dihapus dari disk", tab.Name), true, 3*time.Second)
				tab.LastModTime = time.Time{}
			}
			continue
		}

		if tab.LastModTime.IsZero() {
			tab.LastModTime = info.ModTime()
			continue
		}

		if info.ModTime().After(tab.LastModTime) {
			tab.LastModTime = info.ModTime()
			if !tab.Buffer.IsDirty {
				if err := tab.ReloadFromDisk(); err == nil {
					m.StatusBar.ShowToast(fmt.Sprintf("File '%s' diperbarui dari disk", tab.Name), false, 2*time.Second)
				}
			} else {
				m.StatusBar.ShowToast(fmt.Sprintf("Peringatan: File '%s' diubah dari luar (ada perubahan lokal)", tab.Name), true, 4*time.Second)
				if m.ConfirmDialog != nil && !m.ConfirmDialog.Active {
					m.ConfirmDialog.Open(
						dialog.ConfirmExternalChange,
						"Konflik Perubahan Eksternal",
						fmt.Sprintf("File '%s' telah diubah dari luar, namun terdapat perubahan lokal di editor.\nReload dari disk atau buka Split Diff untuk merge?", tab.Name),
						tab.Path,
					)
				}
			}
		}
	}
}

func (m *Model) toggleFocus() {
	if m.Focus == FocusEditor {
		if m.AutoSave {
			saved, err := editor.TriggerAutoSave(m.TabManager)
			if saved > 0 {
				m.StatusBar.ShowToast(fmt.Sprintf("%d file disimpan", saved), false, 1500*time.Millisecond)
			} else if err != nil {
				m.StatusBar.ShowToast("Auto-save error: "+err.Error(), true, 3*time.Second)
			}
		}
		m.Focus = FocusSidebar
	} else {
		m.Focus = FocusEditor
	}
}

func (m *Model) hasDirtyTabs() bool {
	for _, t := range m.TabManager.Tabs {
		if t.IsDirty() {
			return true
		}
	}
	return false
}

func (m *Model) attemptCloseTab(tabIdx int) {
	if tabIdx < 0 || tabIdx >= m.TabManager.Count() {
		return
	}

	tab := m.TabManager.Tabs[tabIdx]
	if tab.IsDirty() && !m.AutoSave {
		m.ClosingTabIdx = tabIdx
		m.ConfirmDialog.Open(
			dialog.ConfirmUnsavedChanges,
			"Simpan Perubahan?",
			fmt.Sprintf("File '%s' telah dimodifikasi.\nSimpan perubahan sebelum menutup?", tab.Name),
			tab.Path,
		)
		return
	}

	if tab.IsDirty() && m.AutoSave {
		_ = m.TabManager.SaveTab(tabIdx)
	}

	m.TabManager.CloseTab(tabIdx)
	m.syncStatusBarWithActiveTab()
}

func (m *Model) modalWidth() int {
	w := int(float64(m.Width) * 0.8)
	if w < 50 {
		w = 50
	}
	if w > m.Width-4 && m.Width > 4 {
		w = m.Width - 4
	}
	return w
}

func (m *Model) openQuickOpen() {
	if m.QuickOpen != nil {
		m.QuickOpen.SetWidth(m.modalWidth())
	}
	var items []dialog.SwitcherItem
	for i, t := range m.TabManager.Tabs {
		items = append(items, dialog.SwitcherItem{
			Index:   i,
			Title:   t.Name,
			Path:    t.Path,
			IsDirty: t.IsDirty(),
		})
	}
	m.QuickOpen.Open(items, m.TabManager.ActiveIndex, m.RootDir)
}

func (m *Model) openCommandPalette() {
	if m.CommandPalette != nil {
		m.CommandPalette.SetWidth(m.modalWidth())
	}
	commands := []dialog.CommandPaletteItem{
		// File
		{ID: "file.new", Title: "New File", Category: "File", Shortcut: "a"},
		{ID: "file.format", Title: "Format Document", Category: "Edit", Shortcut: "Ctrl+Shift+I"},
		{ID: "setting.format_on_save", Title: "Toggle Format on Save", Category: "Settings", Shortcut: ""},
		{ID: "file.open", Title: "Open File", Category: "File", Shortcut: "Ctrl+O"},
		{ID: "file.open_folder", Title: "Open Folder", Category: "File", Shortcut: "Ctrl+K"},
		{ID: "file.save", Title: "Save File", Category: "File", Shortcut: "Ctrl+S"},
		{ID: "file.close", Title: "Close Tab", Category: "File", Shortcut: "Ctrl+W"},
		{ID: "tab.close_others", Title: "Close Other Tabs", Category: "File", Shortcut: ""},
		{ID: "tab.close_all", Title: "Close All Tabs", Category: "File", Shortcut: ""},
		{ID: "app.exit", Title: "Exit Application", Category: "File", Shortcut: "Ctrl+Q"},

		// Search & Navigation
		{ID: "search.quick_open", Title: "Quick Open File (Workspace)", Category: "Search", Shortcut: "Ctrl+P"},
		{ID: "search.palette", Title: "Command Palette", Category: "Search", Shortcut: "Ctrl+Shift+P / F1"},
		{ID: "search.find", Title: "Find in Current File", Category: "Search", Shortcut: "Ctrl+F"},
		{ID: "search.replace", Title: "Find & Replace in Current File", Category: "Search", Shortcut: "Ctrl+H"},
		{ID: "search.global", Title: "Global Search in Workspace", Category: "Search", Shortcut: "Ctrl+Shift+F"},
		{ID: "edit.goto_line", Title: "Go to Line / Column", Category: "Search", Shortcut: "Ctrl+G"},
		{ID: "edit.undo", Title: "Undo Text Change", Category: "Edit", Shortcut: "Ctrl+Z"},
		{ID: "edit.redo", Title: "Redo Text Change", Category: "Edit", Shortcut: "Ctrl+Y"},
		{ID: "edit.toggle_comment", Title: "Toggle Comment (Line / Selection)", Category: "Edit", Shortcut: "Ctrl+/"},
		{ID: "explorer.filter", Title: "Filter File Explorer", Category: "Explorer", Shortcut: "/"},

		// View
		{ID: "setting.sidebar", Title: "Toggle Sidebar", Category: "View", Shortcut: "Ctrl+B"},
		{ID: "view.sidebar_narrow", Title: "Persempit Sidebar", Category: "View", Shortcut: "Alt+["},
		{ID: "view.sidebar_widen", Title: "Perlebar Sidebar", Category: "View", Shortcut: "Alt+]"},
		{ID: "view.soft_wrap", Title: "Toggle Soft Wrap", Category: "View", Shortcut: "Alt+Z"},
		{ID: "view.focus", Title: "Switch Focus (Sidebar/Editor)", Category: "View", Shortcut: "Ctrl+E / F6"},
		{ID: "view.files", Title: "Switch to Files Explorer", Category: "View", Shortcut: "Alt+1"},
		{ID: "view.git", Title: "Switch to Git Sidebar", Category: "View", Shortcut: "Alt+2"},
		{ID: "view.search", Title: "Switch to Search & Replace Sidebar", Category: "View", Shortcut: "Alt+3"},
		{ID: "view.markdown", Title: "Toggle Markdown Live Preview", Category: "View", Shortcut: "Ctrl+M"},

		// Terminal
		{ID: "terminal.new", Title: "Terminal: New Integrated Terminal", Category: "Terminal", Shortcut: "Ctrl+Shift+\\"},
		{ID: "terminal.toggle_focus", Title: "Terminal: Toggle Focus (Terminal / TUI)", Category: "Terminal", Shortcut: "Ctrl+\\"},
		{ID: "terminal.close", Title: "Terminal: Close Active Terminal Tab", Category: "Terminal", Shortcut: "Ctrl+Shift+W"},

		// Settings & Tools
		{ID: "setting.autosave", Title: "Toggle Auto-Save", Category: "Setting", Shortcut: "Alt+S"},
		{ID: "setting.hidden", Title: "Toggle Hidden Files", Category: "Setting", Shortcut: "."},
		{ID: "setting.ai", Title: "Settings: Configure AI Model (OpenAI API)", Category: "Setting", Shortcut: "Alt+I"},
		{ID: "setting.shortcuts", Title: "Keyboard Shortcuts Help", Category: "Help", Shortcut: "?"},
		{ID: "about.open", Title: "About tui-workflow", Category: "Help", Shortcut: "Alt+A"},

		// Git
		{ID: "git.stage_all", Title: "Git: Stage All Changes", Category: "Git", Shortcut: ""},
		{ID: "git.unstage_all", Title: "Git: Unstage All Changes", Category: "Git", Shortcut: ""},
		{ID: "git.branch_switcher", Title: "Git: Switch Branch", Category: "Git", Shortcut: "Alt+B"},
		{ID: "git.ai_commit", Title: "Git: Generate Commit Message with AI", Category: "Git", Shortcut: "Alt+G"},
		{ID: "git.refresh", Title: "Git: Refresh Worktree Status", Category: "Git", Shortcut: ""},
		{ID: "git.hunk_next", Title: "Git: Next Hunk (Diff/Sidebar)", Category: "Git", Shortcut: "n/]"},
		{ID: "git.hunk_prev", Title: "Git: Previous Hunk (Diff/Sidebar)", Category: "Git", Shortcut: "N/["},
		{ID: "git.hunk_stage", Title: "Git: Stage Active Hunk", Category: "Git", Shortcut: "s"},
		{ID: "git.hunk_unstage", Title: "Git: Unstage Active Hunk", Category: "Git", Shortcut: "u"},
		{ID: "git.hunk_discard", Title: "Git: Discard Active Hunk (Unstaged)", Category: "Git", Shortcut: "x"},
		{ID: "files.refresh", Title: "Files: Refresh File Explorer", Category: "Files", Shortcut: ""},
	}
	m.CommandPalette.Open(commands)
}

func (m *Model) openGlobalSearch() {
	if m.GlobalSearch != nil {
		m.GlobalSearch.SetWidth(m.modalWidth())
	}
	m.GlobalSearch.Open(m.RootDir)
}

func (m *Model) openFindBar() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil || (tab.Type == editor.TabMarkdown && tab.MarkdownPreview) || tab.IsDiff {
		m.StatusBar.ShowToast("Pencarian teks hanya didukung di buffer editor aktif", true, 2*time.Second)
		return
	}
	selectedText := tab.Buffer.GetSelectedText()
	m.FindBar.Open(selectedText)
	if selectedText != "" {
		tab.Buffer.SetSearchQuery(selectedText)
		cur, tot := tab.Buffer.MatchInfo()
		m.FindBar.SetMatches(cur, tot)
	}
}

func (m *Model) openReplaceBar() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil || (tab.Type == editor.TabMarkdown && tab.MarkdownPreview) || tab.IsDiff {
		m.StatusBar.ShowToast("Cari & Ganti hanya didukung di buffer editor aktif", true, 2*time.Second)
		return
	}
	selectedText := tab.Buffer.GetSelectedText()
	m.FindBar.OpenReplace(selectedText)
	if selectedText != "" {
		tab.Buffer.SetSearchQuery(selectedText)
		cur, tot := tab.Buffer.MatchInfo()
		m.FindBar.SetMatches(cur, tot)
	}
}

func (m *Model) openGotoLine() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil || (tab.Type == editor.TabMarkdown && tab.MarkdownPreview) || tab.IsDiff {
		m.StatusBar.ShowToast("Go to Line hanya didukung di buffer editor aktif", true, 2*time.Second)
		return
	}
	curLine := tab.Buffer.CursorRow + 1
	m.InputDialog.Open(dialog.InputGotoLine, "Lompat ke Baris [baris:kolom]", "", strconv.Itoa(curLine))
}

func (m *Model) openFileAt(path string, line, col int) {
	m.openFile(path)
	tab := m.TabManager.ActiveTab()
	if tab != nil && tab.Buffer != nil {
		if line > 0 {
			tab.Buffer.CursorRow = line - 1
		}
		if col > 0 {
			tab.Buffer.CursorCol = col - 1
		}
		tab.Buffer.ClearSelection()
		m.syncStatusBarWithActiveTab()
	}
}

func (m *Model) openBufferSwitcher() {
	if m.TabManager.Count() == 0 {
		m.StatusBar.ShowToast("Tidak ada tab terbuka", false, 2*time.Second)
		return
	}

	var items []dialog.SwitcherItem
	for i, t := range m.TabManager.Tabs {
		items = append(items, dialog.SwitcherItem{
			Index:   i,
			Title:   t.Name,
			Path:    t.Path,
			IsDirty: t.IsDirty(),
		})
	}

	m.Switcher.Open(items, m.TabManager.ActiveIndex)
}

func (m *Model) openBranchSwitcher() {
	if m.GitRepo == nil {
		m.StatusBar.ShowToast("Bukan repositori git aktif", true, 2*time.Second)
		return
	}
	branches, current, err := m.GitRepo.GetBranches()
	if err != nil || len(branches) == 0 {
		m.StatusBar.ShowToast("Tidak dapat membaca branch git", true, 2*time.Second)
		return
	}
	m.BranchSwitcher.Open(branches, current)
}

func (m *Model) syncStatusBarWithActiveTab() {
	if m.GitRepo != nil {
		if branch, err := m.GitRepo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	} else {
		m.StatusBar.Branch = ""
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil {
		m.StatusBar.FilePath = "[Tidak Ada File Terbuka]"
		m.StatusBar.Line = 0
		m.StatusBar.Col = 0
		m.StatusBar.FileSyntax = "None"
		return
	}

	if tab.IsDiff {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Git Diff"
		if tab.DiffView != nil {
			m.StatusBar.Line = tab.DiffView.ScrollOffset + 1
			m.StatusBar.Col = 1
		}
		return
	}

	if tab.Type == editor.TabTerminal {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Terminal (PTY)"
		m.StatusBar.Line = 1
		m.StatusBar.Col = 1
		return
	}

	if tab.Type == editor.TabMarkdown && tab.MarkdownPreview {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Markdown [Preview]"
		if tab.MarkdownViewer != nil {
			m.StatusBar.Line = tab.MarkdownViewer.ScrollOffset + 1
			m.StatusBar.Col = 1
		}
		return
	}

	rel, err := filepath.Rel(m.Explorer.RootPath, tab.Path)
	if err != nil {
		rel = tab.Path
	}
	if tab.IsDirty() {
		rel += " *"
	}
	m.StatusBar.FilePath = rel

	if tab.Buffer != nil {
		m.StatusBar.Line = tab.Buffer.CursorRow + 1
		m.StatusBar.Col = tab.Buffer.CursorCol + 1
	}

	ext := strings.ToLower(filepath.Ext(tab.Path))
	switch ext {
	case ".go":
		m.StatusBar.FileSyntax = "Go"
	case ".md", ".markdown":
		m.StatusBar.FileSyntax = "Markdown"
	case ".json":
		m.StatusBar.FileSyntax = "JSON"
	case ".yaml", ".yml":
		m.StatusBar.FileSyntax = "YAML"
	case ".sh", ".bash":
		m.StatusBar.FileSyntax = "Shell"
	default:
		m.StatusBar.FileSyntax = "Plain Text"
	}
}

func (m *Model) getSelectedParentDir() string {
	node := m.Explorer.SelectedNode()
	if node == nil {
		return m.Explorer.RootPath
	}
	if node.IsDir {
		return node.Path
	}
	return filepath.Dir(node.Path)
}

func (m *Model) handleInputConfirmed() {
	val := m.InputDialog.Value()
	switch m.InputDialog.Type {
	case dialog.InputNewFile:
		createdPath, err := explorer.CreateFile(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.openFile(createdPath)
			m.StatusBar.ShowToast("File dibuat: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputNewFolder:
		createdPath, err := explorer.CreateFolder(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Folder dibuat: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputRename:
		newPath, err := explorer.RenamePath(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Diubah ke: "+filepath.Base(newPath), false, 2*time.Second)
		}

	case dialog.InputOpenFile:
		target := val
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.RootDir, target)
		}
		fi, err := os.Stat(target)
		if err != nil {
			m.StatusBar.ShowToast("File tidak ditemukan: "+err.Error(), true, 3*time.Second)
		} else if fi.IsDir() {
			m.StatusBar.ShowToast("Path adalah direktori, bukan file", true, 3*time.Second)
		} else {
			m.openFile(target)
			m.Focus = FocusEditor
		}

	case dialog.InputOpenFolder:
		target := val
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.RootDir, target)
		}
		cleanPath, err := filepath.Abs(target)
		if err != nil {
			m.StatusBar.ShowToast("Path tidak valid: "+err.Error(), true, 3*time.Second)
			return
		}
		fi, err := os.Stat(cleanPath)
		if err != nil || !fi.IsDir() {
			m.StatusBar.ShowToast("Direktori tidak ditemukan", true, 3*time.Second)
			return
		}

		m.RootDir = cleanPath
		if m.Config != nil {
			m.Config.LastDir = cleanPath
			_ = config.SaveConfig(m.Config)
		}
		m.Explorer = explorer.NewTree(cleanPath)
		if m.Config != nil {
			m.Explorer.ShowHidden = m.Config.ShowHidden
		}
		repo, _ := git.OpenRepo(cleanPath)
		m.GitRepo = repo
		if repo != nil {
			m.GitView = git.NewGitView(repo)
			branch, _ := repo.GetBranchName()
			m.StatusBar.Branch = branch
		} else {
			m.GitView = nil
			m.StatusBar.Branch = ""
		}
		m.syncStatusBarWithActiveTab()
		m.StatusBar.ShowToast("Folder dibuka: "+cleanPath, false, 2*time.Second)

	case dialog.InputGotoLine:
		parts := strings.Split(val, ":")
		line, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		col := 1
		if len(parts) > 1 {
			if c, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && c > 0 {
				col = c
			}
		}
		if err == nil && line > 0 {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil {
				tab.Buffer.CursorRow = line - 1
				tab.Buffer.CursorCol = col - 1
				tab.Buffer.ClearSelection()
				tab.Buffer.ClampCursor()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast(fmt.Sprintf("Baris %d, Kolom %d", line, col), false, 2*time.Second)
			}
		} else {
			m.StatusBar.ShowToast("Format baris tidak valid (gunakan: baris[:kolom])", true, 2*time.Second)
		}

	case dialog.InputFilterExplorer:
		m.Explorer.SetFilter(val)
		if val != "" {
			m.StatusBar.ShowToast(fmt.Sprintf("Filter aktif: '%s' (%d item)", val, len(m.Explorer.FlatNodes)), false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
		}
	}
}

func (m *Model) handleConfirmConfirmed() tea.Cmd {
	switch m.ConfirmDialog.Type {
	case dialog.ConfirmDelete:
		err := explorer.DeletePath(m.ConfirmDialog.TargetPath)
		if err != nil {
			m.StatusBar.ShowToast("Error hapus: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Berhasil dihapus: "+filepath.Base(m.ConfirmDialog.TargetPath), false, 2*time.Second)
		}

	case dialog.ConfirmUnsavedChanges:
		if m.ConfirmDialog.TargetPath == "app.exit" {
			_, _ = editor.TriggerAutoSave(m.TabManager)
			_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
			fmt.Print("\x1b]22;default\x1b\\")
			return tea.Quit
		} else if m.ClosingTabIdx >= 0 {
			_ = m.TabManager.SaveTab(m.ClosingTabIdx)
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Disimpan & ditutup", false, 2*time.Second)
		}

	case dialog.ConfirmExternalChange:
		targetPath := m.ConfirmDialog.TargetPath
		for _, tab := range m.TabManager.Tabs {
			if tab != nil && tab.Path == targetPath && tab.Buffer != nil {
				if err := tab.ReloadFromDisk(); err == nil {
					m.StatusBar.ShowToast(fmt.Sprintf("File '%s' berhasil dimuat ulang dari disk", tab.Name), false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Gagal reload file: "+err.Error(), true, 3*time.Second)
				}
				break
			}
		}
		m.syncStatusBarWithActiveTab()
	case dialog.ConfirmDiscardGit:
		if m.GitRepo != nil {
			err := m.GitRepo.DiscardFile(m.ConfirmDialog.TargetPath)
			if err != nil {
				m.StatusBar.ShowToast("Gagal membuang perubahan: "+err.Error(), true, 3*time.Second)
			} else {
				if m.GitView != nil {
					m.GitView.Refresh()
				}
				for _, tab := range m.TabManager.Tabs {
					if tab != nil && (tab.Path == m.ConfirmDialog.TargetPath || filepath.Base(tab.Path) == filepath.Base(m.ConfirmDialog.TargetPath)) {
						_ = tab.ReloadFromDisk()
					}
				}
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Perubahan dibuang: "+filepath.Base(m.ConfirmDialog.TargetPath), false, 2*time.Second)
			}
		}

	case dialog.ConfirmDiscardGitHunk:
		if m.GitRepo != nil && m.pendingGitHunkPath != "" {
			if err := m.GitRepo.DiscardHunk(m.pendingGitHunkPath, m.pendingGitHunkIndex); err != nil {
				m.StatusBar.ShowToast("Gagal discard hunk: "+err.Error(), true, 3*time.Second)
			} else {
				m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d dibuang", m.pendingGitHunkIndex+1), false, 2*time.Second)
				if m.GitView != nil {
					m.GitView.Refresh()
				}
				m.refreshDiffTabsForPath(m.pendingGitHunkPath)
				// reload any open editor tab of that file
				for _, tab := range m.TabManager.Tabs {
					if tab != nil && tab.Buffer != nil && filepath.ToSlash(filepath.Clean(tab.Path)) == filepath.ToSlash(filepath.Clean(filepath.Join(m.GitRepo.RootPath, m.pendingGitHunkPath))) {
						_ = tab.ReloadFromDisk()
					}
				}
				m.syncStatusBarWithActiveTab()
			}
		}
		m.pendingGitHunkPath = ""
		m.pendingGitHunkIndex = 0
	}
	return nil
}

func (m *Model) handleDiscardConfirmed() tea.Cmd {
	if m.ConfirmDialog.Type == dialog.ConfirmUnsavedChanges {
		if m.ConfirmDialog.TargetPath == "app.exit" {
			_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
			fmt.Print("\x1b]22;default\x1b\\")
			return tea.Quit
		} else if m.ClosingTabIdx >= 0 {
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Perubahan dibuang", false, 2*time.Second)
		}
	} else if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
		targetPath := m.ConfirmDialog.TargetPath
		for _, tab := range m.TabManager.Tabs {
			if tab != nil && tab.Path == targetPath && tab.Buffer != nil {
				diskBytes, err := os.ReadFile(targetPath)
				if err == nil {
					m.TabManager.OpenDiff(targetPath, string(diskBytes), tab.Buffer.Text(), false, "Δ "+filepath.Base(targetPath)+" (External)")
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Split Diff dibuka: Periksa & merge perubahan", false, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Gagal membaca disk: "+err.Error(), true, 3*time.Second)
				}
				break
			}
		}
	}
	return nil
}

func (m *Model) openFile(path string) {
	tab, err := m.TabManager.OpenFile(path)
	if err != nil {
		m.StatusBar.ShowToast(err.Error(), true, 3*time.Second)
		return
	}

	m.syncStatusBarWithActiveTab()
	m.StatusBar.ShowToast("Dibuka: "+tab.Name, false, 1500*time.Millisecond)
	_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
}

func isBinaryBytes(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	return bytes.IndexByte(data[:limit], 0x00) != -1
}


func (m *Model) isUnsafeForHunkAction(relPath string) bool {
	if m == nil || m.GitRepo == nil || relPath == "" {
		return true
	}
	abs := filepath.Join(m.GitRepo.RootPath, relPath)
	info, err := os.Stat(abs)
	if err != nil {
		// untracked/delete/rename-conflict-like ambiguity: keep safe fallback
		return false
	}
	if info.Size() > editor.MaxFileSize {
		return true
	}
	data, err := os.ReadFile(abs)
	if err == nil && isBinaryBytes(data) {
		return true
	}
	// also guard staged/base blobs by diff content size indirectly via openDiff checks elsewhere
	return false
}

func (m *Model) applyDiffHunkAction(tab *editor.Tab, action string) tea.Cmd {
	if m == nil || tab == nil || tab.DiffView == nil || m.GitRepo == nil {
		return nil
	}
	h, ok := tab.DiffView.ActiveHunkData()
	if !ok {
		m.StatusBar.ShowToast("Tidak ada hunk aktif", true, 2*time.Second)
		return nil
	}
	relPath := tab.Path
	if relPath == "" {
		m.StatusBar.ShowToast("Path diff tidak valid", true, 2*time.Second)
		return nil
	}
	if m.isUnsafeForHunkAction(relPath) {
		m.StatusBar.ShowToast("File binary/terlalu besar/ambiguous: gunakan aksi file-level", true, 3*time.Second)
		return nil
	}
	switch action {
	case "stage":
		if tab.DiffStaged {
			m.StatusBar.ShowToast("Gunakan unstage pada diff staged", true, 2*time.Second)
			return nil
		}
		if err := m.GitRepo.StageHunk(relPath, h.Index); err != nil {
			m.StatusBar.ShowToast("Gagal stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
			return nil
		}
		m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage", h.Index+1), false, 2*time.Second)
	case "unstage":
		if !tab.DiffStaged {
			m.StatusBar.ShowToast("Gunakan stage/discard pada diff unstaged", true, 2*time.Second)
			return nil
		}
		if err := m.GitRepo.UnstageHunk(relPath, h.Index); err != nil {
			m.StatusBar.ShowToast("Gagal unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
			return nil
		}
		m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage", h.Index+1), false, 2*time.Second)
	case "discard":
		if tab.DiffStaged {
			m.StatusBar.ShowToast("Discard hunk hanya untuk perubahan unstaged", true, 2*time.Second)
			return nil
		}
		m.pendingGitHunkPath = relPath
		m.pendingGitHunkIndex = h.Index
		m.ConfirmDialog.Open(
			dialog.ConfirmDiscardGitHunk,
			"Buang Hunk?",
			fmt.Sprintf("Buang hunk #%d pada '%s'?\nPerubahan worktree untuk hunk ini akan hilang permanen.", h.Index+1, relPath),
			relPath,
		)
		return nil
	default:
		return nil
	}
	if m.GitView != nil {
		m.GitView.Refresh()
	}
	m.refreshDiffTabsForPath(relPath)
	m.syncStatusBarWithActiveTab()
	return m.requestSessionPersist()
}

func (m *Model) refreshDiffTabsForPath(relPath string) {
	if m == nil || m.GitRepo == nil || relPath == "" {
		return
	}
	for _, t := range m.TabManager.Tabs {
		if t == nil || t.Type != editor.TabDiff || t.DiffView == nil || t.Path != relPath {
			continue
		}
		if t.DiffStaged {
			base, _ := m.GitRepo.GetFileContentAtHEAD(relPath)
			mod, _ := m.GitRepo.GetIndexFileContent(relPath)
			t.DiffView.Result = git.ComputeSplitDiff(relPath, base, mod)
		} else {
			base, _ := m.GitRepo.GetIndexFileContent(relPath)
			mod, _ := m.GitRepo.GetWorktreeFileContent(relPath)
			t.DiffView.Result = git.ComputeSplitDiff(relPath, base, mod)
		}
		t.DiffView.RecomputeHunks()
	}
}

func (m *Model) openDiff(relPath string, staged bool) {
	if m.GitRepo == nil {
		return
	}
	fullPath := filepath.Join(m.GitRepo.RootPath, relPath)
	if info, err := os.Stat(fullPath); err == nil {
		if info.Size() > editor.MaxFileSize {
			m.StatusBar.ShowToast(fmt.Sprintf("File terlalu besar (%d MB > 10MB)", info.Size()/(1024*1024)), true, 3*time.Second)
			return
		}
	}
	label := "Unstaged"
	base, mod := "", ""
	if staged {
		label = "Staged"
		base, _ = m.GitRepo.GetFileContentAtHEAD(relPath)
		mod, _ = m.GitRepo.GetIndexFileContent(relPath)
	} else {
		base, _ = m.GitRepo.GetIndexFileContent(relPath)
		mod, _ = m.GitRepo.GetWorktreeFileContent(relPath)
	}
	if isBinaryBytes([]byte(base)) || isBinaryBytes([]byte(mod)) {
		m.StatusBar.ShowToast("Diff file biner tidak didukung", true, 3*time.Second)
		return
	}
	title := "Δ " + filepath.Base(relPath) + " (" + label + ")"
	m.TabManager.OpenDiff(relPath, base, mod, staged, title)
	m.Focus = FocusEditor
	m.syncStatusBarWithActiveTab()
}

func (m *Model) handleMenuAction(action string) tea.Cmd {
	switch action {
	case "app.exit":
		if m.hasDirtyTabs() && !m.AutoSave {
			m.ConfirmDialog.Open(
				dialog.ConfirmUnsavedChanges,
				"Perubahan Belum Disimpan",
				"Terdapat buffer yang belum disimpan.\nSimpan sebelum keluar?",
				"app.exit",
			)
			return nil
		}
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
		fmt.Print("\x1b]22;default\x1b\\")
		return tea.Quit

	case "file.new":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFile, "Buat File Baru", parentDir, "")
	case "terminal.new":
		var cmd tea.Cmd
		*m, cmd = m.openNewTerminalTab()
		return cmd
	case "terminal.toggle_focus":
		m.toggleFocus()
		return nil
	case "terminal.close":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Type == editor.TabTerminal {
			m.TabManager.CloseTab(m.TabManager.ActiveIndex)
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Terminal ditutup", false, 1500*time.Millisecond)
		}
		return nil

	case "file.open":
		m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)

	case "file.open_folder":
		m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)

	case "file.format":
		var cmd tea.Cmd
		*m, cmd = m.triggerFormatCurrentBuffer()
		return cmd

	case "setting.format_on_save":
		if m.Config != nil {
			m.Config.FormatOnSave = !m.Config.FormatOnSave
			_ = config.SaveConfig(m.Config)
			if m.Config.FormatOnSave {
				m.StatusBar.ShowToast("Format on Save Diaktifkan", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Format on Save Dimatikan", true, 2*time.Second)
			}
		}
		return nil

	case "file.save":
		if err := m.TabManager.SaveActive(); err != nil {
			m.StatusBar.ShowToast("Gagal simpan: "+err.Error(), true, 3*time.Second)
		} else {
			m.StatusBar.ShowToast("File disimpan", false, 2*time.Second)
			m.syncStatusBarWithActiveTab()
		}

	case "file.close":
		m.attemptCloseTab(m.TabManager.ActiveIndex)

	case "setting.autosave":
		m.AutoSave = !m.AutoSave
		m.StatusBar.AutoSave = m.AutoSave
		if m.Config != nil {
			m.Config.AutoSave = m.AutoSave
			_ = config.SaveConfig(m.Config)
		}
		if m.AutoSave {
			m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
		}

	case "setting.sidebar":
		m.SidebarVisible = !m.SidebarVisible

	case "view.sidebar_narrow":
		m.adjustSidebarWidth(-2)

	case "view.sidebar_widen":
		m.adjustSidebarWidth(2)

	case "setting.softwrap", "view.soft_wrap":
		m.toggleSoftWrap()

	case "setting.hidden":
		m.Explorer.ToggleHidden()
		if m.Config != nil {
			m.Config.ShowHidden = m.Explorer.ShowHidden
			_ = config.SaveConfig(m.Config)
		}

	case "setting.shortcuts":
		m.ShowHelp = true
		if m.HelpDialog != nil {
			m.HelpDialog.Open(m.Height)
		}

	case "setting.ai":
		m.openAISettings()

	case "git.ai_commit":
		var cmd tea.Cmd
		*m, cmd = m.triggerAICommit()
		return cmd

	case "about.open":
		m.ShowAbout = true

	case "search.quick_open":
		m.openQuickOpen()

	case "search.palette":
		m.openCommandPalette()

	case "search.find":
		m.openFindBar()

	case "search.replace":
		m.openReplaceBar()

	case "edit.goto_line":
		m.openGotoLine()

	case "edit.undo":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil {
			if tab.Buffer.Undo() {
				tab.Buffer.ClampCursor()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Undo", false, 1*time.Second)
			}
		}

	case "edit.redo":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil {
			if tab.Buffer.Redo() {
				tab.Buffer.ClampCursor()
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Redo", false, 1*time.Second)
			}
		}

	case "edit.toggle_comment":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil {
			tab.Buffer.ToggleLineComment(tab.CommentPrefix())
			m.syncStatusBarWithActiveTab()
		}

	case "git.branch_switcher":
		m.openBranchSwitcher()

	case "explorer.filter":
		m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file:", m.Explorer.FilterQuery)

	case "search.global":
		m.openGlobalSearch()

	case "view.focus":
		m.toggleFocus()

	case "view.files":
		m.SidebarView = SidebarFiles
		m.SidebarVisible = true

	case "view.git":
		m.SidebarView = SidebarGit
		m.SidebarVisible = true
		if m.GitView != nil {
			m.GitView.Refresh()
		}

	case "view.search":
		m.SidebarView = SidebarSearch
		m.SidebarVisible = true
		if m.SearchView != nil {
			m.SearchView.DoSearch()
		}

	case "view.markdown":
		m.TabManager.ToggleActiveMarkdown()
		m.syncStatusBarWithActiveTab()

	case "tab.close_others":
		m.TabManager.CloseOtherTabs(m.TabManager.ActiveIndex)
		m.syncStatusBarWithActiveTab()

	case "tab.close_all":
		m.TabManager.CloseAllTabs()
		m.syncStatusBarWithActiveTab()

	case "git.stage_all":
		if m.GitRepo != nil {
			_ = m.GitRepo.StageAll()
			if m.GitView != nil {
				m.GitView.Refresh()
			}
			m.StatusBar.ShowToast("Semua perubahan dimasukkan ke staging area", false, 2*time.Second)
		}

	case "git.unstage_all":
		if m.GitRepo != nil {
			_ = m.GitRepo.UnstageAll()
			if m.GitView != nil {
				m.GitView.Refresh()
			}
			m.StatusBar.ShowToast("Semua perubahan dikeluarkan dari staging area", false, 2*time.Second)
		}

	case "git.refresh":
		if m.GitView != nil {
			m.GitView.Refresh()
		}
		if m.Explorer != nil {
			m.Explorer.Refresh()
		}
		m.StatusBar.ShowToast("Status explorer & git diperbarui", false, 2*time.Second)

	case "git.hunk_next":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.IsDiff && tab.DiffView != nil {
			tab.DiffView.NextHunk(m.Height - 3)
			m.syncStatusBarWithActiveTab()
		} else if m.GitView != nil && m.SidebarView == SidebarGit {
			if m.GitView.FocusArea == git.GitFocusStaged {
				m.GitView.NextSelectedHunk(true)
			} else {
				m.GitView.NextSelectedHunk(false)
			}
		}

	case "git.hunk_prev":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.IsDiff && tab.DiffView != nil {
			tab.DiffView.PrevHunk(m.Height - 3)
			m.syncStatusBarWithActiveTab()
		} else if m.GitView != nil && m.SidebarView == SidebarGit {
			if m.GitView.FocusArea == git.GitFocusStaged {
				m.GitView.PrevSelectedHunk(true)
			} else {
				m.GitView.PrevSelectedHunk(false)
			}
		}

	case "git.hunk_stage":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.IsDiff && tab.DiffView != nil {
			return m.applyDiffHunkAction(tab, "stage")
		}
		if m.GitView != nil && m.GitRepo != nil && m.GitView.FocusArea == git.GitFocusChanges {
			if f := m.GitView.SelectedChangesFile(); f != nil {
				if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
					if err := m.GitRepo.StageHunk(f.Path, hIdx); err != nil {
						m.StatusBar.ShowToast("Gagal stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
					}
					m.GitView.Refresh()
					m.refreshDiffTabsForPath(f.Path)
				}
			}
		}

	case "git.hunk_unstage":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.IsDiff && tab.DiffView != nil {
			return m.applyDiffHunkAction(tab, "unstage")
		}
		if m.GitView != nil && m.GitRepo != nil && m.GitView.FocusArea == git.GitFocusStaged {
			if f := m.GitView.SelectedStagedFile(); f != nil {
				if hIdx, ok := m.GitView.SelectedHunkIndex(true); ok {
					if err := m.GitRepo.UnstageHunk(f.Path, hIdx); err != nil {
						m.StatusBar.ShowToast("Gagal unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
					}
					m.GitView.Refresh()
					m.refreshDiffTabsForPath(f.Path)
				}
			}
		}

	case "git.hunk_discard":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.IsDiff && tab.DiffView != nil {
			return m.applyDiffHunkAction(tab, "discard")
		}
		if m.GitView != nil && m.GitRepo != nil && m.GitView.FocusArea == git.GitFocusChanges {
			if f := m.GitView.SelectedChangesFile(); f != nil {
				if hIdx, ok := m.GitView.SelectedHunkIndex(false); ok {
					m.pendingGitHunkPath = f.Path
					m.pendingGitHunkIndex = hIdx
					m.ConfirmDialog.Open(
						dialog.ConfirmDiscardGitHunk,
						"Buang Hunk?",
						fmt.Sprintf("Buang hunk #%d pada '%s'?\nPerubahan worktree untuk hunk ini akan hilang permanen.", hIdx+1, f.Path),
						f.Path,
					)
				}
			}
		}

	case "files.refresh":
		if m.Explorer != nil {
			m.Explorer.Refresh()
		}
		m.StatusBar.ShowToast("File explorer dimuat ulang", false, 2*time.Second)
	}
	return nil
}

func (m *Model) handleFilePickerConfirmed(selectedPath string) {
	if m.FilePicker.Mode == dialog.PickerModeFolder {
		cleanPath, err := filepath.Abs(selectedPath)
		if err != nil {
			cleanPath = selectedPath
		}
		fi, err := os.Stat(cleanPath)
		if err != nil || !fi.IsDir() {
			m.StatusBar.ShowToast("Direktori tidak valid", true, 3*time.Second)
			return
		}

		m.RootDir = cleanPath
		if m.Config != nil {
			m.Config.LastDir = cleanPath
			_ = config.SaveConfig(m.Config)
		}
		m.Explorer = explorer.NewTree(cleanPath)
		if m.Config != nil {
			m.Explorer.ShowHidden = m.Config.ShowHidden
		}
		repo, _ := git.OpenRepo(cleanPath)
		m.GitRepo = repo
		if repo != nil {
			m.GitView = git.NewGitView(repo)
			branch, _ := repo.GetBranchName()
			m.StatusBar.Branch = branch
		} else {
			m.GitView = nil
			m.StatusBar.Branch = ""
		}
		m.syncStatusBarWithActiveTab()
		m.StatusBar.ShowToast("Folder dibuka: "+cleanPath, false, 2*time.Second)
	} else {
		m.openFile(selectedPath)
		m.Focus = FocusEditor
	}
}

func (m *Model) openExplorerContextMenu(absX, absY int) {
	items := []ui.ContextMenuItem{
		{ID: "explorer_new_file", Label: "New File", Shortcut: "a"},
		{ID: "explorer_new_folder", Label: "New Folder", Shortcut: "A"},
		{ID: "explorer_rename", Label: "Rename", Shortcut: "F2"},
		{ID: "explorer_delete", Label: "Delete", Shortcut: "d"},
		{ID: "explorer_copy_path", Label: "Copy Relative Path"},
	}
	if node := m.Explorer.SelectedNode(); node == nil {
		items[2].Disabled = true
		items[3].Disabled = true
		items[4].Disabled = true
	}
	m.ContextMenu.OpenCustom(absX, absY, items, m.Width, m.Height)
}

func (m *Model) handleContextMenuAction(action string) tea.Cmd {
	switch action {
	case "tab_close_this":
		if m.ContextMenuTabIdx >= 0 {
			m.attemptCloseTab(m.ContextMenuTabIdx)
		}
		return nil

	case "tab_close_others":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx < 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.AutoSave {
			for i, t := range m.TabManager.Tabs {
				if i != targetIdx && t.IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseOtherTabs(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i, t := range m.TabManager.Tabs {
				if i != targetIdx && t.IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Simpan atau buang perubahan sebelum menutup tab lainnya", true, 3*time.Second)
			} else {
				m.TabManager.CloseOtherTabs(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_right":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx < 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.AutoSave {
			for i := targetIdx + 1; i < m.TabManager.Count(); i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseTabsToRight(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i := targetIdx + 1; i < m.TabManager.Count(); i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Simpan atau buang perubahan sebelum menutup tab di sebelah kanan", true, 3*time.Second)
			} else {
				m.TabManager.CloseTabsToRight(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_left":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx <= 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.AutoSave {
			for i := 0; i < targetIdx; i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseTabsToLeft(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i := 0; i < targetIdx; i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Simpan atau buang perubahan sebelum menutup tab di sebelah kiri", true, 3*time.Second)
			} else {
				m.TabManager.CloseTabsToLeft(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_all":
		if m.AutoSave {
			for i, t := range m.TabManager.Tabs {
				if t.IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseAllTabs()
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i, t := range m.TabManager.Tabs {
				if t.IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Simpan atau buang perubahan sebelum menutup semua tab", true, 3*time.Second)
			} else {
				m.TabManager.CloseAllTabs()
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil
	case "explorer_new_file":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFile, "Buat File Baru", parentDir, "")
		return nil

	case "explorer_new_folder":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFolder, "Buat Folder Baru", parentDir, "")
		return nil

	case "explorer_rename":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.InputDialog.Open(dialog.InputRename, "Ubah Nama: "+node.Name, node.Path, node.Name)
		}
		return nil

	case "explorer_delete":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.ConfirmDialog.Open(
				dialog.ConfirmDelete,
				"Konfirmasi Hapus",
				"Apakah Anda yakin ingin menghapus:\n"+node.Path+"?",
				node.Path,
			)
		}
		return nil

	case "explorer_copy_path":
		if node := m.Explorer.SelectedNode(); node != nil {
			if rel, err := filepath.Rel(m.RootDir, node.Path); err == nil {
				_ = clipboard.WriteAll(rel)
				m.StatusBar.ShowToast("Path disalin: "+rel, false, 2*time.Second)
			}
		}
		return nil

	case "explorer_clear_filter":
		m.Explorer.SetFilter("")
		m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
		return nil

	}

	tab := m.TabManager.ActiveTab()
	if tab == nil {
		return nil
	}

	if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
		switch action {
		case "copy":
			text := tab.MarkdownViewer.GetSelectedText()
			if text == "" {
				text = tab.MarkdownViewer.GetAllCleanText()
			}
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
		case "select_all":
			tab.MarkdownViewer.SelectAll()
		case "paste":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" && tab.Buffer != nil {
				tab.Buffer.InsertString(text)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Ditempel ke dokumen markdown", false, 2*time.Second)
			}
		}
		return nil
	}

	if tab.Buffer == nil {
		return nil
	}
	buf := tab.Buffer
	switch action {
	case "copy":
		text := buf.GetSelectedText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
		}
	case "cut":
		text := buf.GetSelectedText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			buf.DeleteSelection()
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
		}
	case "paste":
		text, err := clipboard.ReadAll()
		if err == nil && text != "" {
			buf.InsertString(text)
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
		}
	case "select_all":
		buf.SelectAll()
		m.syncStatusBarWithActiveTab()
	}
	return nil
}

func (m *Model) openAISettings() {
	if m.AISettingsDialog != nil && m.Config != nil {
		m.AISettingsDialog.Open(m.Config.AI)
	}
}
