---
title: Syntax Highlighting with Color in Editor
status: done
started: 2026-11-12
completed: 2026-11-12
branch: main
priority: high
---

# Syntax Highlighting with Color in Editor

## Objective
Mengintegrasikan Chroma v2 syntax highlighting ke line buffer rendering editor tui-workflow sehingga kode tampil dengan warna sesuai bahasa pemrograman tanpa merusak rendering kursor, seleksi teks, search match, dan soft-wrap.

## Plan
1. Analisis tokenisasi Chroma v2 dan representasi warna (terminal truecolor/256 color).
2. Siapkan data struktur / helper di `internal/syntax` untuk memetakan style warna per baris/kolom atau per span token.
3. Integrasikan ke `internal/editor/buffer.go` di method `Render`:
   - Prioritas style: `isSelected` > `isCursor` > `isActiveMatch` / `isMatch` > `syntaxColor` > plain text.
   - Tetap kompatibel dengan soft-wrap (`ComputeVisualLines`), horizontal scroll (`ScrollCol`), dan visual width (`ansi.StringWidth`).
4. Pastikan Zero-Lag Mandate (<16 ms per frame) dan zero memory leak terjaga via caching.
5. Verifikasi via `go vet ./...` dan `go test -v -race ./...`.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane | ANSI escape code bocor atau merusak visual line width calculation | Gunakan `ansi.StringWidth` / ANSI parser yang valid, jangan potong sequence ANSI mentah |
| Editor Pane | Performance hit jika re-highlighting setiap render frame | Gunakan cache per buffer dirty state, hanya render visual lines yang tampak di viewport |

## UI / Keymap Flow
- Pengguna membuka file di editor (misal file `.go`, `.json`, `.md`, `.py`, dll).
- Kode ditampilkan dengan syntax highlighting (keyword, string, comment, type, function dengan warna kontras).
- Saat teks diseleksi, teks seleksi tetap memiliki background highlight kontras (override syntax color).
- Kursor I-beam dan pencarian kata (`Ctrl+F`) tetap memiliki visual highlight jelas.
- Navigasi keyboard dan scroll mouse berjalan mulus tanpa lag (<16 ms).

## Tasks
- [x] Buat helper pemetaan token / style syntax di `internal/syntax`
- [x] Integrasikan syntax coloring ke visual line text rendering di `internal/editor/buffer.go`
- [x] Pastikan padding & line width calculation menggunakan visual width (`ansi.StringWidth`)
- [x] Unit test di `internal/syntax` dan `internal/editor`
- [x] Verification: go vet ./... ✓, go test -v -race ./... ✓

## Manual Test Checklist
- [ ] Buka file Go (misal `main.go`) -> keyword (`package`, `func`, `import`), string, dan comment memiliki warna berbeda.
- [ ] Gerakkan kursor -> kursor I-beam tampak jelas pada karakter yang berwarna.
- [ ] Seleksi teks dengan Shift+Arrow atau mouse drag -> selection background tampak jelas.
- [ ] Cari kata via `Ctrl+F` -> match background tampak jelas.
- [ ] Buka file dengan soft-wrap aktif -> baris terlipat tetap berwarna dengan benar.

## Acceptance Criteria
- [x] Syntax highlighting aktif dan berwarna di editor sesuai tipe file.
- [x] Kursor, seleksi, dan pencarian tetap berfungsi normal dan terlihat jelas.
- [x] Semua test pass (`go test -v -race ./...`) dan vet bersih (`go vet ./...`).

## Decisions
- Menggunakan Chroma v2 style entries / tokenisation yang ter-cache saat buffer dirty untuk menjaga latensi render <16 ms.

## Blockers
- None.

## Next
- Konfirmasi manual test dari user, lalu arsipkan task ke `task/done/`.
