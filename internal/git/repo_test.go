package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestRepoOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Init git repo
	r, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create initial file & commit
	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("version 1"), 0644)
	_, _ = w.Add("file1.txt")
	_, err = w.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed initial commit: %v", err)
	}

	// Now open using our wrapper
	repo, err := OpenRepo(tempDir)
	if err != nil {
		t.Fatalf("OpenRepo failed: %v", err)
	}

	// Check history
	history, err := repo.GetHistory(10)
	if err != nil || len(history) != 1 {
		t.Fatalf("Expected 1 commit in history, got %d (err: %v)", len(history), err)
	}
	if history[0].Message != "Initial commit" {
		t.Errorf("Expected 'Initial commit', got '%s'", history[0].Message)
	}

	// Modify file1.txt
	_ = os.WriteFile(file1, []byte("version 2"), 0644)

	// Create untracked file2.txt
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("new file"), 0644)

	staged, unstaged, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(staged) != 0 {
		t.Errorf("Expected 0 staged files, got %d", len(staged))
	}
	if len(unstaged) < 2 {
		t.Errorf("Expected at least 2 unstaged files, got %d", len(unstaged))
	}

	// Stage file1.txt
	err = repo.StageFile("file1.txt")
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	staged, unstaged, err = repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus after stage failed: %v", err)
	}
	if len(staged) != 1 || staged[0].Path != "file1.txt" {
		t.Errorf("Expected file1.txt in staged, got %+v", staged)
	}

	// Commit staged changes
	hash, err := repo.Commit("Second commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if hash == "" {
		t.Errorf("Expected commit hash, got empty")
	}

	// Check history again
	history, err = repo.GetHistory(10)
	if err != nil || len(history) != 2 {
		t.Errorf("Expected 2 commits, got %d", len(history))
	}

	// Check GetFileContentAtHEAD
	headContent, err := repo.GetFileContentAtHEAD("file1.txt")
	if err != nil {
		t.Fatalf("GetFileContentAtHEAD failed: %v", err)
	}
	if headContent != "version 2" {
		t.Errorf("Expected 'version 2', got '%s'", headContent)
	}

	// Check GetBranchName
	branch, err := repo.GetBranchName()
	if err != nil {
		t.Fatalf("GetBranchName failed: %v", err)
	}
	if branch == "" {
		t.Errorf("Expected non-empty branch name")
	}

	// Test GitView rendering and button click
	gv := NewGitView(repo)
	_ = gv.Render(30, 20, true)

	// Hit test on buttons row (ClickButtonRow)
	// file2.txt is unstaged, so button should be fullwidth Commit
	var buttonY int = -1
	for y, tgt := range gv.ItemClickMap {
		if tgt.Type == ClickButtonRow {
			buttonY = y
			break
		}
	}
	if buttonY == -1 {
		t.Fatalf("ClickButtonRow not found in ItemClickMap")
	}

	_, action, handled := gv.HandleMouseClick(4, buttonY)
	if !handled || action != "commit" {
		t.Errorf("Expected action 'commit' with changes present, got %q (handled=%v)", action, handled)
	}

	// Test StageAll
	err = gv.StageAll()
	if err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if len(gv.Staged) == 0 {
		t.Errorf("Expected files in staged after StageAll")
	}

	// Test UnstageAll
	err = gv.UnstageAll()
	if err != nil {
		t.Fatalf("UnstageAll failed: %v", err)
	}
	if len(gv.Staged) != 0 {
		t.Errorf("Expected 0 staged files after UnstageAll, got %d", len(gv.Staged))
	}

	// Stage all and commit to clean the working tree
	_ = gv.StageAll()
	gv.CommitInput.SetValue("commit file2")
	_, err = gv.DoCommit()
	if err != nil {
		t.Fatalf("DoCommit failed: %v", err)
	}

	// Working tree is now clean, unpushed commits exist -> button should be Push
	unpushed, err := repo.GetUnpushedCount()
	if err != nil {
		t.Fatalf("GetUnpushedCount failed: %v", err)
	}
	if unpushed <= 0 {
		t.Errorf("Expected unpushed commits > 0, got %d", unpushed)
	}

	_ = gv.Render(30, 20, true)
	buttonY = -1
	for y, tgt := range gv.ItemClickMap {
		if tgt.Type == ClickButtonRow {
			buttonY = y
			break
		}
	}
	if buttonY == -1 {
		t.Fatalf("ClickButtonRow not found in ItemClickMap when clean")
	}

	_, action, handled = gv.HandleMouseClick(4, buttonY)
	if !handled || action != "push" {
		t.Errorf("Expected action 'push' when working tree clean, got %q (handled=%v)", action, handled)
	}

	// Test IsLoading
	gv.IsLoading = true
	gv.LoadingMsg = "Pushing..."
	rendered := gv.Render(30, 20, true)
	if !strings.Contains(rendered, "Pushing...") {
		t.Errorf("Expected rendered to contain 'Pushing...', got %s", rendered)
	}
	_, _, handled = gv.HandleMouseClick(4, buttonY)
	if handled && gv.IsLoading {
		// Click should be ignored during loading
	}
}

