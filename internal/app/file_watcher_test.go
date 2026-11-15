package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"tui-workflow/internal/editor"
)

func TestWatcherDetectsPreservedMtimeExternalWrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-hash-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "notes.md")
	original := []byte("# JUDUL_ASLI_X\n")
	_ = os.WriteFile(filePath, original, 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("expected active tab with buffer")
	}
	if tab.WatchedHash != editor.HashContent(original) {
		t.Fatalf("expected baseline hash set at open")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	oldMtime := info.ModTime()

	// External agent rewrites content but preserves mtime (e.g. cp -p style)
	_ = os.WriteFile(filePath, []byte("# JUDUL_BERUBAH_X\n"), 0644)
	_ = os.Chtimes(filePath, oldMtime, oldMtime)

	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if !strings.Contains(tab.Buffer.Text(), "JUDUL_BERUBAH_X") {
		t.Errorf("expected buffer reloaded despite preserved mtime, got: %q", tab.Buffer.Text())
	}
	if m.ConfirmDialog != nil && m.ConfirmDialog.Active {
		t.Errorf("conflict dialog should not open for a clean buffer")
	}
}

func TestWatcherDeleteRecreateWithPreservedMtimeReloads(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-hash-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "plan.md")
	_ = os.WriteFile(filePath, []byte("# RENCANA_LAMA\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("expected active tab with buffer")
	}

	// Establish baseline
	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	oldMtime := info.ModTime()

	// Delete, then recreate with the same mtime and different content
	_ = os.Remove(filePath)
	newM, _ = m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)
	if tab.WatchedHash != "" {
		t.Errorf("expected baseline cleared after delete detection")
	}

	_ = os.WriteFile(filePath, []byte("# RENCANA_BARU\n"), 0644)
	_ = os.Chtimes(filePath, oldMtime, oldMtime)
	newM, _ = m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if !strings.Contains(tab.Buffer.Text(), "RENCANA_BARU") {
		t.Errorf("expected buffer reloaded after delete+recreate, got: %q", tab.Buffer.Text())
	}
}

func TestWatcherSaveDoesNotSelfTriggerConflict(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-hash-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "autosave.txt")
	_ = os.WriteFile(filePath, []byte("baris satu\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("expected active tab with buffer")
	}

	tab.Buffer.InsertString(" tambahan lokal")
	if err := m.TabManager.SaveActive(); err != nil {
		t.Fatalf("save: %v", err)
	}

	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if m.ConfirmDialog != nil && m.ConfirmDialog.Active {
		t.Errorf("editor's own save must not be reported as an external change")
	}
	disk, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(disk), "tambahan lokal") {
		t.Errorf("expected saved content on disk, got: %q", string(disk))
	}
}

func TestWatcherMarkdownPreviewRefreshesOnExternalChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-md-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := filepath.Join(tempDir, "preview.md")
	_ = os.WriteFile(mdPath, []byte("# PRATINJAU_LAMA_MD\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(mdPath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Type != editor.TabMarkdown {
		t.Fatalf("expected markdown tab, got type %v", tab.Type)
	}
	tab.MarkdownPreview = true
	m.Focus = FocusEditor
	_ = m.View()

	info, err := os.Stat(mdPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	oldMtime := info.ModTime()
	_ = os.WriteFile(mdPath, []byte("# PRATINJAU_BARU_MD\n"), 0644)
	_ = os.Chtimes(mdPath, oldMtime, oldMtime) // preserve mtime

	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if !strings.Contains(tab.Buffer.Text(), "PRATINJAU_BARU_MD") {
		t.Fatalf("expected buffer reloaded, got: %q", tab.Buffer.Text())
	}
	rendered := tab.MarkdownViewer.Render(tab.Buffer.Text(), 100, 29)
	if !strings.Contains(ansi.Strip(rendered), "PRATINJAU_BARU_MD") {
		t.Errorf("expected preview render to contain updated content, got:\n%s", ansi.Strip(rendered))
	}
}

func TestWatcherDirtyConflictGuardWithPreservedMtime(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-hash-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "conflict.txt")
	_ = os.WriteFile(filePath, []byte("konten awal\n"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("expected active tab with buffer")
	}

	// Local edit -> dirty
	tab.Buffer.InsertString(" - suntingan lokal")

	// External change with preserved mtime
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	oldMtime := info.ModTime()
	_ = os.WriteFile(filePath, []byte("konten eksternal\n"), 0644)
	_ = os.Chtimes(filePath, oldMtime, oldMtime)

	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if !strings.Contains(tab.Buffer.Text(), "suntingan lokal") {
		t.Errorf("dirty buffer must be protected from external overwrite, got: %q", tab.Buffer.Text())
	}
	if m.ConfirmDialog == nil || !m.ConfirmDialog.Active {
		t.Errorf("expected external-change conflict dialog for dirty buffer")
	}
}
