package app

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"fmt"
	"strings"
	"time"
	"tui-workflow/internal/editor"
	"tui-workflow/internal/git"
	"tui-workflow/internal/linter"
	"tui-workflow/internal/lsp"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case linter.LintFinishedMsg:
		if m.LinterStore != nil {
			m.LinterStore.Set(msg.FilePath, msg.Diagnostics)
		}
		if tab := m.TabManager.ActiveTab(); tab != nil && tab.Buffer != nil && tab.Path == msg.FilePath {
			if m.LinterStore != nil {
				tab.Buffer.LineDiagnostics = m.LinterStore.LineDiagnostics(msg.FilePath)
			}
		}
		m.syncStatusBarWithActiveTab()
		return m, nil

	case lspCompletionResultMsg:
		if len(msg.Items) > 0 && m.CompletionPopup != nil {
			curX := msg.X
			curY := msg.Y
			if curX < 0 {
				curX = 0
			}
			if curX > m.Width-40 {
				curX = m.Width - 40
			}
			if curY > m.Height-10 {
				curY = curY - 10
			}
			if curY < 2 {
				curY = 2
			}
			// Prepend matching local snippets to LSP items
			var merged []lsp.CompletionItem
			tab := m.TabManager.ActiveTab()
			if tab != nil && tab.Buffer != nil {
				local := lsp.BuildLocalCompletions(tab.Buffer, tab.Path, nil)
				for _, it := range local {
					if it.Kind == lsp.CompletionKindSnippet && (msg.Prefix == "" || strings.HasPrefix(strings.ToLower(it.Label), strings.ToLower(msg.Prefix))) {
						merged = append(merged, it)
					}
				}
			}
			merged = append(merged, msg.Items...)
			m.CompletionPopup.SetItems(merged, curX, curY, msg.Prefix)
		}
		return m, nil
	case sessionPersistMsg:
		// best-effort persistence; avoid noisy errors
		return m, nil

	case symbolIndexReadyMsg:
		return m, nil

	case gitStatusResultMsg:
		if m.GitView != nil {
			m.GitView.ApplyRefresh(msg.data)
			if m.GitRepo != nil {
				if branch, err := m.GitRepo.GetBranchName(); err == nil {
					m.StatusBar.Branch = branch
				}
			}
		}
		return m, nil

	case quickOpenWarmupMsg:
		return m, nil
	case FileWatchTickMsg:
		m.checkWatchedFiles()
		return m, fileWatchTickCmd()

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
				m.StatusBar.ShowToast("Push failed: "+msg.err.Error(), true, 4*time.Second)
			} else {
				m.GitView.Refresh()
				m.StatusBar.ShowToast("Push succeeded", false, 3*time.Second)
			}
		}
		return m, nil

	case gitCommitResultMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			if msg.err != nil {
				var userErr *git.ErrGitUserNotConfigured
				if errors.As(msg.err, &userErr) {
					m.openGitUserNotConfiguredAlert()
					m.StatusBar.ShowToast("Commit failed: Git user identity not configured", true, 4*time.Second)
				} else {
					m.StatusBar.ShowToast("Commit failed: "+msg.err.Error(), true, 4*time.Second)
				}
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

	case aiCommitGeneratedMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
			m.GitView.CommitInput.SetValue(msg.message)
			m.GitView.CommitInput.Focus()
			m.GitView.FocusArea = git.GitFocusCommitInput
		}
		m.StatusBar.ShowToast("Commit message suggested by AI (Ctrl+Enter to commit)", false, 4*time.Second)
		return m, nil

	case aiCommitErrorMsg:
		if m.GitView != nil {
			m.GitView.IsLoading = false
			m.GitView.LoadingMsg = ""
		}
		m.StatusBar.ShowToast(fmt.Sprintf("AI Error: %v", msg.err), true, 4*time.Second)
		return m, nil

	case FormatCompleteMsg:
		tab := m.TabManager.ActiveTab()
		if tab != nil && tab.ID == msg.TabID && tab.Buffer != nil {
			if msg.Err != nil {
				m.StatusBar.ShowToast("Format: "+msg.Err.Error(), true, 4*time.Second)
			} else {
				if msg.NewText != tab.Buffer.Text() {
					tab.Buffer.ReplaceContentWithUndo(msg.NewText)
					m.StatusBar.ShowToast("✓ Formatted with "+msg.ToolName, false, 3*time.Second)
				} else {
					m.StatusBar.ShowToast("Document already formatted ("+msg.ToolName+")", false, 3*time.Second)
				}
			}
		}
		return m, nil

	case MarqueeTickMsg:
		if m.SidebarVisible && m.SidebarView == SidebarFiles && m.Focus == FocusSidebar && m.Explorer != nil {
			if m.Explorer.SelectedNameExceeds(m.SidebarWidth - 1) {
				m.Explorer.TickMarquee()
				return m, marqueeTickCmd()
			}
		}
		// When not needed or idle outside sidebar, check back in 1 second instead of 250ms
		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return MarqueeTickMsg(t)
		})

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.MenuBar.SetWidth(m.Width)
		m.StatusBar.SetWidth(m.Width)
		if m.QuickOpen != nil {
			m.QuickOpen.SetWidth(m.modalWidth())
		}
		if m.CommandPalette != nil {
			m.CommandPalette.SetWidth(m.modalWidth())
		}
		if m.GlobalSearch != nil {
			m.GlobalSearch.SetWidth(m.modalWidth())
		}
		contentWidth := m.Width - m.SidebarWidth
		if !m.SidebarVisible {
			contentWidth = m.Width
		}
		bodyHeight := m.Height - 3
		for _, tab := range m.TabManager.Tabs {
			if tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				tab.TerminalView.Resize(contentWidth, bodyHeight)
			}
		}
		return m, nil

	case TerminalOutputMsg:
		for _, tab := range m.TabManager.Tabs {
			if tab.ID == msg.TabID && tab.Type == editor.TabTerminal && tab.TerminalView != nil {
				return m, waitForTerminalOutput(tab.TerminalView)
			}
		}
		return m, nil

	case TerminalExitMsg:
		for i, tab := range m.TabManager.Tabs {
			if tab.ID == msg.TabID && tab.Type == editor.TabTerminal {
				m.TabManager.CloseTab(i)
				m.syncStatusBarWithActiveTab()
				m.StatusBar.ShowToast("Terminal ditutup (exit)", false, 2*time.Second)
				break
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	}

	return m, nil
}
