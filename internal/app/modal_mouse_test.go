package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModalMouseInteractivity(t *testing.T) {
	tmpDir := t.TempDir()
	sampleFile := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(sampleFile, []byte("line 1\nline 2\nline 3\n"), 0644)

	m, err := NewModelWithPath(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	m.Width = 100
	m.Height = 30
	m.StatusBar.SetWidth(m.Width)
	m.Ready = true

	// 1. Test Command Palette Mouse Click and Outside Click
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m = m2.(Model)
	if m.CommandPalette == nil || !m.CommandPalette.Active {
		t.Fatal("expected Command Palette to be active")
	}

	// Click outside Command Palette -> should dismiss
	dismissMsg := tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft}
	m3, _ := m.Update(dismissMsg)
	m = m3.(Model)
	if m.CommandPalette.Active {
		t.Error("expected Command Palette to close on outside click")
	}

	// Re-open and click item
	m4, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m = m4.(Model)
	modalW := m.CommandPalette.Width
	modalH := strings.Count(m.CommandPalette.Render(), "\n") + 1
	startX := (m.Width - modalW) / 2
	startY := (m.Height - modalH) / 3
	// Click on list item row (list starts at relY 6)
	clickItemMsg := tea.MouseClickMsg{X: startX + 10, Y: startY + 6, Button: tea.MouseLeft}
	m5, _ := m.Update(clickItemMsg)
	m = m5.(Model)
	if m.CommandPalette.Active {
		t.Error("expected Command Palette to close after clicking an action")
	}

	// Close any open dialog from previous action (e.g. InputDialog from file.new)
	if m.InputDialog.Active {
		m.InputDialog.Close()
	}

	// 2. Test QuickOpen Mouse Interaction
	m.openQuickOpen()
	if m.QuickOpen == nil || !m.QuickOpen.Active {
		t.Fatal("expected QuickOpen to be active")
	}
	// Click outside -> dismiss
	m7, _ := m.Update(dismissMsg)
	m = m7.(Model)
	if m.QuickOpen.Active {
		t.Error("expected QuickOpen to close on outside click")
	}

	// 3. Test FindBar Mouse Interaction
	m.openFile(sampleFile)
	m.openFindBar()
	if m.FindBar == nil || !m.FindBar.Active {
		t.Fatal("expected FindBar to be active")
	}

	// Click outside find bar -> dismisses
	outsideFindMsg := tea.MouseClickMsg{X: 10, Y: 10, Button: tea.MouseLeft}
	m8, _ := m.Update(outsideFindMsg)
	m = m8.(Model)
	if m.FindBar.Active {
		t.Error("expected FindBar to close on outside click")
	}

	// 4. Test Help Modal Mouse Dismissal
	m.ShowHelp = true
	helpClickMsg := tea.MouseClickMsg{X: 50, Y: 15, Button: tea.MouseLeft}
	m9, _ := m.Update(helpClickMsg)
	m = m9.(Model)
	if m.ShowHelp {
		t.Error("expected Help Modal to close on mouse click")
	}
}

func TestStatusBarMouseInteraction(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewModelWithPath(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	m.Width = 100
	m.Height = 30
	m.StatusBar.SetWidth(m.Width)
	m.Ready = true
	m.AutoSave = true
	m.StatusBar.AutoSave = true

	// 1. Click on [AutoSave] badge (col 2, bottom row)
	clickAutoSaveMsg := tea.MouseClickMsg{X: 2, Y: m.Height - 1, Button: tea.MouseLeft}
	newModel, _ := m.Update(clickAutoSaveMsg)
	m = newModel.(Model)
	if m.AutoSave {
		t.Error("expected AutoSave to toggle to false on status bar click")
	}

	// Click again to toggle back on
	newModel, _ = m.Update(clickAutoSaveMsg)
	m = newModel.(Model)
	if !m.AutoSave {
		t.Error("expected AutoSave to toggle to true on second click")
	}

	// 2. Click on ?: Help (right side of status bar)
	clickHelpMsg := tea.MouseClickMsg{X: m.Width - 3, Y: m.Height - 1, Button: tea.MouseLeft}
	newModel, _ = m.Update(clickHelpMsg)
	m = newModel.(Model)
	if !m.ShowHelp {
		t.Error("expected ShowHelp to become true after clicking status bar help item")
	}
}
