---
title: Tab Context Menu, Sidebar Refresh Buttons, and Extended Language Syntax Highlighting
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Tab Context Menu, Sidebar Refresh Buttons, and Extended Language Syntax Highlighting

## Objective
Mengimplementasikan tiga item rekomendasi dari `TODO.md`:
1. **#6**: Context menu klik kanan pada tab editor (`Tutup Tab Ini`, `Tutup Tab Lainnya`, `Tutup Tab ke Kanan`, `Tutup Tab ke Kiri`, `Tutup Semua Tab`).
2. **#15**: Tombol refresh visual `[⟳]` pada header sidebar (Files & Git) untuk memuat ulang file tree dan git worktree status.
3. **#17**: Dukungan autodeteksi dan syntax highlighting bawaan untuk bahasa populer: PHP, Go, JavaScript, React (JSX/TSX), HTML, dan Flutter/Dart.

## Plan
1. **Extended Syntax Highlighting (`internal/syntax/highlighter.go`)**:
   - Perluas `DetectLanguage` dan mapping `HighlightCode` untuk mengenali ekstensi:
     - PHP: `.php` -> `PHP`
     - React: `.jsx` -> `JavaScript React`, `.tsx` -> `TypeScript React`
     - Flutter/Dart: `.dart` -> `Dart`
     - Web: `.htm`, `.html` -> `HTML`, `.mjs`, `.cjs` -> `JavaScript`, `.mts`, `.cts` -> `TypeScript`
   - Pastikan chroma lexer fallback bekerja presisi untuk semua ekstensi tersebut.
   - Tambahkan unit test di `internal/syntax/highlighter_test.go`.

2. **Tab Manager Batch Close Operations (`internal/editor/manager.go`)**:
   - Implementasikan fungsi:
     - `CloseTabsToRight(idx int)`
     - `CloseTabsToLeft(idx int)`
     - Helper pengecekan dirty buffers untuk penutupan aman (`HasDirtyTabs()`, dll).
   - Tambahkan unit test di `internal/editor/manager_test.go`.

3. **Generic / Custom Context Menu Support (`internal/ui/contextmenu.go`)**:
   - Tambahkan method `OpenCustom(x, y int, items []ContextMenuItem, screenW, screenH int)` agar `ContextMenu` dapat digunakan baik untuk teks editor maupun tab editor.
   - Pastikan hit-testing mouse, clamping posisi layar, dan navigasi keyboard konsisten.
   - Tambahkan unit test di `internal/ui/contextmenu_test.go`.

4. **Tab Bar Right-Click Integration (`internal/app/model.go`)**:
   - Tangani `tea.MouseButtonRight` pada Baris 1 area tab editor (`msg.Y == 1 && msg.X >= editorStartX`).
   - Cari target tab melalui koordinat klik `TabManager.HandleTabClick`.
   - Munculkan tab context menu dengan opsi:
     - `Tutup` (`close_this`)
     - `Tutup Lainnya` (`close_others`)
     - `Tutup ke Kanan` (`close_right`)
     - `Tutup ke Kiri` (`close_left`)
     - `Tutup Semua` (`close_all`)
   - Tangani aksi penutupan tab dengan pengaman dirty buffer (autosave jika aktif, atau konfirmasi penyimpanan jika kotor).

5. **Sidebar Refresh Button (`internal/app/view.go` & `internal/app/model.go`)**:
   - Pada `renderSidebar`, render tombol `[⟳]` rata kanan di Baris 1 (header sidebar).
   - Tangani klik kiri pada koordinat tombol refresh:
     - Jika tab aktif Files: jalankan `m.Explorer.Refresh()`, tampilkan toast "File explorer dimuat ulang".
     - Jika tab aktif Git: jalankan `m.GitView.Refresh()`, tampilkan toast "Git status diperbarui".
   - Daftarkan aksi refresh di Command Palette (`Ctrl+Shift+P`).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top / Sidebar Header (Baris 1) | Overflow saat tombol `[⟳]` dirender di samping tab `1:Files 2:Git` | Hitung lebar teks secara ANSI-aware, pastikan total lebar <= `SidebarWidth`, potong aman bila sidebar sempit. |
| Editor Tab Bar (Baris 1) | Context menu pop-up meluap keluar layar | Gunakan boundary clamping pada `ContextMenu.OpenCustom` dengan margin layar. |
| Status Bar (Baris Bawah) | Label bahasa baru terlalu panjang | `DetectLanguage` menggunakan string ringkas (`PHP`, `JSX`, `TSX`, `Dart`). |

## UI / Keymap Flow
- **Klik Kanan Tab**: Klik kanan pada sembarang tab di Baris 1 -> pop-up menu muncul di bawah tab -> klik opsi atau tekan `Esc` untuk menutup.
- **Klik Refresh Sidebar**: Klik `[⟳]` di pojok kanan atas sidebar -> pohon file atau git status langsung reload dengan indikator toast.
- **Highlighting Baru**: Membuka file `.php`, `.jsx`, `.tsx`, `.dart` langsung menampilkan highlight Chroma dan label bahasa di status bar.

