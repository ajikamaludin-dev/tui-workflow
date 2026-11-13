package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
)

type sessionRestoreNoticeMsg struct {
	text  string
	isErr bool
}

type sessionPersistMsg struct {
	err error
}

type sessionState struct {
	explicitCLIArg bool
	restored       bool
	skipped        []string
}

func (m *Model) canonicalRootDir() string {
	if m == nil || m.RootDir == "" {
		return ""
	}
	abs, err := filepath.Abs(m.RootDir)
	if err != nil {
		abs = m.RootDir
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = filepath.Clean(resolved)
	}
	return abs
}

func (m *Model) captureWorkspaceSession() *config.WorkspaceSession {
	if m == nil {
		return nil
	}
	root := m.canonicalRootDir()
	sess := &config.WorkspaceSession{
		Version:  config.SessionSchemaVersion,
		RootDir:  root,
		SavedAt:  time.Now(),
		OpenTabs: make([]config.SessionTab, 0, m.TabManager.Count()),
		Sidebar: config.SessionSidebar{
			Visible: m.SidebarVisible,
			Width:   m.SidebarWidth,
			View:    sidebarViewToString(m.SidebarView),
		},
		Focus: focusToString(m.Focus),
	}
	// capture only path-backed file/markdown tabs, no dirty buffer content, no diff/terminal
	for _, t := range m.TabManager.Tabs {
		if t == nil || t.Path == "" {
			continue
		}
		if t.Type == editor.TabDiff || t.Type == editor.TabTerminal || t.IsDiff {
			continue
		}
		if t.Buffer == nil {
			continue
		}
		typ := "file"
		if t.Type == editor.TabMarkdown {
			typ = "markdown"
		}
		// only tabs under or accessible as files; store canonical path
		abs := t.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, abs)
		}
		abs = filepath.Clean(abs)
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = filepath.Clean(resolved)
		}
		st := config.SessionTab{
			Path:            abs,
			Type:            typ,
			CursorRow:       t.Buffer.CursorRow,
			CursorCol:       t.Buffer.CursorCol,
			ScrollRow:       t.Buffer.ScrollRow,
			ScrollCol:       t.Buffer.ScrollCol,
			MarkdownPreview: t.Type == editor.TabMarkdown && t.MarkdownPreview,
		}
		sess.OpenTabs = append(sess.OpenTabs, st)
	}
	active := m.TabManager.ActiveTab()
	if active != nil && active.Path != "" && (active.Type == editor.TabFile || active.Type == editor.TabMarkdown) && active.Buffer != nil && !active.IsDiff {
		ap := active.Path
		if !filepath.IsAbs(ap) {
			ap = filepath.Join(root, ap)
		}
		if resolved, err := filepath.EvalSymlinks(ap); err == nil {
			ap = filepath.Clean(resolved)
		} else {
			ap = filepath.Clean(ap)
		}
		sess.ActiveTab = ap
	} else if len(sess.OpenTabs) > 0 {
		sess.ActiveTab = sess.OpenTabs[len(sess.OpenTabs)-1].Path
	}
	config.NormalizeWorkspaceSession(sess)
	return sess
}

func (m *Model) persistWorkspaceSessionCmd() tea.Cmd {
	if m == nil {
		return nil
	}
	sess := m.captureWorkspaceSession()
	root := m.canonicalRootDir()
	return func() tea.Msg {
		if sess == nil || root == "" {
			return sessionPersistMsg{}
		}
		err := config.SaveWorkspaceSession(root, sess)
		return sessionPersistMsg{err: err}
	}
}

func (m *Model) requestSessionPersist() tea.Cmd {
	if m == nil {
		return nil
	}
	// simple debounce: schedule save after short delay; last wins not tracked but cheap metadata
	sess := m.captureWorkspaceSession()
	root := m.canonicalRootDir()
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg {
		if sess == nil || root == "" {
			return sessionPersistMsg{}
		}
		err := config.SaveWorkspaceSession(root, sess)
		return sessionPersistMsg{err: err}
	})
}

func focusToString(f FocusArea) string {
	switch f {
	case FocusSidebar:
		return "sidebar"
	case FocusMenu:
		return "menu"
	default:
		return "editor"
	}
}

