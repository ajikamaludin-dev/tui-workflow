package syntax

import (
	"testing"
)

func TestHighlighterDetection(t *testing.T) {
	h := NewHighlighter()

	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "Go"},
		{"README.md", "Markdown"},
		{"config.json", "JSON"},
		{"deploy.yaml", "YAML"},
		{"script.sh", "Shell"},
		{"app.py", "Python"},
		{"index.js", "JavaScript"},
		{"index.ts", "TypeScript"},
		{"page.html", "HTML"},
		{"style.css", "CSS"},
		{"query.sql", "SQL"},
		{"lib.rs", "Rust"},
		{"main.c", "C"},
		{"main.cpp", "C++"},
		{"unknown.xyz", "Plain Text"},
	}

	for _, tt := range tests {
		got := h.DetectLanguage(tt.path)
		if got != tt.expected {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestHighlighterHighlightCode(t *testing.T) {
	h := NewHighlighter()

	lines := h.HighlightCode("package main\n\nfunc main() {}\n", "main.go")
	if len(lines) == 0 {
		t.Fatalf("Expected non-empty highlighted lines")
	}

	emptyLines := h.HighlightCode("", "main.go")
	if len(emptyLines) != 1 || emptyLines[0] != "" {
		t.Errorf("Expected 1 empty line for empty code, got: %v", emptyLines)
	}
}
