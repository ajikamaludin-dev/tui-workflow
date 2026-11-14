package editor

import (
	"strings"

	"tui-workflow/internal/search"
)

// MatchPos aliases the engine match so the editor shares one representation.
// Row is 0-based; Col and Length are rune offsets.
type MatchPos = search.Match

// SetSearchQuery updates the search query (case-insensitive literal) and
// recomputes all matches. An empty/whitespace query clears matches.
func (b *Buffer) SetSearchQuery(query string) {
	b.SetSearchOptions(search.Query{Pattern: query})
}

// SetSearchOptions recomputes matches for q (full engine options).
func (b *Buffer) SetSearchOptions(q search.Query) {
	b.SearchQuery = q.Pattern
	b.lastQuery = q
	b.SearchMatches = nil
	b.matchByLine = nil
	b.ActiveMatchIdx = -1

	if strings.TrimSpace(q.Pattern) == "" {
		return
	}
	matcher, err := search.Compile(q)
	if err != nil {
		b.matcher = nil
		return
	}
	b.matcher = matcher
	for i, line := range b.Lines {
		b.SearchMatches = append(b.SearchMatches, matcher.FindInLine(line, i)...)
	}
	if len(b.SearchMatches) == 0 {
		return
	}
	b.matchByLine = make(map[int][]int, len(b.Lines))
	for i, m := range b.SearchMatches {
		b.matchByLine[m.Row] = append(b.matchByLine[m.Row], i)
	}

	// Active match = nearest match at or after the cursor.
	b.ActiveMatchIdx = 0
	for i, m := range b.SearchMatches {
		if m.Row > b.CursorRow || (m.Row == b.CursorRow && m.Col >= b.CursorCol) {
			b.ActiveMatchIdx = i
			break
		}
	}
	cur := b.SearchMatches[b.ActiveMatchIdx]
	b.CursorRow = cur.Row
	b.CursorCol = cur.Col
	b.clampCursor()
	b.CenterCursorRequested = true
}

// FindNext moves to the next search match.
func (b *Buffer) FindNext() bool {
	if len(b.SearchMatches) == 0 {
		return false
	}
	b.ActiveMatchIdx = (b.ActiveMatchIdx + 1) % len(b.SearchMatches)
	cur := b.SearchMatches[b.ActiveMatchIdx]
	b.CursorRow = cur.Row
	b.CursorCol = cur.Col
	b.clampCursor()
	b.CenterCursorRequested = true
	return true
}

// FindPrev moves to the previous search match.
func (b *Buffer) FindPrev() bool {
	if len(b.SearchMatches) == 0 {
		return false
	}
	b.ActiveMatchIdx = (b.ActiveMatchIdx - 1 + len(b.SearchMatches)) % len(b.SearchMatches)
	cur := b.SearchMatches[b.ActiveMatchIdx]
	b.CursorRow = cur.Row
	b.CursorCol = cur.Col
	b.clampCursor()
	b.CenterCursorRequested = true
	return true
}

// ClearSearch clears the search query and all matches.
func (b *Buffer) ClearSearch() {
	b.SearchQuery = ""
	b.SearchMatches = nil
	b.matchByLine = nil
	b.ActiveMatchIdx = -1
	b.matcher = nil
}

// MatchInfo returns (currentMatch 1-based, totalMatches).
func (b *Buffer) MatchInfo() (int, int) {
	if len(b.SearchMatches) == 0 || b.ActiveMatchIdx < 0 {
		return 0, len(b.SearchMatches)
	}
	return b.ActiveMatchIdx + 1, len(b.SearchMatches)
}

// MatchAt returns whether (row, col) is part of a search match and whether it is active.
func (b *Buffer) MatchAt(row, col int) (isMatch bool, isActive bool) {
	indices := b.matchByLine[row]
	for _, i := range indices {
		m := b.SearchMatches[i]
		if col >= m.Col && col < m.Col+m.Length {
			return true, i == b.ActiveMatchIdx
		}
	}
	return false, false
}

