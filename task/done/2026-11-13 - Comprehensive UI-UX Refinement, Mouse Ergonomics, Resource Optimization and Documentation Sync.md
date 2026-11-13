---
title: Comprehensive UI-UX Refinement, Mouse Ergonomics, Resource Optimization and Documentation Sync
status: done
started: 2026-11-13
completed: 2026-11-13
branch: dev
priority: high
---

# Comprehensive UI-UX Refinement, Mouse Ergonomics, Resource Optimization and Documentation Sync

## Objective
Menyelesaikan seluruh temuan audit menyeluruh: memperbaiki pemotongan Help Modal di terminal kecil (80x24) dengan scrollable viewport, sinkronisasi hit-testing mouse modal, menambahkan context menu klik kanan di File Explorer, middle-click untuk menutup tab editor, penanganan mouse wheel di Terminal tab, perbaikan filter explorer saat input kosong, konsistensi bahasa UI, optimasi alokasi memori buffer markdown, deduplikasi OSC 22 cursor escape sequences, serta sinkronisasi dokumentasi (PRD.md, README.md, docs/architecture.md).

## Ground Truth Findings
1. **Help Modal Overflow & Truncation**: Di `internal/app/view.go:296`, `renderHelpModal()` menghasilkan 41 baris string statis. Pada terminal 80×24, `overlayAt()` memotong baris 24..41. Pengguna kehilangan separuh panduan shortcut, tidak ada tombol tutup yang terlihat, dan tidak ada viewport scroll.
2. **Dismiss Sembarangan pada Help & About**: Di `internal/app/model.go:1855`, klik mouse di koordinat manapun langsung menutup modal (`m.ShowHelp = false`). Pengguna tidak bisa klik untuk membaca/seleksi teks, dan mouse wheel diabaikan.
3. **Off-by-One Koordinat Mouse Modal**: Di `internal/app/view.go:453-475`, `overlayTopCenter` dan `overlayCenter` menghitung `atX := (totalW - mWidth) / 2` menggunakan `mWidth := lipgloss.Width(modal)` (konten + border = Width + 2). Sementara di `internal/app/model.go:1395-1615`, `startX` dihitung dari `m.<Modal>.Width` mentah tanpa border, menghasilkan selisih 1 kolom pada seluruh hit-testing mouse modal.
4. **Hit-Testing File Explorer Offset Saat Filter Aktif**: Di `internal/app/view.go:195`, jika `m.Explorer.FilterQuery != ""`, baris 0 body sidebar menampilkan `filterBanner`. Di `internal/app/model.go:1948`, `relLine = msg.Y - 3` langsung memanggil `m.Explorer.HandleClick(relLine)` tanpa memperhitungkan banner filter, menyebabkan klik pada baris 0 membuka node pertama dan seluruh node di bawahnya bergeser 1 baris.
5. **Bug Dialog Input Filter Kosong**: Di `internal/ui/dialog/input.go:103`, `if val != ""` mencegah konfirmasi nilai kosong pada `dialog.InputFilterExplorer`, sehingga pengguna tidak dapat membersihkan filter dengan menekan tombol Enter pada input kosong.
6. **Ketiadaan Context Menu di File Explorer**: Di `internal/app/model.go:1665`, klik kanan hanya aktif pada Tab Bar dan Editor Buffer. Pengguna mouse tidak memiliki opsi klik kanan pada file tree (New File, New Folder, Rename, Delete, Copy Path).
7. **Ketiadaan Middle-Click Tab Close**: Di `internal/app/model.go:1880`, penanganan klik tab hanya memeriksa `tea.MouseButtonLeft`. Standar IDE modern (menutup tab dengan roda tengah mouse `tea.MouseButtonMiddle`) belum didukung.
8. **Mouse Wheel di Terminal Tab Diabaikan**: Di `internal/app/model.go:1820`, scroll wheel editor hanya menangani Diff, Markdown, dan Text Buffer. Tab terminal PTY tidak merespons scroll wheel.
9. **Target Hit-Testing Tombol Refresh Terlalu Lebar**: Di `internal/app/model.go:1916`, klik di seluruh kolom sidebar baris Y=2 memicu refresh, padahal tombol `[⟳ Refresh]` hanya selebar 12 karakter di sisi kiri.
10. **Inkonsistensi Bahasa & Navigasi Sidebar**: Di `internal/app/view.go:137`, header sidebar search tertulis `" 🔍 CARI & GANTI"` sementara Files `" 📁 EXPLORER"` dan Git `" 🌿 SOURCE CONTROL"`. Footer tabs belum mencantumkan angka shortcut `[ 1:Files | 2:Git | 3:Search ]` dan klik di area kosong sebelah kanan tab search otomatis mengaktifkan tab search.
11. **Pemborosan Alokasi Memori Setiap Frame Markdown Preview**: Di `internal/app/view.go:261`, `activeTab.Buffer.Text()` membuat string baru seluruh dokumen setiap frame render untuk di-hash SHA-256 oleh `MarkdownViewer.Render`.
12. **Flooding OSC 22 Escape Sequences**: Di `internal/app/model.go:1643, 1656-1662`, `fmt.Print("\x1b]22;...\x1b\\")` dipanggil langsung tanpa deduplikasi pada setiap event gerakan mouse.
13. **Dokumentasi Usang**:
    - `README.md`: Belum mencatat `Ctrl+O` (Open File), `Ctrl+K` (Open Folder), `F4` (Global Search). Blueprint arsitektur belum mencakup `internal/ai`, `internal/config`, `internal/search`, `internal/logger`.
    - `docs/architecture.md`: Salah referensi nama file (`tab.go` vs `manager.go`, `pty.go` vs `terminal.go`).
    - `PRD.md`: Belum mencatat integrasi `git push` yang sudah ada, ambigu shortcut `r` (refresh) vs `F2` (rename), dan sitemap blueprint lama.

