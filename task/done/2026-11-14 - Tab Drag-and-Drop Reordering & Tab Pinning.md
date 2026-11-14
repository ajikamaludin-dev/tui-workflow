---
title: Tab Drag-and-Drop Reordering & Tab Pinning
status: done
started: 2026-11-14
completed: 2026-11-14
branch: dev
priority: high
---

# Tab Drag-and-Drop Reordering & Tab Pinning

## Ground Truth Findings
1. **Model & Mouse Lifecycle (`internal/app/model.go`)**:
   - `tea.WithMouseCellMotion()` aktif di `cmd/tui-workflow/main.go`.
   - Event `tea.MouseMsg` diproses di baris 2259 `internal/app/model.go`.
   - Baris 1 kolom `msg.X >= editorStartX` menangani klik tab bar (baris 2880), middle click (tutup tab), scroll wheel (`MouseButtonWheelUp`/`MouseButtonWheelDown`), dan right click (buka context menu baris 2617).
   - Sidebar resize menggunakan `ResizingSidebar` dengan menangkap `MouseActionPress`, `MouseActionMotion`, dan `MouseActionRelease` (baris 2575-2595).
2. **Tab Data Structure (`internal/editor/tab.go`)**:
   - `Tab` memiliki field `ID`, `Path`, `Name`, `Type`, `Buffer`, `DiffView`, `MarkdownViewer`, `TerminalView`, `MarkdownPreview`, `IsDiff`, `LastModTime`, `DiffStaged`, `DiffHunkPath`.
   - Belum memiliki field `IsPinned bool`.
3. **Tab Manager Operations (`internal/editor/manager.go`)**:
   - Tab disimpan dalam slice `Tabs []*Tab` dengan indeks `ActiveIndex`.
   - `RenderTabBar(width int)` merender tab dengan `ansi.Truncate` (maks 1 baris, lebar 16 char judul per tab) dan mencatat batas koordinat `ClickRegions []TabClickRegion`.
   - `TabClickRegion` memiliki `TabIdx`, `StartX`, `EndX`, `CloseX`.
   - Metode penutupan batch (`CloseOtherTabs`, `CloseTabsToRight`, `CloseTabsToLeft`, `CloseAllTabs`) menutup tab tanpa memeriksa status pin.
   - Belum ada metode `MoveTab(from, to int)`, `PinTab(idx int)`, `UnpinTab(idx int)`, atau `TogglePinTab(idx int)`.
4. **Context Menu & Shortcut Registry**:
   - Context menu tab baris 2623 memiliki opsi: Tutup, Tutup Lainnya, Tutup ke Kanan, Tutup ke Kiri, Tutup Semua. Belum ada opsi Sematkan/Lepas Sematan Tab.
   - Pop-up bantuan (`internal/ui/dialog/help.go` dan `internal/app/view.go`), Command Palette (`internal/app/model.go`), dan Top Menu bar (`internal/ui/menu.go`) belum mendaftarkan interaksi pin tab dan drag tab.

## Objective & Approach
1. **Tab Pinning**:
   - Tambahkan `IsPinned bool` ke `Tab`.
   - Tab yang disematkan (pinned) selalu dikelompokkan di sebelah kiri tab bar sebelum tab unpinned.
   - Render tab pinned dengan penanda visual pin (`📌 `) dan tanpa tombol tutup `×` (mencegah penutupan tidak sengaja).
   - Lindungi tab pinned dari operasi batch close (`CloseOtherTabs`, `CloseTabsToRight`, `CloseTabsToLeft`, `CloseAllTabs`).
   - Tolak penutupan langsung tab pinned (`Ctrl+W` / middle-click) dengan toast informatif: "Tab disematkan (lepas sematan terlebih dahulu)".
   - Sediakan toggle pin via shortcut `Alt+P`, context menu klik kanan tab ("Sematkan Tab" / "Lepas Sematan"), menu bar, dan Command Palette.
