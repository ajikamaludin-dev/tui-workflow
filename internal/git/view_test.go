package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) (*Repo, string) {
	tempDir, err := os.MkdirTemp("", "tui-gitview-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	r, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create initial commit
	f := filepath.Join(tempDir, "base.txt")
	_ = os.WriteFile(f, []byte("base"), 0644)
	_, _ = w.Add("base.txt")
	_, err = w.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Initial commit failed: %v", err)
	}

	repo, err := OpenRepo(tempDir)
	if err != nil {
		t.Fatalf("OpenRepo failed: %v", err)
	}

	return repo, tempDir
}

func TestGitViewPromptAndFullwidthButton(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)

	// 1. Verify commit prompt is empty (no '>')
	if gv.CommitInput.Prompt != "" {
		t.Errorf("Expected CommitInput.Prompt to be empty, got %q", gv.CommitInput.Prompt)
	}

	// 2. Clean repo with 1 unpushed commit -> should show Push button
	rendered := gv.Render(30, 20, true)
	if !strings.Contains(rendered, "Push") {
		t.Errorf("Expected Push button rendered when working tree clean and unpushed commits exist")
	}

	// 3. Add an unstaged file -> button should switch to Commit
	newFile := filepath.Join(tempDir, "mod.txt")
	_ = os.WriteFile(newFile, []byte("mod"), 0644)
	gv.Refresh()

	rendered = gv.Render(30, 20, true)
	if !strings.Contains(rendered, "Commit") {
		t.Errorf("Expected Commit button rendered when working tree has changes")
	}

	// 4. Loading state
	gv.IsLoading = true
	gv.LoadingMsg = "Committing..."
	rendered = gv.Render(30, 20, true)
	if !strings.Contains(rendered, "Committing...") {
		t.Errorf("Expected Committing... rendered when IsLoading is true")
	}
}

func TestGitViewHeaderStageAndUnstageAll(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)

	// Create 2 unstaged files
	_ = os.WriteFile(filepath.Join(tempDir, "f1.txt"), []byte("1"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "f2.txt"), []byte("2"), 0644)
	gv.Refresh()

	if len(gv.Changes) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(gv.Changes))
	}

	_ = gv.Render(30, 20, true)

	// Find Changes header row
	var changesHeaderY int = -1
	for y, tgt := range gv.ItemClickMap {
		if tgt.Type == ClickSectionChanges {
			changesHeaderY = y
			break
		}
	}
	if changesHeaderY == -1 {
		t.Fatalf("ClickSectionChanges not found")
	}

	// Click [+] at right edge of Changes header
	_, _, handled := gv.HandleMouseClick(28, changesHeaderY)
	if !handled {
		t.Fatalf("HandleMouseClick on [+] not handled")
	}

	if len(gv.Staged) != 2 {
		t.Errorf("Expected 2 staged files after clicking [+], got %d", len(gv.Staged))
	}

	// Re-render and find Staged header row
	_ = gv.Render(30, 20, true)
	var stagedHeaderY int = -1
	for y, tgt := range gv.ItemClickMap {
		if tgt.Type == ClickSectionStaged {
			stagedHeaderY = y
			break
		}
	}
	if stagedHeaderY == -1 {
		t.Fatalf("ClickSectionStaged not found")
	}

	// Click [-] at right edge of Staged header
	_, _, handled = gv.HandleMouseClick(28, stagedHeaderY)
	if !handled {
		t.Fatalf("HandleMouseClick on [-] not handled")
	}

	if len(gv.Staged) != 0 {
		t.Errorf("Expected 0 staged files after clicking [-], got %d", len(gv.Staged))
	}
}

func TestGitViewScrolling(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)

	// Create 15 files to make list long
	for i := 0; i < 15; i++ {
		p := filepath.Join(tempDir, filepath.Clean(string(rune('a'+i))+".txt"))
		_ = os.WriteFile(p, []byte("x"), 0644)
	}
	gv.Refresh()

	_ = gv.Render(30, 10, true)

	if gv.TotalLines <= 10 {
		t.Errorf("Expected TotalLines > 10, got %d", gv.TotalLines)
	}

	// ScrollDown
	gv.ScrollDown(5, 10)
	if gv.ScrollOffset != 5 {
		t.Errorf("Expected ScrollOffset == 5, got %d", gv.ScrollOffset)
	}

	// ScrollUp
	gv.ScrollUp(2)
	if gv.ScrollOffset != 3 {
		t.Errorf("Expected ScrollOffset == 3, got %d", gv.ScrollOffset)
	}

	// ScrollUp beyond 0
	gv.ScrollUp(10)
	if gv.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset clamped to 0, got %d", gv.ScrollOffset)
	}
}

