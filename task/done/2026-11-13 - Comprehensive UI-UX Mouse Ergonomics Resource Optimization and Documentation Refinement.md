---
title: Comprehensive UI/UX, Mouse Ergonomics, Resource Optimization, and Documentation Refinement
status: done
started: 2026-11-13
completed: 2026-11-13
branch: dev
priority: high
---

# Comprehensive UI/UX, Mouse Ergonomics, Resource Optimization, and Documentation Refinement

## Ground Truth Findings
1. **Modal Mouse Fallthrough**:
   - Terverifikasi di `internal/app/model.go:1570-1650`: Hanya `InputDialog`, `ConfirmDialog`, dan `FilePicker` yang memiliki interceptor mouse.
   - Sembilan modal lain (`CommandPalette`, `QuickOpen`, `GlobalSearch`, `BranchSwitcher`, `AISettingsDialog`, `FindBar`, `Switcher`, `HelpModal`, `AboutModal`) tidak memotong event mouse. Klik mouse tembus ke background (menutup tab via tombol `[×]`, klik tombol stage Git, atau geser kursor editor).
   - Dialog `Switcher` (`internal/ui/dialog/switcher.go`) di `model.go:1643` langsung menutup pada klik tanpa mengaktifkan tab yang diklik.
   - Dialog `FindBar` (`internal/ui/dialog/findbar.go`) melayang di pojok kanan atas editor tanpa hit-box klik untuk input maupun tombol aksi (`[Next]`, `[Prev]`, `[Replace]`, `[All]`, `[Close]`).
2. **Visual Inconsistency & Double Borders**:
   - Terverifikasi di `internal/app/view.go:297`: `renderHelpModal()` dan `renderAboutModal()` menggunakan hardcoded ASCII box drawing (`┌─`, `│`, `└─`) yang dibungkus oleh `WarningBoxStyle` (yang sudah memiliki `lipgloss.DoubleBorder()` + border merah).
   - Mengakibatkan visual double-border bertumpuk, tidak serasi dengan skema warna VS Code Dark, dan tidak menggunakan `NormalizeDialogBackground()` sehingga teks editor di background bocor ke sela dialog.
3. **Status Bar & Git Sidebar Header Non-Interactive**:
   - Terverifikasi di `internal/app/model.go:1650`: Baris `msg.Y == m.Height-1` (Status Bar) sama sekali tidak menangani klik mouse. Tombol `[AutoSave: ON/OFF]`, teks `git: <branch>`, kursor `Ln X, Col Y`, dan `?: Help` tidak dapat diklik.
   - Terverifikasi di `internal/app/view.go:136`: Header sidebar Git hanya menampilkan `🌿 SOURCE CONTROL` tanpa nama branch aktif, dan baris 1 sidebar tidak menangani klik mouse untuk membuka dialog branch switcher (melanggar spesifikasi PRD 4.2.B.2).
4. **Resource Inefficiency & Idle CPU/Memory Consumption**:
   - Terverifikasi di `internal/app/model.go:165, 447`: `marqueeTickCmd()` memancarkan pesan setiap 250ms (4 FPS) secara permanen via `tea.Tick`. Walau user idle atau fokus di editor/terminal/git, loop `Update()` tetap berjalan dan Bubble Tea memanggil `View()` untuk me-render ulang seluruh antarmuka 4 FPS saat diam.
   - Terverifikasi di `internal/editor/buffer.go:990-1000` & `internal/syntax/highlighter.go:147`: Setiap ketikan karakter (`b.cachedDirty = true`), `HighlightCodeStyles` men-tokenisasi seluruh isi file via `b.Text()` dan `ParseLineStyles` mengalokasikan slice string per rune untuk seluruh baris file. Melanggar mandat AGENTS.md 4.6 (*"hanya format ulang baris yang tampak di viewport"*).
5. **Missing & Outdated Documentation**:
   - Terverifikasi di filesystem: Direktori `docs/` dan file `docs/architecture.md` (spesifikasi PRD 287) belum ada.
   - Terverifikasi di `README.md:180-240`: Struktur pohon direktori tidak memuat paket `internal/ai/`, `internal/config/`, `internal/formatter/`, `internal/logger/`, `internal/search/`, `internal/terminal/`, serta berbagai dialog modal baru. Tabel shortcut juga belum mencakup pintasan AI settings, terminal tabs, dan formatter.

---

