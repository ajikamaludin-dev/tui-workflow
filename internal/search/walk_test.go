package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func searchFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "a/a.go", "foo1\nFOO2\n")
	writeFixture(t, root, "b/b.txt", "foo3\n")
	writeFixture(t, root, ".git/config", "foo_secret\n")
	// binary file with null byte in first 512 bytes
	p := filepath.Join(root, "a", "bin.bin")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("foo4\x00null"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func baseReq(root string) WalkRequest {
	return WalkRequest{Root: root, Query: Query{Pattern: "foo"}, MaxMatches: 0}
}

func TestWalkSyncFindsAllMatchesCaseInsensitive(t *testing.T) {
	root := searchFixture(t)
	res := WalkSync(baseReq(root))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	var b strings.Builder
	for _, m := range res.Matches {
		b.WriteString(m.RelPath)
		b.WriteString("\n")
	}
	if !strings.Contains(b.String(), "a/a.go") || !strings.Contains(b.String(), "b/b.txt") {
		t.Fatalf("expected a/a.go and b/b.txt in results, got:\n%s", b.String())
	}
	// .git must be hidden
	if strings.Contains(b.String(), ".git") {
		t.Fatalf(".git must be skipped, got:\n%s", b.String())
	}
	// binary must be skipped
	if strings.Contains(b.String(), "bin.bin") {
		t.Fatalf("binary file must be skipped, got:\n%s", b.String())
	}
}

func TestWalkSyncCountsAllMatchesPerLine(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "multi.go", "foo foo\n")
	res := WalkSync(baseReq(root))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	n := 0
	for _, m := range res.Matches {
		if m.RelPath == "multi.go" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 matches in multi.go, got %d", n)
	}
}

func TestWalkSyncIncludeGlob(t *testing.T) {
	root := searchFixture(t)
	req := baseReq(root)
	req.Include = []string{"**/*.go"}
	res := WalkSync(req)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, m := range res.Matches {
		if m.RelPath != "a/a.go" {
			t.Fatalf("include **/*.go leaked %q", m.RelPath)
		}
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected matches in a/a.go")
	}
}

func TestWalkSyncIncludePrefix(t *testing.T) {
	root := searchFixture(t)
	req := baseReq(root)
	req.Include = []string{"a/"}
	res := WalkSync(req)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, m := range res.Matches {
		if !strings.HasPrefix(m.RelPath, "a/") {
			t.Fatalf("include a/ leaked %q", m.RelPath)
		}
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected matches under a/")
	}
}

func TestWalkSyncExcludeGlob(t *testing.T) {
	root := searchFixture(t)
	req := baseReq(root)
	req.Exclude = []string{"*.txt"}
	res := WalkSync(req)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, m := range res.Matches {
		if m.RelPath == "b/b.txt" {
			t.Fatal("exclude *.txt failed")
		}
	}
}

func TestWalkSyncEmptyExcludeKeepsDefaults(t *testing.T) {
	root := searchFixture(t)
	res := WalkSync(baseReq(root))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, m := range res.Matches {
		if strings.HasPrefix(m.RelPath, ".git") {
			t.Fatal("default ignore of .git failed")
		}
	}
}

func TestWalkSyncTruncatesAtMaxMatches(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "many.txt", "foo foo foo\nfoo foo foo\n")
	req := baseReq(root)
	req.MaxMatches = 2
	res := WalkSync(req)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if !res.Truncated {
		t.Fatal("expected Truncated=true")
	}
	if len(res.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(res.Matches))
	}
}

func TestWalkSyncRelPathUsesSlash(t *testing.T) {
	root := searchFixture(t)
	res := WalkSync(baseReq(root))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, m := range res.Matches {
		if strings.ContainsRune(m.RelPath, '\\') {
			t.Fatalf("rel path must use slashes: %q", m.RelPath)
		}
		if filepath.IsAbs(m.RelPath) {
			t.Fatalf("rel path must be relative: %q", m.RelPath)
		}
	}
}

func TestWalkSyncIsDeterministic(t *testing.T) {
	root := searchFixture(t)
	a := WalkSync(baseReq(root))
	b := WalkSync(baseReq(root))
	if a.Err != nil || b.Err != nil {
		t.Fatal(a.Err, b.Err)
	}
	if len(a.Matches) != len(b.Matches) {
		t.Fatal("match count differs between identical walks")
	}
	for i := range a.Matches {
		if a.Matches[i] != b.Matches[i] {
			t.Fatalf("match %d differs between walks", i)
		}
	}
}

func TestWalkSyncEmptyPattern(t *testing.T) {
	root := searchFixture(t)
	req := baseReq(root)
	req.Query.Pattern = ""
	res := WalkSync(req)
	if res.Err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestWalkSyncEmptyRoot(t *testing.T) {
	res := WalkSync(WalkRequest{Root: "", Query: Query{Pattern: "foo"}})
	if res.Err == nil {
		t.Fatal("expected error for empty root")
	}
}
