package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui/dialog"
)

func setupTestGitRepoWithBranch(t *testing.T) (string, *git.Repo) {
	tempDir, err := os.MkdirTemp("", "tui-feature-git-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	repo, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	initFile := filepath.Join(tempDir, "initial.txt")
	_ = os.WriteFile(initFile, []byte("line1\n"), 0644)
	_, _ = wt.Add("initial.txt")
	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	gRepo, err := git.OpenRepo(tempDir)
	if err != nil {
		t.Fatalf("Failed to open git repo wrapper: %v", err)
	}

	return tempDir, gRepo
}

func TestBranchInStatusBarAndUntrackedOpen(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	m, err := NewModelWithPath(repoDir)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	// 1. Verify branch name in status bar
	if m.StatusBar.Branch == "" {
		t.Errorf("Expected branch name in status bar, got empty string")
	}

	// 2. Create an untracked new file
	untrackedPath := filepath.Join(repoDir, "newfile.go")
	_ = os.WriteFile(untrackedPath, []byte("package main\n"), 0644)

	m.SidebarView = SidebarGit
	m.GitView.Refresh()

	// Find the untracked file in Changes
	foundUntracked := false
	for _, ch := range m.GitView.Changes {
		if ch.Path == "newfile.go" && (ch.Status == "?" || ch.Status == "U") {
			foundUntracked = true
			break
		}
	}
	if !foundUntracked {
		t.Fatalf("Untracked newfile.go not found in GitView changes")
	}

	// Simulate selecting and pressing Enter on untracked file
	// Focus on GitView changes
	m.Focus = FocusSidebar
	m.GitView.FocusArea = git.GitFocusChanges
	m.GitView.ChangesIdx = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	// Untracked file should be opened DIRECTLY as editor tab, NOT diff tab
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil {
		t.Fatalf("Expected active tab, got nil")
	}
	if activeTab.IsDiff {
		t.Errorf("Untracked file should be opened directly, not as diff tab")
	}
	if activeTab.Path != untrackedPath {
		t.Errorf("Expected active tab path %s, got %s", untrackedPath, activeTab.Path)
	}
}

func TestOpenFileAndFolderDialogs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-feature-open-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "subdir")
	_ = os.Mkdir(subDir, 0755)
	fileA := filepath.Join(tempDir, "fileA.txt")
	_ = os.WriteFile(fileA, []byte("content A"), 0644)

	m, err := NewModelWithPath(tempDir)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30

	// 1. Test Open File via shortcut Ctrl+O -> Opens FilePicker in PickerModeFile
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(Model)

	if !m.FilePicker.Active || m.FilePicker.Mode != dialog.PickerModeFile {
		t.Fatalf("Expected FilePicker active in PickerModeFile")
	}

	// Find index of fileA in picker.Filtered
	fileAIdx := -1
	for i, it := range m.FilePicker.Filtered {
		if it.Path == fileA {
			fileAIdx = i
			break
		}
	}
	if fileAIdx == -1 {
		t.Fatalf("fileA.txt not found in FilePicker filtered items: %v", m.FilePicker.Filtered)
	}
	m.FilePicker.SelectedIdx = fileAIdx
	m.FilePicker.FocusArea = 1

	// Simulate Enter on selected file item
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	if m.FilePicker.Active {
		t.Errorf("FilePicker should be closed after Enter on file")
	}
	if m.TabManager.ActiveTab() == nil || m.TabManager.ActiveTab().Path != fileA {
		t.Errorf("Expected active tab to be fileA, got: %v", m.TabManager.ActiveTab())
	}

	// 2. Test Open Folder via menu action "file.open_folder" -> Opens FilePicker in PickerModeFolder
	_ = m.handleMenuAction("file.open_folder")
	if !m.FilePicker.Active || m.FilePicker.Mode != dialog.PickerModeFolder {
		t.Fatalf("Expected FilePicker active in PickerModeFolder")
	}

	// Navigate into subdir and confirm folder
	m.FilePicker.CurrentDir = subDir
	// Simulate click on "Buka Folder Ini" button (mouse test)
	modalW := m.FilePicker.Width
	modalH := m.FilePicker.Height
	startX := (m.Width - modalW) / 2
	startY := (m.Height - modalH) / 2
	btnY := startY + modalH - 3
	btnX := startX + 6

	newM, _ = m.Update(tea.MouseMsg{
		X:      btnX,
		Y:      btnY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if m.FilePicker.Active {
		t.Errorf("FilePicker should be closed after clicking Buka Folder Ini")
	}
	if m.RootDir != subDir {
		t.Errorf("Expected root dir to be %s, got %s", subDir, m.RootDir)
	}
	if m.Explorer.RootPath != subDir {
		t.Errorf("Expected Explorer RootPath to be %s, got %s", subDir, m.Explorer.RootPath)
	}
}

func TestModalMouseClicks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-feature-mouse-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30

	// 1. InputDialog Mouse Click Cancel
	m.InputDialog.Open(dialog.InputNewFile, "New File", tempDir, "")
	if !m.InputDialog.Active {
		t.Fatalf("Expected InputDialog active")
	}
	modalW := m.InputDialog.Width
	modalH := strings.Count(m.InputDialog.Render(), "\n") + 1
	startX := (m.Width - modalW) / 2
	startY := (m.Height - modalH) / 2
	clickX := startX + 25
	clickY := startY + modalH - 3
	newM, _ := m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      clickY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)
	if m.InputDialog.Active {
		t.Errorf("InputDialog should be closed after clicking Cancel")
	}

	// 2. ConfirmDialog Mouse Click Cancel
	m.ConfirmDialog.Open(dialog.ConfirmDelete, "Hapus File", "Yakin hapus?", "sample.txt")
	if !m.ConfirmDialog.Active {
		t.Fatalf("Expected ConfirmDialog active")
	}
	modalW = m.ConfirmDialog.Width
	modalH = strings.Count(m.ConfirmDialog.Render(), "\n") + 1
	startX = (m.Width - modalW) / 2
	startY = (m.Height - modalH) / 2
	// Click Cancel button (buttonY = boxH - 3, relX ~ 20)
	newM, _ = m.Update(tea.MouseMsg{
		X:      startX + 20,
		Y:      startY + modalH - 3,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)
	if m.ConfirmDialog.Active {
		t.Errorf("ConfirmDialog should be closed after clicking Cancel")
	}
}

func TestSettingsPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-feature-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m, err := NewModelWithPath(tempDir)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	// Toggle AutoSave
	initialAutoSave := m.AutoSave
	_ = m.handleMenuAction("setting.autosave")
	if m.AutoSave == initialAutoSave {
		t.Errorf("AutoSave should have toggled")
	}
	if m.Config.AutoSave != m.AutoSave {
		t.Errorf("Config.AutoSave should be updated to %v", m.AutoSave)
	}

	// Toggle Hidden
	initialHidden := m.Explorer.ShowHidden
	_ = m.handleMenuAction("setting.hidden")
	if m.Explorer.ShowHidden == initialHidden {
		t.Errorf("ShowHidden should have toggled")
	}
	if m.Config.ShowHidden != m.Explorer.ShowHidden {
		t.Errorf("Config.ShowHidden should be updated to %v", m.Explorer.ShowHidden)
	}
}

func TestGitCommitAndPushButtonInteraction(t *testing.T) {
	repoDir, _ := setupTestGitRepoWithBranch(t)
	defer os.RemoveAll(repoDir)

	m, err := NewModelWithPath(repoDir)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.SidebarView = SidebarGit
	m.GitView.Refresh()

	// Modify file and stage it
	filePath := filepath.Join(repoDir, "initial.txt")
	_ = os.WriteFile(filePath, []byte("line1\nline2\n"), 0644)
	m.GitView.Refresh()
	_ = m.GitRepo.StageFile("initial.txt")
	m.GitView.Refresh()

	// Set commit message
	m.GitView.CommitInput.SetValue("test commit via button")

	// Render GitView so ItemClickMap is populated
	_ = m.GitView.Render(30, 20, true)

	// Find the button row
	var buttonY int = -1
	for y, tgt := range m.GitView.ItemClickMap {
		if tgt.Type == git.ClickButtonRow {
			buttonY = y
			break
		}
	}
	if buttonY == -1 {
		t.Fatalf("ClickButtonRow not found")
	}

	// Click [Commit] (X = 4)
	// Click Y is relative to sidebar content row (relLine = msg.Y - 3)
	screenY := buttonY + 3
	newM, cmd := m.Update(tea.MouseMsg{
		X:      4,
		Y:      screenY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if !m.GitView.IsLoading || m.GitView.LoadingMsg != "Committing..." {
		t.Errorf("Expected IsLoading=true with Committing..., got loading=%v msg=%q", m.GitView.IsLoading, m.GitView.LoadingMsg)
	}

	if cmd != nil {
		resMsg := cmd()
		if batch, ok := resMsg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd != nil {
					subMsg := subCmd()
					newM, _ = m.Update(subMsg)
					m = newM.(Model)
				}
			}
		} else {
			newM, _ = m.Update(resMsg)
			m = newM.(Model)
		}
	}

	if !strings.Contains(m.StatusBar.ToastMsg, "Commit sukses") {
		t.Errorf("Expected toast commit sukses, got %q", m.StatusBar.ToastMsg)
	}
	if m.GitView.IsLoading {
		t.Errorf("Expected IsLoading=false after commit finished")
	}

	// Working tree is now clean, unpushed commits exist -> re-render to switch button to Push
	_ = m.GitView.Render(30, 20, true)
	buttonY = -1
	for y, tgt := range m.GitView.ItemClickMap {
		if tgt.Type == git.ClickButtonRow {
			buttonY = y
			break
		}
	}
	if buttonY == -1 {
		t.Fatalf("ClickButtonRow not found after commit")
	}
	screenY = buttonY + 3

	// Click [Push]
	newM, cmd = m.Update(tea.MouseMsg{
		X:      4,
		Y:      screenY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if !m.GitView.IsLoading || m.GitView.LoadingMsg != "Pushing..." {
		t.Errorf("Expected IsLoading=true with Pushing..., got loading=%v msg=%q", m.GitView.IsLoading, m.GitView.LoadingMsg)
	}

	if cmd != nil {
		resMsg := cmd()
		if batch, ok := resMsg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd != nil {
					subMsg := subCmd()
					newM, _ = m.Update(subMsg)
					m = newM.(Model)
				}
			}
		} else {
			newM, _ = m.Update(resMsg)
			m = newM.(Model)
		}
	}

	if m.GitView.IsLoading {
		t.Errorf("Expected IsLoading=false after push finished")
	}

	// Since there's no remote configured, it should show push error toast gracefully without crash
	if !strings.Contains(m.StatusBar.ToastMsg, "Push gagal") && !strings.Contains(m.StatusBar.ToastMsg, "Push berhasil") {
		t.Errorf("Expected push toast response, got %q", m.StatusBar.ToastMsg)
	}
}

