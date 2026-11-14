// Package symbol menyediakan symbol index in-memory untuk navigasi
// ala VSCode (goto definition / find references) tanpa LSP.
// Extractor berbasis regex per bahasa; cukup untuk simbol top-level.
package symbol

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tui-workflow/internal/ignore"
)

const MaxFileSize = 10 * 1024 * 1024 // 10 MB

type SymbolKind string

const (
	KindFunc    SymbolKind = "func"
	KindMethod  SymbolKind = "method"
	KindType    SymbolKind = "type"
	KindConst   SymbolKind = "const"
	KindVar     SymbolKind = "var"
	KindClass   SymbolKind = "class"
	KindKey     SymbolKind = "key"
	KindHeading SymbolKind = "heading"
)

// Symbol merepresentasikan satu definisi simbol di file.
type Symbol struct {
	Name     string
	Kind     SymbolKind
	FilePath string
	Line     int // 1-based
	Col      int // 1-based
}

// Extractor mengekstrak simbol dari satu file.
type Extractor interface {
	Extensions() []string
	Extract(path string, content []byte) []Symbol
}

var extractors []Extractor

func register(e Extractor) { extractors = append(extractors, e) }

func extractorFor(ext string) Extractor {
	ext = strings.ToLower(ext)
	for _, e := range extractors {
		for _, x := range e.Extensions() {
			if x == ext {
				return e
			}
		}
	}
	return nil
}

// Index adalah symbol index seluruh workspace, aman untuk concurrent access.
type Index struct {
	mu      sync.RWMutex
	byName  map[string][]Symbol
	byFile  map[string][]Symbol
	rootDir string
	matcher *ignore.Matcher
}

func NewIndex(rootDir string) *Index {
	return &Index{
		byName:  make(map[string][]Symbol),
		byFile:  make(map[string][]Symbol),
		rootDir: rootDir,
		matcher: ignore.NewMatcher(rootDir),
	}
}

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "bin": true,
	"dist": true, "build": true, ".idea": true, ".vscode": true,
}

// Build memindai seluruh workspace dan mengisi index.
func (ix *Index) Build() error {
	ix.mu.Lock()
	ix.byName = make(map[string][]Symbol)
	ix.byFile = make(map[string][]Symbol)
	ix.mu.Unlock()
	return filepath.WalkDir(ix.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != ix.rootDir {
				name := d.Name()
				if strings.HasPrefix(name, ".") || ignoredDirs[name] || (ix.matcher != nil && ix.matcher.Matches(path, true)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if ix.matcher != nil && ix.matcher.Matches(path, false) {
			return nil
		}
		ix.addFile(path)
		return nil
	})
}

func (ix *Index) addFile(path string) {
	ext := filepath.Ext(path)
	ext = strings.ToLower(ext)
	e := extractorFor(ext)
	if e == nil {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > MaxFileSize {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	syms := e.Extract(path, content)
	ix.mu.Lock()
	ix.byFile[path] = syms
	for _, s := range syms {
		ix.byName[s.Name] = append(ix.byName[s.Name], s)
	}
	ix.mu.Unlock()
}

// UpdateFile mengindeks ulang satu file (dipanggil saat save/create/rename).
func (ix *Index) UpdateFile(path string) {
	if ix.matcher != nil && ix.matcher.Matches(path, false) {
		ix.removeFile(path)
		return
	}
	ix.removeFile(path)
	ix.addFile(path)
}

// RemoveFile menghapus simbol milik satu file (dipanggil saat delete).
func (ix *Index) RemoveFile(path string) {
	ix.removeFile(path)
}

// RenameFile memindahkan entri index dari oldPath ke newPath.
func (ix *Index) RenameFile(oldPath, newPath string) {
	ix.removeFile(oldPath)
	ix.addFile(newPath)
}

func (ix *Index) removeFile(path string) {
	ix.mu.Lock()
	old, ok := ix.byFile[path]
	if ok {
		delete(ix.byFile, path)
		for _, s := range old {
			list := ix.byName[s.Name]
			for i, x := range list {
				if x.FilePath == path {
					list = append(list[:i], list[i+1:]...)
					break
				}
			}
			if len(list) == 0 {
				delete(ix.byName, s.Name)
			} else {
				ix.byName[s.Name] = list
			}
		}
	}
	ix.mu.Unlock()
}



// AllSymbols returns a snapshot of all indexed symbols.
func (ix *Index) AllSymbols() []Symbol {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	total := 0
	for _, l := range ix.byFile {
		total += len(l)
	}
	out := make([]Symbol, 0, total)
	for _, l := range ix.byFile {
		out = append(out, l...)
	}
	return out
}

// Definitions mengembalikan semua definisi dengan nama persis.
func (ix *Index) Definitions(name string) []Symbol {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return append([]Symbol(nil), ix.byName[name]...)
}

// Count jumlah simbol terindeks (untuk diagnostik/status).
func (ix *Index) Count() int {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	n := 0
	for _, l := range ix.byFile {
		n += len(l)
	}
	return n
}
