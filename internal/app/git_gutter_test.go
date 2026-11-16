package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"tui-workflow/internal/git"
)

// commitTestFile writes name in repoDir and commits it (test helper).
func commitTestFile(t *testing.T, repoDir, name, content string) string {
	t.Helper()
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("plain open: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	path := filepath.Join(repoDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := wt.Add(name); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("add "+name, &gogit.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return path
}

func openInModel(t *testing.T, repoDir, path string) Model {
	t.Helper()
	m, err := NewModelWithPath(repoDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Height = 40
	m.openFile(path)
	return m
}

func TestGitGutterShowsUnstagedModified(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	path := filepath.Join(repoDir, "initial.txt")
	if err := os.WriteFile(path, []byte("line1 changed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := openInModel(t, repoDir, path)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatal("expected active buffer")
	}
	if got := tab.Buffer.GitChangeAt(0); got != git.LineModified {
		t.Errorf("expected line 0 = LineModified, got %v", got)
	}
}

func TestGitGutterStagedFileClears(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	path := filepath.Join(repoDir, "initial.txt")
	if err := os.WriteFile(path, []byte("line1 changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// stage the change: worktree == index → gutter must be empty
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("initial.txt"); err != nil {
		t.Fatal(err)
	}

	m := openInModel(t, repoDir, path)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatal("expected active buffer")
	}
	if got := tab.Buffer.GitChangeAt(0); got != git.LineUnchanged {
		t.Errorf("staged-only file must show no gutter change, got %v", got)
	}
}

func TestGitGutterUntrackedFileAllAdded(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	path := filepath.Join(repoDir, "brand_new.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := openInModel(t, repoDir, path)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatal("expected active buffer")
	}
	for i := 0; i < 2; i++ {
		if got := tab.Buffer.GitChangeAt(i); got != git.LineAdded {
			t.Errorf("untracked line %d = LineAdded expected, got %v", i, got)
		}
	}
}

func TestGitGutterDeletionMarkerOnNextLine(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	path := commitTestFile(t, repoDir, "three.txt", "keep-a\ndelete-me\nkeep-b\n")
	// worktree edit: drop the middle line (unstaged deletion at line 2)
	if err := os.WriteFile(path, []byte("keep-a\nkeep-b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := openInModel(t, repoDir, path)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatal("expected active buffer")
	}
	// deletion of orig line 2; marker lands on the following worktree line
	// (line 2 = "keep-b", 0-based index 1)
	if got := tab.Buffer.GitChangeAt(1); got != git.LineDeleted {
		t.Errorf("expected line 1 = LineDeleted for deletion marker, got %v", got)
	}
	if got := tab.Buffer.GitChangeAt(0); got != git.LineUnchanged {
		t.Errorf("expected line 0 = LineUnchanged, got %v", got)
	}
}
