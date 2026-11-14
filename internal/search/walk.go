package search

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"tui-workflow/internal/ignore"
)

// SearchMatch represents a single search match in a workspace file.
type SearchMatch struct {
	FilePath   string
	RelPath    string
	LineNumber int // 1-based
	ColNumber  int // 1-based
	LineText   string
}

// WalkRequest describes a workspace search.
type WalkRequest struct {
	Root       string
	Include    []string // gitignore-style globs; empty = every file passes
	Exclude    []string // gitignore-style globs; empty = built-in defaults only
	Query      Query
	MaxMatches int // <= 0 means DefaultMaxMatches
}

// WalkResult is the outcome of a workspace walk.
type WalkResult struct {
	Matches   []SearchMatch
	Truncated bool
	Err       error
}

// DefaultMaxMatches is the hard cap for a single walk.
const DefaultMaxMatches = 500

// WalkSync walks the workspace synchronously and collects all matches for
// req.Query. It is intended to be called on a background goroutine by the
// UI; it never touches shared UI state.
func WalkSync(req WalkRequest) WalkResult {
	matcher, err := Compile(req.Query)
	if err != nil {
		return WalkResult{Err: err}
	}
	root := strings.TrimSpace(req.Root)
	if root == "" {
		return WalkResult{Err: fmt.Errorf("empty search root")}
	}
	maxMatches := req.MaxMatches
	if maxMatches <= 0 {
		maxMatches = DefaultMaxMatches
	}

	defaultMatcher := ignore.NewMatcher(root)
	include := compilePatterns(req.Include)
	exclude := compilePatterns(req.Exclude)

	result := WalkResult{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root {
				if strings.HasPrefix(name, ".") || (defaultMatcher != nil && defaultMatcher.Matches(path, true)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		relSlash := filepath.ToSlash(rel)

		if strings.HasPrefix(name, ".") || (defaultMatcher != nil && defaultMatcher.Matches(path, false)) {
			return nil
		}
		if len(include) > 0 && !matchPatterns(include, relSlash, false) {
			return nil
		}
		if matchPatterns(exclude, relSlash, false) {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() == 0 || info.Size() > 10*1024*1024 {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		buf := make([]byte, 512)
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			f.Close()
			return nil
		}
		if bytes.IndexByte(buf[:n], 0) != -1 {
			f.Close()
			return nil
		}
		_, _ = f.Seek(0, 0)

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNum := 1
		for scanner.Scan() {
			lineText := scanner.Text()
			lineRunes := []rune(lineText)
			for _, loc := range matcher.FindInLine(lineRunes, 0) {
				result.Matches = append(result.Matches, SearchMatch{
					FilePath:   path,
					RelPath:    relSlash,
					LineNumber: lineNum,
					ColNumber:  loc.Col + 1,
					LineText:   strings.TrimSpace(lineText),
				})
				if len(result.Matches) >= maxMatches {
					result.Truncated = true
					f.Close()
					return errWalkDone
				}
			}
			lineNum++
		}
		_ = f.Close()
		return nil
	})
	if err == errWalkDone {
		err = nil
	}
	result.Err = err
	return result
}

var errWalkDone = fmt.Errorf("walk: match limit reached")

// compilePatterns parses gitignore-style patterns. A trailing slash is
// expanded so "internal/" includes everything under that directory.
func compilePatterns(patterns []string) []gitignore.Pattern {
	var out []gitignore.Pattern
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		if strings.HasSuffix(p, "/") {
			p = p + "**"
		}
		out = append(out, gitignore.ParsePattern(p, nil))
	}
	return out
}

// matchPatterns reports whether path (slash-separated, relative) matches any
// of patterns.
func matchPatterns(patterns []gitignore.Pattern, relPath string, isDir bool) bool {
	if len(patterns) == 0 {
		return false
	}
	parts := strings.Split(relPath, "/")
	for _, p := range patterns {
		if p.Match(parts, isDir) != gitignore.NoMatch {
			return true
		}
	}
	return false
}
