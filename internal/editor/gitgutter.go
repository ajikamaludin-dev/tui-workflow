package editor

import (
	"tui-workflow/internal/git"
)

// SetGitChanges replaces the per-line git change markers for this buffer.
// Keys are 1-based physical line numbers (worktree side).
func (b *Buffer) SetGitChanges(changes map[int]git.LineChange) {
	if b.LineGitChanges == nil {
		b.LineGitChanges = make(map[int]git.LineChange, len(changes))
	} else {
		for k := range b.LineGitChanges {
			delete(b.LineGitChanges, k)
		}
	}
	for line, change := range changes {
		if line < 1 || change == git.LineUnchanged {
			continue
		}
		b.LineGitChanges[line-1] = change
	}
}

// ClearGitChanges removes all git change markers (e.g. not a git repo,
// unsupported file, or file identical to the index).
func (b *Buffer) ClearGitChanges() {
	if b.LineGitChanges != nil {
		for k := range b.LineGitChanges {
			delete(b.LineGitChanges, k)
		}
	}
}

// GitChangeAt returns the git change marker for a 0-based physical line.
func (b *Buffer) GitChangeAt(lineIdx int) git.LineChange {
	if b.LineGitChanges == nil {
		return git.LineUnchanged
	}
	return b.LineGitChanges[lineIdx]
}
