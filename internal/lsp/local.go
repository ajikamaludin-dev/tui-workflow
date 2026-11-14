package lsp

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"tui-workflow/internal/editor"
	"tui-workflow/internal/snippet"
	"tui-workflow/internal/symbol"
)

// Language keywords for local completion fallback
var jsKeywords = []string{
	"import", "export", "default", "from", "const", "let", "var",
	"function", "return", "if", "else", "switch", "case", "break",
	"continue", "try", "catch", "finally", "throw", "async", "await",
	"class", "extends", "interface", "type", "typeof", "instanceof",
	"new", "this", "super", "null", "undefined", "true", "false",
	"useState", "useEffect", "useContext", "useReducer", "useCallback",
	"useMemo", "useRef", "React",
}

var goKeywords = []string{
	"package", "import", "func", "return", "var", "const", "type",
	"struct", "interface", "if", "else", "switch", "case", "default",
	"for", "range", "select", "go", "defer", "map", "chan",
	"make", "new", "len", "cap", "append", "copy", "delete",
	"panic", "recover", "nil", "true", "false", "error", "string", "int", "bool",
}

var pyKeywords = []string{
	"def", "return", "class", "import", "from", "if", "elif", "else",
	"for", "while", "try", "except", "finally", "with", "as",
	"lambda", "yield", "raise", "async", "await", "True", "False", "None",
}

// BuildLocalCompletions creates a rich set of completion items from snippets,
// symbol index, buffer words, and language keywords.
func BuildLocalCompletions(buf *editor.Buffer, filename string, symbolIndex *symbol.Index) []CompletionItem {
	var items []CompletionItem
	seen := make(map[string]bool)
	ext := strings.ToLower(filepath.Ext(filename))

	// 1. Snippets (highest priority for templates)
	for _, s := range snippet.DefaultSnippets {
		matched := false
		if len(s.Languages) == 0 {
			matched = true
		} else {
			for _, lang := range s.Languages {
				if lang == ext {
					matched = true
					break
				}
			}
		}
		if matched {
			seen[s.Prefix] = true
			items = append(items, CompletionItem{
				Label:      s.Prefix,
				Kind:       CompletionKindSnippet,
				Detail:     s.Description,
				InsertText: s.Prefix,
			})
		}
	}

	// 2. Project Symbols
	if symbolIndex != nil {
		for _, sym := range symbolIndex.AllSymbols() {
			if !seen[sym.Name] && len(sym.Name) >= 2 {
				seen[sym.Name] = true
				kind := CompletionKindVariable
				switch sym.Kind {
				case symbol.KindFunc, symbol.KindMethod:
					kind = CompletionKindFunction
				case symbol.KindClass, symbol.KindType:
					kind = CompletionKindClass
				case symbol.KindConst:
					kind = CompletionKindConstant
				}
				items = append(items, CompletionItem{
					Label:      sym.Name,
					Kind:       kind,
					Detail:     string(sym.Kind) + " (" + filepath.Base(sym.FilePath) + ")",
					InsertText: sym.Name,
				})
			}
		}
	}

	// 3. Language Keywords
	var keywords []string
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx":
		keywords = jsKeywords
	case ".go":
		keywords = goKeywords
	case ".py":
		keywords = pyKeywords
	}
	for _, kw := range keywords {
		if !seen[kw] {
			seen[kw] = true
			items = append(items, CompletionItem{
				Label:      kw,
				Kind:       CompletionKindKeyword,
				Detail:     "Keyword",
				InsertText: kw,
			})
		}
	}

	// 4. Current Buffer Words
	if buf != nil {
		words := extractBufferWords(buf)
		for _, w := range words {
			if !seen[w] && len(w) >= 2 {
				seen[w] = true
				items = append(items, CompletionItem{
					Label:      w,
					Kind:       CompletionKindText,
					Detail:     "Buffer Word",
					InsertText: w,
				})
			}
		}
	}

	return items
}

// FilterCompletions filters and sorts completion items by prefix.
func FilterCompletions(items []CompletionItem, prefix string) []CompletionItem {
	if prefix == "" {
		return items
	}
	pLower := strings.ToLower(prefix)
	var exact []CompletionItem
	var prefixMatch []CompletionItem
	var containsMatch []CompletionItem

	for _, item := range items {
		iLower := strings.ToLower(item.Label)
		if iLower == pLower {
			exact = append(exact, item)
		} else if strings.HasPrefix(iLower, pLower) {
			prefixMatch = append(prefixMatch, item)
		} else if strings.Contains(iLower, pLower) {
			containsMatch = append(containsMatch, item)
		}
	}

	// Sort prefix matches: snippets first, then alphabetically
	sort.SliceStable(prefixMatch, func(i, j int) bool {
		if prefixMatch[i].Kind == CompletionKindSnippet && prefixMatch[j].Kind != CompletionKindSnippet {
			return true
		}
		if prefixMatch[i].Kind != CompletionKindSnippet && prefixMatch[j].Kind == CompletionKindSnippet {
			return false
		}
		return prefixMatch[i].Label < prefixMatch[j].Label
	})

	var res []CompletionItem
	res = append(res, exact...)
	res = append(res, prefixMatch...)
	res = append(res, containsMatch...)
	return res
}

func extractBufferWords(buf *editor.Buffer) []string {
	var words []string
	for r := 0; r < buf.LineCount(); r++ {
		line := buf.Lines[r]
		var current strings.Builder
		for _, ch := range line {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '$' {
				current.WriteRune(ch)
			} else {
				if current.Len() >= 2 {
					words = append(words, current.String())
				}
				current.Reset()
			}
		}
		if current.Len() >= 2 {
			words = append(words, current.String())
		}
	}
	return words
}
