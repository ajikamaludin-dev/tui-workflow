package app

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/symbol"
	"tui-workflow/internal/ui/dialog"
)

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

func (m *Model) openAISettings() {
	if m.AISettingsDialog != nil && m.Config != nil {
		m.AISettingsDialog.Open(m.Config.AI)
	}
}