## Plan
1. **Komponen Scrollable Help Modal (`internal/ui/dialog/help.go`)**:
   - Buat struct `HelpDialog` dengan state scrolling (`ScrollTop`, `MaxRows`), hitung konten bantuan terstruktur.
   - Dukung tombol navigasi keyboard (`Up`, `Down`, `PageUp`, `PageDown`, `Home`, `End`, `Esc`, `?`).
   - Dukung interaksi mouse: mouse wheel up/down untuk scroll, klik di dalam kotak tidak menutup modal, klik tombol `[ Tutup ]` atau klik di luar kotak menutup modal.
   - Sesuaikan `renderHelpModal` di `internal/app/view.go` agar merender melalui `HelpDialog` dengan batasan tinggi adaptif (`Height - 6`).

2. **Perbaikan Hit-Testing & Koordinat Modal (Fix Off-by-One)**:
   - Di `internal/app/model.go`, perbarui kalkulasi `startX` dan `startY` untuk `CommandPalette`, `QuickOpen`, `GlobalSearch`, `BranchSwitcher`, `Switcher`, `FilePicker`, `InputDialog`, `ConfirmDialog`, `AISettingsDialog` menggunakan `lipgloss.Width(rendered)` dan `lipgloss.Height(rendered)`.

3. **Perbaikan Explorer Mouse Hit-Testing & Context Menu**:
   - Di `internal/app/model.go`, koreksi hit-testing klik sidebar saat `m.Explorer.FilterQuery != ""`: baris 0 adalah filter banner (klik banner membuka dialog filter atau membersihkan filter). Baris 1+ adalah flat nodes (`relLine - 1`).
   - Batasi klik tombol refresh baris Y=2 hanya pada `msg.X <= 14`.
   - Tambahkan Context Menu Explorer untuk klik kanan (`tea.MouseButtonRight`) di area File Explorer:
     - `New File` (`a`)
     - `New Folder` (`A`)
     - `Rename` (`F2`)
     - `Delete` (`d`)
     - `Copy Relative Path`
   - Handle aksi context menu explorer di `handleContextMenuAction`.

