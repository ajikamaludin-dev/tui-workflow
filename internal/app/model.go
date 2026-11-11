package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/explorer"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

type Model struct {
	Width          int
	Height         int
	Ready          bool
	Focus          FocusArea
	SidebarVisible bool
	SidebarWidth   int
	SidebarView    SidebarView
	MenuBar        *ui.MenuBar
	StatusBar      *ui.StatusBar
	Explorer       *explorer.Tree
	GitRepo        *git.Repo
	GitView        *git.GitView
	TabManager     *editor.TabManager
	InputDialog    *dialog.InputDialog
	ConfirmDialog  *dialog.ConfirmDialog
	Switcher       *dialog.BufferSwitcher
	AutoSave       bool
	ShowHelp       bool
	ShowAbout      bool
	ClosingTabIdx  int
}

func NewModel() Model {
	tm := editor.NewTabManager()
	// Open PRD.md by default if present
	if _, err := os.Stat("PRD.md"); err == nil {
		_, _ = tm.OpenFile("PRD.md")
	}

	repo, _ := git.OpenRepo(".")
	var gv *git.GitView
	if repo != nil {
		gv = git.NewGitView(repo)
	}

	m := Model{
		Focus:          FocusEditor,
		SidebarVisible: true,
		SidebarWidth:   30,
		SidebarView:    SidebarFiles,
		MenuBar:        ui.NewMenuBar(),
		StatusBar:      ui.NewStatusBar(),
		Explorer:       explorer.NewTree("."),
		GitRepo:        repo,
		GitView:        gv,
		TabManager:     tm,
		InputDialog:    dialog.NewInputDialog(),
		ConfirmDialog:  dialog.NewConfirmDialog(),
		Switcher:       dialog.NewBufferSwitcher(),
		AutoSave:       true,
		ShowHelp:       false,
		ShowAbout:      false,
		ClosingTabIdx:  -1,
	}

	m.syncStatusBarWithActiveTab()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.MenuBar.SetWidth(m.Width)
		m.StatusBar.SetWidth(m.Width)
		return m, nil

	case tea.KeyMsg:
		// 1. Buffer Switcher Modal Handling
		if m.Switcher.Active {
			targetIdx, closed, cmd := m.Switcher.Update(msg)
			if closed {
				if targetIdx >= 0 && targetIdx < m.TabManager.Count() {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.ActiveIndex = targetIdx
					m.syncStatusBarWithActiveTab()
				}
			}
			return m, cmd
		}

		// 2. Input Dialog Handling if active
		if m.InputDialog.Active {
			confirmed, canceled, cmd := m.InputDialog.Update(msg)
			if confirmed {
				m.handleInputConfirmed()
			} else if canceled {
				m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
			}
			return m, cmd
		}

		// 3. Confirm Dialog Handling if active
		if m.ConfirmDialog.Active {
			confirmed, canceled, discard := m.ConfirmDialog.Update(msg)
			if confirmed {
				m.handleConfirmConfirmed()
			} else if discard {
				m.handleDiscardConfirmed()
			} else if canceled {
				m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				m.ClosingTabIdx = -1
			}
			return m, nil
		}

		// 4. Menu Bar Key Handling
		if action, handled := m.MenuBar.HandleKey(msg); handled {
			if action != "" {
				cmd := m.handleMenuAction(action)
				return m, cmd
			}
			return m, nil
		}

		// 5. Modal escape
		if msg.String() == "esc" {
			if m.ShowHelp {
				m.ShowHelp = false
				return m, nil
			}
			if m.ShowAbout {
				m.ShowAbout = false
				return m, nil
			}
		}

		// 6. Global Shortcuts
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			if m.hasDirtyTabs() && !m.AutoSave {
				m.ConfirmDialog.Open(
					dialog.ConfirmUnsavedChanges,
					"Perubahan Belum Disimpan",
					"Terdapat buffer yang belum disimpan.\nSimpan sebelum keluar?",
					"app.exit",
				)
				return m, nil
			}
			if m.AutoSave {
				_, _ = editor.TriggerAutoSave(m.TabManager)
			}
			return m, tea.Quit

		case "ctrl+b":
			m.SidebarVisible = !m.SidebarVisible
			return m, nil

		case "ctrl+e", "f6":
			m.toggleFocus()
			return m, nil

		case "alt+s":
			m.AutoSave = !m.AutoSave
			m.StatusBar.AutoSave = m.AutoSave
			if m.AutoSave {
				m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
			} else {
				m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
			}
			return m, nil

		case "alt+1":
			m.SidebarView = SidebarFiles
			return m, nil

		case "alt+2":
			m.SidebarView = SidebarGit
			if m.GitView != nil {
				m.GitView.Refresh()
			}
			return m, nil

		case "ctrl+enter":
			if m.GitView != nil {
				hash, err := m.GitView.DoCommit()
				if err != nil {
					m.StatusBar.ShowToast("Commit gagal: "+err.Error(), true, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Commit sukses: "+hash, false, 3*time.Second)
				}
			}
			return m, nil

		case "ctrl+p":
			m.openBufferSwitcher()
			return m, nil

		case "ctrl+m", "alt+m":
			if m.TabManager.ToggleActiveMarkdown() {
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.MarkdownPreview {
					m.StatusBar.ShowToast("Mode: Markdown Preview (Glamour)", false, 2*time.Second)
				} else {
					m.StatusBar.ShowToast("Mode: Markdown Editor (Raw)", false, 2*time.Second)
				}
				m.syncStatusBarWithActiveTab()
			} else {
				m.StatusBar.ShowToast("Tab aktif bukan file Markdown", true, 2*time.Second)
			}
			return m, nil

		case "?":
			m.ShowHelp = !m.ShowHelp
			return m, nil
		}

		// 7. Context-Specific: Sidebar (Files or Git)
		if m.Focus == FocusSidebar && m.SidebarVisible {
			if m.SidebarView == SidebarGit && m.GitView != nil {
				cmd, fileToOpen := m.GitView.Update(msg)
				if fileToOpen != nil {
					m.openDiff(fileToOpen.Path)
				}
				return m, cmd
			}

			if m.SidebarView == SidebarFiles {
				switch msg.String() {
			case "j", "down":
				m.Explorer.MoveDown()
				return m, nil

			case "k", "up":
				m.Explorer.MoveUp()
				return m, nil

			case "enter", "l", "right":
				node := m.Explorer.SelectedNode()
				if node != nil {
					if node.IsDir {
						m.Explorer.ToggleExpand()
					} else {
						m.openFile(node.Path)
						m.Focus = FocusEditor
					}
				}
				return m, nil

			case "h", "left":
				m.Explorer.CollapseOrParent()
				return m, nil

			case ".":
				m.Explorer.ToggleHidden()
				return m, nil

			case "r":
				m.Explorer.Refresh()
				m.StatusBar.ShowToast("Pohon file diperbarui", false, 2*time.Second)
				return m, nil

			case "a":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFile, "Buat File Baru", parentDir, "")
				return m, nil

			case "A":
				parentDir := m.getSelectedParentDir()
				m.InputDialog.Open(dialog.InputNewFolder, "Buat Folder Baru", parentDir, "")
				return m, nil

			case "F2":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.InputDialog.Open(dialog.InputRename, "Ubah Nama: "+node.Name, node.Path, node.Name)
				}
				return m, nil

			case "d":
				node := m.Explorer.SelectedNode()
				if node != nil {
					m.ConfirmDialog.Open(
						dialog.ConfirmDelete,
						"Konfirmasi Hapus",
						"Apakah Anda yakin ingin menghapus:\n"+node.Path+"?",
						node.Path,
					)
				}
				return m, nil
			}
		}
	}

	// 8. Context-Specific: Modeless Text Editor Buffer or Diff View
	if m.Focus == FocusEditor {
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.IsDiff && tab.DiffView != nil {
			switch msg.String() {
			case "ctrl+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "up", "k":
				tab.DiffView.ScrollUp(1)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "down", "j":
				tab.DiffView.ScrollDown(1, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgup":
				tab.DiffView.ScrollUp(10)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgdown":
				tab.DiffView.ScrollDown(10, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "home":
				tab.DiffView.ScrollOffset = 0
				m.syncStatusBarWithActiveTab()
				return m, nil
			}
		} else if tab != nil && tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
			switch msg.String() {
			case "ctrl+w":
				m.attemptCloseTab(m.TabManager.ActiveIndex)
				return m, nil
			case "ctrl+n":
				m.TabManager.NextTab()
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "up", "k":
				tab.MarkdownViewer.ScrollUp(1)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "down", "j":
				tab.MarkdownViewer.ScrollDown(1, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgup":
				tab.MarkdownViewer.ScrollUp(10)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "pgdown":
				tab.MarkdownViewer.ScrollDown(10, m.Height-3)
				m.syncStatusBarWithActiveTab()
				return m, nil
			case "home":
				tab.MarkdownViewer.ScrollHome()
				m.syncStatusBarWithActiveTab()
				return m, nil
			}
		} else if tab != nil && tab.Buffer != nil {
			buf := tab.Buffer
			switch msg.String() {
				case "ctrl+s":
					if err := m.TabManager.SaveActive(); err != nil {
						m.StatusBar.ShowToast("Error simpan: "+err.Error(), true, 3*time.Second)
					} else {
						m.StatusBar.ShowToast("Disimpan: "+tab.Name, false, 2*time.Second)
					}
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "ctrl+w":
					m.attemptCloseTab(m.TabManager.ActiveIndex)
					return m, nil

				case "ctrl+n":
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.NextTab()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "enter":
					buf.InsertNewline()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "backspace":
					buf.Backspace()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "delete":
					buf.Delete()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "tab":
					buf.InsertString("    ") // 4 spaces indent
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "up":
					buf.MoveUp()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "down":
					buf.MoveDown()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "left":
					buf.MoveLeft()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "right":
					buf.MoveRight()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "home":
					buf.MoveHome()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "end":
					buf.MoveEnd()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgup":
					buf.MovePageUp(10)
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgdown":
					buf.MovePageDown(10)
					m.syncStatusBarWithActiveTab()
					return m, nil

				default:
					// Insert printable characters
					if len(msg.Runes) > 0 && !msg.Alt {
						for _, r := range msg.Runes {
							buf.InsertRune(r)
						}
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				}
			}
		}

	case tea.MouseMsg:
		// Forward mouse to menu bar
		if action, handled := m.MenuBar.HandleMouse(msg); handled {
			if action != "" {
				cmd := m.handleMenuAction(action)
				return m, cmd
			}
			return m, nil
		}

		// Mouse scroll wheel in editor
		if m.Focus == FocusEditor && m.TabManager.ActiveTab() != nil {
			tab := m.TabManager.ActiveTab()
			if msg.Action == tea.MouseActionPress {
				if tab.IsDiff && tab.DiffView != nil {
					if msg.Button == tea.MouseButtonWheelUp {
						tab.DiffView.ScrollUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						tab.DiffView.ScrollDown(3, m.Height-3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				} else if tab.Type == editor.TabMarkdown && tab.MarkdownPreview && tab.MarkdownViewer != nil {
					if msg.Button == tea.MouseButtonWheelUp {
						tab.MarkdownViewer.ScrollUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						tab.MarkdownViewer.ScrollDown(3, m.Height-3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				} else if tab.Buffer != nil {
					buf := tab.Buffer
					if msg.Button == tea.MouseButtonWheelUp {
						buf.MovePageUp(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					} else if msg.Button == tea.MouseButtonWheelDown {
						buf.MovePageDown(3)
						m.syncStatusBarWithActiveTab()
						return m, nil
					}
				}
			}
		}

		// Mouse click handling
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			editorStartX := 0
			if m.SidebarVisible {
				editorStartX = m.SidebarWidth
			}

			// Row 1: Tab Bar click (Editor columns)
			if msg.Y == 1 && msg.X >= editorStartX {
				relX := msg.X - editorStartX
				action, clickedTabIdx := m.TabManager.HandleTabClick(relX)
				if action == "toggle_markdown" {
					m.TabManager.ToggleActiveMarkdown()
					m.syncStatusBarWithActiveTab()
					return m, nil
				}
				if action == "activate" {
					if m.AutoSave {
						_, _ = editor.TriggerAutoSave(m.TabManager)
					}
					m.TabManager.ActiveIndex = clickedTabIdx
					m.Focus = FocusEditor
					m.syncStatusBarWithActiveTab()
					return m, nil
				} else if action == "close" {
					m.attemptCloseTab(clickedTabIdx)
					return m, nil
				}
			}

			// Row 1: Sidebar tab headers click
			if msg.Y == 1 && m.SidebarVisible && msg.X < m.SidebarWidth {
				if msg.X >= 0 && msg.X < 11 {
					m.SidebarView = SidebarFiles
					m.Focus = FocusSidebar
					return m, nil
				} else if msg.X >= 11 && msg.X < 20 {
					m.SidebarView = SidebarGit
					m.Focus = FocusSidebar
					if m.GitView != nil {
						m.GitView.Refresh()
					}
					return m, nil
				}
			}

			// Sidebar tree rows click (Row 2 to Height-2)
			if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 2 && msg.Y < m.Height-1 {
				if m.Focus != FocusSidebar && m.AutoSave {
					_, _ = editor.TriggerAutoSave(m.TabManager)
				}
				m.Focus = FocusSidebar
				if m.SidebarView == SidebarFiles {
					relLine := msg.Y - 2
					action, node := m.Explorer.HandleClick(relLine)
					if action == "open" && node != nil {
						m.openFile(node.Path)
					}
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					relLine := msg.Y - 2
					targetFile, _ := m.GitView.HandleMouseClick(msg.X, relLine)
					if targetFile != nil {
						m.openDiff(targetFile.Path)
					}
				}
				return m, nil
			}

			// Click inside editor buffer area (Row 2 to Height-2, X >= editorStartX)
			if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
				m.Focus = FocusEditor
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.Buffer != nil {
					// Approximate cursor positioning by row
					relY := msg.Y - 2
					targetRow := tab.Buffer.ScrollRow + relY
					tab.Buffer.SetCursor(targetRow, 0)
					m.syncStatusBarWithActiveTab()
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *Model) toggleFocus() {
	if m.Focus == FocusEditor {
		if m.AutoSave {
			saved, err := editor.TriggerAutoSave(m.TabManager)
			if saved > 0 {
				m.StatusBar.ShowToast(fmt.Sprintf("%d file disimpan", saved), false, 1500*time.Millisecond)
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

func (m *Model) attemptCloseTab(tabIdx int) {
	if tabIdx < 0 || tabIdx >= m.TabManager.Count() {
		return
	}

	tab := m.TabManager.Tabs[tabIdx]
	if tab.IsDirty() && !m.AutoSave {
		m.ClosingTabIdx = tabIdx
		m.ConfirmDialog.Open(
			dialog.ConfirmUnsavedChanges,
			"Simpan Perubahan?",
			fmt.Sprintf("File '%s' telah dimodifikasi.\nSimpan perubahan sebelum menutup?", tab.Name),
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

func (m *Model) openBufferSwitcher() {
	if m.TabManager.Count() == 0 {
		m.StatusBar.ShowToast("Tidak ada tab terbuka", false, 2*time.Second)
		return
	}

	var items []dialog.SwitcherItem
	for i, t := range m.TabManager.Tabs {
		items = append(items, dialog.SwitcherItem{
			Index:   i,
			Title:   t.Name,
			Path:    t.Path,
			IsDirty: t.IsDirty(),
		})
	}

	m.Switcher.Open(items, m.TabManager.ActiveIndex)
}

func (m *Model) syncStatusBarWithActiveTab() {
	tab := m.TabManager.ActiveTab()
	if tab == nil {
		m.StatusBar.FilePath = "[Tidak Ada File Terbuka]"
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
	}

	ext := strings.ToLower(filepath.Ext(tab.Path))
	switch ext {
	case ".go":
		m.StatusBar.FileSyntax = "Go"
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

func (m *Model) getSelectedParentDir() string {
	node := m.Explorer.SelectedNode()
	if node == nil {
		return m.Explorer.RootPath
	}
	if node.IsDir {
		return node.Path
	}
	return filepath.Dir(node.Path)
}

func (m *Model) handleInputConfirmed() {
	val := m.InputDialog.Value()
	switch m.InputDialog.Type {
	case dialog.InputNewFile:
		createdPath, err := explorer.CreateFile(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.openFile(createdPath)
			m.StatusBar.ShowToast("File dibuat: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputNewFolder:
		createdPath, err := explorer.CreateFolder(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Folder dibuat: "+filepath.Base(createdPath), false, 2*time.Second)
		}

	case dialog.InputRename:
		newPath, err := explorer.RenamePath(m.InputDialog.TargetPath, val)
		if err != nil {
			m.StatusBar.ShowToast("Error: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Diubah ke: "+filepath.Base(newPath), false, 2*time.Second)
		}
	}
}

func (m *Model) handleConfirmConfirmed() {
	switch m.ConfirmDialog.Type {
	case dialog.ConfirmDelete:
		err := explorer.DeletePath(m.ConfirmDialog.TargetPath)
		if err != nil {
			m.StatusBar.ShowToast("Error hapus: "+err.Error(), true, 3*time.Second)
		} else {
			m.Explorer.Refresh()
			m.StatusBar.ShowToast("Berhasil dihapus: "+filepath.Base(m.ConfirmDialog.TargetPath), false, 2*time.Second)
		}

	case dialog.ConfirmUnsavedChanges:
		if m.ConfirmDialog.TargetPath == "app.exit" {
			_, _ = editor.TriggerAutoSave(m.TabManager)
			os.Exit(0)
		} else if m.ClosingTabIdx >= 0 {
			_ = m.TabManager.SaveTab(m.ClosingTabIdx)
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Disimpan & ditutup", false, 2*time.Second)
		}
	}
}

func (m *Model) handleDiscardConfirmed() {
	if m.ConfirmDialog.Type == dialog.ConfirmUnsavedChanges {
		if m.ConfirmDialog.TargetPath == "app.exit" {
			os.Exit(0)
		} else if m.ClosingTabIdx >= 0 {
			m.TabManager.CloseTab(m.ClosingTabIdx)
			m.ClosingTabIdx = -1
			m.syncStatusBarWithActiveTab()
			m.StatusBar.ShowToast("Perubahan dibuang", false, 2*time.Second)
		}
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
}

func isBinaryBytes(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	return bytes.IndexByte(data[:limit], 0x00) != -1
}

func (m *Model) openDiff(relPath string) {
	if m.GitRepo == nil {
		return
	}
	fullPath := filepath.Join(m.GitRepo.RootPath, relPath)
	info, err := os.Stat(fullPath)
	if err == nil {
		if info.Size() > editor.MaxFileSize {
			m.StatusBar.ShowToast(fmt.Sprintf("File terlalu besar (%d MB > 10MB)", info.Size()/(1024*1024)), true, 3*time.Second)
			return
		}
	}

	modBytes, _ := os.ReadFile(fullPath)
	if isBinaryBytes(modBytes) {
		m.StatusBar.ShowToast("Diff file biner tidak didukung", true, 3*time.Second)
		return
	}

	orig, _ := m.GitRepo.GetFileContentAtHEAD(relPath)
	if isBinaryBytes([]byte(orig)) {
		m.StatusBar.ShowToast("Diff file biner tidak didukung", true, 3*time.Second)
		return
	}

	m.TabManager.OpenDiff(relPath, orig, string(modBytes))
	m.Focus = FocusEditor
	m.syncStatusBarWithActiveTab()
}

func (m *Model) handleMenuAction(action string) tea.Cmd {
	switch action {
	case "app.exit":
		if m.hasDirtyTabs() && !m.AutoSave {
			m.ConfirmDialog.Open(
				dialog.ConfirmUnsavedChanges,
				"Perubahan Belum Disimpan",
				"Terdapat buffer yang belum disimpan.\nSimpan sebelum keluar?",
				"app.exit",
			)
			return nil
		}
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		}
		return tea.Quit

	case "file.new":
		parentDir := m.getSelectedParentDir()
		m.InputDialog.Open(dialog.InputNewFile, "Buat File Baru", parentDir, "")

	case "file.save":
		if err := m.TabManager.SaveActive(); err != nil {
			m.StatusBar.ShowToast("Gagal simpan: "+err.Error(), true, 3*time.Second)
		} else {
			m.StatusBar.ShowToast("File disimpan", false, 2*time.Second)
			m.syncStatusBarWithActiveTab()
		}

	case "file.close":
		m.attemptCloseTab(m.TabManager.ActiveIndex)

	case "setting.autosave":
		m.AutoSave = !m.AutoSave
		m.StatusBar.AutoSave = m.AutoSave
		if m.AutoSave {
			m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
		}

	case "setting.sidebar":
		m.SidebarVisible = !m.SidebarVisible

	case "setting.hidden":
		m.Explorer.ToggleHidden()

	case "setting.shortcuts":
		m.ShowHelp = true

	case "about.open":
		m.ShowAbout = true
	}
	return nil
}
