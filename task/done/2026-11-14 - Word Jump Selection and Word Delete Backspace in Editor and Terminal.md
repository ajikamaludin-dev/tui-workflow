---
title: Word Jump Selection and Word Delete with Backspace in Editor and Terminal
status: done
started: 2026-11-14
completed: 2026-11-14 
branch: main
priority: high
---

# Word Jump Selection and Word Delete with Backspace in Editor and Terminal

## Ground Truth Findings
1. **Editor Buffer (`internal/editor/buffer.go`)**:
   - `MoveWordLeft()` (L545) & `MoveWordRight()` (L586) sudah mengimplementasikan word boundary menggunakan helper `isWordChar` (huruf, angka, `_`) dan whitespace skip.
   - Buffer selection API sudah tersedia: `StartSelection(row, col)`, `UpdateSelection(row, col)`, `ClearSelection()`, `NormalizedSelection()`, `deleteSelectionInternal()` (L425, L720-775).
   - `Backspace()` (L429) sudah menghapus block selection jika `HasSelection == true`, namun belum memiliki method `DeleteWordLeft()` untuk `Alt+Backspace` / `Ctrl+Backspace`.
   - Belum ada keyboard selection helper (`SelectWordLeft`, `SelectWordRight`, `SelectLeft`, `SelectRight`, dsb.) di `Buffer`; saat ini selection hanya di-trigger via mouse drag di `model.go` L2661.
2. **Terminal Emulator PTY (`internal/terminal/terminal.go`)**:
   - `KeyToBytes(msg tea.KeyMsg)` (L353-426) hanya memetakan `msg.Type == tea.KeyLeft` -> `\x1b[D`, `tea.KeyRight` -> `\x1b[C`, `tea.KeyBackspace` -> `0x7f`.
   - `msg.Alt` diabaikan pada arrow keys dan backspace, sehingga shell CLI (bash/zsh) menerima panah biasa dan backspace 1 karakter, bukan word navigation/kill.
3. **Event Dispatcher (`internal/app/model.go`)**:
   - `case "left":`, `case "right":`, `case "ctrl+left", "alt+left":`, `case "ctrl+right", "alt+right":` (L2045-2070) selalu memanggil `buf.ClearSelection()`.
   - Belum menangani chord shift: `shift+left`, `shift+right`, `shift+alt+left`, `alt+shift+left`, `shift+ctrl+left`, `ctrl+shift+left`, `alt+backspace`, `ctrl+backspace`.
4. **Shortcut Documentation & Modals**:
   - Shortcut baru wajib sinkron ke `HelpDialog` (`internal/app/view.go`), Command Palette (`internal/app/model.go`), `README.md`, dan `PRD.md` sesuai AGENTS.md Bagian 4.11.

## Objective
Menambahkan kemampuan seleksi block text per kata (word jump selection) dan hapus satu kata ke belakang dengan Backspace di Editor, serta memperbaiki word jump dan word deletion di Terminal PTY.

## Plan
1. **`internal/editor/buffer.go`**:
   - Tambahkan method `SelectWordLeft()`, `SelectWordRight()`, `SelectLeft()`, `SelectRight()`, `SelectUp()`, `SelectDown()` untuk navigasi seleksi keyboard.
   - Tambahkan method `DeleteWordLeft()` dengan dukungan undo/redo dan penghapusan selection jika sedang aktif.
2. **`internal/terminal/terminal.go`**:
   - Perbarui `KeyToBytes(msg tea.KeyMsg)` untuk mendeteksi `Alt+Left` / `Ctrl+Left` (`\x1bb`), `Alt+Right` / `Ctrl+Right` (`\x1bf`), `Alt+Backspace` (`\x1b\x7f`), dan `Ctrl+Backspace` (`\x17`).
3. **`internal/app/model.go`**:
   - Tambahkan mapping key event di editor focus untuk `shift+alt+left`, `alt+shift+left`, `shift+ctrl+left`, `ctrl+shift+left`, `shift+alt+right`, `alt+shift+right`, `shift+ctrl+right`, `ctrl+shift+right`, `shift+left`, `shift+right`, `shift+up`, `shift+down`, `alt+backspace`, `ctrl+backspace`.
4. **Dokumentasi & Modal Sinkronisasi**:
   - Perbarui daftar shortcut di `internal/app/view.go` (Help Modal `?`), `internal/app/model.go` (Command Palette), `README.md`, dan `PRD.md`.
5. **Verifikasi & Test**:
   - Tambah unit test di `internal/editor/buffer_test.go` dan `internal/terminal/terminal_test.go`.
   - Jalankan `go vet ./...` dan `go test -v -race ./...`.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane (Zone 2) | Selection highlight render overflow | Menggunakan `NormalizedSelection` dan visual clipping ANSI yang sudah teruji di `Buffer.Render()`. |
