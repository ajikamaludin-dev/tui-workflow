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

func TestParseLineStyles(t *testing.T) {
	// Simple plain text without ANSI
	plain := "hello"
	plainStyles := ParseLineStyles(plain)
	if len(plainStyles) != 5 {
		t.Fatalf("Expected 5 styles for 'hello', got %d", len(plainStyles))
	}
	for _, s := range plainStyles {
		if s != "" {
			t.Errorf("Expected empty style for plain text, got %q", s)
		}
	}

	// ANSI colored text: "\x1b[31mab\x1b[0m\x1b[32mcd\x1b[0m"
	ansiText := "\x1b[31mab\x1b[0m\x1b[32mcd\x1b[0m"
	styles := ParseLineStyles(ansiText)
	if len(styles) != 4 {
		t.Fatalf("Expected 4 styles, got %d", len(styles))
	}
	if styles[0] != "\x1b[31m" || styles[1] != "\x1b[31m" {
		t.Errorf("Expected styles[0..1] == '\\x1b[31m', got %q, %q", styles[0], styles[1])
	}
	if styles[2] != "\x1b[32m" || styles[3] != "\x1b[32m" {
		t.Errorf("Expected styles[2..3] == '\\x1b[32m', got %q, %q", styles[2], styles[3])
	}

	// Empty string
	if len(ParseLineStyles("")) != 0 {
		t.Errorf("Expected 0 styles for empty string")
	}
}

func TestHighlightCodeStyles(t *testing.T) {
	h := NewHighlighter()
	code := "package main\n\nfunc main() {}\n"
	styles := h.HighlightCodeStyles(code, "main.go")

	if len(styles) < 3 {
		t.Fatalf("Expected at least 3 lines of styles, got %d", len(styles))
	}

	// "package main" has 12 runes
	if len(styles[0]) != 12 {
		t.Errorf("Expected 12 rune styles on line 0, got %d", len(styles[0]))
	}

	// First rune of "package" should have keyword style
	if styles[0][0] == "" {
		t.Errorf("Expected non-empty style for keyword 'package'")
	}
}
