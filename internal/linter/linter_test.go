package linter

import (
	"strings"
	"testing"
)

func TestParseESLintJSON(t *testing.T) {
	raw := `[
		{
			"filePath": "src/App.jsx",
			"messages": [
				{
					"ruleId": "react-hooks/rules-of-hooks",
					"severity": 2,
					"message": "React Hook useState is called in function that is neither a React component",
					"line": 12,
					"column": 5
				},
				{
					"ruleId": "no-unused-vars",
					"severity": 1,
					"message": "'data' is defined but never used.",
					"line": 18,
					"column": 11
				}
			]
		}
	]`

	diags := ParseESLintJSON([]byte(raw), "src/App.jsx")
	if len(diags) != 2 {
		t.Fatalf("ParseESLintJSON want 2 diagnostics, got %d", len(diags))
	}

	d1 := diags[0]
	if d1.Line != 12 || d1.Severity != SeverityError || d1.RuleID != "react-hooks/rules-of-hooks" {
		t.Errorf("d1 unexpected: %+v", d1)
	}

	d2 := diags[1]
	if d2.Line != 18 || d2.Severity != SeverityWarning || d2.RuleID != "no-unused-vars" {
		t.Errorf("d2 unexpected: %+v", d2)
	}
}

func TestStore(t *testing.T) {
	store := NewStore()
	diags := []Diagnostic{
		{FilePath: "test.js", Line: 5, Severity: SeverityError, Message: "syntax error"},
		{FilePath: "test.js", Line: 10, Severity: SeverityWarning, Message: "unused variable"},
	}

	store.Set("test.js", diags)

	got := store.Get("test.js")
	if len(got) != 2 {
		t.Fatalf("store.Get('test.js') want 2, got %d", len(got))
	}

	lineMap := store.LineDiagnostics("test.js")
	if len(lineMap) != 2 {
		t.Errorf("LineDiagnostics want 2, got %d", len(lineMap))
	}
	// 5-1 = line 4 (0-based) should have red dot
	if !strings.Contains(lineMap[4], "●") {
		t.Errorf("lineMap[4] want red ● marker, got %s", lineMap[4])
	}
	// 10-1 = line 9 (0-based) should have yellow triangle
	if !strings.Contains(lineMap[9], "▲") {
		t.Errorf("lineMap[9] want yellow ▲ marker, got %s", lineMap[9])
	}

	atLine5 := store.DiagnosticAt("test.js", 5)
	if atLine5 == nil || atLine5.Message != "syntax error" {
		t.Errorf("DiagnosticAt(5) want 'syntax error', got %+v", atLine5)
	}
}

func TestParseBiomeJSON(t *testing.T) {
	raw := `{
		"summary": {
			"diagnostics": 2
		},
		"diagnostics": [
			{
				"category": "lint/suspicious/noExplicitAny",
				"severity": "error",
				"description": "Unexpected any. Specify a different type.",
				"location": {
					"path": {
						"file": "src/types.ts"
					},
					"start": {
						"line": 15,
						"column": 8
					}
				}
			},
			{
				"category": "lint/style/useConst",
				"severity": "warn",
				"description": "This variable is let, use const instead.",
				"location": {
					"start": {
						"line": 22,
						"column": 5
					}
				}
			}
		]
	}`

	diags := ParseBiomeJSON([]byte(raw), "src/types.ts")
	if len(diags) != 2 {
		t.Fatalf("ParseBiomeJSON want 2 diagnostics, got %d", len(diags))
	}

	d1 := diags[0]
	if d1.Line != 15 || d1.Col != 8 || d1.Severity != SeverityError || d1.RuleID != "lint/suspicious/noExplicitAny" {
		t.Errorf("d1 unexpected: %+v", d1)
	}
	if d1.Source != "biome" || d1.FilePath != "src/types.ts" {
		t.Errorf("d1 source/filePath unexpected: %+v", d1)
	}

	d2 := diags[1]
	if d2.Line != 22 || d2.Col != 5 || d2.Severity != SeverityWarning || d2.RuleID != "lint/style/useConst" {
		t.Errorf("d2 unexpected: %+v", d2)
	}
}