| Terminal Pane (Zone 2) | Shell escape sequence mismatch | Menggunakan escape sequence POSIX standard Readline (`\x1bb`, `\x1bf`, `\x1b\x7f`, `\x17`) yang kompatibel dengan bash, zsh, sh. |
| Top/Status Bar (Zone 0/3) | Status bar position indicator shift | Memanggil `syncStatusBarWithActiveTab()` setiap kursor atau selection berubah. |

## UI / Keymap Flow
- **Editor**:
  - `Shift+Alt+Left` / `Shift+Ctrl+Left`: Kursor lompat ke awal kata sebelumnya sambil memblok teks dari anchor.
  - `Shift+Alt+Right` / `Shift+Ctrl+Right`: Kursor lompat ke akhir kata sesudahnya sambil memblok teks dari anchor.
  - `Shift+Left` / `Shift+Right` / `Shift+Up` / `Shift+Down`: Memperluas atau menyusutkan blok seleksi per karakter/baris.
  - Panah biasa tanpa Shift (`Left`, `Right`, `Up`, `Down`, `Alt+Left`, `Alt+Right`): Menghapus blok seleksi (`ClearSelection`) dan memindahkan kursor normal.
  - `Alt+Backspace` / `Ctrl+Backspace`: Jika ada blok seleksi aktif, hapus blok teks. Jika tidak ada seleksi, hapus kata di sebelah kiri kursor.
  - `Backspace`: Jika ada blok seleksi aktif, hapus blok teks.
- **Terminal**:
  - `Alt+Left` / `Ctrl+Left`: Memindahkan kursor shell ke awal kata sebelumnya.
  - `Alt+Right` / `Ctrl+Right`: Memindahkan kursor shell ke awal kata berikutnya.
  - `Alt+Backspace` / `Ctrl+Backspace`: Menghapus satu kata ke belakang di baris perintah shell.

## Tasks
- [x] Task 1: Tambahkan method `SelectWordLeft`, `SelectWordRight`, `SelectLeft`, `SelectRight`, `SelectUp`, `SelectDown`, dan `DeleteWordLeft` di `internal/editor/buffer.go` beserta unit test di `internal/editor/buffer_test.go`.
- [x] Task 2: Perbarui `KeyToBytes` di `internal/terminal/terminal.go` untuk mendukung word jump (`\x1bb`, `\x1bf`) dan word backspace (`\x1b\x7f`, `\x17`) beserta unit test di `internal/terminal/terminal_test.go`.
- [x] Task 3: Sambungkan key handler di `internal/app/model.go` untuk keybindings seleksi dan penghapusan kata.
- [x] Task 4: Sinkronisasi shortcut di Help Modal (`internal/app/view.go`), Command Palette (`internal/app/model.go`), `README.md`, dan `PRD.md`.
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Buka file teks di Editor:
  - Tekan `Shift+Alt+Right` / `Shift+Alt+Left`: teks terblok per kata.
  - Tekan `Backspace`: teks terblok terhapus seketika.
  - Tekan `Ctrl+Z`: teks yang terhapus kembali (undo berfungsi).
  - Tempatkan kursor di tengah kata, tekan `Alt+Backspace`: kata sebelah kiri kursor terhapus.
  - Tekan panah kiri/kanan tanpa shift: seleksi block hilang dan kursor bergerak normal.
- [ ] Buka tab Terminal (`Ctrl+T`):
  - Ketik `echo halo dunia programmer`.
  - Tekan `Alt+Left` berulang: kursor melompat per kata ke kiri.
  - Tekan `Alt+Right` berulang: kursor melompat per kata ke kanan.
  - Tekan `Alt+Backspace`: kata terakhir terhapus dari baris perintah.
- [ ] Tekan `?` untuk membuka Help Modal: pastikan shortcut baru tercantum rapi.

## Acceptance Criteria
- [x] `Shift+Alt+Left` dan `Shift+Alt+Right` memblok teks per kata di Editor.
- [x] `Alt+Backspace` dan `Ctrl+Backspace` menghapus satu kata ke kiri di Editor dan Terminal.
- [x] Menekan `Backspace` saat ada teks terblok di Editor menghapus seluruh blok teks yang terseleksi.
- [x] `go vet ./...` 0 error, 0 warning.
- [x] `go test -v -race ./...` 100% PASS bebas race condition.

## Decisions
- Menggunakan standar Readline `\x1bb` / `\x1bf` untuk kompatibilitas maksimal shell Linux/macOS.
- Menggunakan `isWordChar` eksisting agar konsistensi batas kata identik antara navigasi kursor dan seleksi blok.

## Blockers
- None.

## Next
- Review dan verifikasi manual oleh user di terminal. Setelah oke, arsipkan task ke `task/done/`.
