package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFoldGutterClick(t *testing.T) {
	src := `func main() {
    x := 1
    y := 2
}
func other() {
    z := 3
}
`
	m, err := NewModelWithPath(".", false)
	if err != nil {
		t.Fatal(err)
	}
	m.Width = 80
	m.Height = 24
	m.SidebarVisible = false // editor starts at 0

	tmpFile := filepath.Join(t.TempDir(), "test.go")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = m.TabManager.OpenFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		t.Fatal("no active buffer tab")
	}

	// In row 2 (which is visual row 0 of editor buffer):
	// line 0: "func main() {"
	// It is foldable!
	if _, ok := tab.Buffer.FoldableAt(0); !ok {
		t.Fatal("line 0 should be foldable")
	}

	// Click on gutter at Y = 2 (first line of editor), X = 0 (marker column)
	updated, _ := m.Update(tea.MouseClickMsg{X: 0, Y: 2, Button: tea.MouseLeft})
	m = updated.(Model)

	tab = m.TabManager.ActiveTab()
	if !tab.Buffer.IsFoldedAt(0) {
		t.Errorf("line 0 should be folded after clicking marker at X=0, Y=2. Folded: %v", tab.Buffer.Folded)
	}

	// Click again on gutter at Y = 2 to unfold, but this time at X = 2 (line number digit in gutter)
	updated, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	m = updated.(Model)

	if tab.Buffer.IsFoldedAt(0) {
		t.Errorf("line 0 should be unfolded after clicking gutter at X=2. Folded: %v", tab.Buffer.Folded)
	}

	// Test with sidebar visible (editorStartX = 28)
	m.SidebarVisible = true
	m.SidebarWidth = 28

	// Click on gutter with sidebar offset: X = 28 (marker col)
	updated, _ = m.Update(tea.MouseClickMsg{X: 28, Y: 2, Button: tea.MouseLeft})
	m = updated.(Model)

	if !tab.Buffer.IsFoldedAt(0) {
		t.Errorf("line 0 should be folded after clicking gutter with sidebar offset at X=28. Folded: %v", tab.Buffer.Folded)
	}

	// Click on fold badge (past end of text in line 0): X = 28 + gutterW + 20
	gutterW := tab.Buffer.GutterWidth()
	updated, _ = m.Update(tea.MouseClickMsg{X: 28 + gutterW + 20, Y: 2, Button: tea.MouseLeft})
	m = updated.(Model)

	if tab.Buffer.IsFoldedAt(0) {
		t.Errorf("line 0 should be unfolded after clicking fold badge area. Folded: %v", tab.Buffer.Folded)
	}
}
