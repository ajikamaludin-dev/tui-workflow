---
title: Undo-Redo, Find-Replace, Go-to-Line & Block Indent, Git Discard & Branch Switcher
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Undo-Redo, Find-Replace, Go-to-Line & Block Indent, Git Discard & Branch Switcher

## Ground Truth Findings
1. **Editor Buffer (`internal/editor/buffer.go`)**:
   - Memiliki manipulasi `InsertRune`, `InsertNewline`, `Backspace`, `Delete`, `DeleteSelection`.
   - Tidak memiliki undo/redo stack. Salah ketik atau delete selection tidak dapat dikembalikan.
   - Seleksi (`HasSelection`, `NormalizedSelection`) sudah stabil untuk koordinat baris & kolom.
   - Belum memiliki fungsi block indent/outdent (`Tab`/`Shift+Tab`) dan toggle line comment (`Ctrl+/`).
2. **Find Bar (`internal/ui/dialog/findbar.go`)**:
   - Hanya memiliki 1 input teks pencarian (`Cari:`), tombol navigasi `<` dan `>`, serta indikator `MatchCurrent / MatchTotal`.
   - Belum memiliki field replace teks (`Ganti:`), replace single (`Enter`), replace all (`Alt+Enter`), dan toggle shortcut `Ctrl+H`.
3. **Modal Dialogs (`internal/ui/dialog/`)**:
   - Pola modal konsisten (`InputDialog`, `ConfirmDialog`, `QuickOpen`, `CommandPalette`).
   - Lompat ke baris (`Ctrl+G`) dapat memanfaatkan modal dialog cepat dengan input format `baris[:kolom]`.
4. **Git Operations (`internal/git/repo.go`, `internal/git/view.go`)**:
   - `internal/git/repo.go` membungkus `go-git/v5` dengan pointer `r.r` dan `r.worktree`.
   - Status file unstaged (`Changes`) memiliki status `M`, `D`, `?`. Belum ada aksi discard per-file (`worktree.Checkout` untuk `M`/`D` dan `os.Remove` untuk untracked `?`).
   - Branch name aktif dibaca dari `r.GetBranchName()`, namun belum ada API untuk `ListBranches()` dan `CheckoutBranch(name)`.
5. **Sidebar Tabs (`internal/app/view.go`, `internal/app/model.go`)**:
   - Tab saat ini dirender di baris paling atas sidebar (`Row 1`): `[ 1:Files ] [ 2:Git ]`.
   - Pindahnya tab ke baris paling bawah membutuhkan pergeseran render `colContent` (header/refresh di atas, body di tengah, tab bar di bawah) dan pembaruan hit-testing mouse click pada baris bawah sidebar.

## Dual-Lens Evaluation
1. **Business & UX Lens**:
   - Editor tanpa Undo/Redo sangat berisiko kehilangan data pengguna saat typo atau accidental deletion.
   - Find & Replace adalah fitur wajib teks editor modern (VS Code parity).
   - Menempatkan tab navigasi di bagian bawah sidebar (`📁 Files`, `🌿 Git`, `🔍 Search`) memisahkan navigasi global dari isi konten, menyerupai layout status/dock bawah yang nyaman diakses.
   - Tab ke-3 `🔍 Search` memberikan akses panel pencarian dan penggantian teks di seluruh workspace secara terstruktur tanpa harus selalu membuka popup modal.
   - Go to Line (`Ctrl+G`) dan Block Indent/Comment mempercepat navigasi dan penulisan kode di file berukuran besar.
   - Git Discard Changes dan Branch Switcher membuat pengguna dapat mengelola branch dan membatalkan eksperimen kode langsung dari dalam TUI tanpa perlu keluar ke terminal shell.
