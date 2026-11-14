package search

import "testing"

func TestReplaceJobDescribeAll(t *testing.T) {
	matches := []SearchMatch{
		{FilePath: "/ws/a.go", RelPath: "a.go", LineNumber: 1, ColNumber: 1},
		{FilePath: "/ws/a.go", RelPath: "a.go", LineNumber: 2, ColNumber: 1},
		{FilePath: "/ws/b.go", RelPath: "b.go", LineNumber: 3, ColNumber: 1},
	}
	job := BuildReplaceJob(ReplaceRequest{Scope: ReplaceScopeAll}, Query{Pattern: "x"}, "y", matches)
	want := "Replace 3 occurrence(s) in 2 file(s)?"
	if got := job.Describe(); got != want {
		t.Errorf("Describe() = %q, want %q", got, want)
	}
	if got := job.Occurrences(); got != 3 {
		t.Errorf("Occurrences() = %d, want 3", got)
	}
}

func TestReplaceJobDescribeFile(t *testing.T) {
	matches := []SearchMatch{
		{FilePath: "/ws/a.go", RelPath: "a.go", LineNumber: 1, ColNumber: 1},
		{FilePath: "/ws/a.go", RelPath: "a.go", LineNumber: 9, ColNumber: 1},
		{FilePath: "/ws/b.go", RelPath: "b.go", LineNumber: 3, ColNumber: 1},
	}
	job := BuildReplaceJob(ReplaceRequest{Scope: ReplaceScopeFile, FilePath: "/ws/a.go"}, Query{Pattern: "x"}, "y", matches)
	want := "Replace 2 occurrence(s) in 'a.go'?"
	if got := job.Describe(); got != want {
		t.Errorf("Describe() = %q, want %q", got, want)
	}
	if len(job.Files()) != 1 || job.Files()[0] != "/ws/a.go" {
		t.Errorf("Files() = %v, want [/ws/a.go]", job.Files())
	}
}

func TestReplaceJobDescribeMatch(t *testing.T) {
	matches := []SearchMatch{
		{FilePath: "/ws/a.go", RelPath: "a.go", LineNumber: 4, ColNumber: 2},
	}
	job := BuildReplaceJob(ReplaceRequest{Scope: ReplaceScopeMatch, FilePath: "/ws/a.go", LineNumber: 4, ColNumber: 2}, Query{Pattern: "x"}, "y", matches)
	want := "Replace 1 occurrence in 'a.go'?"
	if got := job.Describe(); got != want {
		t.Errorf("Describe() = %q, want %q", got, want)
	}
}
