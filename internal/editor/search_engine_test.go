package editor

import (
	"strings"
	"testing"

	"tui-workflow/internal/search"
)

func TestMatchAtOnLargeBuffer(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteString("foo line\n")
	}
	b := NewBuffer(sb.String())
	b.SetSearchQuery("foo")
	if len(b.SearchMatches) != 2000 {
		t.Fatalf("expected 2000 matches, got %d", len(b.SearchMatches))
	}
	if len(b.matchByLine) != 2000 {
		t.Fatalf("expected matchByLine to cover 2000 rows, got %d", len(b.matchByLine))
	}
	isMatch, _ := b.MatchAt(1999, 0)
	if !isMatch {
		t.Fatal("expected match at (1999, 0)")
	}
	isMatch, _ = b.MatchAt(1999, 3)
	if isMatch {
		t.Fatal("col 3 must not be part of the match")
	}
	isMatch, isActive := b.MatchAt(0, 0)
	if !isMatch {
		t.Fatal("expected match at (0, 0)")
	}
	// After SetSearchQuery the nearest match at/after cursor becomes active.
	_ = isActive
}

func TestReplaceMatchAtReplacesExactMatch(t *testing.T) {
	b := NewBuffer("foo bar\nfoo")
	b.SetSearchQuery("foo")
	if len(b.SearchMatches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(b.SearchMatches))
	}
	if !b.ReplaceMatchAt(0, 0, "X") {
		t.Fatal("ReplaceMatchAt returned false")
	}
	if got := string(b.Lines[0]); got != "X bar" {
		t.Fatalf("line 0 = %q, want %q", got, "X bar")
	}
	if got := string(b.Lines[1]); got != "foo" {
		t.Fatalf("line 1 = %q, want %q", got, "foo")
	}
	if !b.IsDirty {
		t.Fatal("buffer must be dirty after replace")
	}
	// one undo restores original line
	b.Undo()
	if got := string(b.Lines[0]); got != "foo bar" {
		t.Fatalf("after undo line 0 = %q, want %q", got, "foo bar")
	}
}

func TestReplaceMatchAtNoMatchReturnsFalse(t *testing.T) {
	b := NewBuffer("foo bar")
	b.SetSearchQuery("foo")
	if b.ReplaceMatchAt(0, 4, "X") {
		t.Fatal("expected false for (row, col) without a match")
	}
}

func TestReplaceAllCountsOccurrencesCaseInsensitive(t *testing.T) {
	b := NewBuffer("Foo\nFOO\nfoo")
	n := b.ReplaceAll("foo", "A")
	if n != 3 {
		t.Fatalf("expected 3 occurrences, got %d", n)
	}
	for i, line := range b.Lines {
		if string(line) != "A" {
			t.Fatalf("line %d = %q, want %q", i, string(line), "A")
		}
	}
}

func TestReplaceMatchAtMultipleMatchesSameLine(t *testing.T) {
	b := NewBuffer("foo foo foo")
	b.SetSearchQuery("foo")
	if !b.ReplaceMatchAt(0, 8, "Z") {
		t.Fatal("ReplaceMatchAt returned false")
	}
	if got := string(b.Lines[0]); got != "foo foo Z" {
		t.Fatalf("line = %q, want %q", got, "foo foo Z")
	}
}

func TestReplaceAllRegexCaptureGroup(t *testing.T) {
	b := NewBuffer("a12b3")
	n := b.ReplaceAllOptions(search.Query{Pattern: `(\w)\d`, Regex: true}, "$1*")
	if n != 2 {
		t.Fatalf("expected 2 replacements, got %d", n)
	}
	if got := string(b.Lines[0]); got != "a*2b*" {
		t.Fatalf("line = %q, want %q", got, "a*2b*")
	}
}

func TestReplaceAllLiteralDollarVerbatim(t *testing.T) {
	b := NewBuffer("a12 a12")
	n := b.ReplaceAllOptions(search.Query{Pattern: "a12"}, "$1")
	if n != 2 {
		t.Fatalf("expected 2 replacements, got %d", n)
	}
	if got := string(b.Lines[0]); got != "$1 $1" {
		t.Fatalf("line = %q, want %q", got, "$1 $1")
	}
}
