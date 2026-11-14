package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/explorer"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui/dialog"
)

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
			if m.SymbolIndex != nil {
				m.SymbolIndex.UpdateFile(createdPath)
			}
			m.openFile(createdPath)
			m.StatusBar.ShowToast("File created: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputNewFolder:
		createdPath, err := explorer.CreateFolder(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Folder created: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputRename:
		newPath, err := explorer.RenamePath(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			if m.SymbolIndex != nil {
				m.SymbolIndex.RenameFile(m.InputDialog.TargetPath, newPath)
			}
			m.StatusBar.ShowToast("Diubah ke: "+filepath.Base(newPath), false, 2*time.Second)
		}

	case dialog.InputOpenFile:
		target := val
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.RootDir, target)
		}
		fi, err := os.Stat(target)
		if err != nil {
			m.StatusBar.ShowToast("File not found: "+err.Error(), true, 3*time.Second)
		} else if fi.IsDir() {
			m.StatusBar.ShowToast("Path is a directory, not a file", true, 3*time.Second)
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
			m.StatusBar.ShowToast("Invalid path: "+err.Error(), true, 3*time.Second)
			return
		}
		fi, err := os.Stat(cleanPath)
		if err != nil || !fi.IsDir() {
			m.StatusBar.ShowToast("Directory not found", true, 3*time.Second)
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
				m.StatusBar.ShowToast(fmt.Sprintf("Line %d, Column %d", line, col), false, 2*time.Second)
			}
		} else {
			m.StatusBar.ShowToast("Invalid line format (use: line[:column])", true, 2*time.Second)
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
	case dialog.ConfirmAlert:
		return nil
	case dialog.ConfirmDelete:
		err := explorer.DeletePath(m.ConfirmDialog.TargetPath)
		if err != nil {
			m.StatusBar.ShowToast("Delete error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			if m.SymbolIndex != nil {
				m.SymbolIndex.RemoveFile(m.ConfirmDialog.TargetPath)
			}
			m.StatusBar.ShowToast("Successfully deleted: "+filepath.Base(m.ConfirmDialog.TargetPath), false, 2*time.Second)
		}

	case dialog.ConfirmUnsavedChanges:
		if m.ConfirmDialog.TargetPath == "app.exit" {
			_, _ = editor.TriggerAutoSave(m.TabManager)
			if runningTerm := m.hasRunningTerminalProcesses(); runningTerm != nil {
				m.ConfirmDialog.Open(
					dialog.ConfirmCloseTerminal,
					"Active Terminal Process",
					fmt.Sprintf("Terminal '%s' still has active processes running.\nTerminate process and exit?", runningTerm.Name),
					"app.exit",
				)
				return nil
			}
			_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
			fmt.Print("\x1b]22;default\x1b\\")
			return tea.Quit
		} else if m.ClosingTabIdx >= 0 {
			_ = m.TabManager.SaveTab(m.ClosingTabIdx)
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Saved & closed", false, 2*time.Second)
		}

	case dialog.ConfirmCloseTerminal:
		if m.ConfirmDialog.TargetPath == "app.exit" {
			_, _ = editor.TriggerAutoSave(m.TabManager)
			_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
			fmt.Print("\x1b]22;default\x1b\\")
			return tea.Quit
		} else if m.ClosingTabIdx >= 0 {
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Terminal closed", false, 2*time.Second)
		}

	case dialog.ConfirmExternalChange:
		targetPath := m.ConfirmDialog.TargetPath
		for _, tab := range m.TabManager.Tabs {
			if tab != nil && tab.Path == targetPath && tab.Buffer != nil {
				if err := tab.ReloadFromDisk(); err == nil {
					m.StatusBar.ShowToast(fmt.Sprintf("File '%s' successfully reloaded from disk", tab.Name), false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Failed to reload file: "+err.Error(), true, 3*time.Second)
				}
				break
			}
		}
		m.syncStatusBarWithActiveTab()
	case dialog.ConfirmDiscardGit:
		if m.GitRepo != nil {
			err := m.GitRepo.DiscardFile(m.ConfirmDialog.TargetPath)
			if err != nil {
				m.StatusBar.ShowToast("Failed to discard changes: "+err.Error(), true, 3*time.Second)
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
				m.StatusBar.ShowToast("Changes discarded: "+filepath.Base(m.ConfirmDialog.TargetPath), false, 2*time.Second)
			}
		}

	case dialog.ConfirmDiscardGitHunk:
		if m.GitRepo != nil && m.pendingGitHunkPath != "" {
			if err := m.GitRepo.DiscardHunk(m.pendingGitHunkPath, m.pendingGitHunkIndex); err != nil {
				m.StatusBar.ShowToast("Failed to discard hunk: "+err.Error(), true, 3*time.Second)
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
			if runningTerm := m.hasRunningTerminalProcesses(); runningTerm != nil {
				m.ConfirmDialog.Open(
					dialog.ConfirmCloseTerminal,
					"Active Terminal Process",
					fmt.Sprintf("Terminal '%s' still has active processes running.\nTerminate process and exit?", runningTerm.Name),
					"app.exit",
				)
				return nil
			}
			_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
			fmt.Print("\x1b]22;default\x1b\\")
			return tea.Quit
		} else if m.ClosingTabIdx >= 0 {
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Changes discarded", false, 2*time.Second)
		}
	} else if m.ConfirmDialog.Type == dialog.ConfirmCloseTerminal {
		m.ClosingTabIdx = -1
		m.StatusBar.ShowToast("Terminal close cancelled", false, 1500*time.Millisecond)
	} else if m.ConfirmDialog.Type == dialog.ConfirmExternalChange {
		targetPath := m.ConfirmDialog.TargetPath
		for _, tab := range m.TabManager.Tabs {
			if tab != nil && tab.Path == targetPath && tab.Buffer != nil {
				diskBytes, err := os.ReadFile(targetPath)
				if err == nil {
					m.TabManager.OpenDiff(targetPath, string(diskBytes), tab.Buffer.Text(), false, "Δ "+filepath.Base(targetPath)+" (External)")
					m.syncStatusBarWithActiveTab()
					m.StatusBar.ShowToast("Split Diff opened: Review & merge changes", false, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Failed to read disk: "+err.Error(), true, 3*time.Second)
				}
				break
			}
		}
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
			m.StatusBar.ShowToast("Invalid directory", true, 3*time.Second)
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
