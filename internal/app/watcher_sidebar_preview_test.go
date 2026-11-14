package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/editor"
)

func TestFileWatcherAutoReloadAndConflictGuard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-watch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "watched.txt")
	_ = os.WriteFile(filePath, []byte("versi awal"), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(filePath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("Expected active tab with buffer")
	}

	// 1. External modification while buffer is clean -> Auto Reload
	time.Sleep(10 * time.Millisecond) // ensure modtime difference
	_ = os.WriteFile(filePath, []byte("versi kedua dari luar"), 0644)

	// Trigger file watch tick
	newM, _ := m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if tab.Buffer.Text() != "versi kedua dari luar" {
		t.Errorf("Expected buffer to be reloaded from disk, got: %q", tab.Buffer.Text())
	}
	if tab.Buffer.IsDirty {
		t.Errorf("Expected buffer to remain clean after auto reload")
	}

	// 2. Local edit (dirty) + External modification -> Conflict protection
	tab.Buffer.InsertString(" - tambahan lokal")
	if !tab.Buffer.IsDirty {
		t.Fatalf("Expected buffer to be dirty after local edit")
	}

	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(filePath, []byte("versi ketiga dari luar"), 0644)

	// Trigger file watch tick
	newM, _ = m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	// Local buffer must NOT be overwritten!
	if !strings.Contains(tab.Buffer.Text(), "tambahan lokal") {
		t.Errorf("Expected dirty buffer to be protected from external overwrite, got: %q", tab.Buffer.Text())
	}
	if tab.Buffer.Text() == "versi ketiga dari luar" {
		t.Errorf("Dirty buffer was erroneously overwritten by external file content")
	}
	if !m.ConfirmDialog.Active {
		t.Errorf("Expected ConfirmDialog to be activated on external conflict")
	}
	m.ConfirmDialog.Close()

	// 3. External file deletion -> Warning toast, tab modtime reset
	_ = os.Remove(filePath)
	newM, _ = m.Update(FileWatchTickMsg(time.Now()))
	m = newM.(Model)

	if !tab.LastModTime.IsZero() {
		t.Errorf("Expected LastModTime to be zeroed after file deletion detection")
	}
}

