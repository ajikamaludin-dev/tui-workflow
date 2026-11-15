package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui/dialog"
)

// checkWatchedFiles scans all open tabs for external on-disk changes and
// reloads clean buffers (or raises a conflict dialog for dirty ones).
// Change detection is content-hash based (SHA-256), so writes that preserve
// mtime are still detected, and transient read failures never lose a change.
func (m *Model) checkWatchedFiles() {
	if m.TabManager == nil {
		return
	}
	for _, tab := range m.TabManager.Tabs {
		m.checkWatchedTab(tab)
	}
}

func (m *Model) checkWatchedTab(tab *editor.Tab) {
	if tab == nil || tab.Path == "" || tab.Type == editor.TabDiff || tab.Buffer == nil {
		return
	}

	data, err := os.ReadFile(tab.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if !tab.LastModTime.IsZero() || tab.WatchedHash != "" {
				m.StatusBar.ShowToast(fmt.Sprintf("File '%s' was deleted from disk", tab.Name), true, 3*time.Second)
			}
			tab.LastModTime = time.Time{}
			tab.WatchedHash = ""
			tab.WatchedSize = 0
			return
		}
		// Transient read error (e.g. file mid-rename): keep the baseline untouched
		// so the next tick retries instead of losing the change silently.
		return
	}

	hash := editor.HashContent(data)
	if tab.WatchedHash != "" && tab.WatchedHash == hash {
		tab.WatchedSize = int64(len(data))
		return // content unchanged
	}

	// Content changed on disk (or baseline unknown after deletion).
	if !tab.Buffer.IsDirty {
		if err := tab.ReloadFromDisk(); err == nil {
			tab.WatchedHash = hash
			tab.WatchedSize = int64(len(data))
			m.StatusBar.ShowToast(fmt.Sprintf("File '%s' diperbarui dari disk", tab.Name), false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast(fmt.Sprintf("Gagal reload '%s': %v", tab.Name, err), true, 3*time.Second)
		}
		return
	}

	// Dirty buffer: keep local changes, warn once per change (baseline advances
	// so the conflict dialog does not reopen on every tick).
	tab.WatchedHash = hash
	tab.WatchedSize = int64(len(data))
	if info, errStat := os.Stat(tab.Path); errStat == nil {
		tab.LastModTime = info.ModTime()
	}
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

// refreshWatchedBaselineForPath re-anchors the content baseline of the open
// tab backed by path after the application wrote newContent itself, so the
// watcher does not misread our own writes as an external change.
func (m *Model) refreshWatchedBaselineForPath(path string, newContent []byte) {
	if m.TabManager == nil {
		return
	}
	clean := filepath.Clean(path)
	for _, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Path == "" {
			continue
		}
		if filepath.Clean(tab.Path) != clean {
			continue
		}
		tab.WatchedHash = editor.HashContent(newContent)
		tab.WatchedSize = int64(len(newContent))
		if info, err := os.Stat(path); err == nil {
			tab.LastModTime = info.ModTime()
		}
		return
	}
}
