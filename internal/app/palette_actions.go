package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m *Model) handleMenuAction(action string) tea.Cmd {
	switch action {
	case "app.exit":
		if m.hasDirtyTabs() && !m.AutoSave {
			m.ConfirmDialog.Open(
				dialog.ConfirmUnsavedChanges,
				"Unsaved Changes",
				"There are unsaved buffers.\nSave before exiting?",
				"app.exit",
			)
			return nil
		}
		if runningTerm := m.hasRunningTerminalProcesses(); runningTerm != nil {
			m.ConfirmDialog.Open(
				dialog.ConfirmCloseTerminal,
				"Active Terminal Process",
				fmt.Sprintf("Terminal '%s' still has active processes running.\nTerminate process and exit?", runningTerm.Name),
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
		m.InputDialog.Open(dialog.InputNewFile, "New File", parentDir, "")
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

	case "edit.lsp_completion":
		return m.triggerLSPCompletionCmd()

	case "edit.indent_rainbow":
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil {
			tab.Buffer.IndentRainbow = !tab.Buffer.IndentRainbow
			if tab.Buffer.IndentRainbow {
				m.StatusBar.ShowToast("Indent Rainbow Diaktifkan", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Indent Rainbow Dimatikan", true, 2*time.Second)
			}
		}
		return nil

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
			m.StatusBar.ShowToast("Save failed: "+err.Error(), true, 3*time.Second)
		} else {
			m.StatusBar.ShowToast("File saved", false, 2*time.Second)
			if m.SymbolIndex != nil {
				if t := m.TabManager.ActiveTab(); t != nil {
					m.SymbolIndex.UpdateFile(t.Path)
				}
			}
			m.syncStatusBarWithActiveTab()
		}

	case "file.close":
		m.attemptCloseTab(m.TabManager.ActiveIndex)
	case "tab.toggle_pin":
		if m.TabManager.Count() > 0 && m.TabManager.ActiveIndex >= 0 {
			tab := m.TabManager.ActiveTab()
			wasPinned := tab != nil && tab.IsPinned
			m.TabManager.TogglePinTab(m.TabManager.ActiveIndex)
			if wasPinned {
				m.StatusBar.ShowToast("Tab unpinned", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Tab pinned", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		}
		return nil

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

	case "setting.shortcuts", "about.shortcuts":
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

	case "edit.delete_word_left":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil {
			tab.Buffer.DeleteWordLeft()
			tab.Buffer.ClampCursor()
			m.syncStatusBarWithActiveTab()
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

	case "search.goto_symbol":
		m.openGoToSymbol()

	case "search.goto_definition":
		m.gotoDefinition()
		m.syncStatusBarWithActiveTab()

	case "search.find_references":
		m.findReferences()

	case "edit.add_next_occurrence":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			m.addNextOccurrence()
		}

	case "edit.add_cursor_above":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			tab.Buffer.AddCursorAbove()
			m.syncStatusBarWithActiveTab()
		}

	case "edit.add_cursor_below":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			tab.Buffer.AddCursorBelow()
			m.syncStatusBarWithActiveTab()
		}

	case "edit.fold":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			row := tab.Buffer.CursorRow
			if !tab.Buffer.ToggleFold(row) {
				for i := row - 1; i >= 0; i-- {
					if end, ok := tab.Buffer.FoldableAt(i); ok && end >= row {
						tab.Buffer.ToggleFold(i)
						break
					}
				}
			}
			tab.Buffer.ClampCursor()
			m.syncStatusBarWithActiveTab()
		}

	case "edit.unfold":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			row := tab.Buffer.CursorRow
			for i := row; i >= 0; i-- {
				if tab.Buffer.IsFoldedAt(i) {
					tab.Buffer.ToggleFold(i)
					break
				}
			}
			tab.Buffer.ClampCursor()
			m.syncStatusBarWithActiveTab()
		}

	case "edit.fold_all":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			tab.Buffer.FoldAll()
			tab.Buffer.ClampCursor()
			m.syncStatusBarWithActiveTab()
		}

	case "edit.unfold_all":
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.Buffer != nil && (tab.Type == editor.TabMarkdown && !tab.MarkdownPreview || tab.Type == editor.TabFile) {
			tab.Buffer.UnfoldAll()
			tab.Buffer.ClampCursor()
			m.syncStatusBarWithActiveTab()
		}

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
			m.StatusBar.ShowToast("All changes staged", false, 2*time.Second)
		}

	case "git.unstage_all":
		if m.GitRepo != nil {
			_ = m.GitRepo.UnstageAll()
			if m.GitView != nil {
				m.GitView.Refresh()
			}
			m.StatusBar.ShowToast("All changes unstaged", false, 2*time.Second)
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
						m.StatusBar.ShowToast("Failed to stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
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
						m.StatusBar.ShowToast("Failed to unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
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
		m.StatusBar.ShowToast("File explorer reloaded", false, 2*time.Second)
	}
	return nil
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
	case "tab_toggle_pin":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx >= 0 && targetIdx < m.TabManager.Count() {
			tab := m.TabManager.Tabs[targetIdx]
			wasPinned := tab.IsPinned
			m.TabManager.TogglePinTab(targetIdx)
			if wasPinned {
				m.StatusBar.ShowToast("Tab unpinned", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Tab pinned", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		}
		return nil
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
				m.StatusBar.ShowToast("Save or discard changes before closing other tabs", true, 3*time.Second)
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
				m.StatusBar.ShowToast("Save or discard changes before closing tabs to the right", true, 3*time.Second)
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
				m.StatusBar.ShowToast("Save or discard changes before closing tabs to the left", true, 3*time.Second)
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
				m.StatusBar.ShowToast("Save or discard changes before closing all tabs", true, 3*time.Second)
			} else {
				m.TabManager.CloseAllTabs()
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil
	case "explorer_new_file":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFile, "New File", parentDir, "")
		return nil

	case "explorer_new_folder":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFolder, "New Folder", parentDir, "")
		return nil

	case "explorer_rename":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.InputDialog.Open(dialog.InputRename, "Rename: "+node.Name, node.Path, node.Name)
		}
		return nil

	case "explorer_delete":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.ConfirmDialog.Open(
				dialog.ConfirmDelete,
				"Confirm Delete",
				"Are you sure you want to delete:\n"+node.Path+"?",
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

	if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
		switch action {
		case "copy":
			text := tab.TerminalView.SelectedText()
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
		case "paste":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" {
				_, _ = tab.TerminalView.Write([]byte(text))
				m.StatusBar.ShowToast("Ditempel ke terminal", false, 2*time.Second)
			}
		case "select_all":
			tab.TerminalView.SelectAll()
			m.StatusBar.ShowToast("Seluruh layar terminal dipilih", false, 2*time.Second)
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