func TestContextMenuAndSelection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("Hello World!\nSecond Line Here"), 0644)

	m, err := NewModelWithPath(tmpDir)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	m.openFile(filePath)
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatalf("failed to open buffer")
	}

	// 1. Mouse Drag Selection in Editor
	// Editor starts at X = m.SidebarWidth
	// Gutter is ~6 columns, so editor text starts around X = m.SidebarWidth + 6
	// Y = 2 is line 0 of buffer
	textStartX := m.SidebarWidth + 6
	newM, _ = m.Update(tea.MouseMsg{
		X:      textStartX,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	newM, _ = m.Update(tea.MouseMsg{
		X:      textStartX + 5,
		Y:      2,
		Action: tea.MouseActionMotion,
	})
	m = newM.(Model)

	newM, _ = m.Update(tea.MouseMsg{
		X:      textStartX + 5,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	})
	m = newM.(Model)

	tab = m.TabManager.ActiveTab()
	if !tab.Buffer.HasSelection {
		t.Errorf("expected buffer to have selection after mouse drag")
	}

	// 2. Right click to open ContextMenu
	newM, _ = m.Update(tea.MouseMsg{
		X:      textStartX + 6,
		Y:      5,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)

	if m.ContextMenu == nil || !m.ContextMenu.Active {
		t.Fatalf("expected ContextMenu to be active after right click")
	}

	// Right-click release at same position must NOT close menu
	newM, _ = m.Update(tea.MouseMsg{
		X:      40,
		Y:      5,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionRelease,
	})
	m = newM.(Model)
	if !m.ContextMenu.Active {
		t.Fatalf("expected ContextMenu to stay active after right-click release")
	}

	// 3. Esc closes ContextMenu
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)

	if m.ContextMenu.Active {
		t.Errorf("expected ContextMenu to close on Esc")
	}

	// 4. Ctrl+A selects all
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = newM.(Model)

	tab = m.TabManager.ActiveTab()
	if !tab.Buffer.HasSelection {
		t.Errorf("expected buffer to have selection after Ctrl+A")
	}
	if tab.Buffer.GetSelectedText() != "Hello World!\nSecond Line Here" {
		t.Errorf("unexpected selected text: %q", tab.Buffer.GetSelectedText())
	}
}

func TestSoftWrapToggleAndWordJump(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tempDir, err := os.MkdirTemp("", "tui-wrap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "test.go")
	_ = os.WriteFile(filePath, []byte("package main\n\nfunc main() {\n\t// comment\n}\n"), 0644)

	m, err := NewModelWithPath(tempDir)
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

	// 1. Initial state: SoftWrap should be true
	if !m.SoftWrap || !tab.Buffer.SoftWrap {
		t.Errorf("Expected initial SoftWrap to be true")
	}

	// 2. Press Alt+Z -> Toggle to false
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}, Alt: true})
	m = newM.(Model)
	if m.SoftWrap || tab.Buffer.SoftWrap {
		t.Errorf("Expected SoftWrap to be false after Alt+Z")
	}

	// 3. Menu action setting.softwrap -> Toggle back to true
	_ = m.handleMenuAction("setting.softwrap")
	if !m.SoftWrap || !tab.Buffer.SoftWrap {
		t.Errorf("Expected SoftWrap to be true after menu action")
	}

	// 4. Test Word Jump via key messages
	tab.Buffer.SetCursor(0, 0)
	// Press ctrl+right
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = newM.(Model)
	if tab.Buffer.CursorCol != 8 { // after "package " -> lands at "main" (col 8)
		t.Errorf("Expected cursor at col 8 after Ctrl+Right, got %d", tab.Buffer.CursorCol)
	}

	// Press ctrl+left
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	m = newM.(Model)
	if tab.Buffer.CursorCol != 0 {
		t.Errorf("Expected cursor at col 0 after Ctrl+Left, got %d", tab.Buffer.CursorCol)
	}

	// 5. Test Mouse Click Gutter alignment
	// Gutter width for 5 lines is 7 (3 digits + 4)
	if tab.Buffer.GutterWidth() != 7 {
		t.Errorf("Expected gutter width 7, got %d", tab.Buffer.GutterWidth())
	}
	// Click on Row 2 (first editor text row), X = sidebarWidth + gutterWidth + 3
	editorStartX := 30
	clickX := editorStartX + tab.Buffer.GutterWidth() + 3
	newM, _ = m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = newM.(Model)
	if tab.Buffer.CursorRow != 0 || tab.Buffer.CursorCol != 3 {
		t.Errorf("Expected cursor at (0, 3) after precise click, got (%d, %d)", tab.Buffer.CursorRow, tab.Buffer.CursorCol)
	}
}


