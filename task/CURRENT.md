---
title: VSCode-like Navigation, Multi-Cursor Edit & Fold/Unfold (Path B — Symbol Index)
status: in_progress
started: 2026-11-13
completed:
branch:
priority: high
---

# VSCode-like Navigation, Multi-Cursor Edit & Fold/Unfold (Path B)

## Ground Truth Findings
- `internal/editor/buffer.go` (1340 baris): `Buffer` struct punya `Lines [][]rune`, cursor (`CursorRow/CursorCol`), selection (`HasSelection`, `SelectAnchorRow/Col`), search state (`SearchQuery`, `SearchMatches`), undo/redo stacks. Belum ada multi-cursor, belum ada fold state.
- `internal/editor/manager.go` (547 baris): tab manager. `internal/editor/tab.go` (140 baris): tab struct.
- `internal/search/view.go` (407 baris): `SearchView` sidebar pencarian workspace, `SearchMatch{FilePath, RelPath, LineNumber, ColNumber, LineText}`, focus fields Find/Replace/MatchList, walk filesystem. Bisa dipakai sebagai backend tampilan hasil "find references".
- `internal/syntax/highlighter.go` (185 baris): wrapper Chroma v2, hanya highlighting — tidak ada symbol extraction.
- `internal/app/model.go` (3739 baris), `internal/app/view.go` (488), `internal/app/keymap.go` (31, hanya konstanta `KeyAction`).
- PRD.md + AGENTS.md: **LSP OUT scope v1**. Path A (LSP) ditolak user untuk sekarang; user akan evaluasi hasil B, fallback ke A bila kurang.
- Bahasa target user: Go, JavaScript/React (JSX/TSX), PHP, JSON, Markdown.
- Tidak ada symbol index di kodebase saat ini.

## Objective
Menambah pengalaman coding harian ala VSCode tanpa LSP: navigasi simbol (goto definition / find references) via symbol index sendiri, multi-cursor editing, dan fold/unfold — untuk Go, JS/React, PHP, JSON, Markdown.

## Approach (Path B)
Symbol index in-memory, dibangun saat workspace load + incremental update saat file save/rename/delete. Per-bahasa extractor berbasis regex/scanner (bukan parser penuh) dengan interface seragam:

```go
type Symbol struct {
    Name     string
    Kind     string // func, method, type, const, var, class, heading, key
    FilePath string
    Line     int // 1-based
    Col      int // 1-based
}

type Extractor interface {
    Extensions() []string
    Extract(path string, content []byte) []Symbol
}
```

Extractor per bahasa:
- **Go**: `^func (receiver)? Name(`, `^type Name`, `^const/var Name` (+ block). Regex cukup; akurat untuk top-level & method.
- **JS/React**: `function Name`, `const Name = (...) =>`, `const Name = function`, `class Name`, `export default function Name`, JSX component = PascalCase const/function.
- **PHP**: `function name(`, `class Name`, `interface/trait Name`, `const NAME`.
- **JSON**: keys nested hingga **depth ≤ 10** sebagai simbol kind=key.
- **Markdown**: headings `#{1,6} Name` kind=heading.

**Goto Definition**: ambil ident di bawah cursor → lookup index → jump (tab baru jika file beda). Jika >1 kandidat → tampilkan list picker; Enter pilih. Binding: **Alt+Click** di atas symbol (mouse), **F12** (keyboard). **Find References**: word-boundary search seluruh workspace via SearchView, fokus match list. Binding: **Shift+F12** (keyboard) — hindari Ctrl+D (EOF terminal). Jika simbol tidak ketemu di index → fallback word search. Chord `gd`/`gr` dibuang (rawan nabrak terminal & butuh state "awaiting second key").

**Multi-cursor**: perluas `Buffer` dengan `Cursors []CursorPos` sekunder + `PrimaryCursor`. Semua operasi edit (insert rune, backspace, delete, newline) berlaku ke semua cursor (diproses dari bawah-ke-atas agar offset stabil). Selection per-cursor. Keybindings: **Ctrl+Alt+Click** tambah cursor di posisi klik; keyboard add-next-occurrence = **Ctrl+Shift+D** (Ctrl+D ditolak user — bentrok EOF terminal); `Esc` collapse ke primary.

**Fold/unfold**: indent-based folding. `FoldState` per tab: set line yang disembunyikan + anchor. Deteksi blok fold: baris dengan indent lebih kecil diikuti baris-baris indent lebih besar (atau brace-depth untuk Go/JS/PHP — mulai indent-based saja, konsisten lintas bahasa termasuk MD/JSON). Render melewati baris tersembunyi; cursor math di-adjust. Keybinding: `za` toggle fold di cursor, `zM` fold all, `zR` unfold all. Gutter marker `▸`/`▾` klik untuk toggle.

