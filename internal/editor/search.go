package editor

import (
	"strings"
	"unicode"
)

// MatchPos represents a single search match position in the buffer.
type MatchPos struct {
	Row    int
	Col    int
	Length int
}

// SetSearchQuery updates the search query and recomputes all matches.
func (b *Buffer) SetSearchQuery(query string) {
	b.SearchQuery = query
	b.SearchMatches = nil
	b.ActiveMatchIdx = -1

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return
	}

	lowerQuery := strings.ToLower(trimmed)
	queryRunes := []rune(lowerQuery)
	qLen := len(queryRunes)
	if qLen == 0 {
		return
	}

	for rIdx, line := range b.Lines {
		if len(line) < qLen {
			continue
		}
		// Convert line to lowercase runes for case-insensitive matching
		lowerLine := make([]rune, len(line))
		for cIdx, r := range line {
			lowerLine[cIdx] = unicode.ToLower(r)
		}

		for cIdx := 0; cIdx <= len(lowerLine)-qLen; cIdx++ {
			matched := true
			for k := 0; k < qLen; k++ {
				if lowerLine[cIdx+k] != queryRunes[k] {
					matched = false
					break
				}
			}
			if matched {
				b.SearchMatches = append(b.SearchMatches, MatchPos{
					Row:    rIdx,
					Col:    cIdx,
					Length: qLen,
				})
			}
		}
	}

	if len(b.SearchMatches) > 0 {
		// Set active match to the one closest to current cursor position
		b.ActiveMatchIdx = 0
		for i, m := range b.SearchMatches {
			if m.Row > b.CursorRow || (m.Row == b.CursorRow && m.Col >= b.CursorCol) {
				b.ActiveMatchIdx = i
				break
			}
		}
		// Move cursor to active match
		cur := b.SearchMatches[b.ActiveMatchIdx]
		b.CursorRow = cur.Row
		b.CursorCol = cur.Col
		b.clampCursor()
	}
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
	return true
}

// ClearSearch clears the search query and all matches.
func (b *Buffer) ClearSearch() {
	b.SearchQuery = ""
	b.SearchMatches = nil
	b.ActiveMatchIdx = -1
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
	if len(b.SearchMatches) == 0 {
		return false, false
	}
	for i, m := range b.SearchMatches {
		if m.Row == row && col >= m.Col && col < m.Col+m.Length {
			return true, i == b.ActiveMatchIdx
		}
	}
	return false, false
}
