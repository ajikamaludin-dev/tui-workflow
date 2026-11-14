---
title: Peningkatan Terminal Safety, Menu Shortcut, Viewport Centering, dan Ergonomi Keymap
status: in_progress
started: 2026-11-14
completed:
branch: dev
priority: high
---

# Peningkatan Terminal Safety, Menu Shortcut, Viewport Centering, dan Ergonomi Keymap

## Ground Truth Findings
1. **Terminal Process Lifecycle**:
   - `internal/terminal/terminal.go`: `TerminalView` menyimpan `cmd *exec.Cmd` (`cmd.Process.Pid`) dan `ptyFile *os.File`. Belum ada fungsi pengecekan apakah shell utama memiliki proses turunan (child processes) atau foreground process group yang sedang aktif.
   - `internal/app/model.go`: `attemptCloseTab()` langsung mengeksekusi `CloseTab()` untuk tab terminal tanpa memeriksa status proses di dalamnya. Pada penutupan editor (`Ctrl+Q`), hanya `hasDirtyTabs()` yang dicek.
2. **Menu Category Alignment**:
   - `internal/ui/menu.go`: Item `Keyboard Shortcuts` (`ActionID: "setting.shortcuts"`) saat ini terdaftar di kategori `Setting` (baris 115). Kategori `About` hanya berisi `about.open`.
3. **Viewport Scroll Positioning (Bottom Clamping)**:
   - `internal/editor/buffer_view.go`: Logika scroll buffer pada baris 241-244 menyetel `b.ScrollRow = visualCursorRow - height + 1` saat target baris kursor berada di bawah viewport, menyebabkan hasil pencarian selalu mentok di dasar layar.
   - `internal/git/diff.go`: Logika `ensureActiveHunkVisible()` pada baris 452 menyetel `dv.ScrollOffset = target - contentHeight + 1`, menyebabkan hunk baru selalu tampil di dasar layar.
