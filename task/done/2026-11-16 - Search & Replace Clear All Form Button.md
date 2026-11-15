---
title: Search & Replace Sidebar — Clear All Form Button
status: done
started: 2026-11-16
completed: 2026-11-16
branch: dev
priority: medium
---

# Search & Replace Sidebar — Clear All Form Button

## Objective
Tambahkan tombol `[✕]` (Clear all / Reset) di chip row (row 4) pada sidebar Search & Replace. Satu klik membersihkan seluruh field form (Find, Replace, Path, Exclude) dan hasil pencarian; toggle `[Aa]` `[ab|]` `[.*]` dipertahankan.

## Ground Truth Findings
- Panel = `SidebarSearch` (F4 / Alt+3), dirender `internal/search/view.go` (`SearchView.Render`, `HandleMouseClick`).
- Row 4 (`rowToggle = 4`): ` chips (" [Aa] [ab|] [.*]") + counter right-aligned`. Lebar efektif ~27 kolom.
- Row tombol Find/Replace All (`rowButtons = 15`) tidak diubah (keputusan user).
- Cleanup yang sudah ada di `DoSearch`/`DebounceMsg` menjadi acuan `Reset()`: generation++ (drop hasil async in-flight), Matches/TotalFound/Truncated/ErrMsg/groups/Selected/ScrollTop dikosongkan.
- Mouse hit-test pola: handler di `HandleMouseClick` per-row; aksi di-consumer di `internal/app/mouse_sidebar.go` (`replace-all`, `replace-file`, `open`); view menangani `find`/toggle secara internal.
- Tidak ada shortcut keyboard baru → kewajiban sinkronisasi help modal/menu/palette (AGENTS.md §11) tidak berlaku; PRD.md tetap diupdate 1 baris (aturan repo).
- Toast pola: `m.StatusBar.ShowToast(msg, false, 2*time.Second)` di `mouse_sidebar.go`.

## Plan
1. `internal/search/view.go`:
   - Tambah field `clearHitStart, clearHitEnd int` di `SearchView`.
   - Render row 4: setelah chips, append `" [✕ Clear]"` bila `width >= 38`, selain itu `" [✕]"` (style Red bold). Simpan rentang kolom hit-area. Counter right-align dari lebar chips+clear; jika pad < 1, counter di-drop (anti-overflow).
   - `HandleMouseClick` row 4: klik pada rentang hit clear → `sv.Reset()`, return `MouseAction{Kind: "clear"}`.
   - Method baru `Reset()`: SetValue("") pada Find/Replace/Path/Exclude; generation++; kosongkan ErrMsg/Matches/TotalFound/Truncated/groups/Selected/ScrollTop; `FocusField = FocusFindInput` + `syncFocus()`. Toggle tidak diubah.
   - `MouseAction.Kind` doc: tambahkan `"clear"`.
2. `internal/app/mouse_sidebar.go`: case `"clear"` → toast `"Form pencarian dibersihkan"`.
3. Test `internal/search/view_test.go`:
   - `TestViewResetClearsFormAndResults`: isi 4 field + hasil search + CaseSensitive=true → Reset → semua field kosong, Matches nil, TotalFound 0, ErrMsg "", FocusField==FocusFindInput, toggle tetap true.
   - `TestViewClearButtonHitTest`: `Render(40,...)`, klik titik tengah `[✕]` (x≈17, y=4) → `act.Kind == "clear"`, field kosong.
   - `TestViewClearButtonKeepsToggle` (opsional digabung di test pertama).
4. `PRD.md`: update bullet Search & Replace sidebar — sebutkan tombol `[✕]` clear form + hasil (toggle dipertahankan).
5. Verifikasi: `go vet ./...`, `go test -race ./internal/search/ ./internal/app/`, `wc -l internal/app/*.go` (tidak ada file app yang membengkak).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Sidebar row 4 (chip row) | Overflow > 1 baris / ANSI mentah | `ansi.StringWidth` untuk hitung pad; `ansi.Truncate` tidak diperlukan karena konten dibangun dari segmen pendek; counter di-drop bila sempit |
| Hit-testing mouse | Klik `[✕]` salah sasaran | Rentang kolom disimpan saat Render (pola konstanta row), bukan diukur manual |

## UI / Keymap Flow
- Tidak ada keybinding baru. Klik mouse pada `[✕]` (row 4, setelah `[.*]`) → form & hasil dibersihkan, fokus kembali ke Find input, toast "Form pencarian dibersihkan".

## Tasks
- [x] Task 1: `Reset()` + hit-area fields + render row 4 + hit-test di `internal/search/view.go`
- [x] Task 2: case `clear` di `mouse_sidebar.go` (toast)
- [x] Task 3: Tests di `view_test.go` (reset behavior + hit-test) — `go test -race ./internal/search/` 49 passed
- [x] Task 4: PRD.md bullet update
- [x] Task 5: Verifikasi: go vet ./... ✓, go test -race ./... → 290 passed / 21 packages ✓, `wc -l internal/app/*.go` (tidak ada non-test > 500) ✓

## Manual Test Checklist
- [x] Buka sidebar Search (F4), ketik query + path + exclude, klik `[✕]` → semua field kosong, hasil hilang, fokus kembali ke Find input, toggle tidak berubah (dikonfirmasi user)
- [x] Toast "Form pencarian dibersihkan" tampil
- [x] Setelah clear, ketik query baru → hasil muncul kembali normal (dikonfirmasi user)

## Acceptance Criteria
- [x] Klik `[✕]` mengosongkan Find/Replace/Path/Exclude + menghapus hasil (Matches nil, footer "Search results")
- [x] Toggle `[Aa]` `[ab|]` `[.*]` tidak berubah
- [x] Tidak ada shortcut baru; help/menu/palette tidak berubah
- [x] `go test -race` hijau (290 passed / 21 packages), `go vet` bersih

## Decisions
- Clear hanya field teks + hasil, toggle dipertahankan — pilihan user (opsi "Semua field + hasil").
- Tombol di chip row, bukan row tombol utama — pilihan user.
- Label `[✕ Clear]` bila width ≥ 38, `[✕]` icon-only bila lebih sempit — menjaga 24+ kolom tetap muat (sidebar default ~28).
- Warna Red bold untuk `[✕]` (selaras semantik `[×]` di Find Bar).
- Tanpa shortcut keyboard — user tidak memilihnya; menghindari kewajiban sync 4 tempat.

## Blockers
- (none)

## Next
Task selesai dan diarsipkan ke `task/done/2026-11-16 - Search & Replace Clear All Form Button.md`.
