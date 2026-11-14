package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpDialogNavigationAndMouse(t *testing.T) {
	hd := NewHelpDialog()
	if hd.Active {
		t.Fatal("expected HelpDialog to be inactive initially")
	}

	hd.Open(24)
	if !hd.Active {
		t.Fatal("expected HelpDialog to be active after Open")
	}
	if hd.ScrollTop != 0 {
		t.Fatalf("expected ScrollTop 0, got %d", hd.ScrollTop)
	}
	if hd.MaxRows <= 0 {
		t.Fatalf("expected positive MaxRows, got %d", hd.MaxRows)
	}

	// Down arrow scrolls down
	hd.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if hd.ScrollTop != 1 {
		t.Fatalf("expected ScrollTop 1 after KeyDown, got %d", hd.ScrollTop)
	}

	// Up arrow scrolls up
	hd.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if hd.ScrollTop != 0 {
		t.Fatalf("expected ScrollTop 0 after KeyUp, got %d", hd.ScrollTop)
	}

	// Wheel down scrolls down
	_, handled := hd.HandleMouse(10, 5, false, true)
	if !handled || hd.ScrollTop != 2 {
		t.Fatalf("expected ScrollTop 2 after WheelDown, got %d (handled: %v)", hd.ScrollTop, handled)
	}

	// Wheel up scrolls up
	_, handled = hd.HandleMouse(10, 5, true, false)
	if !handled || hd.ScrollTop != 0 {
		t.Fatalf("expected ScrollTop 0 after WheelUp, got %d", hd.ScrollTop)
	}

	// Inside click does NOT close modal
	closed, handled := hd.HandleMouse(10, 5, false, false)
	if !handled || closed {
		t.Fatalf("expected click inside to be handled without closing, got closed=%v handled=%v", closed, handled)
	}

	// Esc key closes modal
	closed = hd.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !closed || hd.Active {
		t.Fatalf("expected Esc to close modal, got closed=%v active=%v", closed, hd.Active)
	}

	// Outside click closes modal
	hd.Open(24)
	closed, handled = hd.HandleMouse(-5, -5, false, false)
	if !handled || !closed || hd.Active {
		t.Fatalf("expected outside click to close modal, got closed=%v active=%v", closed, hd.Active)
	}
}
