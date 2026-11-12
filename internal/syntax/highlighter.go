package syntax

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type Highlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
}

func NewHighlighter() *Highlighter {
	st := styles.Get("github-dark")
	if st == nil {
		st = styles.Fallback
	}

	fm := formatters.Get("terminal256")
	if fm == nil {
		fm = formatters.Fallback
	}

	return &Highlighter{
		style:     st,
		formatter: fm,
	}
}

func (h *Highlighter) DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "Go"
	case ".md", ".markdown":
		return "Markdown"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".sh", ".bash", ".zsh":
		return "Shell"
	case ".py":
		return "Python"
	case ".js", ".mjs", ".cjs":
		return "JavaScript"
	case ".jsx":
		return "JSX"
	case ".ts", ".mts", ".cts":
		return "TypeScript"
	case ".tsx":
		return "TSX"
	case ".php", ".phtml", ".php4", ".php5":
		return "PHP"
	case ".dart":
		return "Dart"
	case ".html", ".htm":
		return "HTML"
	case ".css", ".scss", ".sass", ".less":
		return "CSS"
	case ".sql":
		return "SQL"
	case ".rs":
		return "Rust"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "C++"
	case ".java":
		return "Java"
	case ".kt", ".kts":
		return "Kotlin"
	case ".xml":
		return "XML"
	case ".toml":
		return "TOML"
	default:
		return "Plain Text"
	}
}

func (h *Highlighter) HighlightCode(code, path string) []string {
	if code == "" {
		return []string{""}
	}

	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return strings.Split(code, "\n")
	}

	var buf bytes.Buffer
	if err := h.formatter.Format(&buf, h.style, iterator); err != nil {
		return strings.Split(code, "\n")
	}

	formatted := buf.String()
	// Split by newline while preserving line count
	return strings.Split(formatted, "\n")
}