func TestRepoDiscardAndBranches(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-git-discard-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	r, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	trackedFile := filepath.Join(tempDir, "tracked.txt")
	_ = os.WriteFile(trackedFile, []byte("head content"), 0644)
	_, _ = w.Add("tracked.txt")
	_, err = w.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed initial commit: %v", err)
	}

	repo, err := OpenRepo(tempDir)
	if err != nil {
		t.Fatalf("OpenRepo failed: %v", err)
	}

	// 1. Test Discard modified tracked file
	_ = os.WriteFile(trackedFile, []byte("dirty content"), 0644)
	err = repo.DiscardFile("tracked.txt")
	if err != nil {
		t.Fatalf("DiscardFile tracked.txt failed: %v", err)
	}
	content, _ := os.ReadFile(trackedFile)
	if string(content) != "head content" {
		t.Errorf("Expected content to be restored to 'head content', got %q", string(content))
	}

	// 2. Test Discard untracked file (removes file)
	untrackedFile := filepath.Join(tempDir, "untracked.txt")
	_ = os.WriteFile(untrackedFile, []byte("untracked content"), 0644)
	err = repo.DiscardFile("untracked.txt")
	if err != nil {
		t.Fatalf("DiscardFile untracked.txt failed: %v", err)
	}
	if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
		t.Errorf("Expected untracked.txt to be removed by DiscardFile")
	}

	// 3. Test GetBranches and CheckoutBranch
	branches, current, err := repo.GetBranches()
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}
	if len(branches) == 0 || current == "" {
		t.Errorf("Expected branches and current branch to be populated, got branches=%v current=%q", branches, current)
	}

	// 4. Test GetStagedDiff
	diff, err := repo.GetStagedDiff()
	if err != nil {
		t.Fatalf("GetStagedDiff on clean repo returned error: %v", err)
	}
	if diff != "" {
		t.Errorf("Expected empty staged diff on clean repo, got: %s", diff)
	}

	// Modify tracked.txt and stage it
	_ = os.WriteFile(trackedFile, []byte("head content\nnew line in tracked"), 0644)
	if err := repo.StageFile("tracked.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	diff, err = repo.GetStagedDiff()
	if err != nil {
		t.Fatalf("GetStagedDiff failed: %v", err)
	}
	if len(diff) == 0 {
		t.Errorf("Expected non-empty staged diff, got empty")
	}
}

func TestGetStatusDeterministicOrder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-order-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	repo, err := OpenRepo(tempDir)
	if err != nil {
		t.Fatalf("OpenRepo failed: %v", err)
	}

	// Create several files in non-alphabetical order
	filenames := []string{"zebra.txt", "alpha.txt", "mango.txt", "beta.txt", "charlie.txt"}
	for _, fn := range filenames {
		_ = os.WriteFile(filepath.Join(tempDir, fn), []byte("content "+fn), 0644)
	}

	expectedOrder := []string{"alpha.txt", "beta.txt", "charlie.txt", "mango.txt", "zebra.txt"}

	// Call GetStatus multiple times, verify order is always identical and sorted
	for iter := 0; iter < 10; iter++ {
		_, unstaged, err := repo.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus iteration %d failed: %v", iter, err)
		}
		if len(unstaged) != len(expectedOrder) {
			t.Fatalf("Expected %d unstaged files, got %d", len(expectedOrder), len(unstaged))
		}
		for i, expected := range expectedOrder {
			if unstaged[i].Path != expected {
				t.Fatalf("Iteration %d: expected unstaged[%d] to be %q, got %q", iter, i, expected, unstaged[i].Path)
			}
		}
	}

	// Stage two files and verify staged list is also sorted
	_ = repo.StageFile("zebra.txt")
	_ = repo.StageFile("beta.txt")

	for iter := 0; iter < 10; iter++ {
		staged, _, err := repo.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus with staged iteration %d failed: %v", iter, err)
		}
		if len(staged) != 2 {
			t.Fatalf("Expected 2 staged files, got %d", len(staged))
		}
		if staged[0].Path != "beta.txt" || staged[1].Path != "zebra.txt" {
			t.Fatalf("Expected staged sorted [beta.txt, zebra.txt], got [%s, %s]", staged[0].Path, staged[1].Path)
		}
	}
}