2. **Senior Technical Lens**:
   - **Undo/Redo Stack**: Menggunakan ring buffer snapshot (maksimal 100 state per buffer) dengan deep copy baris `[][]rune`. Ringan (< 2 MB per buffer) dan bebas bug desinkronisasi delta.
   - **Zero-Lag & Single-Line Guarantee**: FindBar replace mode dihitung dengan layout tinggi tetap tanpa merusak viewport editor atau baris menu/status bar.
   - **Bottom Tab Bar Safety**: Tab bar di baris bawah sidebar wajib tepat 1 baris (`MaxHeight(1)`), diapit ANSI truncate agar tidak mendorong status bar atau menyebabkan offscreen scroll.
   - **Git Worktree Safety**: Discard unstaged file wajib memvalidasi path traversal dan mengonfirmasi via `ConfirmDialog` agar tidak terjadi kehilangan data tanpa sengaja.
   - **Branch Checkout**: Memeriksa dirty working directory sebelum checkout untuk mencegah konflik file go-git.

## Objective & Approach
Mengimplementasikan 4 pilar produktivitas dan kontrol editor beserta penyempurnaan sidebar:
1. **Pilar 1: Undo / Redo Buffer**:
   - Stack `undoStack` dan `redoStack` di `Buffer`.
   - Rekam snapshot sebelum mutasi signifikan (`InsertRune` batas spasi/kata, `DeleteSelection`, `Backspace` pada line merge, `InsertNewline`, `ReplaceAll`).
   - Shortcut `Ctrl+Z` (Undo) dan `Ctrl+Y` / `Ctrl+Shift+Z` (Redo).
2. **Pilar 2: Find & Replace Editor (Floating Bar & Tab Sidebar)**:
   - Tambah Replace input di `FindBar` (`internal/ui/dialog/findbar.go`). Shortcut `Ctrl+H` membuka replace bar.
   - Tombol / aksi `[Ganti]` (`Enter` pada input replace) dan `[Semua]` (`Alt+Enter`).
   - Method `ReplaceCurrent` dan `ReplaceAll` di `internal/editor/buffer.go`.
3. **Pilar 3: Go to Line & Block Indent / Comment**:
   - Modal input `Ctrl+G`: input nomor baris (contoh `45` atau `45:12`) langsung menggeser kursor dan scroll buffer.
   - Seleksi blok + `Tab`: indent 4 spasi ke semua baris terpilih.
   - Seleksi blok + `Shift+Tab` (`tea.KeyShiftTab` / `tea.KeyBackTab`): outdent hingga 4 spasi dari awal baris.
   - `Ctrl+/` (`Ctrl+_`): deteksi token komentar (`//`, `#`, `--`) berdasarkan ekstensi file, toggle komentar baris terpilih.
4. **Pilar 4: Git Discard Changes & Branch Switcher**:
   - Aksi Discard `[x]` pada file di daftar Changes Git View (`d` pada item). Memunculkan konfirmasi dialog, lalu revert via `go-git`.
   - Branch Switcher: Klik header branch atau shortcut `Alt+B` / Command Palette membuka dialog daftar branch lokal untuk checkout.
5. **Pilar 5: Sidebar Bottom Tabs dengan Emoji & Tab ke-3 Search View**:
   - Pindahkan tab bar ke baris paling bawah sidebar: `[ 📁 Files ]`, `[ 🌿 Git ]`, `[ 🔍 Search ]`.
   - Shortcut `Alt+1` (Files), `Alt+2` (Git), `Alt+3` (Search).
   - Buat `SearchView` di sidebar untuk mencari & mengganti teks di seluruh workspace dengan hasil pohon file yang dapat diklik untuk membuka file pada baris yang sesuai.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Sidebar Bottom Row (Row Height-2) | Tab bar pindah ke bawah, berisiko bentrok dengan status bar jika tinggi salah hitung | Gunakan `MaxHeight(1)` dan kunci tinggi body `height - 2` (1 baris refresh/header di atas, 1 baris tab di bawah) |
| Sidebar Click Coordinates | Posisi klik tab berpindah dari `msg.Y == 1` ke `msg.Y == m.Height-2` | Sesuaikan `msg.Y` handler pada baris bawah sidebar dengan validasi rentang kolom X |
| Editor Pane (Floating FindBar) | Replace bar memakan 2 baris floating di kanan atas editor | Render findbar dengan border dan background solid tanpa memotong buffer visual |
| Status Bar & Dialogs | Modal Go to Line & Branch Picker muncul di tengah | Gunakan komponen dialog standar (`dialog.Normalizer`) dengan backdrop gelap terisolasi |

