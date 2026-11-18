package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tui-workflow/internal/editor"
	"tui-workflow/internal/ui/dialog"
)

// checkWatchedFiles scans all open tabs for external on-disk changes and for
// backing files that disappeared. A clean tab whose file is gone is closed
// automatically; a dirty one raises a Save/Discard/Cancel dialog so unsaved
// work is never silently lost. Pinned tabs are kept open with a warning.
// Change detection is content-hash based (SHA-256), so writes that preserve
// mtime are still detected, and transient read failures never lose a change.
func (m *Model) checkWatchedFiles() {
	if m.TabManager == nil {
		return
	}

	var toClose []int
	var dirtyMissing *editor.Tab
	var dirtyMissingName string

	for i, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Path == "" || tab.Type == editor.TabDiff || tab.Buffer == nil {
			continue
		}

		data, err := os.ReadFile(tab.Path)
		if err != nil {
			if !os.IsNotExist(err) {
				// Transient read error (e.g. file mid-rename): keep the
				// baseline untouched so the next tick retries.
				continue
			}
			// Backing file is gone from disk.
			hadBaseline := !tab.LastModTime.IsZero() || tab.WatchedHash != ""
			tab.LastModTime = time.Time{}
			tab.WatchedHash = ""
			tab.WatchedSize = 0

			if tab.IsPinned {
				if hadBaseline {
					m.StatusBar.ShowToast(fmt.Sprintf("File '%s' was deleted (pinned tab kept open)", tab.Name), true, 3*time.Second)
				}
				continue
			}
			if tab.IsDirty() {
				// Only one dialog at a time; other dirty-missing tabs stay
				// unflagged and are offered on a later tick.
				if !tab.DeletedFromDisk && dirtyMissing == nil && m.ConfirmDialog != nil && !m.ConfirmDialog.Active {
					dirtyMissing = tab
					dirtyMissingName = tab.Name
				}
			} else {
				toClose = append(toClose, i)
			}
			continue
		}

		// File is present again after a disappearance: clear the one-shot flag
		// so a future deletion can prompt again.
		if tab.DeletedFromDisk {
			tab.DeletedFromDisk = false
		}

		hash := editor.HashContent(data)
		if tab.WatchedHash != "" && tab.WatchedHash == hash {
			tab.WatchedSize = int64(len(data))
			continue // content unchanged
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
			continue
		}

		// Dirty buffer: keep local changes, warn once per change (baseline
		// advances so the conflict dialog does not reopen on every tick).
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

	// Close clean tabs whose file is gone. Highest index first so earlier
	// indices stay valid as the slice shrinks.
	for i := len(toClose) - 1; i >= 0; i-- {
		idx := toClose[i]
		if idx < 0 || idx >= len(m.TabManager.Tabs) {
			continue
		}
		name := m.TabManager.Tabs[idx].Name
		m.TabManager.CloseTab(idx)
		m.StatusBar.ShowToast(fmt.Sprintf("Tab '%s' closed (file no longer exists)", name), false, 3*time.Second)
	}
	if len(toClose) > 0 {
		m.syncStatusBarWithActiveTab()
	}

	// Offer Save/Discard/Cancel for one dirty tab whose file disappeared.
	// Recompute its index after clean tabs were removed above.
	if dirtyMissing != nil {
		for i, tab := range m.TabManager.Tabs {
			if tab == dirtyMissing {
				dirtyMissing.DeletedFromDisk = true
				m.ClosingTabIdx = i
				m.ConfirmDialog.Open(
					dialog.ConfirmUnsavedChanges,
					"File Deleted — Save Changes?",
					fmt.Sprintf("File '%s' was deleted from disk but has unsaved changes.\nSave it, discard changes, or keep the tab open?", dirtyMissingName),
					dirtyMissing.Path,
				)
				break
			}
		}
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
