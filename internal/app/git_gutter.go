package app

import (
	"path/filepath"
	"strings"

	"tui-workflow/internal/editor"
	"tui-workflow/internal/git"
)

// refreshGitGutters recomputes the git "Changes" gutter (index vs worktree)
// for all open file tabs. force=false skips tabs whose on-disk baseline
// (WatchedHash) is unchanged since the last computation.
func (m *Model) refreshGitGutters(force bool) {
	if m == nil || m.TabManager == nil {
		return
	}
	for _, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Buffer == nil || tab.Path == "" ||
			tab.Type == editor.TabDiff || tab.Type == editor.TabTerminal {
			continue
		}
		key := tab.WatchedHash
		if !force && key == tab.GitGutterKey {
			continue
		}
		m.refreshGitGutterForTab(tab)
		tab.GitGutterKey = key
	}
}

func (m *Model) refreshGitGutterForTab(tab *editor.Tab) {
	if m.GitRepo == nil {
		tab.Buffer.ClearGitChanges()
		return
	}
	rel, err := filepath.Rel(m.GitRepo.RootPath, tab.Path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		tab.Buffer.ClearGitChanges()
		return
	}
	rel = filepath.ToSlash(rel)
	workContent, werr := m.GitRepo.GetWorktreeFileContent(rel)
	if werr != nil {
		tab.Buffer.ClearGitChanges()
		return
	}
	if len(workContent) > editor.MaxFileSize {
		tab.Buffer.ClearGitChanges()
		return
	}
	binaryProbe := workContent
	if len(binaryProbe) > 512 {
		binaryProbe = binaryProbe[:512]
	}
	if isBinaryBytes([]byte(binaryProbe)) {
		tab.Buffer.ClearGitChanges()
		return
	}
	indexContent, ierr := m.GitRepo.GetIndexFileContent(rel)
	if ierr != nil {
		// Untracked (not in index, not in HEAD): the whole file is "added".
		indexContent = ""
	}
	tab.Buffer.SetGitChanges(git.ComputeLineChanges(indexContent, workContent))
}
