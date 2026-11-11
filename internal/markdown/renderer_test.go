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
}
