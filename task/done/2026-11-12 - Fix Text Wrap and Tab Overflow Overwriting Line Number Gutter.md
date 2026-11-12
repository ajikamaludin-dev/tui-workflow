---
title: Fix Text Wrap and Tab Overflow Overwriting Line Number Gutter
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Fix Text Wrap and Tab Overflow Overwriting Line Number Gutter

## Objective
Memperbaiki bug visual di mana teks pada baris panjang (terutama yang memuat karakter tab `\t`) meluap melebihi lebar pane editor dan membungkus paksa (*hardware line wrap*) di terminal pengguna ke kolom 0, sehingga menabrak dan menimpa garis gutter nomor baris.

## Ground Truth Findings
1. File `internal/ai/client.go` baris 183 memuat 4 karakter tab `\t\t\t\t` diikuti string panjang `Content: "You are an expert developer... feat: add ai commit message generator'..."`.
2. Di `internal/editor/buffer.go`:
   - `ComputeVisualLines()` menghitung pemotongan visual baris berdasarkan jumlah rune (`end := start + textWidth`), bukan visual display width. Karakter `\t` hanya dihitung 1 rune padahal di terminal memakan 4–8 kolom display.
   - `Render()` menulis langsung karakter `\t` ke `textPart`. Karakter `\t` adalah control character yang dihitung 0 lebar oleh `ansi.StringWidth()`, sehingga padding spasi dihitung berlebih.
   - Akibatnya, `renderedRow` memiliki lebar visual jauh melebihi `width` pane editor. Saat Bubble Tea merender string ini, terminal melakukan hardware soft-wrap otomatis ke baris berikutnya di kolom 0 (`add ai commi`), menimpa posisi gutter nomor baris.
   - `renderedRow` tidak memiliki clamping `ansi.Truncate(row, width, "")` untuk mematuhi Single-Line Guarantee & Zero Terminal Scroll Overflow Mandate.

## Plan
1. **Visual Tab Expansion**:
   - Di `ComputeVisualLines()`: Hitung akumulasi visual display width per rune. Karakter `\t` dihitung sebagai tab stop (default 4 spasi atau `4 - (col % 4)`). Karakter wide/emoji dihitung 2 kolom. Baris dipotong tepat saat visual display width mencapai `textWidth`.
   - Di `Buffer.Render()`: Visualisasi `\t` dikonversi ke spasi (tab stop 4 spasi) dengan styling yang sesuai. Kursor I-beam pada posisi tab merender spasi pertama dengan kursor.
2. **Strict Row Clamping (Zero Terminal Scroll Overflow Mandate)**:
   - Setiap baris `renderedRow` dijamin tepat berukuran `width` dengan `ansi.Truncate(renderedRow, width, "")` jika berlebih, dan padding spasi jika kurang.
3. **Mouse Hit-Testing Precision (`RowColFromVisual`)**:
   - Perhitungkan lebar visual rune (termasuk ekspansi tab) saat memetakan klik mouse `textX` ke indeks rune fisik `physCol`.
4. **Verifikasi**:
   - Unit test di `internal/editor/buffer_test.go` untuk baris dengan tab `\t` dan teks panjang yang melipat.
   - `go vet ./...` dan `go test -v -race ./...`.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane | Teks meluap melebihi lebar pane editor dan merusak layout terminal | Hitung wrap berdasarkan visual width dan lakukan `ansi.Truncate(row, width, "")` |
| Line Gutter | Gutter tertimpa teks overflow | Baris hasil render tidak akan pernah meluap melewati batas lebar editor |

## UI / Keymap Flow
- Pengguna membuka file dengan indentasi tab (misal kode Go `\t\t\t\tContent: ...`).
- Teks terlipat rapi pada batas kanan pane editor tanpa meluap.
- Baris kelanjutan (*continuation line*) menampilkan gutter simbol `┊` dengan margin yang presisi.
- Tidak ada teks yang menabrak atau menimpa kolom gutter nomor baris.

## Tasks
- [x] Implementasikan visual width tracking & tab expansion di `ComputeVisualLines()`
- [x] Implementasikan visual tab rendering & row clamping di `Buffer.Render()`
- [x] Perbarui `RowColFromVisual()` untuk hit-testing mouse yang akurat pada baris bertab
- [x] Tambahkan unit test tab wrapping & gutter safety di `buffer_test.go`
- [x] Verification: go vet ./... ✓, go test -v -race ./... ✓

## Manual Test Checklist
- [x] Buka file `internal/ai/client.go` di editor dan scroll ke baris 183.
- [x] Pastikan baris 183 melipat dengan rapi ke visual line berikutnya dengan simbol `┊` pada gutter.
- [x] Pastikan teks `add ai commit...` berada di sebelah kanan gutter, tidak menabrak kolom 0 atau nomor baris.
- [x] Klik mouse pada teks baris bertab untuk memastikan posisi kursor akurat.

## Acceptance Criteria
- [x] Tidak ada teks wrap yang meluap ke kolom 0 atau menabrak nomor baris.
- [x] Baris bertab `\t` ter-wrap dan ter-render dengan rapi sesuai lebar viewport.
- [x] Semua unit test dan race detector pass (100%).

## Decisions
- Karakter tab `\t` divisualisasikan sebagai 4 spasi (tab stop modulo 4) di display buffer tanpa mengubah isi fisik buffer asli di disk.

## Blockers
- None.

## Next
- Task selesai dan diarsipkan.
