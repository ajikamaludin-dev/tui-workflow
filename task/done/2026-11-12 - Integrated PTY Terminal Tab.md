---
title: Integrated PTY Terminal Tab
status: done
started: 2026-11-12
completed: 2026-11-12
branch: main
priority: high
---

# Integrated PTY Terminal Tab

## Objective
Menambahkan fitur tab terminal interaktif langsung di dalam editor pane (`internal/terminal/`) menggunakan pseudo-terminal allocation (`creack/pty`) dan emulasi VT100/Xterm (`hinshun/vt10x`). Mendukung penuh eksekusi perintah shell, alat CLI interaktif, dan coding agent TUI bertingkat (seperti `pi`, `claude`, `btop`) tanpa mengacaukan alt-screen atau tata letak terminal tui-workflow.

## Plan
1. **Module `internal/terminal/`**:
   - `TerminalView` struct: memegang descriptor PTY (`*os.File`), command process (`*exec.Cmd`), emulasi virtual terminal (`vt10x.Terminal`), dimensi (`cols`, `rows`), dan `outputChan` reader async.
   - Reader goroutine terisolasi: membaca byte dari PTY slave, menulis ke `vt10x.Terminal`, dan menembakkan sinyal ke `outputChan`.
   - `KeyToBytes`: mengonversi `tea.KeyMsg` Bubble Tea ke ANSI byte stream mentah (runes, enter, backspace, tab, arrows, ctrl keys).
   - `Render`: merender grid buffer virtual terminal baris per baris dengan clamping lebar visual (`ansi.Truncate`) untuk mencegah terminal scroll overflow.
   - `Resize`: mengupdate ukuran PTY via `pty.Setsize` (`TIOCSWINSZ`) dan `vt10x.Resize`.
   - `Close`: teardown bersih (kirim SIGHUP/SIGTERM/Kill ke child process, close PTY descriptor, unblock reader goroutine via done channel).
2. **Ekstensi `internal/editor/`**:
   - Tambahkan enum `TabTerminal` pada `tab.go`.
   - Tambahkan pointer `TerminalView *terminal.TerminalView` pada `Tab`.
   - Update `manager.go`: `OpenTerminal`, dan pastikan semua metode penutupan tab (`CloseTab`, `CloseOtherTabs`, `CloseTabsToRight`, `CloseTabsToLeft`, `CloseAllTabs`) memanggil `tab.TerminalView.Close()` untuk mencegah proses zombie dan kebocoran file descriptor.
