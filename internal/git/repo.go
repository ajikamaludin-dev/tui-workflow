package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

	sort.Slice(staged, func(i, j int) bool {
		return staged[i].Path < staged[j].Path
	})
	sort.Slice(unstaged, func(i, j int) bool {
		return unstaged[i].Path < unstaged[j].Path
	})

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
	sort.Strings(branches)
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

func (r *Repo) GetIndexFileContent(relPath string) (string, error) {
	if r == nil || r.r == nil {
		return "", fmt.Errorf("repositori tidak valid")
	}
	idx, err := r.r.Storer.Index()
	if err != nil || idx == nil {
		return "", nil
	}
	for _, entry := range idx.Entries {
		if entry.Name != relPath {
			continue
		}
		blob, err := r.r.BlobObject(entry.Hash)
		if err != nil {
			return "", err
		}
		reader, err := blob.Reader()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return r.GetFileContentAtHEAD(relPath)
}

func (r *Repo) SetIndexFileContent(relPath string, content string) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("repositori tidak valid")
	}
	fullPath := filepath.Join(r.RootPath, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0644)
	if st, err := os.Stat(fullPath); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(fullPath, []byte(content), mode); err != nil {
		return err
	}
	if _, err := r.worktree.Add(relPath); err != nil {
		return fmt.Errorf("gagal memperbarui index untuk '%s': %w", relPath, err)
	}
	return nil
}

func (r *Repo) GetWorktreeFileContent(relPath string) (string, error) {
	if r == nil || r.worktree == nil {
		return "", fmt.Errorf("worktree tidak valid")
	}
	fullPath := filepath.Join(r.RootPath, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (r *Repo) GetDiffHunks(relPath string, staged bool, contextLines int) ([]DiffHunk, SplitDiffResult, string, string, error) {
	if r == nil || r.worktree == nil || r.r == nil {
		return nil, SplitDiffResult{}, "", "", fmt.Errorf("repositori tidak valid")
	}
	headContent, err := r.GetFileContentAtHEAD(relPath)
	if err != nil {
		headContent = ""
	}
	if staged {
		indexContent, err := r.GetIndexFileContent(relPath)
		if err != nil {
			return nil, SplitDiffResult{}, "", "", err
		}
		res := ComputeSplitDiff(relPath, headContent, indexContent)
		return SplitDiffHunks(res, contextLines), res, headContent, indexContent, nil
	}
	indexContent, err := r.GetIndexFileContent(relPath)
	if err != nil {
		indexContent = headContent
	}
	workContent, err := r.GetWorktreeFileContent(relPath)
	if err != nil {
		return nil, SplitDiffResult{}, "", "", err
	}
	res := ComputeSplitDiff(relPath, indexContent, workContent)
	return SplitDiffHunks(res, contextLines), res, indexContent, workContent, nil
}

func invertUnifiedLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			out = append(out, l)
			continue
		}
		switch l[0] {
		case '+':
			out = append(out, "-"+l[1:])
		case '-':
			out = append(out, "+"+l[1:])
		default:
			out = append(out, l)
		}
	}
	return out
}

func (r *Repo) StageHunk(relPath string, hunkIndex int) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("repositori tidak valid")
	}
	hunks, _, indexContent, _, err := r.GetDiffHunks(relPath, false, 3)
	if err != nil {
		return err
	}
	if hunkIndex < 0 || hunkIndex >= len(hunks) {
		return fmt.Errorf("hunk tidak ditemukan")
	}
	newIndex, err := ApplyUnifiedHunkToText(indexContent, hunks[hunkIndex], "orig")
	if err != nil {
		return fmt.Errorf("gagal stage hunk: %w", err)
	}
	return r.SetIndexFileContent(relPath, newIndex)
}

func (r *Repo) UnstageHunk(relPath string, hunkIndex int) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("repositori tidak valid")
	}
	hunks, _, _, indexContent, err := r.GetDiffHunks(relPath, true, 3)
	if err != nil {
		return err
	}
	if hunkIndex < 0 || hunkIndex >= len(hunks) {
		return fmt.Errorf("hunk tidak ditemukan")
	}
	inv := hunks[hunkIndex]
	inv.Lines = invertUnifiedLines(inv.Lines)
	newIndex, err := ApplyUnifiedHunkToText(indexContent, inv, "mod")
	if err != nil {
		return fmt.Errorf("gagal unstage hunk: %w", err)
	}
	return r.SetIndexFileContent(relPath, newIndex)
}

func (r *Repo) DiscardHunk(relPath string, hunkIndex int) error {
	if r == nil || r.worktree == nil {
		return fmt.Errorf("repositori tidak valid")
	}
	hunks, _, _, workContent, err := r.GetDiffHunks(relPath, false, 3)
	if err != nil {
		return err
	}
	if hunkIndex < 0 || hunkIndex >= len(hunks) {
		return fmt.Errorf("hunk tidak ditemukan")
	}
	inv := hunks[hunkIndex]
	inv.Lines = invertUnifiedLines(inv.Lines)
	newWork, err := ApplyUnifiedHunkToText(workContent, inv, "mod")
	if err != nil {
		return fmt.Errorf("gagal discard hunk: %w", err)
	}
	fullPath := filepath.Join(r.RootPath, relPath)
	mode := os.FileMode(0644)
	if st, err := os.Stat(fullPath); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(newWork), mode)
}
// GetStagedDiff returns unified diff text of all staged files against HEAD.
func (r *Repo) GetStagedDiff() (string, error) {
	if r == nil || r.worktree == nil {
		return "", fmt.Errorf("worktree tidak valid")
	}

	staged, _, err := r.GetStatus()
	if err != nil {
		return "", err
	}
	if len(staged) == 0 {
		return "", nil
	}

	var idxEntries map[string]plumbing.Hash
	if idx, err := r.r.Storer.Index(); err == nil && idx != nil {
		idxEntries = make(map[string]plumbing.Hash, len(idx.Entries))
		for _, entry := range idx.Entries {
			idxEntries[entry.Name] = entry.Hash
		}
	}

	var sb strings.Builder
	for _, f := range staged {
		orig, _ := r.GetFileContentAtHEAD(f.Path)

		var stagedContent string
		if f.Status == "D" {
			stagedContent = ""
		} else if hash, exists := idxEntries[f.Path]; exists {
			if blob, err := r.r.BlobObject(hash); err == nil {
				if reader, err := blob.Reader(); err == nil {
					data, _ := io.ReadAll(reader)
					_ = reader.Close()
					stagedContent = string(data)
				}
			}
		} else {
			// Fallback to worktree file
			fullPath := filepath.Join(r.RootPath, f.Path)
			if data, err := os.ReadFile(fullPath); err == nil {
				stagedContent = string(data)
			}
		}

		diffRes := ComputeSplitDiff(f.Path, orig, stagedContent)
		patch := FormatUnifiedDiff(diffRes, 3)
		if patch != "" {
			sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", f.Path, f.Path))
			if f.Status == "A" {
				sb.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n", f.Path))
			} else if f.Status == "D" {
				sb.WriteString(fmt.Sprintf("--- a/%s\n+++ /dev/null\n", f.Path))
			} else {
				sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", f.Path, f.Path))
			}
			sb.WriteString(patch)
		}
	}

	return sb.String(), nil
}