## Objective & Approach
1. **Mengeradikasi Seluruh Mouse Fallthrough**: Menambahkan method `HandleMouse(relX, relY, ...)` pada seluruh modal (`CommandPalette`, `QuickOpen`, `GlobalSearch`, `BranchSwitcher`, `AISettingsDialog`, `FindBar`, `Switcher`, `HelpModal`, `AboutModal`) dan menangani routing klik di `model.go` sehingga klik di dalam modal mengeksekusi item/aksi, klik di luar modal menutup modal (*dismiss*), dan tidak ada event yang tembus ke widget di belakangnya.
2. **Standardisasi Visual Modal**: Mengubah `renderHelpModal` dan `renderAboutModal` menjadi komponen standar ber-`RoundedBorder` dengan warna `Accent`, padding proporsional, `NormalizeDialogBackground`, serta tombol tutup interaktif `[ Tutup ]`.
3. **Interaktivitas Mouse Baris Bawah & Git Header**:
   - Status Bar: Klik `[AutoSave]` toggle autosave, klik branch buka branch switcher, klik `Ln/Col` buka go-to-line, klik `?: Help` buka help modal.
   - Git Sidebar Header: Tampilkan `🌿 SOURCE CONTROL (nama-branch)` dan tangani klik baris 1 untuk membuka branch switcher (`Alt+B`).
4. **Optimasi Resource & Zero Idle Loop**:
   - Modifikasi `MarqueeTickMsg` agar hanya dijadwalkan ulang saat teks node explorer terpilih memang meluap melebihi lebar sidebar dan fokus berada di sidebar.
   - Optimasi `HighlightCodeStyles` agar tokenisasi Chroma dan parsing style ANSI hanya dilakukan untuk baris-baris di sekitar jendela viewport (`ScrollRow` hingga `ScrollRow + height + margin`), menghindari alokasi memory masif per ketikan.
5. **Pembaruan Dokumentasi Lengkap**:
   - Buat `docs/architecture.md` mencakup arsitektur 4 zona, event loop Bubble Tea, state cycle, modul buffer, git engine, dan PTY emulator.
   - Perbarui `README.md` memuat seluruh struktur paket aktual dan tabel keybindings lengkap.

---

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu (Row 0) | Tidak ada perubahan layout. | Pertahankan tinggi tepat 1 baris. |
| Sidebar (Col 1) | Header Git bertambah teks branch. | Terapkan `ansi.Truncate(title, width-1, "")` agar tidak meluap dari kolom sidebar. |
| Editor Pane (Col 2) | Floating FindBar menangani mouse. | Pastikan bounding box hit-test FindBar dihitung presisi relatif terhadap posisi floating. |
| Status Bar (Row Height-1) | Klik mouse pada item status bar. | Pertahankan tinggi tepat 1 baris dengan `MaxHeight(1)` dan hit-test kolom horizontal. |

---

## UI / Keymap Flow
1. **Modal Interaksi Mouse**:
   - Buka `CommandPalette` (`F1`) / `QuickOpen` (`Ctrl+P`): Klik item pada daftar untuk mengeksekusi/membuka; scroll mouse wheel atas/bawah untuk menggulir daftar; klik di luar kotak modal untuk menutup dialog secara instan.
   - Buka `FindBar` (`Ctrl+F` / `Ctrl+H`): Klik kotak input untuk memindahkan fokus; klik tombol `[↓]`, `[↑]`, `[×]`, `[Ganti]`, `[Semua]` untuk aksi langsung.
   - Buka `Help` (`?`) / `About` (`Alt+A`): Klik tombol `[ Tutup ]` atau klik di luar kotak untuk menutup.
2. **Status Bar Interaksi Mouse**:
   - Klik `[AutoSave: ON/OFF]` -> Toggle status Auto-Save seketika.
   - Klik `git: <branch>` -> Buka dialog Branch Switcher.
   - Klik `Ln X, Col Y` -> Buka dialog Go to Line.
   - Klik `?: Help` -> Buka dialog Bantuan Shortcut.
3. **Git Sidebar Header**:
   - Klik pada header `🌿 SOURCE CONTROL (<branch>)` -> Buka dialog Branch Switcher.

---

