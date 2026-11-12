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
		{"module.mjs", "JavaScript"},
		{"component.jsx", "JSX"},
		{"index.ts", "TypeScript"},
		{"app.tsx", "TSX"},
		{"index.php", "PHP"},
		{"main.dart", "Dart"},
		{"page.html", "HTML"},
		{"page.htm", "HTML"},
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

	languagesToTest := []struct {
		file string
		code string
	}{
		{"index.php", "<?php echo 'hello'; ?>"},
		{"main.dart", "void main() { print('hello'); }"},
		{"app.jsx", "const App = () => <div>Hello</div>;"},
		{"app.tsx", "const App: React.FC = () => <div>Hello</div>;"},
		{"index.html", "<!DOCTYPE html><html><body><h1>Hello</h1></body></html>"},
		{"index.js", "const a = 123;"},
		{"main.go", "package main\nfunc main() {}"},
	}

	for _, tc := range languagesToTest {
		lines := h.HighlightCode(tc.code, tc.file)
		if len(lines) == 0 {
			t.Errorf("HighlightCode returned empty lines for %s", tc.file)
		}
	}
}
