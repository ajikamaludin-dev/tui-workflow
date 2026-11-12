---
title: Formatter Engine & Smart Fallback Chain
status: review
started: 2026-11-12
completed:
branch: dev
priority: high
---

# Formatter Engine & Smart Fallback Chain

## Objective
Mengintegrasikan format code engine berbasis CLI tools eksternal (Prettier, Biome, Pint, Gofmt, dll) via STDIN/STDOUT non-blocking pipe dengan smart fallback chain, format-on-save toggle, dan undo support (Ctrl+Z).

## Plan
- Buat package `internal/formatter` dengan `FormatRequest`, `FormatResult`, dan `Smart Fallback Chain` untuk berbagai bahasa.
- Tambahkan method `ReplaceContentWithUndo` di `internal/editor/buffer.go` agar hasil formatting dapat di-undo via `Ctrl+Z`.
- Tambahkan `FormatOnSave` dan custom `Formatters` di `internal/config/config.go`.
- Integrasikan trigger formatting (manual `Ctrl+Shift+I` / `Alt+Shift+F` dan otomatis saat Save jika aktif) serta message handler `FormatCompleteMsg` di `internal/app/model.go`.
- Daftarkan menu `Format Document` di menu `File`, `Toggle Format on Save` di `Setting`, Command Palette (`F1`), Help Modal (`?`), serta perbarui `PRD.md` dan `README.md`.

## Terminal & Layout Impact Analysis
No layout impact. Output formatter berupa text replacement in-buffer dan notifikasi toast 1-baris pada status bar.

| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane   | Text replacement merusak cursor/undo | `ReplaceContentWithUndo` mencatat snapshot undo dan melakukan `clampCursor` |
| Status Bar    | Toast message terlalu panjang        | Format toast dibatasi pendek (e.g. `✓ Formatted with gofmt`) |

## UI / Keymap Flow
1. **Shortcut Format Manual**: Tekan `Ctrl+Shift+I` atau `Alt+Shift+F` di buffer editor aktif ➔ Status bar menampilkan toast `Memformat dokumen...` ➔ Eksekusi CLI non-blocking via tea.Cmd ➔ Hasil diformat masuk ke buffer dengan toast `✓ Formatted with <tool>`.
2. **Undo**: Tekan `Ctrl+Z` ➔ Buffer kembali persis ke teks sebelum diformat.
3. **Format on Save**: Aktifkan via menu `[Setting] ➔ Toggle Format on Save` atau `setting.format_on_save` di Command Palette ➔ Saat `Ctrl+S`, buffer diformat otomatis sebelum disimpan ke disk.

## Tasks
- [x] Buat package `internal/formatter` dengan runner pipe STDIN/STDOUT dan Smart Fallback Chain
- [x] Implementasikan `ReplaceContentWithUndo` pada Buffer di `internal/editor/buffer.go`
- [x] Tambahkan opsi konfigurasi `FormatOnSave` dan `Formatters` di `internal/config/config.go`
- [x] Integrasikan trigger format dan handler `FormatCompleteMsg` di `internal/app/model.go`
- [x] Daftarkan menu dropdown `File` & `Setting`, Command Palette (`F1`), dan Help Modal (`?`)
- [x] Sinkronisasi dokumentasi `PRD.md` dan `README.md`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Buka file Go berantakan (misal `test.go`), tekan `Ctrl+Shift+I` atau `Alt+Shift+F` ➔ Kode rapi via `gofmt` & toast `✓ Formatted with gofmt` muncul.
- [ ] Tekan `Ctrl+Z` ➔ Teks kembali berantakan (undo berfungsi).
- [ ] Buka menu `[Setting]` ➔ Pilih `Toggle Format on Save` ➔ Toast `Format on Save Diaktifkan` muncul.
- [ ] Edit file Go tanpa rapi, tekan `Ctrl+S` ➔ File otomatis rapi dan tersimpan.
- [ ] Buka Command Palette (`F1`), ketik `Format Document` ➔ Muncul item dengan shortcut `Ctrl+Shift+I`.
- [ ] Tekan `?` ➔ Buka dialog shortcut, pastikan `Ctrl+Shift+I` terdaftar di bawah `Editor & Buffer`.

## Acceptance Criteria
- [x] STDIN/STDOUT piping non-blocking dengan timeout 4 detik.
- [x] Smart fallback chain untuk React/TSX/JSX, PHP/Blade, Go, Python, Rust, C/C++, Shell, HTML/CSS/JSON, dll.
- [x] Kemampuan mendeteksi local binary (`node_modules/.bin`, `vendor/bin`) sebelum global binary.
- [x] Support custom formatter overrides di `config.json`.
- [x] `Ctrl+Z` mengembalikan konten sebelum diformat secara utuh.
- [x] Rule 11 shortcut synchronization terpenuhi (Menu Bar, Command Palette, Help Modal, PRD, README).
- [x] `go vet ./...` 100% clean, `go test -v -race ./...` 100% PASS.

## Decisions
- Menggunakan STDIN/STDOUT piping langsung tanpa temporary file disk ➔ Lebih cepat (<20ms), aman dari disk race condition, dan tidak meninggalkan file sampah.
- Timeout 4 detik pada context exec ➔ Mencegah CLI formatter yang hang membekukan thread.
- Fallback chain memprioritaskan local project tools (`node_modules/.bin/biome`, `vendor/bin/pint`) sebelum global tools ➔ Menjamin konsistensi versi linter/formatter tiap proyek.

## Blockers
None.

## Next
Menunggu review dan pengujian manual oleh user di terminal sebelum diarsipkan ke `task/done/`.
