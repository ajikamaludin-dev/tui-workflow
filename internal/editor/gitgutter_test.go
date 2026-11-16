package editor

import (
	"testing"

	"tui-workflow/internal/git"
)

func TestSetGitChangesMapping(t *testing.T) {
	buf := NewBuffer("a\nb\nc\n")
	buf.SetGitChanges(map[int]git.LineChange{
		1: git.LineAdded,
		3: git.LineModified,
		9: git.LineDeleted, // out of range line: stored but harmless
	})

	if got := buf.GitChangeAt(0); got != git.LineAdded {
		t.Errorf("line 0 (1-based 1) = LineAdded expected, got %v", got)
	}
	if got := buf.GitChangeAt(1); got != git.LineUnchanged {
		t.Errorf("line 1 (1-based 2) = LineUnchanged expected, got %v", got)
	}
	if got := buf.GitChangeAt(2); got != git.LineModified {
		t.Errorf("line 2 (1-based 3) = LineModified expected, got %v", got)
	}
}

func TestSetGitChangesReplaceAndClear(t *testing.T) {
	buf := NewBuffer("a\nb\n")
	buf.SetGitChanges(map[int]git.LineChange{1: git.LineAdded, 2: git.LineDeleted})

	// Replace with a smaller set: stale entries must disappear.
	buf.SetGitChanges(map[int]git.LineChange{2: git.LineModified})
	if got := buf.GitChangeAt(0); got != git.LineUnchanged {
		t.Errorf("stale entry not cleared, got %v", got)
	}
	if got := buf.GitChangeAt(1); got != git.LineModified {
		t.Errorf("expected LineModified, got %v", got)
	}

	buf.ClearGitChanges()
	if got := buf.GitChangeAt(1); got != git.LineUnchanged {
		t.Errorf("expected LineUnchanged after ClearGitChanges, got %v", got)
	}
}

func TestSetGitChangesIgnoresUnchanged(t *testing.T) {
	buf := NewBuffer("a\n")
	buf.SetGitChanges(map[int]git.LineChange{1: git.LineUnchanged})
	if got := buf.GitChangeAt(0); got != git.LineUnchanged {
		t.Errorf("LineUnchanged entries must not be stored, got %v", got)
	}
}
