package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func writeViewFixture(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func groupFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	f1 := writeViewFixture(t, root, "a/one.go", "foo1\nalpha\nfoo3\n")
	f2 := writeViewFixture(t, root, "b/two.txt", "foo4\n")
	return root, f1, f2
}

func TestViewGroupsMatchesByFile(t *testing.T) {
	root, _, _ := groupFixture(t)
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()

	if sv.TotalFound != 3 {
		t.Errorf("Expected 3 matches, got %d", sv.TotalFound)
	}
	rendered := sv.Render(40, 24, true)
	if !strings.Contains(rendered, "a/one.go") || !strings.Contains(rendered, "(2)") {
		t.Errorf("Expected group header for a/one.go with (2), got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "b/two.txt") || !strings.Contains(rendered, "(1)") {
		t.Errorf("Expected group header for b/two.txt with (1), got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[Replace]") {
		t.Errorf("Expected per-file [Replace] button in render, got:\n%s", rendered)
	}
}

func TestViewTruncates500WithPlus(t *testing.T) {
	root := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("foo\n")
	}
	writeViewFixture(t, root, "many.txt", sb.String())
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()
	if len(sv.Matches) != 500 {
		t.Fatalf("Expected matches capped at 500, got %d", len(sv.Matches))
	}
	if !sv.Truncated {
		t.Fatal("Expected Truncated=true")
	}
	rendered := sv.Render(40, 24, true)
	if !strings.Contains(rendered, "500+") {
		t.Errorf("Expected footer to show 500+, got:\n%s", rendered)
	}
}

func TestViewToggles(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "x.txt", "foo FOO\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")

	sv.Update(tea.KeyPressMsg{Code: 'c', Text: "c", Mod: tea.ModAlt})
	if !sv.CaseSensitive {
		t.Error("Expected CaseSensitive after alt+c")
	}
	rendered := sv.Render(40, 24, true)
	if !strings.Contains(rendered, "[Aa]") {
		t.Errorf("Expected [Aa] badge in render, got:\n%s", rendered)
	}
	sv.Update(tea.KeyPressMsg{Code: 'c', Text: "c", Mod: tea.ModAlt})
	if sv.CaseSensitive {
		t.Error("Expected CaseSensitive off after second alt+c")
	}

	sv.Update(tea.KeyPressMsg{Code: 'w', Text: "w", Mod: tea.ModAlt})
	if !sv.WholeWord {
		t.Error("Expected WholeWord after alt+w")
	}
	sv.Update(tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModAlt})
	if !sv.Regex {
		t.Error("Expected Regex after alt+x")
	}
}

func TestViewInvalidRegexShowsHintNoPanic(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "x.txt", "foo\n")
	sv := NewSearchView(root)
	sv.Regex = true
	sv.FindInput.SetValue("(")
	sv.DoSearch() // must not panic
	if len(sv.Matches) != 0 {
		t.Fatalf("Expected 0 matches for invalid regex, got %d", len(sv.Matches))
	}
	if sv.ErrMsg == "" {
		t.Fatal("Expected an inline hint for invalid regex")
	}
	rendered := sv.Render(40, 24, true)
	if !strings.Contains(rendered, "regex") {
		t.Errorf("Expected invalid regex hint in render, got:\n%s", rendered)
	}
}

func TestViewSelectionAndEnterReturnsPath(t *testing.T) {
	root := t.TempDir()
	target := writeViewFixture(t, root, "one.go", "nope\nfoo here\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()

	sv.FocusField = FocusMatchList
	sv.Selected = 0
	filePath, line, col, _ := sv.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if filePath != target {
		t.Errorf("Expected %q, got %q", target, filePath)
	}
	if line != 2 {
		t.Errorf("Expected line 2, got %d", line)
	}
	if col != 1 {
		t.Errorf("Expected col 1, got %d", col)
	}
}

func TestViewStaleGenerationDiscarded(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "a.txt", "foo\n")
	sv := NewSearchView(root)
	sv.DoSearch() // generation = 1
	countBefore := len(sv.Matches)

	sv.HandleResults(ResultsMsg{Generation: 0, Matches: nil, Truncated: false})
	if len(sv.Matches) != countBefore {
		t.Fatal("Stale generation must be discarded")
	}
	sv.HandleResults(ResultsMsg{
		Generation: sv.generation,
		Matches:    []SearchMatch{{FilePath: "x", RelPath: "x", LineNumber: 3, ColNumber: 1}},
	})
	if len(sv.Matches) != 1 {
		t.Fatal("Fresh generation must be applied")
	}
}

func TestViewEmptyExcludeUsesDefaults(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, ".git/secret.txt", "foo\n")
	writeViewFixture(t, root, "keep.txt", "foo\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()
	for _, m := range sv.Matches {
		if strings.HasPrefix(m.RelPath, ".git") {
			t.Fatal("Default ignores must still apply with empty exclude")
		}
	}

	sv.ExcludeInput.SetValue("*.txt")
	sv.DoSearch()
	if len(sv.Matches) != 0 {
		t.Fatalf("Exclude *.txt must exclude all txt files, got %d matches", len(sv.Matches))
	}
}

