package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"path/filepath"
	"strconv"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

func (m *Model) openExplorerContextMenu(absX, absY int) {
	items := []ui.ContextMenuItem{
		{ID: "explorer_new_file", Label: "New File", Shortcut: "a"},
		{ID: "explorer_new_folder", Label: "New Folder", Shortcut: "A"},
		{ID: "explorer_rename", Label: "Rename", Shortcut: "F2"},
		{ID: "explorer_delete", Label: "Delete", Shortcut: "d"},
		{ID: "explorer_copy_path", Label: "Copy Relative Path"},
	}
	if node := m.Explorer.SelectedNode(); node == nil {
		items[2].Disabled = true
		items[3].Disabled = true
		items[4].Disabled = true
	}
	m.ContextMenu.OpenCustom(absX, absY, items, m.Width, m.Height)
}

func (m *Model) handleContextMenuAction(action string) tea.Cmd {
	switch action {
	case "tab_toggle_pin":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx >= 0 && targetIdx < m.TabManager.Count() {
			tab := m.TabManager.Tabs[targetIdx]
			wasPinned := tab.IsPinned
			m.TabManager.TogglePinTab(targetIdx)
			if wasPinned {
				m.StatusBar.ShowToast("Tab unpinned", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Tab pinned", false, 2*time.Second)
			}
			m.syncStatusBarWithActiveTab()
		}
		return nil
	case "tab_close_this":
		if m.ContextMenuTabIdx >= 0 {
			m.attemptCloseTab(m.ContextMenuTabIdx)
		}
		return nil

	case "tab_close_others":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx < 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.blockBatchCloseIfRunning("others", targetIdx) {
			return nil
		}
		if m.AutoSave {
			for i, t := range m.TabManager.Tabs {
				if i != targetIdx && t.IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseOtherTabs(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i, t := range m.TabManager.Tabs {
				if i != targetIdx && t.IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Save or discard changes before closing other tabs", true, 3*time.Second)
			} else {
				m.TabManager.CloseOtherTabs(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_right":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx < 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.blockBatchCloseIfRunning("right", targetIdx) {
			return nil
		}
		if m.AutoSave {
			for i := targetIdx + 1; i < m.TabManager.Count(); i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseTabsToRight(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i := targetIdx + 1; i < m.TabManager.Count(); i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Save or discard changes before closing tabs to the right", true, 3*time.Second)
			} else {
				m.TabManager.CloseTabsToRight(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_left":
		targetIdx := m.ContextMenuTabIdx
		if targetIdx <= 0 || targetIdx >= m.TabManager.Count() {
			return nil
		}
		if m.blockBatchCloseIfRunning("left", targetIdx) {
			return nil
		}
		if m.AutoSave {
			for i := 0; i < targetIdx; i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseTabsToLeft(targetIdx)
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i := 0; i < targetIdx; i++ {
				if m.TabManager.Tabs[i].IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Save or discard changes before closing tabs to the left", true, 3*time.Second)
			} else {
				m.TabManager.CloseTabsToLeft(targetIdx)
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil

	case "tab_close_all":
		if m.blockBatchCloseIfRunning("all", -1) {
			return nil
		}
		if m.AutoSave {
			for i, t := range m.TabManager.Tabs {
				if t.IsDirty() {
					_ = m.TabManager.SaveTab(i)
				}
			}
			m.TabManager.CloseAllTabs()
			m.syncStatusBarWithActiveTab()
		} else {
			firstDirty := -1
			for i, t := range m.TabManager.Tabs {
				if t.IsDirty() {
					firstDirty = i
					break
				}
			}
			if firstDirty != -1 {
				m.attemptCloseTab(firstDirty)
				m.StatusBar.ShowToast("Save or discard changes before closing all tabs", true, 3*time.Second)
			} else {
				m.TabManager.CloseAllTabs()
				m.syncStatusBarWithActiveTab()
			}
		}
		return nil
	case "explorer_new_file":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFile, "New File", parentDir, "")
		return nil

	case "explorer_new_folder":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFolder, "New Folder", parentDir, "")
		return nil

	case "explorer_rename":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.InputDialog.Open(dialog.InputRename, "Rename: "+node.Name, node.Path, node.Name)
		}
		return nil

	case "explorer_delete":
		if node := m.Explorer.SelectedNode(); node != nil {
			m.ConfirmDialog.Open(
				dialog.ConfirmDelete,
				"Confirm Delete",
				"Are you sure you want to delete:\n"+node.Path+"?",
				node.Path,
			)
		}
		return nil

	case "explorer_copy_path":
		if node := m.Explorer.SelectedNode(); node != nil {
			if rel, err := filepath.Rel(m.RootDir, node.Path); err == nil {
				_ = clipboard.WriteAll(rel)
				m.StatusBar.ShowToast("Path disalin: "+rel, false, 2*time.Second)
			}
		}
		return nil

	case "explorer_clear_filter":
		m.Explorer.SetFilter("")
		m.StatusBar.ShowToast("Filter file dibersihkan", false, 2*time.Second)
		return nil

	}

	tab := m.TabManager.ActiveTab()
	if tab == nil {
		return nil
	}

	if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
		switch action {
		case "copy":
			text := tab.MarkdownViewer.GetSelectedText()
			if text == "" {
				text = tab.MarkdownViewer.GetAllCleanText()
			}
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
		case "select_all":
			tab.MarkdownViewer.SelectAll()
		case "paste":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" && tab.Buffer != nil {
				tab.Buffer.InsertString(text)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Ditempel ke dokumen markdown", false, 2*time.Second)
			}
		}
		return nil
	}

	if tab != nil && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
		switch action {
		case "copy":
			text := tab.TerminalView.SelectedText()
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
			}
		case "paste":
			text, err := clipboard.ReadAll()
			if err == nil && text != "" {
				_, _ = tab.TerminalView.Write([]byte(text))
				m.StatusBar.ShowToast("Ditempel ke terminal", false, 2*time.Second)
			}
		case "select_all":
			tab.TerminalView.SelectAll()
			m.StatusBar.ShowToast("Seluruh layar terminal dipilih", false, 2*time.Second)
		}
		return nil
	}

	if tab.Buffer == nil {
		return nil
	}
	buf := tab.Buffer
	switch action {
	case "copy":
		text := buf.GetSelectedText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
		}
	case "cut":
		text := buf.GetSelectedText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			buf.DeleteSelection()
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
		}
	case "paste":
		text, err := clipboard.ReadAll()
		if err == nil && text != "" {
			buf.InsertString(text)
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
		}
	case "select_all":
		buf.SelectAll()
		m.syncStatusBarWithActiveTab()
	}
	return nil
}
