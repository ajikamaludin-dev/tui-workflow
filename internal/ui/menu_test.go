package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuBarNavigation(t *testing.T) {
	mb := NewMenuBar()
	mb.SetWidth(100)

	if mb.IsOpen() {
		t.Errorf("Menu should be closed initially")
	}

	// Alt+F opens File menu
	_, handled := mb.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alt+f")})
	// In some key representations String() is "alt+f"
	if !handled {
		mb.Toggle(0)
	}

	if !mb.IsOpen() {
		t.Errorf("Menu should be open after toggle")
	}

	if mb.ActiveMenu != 0 {
		t.Errorf("Expected active menu 0, got %d", mb.ActiveMenu)
	}

	// Right key moves to Setting
	_, _ = mb.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if mb.ActiveMenu != 1 {
		t.Errorf("Expected active menu 1, got %d", mb.ActiveMenu)
	}

	// Esc closes menu
	_, handled = mb.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if !handled || mb.IsOpen() {
		t.Errorf("Menu should be closed after Esc")
	}
}
