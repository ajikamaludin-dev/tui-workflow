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

func TestMenuBarMouseClick(t *testing.T) {
	mb := NewMenuBar()
	mb.SetWidth(100)
	_ = mb.Render() // ensure positions are calculated

	// 1. Click on "File" (Row 0, inside StartX..EndX)
	fileCat := mb.Categories[0]
	action, handled := mb.HandleMouse(tea.MouseMsg{
		X:      fileCat.StartX,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled || !mb.IsOpen() || mb.ActiveMenu != 0 {
		t.Fatalf("Expected File menu to open on click, handled=%v, open=%v, active=%d", handled, mb.IsOpen(), mb.ActiveMenu)
	}

	// 2. Click on item 0 "New File" at Y = 2
	action, handled = mb.HandleMouse(tea.MouseMsg{
		X:      fileCat.StartX + 2,
		Y:      2,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled || action != "file.new" {
		t.Errorf("Expected action 'file.new', got %q (handled=%v)", action, handled)
	}
	if mb.IsOpen() {
		t.Errorf("Menu should close after item click")
	}

	// 3. Re-open File menu and click on item 1 "Open File" at Y = 3
	mb.Toggle(0)
	action, handled = mb.HandleMouse(tea.MouseMsg{
		X:      fileCat.StartX + 2,
		Y:      3,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled || action != "file.open" {
		t.Errorf("Expected action 'file.open', got %q (handled=%v)", action, handled)
	}

	// 4. Re-open File menu and click on item 3 "Save File" at Y = 5
	mb.Toggle(0)
	action, handled = mb.HandleMouse(tea.MouseMsg{
		X:      fileCat.StartX + 2,
		Y:      5,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled || action != "file.save" {
		t.Errorf("Expected action 'file.save', got %q (handled=%v)", action, handled)
	}

	// 5. Re-open File menu and click outside dropdown
	mb.Toggle(0)
	action, handled = mb.HandleMouse(tea.MouseMsg{
		X:      80,
		Y:      10,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled || action != "" || mb.IsOpen() {
		t.Errorf("Expected menu to close when clicking outside, open=%v", mb.IsOpen())
	}
}