// expandReplacement expands $1-style group references against the matched
// text when the active search is in regex mode; otherwise returns repl verbatim.
func (b *Buffer) expandReplacement(m MatchPos, repl string) string {
	if b.matcher == nil || !b.matcher.RegexMode() {
		return repl
	}
	if m.Row < 0 || m.Row >= len(b.Lines) {
		return repl
	}
	line := b.Lines[m.Row]
	if m.Col < 0 || m.Col+m.Length > len(line) {
		return repl
	}
	matched := string(line[m.Col : m.Col+m.Length])
	out, _ := b.matcher.ReplaceAllInText(matched, repl)
	return out
}

// replaceMatchAtIndex splices search match matchIndex with replacement. Caller
// must have already called endTypingGroup/pushUndo via the Replace* wrappers.
func (b *Buffer) replaceMatchAtIndex(matchIndex int, replacement string) bool {
	if matchIndex < 0 || matchIndex >= len(b.SearchMatches) {
		return false
	}
	cur := b.SearchMatches[matchIndex]
	if cur.Row < 0 || cur.Row >= len(b.Lines) {
		return false
	}
	line := b.Lines[cur.Row]
	if cur.Col < 0 || cur.Col+cur.Length > len(line) {
		return false
	}
	expanded := b.expandReplacement(cur, replacement)

	repRunes := []rune(expanded)
	newLine := make([]rune, 0, len(line)-cur.Length+len(repRunes))
	newLine = append(newLine, line[:cur.Col]...)
	newLine = append(newLine, repRunes...)
	newLine = append(newLine, line[cur.Col+cur.Length:]...)
	b.Lines[cur.Row] = newLine

	b.CursorRow = cur.Row
	b.CursorCol = cur.Col + len(repRunes)
	b.IsDirty = true
	b.cachedDirty = true
	b.clampCursor()

	q := b.lastQuery
	b.SetSearchOptions(q)
	return true
}

// ReplaceMatchAt replaces the match at exactly (row, col) with replacement.
func (b *Buffer) ReplaceMatchAt(row, col int, replacement string) bool {
	for i, m := range b.SearchMatches {
		if m.Row == row && m.Col == col {
			b.endTypingGroup()
			b.pushUndo()
			return b.replaceMatchAtIndex(i, replacement)
		}
	}
	return false
}

// ReplaceCurrent replaces the currently active search match with replacement string.
func (b *Buffer) ReplaceCurrent(replacement string) bool {
	if len(b.SearchMatches) == 0 || b.ActiveMatchIdx < 0 || b.ActiveMatchIdx >= len(b.SearchMatches) {
		return false
	}
	b.endTypingGroup()
	b.pushUndo()
	return b.replaceMatchAtIndex(b.ActiveMatchIdx, replacement)
}

// ReplaceAll replaces all search matches in the buffer with replacement string.
func (b *Buffer) ReplaceAll(query, replacement string) int {
	return b.ReplaceAllOptions(search.Query{Pattern: query}, replacement)
}

// ReplaceAllOptions is ReplaceAll with explicit engine options.
func (b *Buffer) ReplaceAllOptions(q search.Query, replacement string) int {
	b.SetSearchOptions(q)
	total := len(b.SearchMatches)
	if total == 0 {
		return 0
	}

	b.endTypingGroup()
	b.pushUndo()

	// Process bottom-up, right-to-left so column offsets remain valid.
	for i := len(b.SearchMatches) - 1; i >= 0; i-- {
		m := b.SearchMatches[i]
		if m.Row < 0 || m.Row >= len(b.Lines) {
			continue
		}
		line := b.Lines[m.Row]
		if m.Col < 0 || m.Col+m.Length > len(line) {
			continue
		}
		expanded := b.expandReplacement(m, replacement)
		repRunes := []rune(expanded)
		newLine := make([]rune, 0, len(line)-m.Length+len(repRunes))
		newLine = append(newLine, line[:m.Col]...)
		newLine = append(newLine, repRunes...)
		newLine = append(newLine, line[m.Col+m.Length:]...)
		b.Lines[m.Row] = newLine
	}

	b.IsDirty = true
	b.cachedDirty = true
	b.clampCursor()

	b.SetSearchOptions(b.lastQuery)
	return total
}
