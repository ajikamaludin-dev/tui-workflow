package editor

import "sort"

// editCursor adalah salinan state satu cursor (primary atau extra) untuk diproses.
type editCursor struct {
	Row, Col     int
	ARow, ACol   int
	HasSel       bool
	IsPrimary    bool
}

// CursorCount jumlah total cursor (primary + extra).
func (b *Buffer) CursorCount() int { return 1 + len(b.ExtraCursors) }

// HasMultipleCursors true bila ada cursor tambahan.
func (b *Buffer) HasMultipleCursors() bool { return len(b.ExtraCursors) > 0 }

// CollapseCursors menghapus semua cursor tambahan, menyisakan primary.
func (b *Buffer) CollapseCursors() { b.ExtraCursors = nil }

func (b *Buffer) clampExtraCursors() {
	if len(b.ExtraCursors) == 0 {
		return
	}
	for i := range b.ExtraCursors {
		if b.ExtraCursors[i].Row < 0 {
			b.ExtraCursors[i].Row = 0
		}
		if b.ExtraCursors[i].Row >= len(b.Lines) {
			b.ExtraCursors[i].Row = len(b.Lines) - 1
		}
		if b.ExtraCursors[i].Col < 0 {
			b.ExtraCursors[i].Col = 0
		}
		if b.ExtraCursors[i].Col > len(b.Lines[b.ExtraCursors[i].Row]) {
			b.ExtraCursors[i].Col = len(b.Lines[b.ExtraCursors[i].Row])
		}
		if b.ExtraCursors[i].SelAnchorRow >= len(b.Lines) {
			b.ExtraCursors[i].SelAnchorRow = len(b.Lines) - 1
		}
		if b.ExtraCursors[i].SelAnchorRow < 0 {
			b.ExtraCursors[i].SelAnchorRow = 0
		}
		if b.ExtraCursors[i].SelAnchorCol > len(b.Lines[b.ExtraCursors[i].SelAnchorRow]) {
			b.ExtraCursors[i].SelAnchorCol = len(b.Lines[b.ExtraCursors[i].SelAnchorRow])
		}
		if b.ExtraCursors[i].SelAnchorCol < 0 {
			b.ExtraCursors[i].SelAnchorCol = 0
		}
	}
}

// AddCursor menambahkan cursor tambahan di (row, col).
func (b *Buffer) AddCursor(row, col int) {
	if row < 0 {
		row = 0
	}
	if row >= len(b.Lines) {
		row = len(b.Lines) - 1
	}
	if col < 0 {
		col = 0
	}
	if col > len(b.Lines[row]) {
		col = len(b.Lines[row])
	}
	for _, e := range b.ExtraCursors {
		if e.Row == row && e.Col == col {
			return
		}
	}
	if b.CursorRow == row && b.CursorCol == col {
		return
	}
	b.ExtraCursors = append(b.ExtraCursors, extraCursor{Row: row, Col: col})
}

// AddCursorWithSelection menambahkan cursor tambahan dengan selection (start-anchor).
func (b *Buffer) AddCursorWithSelection(row, col, anchorRow, anchorCol int) {
	if row < 0 {
		row = 0
	}
	if row >= len(b.Lines) {
		row = len(b.Lines) - 1
	}
	if anchorRow < 0 {
		anchorRow = 0
	}
	if anchorRow >= len(b.Lines) {
		anchorRow = len(b.Lines) - 1
	}
	b.ExtraCursors = append(b.ExtraCursors, extraCursor{
		Row: row, Col: col,
		SelAnchorRow: anchorRow, SelAnchorCol: anchorCol,
		HasSelection: true,
	})
}

// wordRangeAtOrSelection mengembalikan rentang kata dari selection primary,
// atau kata di bawah cursor primary bila tidak ada selection.
func (b *Buffer) wordRange() (startRow, startCol, endRow, endCol int, ok bool) {
	if b.HasSelection {
		sRow, sCol, eRow, eCol := b.NormalizedSelection()
		if sRow == eRow && eCol-sCol > 0 {
			return sRow, sCol, eRow, eCol, true
		}
	}
	row := b.CursorRow
	if row < 0 || row >= len(b.Lines) {
		return 0, 0, 0, 0, false
	}
	line := b.Lines[row]
	col := b.CursorCol
	if col > len(line) {
		col = len(line)
	}
	// Tentukan titik tengah kata: bila cursor ada di atas huruf kata, gunakan col.
	// Bila di akhir kata (col > 0 dan char sebelumnya adalah word char, tapi char sekarang bukan),
	// gunakan col-1.
	target := col
	if target >= len(line) || !isWordChar(line[target]) {
		if target > 0 && isWordChar(line[target-1]) {
			target--
		} else {
			return 0, 0, 0, 0, false
		}
	}

	s := target
	for s > 0 && isWordChar(line[s-1]) {
		s--
	}
	e := target
	for e < len(line) && isWordChar(line[e]) {
		e++
	}
	if s >= e {
		return 0, 0, 0, 0, false
	}
	return row, s, row, e, true
}

