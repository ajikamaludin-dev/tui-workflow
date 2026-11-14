package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFindBarOpenModesAndMatches(t *testing.T) {
	fb := NewFindBar()
	if fb.Active {
		t.Fatalf("Expected FindBar to start inactive")
	}

	// 1. Open Find mode
	fb.Open("main")
	if !fb.Active || fb.IsReplace {
		t.Fatalf("Expected FindBar to be active in search mode (IsReplace=false)")
	}
	if fb.Input.Value() != "main" {
		t.Errorf("Expected initial value 'main', got %q", fb.Input.Value())
	}

	// 2. Set matches
	fb.SetMatches(1, 5)
	if fb.MatchCurrent != 1 || fb.MatchTotal != 5 {
		t.Errorf("Expected matches (1, 5), got (%d, %d)", fb.MatchCurrent, fb.MatchTotal)
	}

	rendered := fb.Render()
	if !strings.Contains(rendered, "1/5") {
		t.Errorf("Expected rendered findbar to contain '1/5', got %s", rendered)
	}

	// 3. Open Replace mode
	fb.OpenReplace("foo")
	if !fb.Active || !fb.IsReplace {
		t.Fatalf("Expected FindBar to be in replace mode")
	}
	renderedReplace := fb.Render()
	if !strings.Contains(renderedReplace, "Replace") {
		t.Errorf("Expected replace prompt in rendered view")
	}

	// 4. Close
	fb.Close()
	if fb.Active {
		t.Errorf("Expected FindBar to be inactive after Close()")
	}
}

func TestFindBarKeyNavigation(t *testing.T) {
	fb := NewFindBar()
	fb.Open("test")

	// Enter triggers next
	action, closed, _ := fb.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if closed || action != "next" {
		t.Errorf("Expected action 'next', got %q (closed=%v)", action, closed)
	}

	// Shift+Enter / up triggers prev
	action, closed, _ = fb.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if closed || action != "prev" {
		t.Errorf("Expected action 'prev', got %q", action)
	}

	// Esc triggers close
	action, closed, _ = fb.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !closed || action != "close" || fb.Active {
		t.Errorf("Expected close on Esc, got action=%q closed=%v", action, closed)
	}
}

func TestFindBarReplaceKeyNavigation(t *testing.T) {
	fb := NewFindBar()
	fb.OpenReplace("query")

	// Switch focus to replace input via Tab
	action, closed, _ := fb.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if closed || action != "" || fb.FocusField != FindFocusReplace {
		t.Errorf("Expected focus switched to FindFocusReplace on Tab")
	}

	// Switch focus back via Tab
	fb.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if fb.FocusField != FindFocusInput {
		t.Errorf("Expected focus switched back to FindFocusInput on Tab")
	}

	// Switch focus to replace to test replace_one on Enter or Alt+R
	fb.FocusField = FindFocusReplace

	// Alt+R / Ctrl+R triggers replace_one
	action, _, _ = fb.Update(tea.KeyPressMsg{Code: 'r', Text: "r", Mod: tea.ModAlt})
	if action != "replace_one" {
		action, _, _ = fb.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	}
	if action != "replace_one" {
		t.Errorf("Expected action 'replace_one', got %q", action)
	}

	// Alt+Enter triggers replace_all
	action, _, _ = fb.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if action != "replace_all" {
		t.Errorf("Expected action 'replace_all' on Alt+Enter, got %q", action)
	}
}

func TestFindBarRenderDimensions(t *testing.T) {
	fb := NewFindBar()
	fb.OpenReplace("test")
	fb.SetMatches(2, 8)

	rendered := fb.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Errorf("Expected multiline render for replace mode, got %d lines", len(lines))
	}
	if !strings.Contains(rendered, "2/8") {
		t.Errorf("Expected match count in rendered output")
	}
}

func TestFindBarToggles(t *testing.T) {
	fb := NewFindBar()
	fb.Open("test")

	action, closed, _ := fb.Update(tea.KeyPressMsg{Code: 'c', Text: "c", Mod: tea.ModAlt})
	if closed || action != "change" || !fb.QueryCase {
		t.Errorf("Expected QueryCase=true and action 'change', got action=%q closed=%v case=%v", action, closed, fb.QueryCase)
	}

	action, _, _ = fb.Update(tea.KeyPressMsg{Code: 'w', Text: "w", Mod: tea.ModAlt})
	if action != "change" || !fb.QueryWholeWord {
		t.Errorf("Expected QueryWholeWord=true and action 'change', got action=%q word=%v", action, fb.QueryWholeWord)
	}

	action, _, _ = fb.Update(tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModAlt})
	if action != "change" || !fb.QueryRegex {
		t.Errorf("Expected QueryRegex=true and action 'change', got action=%q regex=%v", action, fb.QueryRegex)
	}

	// toggle back off
	fb.Update(tea.KeyPressMsg{Code: 'c', Text: "c", Mod: tea.ModAlt})
	if fb.QueryCase {
		t.Errorf("Expected QueryCase=false after second alt+c")
	}

	rendered := fb.Render()
	for _, badge := range []string{"[Aa]", "[ab|]", "[.*]"} {
		if !strings.Contains(rendered, badge) {
			t.Errorf("Expected rendered findbar to contain badge %q, got %s", badge, rendered)
		}
	}
}