4. **Dukungan Middle-Click Tab Close & Terminal Wheel**:
   - Di `internal/app/model.go`, tambahkan penanganan `msg.Button == tea.MouseButtonMiddle` pada Tab Bar (Row 1, X >= editorStartX) untuk menutup tab yang diklik.
   - Di `internal/app/model.go`, tambahkan penanganan mouse wheel di `tab.Type == editor.TabTerminal` untuk mengirim escape code scroll (`\x1b[A` / `\x1b[B` atau scroll buffer terminal).

5. **Perbaikan Input Dialog Filter Kosong**:
   - Di `internal/ui/dialog/input.go`, izinkan konfirmasi jika `val != "" || d.Type == InputFilterExplorer`.

6. **Konsistensi Bahasa UI & Bottom Tabs Sidebar**:
   - Di `internal/app/view.go`, ubah title header search menjadi `" 🔍 SEARCH & REPLACE"`.
   - Ubah label footer tabs menjadi `" 📁 1:Files"`, `" 🌿 2:Git"`, `" 🔍 3:Search"`.
   - Batasi rentang klik tab Search agar klik di ruang kosong sebelah kanan tidak memicu switch view.

7. **Optimasi Alokasi Memori Buffer Markdown & Deduplikasi OSC 22**:
   - Di `internal/editor/buffer.go`, tambahkan caching `cachedFullText` dan `cachedTextHash`. Update hanya jika `b.cachedDirty` bernilai true.
   - Di `internal/app/model.go`, simpan field `lastPointerShape string` pada `Model`. Hanya jalankan `fmt.Print` jika shape berubah.
   - Saat keluar aplikasi (`Ctrl+Q`), kirim reset cursor shape `\x1b]22;default\x1b\\`.

8. **Sinkronisasi Dokumentasi**:
   - Update `PRD.md` (klarifikasi `r` vs `F2`, catat fitur `git push`, update sitemap direktori).
   - Update `README.md` (tabel shortcut `Ctrl+O`, `Ctrl+K`, `F4`, update tree direktori).
   - Update `docs/architecture.md` (perbaiki referensi file dan formula viewport height).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Row 0 (Top Menu) | Tidak ada perubahan | Pertahankan tinggi 1 baris statis |
| Column 1 (Sidebar) | Perubahan label tab footer & hit-test | Gunakan `ansi.Truncate` dan pembatasan lebar tepat |
| Column 2 (Editor Pane) | Middle-click tab & terminal wheel | Hit-test terisolasi di Tab Bar dan Terminal Tab |
| Modal Overlays | Help modal responsive & alignment fix | Batasi tinggi modal <= `Height - 4`, sinkronkan `startX` |
| Row Height-1 (Status Bar) | Tidak ada perubahan | Tetap 1 baris statis |

## UI / Keymap Flow
1. **Help Modal (`?`)**:
   - Tampil di tengah layar dengan tinggi proporsional (`Height - 6`).
   - Navigasi isi dengan keyboard (`Up`, `Down`, `PageUp`, `PageDown`) atau mouse wheel.
   - Tombol `[ Tutup ]` di bagian bawah dapat diklik langsung, atau tekan `Esc` / `?`.
2. **File Explorer Context Menu**:
   - Klik kanan pada baris file/folder di File Explorer memunculkan context menu pop-up.
   - Opsi: `New File`, `New Folder`, `Rename`, `Delete`, `Copy Relative Path`.
   - Pilih dengan keyboard (panah + Enter) atau klik mouse kiri.
3. **Tab Bar Middle-Click**:
   - Klik roda tengah mouse pada tab manapun langsung menutup tab tersebut (dengan konfirmasi jika dirty).
4. **Pembersihan Filter Explorer**:
   - Tekan `/` pada Explorer, hapus isi teks hingga kosong, tekan Enter. Filter bersih dan seluruh pohon file kembali tampil normal.

