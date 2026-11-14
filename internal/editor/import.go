package editor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ReactKnownExports lists standard hooks and utilities imported from 'react'.
var ReactKnownExports = map[string]bool{
	"useState":         true,
	"useEffect":        true,
	"useContext":       true,
	"useReducer":       true,
	"useCallback":      true,
	"useMemo":          true,
	"useRef":           true,
	"useLayoutEffect":  true,
	"useId":            true,
	"useTransition":    true,
	"useDeferredValue": true,
	"memo":             true,
	"forwardRef":       true,
	"createContext":    true,
	"lazy":             true,
	"Suspense":         true,
	"Fragment":         true,
}

var (
	// importStatementRegex matches any single or multi-line import statement in JS/TS.
	importStatementRegex = regexp.MustCompile(`(?s)import\s+(?:type\s+)?([\s\S]*?)\s+from\s+['"][^'"]+['"]`)
	reactImportRegex     = regexp.MustCompile(`(?m)^import\s+(.*?)\s+from\s+['"]react['"];?`)
	namedImportsRegex    = regexp.MustCompile(`\{([^}]*)\}`)
)

// IsSymbolImported checks if symbolName is already imported anywhere in the buffer,
// supporting single-line, multi-line, barrel (@/components/index), alias, named, and default imports.
func IsSymbolImported(buf *Buffer, symbolName string) bool {
	if buf == nil || symbolName == "" {
		return false
	}
	text := buf.Text()
	symRegex := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbolName) + `\b`)

	// 1. Scan all import ... from ... statements
	matches := importStatementRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			clause := m[1]
			if symRegex.MatchString(clause) {
				return true
			}
		}
	}

	// 2. Also check direct side-effect or require imports e.g. const { Button } = require(...)
	requireRegex := regexp.MustCompile(`\b(?:const|let|var)\s+\{[^}]*\b` + regexp.QuoteMeta(symbolName) + `\b[^}]*\}\s*=\s*require`)
	if requireRegex.MatchString(text) {
		return true
	}

	return false
}

// EnsureReactImport ensures that hookName is imported from 'react' in the buffer.
// If already imported, returns false. If added or merged, returns true.
func EnsureReactImport(buf *Buffer, hookName string) bool {
	if buf == nil || hookName == "" {
		return false
	}

	// 1. Check if already imported anywhere in file
	if IsSymbolImported(buf, hookName) {
		return false
	}

	text := buf.Text()

	// 2. Look for existing 'react' import line in buffer
	reactMatch := reactImportRegex.FindStringSubmatchIndex(text)
	if len(reactMatch) >= 4 {
		startOffset := reactMatch[0]
		lineIdx, lineText := findLineByByteOffset(buf, startOffset)
		if lineIdx >= 0 {
			namedMatch := namedImportsRegex.FindStringSubmatch(lineText)
			if len(namedMatch) >= 2 {
				existingNamed := strings.TrimSpace(namedMatch[1])
				var newNamed string
				if existingNamed == "" {
					newNamed = "{ " + hookName + " }"
				} else {
					newNamed = "{ " + existingNamed + ", " + hookName + " }"
				}
				newLine := namedImportsRegex.ReplaceAllString(lineText, newNamed)
				buf.Lines[lineIdx] = []rune(newLine)
				buf.cachedTextDirty = true
				buf.markContentChanged()
				return true
			} else {
				var newLine string
				if strings.Contains(lineText, "import React from") {
					newLine = strings.Replace(lineText, "import React from", "import React, { "+hookName+" } from", 1)
				} else {
					newLine = "import { " + hookName + " } from 'react';\n" + lineText
				}
				buf.Lines[lineIdx] = []rune(newLine)
				buf.cachedTextDirty = true
				buf.markContentChanged()
				return true
			}
		}
	}

	// 3. No react import exists: insert on top
	importLine := fmt.Sprintf("import { %s } from 'react';", hookName)
	buf.InsertTopImport(importLine)
	return true
}

// EnsureWorkspaceImport ensures that symbolName is imported from sourcePath relative to currentFilePath.
func EnsureWorkspaceImport(buf *Buffer, symbolName, sourcePath, currentFilePath string) bool {
	if buf == nil || symbolName == "" || sourcePath == "" || currentFilePath == "" {
		return false
	}

	// Avoid self import
	if filepath.Clean(sourcePath) == filepath.Clean(currentFilePath) {
		return false
	}

	// 1. Check if already imported anywhere in file
	if IsSymbolImported(buf, symbolName) {
		return false
	}

	// 2. Compute clean relative import path
	currDir := filepath.Dir(currentFilePath)
	rel, err := filepath.Rel(currDir, sourcePath)
	if err != nil {
		return false
	}

	rel = filepath.ToSlash(rel)
	ext := filepath.Ext(rel)
	rel = strings.TrimSuffix(rel, ext)

	if !strings.HasPrefix(rel, "./") && !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}

	// 3. Insert import statement at top
	importLine := fmt.Sprintf("import { %s } from '%s';", symbolName, rel)
	buf.InsertTopImport(importLine)
	return true
}

// EnsureTextEditImport inserts importText at the top of buffer only if symbolName is not already imported.
// It also ensures identical import lines are never duplicated.
func EnsureTextEditImport(buf *Buffer, symbolName, importText string) bool {
	if buf == nil || symbolName == "" {
		return false
	}

	// 1. If symbol is already imported in any import statement (single or multi-line), do not add duplicate!
	if IsSymbolImported(buf, symbolName) {
		return false
	}

	trimmed := strings.TrimSpace(importText)
	if trimmed == "" {
		return false
	}

	// 2. Check if identical line already exists in file
	if strings.Contains(buf.Text(), trimmed) {
		return false
	}

	buf.InsertTopImport(trimmed)
	return true
}

func findLineByByteOffset(buf *Buffer, byteOffset int) (int, string) {
	currentOffset := 0
	for i, l := range buf.Lines {
		lineStr := string(l)
		lineBytes := len(lineStr) + 1 // +1 for \n
		if byteOffset >= currentOffset && byteOffset < currentOffset+lineBytes {
			return i, lineStr
		}
		currentOffset += lineBytes
	}
	return -1, ""
}
