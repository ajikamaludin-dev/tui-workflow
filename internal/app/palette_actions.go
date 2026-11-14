package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"path/filepath"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/git"
	"tui-workflow/internal/search"
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
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.Type == editor.TabTerminal {
			m.attemptCloseTab(m.TabManager.ActiveIndex)
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
		m.openSearchSidebar()

	case "search.toggle_case":
		if m.SearchView != nil {
			m.SearchView.CaseSensitive = !m.SearchView.CaseSensitive
			m.SearchView.DoSearch()
		}

	case "search.toggle_word":
		if m.SearchView != nil {
			m.SearchView.WholeWord = !m.SearchView.WholeWord
			m.SearchView.DoSearch()
		}

	case "search.toggle_regex":
		if m.SearchView != nil {
			m.SearchView.Regex = !m.SearchView.Regex
			m.SearchView.DoSearch()
		}

	case "search.replace_all":
		nm, cmd := m.handleSearchReplaceRequest(search.ReplaceRequest{Scope: search.ReplaceScopeAll})
		_ = nm
		return cmd

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
		if m.blockBatchCloseIfRunning("others", m.TabManager.ActiveIndex) {
			return nil
		}
		m.TabManager.CloseOtherTabs(m.TabManager.ActiveIndex)
		m.syncStatusBarWithActiveTab()

	case "tab.close_all":
		if m.blockBatchCloseIfRunning("all", -1) {
			return nil
		}
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
