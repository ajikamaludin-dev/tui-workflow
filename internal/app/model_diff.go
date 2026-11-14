package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui/dialog"
)

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
