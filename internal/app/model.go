package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"tui-workflow/internal/config"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/explorer"
	"tui-workflow/internal/git"
	"tui-workflow/internal/ui"
	"tui-workflow/internal/ui/dialog"
)

type Model struct {
	RootDir        string
	Config         *config.Config
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
	FilePicker     *dialog.FilePicker
	InputDialog    *dialog.InputDialog
	ConfirmDialog  *dialog.ConfirmDialog
	Switcher       *dialog.BufferSwitcher
	ContextMenu    *ui.ContextMenu
	AutoSave       bool
	ShowHelp       bool
	ShowAbout      bool
	ClosingTabIdx  int
}

func NewModelWithPath(targetPath string) (Model, error) {
	if targetPath == "" {
		targetPath = "."
	}
	cleanPath, err := filepath.Abs(targetPath)
	if err != nil {
		cleanPath = targetPath
	}
	fi, err := os.Stat(cleanPath)
	if err != nil {
		return Model{}, fmt.Errorf("path tidak ditemukan: %w", err)
	}

	var rootDir string
	var initialFile string

	if fi.IsDir() {
		rootDir = cleanPath
		prdPath := filepath.Join(rootDir, "PRD.md")
		if _, err := os.Stat(prdPath); err == nil {
			initialFile = prdPath
		}
	} else {
		rootDir = filepath.Dir(cleanPath)
		initialFile = cleanPath
	}

	tm := editor.NewTabManager()
	if initialFile != "" {
		_, _ = tm.OpenFile(initialFile)
	}

	repo, _ := git.OpenRepo(rootDir)
	var gv *git.GitView
	if repo != nil {
		gv = git.NewGitView(repo)
	}

	cfg, _ := config.LoadConfig()

	m := Model{
		RootDir:        rootDir,
		Config:         cfg,
		Focus:          FocusEditor,
		SidebarVisible: true,
		SidebarWidth:   30,
		SidebarView:    SidebarFiles,
		MenuBar:        ui.NewMenuBar(),
		StatusBar:      ui.NewStatusBar(),
		Explorer:       explorer.NewTree(rootDir),
		GitRepo:        repo,
		GitView:        gv,
		TabManager:     tm,
		FilePicker:     dialog.NewFilePicker(),
		InputDialog:    dialog.NewInputDialog(),
		ConfirmDialog:  dialog.NewConfirmDialog(),
		Switcher:       dialog.NewBufferSwitcher(),
		ContextMenu:    ui.NewContextMenu(),
		AutoSave:       cfg.AutoSave,
		ShowHelp:       false,
		ShowAbout:      false,
		ClosingTabIdx:  -1,
	}
	m.Explorer.ShowHidden = cfg.ShowHidden
	m.StatusBar.AutoSave = cfg.AutoSave
	if repo != nil {
		if branch, err := repo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	}
	m.syncStatusBarWithActiveTab()
	return m, nil
}

func NewModel() Model {
	m, _ := NewModelWithPath(".")
	return m
}

type MarqueeTickMsg time.Time

func marqueeTickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return MarqueeTickMsg(t)
	})
}

type gitLoadingTickMsg struct{}

func gitLoadingTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return gitLoadingTickMsg{}
	})
}

type gitPushResultMsg struct {
	err error
}

func gitPushCmd(repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitPushResultMsg{err: fmt.Errorf("repositori git tidak ditemukan")}
		}
		err := repo.Push()
		return gitPushResultMsg{err: err}
	}
}

type gitCommitResultMsg struct {
	hash string
	err  error
}

func gitCommitCmd(repo *git.Repo, commitMsg string) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return gitCommitResultMsg{err: fmt.Errorf("repositori git tidak ditemukan")}
		}
		hash, err := repo.Commit(commitMsg)
		return gitCommitResultMsg{hash: hash, err: err}
	}
}