2. **Drag-and-Drop Tab Reordering**:
   - Manfaatkan event Bubble Tea `tea.MouseCellMotion`:
     - Pada `MouseActionPress` (Left Button di tab bar, bukan di tombol close/scroll/preview): inisialisasi candidate drag `DraggingTabIdx`, `DragStartX`.
     - Pada `MouseActionMotion`: jika drag aktif dan kursor bergerak melintasi batas tab lain, lakukan live reorder via `MoveTab(from, to)`.
       - Pinned tab hanya dapat dipindahkan di antara sesama pinned tab.
       - Unpinned tab hanya dapat dipindahkan di antara sesama unpinned tab.
     - Pada `MouseActionRelease`: selesaikan drag, bersihkan state drag. Jika mouse dilepas tanpa drag, jalankan aksi klik normal (aktivasi tab).
   - Visual feedback saat drag: tab yang sedang di-drag diberi highlight visual pembeda agar pengguna mengetahui tab mana yang sedang dipindahkan.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Zone 0: Top Menu / Zone 1: Tab Bar | Overflow teks judul akibat ikon pin `📌 ` mendorong baris ke bawah | Gunakan `ansi.StringWidth` dan `ansi.Truncate` ketat dengan batas `MaxHeight(1)` pada `RenderTabBar`. |
| Zone 2: Editor Pane | Event mouse drag tab bocor ke text selection editor buffer | Cek koordinat `msg.Y == 1` khusus tab bar. Bila `DraggingTabIdx >= 0`, abaikan propagasi mouse ke editor buffer. |
| Status Bar | Toast notifikasi pin/unpin | Tampilkan pesan toast 2 detik tanpa mengubah layout status bar 1 baris. |

## UI / Keymap Flow
1. **Drag-and-Drop**:
   - Klik tahan (Left Click Drag) pada tab di baris 1 tab bar.
   - Gerakkan mouse ke kiri/kanan melintasi tab lain -> tab bertukar posisi secara langsung (live reorder).
   - Lepas mouse (Release) -> posisi tab baru terkunci, kursor mouse kembali normal.
2. **Pin / Unpin Tab**:
   - Shortcut keyboard: `Alt+P` pada tab aktif untuk toggle status pin.
   - Klik kanan tab -> pilih menu "Sematkan Tab" / "Lepas Sematan".
   - Top Menu `[File]` -> "Sematkan Tab" (`Alt+P`).
   - Command Palette (`Ctrl+Shift+P`) -> "Editor: Toggle Pin Tab".
   - Pinned tab menampilkan ikon `📌 ` di sisi kiri judul dan menyembunyikan tombol `×`.

## Files Affected & Execution Steps
1. **`internal/editor/tab.go`**:
   - Tambah field `IsPinned bool` pada struct `Tab`.
   - Tambah method `TogglePin()` atau getter/setter pendukung.
2. **`internal/editor/manager.go`**:
   - Tambah method `MoveTab(from, to int)` dengan penyesuaian `ActiveIndex` dan validasi batas.
   - Tambah method `TogglePinTab(idx int)`:
     - Jika di-pin: pindahkan ke akhir grup pinned tab.
     - Jika di-unpin: pindahkan ke awal grup unpinned tab.
   - Update `RenderTabBar(width int)`:
     - Tampilkan indikator `📌 ` untuk tab pinned.
     - Sembunyikan `×` (`CloseX = -1`) untuk tab pinned.
     - Beri penanda visual jika tab sedang di-drag.
   - Update metode batch close (`CloseOtherTabs`, `CloseTabsToRight`, `CloseTabsToLeft`, `CloseAllTabs`): lewati tab pinned.
3. **`internal/app/model.go`**:
   - Tambahkan field drag state: `DraggingTabIdx int`, `IsDraggingTab bool`, `DragStartX int`.
   - Update event handling `tea.MouseMsg`:
     - Tangkap `MouseActionPress` di tab bar untuk mulai drag.
     - Tangkap `MouseActionMotion` untuk live swapping tab via `MoveTab`.
     - Tangkap `MouseActionRelease` untuk finalize drag atau fallback ke klik normal.
   - Tambahkan handler `Alt+P` di keybinding editor.
   - Update `attemptCloseTab`: cegah penutupan tab pinned dengan peringatan toast.
   - Tambahkan item context menu `tab_toggle_pin` pada klik kanan tab baris 2623.
   - Daftarkan item Command Palette `editor_toggle_pin`.
4. **`internal/ui/menu.go`**:
   - Daftarkan item "Sematkan Tab" di menu `[File]` dengan shortcut `Alt+P`.