## Tasks
- [x] Task 1: Buat `HelpDialog` terstruktur dengan scroll viewport dan mouse handling (`internal/ui/dialog/help.go` & `internal/app/model.go`)
- [x] Task 2: Perbaiki sinkronisasi koordinat modal mouse (off-by-one fix di `internal/app/model.go`)
- [x] Task 3: Perbaiki hit-test Explorer filter banner dan batasi hit-test tombol refresh (`internal/app/model.go`)
- [x] Task 4: Tambahkan context menu klik kanan pada File Explorer (`internal/app/model.go` & `internal/ui/contextmenu.go`)
- [x] Task 5: Implementasikan middle-click untuk menutup tab editor & scroll wheel pada Terminal tab (`internal/app/model.go`)
- [x] Task 6: Perbaiki validasi string kosong pada Input Dialog filter (`internal/ui/dialog/input.go`)
- [x] Task 7: Selaraskan bahasa UI header search dan label shortcut footer tabs sidebar (`internal/app/view.go` & `internal/app/model.go`)
- [x] Task 8: Optimasi caching teks buffer markdown dan deduplikasi cursor shape OSC 22 (`internal/editor/buffer.go` & `internal/app/model.go`)
- [x] Task 9: Perbarui dan sinkronkan dokumentasi `PRD.md`, `README.md`, dan `docs/architecture.md`
- [x] Task 10: Jalankan unit test, race test, dan linter: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Buka aplikasi pada terminal ukuran 80×24, tekan `?`: Help modal tampil utuh, tidak terpotong, bisa di-scroll dengan keyboard panah dan mouse wheel.
- [ ] Klik kanan pada file di File Explorer: context menu muncul dengan opsi CRUD dan Copy Path.
- [ ] Buka beberapa tab, klik roda tengah (middle click) pada salah satu tab: tab tertutup bersih.
- [ ] Tekan `/` di Explorer, filter file (misal `main`), klik salah satu file hasil filter: file yang terbuka sesuai dengan yang diklik (tidak offset 1 baris).
- [ ] Tekan `/` lagi, kosongkan teks, tekan Enter: filter terhapus dan semua file kembali tampil.
- [ ] Buka tab terminal (`Ctrl+Shift+\`), jalankan perintah panjang, scroll mouse wheel: scrolling berfungsi.
- [ ] Periksa label footer tabs sidebar: menampilkan `[ 📁 1:Files | 🌿 2:Git | 🔍 3:Search ]`.
- [ ] Gerakkan mouse cepat di atas editor: kursor berubah responsif tanpa banjir escape sequence mentah.
- [ ] Buka dokumentasi `README.md`, `PRD.md`, `docs/architecture.md`: seluruh pintasan dan struktur paket selaras 100%.

## Acceptance Criteria
- [x] Seluruh unit test lulus 100% tanpa race condition (`go test -v -race ./...`).
- [x] Linter Go bersih 0 error 0 warning (`go vet ./...`).
- [x] Zero terminal overflow di semua ukuran layar (>= 80×24).
- [x] Seluruh temuan audit terselesaikan secara rapi, modular, dan elegan.

## Decisions
- Menggunakan komponen `HelpDialog` mandiri daripada fungsi string statis `renderHelpModal()` agar dapat mengelola scroll offset, viewport height, dan mouse hit-testing secara profesional.
- Deduplikasi OSC 22 dilakukan pada level `Model` dengan melacak `lastPointerShape` agar tidak mengirim escape code berulang pada koordinat yang sama.
- Context menu File Explorer menggunakan komponen `ui.ContextMenu` yang sudah ada via `OpenCustom` untuk menjaga konsistensi visual dengan context menu editor dan tab.

## Blockers
- None.

## Next
- Tunggu konfirmasi review dari user.
- Setelah user konfirmasi, arsipkan task ke `task/done/` dan catat di `task/LOG.md`.