func TestViewIncludeScopes(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "a/one.go", "foo\n")
	writeViewFixture(t, root, "b/two.txt", "foo\n")
	sv := NewSearchView(root)
	sv.PathInput.SetValue("**/*.go")
	sv.FindInput.SetValue("foo")
	sv.DoSearch()
	if len(sv.Matches) != 1 || sv.Matches[0].RelPath != "a/one.go" {
		t.Fatalf("Include **/*.go failed, got %+v", sv.Matches)
	}
}

func TestViewPerFileReplaceButtonHitTest(t *testing.T) {
	root, f1, _ := groupFixture(t)
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()

	// Render at a known size; list starts at row 9 (2+1+2+1+1+1+1).
	_ = sv.Render(40, 24, true)
	act := sv.HandleMouseClick(40-2, 9)
	if act.Kind != "replace-file" {
		t.Fatalf("Expected replace-file action on header [Replace], got %+v", act)
	}
	if act.FilePath != f1 && !strings.HasSuffix(act.FilePath, "one.go") {
		t.Errorf("Expected file path of first group, got %q", act.FilePath)
	}
}

func TestViewFindReplaceAllButtonHitTest(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "x.txt", "foo\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()
	_ = sv.Render(40, 24, true)
	act := sv.HandleMouseClick(2, 7)
	if act.Kind != "find" {
		t.Errorf("Expected find action on [Find] button, got %+v", act)
	}
	act = sv.HandleMouseClick(12, 7)
	if act.Kind != "replace-all" {
		t.Errorf("Expected replace-all action on [Replace All] button, got %+v", act)
	}
}

func TestViewToggleHitTest(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "x.txt", "foo\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("foo")
	sv.DoSearch()
	_ = sv.Render(40, 24, true)
	act := sv.HandleMouseClick(2, 2)
	if act.Kind != "toggle-case" {
		t.Errorf("Expected toggle-case at x=2,y=2, got %+v", act)
	}
}

func TestDoSearchWordStillWorks(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "w.txt", "word swords word\n")
	sv := NewSearchView(root)
	sv.DoSearchWord("word")
	if len(sv.Matches) != 2 {
		t.Fatalf("Expected 2 whole-word matches, got %d", len(sv.Matches))
	}
}

func TestSearchViewNavigation(t *testing.T) {
	root := t.TempDir()
	sv := NewSearchView(root)

	if sv.FocusField != FocusFindInput {
		t.Fatalf("Expected initial focus FocusFindInput")
	}

	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if sv.FocusField != FocusReplaceInput {
		t.Errorf("Expected FocusReplaceInput after Tab, got %v", sv.FocusField)
	}
	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if sv.FocusField != FocusPathInput {
		t.Errorf("Expected FocusPathInput after Tab, got %v", sv.FocusField)
	}
	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if sv.FocusField != FocusExcludeInput {
		t.Errorf("Expected FocusExcludeInput after Tab, got %v", sv.FocusField)
	}
	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if sv.FocusField != FocusMatchList {
		t.Errorf("Expected FocusMatchList after Tab, got %v", sv.FocusField)
	}
	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if sv.FocusField != FocusFindInput {
		t.Errorf("Expected wrap to FocusFindInput after Tab, got %v", sv.FocusField)
	}

	sv.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if sv.FocusField != FocusMatchList {
		t.Errorf("Expected Shift+Tab wrap to FocusMatchList, got %v", sv.FocusField)
	}
}

func TestSearchViewSearchOpensFile(t *testing.T) {
	root := t.TempDir()
	target := writeViewFixture(t, root, "hello.txt", "hello world\n")
	sv := NewSearchView(root)
	sv.FindInput.SetValue("hello")
	sv.DoSearch()
	sv.FocusField = FocusMatchList
	sv.Selected = 0
	filePath, line, _, _ := sv.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if filePath != target || line != 1 {
		t.Fatalf("Expected (%q, 1), got (%q, %d)", target, filePath, line)
	}
	_ = fmt.Sprintf("%d", sv.TotalFound)
}

func TestViewEmptyQueryIsNotAnError(t *testing.T) {
	root := t.TempDir()
	writeViewFixture(t, root, "x.txt", "foo\n")
	sv := NewSearchView(root)
	sv.Regex = true
	sv.FindInput.SetValue("") // initial state with regex toggled on
	sv.DoSearch()             // must not produce an error hint
	if sv.ErrMsg != "" {
		t.Fatalf("empty query must not set an error, got %q", sv.ErrMsg)
	}
	rendered := sv.Render(40, 24, true)
	if strings.Contains(rendered, "invalid regex") || strings.Contains(rendered, "ERR") {
		t.Errorf("initial state must be clean, got:\n%s", rendered)
	}
}
