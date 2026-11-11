package editor

import (
	"path/filepath"
	"strings"

	"tui-workflow/internal/git"
	"tui-workflow/internal/markdown"
)

type TabType int

const (
	TabFile TabType = iota
	TabMarkdown
	TabDiff
)

type Tab struct {
	ID              string
	Path            string
	Name            string
	Type            TabType
	Buffer          *Buffer
	DiffView        *git.DiffView
	MarkdownViewer  *markdown.MarkdownViewer
	MarkdownPreview bool
	IsDiff          bool
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
	name := "[Diff] " + filepath.Base(path)
	return &Tab{
		ID:              "diff:" + path,
		Path:            path,
		Name:            name,
		Type:            TabDiff,
		DiffView:        git.NewDiffView(path, original, modified),
		MarkdownPreview: false,
		IsDiff:          true,
	}
}

func (t *Tab) IsDirty() bool {
	if t.Buffer != nil {
		return t.Buffer.IsDirty
	}
	return false
}

func (t *Tab) Title() string {
	title := t.Name
	if t.IsDirty() {
		title += "*"
	}
	return title
}
