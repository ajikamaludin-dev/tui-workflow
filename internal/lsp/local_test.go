package lsp

import (
	"testing"

	"tui-workflow/internal/editor"
)

func TestLocalCompletions(t *testing.T) {
	buf := editor.NewBuffer("const myCustomVariable = 42;\nfunction handleCustomClick() {}\n")

	items := BuildLocalCompletions(buf, "test.jsx", nil)
	if len(items) == 0 {
		t.Fatalf("Expected non-empty local completions")
	}

	// Verify React snippets are included for .jsx
	hasRafce := false
	hasUseState := false
	for _, it := range items {
		if it.Label == "rafce" && it.Kind == CompletionKindSnippet {
			hasRafce = true
		}
		if it.Label == "useState" && it.Kind == CompletionKindSnippet {
			hasUseState = true
		}
	}
	if !hasRafce {
		t.Errorf("Expected 'rafce' snippet in local completions for .jsx")
	}
	if !hasUseState {
		t.Errorf("Expected 'useState' snippet in local completions for .jsx")
	}

	// Test Filtering
	filtered := FilterCompletions(items, "raf")
	if len(filtered) == 0 || filtered[0].Label != "rafce" {
		t.Errorf("Expected 'rafce' as first match for prefix 'raf', got %+v", filtered)
	}

	// Test Buffer Word completion
	varFiltered := FilterCompletions(items, "myCust")
	if len(varFiltered) == 0 || varFiltered[0].Label != "myCustomVariable" {
		t.Errorf("Expected 'myCustomVariable' for prefix 'myCust', got %+v", varFiltered)
	}
}