3. **Integrasi `internal/app/` (Model & Update & View)**:
   - Handle custom message `TerminalOutputMsg` dan `TerminalExitMsg`.
   - `openNewTerminalTab`: alokasi PTY baru dan spawn shell pengguna (`$SHELL`).
   - `WindowSizeMsg`: sesuaikan ukuran terminal tab aktif sesuai ukuran editor pane.
   - Intersepsi keystroke mentah saat fokus berada di `TabTerminal`.
   - Navigasi & fokus: `Ctrl+\` / `Ctrl+` ` / `F6` untuk beralih fokus antara Terminal dan Sidebar.
   - Shortcut `Ctrl+Shift+\` / `Ctrl+T` untuk membuka terminal baru.
   - Shortcut `Alt+W` / `Ctrl+Shift+W` untuk menutup terminal tab aktif.
   - Command Palette & Menu: daftarkan aksi `terminal.new`, `terminal.toggle_focus`, `terminal.close`.
4. **Dokumentasi & Verifikasi**:
   - Update `PRD.md`, `README.md`, dan dialog bantuan `?` (`renderHelpModal`).
   - Verifikasi linting (`go vet ./...`) dan unit test lengkap dengan race detector (`go test -count=1 -race ./...`).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane (Row 1 to Height-2) | Meluapnya output baris shell / TUI bertingkat ke baris menu atas / status bar bawah | Rendering terminal baris per baris dibatasi tepat pada `height` dan dipotong visual dengan `ansi.Truncate(line, width, "")`. |
| Top Menu & Status Bar | Tertimpa atau terdorong oleh output PTY | Tidak ada output terminal yang lolos keluar dari editor pane. Single-line guarantee menu bar dan status bar tetap terjaga. |
| Alt-Screen & Keystroke Loop | Keystroke TUI tui-workflow berebut dengan subprocess shell / coding agent | Focus isolation: saat fokus di terminal, keystroke di-stream langsung ke PTY. Global hotkey `Ctrl+\` disediakan untuk escape/toggle kembali ke Sidebar. |

## UI / Keymap Flow
- **Buka Terminal Baru**: Tekan `Ctrl+Shift+\` atau `Ctrl+T`, atau buka menu `[File] ➔ New Terminal Tab`, atau via Command Palette (`Ctrl+Shift+P ➔ Terminal: New Integrated Terminal`).
- **Input Terminal**: Ketik langsung perintah (`ls`, `git status`, `go test`, `python`, `pi`, `btop`). Output dirender secara real-time.
- **Pindah Fokus**: Tekan `Ctrl+\` atau `F6` untuk beralih antara Terminal dan Sidebar tanpa mematikan sesi terminal.
- **Tutup Terminal**: Ketik `exit` di shell, atau tekan `Alt+W` / `Ctrl+Shift+W`, atau klik tombol `[×]` pada tab.

## Tasks
- [x] Dependensi: instal `github.com/creack/pty` dan `github.com/hinshun/vt10x` ✓
- [x] Implementasi engine `internal/terminal/terminal.go` dan unit test `internal/terminal/terminal_test.go` ✓
- [x] Ekstensi `TabTerminal` pada `internal/editor/tab.go` dan `internal/editor/manager.go` ✓
- [x] Update render editor pane di `internal/app/view.go` untuk menangani `TabTerminal` ✓
- [x] Update dialog bantuan shortcut `renderHelpModal` di `internal/app/view.go` ✓
- [x] Update menu bar `[File]` di `internal/ui/menu.go` dan perbaiki `internal/ui/menu_test.go` ✓
- [x] Integrasi `TerminalOutputMsg`, `TerminalExitMsg`, window resizing, dan raw key forwarding di `internal/app/model.go` ✓
- [x] Pendaftaran item terminal di Command Palette (`internal/app/model.go`) ✓
- [x] Status bar sinkronisasi untuk `TabTerminal` ✓
- [x] Integration test komprehensif di `internal/app/terminal_tab_test.go` ✓
- [x] Update dokumentasi di `PRD.md` dan `README.md` ✓
- [x] Verifikasi: `go vet ./...` ✓ (0 error, 0 warning)
- [x] Verifikasi: `go test -count=1 -race ./...` ✓ (100% PASS, bebas race condition)

## Manual Test Checklist
- [x] Jalankan `./tui-workflow`
- [x] Tekan `Ctrl+Shift+\` atau `Ctrl+T` ➔ Tab baru `>_ bash` (atau zsh/sh) terbuka di editor pane dengan fokus aktif.
- [x] Ketik perintah `ls -la` ➔ Output direktori muncul rapi di dalam editor pane tanpa overflow.
- [x] Jalankan program TUI seperti `btop`, `htop`, atau coding agent `pi` ➔ Berjalan mulus di dalam tab.
- [x] Tekan `Ctrl+\` ➔ Fokus berpindah ke Sidebar (Files). Navigasi j/k di file tree.
- [x] Tekan `Ctrl+\` lagi ➔ Fokus kembali ke Terminal.
- [x] Tekan `Ctrl+N` ➔ Berpindah antar tab terbuka.
- [x] Ketik `exit` pada shell atau tekan `Alt+W` ➔ Tab terminal tertutup secara bersih.

## Acceptance Criteria
- [x] Terminal berjalan sebagai tab reguler di Editor Pane berdampingan dengan tab file, diff, dan markdown preview.
- [x] Multi-tab terminal didukung (bisa membuka lebih dari 1 terminal secara simultan).
- [x] Mendukung resize terminal tanpa glitch layout atau terminal scroll overflow.
- [x] Teardown proses bersih: menutup tab membunuh subprocess shell dan membersihkan file descriptor PTY tanpa zombie process.
- [x] Zero race condition pada pengujian `-race`.
- [x] 100% unit dan integration tests PASS.

## Decisions
- Menggunakan `creack/pty` untuk alokasi UNIX Pseudo-Terminal dan `hinshun/vt10x` untuk virtual terminal screen buffer emulation.
- Sinkronisasi channel reader menggunakan pattern `done` channel terdedikasi + `defer close(outputChan)` untuk menjamin zero race condition dan menghindari panic send on closed channel saat teardown.
- Menambahkan shortcut alternatif `Ctrl+T` (selain `Ctrl+Shift+\`) dan `Alt+W` (selain `Ctrl+Shift+W`) untuk kompatibilitas emulator terminal yang tidak mengirim modifier shift pada tombol kontrol.