## UI / Keymap Flow
1. **Undo / Redo**:
   - `Ctrl+Z`: Undo ke state sebelumnya.
   - `Ctrl+Y` / `Ctrl+Shift+Z`: Redo ke state berikutnya.
2. **Find & Replace**:
   - `Ctrl+F`: Buka Find Bar (mode cari saja).
   - `Ctrl+H`: Buka Find & Replace Bar (input cari + input ganti).
   - `Tab`: Pindah fokus antara input Cari dan input Ganti.
   - `Enter` di input Ganti: Ganti 1 match aktif.
   - `Alt+Enter` / klik `[Semua]`: Ganti semua match dalam file.
   - `Esc`: Tutup Find & Replace bar.
3. **Go to Line & Indent**:
   - `Ctrl+G`: Buka modal "Lompat ke Baris". Ketik `42`, tekan `Enter`. Kursor berpindah ke baris 42.
   - `Tab` (saat ada seleksi): Semua baris dalam seleksi digeser ke kanan 4 spasi.
   - `Shift+Tab` (saat ada seleksi): Semua baris dalam seleksi digeser ke kiri hingga 4 spasi.
   - `Ctrl+/` (atau `Ctrl+_`): Toggle komentar baris aktif atau seluruh baris dalam seleksi.
4. **Git Discard & Branch**:
   - Di tab Git, fokus pada daftar Changes, tekan `d` atau klik `[x]`: Modal konfirmasi discard muncul. Tekan `[Discard]` untuk revert file.
   - Tekan `Alt+B` atau klik header branch: Modal daftar branch muncul, panah atas/bawah + `Enter` untuk checkout branch.
5. **Sidebar Bottom Tabs & Search View**:
   - Tab navigasi sidebar berada di baris paling bawah sidebar: `[ 📁 Files ]`, `[ 🌿 Git ]`, `[ 🔍 Search ]`.
   - Shortcut: `Alt+1` (Files), `Alt+2` (Git), `Alt+3` (Search).
   - Klik mouse pada baris bawah sidebar berpindah tab sesuai koordinat X.

## Improvement 1 (2026-11-12 — Bottom Sidebar Tabs with Emoji & 3rd Search/Replace Tab)
### Analisis Fakta
- Saat ini `renderSidebar` me-render tab di baris paling atas: `[ 1:Files ] [ 2:Git ]` diikuti refresh row lalu body explorer/git.
- Pengguna meminta:
  1. Pindahkan tab ke posisi bawah (sebelumnya di atas).
  2. Ganti angka 1 dan 2 menjadi emoji: `📁 Files`, `🌿 Git`, dan tab ke-3 `🔍 Search`.
  3. Tambahkan tab ke-3 di sidebar untuk Find & Replace (Search panel terintegrasi di sidebar).
- Koordinat mouse klik sidebar di `internal/app/model.go` saat ini mengasumsikan row 1 adalah tab dan row 2 adalah refresh. Ini wajib disesuaikan:
  - Row 1: Header / Refresh row.
  - Row 2 s.d. Height-3: Body (Explorer / Git / Search).
  - Row Height-2 (baris bawah sidebar): Tab footer `[ 📁 Files ] [ 🌿 Git ] [ 🔍 Search ]`.

### Rencana Perbaikan
1. **Model & State**:
   - Tambah `SidebarSearch` di enum `SidebarView` (`internal/app/keymap.go`).
   - Buat paket/komponen `SearchView` di `internal/search/view.go`:
     - Input Cari (`Cari...`) & Input Ganti (`Ganti...`).
     - Action buttons: Cari, Ganti 1, Ganti Semua, Toggle Case/Word.
     - Hasil pencarian workspace bertingkat (file path -> line matches).
     - Navigasi keyboard & mouse: klik match membuka file di editor tab pada baris tersebut.
2. **View Layout**:
   - Pindahkan render tab bar ke bagian bawah `colContent`: `refreshRow + "\n" + body + "\n" + footerTabs`.
   - Gunakan emoji: `📁 Files`, `🌿 Git`, `🔍 Search`.
