package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestContextMenu(t *testing.T) {
	cm := NewContextMenu()
	if cm.Active {
		t.Fatalf("expected ContextMenu to be inactive initially")
	}

	cm.Open(10, 5, true, true, 80, 24)
	if !cm.Active {
		t.Fatalf("expected ContextMenu to be active after Open")
	}
	if cm.X != 10 || cm.Y != 5 {
		t.Errorf("expected position (10, 5), got (%d, %d)", cm.X, cm.Y)
	}

	// Verify items: copy, cut, paste, select_all
	if len(cm.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(cm.Items))
	}

	rendered := cm.Render()
	if !strings.Contains(rendered, "Copy") || !strings.Contains(rendered, "Paste") {
		t.Errorf("rendered menu missing expected labels: %s", rendered)
	}

	// 1. Right-click release at opening position must NOT close menu
	action, handled := cm.HandleMouse(tea.MouseReleaseMsg{X: 10, Y: 5, Button: tea.MouseRight})
	if !handled || !cm.Active {
		t.Errorf("expected right-click release to be consumed without closing menu")
	}

	// 2. Mouse motion over item 0 ("Copy", X=12, Y=6)
	action, handled = cm.HandleMouse(tea.MouseMotionMsg{X: 12, Y: 6})
	if !handled || !cm.Active || cm.SelectedIdx != 0 {
		t.Errorf("expected hover over item 0 to keep menu active and select idx 0")
	}

	// 3. Left click press on item 0 ("Copy", X=12, Y=6)
	action, handled = cm.HandleMouse(tea.MouseClickMsg{X: 12, Y: 6, Button: tea.MouseLeft})
	if !handled || action != "copy" || cm.Active {
		t.Errorf("expected action 'copy', handled true, closed; got %s, %v, %v", action, handled, cm.Active)
	}

	// 4. Reopen menu for outside click test
	cm.Open(10, 5, true, true, 80, 24)

	// Left click outside menu (X=0, Y=0) closes it
	action, handled = cm.HandleMouse(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if !handled || action != "" || cm.Active {
		t.Errorf("expected left click outside to close menu, got handled: %v, active: %v", handled, cm.Active)
	}
}

func TestContextMenuCustom(t *testing.T) {
	cm := NewContextMenu()
	customItems := []ContextMenuItem{
		{ID: "close_this", Label: "Close", Shortcut: "Ctrl+W"},
		{ID: "close_others", Label: "Close Others"},
		{ID: "close_right", Label: "Close to the Right"},
		{ID: "close_left", Label: "Close to the Left"},
		{ID: "close_all", Label: "Close All"},
	}

	cm.OpenCustom(15, 2, customItems, 80, 24)
	if !cm.Active {
		t.Fatalf("expected custom context menu to be active")
	}
	if len(cm.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(cm.Items))
	}

	rendered := cm.Render()
	if !strings.Contains(rendered, "Close Others") || !strings.Contains(rendered, "Close to the Right") {
		t.Errorf("rendered menu missing custom labels: %s", rendered)
	}

	// Click on item 1: "Close Others" at X=17, Y=4
	action, handled := cm.HandleMouse(tea.MouseClickMsg{X: 17, Y: 4, Button: tea.MouseLeft})
	if !handled || action != "close_others" || cm.Active {
		t.Errorf("expected action 'close_others', got %q, handled=%v, active=%v", action, handled, cm.Active)
	}
}