## Tasks
- [x] **Batch 1: Modal Mouse Interactivity & Visual Polish**
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/palette.go` (klik item, mouse wheel scroll, dismiss luar).
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/quickopen.go` (klik item, mouse wheel scroll, dismiss luar).
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/globalsearch.go` (klik match item, mouse wheel scroll, dismiss luar).
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/branch.go` (klik branch item, mouse wheel scroll, dismiss luar).
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/aisettings.go` (klik input Endpoint, API Key, Model, klik tombol Simpan & Batal).
  - [x] Implementasi `HandleMouse` pada `internal/ui/dialog/switcher.go` (klik tab terpilih untuk aktivasi).
  - [x] Implementasi `HandleMouse` dan perbaikan visual tombol pada `internal/ui/dialog/findbar.go`.
  - [x] Refactor `renderHelpModal()` dan `renderAboutModal()` di `internal/app/view.go` menggunakan `RoundedBorder`, warna `Accent`, `NormalizeDialogBackground`, dan tombol tutup.
  - [x] Integrasikan seluruh interceptor mouse modal di `internal/app/model.go` agar mencegah klik fallthrough.
- [x] **Batch 2: Interactive Status Bar & Git Sidebar Header**
  - [x] Implementasi hit-testing mouse pada Status Bar di `internal/app/model.go` (`AutoSave`, `Branch`, `Ln/Col`, `Help`).
  - [x] Tampilkan nama branch pada header Git Sidebar di `internal/app/view.go` dan tangani klik baris 1 di `model.go` untuk membuka branch switcher.
- [x] **Batch 3: Resource & Performance Optimizations**
  - [x] Optimasi `marqueeTickCmd` di `internal/app/model.go`: Ticking kondisional hanya saat marquee dibutuhkan.
  - [x] Optimasi syntax highlighting di `internal/editor/buffer.go` & `internal/syntax/highlighter.go`: Viewport-bounded line style parsing untuk mengeliminasi alokasi masif pada file besar.
- [x] **Batch 4: Documentation Synchronization**
  - [x] Buat file `docs/architecture.md` (arsitektur sistem, state machine, modul, diagram alir).
  - [x] Perbarui `README.md` (pohon struktur modul dan tabel keybindings lengkap).
- [x] **Batch 5: Verification Suite**
  - [x] Jalankan `go vet ./...` (wajib 0 error).
  - [x] Tambahkan unit test untuk interaksi mouse modal dan status bar (`internal/app/modal_mouse_test.go`).
  - [x] Jalankan `go test -v -race ./...` (wajib 100% lulus).

---

## Manual Test Checklist
- [ ] Buka `F1` (Command Palette), klik salah satu item perintah dengan mouse -> Perintah dieksekusi tanpa efek samping pada editor di belakangnya.
- [ ] Buka `F1`, klik di luar area modal -> Modal tertutup bersih tanpa memicu klik editor.
- [ ] Buka `Ctrl+F` (FindBar), klik tombol `[↓]` dan `[↑]` dengan mouse -> Navigasi match bekerja tanpa memindahkan kursor editor teks di belakangnya.
- [ ] Buka `?` (Help Modal), klik tombol `[ Tutup ]` -> Modal tertutup.
- [ ] Klik badge `[AutoSave: ON]` di status bar -> Berubah menjadi `[AutoSave: OFF]`.
- [ ] Klik `git: <branch>` di status bar -> Modal Branch Switcher terbuka.
- [ ] Klik `Ln X, Col Y` di status bar -> Modal Go to Line terbuka.
- [ ] Klik `?: Help` di status bar -> Modal Bantuan Shortcut terbuka.
- [ ] Klik header `🌿 SOURCE CONTROL` di sidebar -> Modal Branch Switcher terbuka.
- [ ] Idle monitoring: Aplikasi tidak me-render loop 4 FPS terus-menerus saat fokus di editor.

---

## Acceptance Criteria
- [ ] Sembilan modal yang sebelumnya tidak mendukung mouse kini dapat diklik dan menutup saat area luar diklik.
- [ ] Tidak ada event klik mouse yang tembus ke background saat dialog modal aktif.
- [ ] Modal Bantuan (`?`) dan Tentang (`Alt+A`) memiliki tampilan serasi bertema VS Code Dark dengan rounded border tunggal dan background rapi tanpa celah ANSI reset.
- [ ] Seluruh item relevan pada Status Bar dan header Git Sidebar responsif terhadap klik mouse.
- [ ] `marqueeTickCmd` tidak memicu re-render idle saat fokus di editor/terminal/git atau saat nama file tidak meluap.
- [ ] `docs/architecture.md` tersedia dan terdokumentasi lengkap.
- [ ] `README.md` sinkron dengan kode riil.
- [ ] `go vet ./...` bersih dan `go test -v -race ./...` lolos 100%.

---

## Decisions
- Menggunakan bounded viewport line parsing untuk syntax highlighting agar sesuai aturan PRD 4.6 tanpa merusak akurasi Chroma tokenizer.
- Mempertahankan konsistensi modal styling dengan RoundedBorder dan `NormalizeDialogBackground` untuk semua modal tanpa terkecuali.

## Blockers
- Tidak ada.

## Next
- Tunggu persetujuan (approval) dari user, kemudian ubah status ke `in_progress` dan mulai eksekusi Batch 1.
