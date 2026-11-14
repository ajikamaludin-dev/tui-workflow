package editor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"tui-workflow/internal/git"
	"tui-workflow/internal/markdown"
	"tui-workflow/internal/terminal"
)

type TabType int

const (
	TabFile TabType = iota
	TabMarkdown
	TabDiff
	TabTerminal
)

type Tab struct {
	ID              string
	Path            string
	Name            string
	Type            TabType
	Buffer          *Buffer
	DiffView        *git.DiffView
	MarkdownViewer  *markdown.MarkdownViewer
	TerminalView    *terminal.TerminalView
	MarkdownPreview bool
	IsDiff          bool
	IsPinned        bool
	LastModTime     time.Time

	// Diff metadata for staged/unstaged and hunk actions
	DiffStaged   bool
	DiffHunkPath string
}

func NewFileTab(path, content string) *Tab {
	name := filepath.Base(path)
	tType := TabFile
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".md" || ext == ".markdown" {
		tType = TabMarkdown
	}

	var mv *markdown.MarkdownViewer
	if tType == TabMarkdown {
		mv = markdown.NewMarkdownViewer()
	}

	return &Tab{
		ID:              path,
		Path:            path,
		Name:            name,
		Type:            tType,
		Buffer:          NewBuffer(content),
		MarkdownViewer:  mv,
		MarkdownPreview: false,
		IsDiff:          false,
	}
}

func NewDiffTab(path, original, modified string) *Tab {
	name := "Δ " + filepath.Base(path)
	return &Tab{
		ID:              "diff:" + path + ":false",
		Path:            path,
		Name:            name,
		Type:            TabDiff,
		DiffView:        git.NewDiffView(path, original, modified),
		MarkdownPreview: false,
		IsDiff:          true,
		DiffStaged:      false,
		DiffHunkPath:    path,
	}
}

func NewTerminalTab(id, name string, term *terminal.TerminalView) *Tab {
	if name == "" && term != nil {
		name = term.Title
	}
	return &Tab{
		ID:           id,
		Path:         id,
		Name:         name,
		Type:         TabTerminal,
		TerminalView: term,
	}
}

func (t *Tab) IsDirty() bool {
	if t.Buffer != nil {
		return t.Buffer.IsDirty
	}
	return false
}

// ReloadFromDisk reloads the tab buffer directly from the file on disk.
func (t *Tab) ReloadFromDisk() error {
	if t.Path == "" || t.Buffer == nil {
		return nil
	}
	content, err := os.ReadFile(t.Path)
	if err != nil {
		return err
	}
	if info, errStat := os.Stat(t.Path); errStat == nil {
		t.LastModTime = info.ModTime()
	}
	t.Buffer.SetText(string(content))
	t.Buffer.IsDirty = false
	return nil
}

func (t *Tab) Title() string {
	title := t.Name
	if t.IsDirty() {
		title += "*"
	}
	return title
}

// CommentPrefix returns the appropriate single-line comment prefix for file type.
func (t *Tab) CommentPrefix() string {
	ext := strings.ToLower(filepath.Ext(t.Path))
	switch ext {
	case ".py", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml", ".rb", ".dockerfile", ".conf", ".ini", ".env":
		return "# "
	case ".sql", ".lua", ".hs":
		return "-- "
	default:
		if strings.EqualFold(filepath.Base(t.Path), "Dockerfile") {
			return "# "
		}
		return "// "
	}
}
