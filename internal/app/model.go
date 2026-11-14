package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func (m *Model) openGoToSymbol() {
	if m.SymbolIndex == nil {
		m.StatusBar.ShowToast("Symbol index not ready", true, 2*time.Second)
		return
	}
	m.GoToSymbolActive = true
	m.GoToSymbolQuery = ""
	m.GoToSymbolSelected = 0
	m.GoToSymbolScroll = 0
	m.updateGoToSymbolResults()
}

func (m *Model) closeGoToSymbol() {
	m.GoToSymbolActive = false
	m.GoToSymbolQuery = ""
	m.GoToSymbolResults = nil
	m.GoToSymbolSelected = 0
	m.GoToSymbolScroll = 0
}

func (m *Model) updateGoToSymbolResults() {
	if m.SymbolIndex == nil {
		m.GoToSymbolResults = nil
		return
	}
	q := strings.ToLower(strings.TrimSpace(m.GoToSymbolQuery))
	all := m.SymbolIndex.AllSymbols()
	if q == "" {
		m.GoToSymbolResults = append([]symbol.Symbol(nil), all...)
	} else {
		out := make([]symbol.Symbol, 0, len(all))
		for _, s := range all {
			name := strings.ToLower(s.Name)
			path := strings.ToLower(filepath.Base(s.FilePath))
			if strings.Contains(name, q) || strings.Contains(path, q) {
				out = append(out, s)
			}
		}
		m.GoToSymbolResults = out
	}
	sort.SliceStable(m.GoToSymbolResults, func(i, j int) bool {
		ai, aj := m.GoToSymbolResults[i], m.GoToSymbolResults[j]
		if q := strings.ToLower(strings.TrimSpace(m.GoToSymbolQuery)); q != "" {
			pi := strings.HasPrefix(strings.ToLower(ai.Name), q)
			pj := strings.HasPrefix(strings.ToLower(aj.Name), q)
			if pi != pj {
				return pi
			}
		}
		if ai.Name != aj.Name {
			return ai.Name < aj.Name
		}
		if ai.FilePath != aj.FilePath {
			return ai.FilePath < aj.FilePath
		}
		return ai.Line < aj.Line
	})
	if m.GoToSymbolSelected >= len(m.GoToSymbolResults) {
		m.GoToSymbolSelected = len(m.GoToSymbolResults) - 1
	}
	if m.GoToSymbolSelected < 0 {
		m.GoToSymbolSelected = 0
	}
	if m.GoToSymbolSelected < m.GoToSymbolScroll {
		m.GoToSymbolScroll = m.GoToSymbolSelected
	}
	maxRows := 12
	if m.GoToSymbolSelected >= m.GoToSymbolScroll+maxRows {
		m.GoToSymbolScroll = m.GoToSymbolSelected - maxRows + 1
	}
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case linter.LintFinishedMsg:
		if m.LinterStore != nil {
			m.LinterStore.Set(msg.FilePath, msg.Diagnostics)
		}
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && tab.Path == msg.FilePath {
			if m.LinterStore != nil {
				tab.Buffer.LineDiagnostics = m.LinterStore.LineDiagnostics(msg.FilePath)
			}
		}
		m.syncStatusBarWithActiveTab()
		return m, nil

	case lspCompletionResultMsg:
		if len(msg.Items) > 0 && m.CompletionPopup != nil {
			curX := msg.X
			curY := msg.Y
			if curX < 0 {
				curX = 0
			}
			if curX > m.Width-40 {
				curX = m.Width - 40
			}
			if curY > m.Height-10 {
				curY = curY - 10
			}
			if curY < 2 {
				curY = 2
			}
			// Prepend matching local snippets to LSP items
			var merged []lsp.CompletionItem
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil {
				local := lsp.BuildLocalCompletions(tab.Buffer, tab.Path, nil)
				for _, it := range local {
					if it.Kind == lsp.CompletionKindSnippet && (msg.Prefix == "" || strings.HasPrefix(strings.ToLower(it.Label), strings.ToLower(msg.Prefix))) {
						merged = append(merged, it)
					}
				}
			}
			merged = append(merged, msg.Items...)
			m.CompletionPopup.SetItems(merged, curX, curY, msg.Prefix)
		}
		return m, nil
	case sessionPersistMsg:
		// best-effort persistence; avoid noisy errors
		return m, nil

	case symbolIndexReadyMsg:
		return m, nil

	case gitStatusResultMsg:
		if m.GitView != nil {
			m.GitView.ApplyRefresh(msg.data)
			if m.GitRepo != nil {
				if branch, err := m.GitRepo.GetBranchName(); err == nil {
					m.StatusBar.Branch = branch
				}
			}
		}
		return m, nil

	case quickOpenWarmupMsg:
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
				m.StatusBar.ShowToast("Push failed: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Push succeeded", false, 3*time.Second)
			}
		}
		return m, nil

	case gitCommitResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				var userErr *git.ErrGitUserNotConfigured
				if errors.As(msg.err, &userErr) {
					m.openGitUserNotConfiguredAlert()
					m.StatusBar.ShowToast("Commit failed: Git user identity not configured", true, 4*time.Second)
				} else {
					m.StatusBar.ShowToast("Commit failed: "+msg.err.Error(), true, 4*time.Second)
				}
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
		m.StatusBar.ShowToast("Commit message suggested by AI (Ctrl+Enter to commit)", false, 4*time.Second)
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
					m.StatusBar.ShowToast("Document already formatted ("+msg.ToolName+")", false, 3*time.Second)
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
		m.syncTabDimensions()
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
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
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
		m.syncTabDimensions()
	}
}

