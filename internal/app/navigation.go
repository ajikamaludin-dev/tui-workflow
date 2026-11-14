package app

import (
	"fmt"
	"time"
)

// identifierRe untuk validasi nama simbol yang sederhana.
func isIdentChar(r rune) bool {
	return r == '_' || r == '$' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// gotoDefinition melompat ke definisi simbol di bawah cursor.
func (m *Model) gotoDefinition() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return
	}
	word := tab.Buffer.WordAtCursor()
	if word == "" {
		m.StatusBar.ShowToast("Cursor tidak berada di atas simbol", true, 2*time.Second)
		return
	}
	m.gotoDefinitionWord(word)
}

// gotoDefinitionAt melompat ke definisi untuk kata pada posisi eksplisit (row,col, 0-based).
func (m *Model) gotoDefinitionAt(row, col int) {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return
	}
	buf := tab.Buffer
	if row < 0 {
		row = 0
	}
	if row >= len(buf.Lines) {
		row = len(buf.Lines) - 1
	}
	line := buf.Lines[row]
	if col > len(line) {
		col = len(line)
	}
	// ekspansi ke kiri dan kanan dari titik klik
	s := col
	for s > 0 && isIdentChar(line[s-1]) {
		s--
	}
	e := col
	for e < len(line) && isIdentChar(line[e]) {
		e++
	}
	if s >= e {
		m.StatusBar.ShowToast("Tidak ada simbol di posisi klik", true, 2*time.Second)
		return
	}
	word := string(line[s:e])
	buf.CursorRow, buf.CursorCol = row, s
	buf.CollapseCursors()
	buf.ClearSelection()
	m.gotoDefinitionWord(word)
}

func (m *Model) gotoDefinitionWord(word string) {
	if m.SymbolIndex == nil {
		m.StatusBar.ShowToast("Symbol index belum siap", true, 2*time.Second)
		return
	}
	defs := m.SymbolIndex.Definitions(word)
	if len(defs) == 0 {
		m.StatusBar.ShowToast("Tidak ada definisi untuk '"+word+"'", true, 2*time.Second)
		return
	}
	d := defs[0]
	if len(defs) > 1 {
		m.StatusBar.ShowToast(fmt.Sprintf("%d definisi ditemukan, loncat ke pertama (%s:%d)", len(defs), d.FilePath, d.Line), false, 2*time.Second)
	} else {
		m.StatusBar.ShowToast(fmt.Sprintf("Definisi '%s' → %s:%d", word, d.FilePath, d.Line), false, 2*time.Second)
	}
	m.openFileAt(d.FilePath, d.Line, d.Col)
}

// findReferences mengisi Search sidebar dengan semua kemunculan kata (word-boundary).
func (m *Model) findReferences() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return
	}
	word := tab.Buffer.WordAtCursor()
	if word == "" {
		m.StatusBar.ShowToast("Cursor tidak berada di atas simbol", true, 2*time.Second)
		return
	}
	if m.SearchView == nil {
		return
	}
	m.SearchView.DoSearchWord(word)
	m.SidebarVisible = true
	m.SidebarView = SidebarSearch
	m.Focus = FocusSidebar
	m.syncStatusBarWithActiveTab()
	m.StatusBar.ShowToast(fmt.Sprintf("Referensi '%s': %d temuan", word, m.SearchView.TotalFound), false, 2*time.Second)
}

// addNextOccurrence menambahkan cursor pada occurrence berikutnya (ala VSCode Ctrl+D).
func (m *Model) addNextOccurrence() {
	tab := m.TabManager.ActiveTab()
	if tab == nil || tab.Buffer == nil {
		return
	}
	buf := tab.Buffer
	if buf.AddNextOccurrence() {
		m.syncStatusBarWithActiveTab()
		m.StatusBar.ShowToast(fmt.Sprintf("Cursor: %d", buf.CursorCount()), false, 1*time.Second)
		return
	}
	m.StatusBar.ShowToast("Tidak ada occurrence lain", true, 2*time.Second)
}