## Tasks
- [x] Perluas deteksi bahasa dan lexer di `internal/syntax/highlighter.go` & test
- [x] Tambahkan operasi `CloseTabsToRight` & `CloseTabsToLeft` di `internal/editor/manager.go` & test
- [x] Tambahkan `OpenCustom` di `internal/ui/contextmenu.go` & test
- [x] Integrasikan klik kanan tab bar dan aksi penutupan di `internal/app/model.go`
- [x] Tambahkan tombol `[⟳]` di `internal/app/view.go` dan handler klik di `internal/app/model.go`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓
- [x] Update `TODO.md` (#6, #15, #17)

## Manual Test Checklist
- [x] Buka file `.php`, `.jsx`, `.tsx`, `.dart`: cek status bar bahasa dan syntax coloring.
- [x] Buka 4 tab, klik kanan tab ke-2: pilih `Tutup ke Kanan` -> tab 3 dan 4 tertutup.
- [x] Klik kanan tab: pilih `Tutup Lainnya` -> hanya tab target yang tersisa.
- [x] Klik tombol `[⟳]` di Files tab: pohon file reload dan muncul toast.
- [x] Klik tombol `[⟳]` di Git tab: git status reload dan muncul toast.

## Acceptance Criteria
- [x] Klik kanan tab menampilkan context menu 5 opsi penutupan.
- [x] Tombol refresh sidebar responsif terhadap mouse click.
- [x] PHP, Go, JS, React (JSX/TSX), HTML, Flutter (Dart) ter-highlight dan terdeteksi di status bar.
- [x] `go vet ./...` 0 error, `go test -v -race ./...` 100% PASS.

## Decisions
- Menggunakan `OpenCustom` pada `ContextMenu` eksisting agar konsisten dengan tema dan styling pop-up lain tanpa membuat duplikasi kode.
- Tombol refresh diletakkan di Baris 1 sisi kanan sidebar header (`[⟳]`) agar hemat ruang dan ergonomis.

## Blockers
- None.

## Next
- Konfirmasi manual test checklist dari user sebelum archive task ke task/done/.

## Improvement 1 (2026-11-12 — Fix Editor Question Mark Typing and Relocate Sidebar Refresh Button)

### Fact Analysis
1. Global keybinding `?` mengintersepsi karakter `?` saat fokus berada di editor buffer, sehingga user tidak dapat mengetik tanda tanya `?`.
2. Tombol refresh `[⟳]` di baris tab header (`1:Files 2:Git`) kurang terlihat pada beberapa font terminal dan menempati baris tab. User meminta memindahkan tombol refresh ke baris tersendiri di paling atas di dalam tab (Row 2, sebelum file tree / git view) dengan teks & ikon yang jelas (`[⟳ Refresh]`).

### Plan
1. Pada `internal/app/model.go`: Jika `m.Focus == FocusEditor` dan buffer aktif dapat diedit, biarkan `?` jatuh ke handler buffer (`buf.InsertRune('?')`). `?` hanya membuka modal bantuan bila fokus bukan di editor teks aktif.
2. Pada `internal/app/view.go`:
   - Kembalikan header tab sidebar menjadi murni `1:Files 2:Git`.
   - Tambahkan baris toolbar tersendiri di paling atas konten tab aktif dengan tombol `[⟳ Refresh]` yang jelas dan kontras.
   - Kurangi tinggi body sidebar sebanyak 1 baris (`bodyHeight := height - 2`).
3. Pada `internal/app/model.go`:
   - Klik di Row 2 sidebar memicu refresh Files / Git view.
   - Klik di Row 3 ke atas memicu navigasi pohon file / git view (`relLine := msg.Y - 3`).
4. Update unit test di `internal/app/tab_context_test.go` dan `internal/app/features_test.go`.

### Tasks
- [x] Izinkan pengetikan `?` di editor buffer aktif saat fokus di editor (`internal/app/model.go`)
- [x] Pindahkan tombol `[⟳ Refresh]` ke baris paling atas di dalam tab sidebar (`internal/app/view.go`)
- [x] Sesuaikan koordinat klik mouse sidebar (Row 2 untuk refresh, Row 3+ untuk node) (`internal/app/model.go`)
- [x] Sesuaikan dan jalankan test suite (`internal/app/tab_context_test.go` & `internal/app/features_test.go`)
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

### Manual Test Checklist (Improvement 1)
- [x] Buka file teks di editor buffer, ketik `?` -> karakter `?` masuk ke teks, help modal tidak terbuka.
- [x] Pindah fokus ke sidebar (`Ctrl+E` atau klik sidebar), tekan `?` -> help modal terbuka normal.
- [x] Lihat sidebar: tab header bersih `1:Files 2:Git`. Tepat di bawahnya ada baris sendiri `[⟳ Refresh]` dengan teks dan icon jelas.
- [x] Klik baris `[⟳ Refresh]` pada Files tab -> toast `File explorer dimuat ulang`.
- [x] Klik baris `[⟳ Refresh]` pada Git tab -> toast `Git status diperbarui`.
