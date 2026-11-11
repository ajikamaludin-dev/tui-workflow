package git

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitFileStatus struct {
	Path     string
	Name     string
	Status   string // "M", "A", "D", "?"
	IsStaged bool
}

type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	When    time.Time
}

func (c CommitInfo) RelativeTime() string {
	diff := time.Since(c.When)
	if diff < time.Minute {
		return "baru saja"
	} else if diff < time.Hour {
		return fmt.Sprintf("%dm lalu", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%dj lalu", int(diff.Hours()))
	}
	return fmt.Sprintf("%dh lalu", int(diff.Hours()/24))
}

type Repo struct {
	r        *gogit.Repository
	worktree *gogit.Worktree
	RootPath string
}

func OpenRepo(path string) (*Repo, error) {
	r, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("bukan repositori git: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("gagal membuka worktree: %w", err)
	}

	root := w.Filesystem.Root()
	return &Repo{
		r:        r,
		worktree: w,
		RootPath: root,
	}, nil
}

func (r *Repo) GetStatus() (staged []GitFileStatus, unstaged []GitFileStatus, err error) {
	status, err := r.worktree.Status()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal membaca status git: %w", err)
	}

	for path, fileStatus := range status {
		// 1. Check Staged (Index)
		if fileStatus.Staging != gogit.Unmodified && fileStatus.Staging != gogit.Untracked {
			staged = append(staged, GitFileStatus{
				Path:     path,
				Name:     filepath.Base(path),
				Status:   formatStatusCode(fileStatus.Staging),
				IsStaged: true,
			})
		}

		// 2. Check Unstaged (Worktree)
		if fileStatus.Worktree != gogit.Unmodified {
			unstaged = append(unstaged, GitFileStatus{
				Path:     path,
				Name:     filepath.Base(path),
				Status:   formatStatusCode(fileStatus.Worktree),
				IsStaged: false,
			})
		}
	}

	return staged, unstaged, nil
}

func formatStatusCode(code gogit.StatusCode) string {
	switch code {
	case gogit.Added:
		return "A"
	case gogit.Modified:
		return "M"
	case gogit.Deleted:
		return "D"
	case gogit.Untracked:
		return "U"
	case gogit.Renamed:
		return "R"
	case gogit.Copied:
		return "C"
	default:
		return "?"
	}
}

func (r *Repo) StageFile(relPath string) error {
	_, err := r.worktree.Add(relPath)
	if err != nil {
		return fmt.Errorf("gagal stage file '%s': %w", relPath, err)
	}
	return nil
}

func (r *Repo) UnstageFile(relPath string) error {
	// Equivalent to git reset HEAD <file>
	head, err := r.r.Head()
	if err != nil {
		// Initial commit with no HEAD yet, reset by removing from index
		return nil
	}

	commit, err := r.r.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	// Reset file back to HEAD state in index
	err = r.worktree.Reset(&gogit.ResetOptions{
		Commit: commit.Hash,
		Files:  []string{relPath},
		Mode:   gogit.MixedReset,
	})
	if err != nil {
		return fmt.Errorf("gagal unstage file '%s': %w", relPath, err)
	}
	return nil
}

func (r *Repo) Commit(message string) (string, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "", fmt.Errorf("pesan commit tidak boleh kosong")
	}

	commitHash, err := r.worktree.Commit(trimmed, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "tui-workflow",
			Email: "developer@tui-workflow.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("gagal melakukan commit: %w", err)
	}

	return commitHash.String()[:7], nil
}

func (r *Repo) GetHistory(limit int) ([]CommitInfo, error) {
	head, err := r.r.Head()
	if err != nil {
		return []CommitInfo{}, nil
	}

	iter, err := r.r.Log(&gogit.LogOptions{
		From: head.Hash(),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var commits []CommitInfo
	count := 0
	_ = iter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return io.EOF
		}
		hashShort := c.Hash.String()
		if len(hashShort) > 7 {
			hashShort = hashShort[:7]
		}
		firstLine := strings.Split(c.Message, "\n")[0]
		commits = append(commits, CommitInfo{
			Hash:    hashShort,
			Message: firstLine,
			Author:  c.Author.Name,
			When:    c.Author.When,
		})
		count++
		return nil
	})

	return commits, nil
}

func (r *Repo) GetFileContentAtHEAD(relPath string) (string, error) {
	head, err := r.r.Head()
	if err != nil {
		return "", nil // No commits yet
	}

	commit, err := r.r.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	file, err := tree.File(relPath)
	if err != nil {
		// File might not exist in HEAD (e.g. newly added)
		return "", nil
	}

	return file.Contents()
}