func sidebarViewToString(v SidebarView) string {
	switch v {
	case SidebarGit:
		return "git"
	case SidebarSearch:
		return "search"
	default:
		return "files"
	}
}

func focusFromString(s string) FocusArea {
	switch strings.ToLower(s) {
	case "sidebar":
		return FocusSidebar
	case "menu":
		return FocusMenu
	default:
		return FocusEditor
	}
}

func sidebarViewFromString(s string) SidebarView {
	switch strings.ToLower(s) {
	case "git":
		return SidebarGit
	case "search":
		return SidebarSearch
	default:
		return SidebarFiles
	}
}

func isSessionRestorableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path adalah direktori")
	}
	if info.Size() > editor.MaxFileSize {
		return fmt.Errorf("ukuran file (%d MB) melebihi batas 10MB", info.Size()/(1024*1024))
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 512)
	n, _ := f.Read(header)
	if bytes.IndexByte(header[:n], 0x00) != -1 {
		return fmt.Errorf("file biner tidak didukung")
	}
	return nil
}

func (m *Model) restoreWorkspaceSession(sess *config.WorkspaceSession) (int, []string) {
	skipped := []string{}
	if sess == nil || m == nil {
		return 0, skipped
	}
	config.NormalizeWorkspaceSession(sess)
	restored := 0
	// start clean for deterministic restore
	m.TabManager.CloseAllTabs()

	pathToIdx := map[string]int{}
	for _, st := range sess.OpenTabs {
		if st.Path == "" {
			continue
		}
		if err := isSessionRestorableFile(st.Path); err != nil {
			skipped = append(skipped, filepath.Base(st.Path))
			continue
		}
		tab, err := m.TabManager.OpenFile(st.Path)
		if err != nil || tab == nil || tab.Buffer == nil {
			skipped = append(skipped, filepath.Base(st.Path))
			continue
		}
		// apply cursor/scroll/preview (metadata only)
		maxRow := tab.Buffer.LineCount() - 1
		if maxRow < 0 {
			maxRow = 0
		}
		if st.CursorRow > maxRow {
			st.CursorRow = maxRow
		}
		if st.CursorRow < 0 {
			st.CursorRow = 0
		}
		lineLen := 0
		if st.CursorRow >= 0 && st.CursorRow < tab.Buffer.LineCount() {
			lineLen = len(tab.Buffer.Lines[st.CursorRow])
		}
		if st.CursorCol > lineLen {
			st.CursorCol = lineLen
		}
		if st.CursorCol < 0 {
			st.CursorCol = 0
		}
		tab.Buffer.CursorRow = st.CursorRow
		tab.Buffer.CursorCol = st.CursorCol
		tab.Buffer.ScrollRow = st.ScrollRow
		tab.Buffer.ScrollCol = st.ScrollCol
		if st.ScrollRow < 0 {
			tab.Buffer.ScrollRow = 0
		}
		if st.ScrollCol < 0 {
			tab.Buffer.ScrollCol = 0
		}
		if tab.Type == editor.TabMarkdown {
			tab.MarkdownPreview = st.MarkdownPreview
		}
		pathToIdx[st.Path] = len(m.TabManager.Tabs) - 1
		restored++
	}

	if sess.ActiveTab != "" {
		if idx, ok := pathToIdx[sess.ActiveTab]; ok {
			m.TabManager.ActiveIndex = idx
			m.TabManager.EnsureActiveTabVisible()
		}
	}
	if m.TabManager.Count() > 0 && (m.TabManager.ActiveIndex < 0 || m.TabManager.ActiveIndex >= m.TabManager.Count()) {
		m.TabManager.ActiveIndex = m.TabManager.Count() - 1
	}

	// restore sidebar/focus
	m.SidebarVisible = sess.Sidebar.Visible
	if sess.Sidebar.Width >= 15 {
		m.SidebarWidth = sess.Sidebar.Width
	} else if sess.Sidebar.Width > 0 {
		m.SidebarWidth = 15
	}
	m.SidebarView = sidebarViewFromString(sess.Sidebar.View)
	m.Focus = focusFromString(sess.Focus)
	if !m.SidebarVisible && m.Focus == FocusSidebar {
		m.Focus = FocusEditor
	}
	if m.Focus == FocusMenu {
		// menu focus is transient in UI flow; keep safe default
		m.Focus = FocusEditor
	}
	return restored, skipped
}
