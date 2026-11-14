package snippet

// Session tracks active tabstop navigation for snippet in editor.
type Session struct {
	Active     bool
	StartRow   int
	StartCol   int
	CurrentIdx int // Index in TabStops slice
	TabStops   []TabStop
}

// NewSession starts new snippet navigation session.
func NewSession(startRow, startCol int, tabStops []TabStop) *Session {
	return &Session{
		Active:     len(tabStops) > 0,
		StartRow:   startRow,
		StartCol:   startCol,
		CurrentIdx: 0,
		TabStops:   tabStops,
	}
}

// Current returns current tabstop absolute position (row, col, length, ok).
func (s *Session) Current() (int, int, int, bool) {
	if !s.Active || s.CurrentIdx < 0 || s.CurrentIdx >= len(s.TabStops) {
		return 0, 0, 0, false
	}
	ts := s.TabStops[s.CurrentIdx]
	row := s.StartRow + ts.LineOffset
	col := ts.ColOffset
	if ts.LineOffset == 0 {
		col += s.StartCol
	}
	return row, col, ts.Length, true
}

// Next advances to next tabstop, wrapping around cleanly.
func (s *Session) Next() (int, int, int, bool) {
	if !s.Active || len(s.TabStops) == 0 {
		return 0, 0, 0, false
	}
	s.CurrentIdx++
	if s.CurrentIdx >= len(s.TabStops) {
		s.CurrentIdx = 0
	}
	return s.Current()
}

// Prev moves back to previous tabstop, wrapping around cleanly.
func (s *Session) Prev() (int, int, int, bool) {
	if !s.Active || len(s.TabStops) == 0 {
		return 0, 0, 0, false
	}
	s.CurrentIdx--
	if s.CurrentIdx < 0 {
		s.CurrentIdx = len(s.TabStops) - 1
	}
	return s.Current()
}

// Progress returns 1-based current index and total tab-stops.
func (s *Session) Progress() (int, int) {
	if !s.Active || len(s.TabStops) == 0 {
		return 0, 0
	}
	return s.CurrentIdx + 1, len(s.TabStops)
}

// Cancel terminates snippet session.
func (s *Session) Cancel() {
	s.Active = false
	s.TabStops = nil
}
