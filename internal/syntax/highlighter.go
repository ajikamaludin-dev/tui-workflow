package syntax

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

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

// ParseLineStyles parses an ANSI-formatted line and returns a slice of ANSI style strings,
// one for each visual rune in the line.
func ParseLineStyles(ansiLine string) []string {
	if ansiLine == "" {
		return nil
	}

	var styles []string
	currentStyle := ""
	n := len(ansiLine)
	i := 0

	for i < n {
		if ansiLine[i] == 0x1b && i+1 < n {
			// Check for CSI sequence: ESC [
			if ansiLine[i+1] == '[' {
				j := i + 2
				for j < n && (ansiLine[j] < 0x40 || ansiLine[j] > 0x7e) {
					j++
				}
				if j < n {
					finalByte := ansiLine[j]
					seq := ansiLine[i : j+1]
					if finalByte == 'm' {
						if seq == "\x1b[0m" || seq == "\x1b[m" {
							currentStyle = ""
						} else {
							if currentStyle == "" {
								currentStyle = seq
							} else {
								currentStyle += seq
							}
						}
					}
					i = j + 1
					continue
				}
			}
			// Other escape sequence, skip ESC byte
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(ansiLine[i:])
		if r == utf8.RuneError && size <= 1 {
			i++
			continue
		}

		styles = append(styles, currentStyle)
		i += size
	}

	return styles
}

// HighlightCodeStyles highlights code and returns a 2D slice: [physicalLineIdx][runeColIdx]ansiEscape.
func (h *Highlighter) HighlightCodeStyles(code, path string) [][]string {
	lines := h.HighlightCode(code, path)
	result := make([][]string, len(lines))
	for i, l := range lines {
		result[i] = ParseLineStyles(l)
	}
	return result
}

