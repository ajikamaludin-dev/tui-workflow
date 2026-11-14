package search

import "testing"

func mustCompile(t *testing.T, q Query) *Matcher {
	t.Helper()
	m, err := Compile(q)
	if err != nil {
		t.Fatalf("Compile(%+v) failed: %v", q, err)
	}
	return m
}

func TestCompileEmptyPatternErrors(t *testing.T) {
	if _, err := Compile(Query{Pattern: ""}); err == nil {
		t.Fatal("expected error for empty pattern")
	}
	if _, err := Compile(Query{Pattern: "   "}); err == nil {
		t.Fatal("expected error for whitespace-only pattern")
	}
}

func TestCompileInvalidRegexErrors(t *testing.T) {
	if _, err := Compile(Query{Pattern: "(", Regex: true}); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLiteralCaseInsensitiveDefault(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "FOO"})
	got := m.FindAll([][]rune{[]rune("foo Foo fOO")})
	if len(got) != 3 {
		t.Fatalf("expected 3 CI matches, got %v", got)
	}
}

func TestLiteralCaseSensitive(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "FOO", CaseSensitive: true})
	got := m.FindAll([][]rune{[]rune("foo Foo FOO")})
	if len(got) != 1 || got[0].Col != 8 {
		t.Fatalf("expected 1 match at col 8, got %v", got)
	}
}

func TestWholeWord(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo", WholeWord: true})
	got := m.FindAll([][]rune{[]rune("a foo b foobar foo,")})
	if len(got) != 2 {
		t.Fatalf("expected 2 whole-word matches, got %v", got)
	}
	if got[0].Col != 2 || got[1].Col != 15 {
		t.Fatalf("wrong cols: %v", got)
	}
}

func TestWholeWordCaseInsensitive(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo", WholeWord: true})
	got := m.FindAll([][]rune{[]rune("FOO foobar")})
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %v", got)
	}
}

func TestRegexMode(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "f.o", Regex: true})
	got := m.FindAll([][]rune{[]rune("foo fxo f  o")})
	if len(got) != 2 {
		t.Fatalf("expected 2 regex matches, got %v", got)
	}
}

func TestRegexDigitCount(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "\\d+", Regex: true})
	got := m.FindAll([][]rune{[]rune("a12b3")})
	if len(got) != 2 || got[0].Col != 1 || got[1].Col != 4 {
		t.Fatalf("wrong digit matches: %v", got)
	}
}

func TestRegexInvalidFindAllReturnsNothing(t *testing.T) {
	// Zero Matcher must not panic and must not match.
	var m *Matcher
	got := m.FindAll([][]rune{[]rune("abc")})
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	var m2 Matcher
	got = m2.FindAll([][]rune{[]rune("abc")})
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestReplaceAllInTextCaseInsensitive(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo"})
	got, count := m.ReplaceAllInText("Foo fOO foo", "X")
	if got != "X X X" || count != 3 {
		t.Fatalf("got %q (%d), want %q (3)", got, count, "X X X")
	}
}

func TestReplaceAllInTextCaseInsensitiveSplice(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo"})
	got, count := m.ReplaceAllInText("aFooB fOOb", "[bar]")
	want := "a[bar]B [bar]b"
	if got != want || count != 2 {
		t.Fatalf("got %q (%d), want %q (2)", got, count, want)
	}
}

func TestReplaceAllInTextRegexDigit(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "\\d+", Regex: true})
	got, count := m.ReplaceAllInText("a12b3", "#")
	if got != "a#b#" || count != 2 {
		t.Fatalf("got %q (%d), want %q (2)", got, count, "a#b#")
	}
}

func TestReplaceAllInTextRegexCaptureGroup(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "(\\w)\\d", Regex: true})
	got, count := m.ReplaceAllInText("a12b3", "$1*")
	if got != "a*2b*" || count != 2 {
		t.Fatalf("got %q (%d), want %q (2)", got, count, "a*2b*")
	}
}

func TestReplaceAllInTextLiteralDollarSignNotExpanded(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo"})
	got, count := m.ReplaceAllInText("foo foo", "$1")
	if got != "$1 $1" || count != 2 {
		t.Fatalf("got %q (%d), want literal $1 (2)", got, count)
	}
}

func TestMultiByteColAndLength(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "llo"})
	got := m.FindAll([][]rune{[]rune("héllo")})
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %v", got)
	}
	// runes: h é l l o → "llo" starts at rune col 2, length 3
	if got[0].Col != 2 || got[0].Length != 3 {
		t.Fatalf("wrong multi-byte col/len: %v", got[0])
	}
}

func TestMultiByteJapanese(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "日本"})
	got := m.FindAll([][]rune{[]rune("ab日本語cd")})
	if len(got) != 1 || got[0].Col != 2 || got[0].Length != 2 {
		t.Fatalf("wrong Japanese match: %v", got)
	}
}

func TestFindAllRowsDistinct(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo"})
	lines := [][]rune{[]rune("foo"), []rune("bar"), []rune("foo foo")}
	got := m.FindAll(lines)
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %v", got)
	}
	if got[0].Row != 0 || got[1].Row != 2 || got[2].Row != 2 {
		t.Fatalf("wrong rows: %v", got)
	}
}

func TestFindInLineRowPassthrough(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "foo"})
	got := m.FindInLine([]rune("foo"), 7)
	if len(got) != 1 || got[0].Row != 7 {
		t.Fatalf("wrong row passthrough: %v", got)
	}
}