func TestAdjustableSidebarWidth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-sidebar-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30

	initialWidth := m.SidebarWidth
	if initialWidth <= 0 {
		initialWidth = 30
		m.SidebarWidth = 30
	}

	// 1. Keyboard Alt+] to widen
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
	m = newM.(Model)
	if m.SidebarWidth != initialWidth+2 {
		t.Errorf("Expected sidebar width to increase to %d, got %d", initialWidth+2, m.SidebarWidth)
	}

	// 2. Keyboard Alt+[ to narrow
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}, Alt: true})
	m = newM.(Model)
	if m.SidebarWidth != initialWidth {
		t.Errorf("Expected sidebar width to return to %d, got %d", initialWidth, m.SidebarWidth)
	}

	// 3. Minimum width clamping (min 15)
	for i := 0; i < 30; i++ {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}, Alt: true})
		m = newM.(Model)
	}
	if m.SidebarWidth != 15 {
		t.Errorf("Expected sidebar width clamped at 15, got %d", m.SidebarWidth)
	}

	// 4. Maximum width clamping (max Width - 20 = 80)
	for i := 0; i < 50; i++ {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
		m = newM.(Model)
	}
	if m.SidebarWidth != 80 {
		t.Errorf("Expected sidebar width clamped at 80, got %d", m.SidebarWidth)
	}

	// 5. Mouse Drag on border
	m.SidebarWidth = 30
	borderX := m.SidebarWidth - 1 // 29

	// Click on border to start resizing
	newM, _ = m.Update(tea.MouseMsg{
		X:      borderX,
		Y:      5,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = newM.(Model)
	if !m.ResizingSidebar {
		t.Errorf("Expected ResizingSidebar to be true on border press")
	}

	// Drag mouse to column 42
	newM, _ = m.Update(tea.MouseMsg{
		X:      42,
		Y:      5,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	})
	m = newM.(Model)
	if m.SidebarWidth != 42 {
		t.Errorf("Expected sidebar width to be 42 after drag, got %d", m.SidebarWidth)
	}

	// Release mouse
	newM, _ = m.Update(tea.MouseMsg{
		X:      42,
		Y:      5,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	})
	m = newM.(Model)
	if m.ResizingSidebar {
		t.Errorf("Expected ResizingSidebar to be false after release")
	}

	// 6. Config persistence check
	if m.Config == nil || m.Config.SidebarWidth != 42 {
		t.Errorf("Expected Config.SidebarWidth to be 42, got %+v", m.Config)
	}
}

func TestMarkdownPreviewCopyAndPaste(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-md-copy-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := filepath.Join(tempDir, "doc.md")
	mdContent := "# Judul Preview\n\nParagraf pertama disini.\n\n```go\nfunc Hello() {}\n```\n"
	_ = os.WriteFile(mdPath, []byte(mdContent), 0644)

	m, err := NewModelWithPath(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.openFile(mdPath)

	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Type != editor.TabMarkdown {
		t.Fatalf("Expected active tab to be markdown tab")
	}

	// Switch to preview mode
	tab.MarkdownPreview = true
	m.Focus = FocusEditor

	// Render once to populate cachedLines in viewer
	_ = m.View()

	if tab.MarkdownViewer == nil {
		t.Fatalf("Expected tab.MarkdownViewer to be initialized")
	}

	// 1. Test Select All via Ctrl+A in Preview
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = newM.(Model)
	if !tab.MarkdownViewer.HasSelection {
		t.Errorf("Expected HasSelection to be true after Ctrl+A")
	}

	// 2. Test Copy via Ctrl+C in Preview
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = newM.(Model)

	clipText, err := clipboard.ReadAll()
	if err == nil {
		if !strings.Contains(clipText, "Judul Preview") {
			t.Errorf("Expected clipboard to contain 'Judul Preview', got %q", clipText)
		}
	}

	// 3. Test Escape clears selection
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(Model)
	if tab.MarkdownViewer.HasSelection {
		t.Errorf("Expected HasSelection to be false after Escape")
	}

	// 4. Test Mouse Drag selection in Preview
	editorStartX := m.SidebarWidth
	// Click inside preview area
	newM, _ = m.Update(tea.MouseMsg{
		X:      editorStartX + 5,
		Y:      2,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = newM.(Model)
	if !tab.MarkdownViewer.HasSelection {
		t.Errorf("Expected HasSelection to be true after mouse press in preview")
	}

	// Drag inside preview area
	newM, _ = m.Update(tea.MouseMsg{
		X:      editorStartX + 20,
		Y:      4,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	})
	m = newM.(Model)
	selText := tab.MarkdownViewer.GetSelectedText()
	if len(selText) == 0 {
		t.Errorf("Expected non-empty selected text after mouse drag in preview")
	}

	// 5. Test Paste action (Ctrl+V) in Preview
	_ = clipboard.WriteAll("\n\n## Tambahan dari Paste")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	m = newM.(Model)

	if !strings.Contains(tab.Buffer.Text(), "Tambahan dari Paste") {
		t.Errorf("Expected tab buffer to contain pasted text, got: %q", tab.Buffer.Text())
	}

	// 6. Test Context Menu actions on preview
	_ = m.handleContextMenuAction("select_all")
	if !tab.MarkdownViewer.HasSelection {
		t.Errorf("Expected selection after context menu select_all")
	}

	_ = m.handleContextMenuAction("copy")
	// Verify toast or clipboard
	if !strings.Contains(m.StatusBar.ToastMsg, "Disalin") {
		t.Errorf("Expected toast notification for copy, got %q", m.StatusBar.ToastMsg)
	}
}

func TestExternalChangeConflictDialogOptions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-conflict-dialog-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "conflict.txt")
	_ = os.WriteFile(filePath, []byte("isi awal disk"), 0644)

	// Subtest 1: User selects Merge (Split Diff)
	{
		m, err := NewModelWithPath(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to create model: %v", err)
		}
		m.Ready = true
		m.Width = 100
		m.Height = 30
		m.openFile(filePath)

		tab := m.TabManager.ActiveTab()
		tab.Buffer.InsertString(" + edit lokal")

		time.Sleep(10 * time.Millisecond)
		_ = os.WriteFile(filePath, []byte("isi diubah disk luar"), 0644)

		newM, _ := m.Update(FileWatchTickMsg(time.Now()))
		m = newM.(Model)

		if !m.ConfirmDialog.Active {
			t.Fatalf("Expected confirm dialog to be active")
		}

		// Navigate to button 1: Merge (Diff)
		m.ConfirmDialog.Selected = 1
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newM.(Model)

		activeTab := m.TabManager.ActiveTab()
		if activeTab == nil || activeTab.Type != editor.TabDiff {
			t.Errorf("Expected active tab to be Split Diff after choosing Merge, got %v", activeTab)
		}
	}

	// Subtest 2: User selects Reload
	{
		m, err := NewModelWithPath(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to create model: %v", err)
		}
		m.Ready = true
		m.Width = 100
		m.Height = 30
		m.openFile(filePath)

		tab := m.TabManager.ActiveTab()
		tab.Buffer.InsertString(" + edit lokal lain")

		time.Sleep(10 * time.Millisecond)
		_ = os.WriteFile(filePath, []byte("isi disk versi akhir"), 0644)

		newM, _ := m.Update(FileWatchTickMsg(time.Now()))
		m = newM.(Model)

		if !m.ConfirmDialog.Active {
			t.Fatalf("Expected confirm dialog to be active")
		}

		// Select button 0: Reload
		m.ConfirmDialog.Selected = 0
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newM.(Model)

		tab = m.TabManager.ActiveTab()
		if tab.Buffer.Text() != "isi disk versi akhir" {
			t.Errorf("Expected buffer to be reloaded with disk content, got: %q", tab.Buffer.Text())
		}
		if tab.Buffer.IsDirty {
			t.Errorf("Expected buffer to be clean after Reload")
		}
	}

	// Subtest 3: User cancels (Keep Local)
	{
		m, err := NewModelWithPath(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to create model: %v", err)
		}
		m.Ready = true
		m.Width = 100
		m.Height = 30
		m.openFile(filePath)

		tab := m.TabManager.ActiveTab()
		tab.Buffer.InsertString(" + edit lokal saya")

		time.Sleep(10 * time.Millisecond)
		_ = os.WriteFile(filePath, []byte("external changes again"), 0644)

		newM, _ := m.Update(FileWatchTickMsg(time.Now()))
		m = newM.(Model)

		if !m.ConfirmDialog.Active {
			t.Fatalf("Expected confirm dialog to be active")
		}

		// Select button 2: Cancel / Keep Local
		m.ConfirmDialog.Selected = 2
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newM.(Model)

		tab = m.TabManager.ActiveTab()
		if !strings.Contains(tab.Buffer.Text(), "edit lokal saya") {
			t.Errorf("Expected buffer to keep local edits on cancel, got: %q", tab.Buffer.Text())
		}
		if !strings.Contains(m.StatusBar.ToastMsg, "External changes ignored") {
			t.Errorf("Expected toast notification for cancel/keep local, got %q", m.StatusBar.ToastMsg)
		}
	}
}
