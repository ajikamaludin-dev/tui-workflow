package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/git"
)

// setupGitChangesModel builds a repo with one committed file and one modified
// (unstaged) file, returning a model pointed at that repo with the Git sidebar active.
func setupGitChangesModel(t *testing.T) (string, Model) {
	t.Helper()
	repoDir, _ := setupTestGitRepoWithBranch(t)

	// Modify the tracked file so it appears in the Changes list
	if err := os.WriteFile(filepath.Join(repoDir, "initial.txt"), []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatalf("Failed to modify tracked file: %v", err)
	}

	m, err := NewModelWithPath(repoDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Width = 100
	m.Height = 30
	m.SidebarWidth = 28
	m.SidebarVisible = true
	m.SidebarView = SidebarGit
	if m.GitView == nil {
		t.Fatal("Expected GitView to be initialized")
	}
	m.GitView.Refresh()
	if len(m.GitView.Changes) == 0 {
		t.Fatal("Expected at least one change after modifying tracked file")
	}
	// Render to populate ItemClickMap
	_ = m.GitView.Render(m.SidebarWidth-1, m.Height-4, true)
	return repoDir, m
}

// changesItemScreenY locates the screen Y of the first ClickItemChanges row.
func changesItemScreenY(t *testing.T, m Model) int {
	t.Helper()
	for line, tgt := range m.GitView.ItemClickMap {
		if tgt.Type == git.ClickItemChanges {
			y := 3 + (line - m.GitView.ScrollOffset)
			if y >= 3 && y < m.Height-2 {
				return y
			}
		}
	}
	t.Fatal("No ClickItemChanges row found in ItemClickMap")
	return -1
}

func TestGitRightClickOpensContextMenuOnChangesItem(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	y := changesItemScreenY(t, m)
	newM, _ := m.Update(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseRight})
	m = newM.(Model)

	if m.ContextMenu == nil || !m.ContextMenu.Active {
		t.Fatal("Expected context menu to be active after right-clicking a changes item")
	}
	if m.ContextMenuGitPath == "" {
		t.Fatal("Expected ContextMenuGitPath to be set")
	}
	if m.ContextMenuGitStaged {
		t.Error("Expected ContextMenuGitStaged=false for a changes item")
	}
	ids := []string{}
	for _, item := range m.ContextMenu.Items {
		ids = append(ids, item.ID)
	}
	joined := strings.Join(ids, ",")
	for _, want := range []string{"git_stage", "git_open_file", "git_open_diff", "git_copy_path", "git_discard"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Expected menu to contain %q, got items: %s", want, joined)
		}
	}
	// The clicked row must be selected
	if m.GitView.ChangesIdx < 0 || m.GitView.Changes[m.GitView.ChangesIdx].Path != m.ContextMenuGitPath {
		t.Errorf("Expected ChangesIdx to match the clicked row path %q", m.ContextMenuGitPath)
	}
}

func TestGitRightClickOnHistoryRowDoesNotOpenMenu(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	// Render again to capture a History row (commit exists from repo setup)
	_ = m.GitView.Render(m.SidebarWidth-1, m.Height-4, true)
	for line, tgt := range m.GitView.ItemClickMap {
		if tgt.Type == git.ClickItemHistory {
			y := 3 + (line - m.GitView.ScrollOffset)
			if y >= 3 && y < m.Height-2 {
				newM, _ := m.Update(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseRight})
				m2 := newM.(Model)
				if m2.ContextMenu != nil && m2.ContextMenu.Active {
					t.Error("Expected no context menu when right-clicking a history row")
				}
				return
			}
		}
	}
	t.Skip("No history row visible; skipping")
}

func TestGitContextMenuStageAndUnstage(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	y := changesItemScreenY(t, m)
	newM, _ := m.Update(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseRight})
	m = newM.(Model)
	stagedPath := m.ContextMenuGitPath

	// Stage via context menu action
	_ = m.handleContextMenuAction("git_stage")
	m.GitView.Refresh()
	found := false
	for _, f := range m.GitView.Staged {
		if f.Path == stagedPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("Expected %q to be staged after git_stage action", stagedPath)
	}

	// Unstage via context menu action
	m.ContextMenuGitPath = stagedPath
	m.ContextMenuGitStaged = true
	_ = m.handleContextMenuAction("git_unstage")
	m.GitView.Refresh()
	for _, f := range m.GitView.Staged {
		if f.Path == stagedPath {
			t.Fatalf("Expected %q to be unstaged after git_unstage action", stagedPath)
		}
	}
}

func TestGitContextMenuOpenFileDisableOnDeleted(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	// Delete the tracked file -> status D in Changes
	if err := os.Remove(filepath.Join(repoDir, "initial.txt")); err != nil {
		t.Fatalf("Failed to delete tracked file: %v", err)
	}
	m.GitView.Refresh()
	_ = m.GitView.Render(m.SidebarWidth-1, m.Height-4, true)

	m.ContextMenuGitPath = "initial.txt"
	m.ContextMenuGitStaged = false
	m.ContextMenuGitStatus = "D"
	m.openGitContextMenu(5, 4)

	if m.ContextMenu == nil || !m.ContextMenu.Active {
		t.Fatal("Expected context menu to be active")
	}
	for _, item := range m.ContextMenu.Items {
		if item.ID == "git_open_file" && !item.Disabled {
			t.Error("Expected git_open_file to be disabled for a deleted (D) file")
		}
	}

	// Deleted file: Open File action must not open a tab
	before := m.TabManager.Count()
	_ = m.handleContextMenuAction("git_open_file")
	if m.TabManager.Count() != before {
		t.Error("Expected git_open_file to be a no-op for a deleted (D) file")
	}
}

func TestGitContextMenuOpenFileOpensEditorTab(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	before := m.TabManager.Count()
	m.ContextMenuGitPath = "initial.txt"
	m.ContextMenuGitStaged = false
	m.ContextMenuGitStatus = "M"
	_ = m.handleContextMenuAction("git_open_file")

	if m.TabManager.Count() != before+1 {
		t.Errorf("Expected one new tab after git_open_file, before=%d after=%d", before, m.TabManager.Count())
	}
	if m.Focus != FocusEditor {
		t.Error("Expected focus to move to the editor after git_open_file")
	}
}

func TestGitContextMenuGitLinkUsesRepoRoot(t *testing.T) {
	repoDir, m := setupGitChangesModel(t)
	defer os.RemoveAll(repoDir)

	m.ContextMenuGitPath = "initial.txt"
	m.ContextMenuGitStaged = false
	got := m.gitContextFullPath()
	want := filepath.Join(m.GitRepo.RootPath, "initial.txt")
	if got != want {
		t.Errorf("Expected full path %q, got %q", want, got)
	}
}
