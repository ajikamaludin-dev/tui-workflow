package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCommandPalette(t *testing.T) {
	items := []CommandPaletteItem{
		{ID: "file.save", Title: "Save File", Category: "File", Shortcut: "Ctrl+S"},
		{ID: "setting.sidebar", Title: "Toggle Sidebar", Category: "View", Shortcut: "Ctrl+B"},
		{ID: "app.exit", Title: "Exit Application", Category: "App", Shortcut: "Ctrl+Q"},
	}

	cp := NewCommandPalette()
	cp.Open(items)

	if len(cp.Filtered) != 3 {
		t.Fatalf("expected 3 items, got %d", len(cp.Filtered))
	}

	// Filter
	cp.Filter.SetValue("sidebar")
	cp.applyFilter()
	if len(cp.Filtered) != 1 {
		t.Fatalf("expected 1 match for 'sidebar', got %d", len(cp.Filtered))
	}
	if cp.Filtered[0].ID != "setting.sidebar" {
		t.Errorf("expected setting.sidebar, got %s", cp.Filtered[0].ID)
	}

	// Enter
	action, closed, _ := cp.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !closed || action != "setting.sidebar" {
		t.Errorf("expected closed=true and action=setting.sidebar, got closed=%v, action=%s", closed, action)
	}
}
