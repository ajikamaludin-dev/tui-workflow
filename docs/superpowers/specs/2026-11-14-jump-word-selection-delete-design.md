# Design: Word-by-Word Text Selection & Word Deletion (Editor & Terminal)

**Date:** 2026-11-14
**Status:** Approved

## Intent & Why
Pengguna membutuhkan:
1. Seleksi block text berbasis kata (word-by-word text selection) di dalam Editor menggunakan kombinasi Shift + Alt + Panah (dan Shift + Ctrl + Panah).
2. Penghapusan satu kata ke belakang dengan Backspace menggunakan Alt + Backspace dan Ctrl + Backspace di dalam Editor (serta menghapus block text jika sedang ada teks terseleksi).
3. Navigasi jump word dan hapus kata di dalam Terminal bawaan (PTY) menggunakan Alt/Ctrl + Panah dan Alt/Ctrl + Backspace agar selaras dengan standar shell CLI / Readline.

## Scope
- **In-Scope (Editor)**:
  - `Shift+Alt+Left`, `Shift+Ctrl+Left`, `Shift+Alt+Right`, `Shift+Ctrl+Right` untuk ekspansi seleksi per kata.
  - `Shift+Left`, `Shift+Right`, `Shift+Up`, `Shift+Down` untuk seleksi karakter/baris standar agar navigasi seleksi konsisten.
  - `Alt+Backspace`, `Ctrl+Backspace` untuk menghapus kata ke kiri (`DeleteWordLeft`), atau menghapus seleksi jika `HasSelection`.
- **In-Scope (Terminal)**:
  - `Alt+Left`, `Ctrl+Left` mengirim sequence `\x1bb` (Readline backward-word).
  - `Alt+Right`, `Ctrl+Right` mengirim sequence `\x1bf` (Readline forward-word).
  - `Alt+Backspace` mengirim sequence `\x1b\x7f` (Readline backward-kill-word).
  - `Ctrl+Backspace` mengirim sequence `\x17` (Readline unix-word-rubout / Ctrl+W).
- **Out-of-Scope**:
  - Text selection visual / block text di layar terminal (dikonfirmasi user tidak diperlukan).
  - Integrasi remote git / plugin eksternal.

## Approaches Considered
1. **Pendekatan A: Modular Buffer API (Dipilih)**
   - Menambahkan method seleksi dan hapus kata langsung pada struct `Buffer` (`SelectWordLeft`, `SelectWordRight`, `SelectLeft`, `SelectRight`, `SelectUp`, `SelectDown`, `DeleteWordLeft`).
   - `model.go` bertindak sebagai dispatcher tipis yang memanggil method buffer dan menyinkronkan status bar.
   - `terminal.go` `KeyToBytes` memetakan modifier `Alt` dan kontrol key ke byte escape sequence yang sesuai.
   - *Alasan*: Menjaga isolasi domain, mempermudah unit test berbasis `buffer_test.go` dan `terminal_test.go`.
2. **Pendekatan B: Inline di `model.go`**
   - Logika manipulasi baris dan kursor disisipkan langsung di event loop. Ditolak karena merusak arsitektur modular dan sulit diuji.

## Decisions Made
- Word boundary mengikuti standar VS Code dan helper `isWordChar` yang sudah ada di `internal/editor/buffer.go`.
- Penghapusan kata (`DeleteWordLeft`) memanfaatkan stack undo/redo yang sudah ada sehingga dapat di-undo dengan `Ctrl+Z`.
