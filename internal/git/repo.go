package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func (r *Repo) GetBranchName() (string, error) {
	if r == nil || r.r == nil {
		return "", fmt.Errorf("repositori tidak valid")
	}
	head, err := r.r.Head()
	if err != nil {
		return "", err
	}
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	h := head.Hash().String()
	if len(h) > 7 {
		return h[:7], nil
	}
	return h, nil
}

func (r *Repo) Push() error {
	if r == nil || r.r == nil {
		return fmt.Errorf("repositori tidak valid")
	}
	err := r.r.Push(&gogit.PushOptions{})
	if err != nil {
		if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return nil
		}
		return err
	}
	return nil
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

func (r *Repo) StageAll() error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("worktree tidak valid")
	}
	err := r.worktree.AddWithOptions(&gogit.AddOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("gagal stage semua file: %w", err)
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

func (r *Repo) UnstageAll() error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("worktree tidak valid")
	}
	head, err := r.r.Head()
	if err != nil {
		return nil
	}
	commit, err := r.r.CommitObject(head.Hash())
	if err != nil {
		return err
	}
	err = r.worktree.Reset(&gogit.ResetOptions{
		Commit: commit.Hash,
		Mode:   gogit.MixedReset,
	})
	if err != nil {
		return fmt.Errorf("gagal unstage semua file: %w", err)
	}
	return nil
}

func (r *Repo) GetUnpushedCount() (int, error) {
	if r == nil || r.r == nil {
		return 0, fmt.Errorf("repositori tidak valid")
	}
	branch, err := r.GetBranchName()
	if err != nil {
		return 0, err
	}
	head, err := r.r.Head()
	if err != nil {
		return 0, nil
	}
	remotes, err := r.r.Remotes()
	if err != nil || len(remotes) == 0 {
		iter, err := r.r.Log(&gogit.LogOptions{From: head.Hash()})
		if err != nil {
			return 0, nil
		}
		defer iter.Close()
		count := 0
		_ = iter.ForEach(func(c *object.Commit) error {
			count++
			return nil
		})
		return count, nil
	}
	remoteRefName := plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", branch))
	remoteRef, err := r.r.Reference(remoteRefName, true)
	if err != nil {
		iter, err := r.r.Log(&gogit.LogOptions{From: head.Hash()})
		if err != nil {
			return 0, nil
		}
		defer iter.Close()
		count := 0
		_ = iter.ForEach(func(c *object.Commit) error {
			count++
			return nil
		})
		return count, nil
	}
	if remoteRef.Hash() == head.Hash() {
		return 0, nil
	}
	iter, err := r.r.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	count := 0
	_ = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == remoteRef.Hash() {
			return io.EOF
		}
		count++
		return nil
	})
	return count, nil
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

// DiscardFile discards unstaged changes for a specific file in the worktree.
func (r *Repo) DiscardFile(relPath string) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("worktree tidak valid")
	}
	status, err := r.worktree.Status()
	if err != nil {
		return err
	}
	fileStatus, exists := status[relPath]
	if !exists {
		return nil
	}
	absPath := filepath.Join(r.RootPath, relPath)
	if fileStatus.Worktree == gogit.Untracked {
		return os.RemoveAll(absPath)
	}

	// For modified/deleted file: restore content from HEAD
	content, err := r.GetFileContentAtHEAD(relPath)
	if err != nil {
		return err
	}
	if content == "" {
		// File didn't exist in HEAD
		return os.RemoveAll(absPath)
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(absPath, []byte(content), 0644)
}

// GetBranches returns all local branch names and the current branch name.
func (r *Repo) GetBranches() (branches []string, current string, err error) {
	if r == nil || r.r == nil {
		return nil, "", fmt.Errorf("repositori tidak valid")
	}
	head, err := r.r.Head()
	if err == nil && head.Name().IsBranch() {
		current = head.Name().Short()
	}

	iter, err := r.r.Branches()
	if err != nil {
		return nil, current, err
	}
	defer iter.Close()

	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			branches = append(branches, ref.Name().Short())
		}
		return nil
	})
	return branches, current, nil
}

// CheckoutBranch switches the worktree to the specified local branch.
func (r *Repo) CheckoutBranch(branchName string) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("worktree tidak valid")
	}
	branchRef := plumbing.NewBranchReferenceName(branchName)
	return r.worktree.Checkout(&gogit.CheckoutOptions{
		Branch: branchRef,
		Force:  false,
	})
}