func (m Model) Init() tea.Cmd {
	return marqueeTickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gitLoadingTickMsg:
		if m.GitView != nil && m.GitView.IsLoading {
			m.GitView.AdvanceSpinner()
			return m, gitLoadingTickCmd()
		}
		return m, nil

	case gitPushResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				m.StatusBar.ShowToast("Push gagal: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Push berhasil", false, 3*time.Second)
			}
		}
		return m, nil

	case gitCommitResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				m.StatusBar.ShowToast("Commit gagal: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.CommitInput.SetValue("")
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Commit sukses: "+msg.hash, false, 3*time.Second)
				if m.GitRepo != nil {
					if branch, err := m.GitRepo.GetBranchName(); err == nil {
						m.StatusBar.Branch = branch
					}
				}
			}
		}
		return m, nil

	case MarqueeTickMsg:
		if m.SidebarVisible && m.SidebarView == SidebarFiles && m.Focus == FocusSidebar && m.Explorer != nil {
			if m.Explorer.SelectedNameExceeds(m.SidebarWidth - 1) {
				m.Explorer.TickMarquee()
			}
		}
		return m, marqueeTickCmd()

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.MenuBar.SetWidth(m.Width)
		m.StatusBar.SetWidth(m.Width)
		return m, nil

	case tea.KeyMsg:
		// 1. Context Menu Handling if active
		if m.ContextMenu != nil && m.ContextMenu.Active {
			switch msg.String() {
			case "esc":
				m.ContextMenu.Close()
				return m, nil
			case "up", "k":
				m.ContextMenu.Prev()
				return m, nil
			case "down", "j":
				m.ContextMenu.Next()
				return m, nil
			case "enter":
				item := m.ContextMenu.SelectedItem()
				m.ContextMenu.Close()
				if item != nil && !item.Disabled {
					cmd := m.handleContextMenuAction(item.ID)
					return m, cmd
				}
				return m, nil
			}
			return m, nil
		}

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

		// 1.5. File Picker Handling if active
		if m.FilePicker != nil && m.FilePicker.Active {
			confirmed, canceled, selectedPath, cmd := m.FilePicker.Update(msg)
			if confirmed {
				m.handleFilePickerConfirmed(selectedPath)
			} else if canceled {
				m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
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
			if m.GitView != nil && m.GitRepo != nil {
				if m.GitView.IsLoading {
					return m, nil
				}
				if len(m.GitView.Staged) == 0 {
					m.StatusBar.ShowToast("Tidak ada perubahan di staging area", true, 3*time.Second)
					return m, nil
				}
				commitMsg := m.GitView.CommitInput.Value()
				m.GitView.IsLoading = true
				m.GitView.LoadingMsg = "Committing..."
				m.GitView.SpinnerFrame = 0
				m.StatusBar.ShowToast("Melakukan commit...", false, 3*time.Second)
				return m, tea.Batch(gitCommitCmd(m.GitRepo, commitMsg), gitLoadingTickCmd())
			}
			return m, nil

		case "ctrl+o":
			m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)
			return m, nil

		case "ctrl+k":
			m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)
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
					if fileToOpen.Status == "?" || fileToOpen.Status == "U" {
						fullPath := filepath.Join(m.RootDir, fileToOpen.Path)
						m.openFile(fullPath)
						m.Focus = FocusEditor
					} else {
						m.openDiff(fileToOpen.Path)
					}
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

				case "ctrl+c":
					text := buf.GetSelectedText()
					if text != "" {
						_ = clipboard.WriteAll(text)
						m.StatusBar.ShowToast("Disalin ke clipboard ("+strconv.Itoa(len(text))+" karakter)", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+v":
					text, err := clipboard.ReadAll()
					if err == nil && text != "" {
						buf.InsertString(text)
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Ditempel dari clipboard", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+x":
					text := buf.GetSelectedText()
					if text != "" {
						_ = clipboard.WriteAll(text)
						buf.DeleteSelection()
						m.syncStatusBarWithActiveTab()
						m.StatusBar.ShowToast("Dipotong ke clipboard", false, 2*time.Second)
					}
					return m, nil

				case "ctrl+a":
					buf.SelectAll()
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
					buf.ClearSelection()
					buf.MoveUp()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "down":
					buf.ClearSelection()
					buf.MoveDown()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "left":
					buf.ClearSelection()
					buf.MoveLeft()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "right":
					buf.ClearSelection()
					buf.MoveRight()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "home":
					buf.ClearSelection()
					buf.MoveHome()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "end":
					buf.ClearSelection()
					buf.MoveEnd()
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgup":
					buf.ClearSelection()
					buf.MovePageUp(10)
					m.syncStatusBarWithActiveTab()
					return m, nil

				case "pgdown":
					buf.ClearSelection()
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
		// Forward mouse to context menu
		if m.ContextMenu != nil && m.ContextMenu.Active {
			action, handled := m.ContextMenu.HandleMouse(msg)
			if handled {
				if action != "" {
					cmd := m.handleContextMenuAction(action)
					return m, cmd
				}
				return m, nil
			}
		}

		editorStartX := 0
		if m.SidebarVisible {
			editorStartX = m.SidebarWidth
		}

		// Update mouse pointer shape: text over editor, default elsewhere
		if msg.X >= editorStartX && msg.Y >= 2 && msg.Y < m.Height-1 {
			fmt.Print("\x1b]22;text\x1b\\")
		} else {
			fmt.Print("\x1b]22;default\x1b\\")
		}

		// Right-click inside editor area -> Open Context Menu
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
			if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
				m.Focus = FocusEditor
				hasSelection := false
				if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil {
					hasSelection = tab.Buffer.HasSelection
				}
				clipText, _ := clipboard.ReadAll()
				hasClipboard := len(clipText) > 0
				m.ContextMenu.Open(msg.X, msg.Y, hasSelection, hasClipboard, m.Width, m.Height)
				return m, nil
			}
		}

		// Drag mouse inside editor buffer area to select text
		if msg.Action == tea.MouseActionMotion && msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil {
				relY := msg.Y - 2
				gutterDigits := len(strconv.Itoa(tab.Buffer.LineCount()))
				if gutterDigits < 2 {
					gutterDigits = 2
				}
				gutterW := gutterDigits + 4
				textX := msg.X - editorStartX - gutterW
				if textX < 0 {
					textX = 0
				}
				textW := m.Width - editorStartX - gutterW
				if textW < 10 {
					textW = 10
				}
				physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)
				tab.Buffer.UpdateSelection(physRow, physCol)
				m.syncStatusBarWithActiveTab()
			}
			return m, nil
		}

		// Release mouse inside editor area
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil && tab.Buffer.HasSelection {
				if tab.Buffer.SelectAnchorRow == tab.Buffer.CursorRow && tab.Buffer.SelectAnchorCol == tab.Buffer.CursorCol {
					tab.Buffer.ClearSelection()
					m.syncStatusBarWithActiveTab()
				}
			}
			return m, nil
		}

		// Forward mouse to menu bar
		if action, handled := m.MenuBar.HandleMouse(msg); handled {
			if action != "" {
				cmd := m.handleMenuAction(action)
				return m, cmd
			}
			return m, nil
		}

		// Mouse scroll wheel on Tab Bar (Row 1): scroll tabs horizontally
		if msg.Y == 1 && msg.X >= editorStartX {
			if msg.Button == tea.MouseButtonWheelUp {
				m.TabManager.ScrollTabsLeft()
				return m, nil
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.TabManager.ScrollTabsRight()
				return m, nil
			}
		}

		// Mouse scroll wheel in sidebar (Row 2 to Height-2)
		if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 2 && msg.Y < m.Height-1 {
			sidebarH := m.Height - 3
			if msg.Button == tea.MouseButtonWheelUp {
				if m.SidebarView == SidebarFiles && m.Explorer != nil {
					m.Explorer.ScrollUp(3)
					return m, nil
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					m.GitView.ScrollUp(3)
					return m, nil
				}
			} else if msg.Button == tea.MouseButtonWheelDown {
				if m.SidebarView == SidebarFiles && m.Explorer != nil {
					m.Explorer.ScrollDown(3, sidebarH)
					return m, nil
				} else if m.SidebarView == SidebarGit && m.GitView != nil {
					m.GitView.ScrollDown(3, sidebarH)
					return m, nil
				}
			}
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

		// Intercept mouse clicks for active modals
		if m.FilePicker != nil && m.FilePicker.Active {
			modalW := m.FilePicker.Width
			modalH := m.FilePicker.Height
			startX := (m.Width - modalW) / 2
			startY := (m.Height - modalH) / 2
			relX := msg.X - startX
			relY := msg.Y - startY
			isWheelUp := msg.Button == tea.MouseButtonWheelUp
			isWheelDown := msg.Button == tea.MouseButtonWheelDown
			if msg.Action == tea.MouseActionPress || isWheelUp || isWheelDown {
				confirmed, canceled, selectedPath, handled := m.FilePicker.HandleMouse(relX, relY, isWheelUp, isWheelDown)
				if confirmed {
					m.handleFilePickerConfirmed(selectedPath)
				} else if canceled {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.InputDialog.Active {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				modalW := m.InputDialog.Width
				modalH := strings.Count(m.InputDialog.Render(), "\n") + 1
				startX := (m.Width - modalW) / 2
				startY := (m.Height - modalH) / 2
				relX := msg.X - startX
				relY := msg.Y - startY
				confirmed, canceled, handled := m.InputDialog.HandleMouse(relX, relY)
				if confirmed {
					m.handleInputConfirmed()
				} else if canceled {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.ConfirmDialog.Active {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				modalW := m.ConfirmDialog.Width
				modalH := strings.Count(m.ConfirmDialog.Render(), "\n") + 1
				startX := (m.Width - modalW) / 2
				startY := (m.Height - modalH) / 2
				relX := msg.X - startX
				relY := msg.Y - startY
				confirmed, canceled, discard, handled := m.ConfirmDialog.HandleMouse(relX, relY)
				if confirmed {
					m.handleConfirmConfirmed()
				} else if discard {
					m.handleDiscardConfirmed()
				} else if canceled {
					m.StatusBar.ShowToast("Operasi dibatalkan", false, 2*time.Second)
					m.ClosingTabIdx = -1
				}
				if handled {
					return m, nil
				}
			}
			return m, nil
		}

		if m.Switcher.Active {
			if msg.Action == tea.MouseActionPress {
				m.Switcher.Close()
			}
			return m, nil
		}

		// Mouse click handling
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Row 1: Tab Bar click (Editor columns)
			if msg.Y == 1 && msg.X >= editorStartX {
				relX := msg.X - editorStartX
				action, clickedTabIdx := m.TabManager.HandleTabClick(relX)
				if action == "scroll_left" {
					m.TabManager.ScrollTabsLeft()
					return m, nil
				}
				if action == "scroll_right" {
					m.TabManager.ScrollTabsRight()
					return m, nil
				}
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
			if m.SidebarVisible && msg.X < m.SidebarWidth && msg.Y >= 2 && msg.Y < m.Height-1 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
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
					targetFile, action, _ := m.GitView.HandleMouseClick(msg.X, relLine)
					if action == "commit" {
						if m.GitView.IsLoading {
							return m, nil
						}
						if len(m.GitView.Staged) == 0 {
							m.StatusBar.ShowToast("Tidak ada perubahan di staging area", true, 3*time.Second)
							return m, nil
						}
						commitMsg := m.GitView.CommitInput.Value()
						m.GitView.IsLoading = true
						m.GitView.LoadingMsg = "Committing..."
						m.GitView.SpinnerFrame = 0
						m.StatusBar.ShowToast("Melakukan commit...", false, 3*time.Second)
						return m, tea.Batch(gitCommitCmd(m.GitRepo, commitMsg), gitLoadingTickCmd())
					} else if action == "push" {
						if m.GitView.IsLoading {
							return m, nil
						}
						if m.GitRepo == nil {
							m.StatusBar.ShowToast("Repositori git tidak ditemukan", true, 3*time.Second)
							return m, nil
						}
						m.GitView.IsLoading = true
						m.GitView.LoadingMsg = "Pushing..."
						m.GitView.SpinnerFrame = 0
						m.StatusBar.ShowToast("Pushing ke remote repository...", false, 5*time.Second)
						return m, tea.Batch(gitPushCmd(m.GitRepo), gitLoadingTickCmd())
					}
					if targetFile != nil {
						if targetFile.Status == "?" || targetFile.Status == "U" {
							fullPath := filepath.Join(m.RootDir, targetFile.Path)
							m.openFile(fullPath)
							m.Focus = FocusEditor
						} else {
							m.openDiff(targetFile.Path)
						}
					}
				}
				return m, nil
			}

			// Click inside editor buffer area (Row 2 to Height-2, X >= editorStartX)
			if msg.Y >= 2 && msg.Y < m.Height-1 && msg.X >= editorStartX {
				m.Focus = FocusEditor
				tab := m.TabManager.ActiveTab()
				if tab != nil && tab.Buffer != nil {
					relY := msg.Y - 2
					gutterDigits := len(strconv.Itoa(tab.Buffer.LineCount()))
					if gutterDigits < 2 {
						gutterDigits = 2
					}
					gutterW := gutterDigits + 4
					textX := msg.X - editorStartX - gutterW
					if textX < 0 {
						textX = 0
					}
					textW := m.Width - editorStartX - gutterW
					if textW < 10 {
						textW = 10
					}
					physRow, physCol := tab.Buffer.RowColFromVisual(relY, textX, textW)
					tab.Buffer.StartSelection(physRow, physCol)
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
	if m.GitRepo != nil {
		if branch, err := m.GitRepo.GetBranchName(); err == nil {
			m.StatusBar.Branch = branch
		}
	} else {
		m.StatusBar.Branch = ""
	}

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

	case dialog.InputOpenFile:
		target := val
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.RootDir, target)
		}
		fi, err := os.Stat(target)
		if err != nil {
			m.StatusBar.ShowToast("File tidak ditemukan: "+err.Error(), true, 3*time.Second)
		} else if fi.IsDir() {
			m.StatusBar.ShowToast("Path adalah direktori, bukan file", true, 3*time.Second)
		} else {
			m.openFile(target)
			m.Focus = FocusEditor
		}

	case dialog.InputOpenFolder:
		target := val
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.RootDir, target)
		}
		cleanPath, err := filepath.Abs(target)
		if err != nil {
			m.StatusBar.ShowToast("Path tidak valid: "+err.Error(), true, 3*time.Second)
			return
		}
		fi, err := os.Stat(cleanPath)
		if err != nil || !fi.IsDir() {
			m.StatusBar.ShowToast("Direktori tidak ditemukan", true, 3*time.Second)
			return
		}

		m.RootDir = cleanPath
		if m.Config != nil {
			m.Config.LastDir = cleanPath
			_ = config.SaveConfig(m.Config)
		}
		m.Explorer = explorer.NewTree(cleanPath)
		if m.Config != nil {
			m.Explorer.ShowHidden = m.Config.ShowHidden
		}
		repo, _ := git.OpenRepo(cleanPath)
		m.GitRepo = repo
		if repo != nil {
			m.GitView = git.NewGitView(repo)
			branch, _ := repo.GetBranchName()
			m.StatusBar.Branch = branch
		} else {
			m.GitView = nil
			m.StatusBar.Branch = ""
		}
		m.syncStatusBarWithActiveTab()
		m.StatusBar.ShowToast("Folder dibuka: "+cleanPath, false, 2*time.Second)
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

	case "file.open":
		m.FilePicker.Open(dialog.PickerModeFile, m.RootDir, m.Explorer.ShowHidden)

	case "file.open_folder":
		m.FilePicker.Open(dialog.PickerModeFolder, m.RootDir, m.Explorer.ShowHidden)

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
		if m.Config != nil {
			m.Config.AutoSave = m.AutoSave
			_ = config.SaveConfig(m.Config)
		}
		if m.AutoSave {
			m.StatusBar.ShowToast("Auto-Save Diaktifkan", false, 2*time.Second)
		} else {
			m.StatusBar.ShowToast("Auto-Save Dimatikan", true, 2*time.Second)
		}

	case "setting.sidebar":
		m.SidebarVisible = !m.SidebarVisible

	case "setting.hidden":
		m.Explorer.ToggleHidden()
		if m.Config != nil {
			m.Config.ShowHidden = m.Explorer.ShowHidden
			_ = config.SaveConfig(m.Config)
		}

	case "setting.shortcuts":
		m.ShowHelp = true

	case "about.open":
		m.ShowAbout = true
	}
	return nil
}

