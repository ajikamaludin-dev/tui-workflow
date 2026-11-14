package search

import (
	"fmt"
	"path/filepath"
)

// ReplaceJob is a pending replace operation built from a ReplaceRequest and a
// snapshot of the matches at the time the user asked for it.
type ReplaceJob struct {
	Scope       ReplaceScope
	FilePath    string // absolute path for ScopeFile and ScopeMatch
	LineNumber  int    // 1-based, ScopeMatch only
	ColNumber   int    // 1-based, ScopeMatch only
	Query       Query
	Replacement string
	Matches     []SearchMatch // snapshot to execute against
}

// Files returns the unique absolute file paths the snapshot touches.
func (j ReplaceJob) Files() []string {
	seen := make(map[string]bool)
	var out []string
	switch j.Scope {
	case ReplaceScopeFile, ReplaceScopeMatch:
		return []string{j.FilePath}
	default:
		for _, m := range j.Matches {
			if !seen[m.FilePath] {
				seen[m.FilePath] = true
				out = append(out, m.FilePath)
			}
		}
		return out
	}
}

// Occurrences returns the number of matches in the snapshot.
func (j ReplaceJob) Occurrences() int {
	switch j.Scope {
	case ReplaceScopeMatch:
		return 1
	case ReplaceScopeFile:
		n := 0
		for _, m := range j.Matches {
			if m.FilePath == j.FilePath {
				n++
			}
		}
		return n
	default:
		return len(j.Matches)
	}
}

// Describe returns the confirmation message.
func (j ReplaceJob) Describe() string {
	switch j.Scope {
	case ReplaceScopeFile:
		return fmt.Sprintf("Replace %d occurrence(s) in '%s'?", j.Occurrences(), filepath.Base(j.FilePath))
	case ReplaceScopeMatch:
		return fmt.Sprintf("Replace 1 occurrence in '%s'?", filepath.Base(j.FilePath))
	default:
		return fmt.Sprintf("Replace %d occurrence(s) in %d file(s)?", j.Occurrences(), len(j.Files()))
	}
}

// BuildReplaceJob filters the given matches by scope.
func BuildReplaceJob(req ReplaceRequest, query Query, replacement string, matches []SearchMatch) ReplaceJob {
	job := ReplaceJob{
		Scope:       req.Scope,
		FilePath:    req.FilePath,
		LineNumber:  req.LineNumber,
		ColNumber:   req.ColNumber,
		Query:       query,
		Replacement: replacement,
	}
	switch req.Scope {
	case ReplaceScopeFile:
		for _, m := range matches {
			if m.FilePath == req.FilePath {
				job.Matches = append(job.Matches, m)
			}
		}
	case ReplaceScopeMatch:
		for _, m := range matches {
			if m.FilePath == req.FilePath && m.LineNumber == req.LineNumber && m.ColNumber == req.ColNumber {
				job.Matches = append(job.Matches, m)
				break
			}
		}
	default:
		job.Matches = append(job.Matches, matches...)
	}
	return job
}

// Token encodes the job as the ConfirmDialog TargetPath payload.
func (j ReplaceJob) Token() string {
	switch j.Scope {
	case ReplaceScopeAll:
		return "replace:all"
	case ReplaceScopeFile:
		return "replace:file:" + j.FilePath
	case ReplaceScopeMatch:
		return fmt.Sprintf("replace:match:%s:%d:%d", j.FilePath, j.LineNumber, j.ColNumber)
	default:
		return "replace:all"
	}
}
