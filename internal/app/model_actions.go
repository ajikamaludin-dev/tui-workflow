package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/ui/dialog"
)

func (m *Model) toggleSoftWrap() {
	m.SoftWrap = !m.SoftWrap
	m.TabManager.SetSoftWrap(m.SoftWrap)
	if m.SoftWrap {
		m.StatusBar.ShowToast("Soft-Wrap Diaktifkan", false, 2*time.Second)
	} else {
		m.StatusBar.ShowToast("Soft-Wrap Dimatikan", true, 2*time.Second)
	}
	if m.Config != nil {
		m.Config.SoftWrap = m.SoftWrap
		_ = config.SaveConfig(m.Config)
	}
}

func (m *Model) adjustSidebarWidth(delta int) {
	if !m.SidebarVisible {
		return
	}
	m.setSidebarWidth(m.SidebarWidth + delta)
}

func (m *Model) setSidebarWidth(width int) {
	minWidth := 15
	maxWidth := m.Width - 20
	if maxWidth < minWidth {
		maxWidth = minWidth
	}
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if width != m.SidebarWidth {
		m.SidebarWidth = width
		if m.Config != nil {
			m.Config.SidebarWidth = width
			_ = config.SaveConfig(m.Config)
		}
	}
}

func (m *Model) setMousePointer(shape string) {
	if m.lastPointerShape != shape {
		m.lastPointerShape = shape
		fmt.Print("\x1b]22;" + shape + "\x1b\\")
	}
}

func (m *Model) checkWatchedFiles() {
	if m.TabManager == nil {
		return
	}
	for _, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Path == "" || tab.Type == editor.TabDiff || tab.Buffer == nil {
			continue
		}
		info, err := os.Stat(tab.Path)
		if err != nil {
			if os.IsNotExist(err) && !tab.LastModTime.IsZero() {
				m.StatusBar.ShowToast(fmt.Sprintf("File '%s' was deleted from disk", tab.Name), true, 3*time.Second)
				tab.LastModTime = time.Time{}
			}
			continue
		}

		if tab.LastModTime.IsZero() {
			tab.LastModTime = info.ModTime()
			continue
		}

		if info.ModTime().After(tab.LastModTime) {
			tab.LastModTime = info.ModTime()
			if !tab.Buffer.IsDirty {
				if err := tab.ReloadFromDisk(); err == nil {
					m.StatusBar.ShowToast(fmt.Sprintf("File '%s' diperbarui dari disk", tab.Name), false, 2*time.Second)
				}
			} else {
				m.StatusBar.ShowToast(fmt.Sprintf("Warning: File '%s' changed externally (has local modifications)", tab.Name), true, 4*time.Second)
				if m.ConfirmDialog != nil && !m.ConfirmDialog.Active {
					m.ConfirmDialog.Open(
						dialog.ConfirmExternalChange,
						"External Change Conflict",
						fmt.Sprintf("File '%s' was modified externally, but has local changes in editor.\nReload from disk or open Split Diff to merge?", tab.Name),
						tab.Path,
					)
				}
			}
		}
	}
}

func (m *Model) toggleFocus() {
	if m.Focus == FocusEditor {
		if m.AutoSave {
			saved, err := editor.TriggerAutoSave(m.TabManager)
			if saved > 0 {
				m.StatusBar.ShowToast(fmt.Sprintf("%d files saved", saved), false, 1500*time.Millisecond)
			} else if err != nil {
				m.StatusBar.ShowToast("Auto-save error: "+err.Error(), true, 3*time.Second)
			}
		}
		m.Focus = FocusSidebar
	} else {
		m.Focus = FocusEditor
	}
}

func (m *Model) hasDirtyTabs() bool {
	for _, t := range m.TabManager.Tabs {
		if t.IsDirty() {
			return true
		}
	}
	return false
}

func (m *Model) hasRunningTerminalProcesses() *editor.Tab {
	for _, tab := range m.TabManager.Tabs {
		if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil && tab.TerminalView.HasRunningProcess() {
			return tab
		}
	}
	return nil
}

// firstRunningTerminalInBatch returns the first unpinned terminal tab with an
// active child process that belongs to a bulk-close range ("right", "left",
// "others", or "all" relative to targetIdx; targetIdx ignored for "all").
func (m *Model) firstRunningTerminalInBatch(dir string, targetIdx int) *editor.Tab {
	for i, tab := range m.TabManager.Tabs {
		var inRange bool
		switch dir {
		case "right":
			inRange = i > targetIdx
		case "left":
			inRange = i < targetIdx
		case "others":
			inRange = i != targetIdx
		case "all":
			inRange = true
		}
		if !inRange || tab.IsPinned {
			continue
		}
		if tab.Type == editor.TabTerminal && tab.TerminalView != nil && tab.TerminalView.HasRunningProcess() {
			return tab
		}
	}
	return nil
}

// blockBatchCloseIfRunning blocks a bulk tab close when any affected terminal
// tab has an active child process, mirroring the dirty-buffer safeguard.
// Returns true when the bulk close was blocked.
func (m *Model) blockBatchCloseIfRunning(dir string, targetIdx int) bool {
	if running := m.firstRunningTerminalInBatch(dir, targetIdx); running != nil {
		m.StatusBar.ShowToast(fmt.Sprintf("Terminal '%s' masih memiliki proses berjalan — tutup satu per satu", running.Name), true, 3*time.Second)
		return true
	}
	return false
}

