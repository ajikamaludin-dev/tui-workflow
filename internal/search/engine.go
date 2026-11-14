package search

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Query describes a user search query.
//
// Col and Length in Match are measured in runes (not bytes) so they map
// directly onto the editor's []rune line representation.
type Query struct {
	Pattern       string
	CaseSensitive bool
	WholeWord     bool
	Regex         bool
}

// Match is a single match position within one line.
// Row is 0-based; Col and Length are rune offsets.
type Match struct {
	Row    int
	Col    int
	Length int
}

// Matcher is a compiled Query. It is safe to use a nil *Matcher or a zero
// Matcher (no methods panic; nothing ever matches).
type Matcher struct {
	re        *regexp.Regexp
	regexMode bool
}

// Compile builds a Matcher from q. Literal mode lowercases both sides via
// \b-wrapping and (?i) unless CaseSensitive is set. An empty (or
// whitespace-only) pattern, or an invalid regex, is an error.
func Compile(q Query) (*Matcher, error) {
	p := strings.TrimSpace(q.Pattern)
	if p == "" {
		return nil, fmt.Errorf("empty search pattern")
	}
	body := p
	if !q.Regex {
		body = regexp.QuoteMeta(body)
	}
	if q.WholeWord {
		body = `\b(?:` + body + `)\b`
	}
	flags := ""
	if !q.CaseSensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + body)
	if err != nil {
		return nil, fmt.Errorf("invalid search pattern: %w", err)
	}
	return &Matcher{re: re, regexMode: q.Regex}, nil
}

// IsValid reports whether q compiles.
func IsValid(q Query) bool {
	_, err := Compile(q)
	return err == nil
}

// RegexMode reports whether the matcher was compiled from a regex pattern.
func (m *Matcher) RegexMode() bool {
	if m == nil {
		return false
	}
	return m.regexMode
}

func (m *Matcher) compiled() *regexp.Regexp {
	if m == nil {
		return nil
	}
	return m.re
}

// FindInLine returns all matches in one line; row is recorded in each Match.
func (m *Matcher) FindInLine(line []rune, row int) []Match {
	re := m.compiled()
	if re == nil {
		return nil
	}
	s := string(line)
	locs := re.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return nil
	}
	matches := make([]Match, 0, len(locs))
	for _, loc := range locs {
		col := utf8.RuneCountInString(s[:loc[0]])
		length := utf8.RuneCountInString(s[loc[0]:loc[1]])
		matches = append(matches, Match{Row: row, Col: col, Length: length})
	}
	return matches
}

// FindAll scans every line and returns all matches with Row set to the line index.
func (m *Matcher) FindAll(lines [][]rune) []Match {
	if m.compiled() == nil {
		return nil
	}
	var out []Match
	for i, line := range lines {
		out = append(out, m.FindInLine(line, i)...)
	}
	return out
}

// ReplaceAllInText splices every match of the compiled pattern in text with
// repl. In Regex mode, $1-style group references in repl are expanded against
// each matched substring; in literal mode repl is used verbatim. Returns the
// new text and the number of replacements.
func (m *Matcher) ReplaceAllInText(text string, repl string) (string, int) {
	re := m.compiled()
	if re == nil {
		return text, 0
	}
	locs := re.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return text, 0
	}
	var sb strings.Builder
	sb.Grow(len(text) + len(repl)*len(locs))
	last := 0
	for _, loc := range locs {
		sb.WriteString(text[last:loc[0]])
		if m.regexMode {
			sb.WriteString(re.ReplaceAllString(text[loc[0]:loc[1]], repl))
		} else {
			sb.WriteString(repl)
		}
		last = loc[1]
	}
	sb.WriteString(text[last:])
	return sb.String(), len(locs)
}