// AddNextOccurrence menambahkan cursor pada occurrence berikutnya dari
// selection/kata di bawah primary cursor (ala VSCode Ctrl+D).
// Mengembalikan false bila tidak ada occurrence lain.
func (b *Buffer) AddNextOccurrence() bool {
	sRow, sCol, eRow, eCol, ok := b.wordRange()
	if !ok {
		return false
	}
	word := make([]rune, 0, eCol-sCol)
	word = append(word, b.Lines[sRow][sCol:eCol]...)
	if len(word) == 0 {
		return false
	}

	wasSelected := b.HasSelection
	if !b.HasSelection {
		b.SelectAnchorRow, b.SelectAnchorCol = sRow, sCol
		b.CursorRow, b.CursorCol = eRow, eCol
		b.HasSelection = true
	}

	// Posisi pencarian mulai sesudah selection/cursor primary.
	searchRow, searchCol := eRow, eCol

	// Kumpulkan semua occurrence setelah posisi itu (wrap sekali).
	type occ struct{ row, col int }
	var occurrences []occ
	for pass := 0; pass < 2; pass++ {
		for r := 0; r < len(b.Lines); r++ {
			line := b.Lines[r]
			for c := 0; c+len(word) <= len(line); c++ {
				match := true
				for k := range word {
					if line[c+k] != word[k] {
						match = false
						break
					}
				}
				if !match {
					continue
				}
				if pass == 0 && (r < searchRow || (r == searchRow && c < searchCol)) {
					continue
				}
				// batas kata di kiri/kanan
				if c > 0 && isWordChar(line[c-1]) {
					continue
				}
				if c+len(word) < len(line) && isWordChar(line[c+len(word)]) {
					continue
				}
				occurrences = append(occurrences, occ{r, c})
			}
		}
		if len(occurrences) > 0 {
			break
		}
	}
	if len(occurrences) == 0 {
		return !wasSelected
	}

	// occurrence yang sudah punya extra cursor dengan selection diabaikan
	for _, o := range occurrences {
		already := false
		for _, e := range b.ExtraCursors {
			if e.HasSelection && e.Row == o.row && e.Col-len(word) == o.col && e.SelAnchorRow == o.row && e.SelAnchorCol == o.col {
				already = true
				break
			}
		}
		if already {
			continue
		}
		b.AddCursorWithSelection(o.row, o.col+len(word), o.row, o.col)
		return true
	}
	return false
}

// collectCursors mengumpulkan semua cursor (primary + extra),
// diurutkan bottom-to-top agar edit offset-nya stabil.
func (b *Buffer) collectCursors() []editCursor {
	cs := make([]editCursor, 0, len(b.ExtraCursors)+1)
	cs = append(cs, editCursor{
		Row: b.CursorRow, Col: b.CursorCol,
		ARow: b.SelectAnchorRow, ACol: b.SelectAnchorCol,
		HasSel: b.HasSelection, IsPrimary: true,
	})
	for _, e := range b.ExtraCursors {
		cs = append(cs, editCursor{
			Row: e.Row, Col: e.Col,
			ARow: e.SelAnchorRow, ACol: e.SelAnchorCol,
			HasSel: e.HasSelection, IsPrimary: false,
		})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].Row == cs[j].Row {
			return cs[i].Col > cs[j].Col
		}
		return cs[i].Row > cs[j].Row
	})
	return cs
}

// applyToAllCursors menjalankan op() di semua cursor.
// Selection tiap cursor dihapus lebih dulu (seperti mengetik di VSCode).
func (b *Buffer) applyToAllCursors(op func()) {
	cs := b.collectCursors()
	for i := range cs {
		b.CursorRow, b.CursorCol = cs[i].Row, cs[i].Col
		b.SelectAnchorRow, b.SelectAnchorCol, b.HasSelection = cs[i].ARow, cs[i].ACol, cs[i].HasSel
		if cs[i].HasSel {
			b.deleteSelectionInternal()
		}
		op()
		cs[i].Row, cs[i].Col = b.CursorRow, b.CursorCol
		cs[i].HasSel = false
	}
	newExtras := make([]extraCursor, 0, len(cs))
	for _, c := range cs {
		if c.IsPrimary {
			b.CursorRow, b.CursorCol = c.Row, c.Col
			b.HasSelection = false
			continue
		}
		newExtras = append(newExtras, extraCursor{Row: c.Row, Col: c.Col})
	}
	b.ExtraCursors = newExtras
	b.clampCursor()
	b.clampExtraCursors()
}


// AddCursorAbove adds a cursor one visible/logical line above the topmost cursor.
func (b *Buffer) AddCursorAbove() {
	b.endTypingGroup()
	// find topmost cursor position
	minRow, minCol := b.CursorRow, b.CursorCol
	for _, e := range b.ExtraCursors {
		if e.Row < minRow || (e.Row == minRow && e.Col < minCol) {
			minRow, minCol = e.Row, e.Col
		}
	}
	target := minRow - 1
	for target >= 0 && b.IsHiddenLine(target) {
		target--
	}
	if target < 0 {
		return
	}
	col := minCol
	if col > len(b.Lines[target]) {
		col = len(b.Lines[target])
	}
	b.AddCursor(target, col)
}

// AddCursorBelow adds a cursor one visible/logical line below the bottommost cursor.
func (b *Buffer) AddCursorBelow() {
	b.endTypingGroup()
	maxRow, maxCol := b.CursorRow, b.CursorCol
	for _, e := range b.ExtraCursors {
		if e.Row > maxRow || (e.Row == maxRow && e.Col > maxCol) {
			maxRow, maxCol = e.Row, e.Col
		}
	}
	target := maxRow + 1
	for target < len(b.Lines) && b.IsHiddenLine(target) {
		target++
	}
	if target >= len(b.Lines) {
		return
	}
	col := maxCol
	if col > len(b.Lines[target]) {
		col = len(b.Lines[target])
	}
	b.AddCursor(target, col)
}

// WordAtCursor mengembalikan kata (identifier) di bawah cursor primary.
// Mengembalikan "" bila tidak ada kata.
func (b *Buffer) WordAtCursor() string {
	sRow, sCol, _, eCol, ok := b.wordRange()
	if !ok {
		return ""
	}
	return string(b.Lines[sRow][sCol:eCol])
}
