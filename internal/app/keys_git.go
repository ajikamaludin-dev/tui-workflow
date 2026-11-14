package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"path/filepath"
	"time"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m Model) handleSidebarKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// 7. Context-Specific: Sidebar (Files or Git)
	if m.Focus == FocusSidebar && m.SidebarVisible {
		if m.SidebarView == SidebarGit && m.GitView != nil {
			if ui.KeyName(msg) == "d" || ui.KeyName(msg) == "x" {
				if m.GitView.FocusArea == git.GitFocusChanges {
					sel := m.GitView.SelectedChangesFile()
					if sel != nil {
						m.ConfirmDialog.Open(
							dialog.ConfirmDiscardGit,
							"Discard Changes",
							fmt.Sprintf("Discard all local changes in '%s'? This action cannot be undone.", sel.Name),
							sel.Path,
						)
						return m, nil, true
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
								m.StatusBar.ShowToast("Failed to stage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
							} else {
								m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-stage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
							}
							if m.GitView != nil {
								m.GitView.Refresh()
							}
							m.refreshDiffTabsForPath(f.Path)
						} else {
							m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
						}
					}
				case "__hunk_unstage__":
					if f := m.GitView.SelectedStagedFile(); f != nil {
						if hIdx, ok := m.GitView.SelectedHunkIndex(true); ok {
							if err := m.GitRepo.UnstageHunk(f.Path, hIdx); err != nil {
								m.StatusBar.ShowToast("Failed to unstage hunk (fallback file-level): "+err.Error(), true, 3*time.Second)
							} else {
								m.StatusBar.ShowToast(fmt.Sprintf("Hunk %d di-unstage (%s)", hIdx+1, filepath.Base(f.Path)), false, 2*time.Second)
							}
							if m.GitView != nil {
								m.GitView.Refresh()
							}
							m.refreshDiffTabsForPath(f.Path)
						} else {
							m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
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
							return m, cmd, true
						}
						m.StatusBar.ShowToast("No active hunk", true, 2*time.Second)
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
			return m, cmd, true
		}

		if m.SidebarView == SidebarSearch && m.SearchView != nil {
			filePath, line, col, cmd := m.SearchView.Update(msg)
			if filePath != "" {
				m.openFileAt(filePath, line, col)
				m.Focus = FocusEditor
			}
			return m, cmd, true
		}

		if m.SidebarView == SidebarFiles {
			switch ui.KeyName(msg) {
			case "j", "down":
				m.Explorer.MoveDown()
				return m, nil, true

			case "k", "up":
				m.Explorer.MoveUp()
				return m, nil, true

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
				return m, nil, true

			case "h", "left":
				m.Explorer.CollapseOrParent()
				return m, nil, true

			case ".":
				m.Explorer.ToggleHidden()
				return m, nil, true

			case "r":
				m.Explorer.Refresh()
				m.StatusBar.ShowToast("Pohon file diperbarui", false, 2*time.Second)
				return m, nil, true
			case "/":
				m.InputDialog.Open(dialog.InputFilterExplorer, "Filter File Explorer", "Masukkan nama atau ekstensi file:", m.Explorer.FilterQuery)
				return m, nil, true
			case "esc":
				if m.Explorer.FilterQuery != "" {
					m.Explorer.SetFilter("")
					m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
					return m, nil, true
				}

			case "a":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFile, "New File", parentDir, "")
				return m, nil, true

			case "A":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFolder, "New Folder", parentDir, "")
				return m, nil, true

			case "F2":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.InputDialog.Open(dialog.InputRename, "Rename: "+node.Name, node.Path, node.Name)
				}
				return m, nil, true

			case "d":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.ConfirmDialog.Open(
						dialog.ConfirmDelete,
						"Confirm Delete",
						"Are you sure you want to delete:\n"+node.Path+"?",
						node.Path,
					)
				}
				return m, nil, true
			}
		}
	}
	return m, nil, false
}
