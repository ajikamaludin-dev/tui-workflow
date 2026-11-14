package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// DefaultIgnoredNames are directories/files always ignored by default.
var DefaultIgnoredNames = []string{
	".git",
	"node_modules",
	"vendor",
}

// Matcher checks whether files and directories should be ignored.
type Matcher struct {
	rootDir string
	matcher gitignore.Matcher
}

// NewMatcher creates an ignore matcher for the specified root directory.
// It automatically reads patterns from:
// 1. Built-in defaults (.git, node_modules, vendor)
// 2. .git/info/exclude (if repository exists)
// 3. .gitignore at rootDir
func NewMatcher(rootDir string) *Matcher {
	cleanRoot := filepath.Clean(rootDir)
	var patterns []gitignore.Pattern

	// 1. Built-in defaults
	for _, def := range DefaultIgnoredNames {
		patterns = append(patterns, gitignore.ParsePattern(def, nil))
		patterns = append(patterns, gitignore.ParsePattern(def+"/", nil))
		patterns = append(patterns, gitignore.ParsePattern("**/"+def, nil))
		patterns = append(patterns, gitignore.ParsePattern("**/"+def+"/", nil))
	}

	// 2. Read .git/info/exclude if exists
	excludePath := filepath.Join(cleanRoot, ".git", "info", "exclude")
	if excludePatterns, err := readPatternFile(excludePath, nil); err == nil {
		patterns = append(patterns, excludePatterns...)
	}

	// 3. Read root .gitignore if exists
	rootGitignore := filepath.Join(cleanRoot, ".gitignore")
	if rootPatterns, err := readPatternFile(rootGitignore, nil); err == nil {
		patterns = append(patterns, rootPatterns...)
	}

	return &Matcher{
		rootDir: cleanRoot,
		matcher: gitignore.NewMatcher(patterns),
	}
}

// readPatternFile reads gitignore formatted patterns from a file.
func readPatternFile(path string, domain []string) ([]gitignore.Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}

	if err := scanner.Err(); err != nil {
		return patterns, err
	}
	return patterns, nil
}

// Matches returns true if the given path (absolute or relative) should be ignored.
func (m *Matcher) Matches(targetPath string, isDir bool) bool {
	if m == nil || m.matcher == nil {
		return false
	}

	// Compute relative path from rootDir
	rel := targetPath
	if filepath.IsAbs(targetPath) {
		r, err := filepath.Rel(m.rootDir, targetPath)
		if err == nil {
			rel = r
		}
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return false
	}

	// Convert OS path separator to slash-separated components
	slashPath := filepath.ToSlash(rel)
	parts := strings.Split(slashPath, "/")

	// Fast path: check known hard-coded names in parts
	for _, part := range parts {
		if part == ".git" || part == "node_modules" || part == "vendor" {
			return true
		}
	}

	return m.matcher.Match(parts, isDir)
}