5. **`internal/ui/dialog/help.go` & `internal/app/view.go`**:
   - Tambahkan entri shortcut `Alt+P` ("Sematkan / Lepas Sematan Tab") dan instruksi mouse "Drag & drop tab untuk mengubah urutan tab".
6. **`README.md` & `PRD.md`**:
   - Dokumentasikan fitur tab pinning dan tab drag-and-drop di tabel shortcut dan deskripsi Tab Bar.
7. **Unit Tests**:
   - `internal/editor/manager_test.go`: test `MoveTab`, `TogglePinTab`, pinned tab protection pada batch close, single-line guarantee.
   - `internal/app/tab_drag_pin_test.go`: test simulasi drag-and-drop mouse, toggle pin shortcut `Alt+P`, context menu pin/unpin, cegah close pinned tab.

## Tasks
- [x] Implementasi field `IsPinned` dan helper di `internal/editor/tab.go`
- [x] Implementasi `MoveTab`, `TogglePinTab`, dan proteksi batch close di `internal/editor/manager.go`
- [x] Update `RenderTabBar` untuk rendering ikon pin `📌 ` dan state drag
- [x] Implementasi mouse drag-and-drop handler (`Press`, `Motion`, `Release`) di `internal/app/model.go`
- [x] Implementasi shortcut `Alt+P`, guard `attemptCloseTab`, dan context menu di `internal/app/model.go`
- [x] Sinkronisasi Top Menu bar (`internal/ui/menu.go`), Dialog Bantuan (`help.go`, `view.go`), Command Palette
- [x] Dokumentasi di `PRD.md` dan `README.md`
- [x] Update checklist di `TODO.md`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Buka 3+ file tab. Tekan dan tahan mouse kiri di salah satu tab, geser ke posisi tab lain. Verifikasi urutan tab berpindah secara langsung (live reorder).
- [ ] Lepas mouse di posisi baru, klik tab tersebut, pastikan konten editor sesuai dan tab aktif berpindah dengan benar.
- [ ] Klik kanan tab -> pilih "Pin Tab" (atau tekan `Alt+P`). Verifikasi ikon `📌 ` muncul dan tab berpindah ke sisi paling kiri (kelompok pinned).
- [ ] Verifikasi tombol `[×]` tidak muncul pada tab pinned.
- [ ] Coba tekan `Ctrl+W` pada tab pinned. Verifikasi tab tidak tertutup dan muncul toast "Tab is pinned (unpin tab first to close)".
- [ ] Klik kanan tab lain -> jalankan "Close Others" atau "Close All". Verifikasi tab pinned tetap bertahan terbuka.
- [ ] Tekan `Alt+P` kembali pada tab pinned. Verifikasi status pin terlepas (toast "Tab unpinned") dan tab dapat ditutup normal.

## Acceptance Criteria
- [x] Tab editor dapat dipindahkan posisinya secara interaktif menggunakan mouse drag-and-drop.
- [x] Tab editor dapat di-pin / unpin via shortcut `Alt+P` dan context menu klik kanan.
- [x] Tab pinned otomatis berada di kelompok kiri tab bar, memiliki ikon pin `📌 `, dan tidak memiliki tombol `×`.
- [x] Tab pinned kebal dari batch close ("Tutup Lainnya", "Tutup ke Kanan", "Tutup Semua").
- [x] Rendering tab bar tetap memenuhi Single-Line Guarantee (1 baris) tanpa offscreen scroll.
- [x] Dialog bantuan `?`, menu bar, command palette, PRD, dan README sinkron.
- [x] `go vet ./...` 0 error, `go test -v -race ./...` 100% PASS.

## Decisions
- Live reorder saat `MouseActionMotion` dibanding swap saat `MouseActionRelease`: Live reorder memberikan visual feedback langsung di terminal TUI selagi pengguna menyeret kursor mouse.
- Pemisahan zona pinned vs unpinned: Pinned tab selalu berada di sebelah kiri; drag di dalam grup pinned hanya mengubah urutan sesama pinned, sedangkan drag unpinned hanya mengubah urutan sesama unpinned agar konsisten dengan standar modern editor (VS Code/Chrome).
- Proteksi batch close: Menutup tab massal tidak boleh membuang tab yang sengaja disematkan pengguna.

## Blockers
- None.

## Next
- Tunggu konfirmasi review dan uji manual dari user di terminal sebelum task diarsipkan ke `task/done/`.
