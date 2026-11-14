package editor

import (
	"testing"
)

func TestDetectAutoCloseTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<div", "</div>"},
		{"<Button", "</Button>"},
		{"<custom-card", "</custom-card>"},
		{"<div className=\"container\"", "</div>"},
		{"<MyComp foo={123} bar=\"test\"", "</MyComp>"},
		{"<", "</>"}, // React Fragment
		{"</div", ""}, // already closing tag
		{"<img /", ""}, // self-closing tag
		{"<input", ""}, // void HTML element
		{"<br", ""},    // void HTML element
		{"a < b", ""},  // comparison operator
		{"x =", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := DetectAutoCloseTag(tt.input)
		if got != tt.expected {
			t.Errorf("DetectAutoCloseTag(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsAutoCloseTagSupported(t *testing.T) {
	if !IsAutoCloseTagSupported("App.jsx") {
		t.Errorf("expected App.jsx to be supported")
	}
	if !IsAutoCloseTagSupported("Component.tsx") {
		t.Errorf("expected Component.tsx to be supported")
	}
	if !IsAutoCloseTagSupported("index.html") {
		t.Errorf("expected index.html to be supported")
	}
	if IsAutoCloseTagSupported("main.go") {
		t.Errorf("expected main.go to NOT be supported")
	}
}
