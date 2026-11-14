package editor

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Void HTML elements that should never be auto-closed.
var voidElements = map[string]bool{
	"area":    true,
	"base":    true,
	"br":      true,
	"col":     true,
	"embed":   true,
	"hr":      true,
	"img":     true,
	"input":   true,
	"link":    true,
	"meta":    true,
	"param":   true,
	"source":  true,
	"track":   true,
	"wbr":     true,
}

// Regex matching open JSX/HTML tag before the closing '>'
// Tag name must start with a letter: [a-zA-Z][a-zA-Z0-9_$-]*
// Matches:
// - Standard tags: <div, <Button, <custom-element
// - Tags with attributes: <div className="foo", <Button onClick={handleClick}
var openTagRegex = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9_$-]*)(?:\s+[^<>]*)?$`)

// IsAutoCloseTagSupported returns true if the file extension supports JSX/HTML tag auto-closing.
func IsAutoCloseTagSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jsx", ".tsx", ".js", ".ts", ".html", ".htm", ".xml", ".svg":
		return true
	default:
		return false
	}
}

// DetectAutoCloseTag checks if the string immediately before cursor (excluding the newly typed '>')
// forms an unclosed opening JSX/HTML tag. If so, returns the closing tag string (e.g. "</div>", "</Button>", "</>").
// Otherwise returns empty string.
func DetectAutoCloseTag(lineBeforeGT string) string {
	line := strings.TrimRight(lineBeforeGT, " \t")
	if line == "" {
		return ""
	}

	// 1. Do not auto-close if it's already a closing tag, e.g. "</div" or self-closing "<img /"
	if strings.HasSuffix(line, "/") || strings.Contains(line, "</") {
		return ""
	}

	// 2. React Fragment: "<" directly preceding '>'
	if strings.HasSuffix(lineBeforeGT, "<") {
		if strings.HasSuffix(lineBeforeGT, "<=") || strings.HasSuffix(lineBeforeGT, "<<") {
			return ""
		}
		return "</>"
	}

	// 3. Match opening tag
	matches := openTagRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}

	tagName := matches[1]
	if tagName == "" {
		return ""
	}

	// 4. Check if tag name is a void HTML element
	if voidElements[strings.ToLower(tagName)] {
		return ""
	}

	return "</" + tagName + ">"
}
