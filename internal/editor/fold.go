package editor

// Fold berbasis indentasi. FoldRegion{Start,End}: baris Start terlihat
// (marker ▸/▾), baris (Start, End] disembunyikan.

// FoldableAt melaporkan apakah baris row adalah awal blok yang bisa dilipat,
// dan mengembalikan baris terakhir blok itu.
func (b *Buffer) FoldableAt(row int) (end int, ok bool) {
	if row < 0 || row >= len(b.Lines) {
		return 0, false
	}
	if b.IsHiddenLine(row) {
		return 0, false // baris fold-start tidak pernah hidden, tapi internal aman
	}
	line := b.Lines[row]
	if len(trimSpace(line)) == 0 {
		return 0, false
	}
	baseIndent := indentOf(line)
	end = row
	for i := row + 1; i < len(b.Lines); i++ {
		l := b.Lines[i]
		if len(trimSpace(l)) == 0 {
			continue
		}
		if indentOf(l) <= baseIndent {
			break
		}
		end = i
	}
	if end > row {
		return end, true
	}
	return 0, false
}

func indentOf(line []rune) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func trimSpace(line []rune) []rune {
	s := 0
	for s < len(line) && (line[s] == ' ' || line[s] == '\t') {
		s++
	}
	e := len(line)
	for e > s && (line[e-1] == ' ' || line[e-1] == '\t') {
		e--
	}
	return line[s:e]
}

// IsHiddenLine true bila baris berada di dalam region fold (bukan anchor).
func (b *Buffer) IsHiddenLine(line int) bool {
	for _, r := range b.Folded {
		if line > r.Start && line <= r.End {
			return true
		}
	}
	return false
}

func (b *Buffer) FoldIndexAt(start int) int {
	for i, r := range b.Folded {
		if r.Start == start {
			return i
		}
	}
	return -1
}

func (b *Buffer) IsFoldedAt(start int) bool { return b.FoldIndexAt(start) >= 0 }

// ToggleFold toggles fold di baris row (anchor). Baris anchor yang tidak
// folded dan foldable akan dilipat; anchor folded akan dibuka.
func (b *Buffer) ToggleFold(row int) bool {
	if idx := b.FoldIndexAt(row); idx >= 0 {
		b.Folded = append(b.Folded[:idx], b.Folded[idx+1:]...)
		return true
	}
	end, ok := b.FoldableAt(row)
	if !ok {
		return false
	}
	b.Folded = append(b.Folded, FoldRegion{Start: row, End: end})
	// Pin cursor keluar dari region yang baru dilipat.
	if b.CursorRow > row && b.CursorRow <= end {
		b.CursorRow = row
		b.CursorCol = len(trimSpace(b.Lines[row]))
		b.clampCol()
	}
	for i := range b.ExtraCursors {
		if b.ExtraCursors[i].Row > row && b.ExtraCursors[i].Row <= end {
			b.ExtraCursors[i].Row = row
			b.ExtraCursors[i].Col = len(b.Lines[row])
		}
	}
	return true
}

// FoldAll melipat semua region foldable (terluar saja agar tidak tumpang tindih).
func (b *Buffer) FoldAll() {
	b.Folded = nil
	for row := 0; row < len(b.Lines); {
		_, ok := b.FoldableAt(row)
		if !ok {
			row++
			continue
		}
		// FoldableAt sudah cek hidden; saat Folded kosong semua visible.
		end, _ := b.FoldableAt(row)
		b.Folded = append(b.Folded, FoldRegion{Start: row, End: end})
		row = end + 1
	}
	// Pin cursor.
	for _, r := range b.Folded {
		if b.CursorRow > r.Start && b.CursorRow <= r.End {
			b.CursorRow = r.Start
			b.clampCol()
		}
	}
}

// UnfoldAll membuka semua fold.
func (b *Buffer) UnfoldAll() { b.Folded = nil }

// VisiblePhysicalLines mengembalikan indeks fisik baris yang terlihat (tidak hidden).
func (b *Buffer) VisiblePhysicalLines() []int {
	out := make([]int, 0, len(b.Lines))
	for i := range b.Lines {
		if !b.IsHiddenLine(i) {
			out = append(out, i)
		}
	}
	return out
}
