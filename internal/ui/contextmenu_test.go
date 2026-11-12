package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	if !strings.Contains(rendered, "Salin") || !strings.Contains(rendered, "Tempel") {
		t.Errorf("rendered menu missing expected labels: %s", rendered)
	}

	// 1. Right-click release at opening position must NOT close menu
	action, handled := cm.HandleMouse(tea.MouseMsg{
		X:      10,
		Y:      5,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionRelease,
	})
	if !handled || !cm.Active {
		t.Errorf("expected right-click release to be consumed without closing menu")
	}

	// 2. Mouse motion over item 0 ("Salin", X=12, Y=6)
	action, handled = cm.HandleMouse(tea.MouseMsg{
		X:      12,
		Y:      6,
		Action: tea.MouseActionMotion,
	})
	if !handled || !cm.Active || cm.SelectedIdx != 0 {
		t.Errorf("expected hover over item 0 to keep menu active and select idx 0")
	}

	// 3. Left click press on item 0 ("Salin", X=12, Y=6)
	action, handled = cm.HandleMouse(tea.MouseMsg{
		X:      12,
		Y:      6,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !handled || action != "copy" || cm.Active {
		t.Errorf("expected action 'copy', handled true, closed; got %s, %v, %v", action, handled, cm.Active)
	}

	// 4. Reopen menu for outside click test
	cm.Open(10, 5, true, true, 80, 24)

	// Left click outside menu (X=0, Y=0) closes it
	action, handled = cm.HandleMouse(tea.MouseMsg{
		X:      0,
		Y:      0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !handled || action != "" || cm.Active {
		t.Errorf("expected left click outside to close menu, got handled: %v, active: %v", handled, cm.Active)
	}
}

func TestContextMenuCustom(t *testing.T) {
	cm := NewContextMenu()
	customItems := []ContextMenuItem{
		{ID: "close_this", Label: "Tutup", Shortcut: "Ctrl+W"},
		{ID: "close_others", Label: "Tutup Lainnya"},
		{ID: "close_right", Label: "Tutup ke Kanan"},
		{ID: "close_left", Label: "Tutup ke Kiri"},
		{ID: "close_all", Label: "Tutup Semua"},
	}

	cm.OpenCustom(15, 2, customItems, 80, 24)
	if !cm.Active {
		t.Fatalf("expected custom context menu to be active")
	}
	if len(cm.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(cm.Items))
	}

	rendered := cm.Render()
	if !strings.Contains(rendered, "Tutup Lainnya") || !strings.Contains(rendered, "Tutup ke Kanan") {
		t.Errorf("rendered menu missing custom labels: %s", rendered)
	}

	// Click on item 1: "Tutup Lainnya" at X=17, Y=4
	action, handled := cm.HandleMouse(tea.MouseMsg{
		X:      17,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !handled || action != "close_others" || cm.Active {
		t.Errorf("expected action 'close_others', got %q, handled=%v, active=%v", action, handled, cm.Active)
	}
}