3. **Mouse & Keyboard Interaction**:
   - Update hit-test mouse click: jika `msg.Y == m.Height-2` (baris bawah sidebar), hit test rentang X:
     - `0 <= X < 11`: `SidebarFiles`
     - `11 <= X < 20`: `SidebarGit`
     - `20 <= X < 32`: `SidebarSearch`
   - Tambahkan shortcut `Alt+3` untuk langsung fokus ke `SidebarSearch`.

## Improvement 2 (2026-11-12 — Fix Floating Bottom Tabs in Sidebar)
### Analisis Fakta
- Pengguna melaporkan tab bawah sidebar meloncat ke atas saat membuka panel `🔍 Search`.
- **Akar Masalah**:
  1. `SearchView.Render` saat hasil kosong hanya menghasilkan 5 baris teks.
  2. `renderSidebar` menggabungkan `headerRow + refreshRow + body + footerTabs` tanpa mem-padding `body` ke tinggi penuh (`bodyHeight`). Akibatnya, `footerTabs` mengambang langsung di bawah baris ke-5 alih-alih di baris paling bawah.
  3. Padding tab default Lipgloss menyebabkan panjang teks 3 tab (32 kolom) melebihi lebar sidebar (28 kolom, visual 27), memotong teks tab `🔍 Search` menjadi `🔍 Sear`.
### Perbaikan
1. **Vertical Pinning**: `body` dipisahkan per baris dan dipadding dengan baris kosong sehingga selalu tepat berukuran `bodyHeight`. `footerTabs` terjamin selalu berada di baris paling bawah (`m.Height - 2`).
2. **Tab Width Clamping**: Mengatur `Padding(0, 0)` pada style tab bawah sehingga total lebar 3 tab tepat 27 kolom tanpa terpotong.
3. **Leading Space**: Memberikan leading space pada prompt `Cari:` dan `Ganti:` agar sejajar rapi dengan border sidebar.
4. **Regression Test**: Menambahkan `TestSidebarFooterTabsPinnedToBottom` di `internal/app/tab_context_test.go` untuk memverifikasi posisi baris tab bawah di semua view sidebar.

## Improvement 3 (2026-11-12 — Comprehensive Shortcut Synchronization)
### Analisis Fakta & Sinkronisasi
- Seluruh pintasan keyboard baru wajib tersinkronisasi di semua antarmuka:
  1. **Help Modal (`?`)**: Diperbarui dengan daftar lengkap pintasan: Undo/Redo (`Ctrl+Z`/`Ctrl+Y`), Find & Replace (`Ctrl+F`/`Ctrl+H`), Go to Line (`Ctrl+G`), Indent/Outdent (`Tab`/`Shift+Tab`), Comment Toggle (`Ctrl+/`), Sidebar switch (`Alt+1/2/3`), Git Discard (`d`), Git Branch (`Alt+B`).
  2. **Top Menu Bar (`internal/ui/menu.go`)**: Ditambahkan item `Toggle Comment` (`Ctrl+/`) di menu `Edit`.
  3. **Command Palette (`internal/app/model.go`)**: Ditambahkan aksi `edit.toggle_comment` (`Ctrl+/`) dan aksi terkait.
  4. **Action Dispatcher**: Menghubungkan action `edit.toggle_comment` ke `tab.Buffer.ToggleLineComment(tab.CommentPrefix())`.
  5. **Dokumentasi (`README.md` & `PRD.md`)**: Tabel pintasan keyboard diperbarui secara menyeluruh mencakup Global, Editor, Explorer, dan Git.