## Tasks
- [ ] Buat `internal/symbol/` package: `Symbol`, `Extractor` interface, extractor Go/JS/PHP/JSON/MD, `Index` (build + incremental update + lookup by name)
- [ ] Hook index build ke workspace load (`internal/app`) + update on save/create/delete/rename (`internal/explorer`, `internal/editor`)
- [ ] F12 / Shift+F12 / Alt+Click keybinding + handler di `internal/app` (goto def, references via SearchView reuse)
- [ ] Multi-cursor di `internal/editor/buffer.go`: struct, edit broadcast, selection, undo/redo snapshot konsisten
- [ ] Keybinding multi-cursor: `Ctrl+Alt+Click` (mouse), `Ctrl+Shift+D` (add next occurrence), `Esc` collapse
- [ ] Fold/unfold: `FoldState` per tab, renderer skip hidden lines, cursor math adjust, `za`/`zM`/`zR`, gutter marker + mouse toggle
- [ ] Update help modal (`renderHelpModal` di `internal/app/view.go`), Top Menu dropdown (`internal/ui/menu.go`), Command Palette (`internal/app/model.go`), README cheatsheet, PRD.md (fitur baru → wajib dok), shortcut sync mandate
- [ ] Unit tests: extractor per bahasa, index lookup, multi-cursor edit, fold render/cursor math; `leak_test.go` tetap hijau
- [ ] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor pane | Fold menyembunyikan baris → line number & cursor math bisa desync | Semua konversi visual↔physical lewat satu helper; test cursor math |
| Gutter | Marker fold tambah 1 kolom | Lebar gutter dihitung dinamis, clamp ke editor width |
| Status bar | Perlu indikator multi-cursor count & fold state | Tambah segmen kecil, truncate ANSI-aware |
| Search/sidebar | `gr` reuse SearchView — tidak ada zona baru | Reuse komponen, no layout change |
| Mouse | `Alt+Click` goto def, `Ctrl+Alt+Click` tambah cursor, klik gutter fold | Hit-test `relX/relY` validasi kolom+baris (aturan #4) |

## UI / Keymap Flow
- Mode normal editor: ketik biasa. `Ctrl+Shift+D` → select kata di bawah cursor + occurrence berikutnya jadi multi-cursor. `Ctrl+Alt+Click` → tambah cursor di posisi klik. `Esc` → kembali 1 cursor.
- `F12` atau `Alt+Click` di atas ident → jump ke definisi (tab baru bila beda file, status bar toast bila tidak ketemu + fallback search). `Shift+F12` → sidebar Search terisi semua referensi, fokus match list, Enter jump.
- `za` di baris foldable → lipat/buka. `zM` lipat semua, `zR` buka semua. Gutter: `▾` (terbuka) / `▸` (terlipat), klik toggle.
- Help modal (`?`) dapat section baru "Navigation & Multi-Cursor"; menu bar `[Setting]`/`[File]`? → item terkait di dropdown relevan; Command Palette: `Go to Definition`, `Find References`, `Add Next Occurrence`, `Fold/Unfold`, `Toggle Fold All`.

## Acceptance Criteria
- [ ] F12 / Alt+Click di fungsi Go / JS / PHP / heading MD / key JSON (depth ≤ 10) → lompat ke definisi benar
- [ ] Shift+F12 menampilkan semua referensi di SearchView, Enter jump berfungsi
- [ ] Multi-cursor: edit/ketik/hapus/undo berlaku ke semua cursor; `Ctrl+Shift+D` add next occurrence; `Esc` collapse; `Ctrl+Alt+Click` tambah cursor
- [ ] Fold/unfold: baris tersembunyi tidak dirender, cursor skip baris fold, fold state persist per tab (hilang saat tab ditutup)
- [ ] Tidak ada regresi: `go vet` bersih, `go test -race ./...` hijau, tab churn leak test hijau
- [ ] Help modal, menu dropdown, Command Palette, README, PRD ter-update (shortcut sync mandate)

## Decisions
- Fold state tidak persist di session, hilang saat tab ditutup (rekomendasi disetujui user)
- JSON symbol depth ≤ 10 (user: "depth tentu coba maksimal di 10")
- Chord `gd`/`gr` dibuang; F12/Shift+F12 ala VSCode dipakai (Ctrl+D bentrok EOF terminal, ditolak user)
- Path B (symbol index) dipilih user — LSP (Path A) ditunda; user akan evaluasi, fallback ke A bila kurang. — Karena LSP out scope v1 & berat; B tanpa dependensi baru.
- Extractor regex per bahasa, bukan parser — JSON/MD tidak butuh parser, Go/JS/PHP regex cukup untuk top-level symbols; trade-off: tidak akurat untuk edge case (generated code, nested Go func literals).
- Fold indent-based, bukan brace-based — seragam lintas 5 bahasa; brace matching bisa improvement nanti.
- `gr` reuse SearchView — tidak membuat komponen sidebar baru.
- Symbol index in-memory, rebuild incremental — tidak ada file cache di disk (hindari stale state).

## Open Questions
- (semua terjawab 2026-11-13: goto def = Alt+Click / F12; references = Shift+F12; multi-cursor mouse = Ctrl+Alt+Click, keyboard = Ctrl+Shift+D (Ctrl+D ditolak); JSON depth ≤10; fold tidak persist; chord gd/gr dibuang)

## Blockers
- (tidak ada)

## Next
User review plan ini → jawab Open Questions → set status `plan_approved` → mulai implementasi. Konfirmasi juga: eksekusi langsung di sesi ini atau delegasi ke `acp_delegate` (worker)?