func (m *Model) attemptCloseTab(tabIdx int) {
	if tabIdx < 0 || tabIdx >= m.TabManager.Count() {
		return
	}

	tab := m.TabManager.Tabs[tabIdx]
	if tab.IsPinned {
		m.StatusBar.ShowToast("Tab is pinned (unpin tab first to close)", true, 3*time.Second)
		return
	}
	if tab.Type == editor.TabTerminal && tab.TerminalView != nil && tab.TerminalView.HasRunningProcess() {
		m.ClosingTabIdx = tabIdx
		m.ConfirmDialog.Open(
			dialog.ConfirmCloseTerminal,
			"Close Terminal?",
			fmt.Sprintf("Terminal '%s' has active processes running.\nTerminate process and close?", tab.Name),
			tab.Path,
		)
		return
	}
	if tab.IsDirty() && !m.AutoSave {
		m.ClosingTabIdx = tabIdx
		m.ConfirmDialog.Open(
			dialog.ConfirmUnsavedChanges,
			"Save Changes?",
			fmt.Sprintf("File '%s' has been modified.\nSave changes before closing?", tab.Name),
			tab.Path,
		)
		return
	}

	if tab.IsDirty() && m.AutoSave {
		_ = m.TabManager.SaveTab(tabIdx)
	}

	m.TabManager.CloseTab(tabIdx)
	m.syncStatusBarWithActiveTab()
}

func (m *Model) syncStatusBarWithActiveTab() {
	m.StatusBar.CursorCount = 1
	if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil {
		m.StatusBar.CursorCount = tab.Buffer.CursorCount()
	}
	if m.GitRepo != nil {
		if branch, err := m.GitRepo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	} else {
		m.StatusBar.Branch = ""
	}

	tab := m.TabManager.ActiveTab()
	if tab == nil {
		m.StatusBar.FilePath = "[No Open Files]"
		m.StatusBar.Line = 0
		m.StatusBar.Col = 0
		m.StatusBar.FileSyntax = "None"
		return
	}

	if tab.IsDiff {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Git Diff"
		if tab.DiffView != nil {
			m.StatusBar.Line = tab.DiffView.ScrollOffset + 1
			m.StatusBar.Col = 1
		}
		return
	}

	if tab.Type == editor.TabTerminal {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Terminal (PTY)"
		m.StatusBar.Line = 1
		m.StatusBar.Col = 1
		return
	}

	if tab.Type == editor.TabMarkdown && tab.MarkdownPreview {
		m.StatusBar.FilePath = tab.Name
		m.StatusBar.FileSyntax = "Markdown [Preview]"
		if tab.MarkdownViewer != nil {
			m.StatusBar.Line = tab.MarkdownViewer.ScrollOffset + 1
			m.StatusBar.Col = 1
		}
		return
	}

	rel, err := filepath.Rel(m.Explorer.RootPath, tab.Path)
	if err != nil {
		rel = tab.Path
	}
	if tab.IsDirty() {
		rel += " *"
	}
	m.StatusBar.FilePath = rel

	if tab.Buffer != nil {
		m.StatusBar.Line = tab.Buffer.CursorRow + 1
		m.StatusBar.Col = tab.Buffer.CursorCol + 1

		if m.LinterStore != nil {
			if diag := m.LinterStore.DiagnosticAt(tab.Path, tab.Buffer.CursorRow+1); diag != nil {
				m.StatusBar.ShowToast(fmt.Sprintf("[%s] %s: %s", diag.Source, diag.RuleID, diag.Message), diag.Severity == linter.SeverityError, 2*time.Second)
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(tab.Path))
	switch ext {
	case ".go":
		m.StatusBar.FileSyntax = "Go"
	case ".js", ".mjs", ".cjs":
		m.StatusBar.FileSyntax = "JavaScript"
	case ".jsx":
		m.StatusBar.FileSyntax = "React JSX"
	case ".ts":
		m.StatusBar.FileSyntax = "TypeScript"
	case ".tsx":
		m.StatusBar.FileSyntax = "React TSX"
	case ".md", ".markdown":
		m.StatusBar.FileSyntax = "Markdown"
	case ".json":
		m.StatusBar.FileSyntax = "JSON"
	case ".yaml", ".yml":
		m.StatusBar.FileSyntax = "YAML"
	case ".sh", ".bash":
		m.StatusBar.FileSyntax = "Shell"
	default:
		m.StatusBar.FileSyntax = "Plain Text"
	}
}

func (m *Model) openFile(path string) {
	tab, err := m.TabManager.OpenFile(path)
	if err != nil {
		m.StatusBar.ShowToast(err.Error(), true, 3*time.Second)
		return
	}

	m.syncStatusBarWithActiveTab()
	m.StatusBar.ShowToast("Dibuka: "+tab.Name, false, 1500*time.Millisecond)
	_ = config.SaveWorkspaceSession(m.canonicalRootDir(), m.captureWorkspaceSession())
}

func isBinaryBytes(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	return bytes.IndexByte(data[:limit], 0x00) != -1
}