func TestGitViewSpinnerAnimation(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)
	gv.IsLoading = true
	gv.LoadingMsg = "Pushing..."

	// Frame 0 should have "⠋ Pushing..."
	rendered := gv.Render(30, 20, true)
	if !strings.Contains(rendered, "⠋ Pushing...") {
		t.Errorf("Expected frame 0 '⠋ Pushing...', got %s", rendered)
	}

	// Advance to frame 1 -> "⠙ Pushing..."
	gv.AdvanceSpinner()
	rendered = gv.Render(30, 20, true)
	if !strings.Contains(rendered, "⠙ Pushing...") {
		t.Errorf("Expected frame 1 '⠙ Pushing...', got %s", rendered)
	}

	// Advance through all 10 frames to test wrap-around
	for i := 0; i < len(spinnerFrames)*2; i++ {
		gv.AdvanceSpinner()
	}
	// Should wrap around safely
	rendered = gv.Render(30, 20, true)
	if !strings.Contains(rendered, "Pushing...") {
		t.Errorf("Expected rendered to contain Pushing... after full rotation, got %s", rendered)
	}
}

func TestGitView_CornerAIButton(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)
	// Render sidebar with width 32
	_ = gv.Render(32, 20, true)

	if gv.AIButtonStartX <= 0 {
		t.Errorf("expected AIButtonStartX > 0, got %d", gv.AIButtonStartX)
	}

	// Click on the right corner (relX >= AIButtonStartX, relY on commit input row e.g. line 1)
	_, action, handled := gv.HandleMouseClick(gv.AIButtonStartX+1, 1)
	if !handled || action != "ai_commit" {
		t.Errorf("expected action 'ai_commit', got action=%q handled=%v", action, handled)
	}

	// Click on the input field text area (relX = 5, relY = 1)
	_, action, handled = gv.HandleMouseClick(5, 1)
	if !handled || action != "" {
		t.Errorf("expected focus input (action ''), got action=%q handled=%v", action, handled)
	}
	if gv.FocusArea != GitFocusCommitInput {
		t.Errorf("expected focus area GitFocusCommitInput, got %v", gv.FocusArea)
	}
}

func TestGitView_MultilineCommitInputAndTextWrap(t *testing.T) {
	repo, tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gv := NewGitView(repo)

	// Verify height is 3 lines
	if gv.CommitInput.Height() != 3 {
		t.Errorf("Expected CommitInput.Height() to be 3, got %d", gv.CommitInput.Height())
	}

	// Verify rendering with long message wraps
	longMsg := "feat(git): add multiline commit input with auto wrap for long commit messages"
	gv.CommitInput.SetValue(longMsg)

	rendered := gv.Render(30, 20, true)

	// Rendered output should contain parts of wrapped message
	if !strings.Contains(rendered, "feat(git):") {
		t.Errorf("Expected rendered to contain 'feat(git):', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "multiline") {
		t.Errorf("Expected rendered to contain 'multiline', got:\n%s", rendered)
	}

	// Test keyboard navigation: Up stays at top row when row=0
	gv.CommitInput.SetCursor(0)
	gv.Update(tea.KeyMsg{Type: tea.KeyUp})
	if gv.FocusArea != GitFocusCommitInput {
		t.Errorf("Expected focus to remain GitFocusCommitInput on KeyUp, got %v", gv.FocusArea)
	}

	// Test KeyDown moves down in text until last line, then moves to GitFocusStaged
	gv.CommitInput.SetValue("line 1\nline 2")
	gv.CommitInput.CursorUp()
	if gv.CommitInput.Line() != 0 {
		t.Fatalf("Expected cursor on line 0, got %d", gv.CommitInput.Line())
	}
	// Line 1 -> KeyDown moves to Line 2
	gv.Update(tea.KeyMsg{Type: tea.KeyDown})
	if gv.FocusArea != GitFocusCommitInput {
		t.Errorf("Expected focus to remain GitFocusCommitInput while navigating lines, got %v", gv.FocusArea)
	}
	// On line 2 (last line) -> KeyDown exits to GitFocusStaged
	gv.Update(tea.KeyMsg{Type: tea.KeyDown})
	if gv.FocusArea != GitFocusStaged {
		t.Errorf("Expected focus to transition to GitFocusStaged at bottom line, got %v", gv.FocusArea)
	}
}
