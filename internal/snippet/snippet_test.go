package snippet

import (
	"strings"
	"testing"
)

func TestFindSnippet(t *testing.T) {
	s := FindSnippet("rafce", "App.jsx")
	if s == nil {
		t.Fatalf("FindSnippet('rafce', 'App.jsx') want non-nil, got nil")
	}
	if s.Prefix != "rafce" {
		t.Errorf("Snippet.Prefix = %s, want rafce", s.Prefix)
	}

	// Should not match on .go file
	sGo := FindSnippet("rafce", "main.go")
	if sGo != nil {
		t.Errorf("FindSnippet('rafce', 'main.go') want nil, got %v", sGo)
	}
}

func TestExpandSnippet(t *testing.T) {
	s := FindSnippet("rafce", "UserProfile.jsx")
	if s == nil {
		t.Fatalf("rafce snippet not found")
	}

	expanded := Expand(s, "UserProfile.jsx")
	if !strings.Contains(expanded.Text, "export const UserProfile = () => {") {
		t.Errorf("Expanded text does not contain component name 'UserProfile', got:\n%s", expanded.Text)
	}

	if len(expanded.TabStops) == 0 {
		t.Fatalf("Expected tabstops, got 0")
	}

	// TabStop 1 should be UserProfile
	ts1 := expanded.TabStops[0]
	if ts1.Index != 1 || ts1.Placeholder != "UserProfile" {
		t.Errorf("Tabstop 1 want index 1 placeholder 'UserProfile', got %+v", ts1)
	}

	// Last tabstop should be index 0 ($0)
	lastTs := expanded.TabStops[len(expanded.TabStops)-1]
	if lastTs.Index != 0 {
		t.Errorf("Last tabstop want index 0, got %d", lastTs.Index)
	}
}

func TestSnippetSession(t *testing.T) {
	s := FindSnippet("rafce", "Button.tsx")
	expanded := Expand(s, "Button.tsx")

	session := NewSession(10, 0, expanded.TabStops)
	if !session.Active {
		t.Fatalf("Session should be active")
	}

	row, col, length, ok := session.Current()
	if !ok || row != 12 || length != len("Button") {
		t.Errorf("Current tabstop want (12, col, %d), got (%d, %d, %d)", len("Button"), row, col, length)
	}

	// Advance to Tabstop 2 (${2:content})
	row2, col2, _, ok2 := session.Next()
	if !ok2 || row2 != 15 || col2 != 6 {
		t.Errorf("Next tabstop want (15, 6), got (%d, %d)", row2, col2)
	}

	// Advance to Tabstop 0 ($0)
	row3, col3, _, ok3 := session.Next()
	if !ok3 || row3 != 15 || col3 != 13 {
		t.Errorf("Next tabstop ($0) want (15, 13), got (%d, %d)", row3, col3)
	}

	// Wrap around to Tabstop 1
	row4, _, _, ok4 := session.Next()
	if !ok4 || row4 != 12 {
		t.Errorf("Next tabstop after wrap want row 12, got %d", row4)
	}

	// Prev back to Tabstop 0 ($0)
	row5, _, _, ok5 := session.Prev()
	if !ok5 || row5 != 15 {
		t.Errorf("Prev tabstop after wrap want row 15, got %d", row5)
	}

	// Cancel session
	session.Cancel()
	if session.Active {
		t.Errorf("Session should be inactive after Cancel()")
	}
}
