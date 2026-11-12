package markdown

import (
	"testing"
)

func TestMarkdownRenderer(t *testing.T) {
	mv := NewMarkdownViewer()

	mdText := "# Judul Dokumen\n\nIni adalah teks **tebal** dan *miring*.\n\n- Item 1\n- Item 2"

	// Initial render
	out1 := mv.Render(mdText, 80, 20)
	if len(out1) == 0 {
		t.Fatalf("Expected non-empty rendered markdown")
	}

	if len(mv.cachedLines) == 0 {
		t.Fatalf("Expected cached lines to be populated")
	}

	// Verify caching on identical content
	oldKey := mv.cachedKey
	out2 := mv.Render(mdText, 80, 20)
	if mv.cachedKey != oldKey {
		t.Errorf("Cache key changed unexpectedly: %s vs %s", oldKey, mv.cachedKey)
	}
	if out1 != out2 {
		t.Errorf("Output differs on cached render")
	}

	// Scrolling
	mv.ScrollDown(5, 10)
	if mv.ScrollOffset == 0 && len(mv.cachedLines) > 10 {
		t.Errorf("Expected ScrollOffset to increase")
	}

	mv.ScrollUp(2)
	mv.ScrollHome()
	if mv.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset to be 0 after ScrollHome")
	}

	// Selection & Copy Test
	mv.SelectAll()
	if !mv.HasSelection {
		t.Errorf("Expected HasSelection to be true after SelectAll")
	}
	allText := mv.GetSelectedText()
	if len(allText) == 0 {
		t.Errorf("Expected non-empty text from GetSelectedText")
	}

	// Partial selection on a line with content
	targetLine := -1
	for idx, l := range mv.cachedLines {
		if len(l) > 5 {
			targetLine = idx
			break
		}
	}
	if targetLine >= 0 {
		mv.StartSelection(targetLine, 0)
		mv.UpdateSelection(targetLine, 5)
		selPart := mv.GetSelectedText()
		if len(selPart) == 0 {
			t.Errorf("Expected partial selected text on line %d", targetLine)
		}
	} else {
		t.Errorf("No cached line with >5 chars found")
	}

	mv.ClearSelection()
	if mv.HasSelection {
		t.Errorf("Expected HasSelection to be false after ClearSelection")
	}
}