func (m *Model) handleFilePickerConfirmed(selectedPath string) {
	if m.FilePicker.Mode == dialog.PickerModeFolder {
		cleanPath, err := filepath.Abs(selectedPath)
		if err != nil {
			cleanPath = selectedPath
		}
		fi, err := os.Stat(cleanPath)
		if err != nil || !fi.IsDir() {
			m.StatusBar.ShowToast("Direktori tidak valid", true, 3*time.Second)
			return
		}

		m.RootDir = cleanPath
		if m.Config != nil {
			m.Config.LastDir = cleanPath
			_ = config.SaveConfig(m.Config)
		}
		m.Explorer = explorer.NewTree(cleanPath)
		if m.Config != nil {
			m.Explorer.ShowHidden = m.Config.ShowHidden
		}
		repo, _ := git.OpenRepo(cleanPath)
		m.GitRepo = repo
		if repo != nil {
			m.GitView = git.NewGitView(repo)
			branch, _ := repo.GetBranchName()
			m.StatusBar.Branch = branch
		} else {
			m.GitView = nil
			m.StatusBar.Branch = ""
		}
		m.syncStatusBarWithActiveTab()
		m.StatusBar.ShowToast("Folder dibuka: "+cleanPath, false, 2*time.Second)
	} else {
		m.openFile(selectedPath)
		m.Focus = FocusEditor
	}
}

func (m *Model) handleContextMenuAction(action string) tea.Cmd {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
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