func (m *Model) editorBodyHeight() int {
	h := m.Height - 5
	if h < 2 {
		h = 2
	}
	return h
}

func (m *Model) editorContentWidth() int {
	if m.SidebarVisible {
		w := m.Width - m.SidebarWidth
		if w < 10 {
			w = 10
		}
		return w
	}
	return m.Width
}

func (m *Model) syncTabDimensions() {
	contentWidth := m.editorContentWidth()
	bodyHeight := m.editorBodyHeight()
	for _, tab := range m.TabManager.Tabs {
		if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
			tab.TerminalView.Resize(contentWidth, bodyHeight)
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
				m.StatusBar.ShowToast(fmt.Sprintf("File '%s' was deleted from disk", tab.Name), true, 3*time.Second)
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
				m.StatusBar.ShowToast(fmt.Sprintf("Warning: File '%s' changed externally (has local modifications)", tab.Name), true, 4*time.Second)
				if m.ConfirmDialog != nil && !m.ConfirmDialog.Active {
					m.ConfirmDialog.Open(
						dialog.ConfirmExternalChange,
						"External Change Conflict",
						fmt.Sprintf("File '%s' was modified externally, but has local changes in editor.\nReload from disk or open Split Diff to merge?", tab.Name),
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
				m.StatusBar.ShowToast(fmt.Sprintf("%d files saved", saved), false, 1500*time.Millisecond)
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
	if tab.IsPinned {
		m.StatusBar.ShowToast("Tab is pinned (unpin tab first to close)", true, 3*time.Second)
		return
	}
	if tab.IsDirty() && !m.AutoSave {
		m.ClosingTabIdx = tabIdx
		m.ConfirmDialog.Open(
			dialog.ConfirmUnsavedChanges,
			"Save Changes?",
			fmt.Sprintf("File '%s' has been modified.\nSave changes before closing?", tab.Name),
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
		{ID: "file.open_folder", Title: "Open Folder", Category: "File", Shortcut: "Ctrl+K Ctrl+K"},
		{ID: "file.save", Title: "Save File", Category: "File", Shortcut: "Ctrl+S"},
		{ID: "file.close", Title: "Close Tab", Category: "File", Shortcut: "Ctrl+W"},
		{ID: "tab.toggle_pin", Title: "Pin / Unpin Tab", Category: "File", Shortcut: "Alt+P"},
		{ID: "tab.close_others", Title: "Close Other Tabs", Category: "File", Shortcut: ""},
		{ID: "tab.close_all", Title: "Close All Tabs", Category: "File", Shortcut: ""},
		{ID: "app.exit", Title: "Exit Application", Category: "File", Shortcut: "Ctrl+Q"},

		// Search & Navigation
		{ID: "search.quick_open", Title: "Quick Open File (Workspace)", Category: "Search", Shortcut: "Ctrl+P"},
		{ID: "search.palette", Title: "Command Palette", Category: "Search", Shortcut: "Ctrl+Shift+P / F1"},
		{ID: "search.goto_symbol", Title: "Go to Symbol in Workspace", Category: "Search", Shortcut: "Ctrl+Shift+O"},
		{ID: "search.goto_definition", Title: "Go to Definition", Category: "Search", Shortcut: "F12"},
		{ID: "search.find_references", Title: "Find References", Category: "Search", Shortcut: "Shift+F12"},
		{ID: "search.find", Title: "Find in Current File", Category: "Search", Shortcut: "Ctrl+F"},
		{ID: "search.replace", Title: "Find & Replace in Current File", Category: "Search", Shortcut: "Ctrl+H"},
		{ID: "search.global", Title: "Global Search in Workspace", Category: "Search", Shortcut: "Ctrl+Shift+F"},
		{ID: "edit.goto_line", Title: "Go to Line / Column", Category: "Search", Shortcut: "Ctrl+G"},
		{ID: "edit.add_next_occurrence", Title: "Add Next Occurrence (Multi-Cursor)", Category: "Edit", Shortcut: "Ctrl+Shift+D"},
		{ID: "edit.add_cursor_above", Title: "Add Cursor Above", Category: "Edit", Shortcut: "Ctrl+Alt+Up"},
		{ID: "edit.add_cursor_below", Title: "Add Cursor Below", Category: "Edit", Shortcut: "Ctrl+Alt+Down"},
		{ID: "edit.fold", Title: "Fold Block", Category: "Edit", Shortcut: "Ctrl+Shift+["},
		{ID: "edit.unfold", Title: "Unfold Block", Category: "Edit", Shortcut: "Ctrl+Shift+]"},
		{ID: "edit.fold_all", Title: "Fold All", Category: "Edit", Shortcut: "Ctrl+K Ctrl+0"},
		{ID: "edit.unfold_all", Title: "Unfold All", Category: "Edit", Shortcut: "Ctrl+K Ctrl+J"},
		{ID: "edit.undo", Title: "Undo Text Change", Category: "Edit", Shortcut: "Ctrl+Z"},
		{ID: "edit.redo", Title: "Redo Text Change", Category: "Edit", Shortcut: "Ctrl+Y"},
		{ID: "edit.toggle_comment", Title: "Toggle Comment (Line / Selection)", Category: "Edit", Shortcut: "Ctrl+/"},
		{ID: "edit.delete_word_left", Title: "Delete Word Left", Category: "Edit", Shortcut: "Alt+Backspace / Ctrl+Backspace"},
		{ID: "edit.lsp_completion", Title: "Trigger LSP Completion & Auto-Import", Category: "Edit", Shortcut: "Ctrl+Space"},
		{ID: "edit.indent_rainbow", Title: "Toggle Indent Rainbow Guide", Category: "Settings", Shortcut: ""},
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
		m.StatusBar.ShowToast("Text search is only supported in active editor buffer", true, 2*time.Second)
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
		m.StatusBar.ShowToast("Find & Replace is only supported in active editor buffer", true, 2*time.Second)
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
		m.StatusBar.ShowToast("Go to Line is only supported in active editor buffer", true, 2*time.Second)
		return
	}
	curLine := tab.Buffer.CursorRow + 1
	m.InputDialog.Open(dialog.InputGotoLine, "Go to Line [line:col]", "", strconv.Itoa(curLine))
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
		m.StatusBar.ShowToast("No open tabs", false, 2*time.Second)
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
		m.StatusBar.ShowToast("Not an active git repository", true, 2*time.Second)
		return
	}
	branches, current, err := m.GitRepo.GetBranches()
	if err != nil || len(branches) == 0 {
		m.StatusBar.ShowToast("Cannot read git branch", true, 2*time.Second)
		return
	}
	m.BranchSwitcher.Open(branches, current)
}

func (m *Model) syncStatusBarWithActiveTab() {
	m.StatusBar.CursorCount = 1
	if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil {
		m.StatusBar.CursorCount = tab.Buffer.CursorCount()
	}
	if m.GitRepo != nil {
		if branch, err := m.GitRepo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	} else {
		m.StatusBar.Branch = ""
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil {
		m.StatusBar.FilePath = "[No Open Files]"
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

		if m.LinterStore != nil {
			if diag := m.LinterStore.DiagnosticAt(tab.Path, tab.Buffer.CursorRow+1); diag != nil {
				m.StatusBar.ShowToast(fmt.Sprintf("[%s] %s: %s", diag.Source, diag.RuleID, diag.Message), diag.Severity == linter.SeverityError, 2*time.Second)
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(tab.Path))
	switch ext {
	case ".go":
		m.StatusBar.FileSyntax = "Go"
	case ".js", ".mjs", ".cjs":
		m.StatusBar.FileSyntax = "JavaScript"
	case ".jsx":
		m.StatusBar.FileSyntax = "React JSX"
	case ".ts":
		m.StatusBar.FileSyntax = "TypeScript"
	case ".tsx":
		m.StatusBar.FileSyntax = "React TSX"
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
		// deleted/untracked/ambiguous file: fallback to safe file-level actions
		return true
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
		m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
		return nil
	}
	relPath := tab.Path
	if relPath == "" {
		m.StatusBar.ShowToast("Invalid diff path", true, 2*time.Second)
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
			m.StatusBar.ShowToast("Failed to stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
			return nil
		}
		m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage", h.Index+1), false, 2*time.Second)
	case "unstage":
		if !tab.DiffStaged {
			m.StatusBar.ShowToast("Gunakan stage/discard pada diff unstaged", true, 2*time.Second)
			return nil
		}
		if err := m.GitRepo.UnstageHunk(relPath, h.Index); err != nil {
			m.StatusBar.ShowToast("Failed to unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
			return nil
		}
		m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage", h.Index+1), false, 2*time.Second)
	case "discard":
		if tab.DiffStaged {
			m.StatusBar.ShowToast("Discard hunk is only for unstaged changes", true, 2*time.Second)
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
		m.StatusBar.ShowToast("Binary file diff is not supported", true, 3*time.Second)
		return
	}
	title := "Δ " + filepath.Base(relPath) + " (" + label + ")"
	m.TabManager.OpenDiff(relPath, base, mod, staged, title)
	m.Focus = FocusEditor
	m.syncStatusBarWithActiveTab()
}

func (m *Model) openAISettings() {
	if m.AISettingsDialog != nil && m.Config != nil {
		m.AISettingsDialog.Open(m.Config.AI)
	}
}
