package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"tui-workflow/internal/ai"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/explorer"
	"tui-workflow/internal/git"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/lsp"
	"tui-workflow/internal/search"
	"tui-workflow/internal/snippet"
	"tui-workflow/internal/symbol"
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
	SymbolIndex       *symbol.Index
	ContextMenu       *ui.ContextMenu
	AutoSave          bool
	SoftWrap          bool
	ShowHelp          bool
	ShowAbout         bool
	ClosingTabIdx     int
	ContextMenuTabIdx int
	DraggingTabIdx    int
	IsDraggingTab     bool
	DragTabStartX     int
	ResizingSidebar   bool
	lastPointerShape  string

	SessionExplicitArg bool

	// Pending git hunk discard confirmation
	pendingGitHunkPath  string
	pendingGitHunkIndex int
	SessionDirtySave    bool

	// Go To Symbol (Ctrl+Shift+O) modal state
	GoToSymbolActive   bool
	GoToSymbolQuery    string
	GoToSymbolResults  []symbol.Symbol
	GoToSymbolSelected int
	GoToSymbolScroll   int

	// Ctrl+K chord (fold commands like Ctrl+K Ctrl+0)
	ctrlKPending bool

	// Intelligent Code Assistance Suite
	LinterStore     *linter.Store
	LSPManager      *lsp.Manager
	CompletionPopup *lsp.CompletionPopup
	SnippetSession  *snippet.Session
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
		return Model{}, fmt.Errorf("path not found: %w", err)
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
		gv = git.NewGitViewLazy(repo)
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
		RootDir:            rootDir,
		Config:             cfg,
		Focus:              FocusEditor,
		SidebarVisible:     true,
		SidebarWidth:       initialSidebarWidth,
		SidebarView:        SidebarFiles,
		MenuBar:            ui.NewMenuBar(),
		StatusBar:          ui.NewStatusBar(),
		Explorer:           explorer.NewTree(rootDir),
		GitRepo:            repo,
		GitView:            gv,
		TabManager:         tm,
		FilePicker:         dialog.NewFilePicker(),
		InputDialog:        dialog.NewInputDialog(),
		ConfirmDialog:      dialog.NewConfirmDialog(),
		Switcher:           dialog.NewBufferSwitcher(),
		QuickOpen:          dialog.NewQuickOpen(),
		CommandPalette:     dialog.NewCommandPalette(),
		GlobalSearch:       dialog.NewGlobalSearch(),
		FindBar:            dialog.NewFindBar(),
		BranchSwitcher:     dialog.NewBranchSwitcher(),
		AISettingsDialog:   dialog.NewAISettingsDialog(),
		HelpDialog:         dialog.NewHelpDialog(),
		AIClient:           ai.NewClient(),
		SearchView:         search.NewSearchView(rootDir),
		SymbolIndex:        symbol.NewIndex(rootDir),
		ContextMenu:        ui.NewContextMenu(),
		AutoSave:           cfg.AutoSave,
		SoftWrap:           cfg.SoftWrap,
		ShowHelp:           false,
		ShowAbout:          false,
		ClosingTabIdx:      -1,
		ContextMenuTabIdx:  -1,
		DraggingTabIdx:     -1,
		SessionExplicitArg: explicitArg,
		LinterStore:        linter.NewStore(),
		LSPManager:         lsp.NewManager(rootDir),
		CompletionPopup:    lsp.NewCompletionPopup(),
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

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, marqueeTickCmd(), fileWatchTickCmd())
	if m.SymbolIndex != nil {
		cmds = append(cmds, buildSymbolIndexCmd(m.SymbolIndex))
	}
	if m.GitRepo != nil && m.GitView != nil {
		m.GitView.IsLoading = true
		m.GitView.LoadingMsg = "Memeriksa status git..."
		cmds = append(cmds, gitRefreshCmd(m.GitRepo), gitLoadingTickCmd())
	}
	if m.QuickOpen != nil && m.RootDir != "" {
		cmds = append(cmds, warmupQuickOpenCmd(m.QuickOpen, m.RootDir))
	}
	return tea.Batch(cmds...)
}
