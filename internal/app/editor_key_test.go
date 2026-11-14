package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/editor"
)

func TestEditorWordJumpSelectionAndWordBackspace(t *testing.T) {
	m, _ := NewModelWithPath(t.TempDir(), false)
	m.Width = 100
	m.Height = 30
	m.Ready = true
	m.Focus = FocusEditor

	tab := editor.NewFileTab("test.txt", "hello world test")
	m.TabManager.Tabs = []*editor.Tab{tab}
	m.TabManager.ActiveIndex = 0
	tab.Buffer.SetCursor(0, 0)
	tab.Buffer.ClearSelection()

	// 1. Alt+Shift+Right (KeyShiftRight with Alt=true) selects "hello "
	updM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt | tea.ModShift})
	m = updM.(Model)

	if !tab.Buffer.HasSelection {
		t.Fatalf("expected HasSelection true after alt+shift+right")
	}
	if tab.Buffer.CursorCol != 6 {
		t.Errorf("expected cursor at 6, got %d", tab.Buffer.CursorCol)
	}
	if sel := tab.Buffer.GetSelectedText(); sel != "hello " {
		t.Errorf("expected 'hello ', got %q", sel)
	}

	// 2. Backspace deletes selection
	updM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updM.(Model)
	if tab.Buffer.HasSelection {
		t.Errorf("expected selection cleared after backspace")
	}
	if string(tab.Buffer.Lines[0]) != "world test" {
		t.Errorf("expected 'world test', got %q", string(tab.Buffer.Lines[0]))
	}

	// 3. Alt+Backspace deletes word left
	tab.Buffer.SetCursor(0, 5) // cursor after "world" in "world test"
	updM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModAlt})
	m = updM.(Model)
	if string(tab.Buffer.Lines[0]) != " test" {
		t.Errorf("expected ' test', got %q", string(tab.Buffer.Lines[0]))
	}
	if tab.Buffer.CursorCol != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.Buffer.CursorCol)
	}

	// 4. Shift+Right selects single character
	tab.Buffer.SetCursor(0, 0)
	tab.Buffer.ClearSelection()
	updM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	m = updM.(Model)
	if !tab.Buffer.HasSelection {
		t.Fatalf("expected HasSelection true after Shift+Right")
	}
	if sel := tab.Buffer.GetSelectedText(); sel != " " {
		t.Errorf("expected ' ', got %q", sel)
	}

	// 5. Arrow without shift clears selection
	updM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updM.(Model)
	if tab.Buffer.HasSelection {
		t.Errorf("expected selection cleared after Left arrow")
	}
}
