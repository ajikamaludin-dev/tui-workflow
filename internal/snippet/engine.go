package snippet

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TabStop defines a jump-target location within an expanded snippet.
type TabStop struct {
	Index       int    // 1-based order, 0 is final cursor position
	LineOffset  int    // Row relative to snippet insertion point
	ColOffset   int    // Col relative to snippet line start
	Length      int    // Length of placeholder text
	Placeholder string // Default text to select
}

// ExpandedSnippet contains the expanded multi-line text and its tabstops.
type ExpandedSnippet struct {
	Text     string
	TabStops []TabStop
}

var (
	// Matches ${1:placeholder} or $1
	tabStopRegex = regexp.MustCompile(`\$\{([0-9]+):([^}]*)\}|\$([0-9]+)`)
)

// Expand parses snippet template lines, replaces placeholders, and returns ExpandedSnippet.
func Expand(s *Snippet, filename string) *ExpandedSnippet {
	compName := BaseFilename(filename)
	var finalLines []string
	var tabStops []TabStop

	for lineIdx, rawLine := range s.Body {
		var lineBuilder strings.Builder
		lastPos := 0

		matches := tabStopRegex.FindAllStringSubmatchIndex(rawLine, -1)
		for _, m := range matches {
			// Append literal text before match
			lineBuilder.WriteString(rawLine[lastPos:m[0]])

			var indexStr, placeholder string
			if m[2] != -1 && m[3] != -1 {
				// ${N:placeholder}
				indexStr = rawLine[m[2]:m[3]]
				placeholder = rawLine[m[4]:m[5]]
			} else if m[6] != -1 && m[7] != -1 {
				// $N
				indexStr = rawLine[m[6]:m[7]]
				placeholder = ""
			}

			idx, _ := strconv.Atoi(indexStr)

			// Dynamic ComponentName replacement
			if placeholder == "ComponentName" && compName != "" {
				placeholder = compName
			}

			col := lineBuilder.Len()
			tabStops = append(tabStops, TabStop{
				Index:       idx,
				LineOffset:  lineIdx,
				ColOffset:   col,
				Length:      len(placeholder),
				Placeholder: placeholder,
			})

			lineBuilder.WriteString(placeholder)
			lastPos = m[1]
		}

		lineBuilder.WriteString(rawLine[lastPos:])
		finalLines = append(finalLines, lineBuilder.String())
	}

	// Deduplicate tabstops by Index (keep first occurrence for navigation)
	seen := make(map[int]bool)
	uniqueTabStops := make([]TabStop, 0, len(tabStops))
	for _, ts := range tabStops {
		if !seen[ts.Index] {
			seen[ts.Index] = true
			uniqueTabStops = append(uniqueTabStops, ts)
		}
	}
	tabStops = uniqueTabStops

	// Sort tabstops: 1, 2, 3... and 0 at the end
	sort.SliceStable(tabStops, func(i, j int) bool {
		idxI := tabStops[i].Index
		idxJ := tabStops[j].Index
		if idxI == 0 {
			return false
		}
		if idxJ == 0 {
			return true
		}
		return idxI < idxJ
	})

	return &ExpandedSnippet{
		Text:     strings.Join(finalLines, "\n"),
		TabStops: tabStops,
	}
}
