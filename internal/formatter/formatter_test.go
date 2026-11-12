package formatter

import (
	"strings"
	"testing"
)

func TestDetectExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", ".go"},
		{"src/App.tsx", ".tsx"},
		{"resources/views/welcome.blade.php", ".blade.php"},
		{"UserController.php", ".php"},
		{"README.MD", ".md"},
		{"no_ext", ""},
	}

	for _, tt := range tests {
		got := DetectExtension(tt.path)
		if got != tt.expected {
			t.Errorf("DetectExtension(%q) = %q, expected %q", tt.path, got, tt.expected)
		}
	}
}

func TestGetFallbackChain(t *testing.T) {
	// React JSX
	chain := GetFallbackChain(".jsx", "test.jsx")
	if len(chain) == 0 {
		t.Fatalf("Expected fallback chain for .jsx, got empty")
	}
	hasBiome := false
	hasPrettier := false
	for _, c := range chain {
		if strings.Contains(c.Name, "biome") {
			hasBiome = true
		}
		if strings.Contains(c.Name, "prettier") {
			hasPrettier = true
		}
	}
	if !hasBiome || !hasPrettier {
		t.Errorf("Expected chain for .jsx to contain biome and prettier, got %+v", chain)
	}

	// PHP
	phpChain := GetFallbackChain(".php", "test.php")
	hasPint := false
	for _, c := range phpChain {
		if strings.Contains(c.Name, "pint") {
			hasPint = true
		}
	}
	if !hasPint {
		t.Errorf("Expected chain for .php to contain pint, got %+v", phpChain)
	}

	// Blade
	bladeChain := GetFallbackChain(".blade.php", "test.blade.php")
	if len(bladeChain) == 0 || bladeChain[0].Name != "blade-formatter" {
		t.Errorf("Expected first tool for blade to be blade-formatter, got %+v", bladeChain)
	}
}

func TestFormatGoWithGofmt(t *testing.T) {
	unformatted := "package main\n\nfunc main(){\nprintln(  \"hello world\"  )\n}\n"
	req := FormatRequest{
		Content:      unformatted,
		FilePath:     "test.go",
		WorkspaceDir: "",
	}

	res, err := Format(req)
	if err != nil {
		t.Fatalf("gofmt failed: %v", err)
	}

	if res.ToolName != "gofmt" {
		t.Errorf("Expected tool gofmt, got %s", res.ToolName)
	}

	expected := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	if res.Formatted != expected {
		t.Errorf("Formatted mismatch.\nGot:\n%q\nExpected:\n%q", res.Formatted, expected)
	}
}

func TestFormatMissingFormatter(t *testing.T) {
	req := FormatRequest{
		Content:      "foo bar",
		FilePath:     "test.sql",
		WorkspaceDir: "",
	}

	_, err := Format(req)
	// sql-formatter is likely not installed, so it should return descriptive error
	if err == nil {
		t.Logf("sql-formatter was installed in environment, which is fine")
	} else {
		if !strings.Contains(err.Error(), "formatter not found in PATH") {
			t.Errorf("Expected 'formatter not found in PATH' error, got %v", err)
		}
	}
}

func TestFormatCustomOverride(t *testing.T) {
	req := FormatRequest{
		Content:      "package main\n\nfunc main(){}\n",
		FilePath:     "main.go",
		WorkspaceDir: "",
		CustomOverrides: map[string][]string{
			".go": {"gofmt"},
		},
	}

	res, err := Format(req)
	if err != nil {
		t.Fatalf("Custom override failed: %v", err)
	}
	if res.ToolName != "gofmt" {
		t.Errorf("Expected tool gofmt, got %s", res.ToolName)
	}
}