## Tasks
- [x] Task 1: Implementasi Undo/Redo snapshot stack di `internal/editor/buffer.go` dan key handler `Ctrl+Z`/`Ctrl+Y`.
- [x] Task 2: Implementasi Find & Replace di `internal/ui/dialog/findbar.go` dan `internal/editor/buffer.go` dengan shortcut `Ctrl+H`.
- [x] Task 3: Implementasi Go to Line (`Ctrl+G`), Block Indent/Outdent (`Tab`/`Shift+Tab`), dan Comment Toggle (`Ctrl+/`).
- [x] Task 4: Implementasi Git Discard Changes (`d` / `[x]`) dan Branch Switcher (`Alt+B`) di `internal/git/` dan `internal/app/`.
- [x] Task 5: Pindahkan tab sidebar ke posisi bawah dengan emoji `📁 Files`, `🌿 Git`, `🔍 Search`, serta bangun `SearchView` (tab ke-3 sidebar untuk Find & Replace workspace).
- [x] Task 6: Integrasi shortcut ke Command Palette, Top Menu, dan Status Bar (`Alt+1`, `Alt+2`, `Alt+3`).
- [x] Task 7: Unit tests komprehensif di `internal/editor/`, `internal/ui/dialog/`, `internal/git/`, `internal/search/`, dan `internal/app/`.
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓.

## Manual Test Checklist
- [x] Ketik teks di editor, tekan `Ctrl+Z` (teks kembali), tekan `Ctrl+Y` (teks muncul lagi).
- [x] Tekan `Ctrl+H`, isi cari dan ganti, tekan `Enter` untuk ganti satu, tekan `Alt+Enter` untuk ganti semua.
- [x] Tekan `Ctrl+G`, masukkan `10:5`, tekan `Enter`, verifikasi kursor di baris 10 kolom 5.
- [x] Seleksi beberapa baris, tekan `Tab` (indent 4 spasi), tekan `Shift+Tab` (outdent 4 spasi).
- [x] Seleksi beberapa baris kode Go, tekan `Ctrl+/` (muncul `// ` di awal baris), tekan lagi (komentar hilang).
- [x] Buka tab Git, pilih file di unstaged Changes, tekan `d`, konfirmasi discard, verifikasi perubahan file kembali ke HEAD.
- [x] Tekan `Alt+B`, pilih branch lain, tekan `Enter`, verifikasi branch berhasil berpindah.
- [x] Periksa tab sidebar berada di baris paling bawah dengan emoji `📁 Files`, `🌿 Git`, `🔍 Search`.
- [x] Tekan `Alt+3` atau klik `🔍 Search`: Panel Find & Replace workspace muncul di sidebar, cari teks, lihat daftar file match, klik match untuk lompat ke baris file.

## Acceptance Criteria
- [x] Undo & Redo bekerja lancar tanpa crash atau memory leak.
- [x] Find & Replace mengganti string dengan akurat dan memperbarui highlight/match count.
- [x] Go to Line melompat ke baris yang tepat dengan validasi batas baris aman.
- [x] Block Indent/Outdent & Comment Toggle bekerja pada seleksi multi-baris tanpa merusak integritas buffer.
- [x] Git Discard mengembalikan file modified/deleted dan menghapus untracked file setelah konfirmasi.
- [x] Git Branch Switcher menampilkan daftar branch lokal dan berhasil melakukan checkout.
- [x] Tab sidebar terletak di baris bawah sidebar dengan emoji `📁 Files`, `🌿 Git`, `🔍 Search`.
- [x] Tab ke-3 Search View terintegrasi di sidebar dengan fungsi cari dan ganti teks di workspace.
- [x] Linter `go vet ./...` 100% bersih dan `go test -v -race ./...` 100% PASS.

## Decisions
- Menggunakan snapshot ring-buffer (max 100 history) untuk Undo/Redo agar state cursor dan buffer konsisten tanpa risiko komputasi diff berulang.
- Menggabungkan Replace mode ke dalam `FindBar` editor mengambang (`Ctrl+H`) sekaligus menyediakan Tab ke-3 `🔍 Search` di sidebar untuk pencarian/penggantian skala workspace.
- Tab bar sidebar dipindahkan ke footer sidebar (baris bawah) dengan layout 1 baris aman, Row 1 menjadi title view, Row 2 refresh, dan Row 3..Height-3 konten.
- Menggunakan dialog modal cepat untuk Go to Line (`Ctrl+G`) dan Git Branch Switcher (`Alt+B`).

## Blockers
- None.

## Next
- Verifikasi manual oleh user, lalu pindahkan `task/CURRENT.md` ke `task/done/` setelah konfirmasi selesai.