4. **Terminal Keystroke Handling**:
   - `internal/app/update_keys.go`: Baris 629 hanya mendengarkan `"ctrl+shift+\\"` dan `"ctrl+t"`. Terminal ANSI standar mengirim `Ctrl+\` / `Ctrl+|` sebagai ASCII byte `0x1C` (`FS`), yang diurai oleh Bubble Tea sebagai `"ctrl+\\"` atau `"ctrl+|"`. Selain itu, saat terminal aktif dan terfokus, keystroke diserap mentah oleh PTY.
5. **Command Palette Shortcut Constraints**:
   - Terminal ANSI standar tidak mengirim modifier Shift pada `Ctrl+Shift+P` (mengirim `0x10` yang identik dengan `Ctrl+P`), sehingga `Ctrl+Shift+P` memicu Quick Open bukan Command Palette. `Ctrl+.` juga tidak memiliki representasi kontrol ASCII standar tanpa protokol Kitty/CSI u.

## Objective
1. Mencegah penutupan terminal atau editor secara tidak sengaja ketika terdapat proses aktif (running child process) dengan menampilkan modal konfirmasi dialog.
2. Memindahkan item menu `Keyboard Shortcuts` dari kategori `Setting` ke kategori `About`.
3. Mengubah perilaku viewport jump pada pencarian keyword buffer dan navigasi Git hunk agar selalu dipusatkan secara vertikal (`center vertical`).
4. Memperbaiki pintasan pembuka tab terminal agar dapat dipicu via `Ctrl+\`, `Ctrl+|`, maupun `Ctrl+Shift+\`.
5. Menetapkan pintasan Command Palette yang andal di terminal tanpa kombinasi Shift (`F1`, serta integrasi prefix `>` pada Quick Open `Ctrl+P`).

## Plan

### 1. Terminal Active Process Detection & Safety Confirmation
- Tambahkan method `HasRunningProcess() bool` pada `internal/terminal/TerminalView`.
  - Pada Linux/Unix: periksa keberadaan child process dari `t.cmd.Process.Pid` (membaca `/proc/<pid>/task/<pid>/children` atau mengecek foreground process group PTY via `unix.TIOCGPGRP` terhadap PID shell).
  - Jika `tcgetpgrp(ptyFd) != shellPid` atau terdapat child process aktif, kembalikan `true`.
- Perbarui `attemptCloseTab` di `internal/app/model.go`:
  - Jika tab bertipe `TabTerminal` dan `TerminalView.HasRunningProcess()` bernilai true, buka `ConfirmDialog` bertipe `ConfirmCloseTerminal` ("Active process is running in terminal. Terminate and close?").
- Perbarui penanganan `Ctrl+Q` (`app.exit`):
  - Tambahkan pengecekan tab terminal yang sedang menjalankan proses aktif. Jika ada, tampilkan peringatan konfirmasi sebelum menutup aplikasi.

### 2. Menu Reorganization (Keyboard Shortcuts -> About)
- Di `internal/ui/menu.go`:
  - Hapus item `Keyboard Shortcuts` dari kategori `Setting`.
  - Tambahkan item `Keyboard Shortcuts` ke kategori `About` (`ActionID: "setting.shortcuts"` atau alias `about.shortcuts`).
- Di `internal/app/view.go` & `internal/app/palette_actions.go`:
  - Pastikan action handler tetap berjalan normal tanpa mengubah ID aksi yang sudah terikat ke Command Palette dan test suite.

### 3. Center Vertical Viewport Scrolling
- **Editor Buffer (`internal/editor/buffer.go` & `internal/editor/search.go` / `buffer_view.go`)**:
  - Saat `FindNext()` atau `FindPrev()` dipanggil, tandai flag pemusatan vertikal atau langsung sesuaikan `ScrollRow` sehingga baris target berada di tengah viewport:
    `b.ScrollRow = visualCursorRow - (height / 2)`.
  - Lakukan clamping: jika `ScrollRow < 0`, set ke `0`.
- **Git Split Diff (`internal/git/diff.go`)**:
  - Perbarui `ensureActiveHunkVisible(viewHeight int)`:
    Gantikan penempatan dasar layar (`target - contentHeight + 1`) dengan penempatan tengah:
    `dv.ScrollOffset = target - (contentHeight / 2)`.
  - Lakukan clamping: batas bawah `0` dan batas atas `len(dv.Result.Rows) - viewHeight`.

### 4. Robust Terminal Tab Shortcut
- Di `internal/app/update_keys.go`:
  - Perluas matching shortcut pembuka terminal:
    `case "ctrl+\\", "ctrl+|", "ctrl+shift+\\", "ctrl+t":`
  - Pastikan interceptor terminal saat focused juga mengizinkan pintasan ini agar pengguna dapat membuka tab terminal baru meskipun sedang fokus di shell.

### 5. Rekomendasi Shortcut Command Palette (Tanpa Shift)
- **Implementasi**:
  1. **`F1`** (Sudah didukung, 100% kompatibel di semua terminal tanpa kombinasi Shift).
  2. **Prefix `>` pada Quick Open (`Ctrl+P`)**: Jika pengguna menekan `Ctrl+P` lalu mengetik karakter awal `>`, otomatis alihkan ke antarmuka Command Palette (seperti perilaku standar VS Code). Tidak memerlukan kombinasi Shift atau shortcut baru yang rentan bentrok dengan OS desktop.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu Bar (Zone 1) | Perubahan jumlah item pada dropdown Setting dan About | Lebar dropdown dihitung dinamis; tetap terisolasi dalam 1 baris menu statis (`MaxHeight(1)`). |
| Editor Pane (Zone 2) | Jump keyword/hunk mengubah `ScrollRow` dan `ScrollOffset` | Gunakan clamping strictly `0` s/d `maxLines` agar tidak terjadi out-of-bounds rendering atau whitespace berlebih. |
| Status Bar (Zone 4) | Peringatan proses aktif saat keluar aplikasi | Menggunakan `ConfirmDialog` terpusat dan toast error di status bar jika pembatalan terjadi. |

## UI / Keymap Flow
1. **Buka Terminal Baru**:
   - Tekan `Ctrl+\`, `Ctrl+|`, atau `Ctrl+T` dari mana saja -> Tab terminal baru terbuka.
2. **Buka Command Palette (Tanpa Shift)**:
   - Tekan `F1` -> Command palette modal muncul.
   - Atau tekan `Ctrl+P` lalu ketik `>` -> Switch otomatis ke Command palette.
3. **Pencarian Kata / Git Hunk**:
   - Tekan `Ctrl+F` -> cari kata -> tekan `Enter` / `F3` / `n` -> viewport otomatis menempatkan baris yang ditemukan tepat di tengah tinggi editor (`center vertical`).
   - Pada tab diff: tekan `n` / `]` -> hunk berpindah dan diposisikan di tengah tinggi editor diff.
4. **Keamanan Proses Terminal**:
   - Jalankan perintah panjang di terminal (misal: `top` atau `sleep 100`).
   - Tekan `Ctrl+W` pada tab terminal -> Muncul modal konfirmasi: "Process still running. Force close? [Yes / No]".
   - Tekan `Ctrl+Q` -> Muncul konfirmasi yang sama sebelum aplikasi keluar.

## Tasks
- [ ] Implementasi deteksi active child process pada `internal/terminal/terminal.go`
- [ ] Implementasi konfirmasi penutupan terminal & exit guard pada `internal/app/model.go`
- [ ] Pindahkan menu `Keyboard Shortcuts` dari `Setting` ke `About` pada `internal/ui/menu.go`
- [ ] Implementasi vertical centering untuk `FindNext`/`FindPrev` pada `internal/editor/`
- [ ] Implementasi vertical centering untuk `NextHunk`/`PrevHunk` pada `internal/git/diff.go`
- [ ] Tambahkan keybinding `ctrl+\`, `ctrl+|` pada `internal/app/update_keys.go`
- [ ] Implementasi switch otomatis ke Command Palette saat Quick Open (`Ctrl+P`) diawali karakter `>`
- [ ] Sinkronisasi cheatsheet shortcut di `README.md`, dialog `?`, dan menu bar
- [ ] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Tekan `Ctrl+\` di sembarang mode -> tab terminal baru terbuka.
- [ ] Jalankan `sleep 30` di terminal tab -> tekan `Ctrl+W` -> modal konfirmasi muncul.
- [ ] Buka menu `About` (`Alt+A`) -> verifikasi ada item `Keyboard Shortcuts`.
- [ ] Buka file panjang -> cari kata dengan `Ctrl+F` yang berada di baris bawah -> verifikasi baris target muncul di tengah vertikal viewport.
- [ ] Buka tab git diff -> tekan `n` -> verifikasi hunk target berpindah ke tengah vertikal viewport.
- [ ] Tekan `F1` -> command palette terbuka dengan lancar.
- [ ] Tekan `Ctrl+P` lalu ketik `>` -> otomatis beralih membuka Command Palette.

## Acceptance Criteria
- [ ] Tab terminal tidak tertutup tiba-tiba jika ada child process aktif tanpa persetujuan user.
- [ ] Item `Keyboard Shortcuts` berada di menu `About`.
- [ ] Pencarian keyword dan navigasi git hunk menempatkan baris aktif di area tengah vertikal (bukan baris paling bawah).
- [ ] Terminal dapat dibuka via `Ctrl+\`, `Ctrl+|`, dan `Ctrl+T`.
- [ ] Command palette dapat dibuka via `F1` dan prefix `>` pada Quick Open.
- [ ] Semua pengujian lolos: `go vet ./...` dan `go test -v -race ./...`.

## Decisions
- Menggunakan `F1` dan prefix `>` pada Quick Open untuk Command Palette — Menghindari kelemahan terminal ANSI yang tidak membedakan modifier Shift pada kombinasi `Ctrl+Shift+P`, ketiadaan control code untuk `Ctrl+.`, serta mencegah bentrok dengan shortcut desktop global pengguna.
- Menggunakan `ScrollOffset = target - (height / 2)` — Menjamin baris hasil pencarian dan Git hunk memiliki konteks baris atas dan baris bawah yang seimbang saat dibaca.

## Blockers
- None.

## Next
- Tunggu approval user untuk mulai tahap implementasi (`status: in_progress`).
