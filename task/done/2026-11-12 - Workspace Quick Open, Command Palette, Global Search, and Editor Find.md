---
title: Workspace Quick Open, Command Palette, Global Search, and Editor Find
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Workspace Quick Open, Command Palette, Global Search, and Editor Find

## Ground Truth Findings
1. **Buffer Switcher State (`internal/ui/dialog/switcher.go` & `internal/app/model.go`)**:
   - `BufferSwitcher` saat ini hanya menerima `[]SwitcherItem` dari `TabManager.Tabs` saat `Ctrl+P` ditekan.
   - Tidak memindai workspace root; pencarian hanya memeriksa judul tab dan path tab yang sudah terbuka.
2. **Menu & Action Dispatching (`internal/ui/menu.go` & `internal/app/model.go`)**:
   - `MenuBar` memiliki daftar `MenuItem` statis di bawah kategori File, Setting, dan About.
   - Aksi dieksekusi via `handleMenuAction(action string) tea.Cmd`.
   - Belum ada modal Command Palette terpusat (`Ctrl+Shift+P` / `F1`) yang mengumpulkan seluruh aksi aplikasi dan mengeksekusinya secara fuzzy search.
3. **Editor Buffer Navigation & Rendering (`internal/editor/buffer.go` & `internal/app/view.go`)**:
   - `Buffer` menyimpan baris dalam `Lines [][]rune`, kursor di `CursorRow` dan `CursorCol`, viewport scroll di `ScrollRow`.
   - `Render` merender gutter, kursor I-beam, dan visual selection. Belum ada field dan styling untuk highlight pencarian teks (`SearchQuery`, `SearchMatches`, active match index).
   - Belum ada overlay search bar `Ctrl+F` di editor pane untuk navigasi `next` / `prev` match.
4. **Workspace Traversal & File Safety (`internal/explorer/`, `internal/ui/dialog/filepicker.go`)**:
   - Pemindaian workspace wajib mematuhi aturan keamanan: lewati folder `.git`, `node_modules`, `vendor`, file biner (null byte pada 512B pertama), dan file > 10 MB.
   - Belum ada dialog Global Search (`Ctrl+Shift+F`) untuk mencari kecocokan teks di seluruh file workspace dan melompat ke file:baris:kolom.

## Objective & Approach
Mengimplementasikan 4 fitur pencarian dan navigasi utama:
1. **Quick Open File (`Ctrl+P` - No 2)**: Meng-upgrade Buffer Switcher menjadi Workspace Quick Open yang menampilkan tab terbuka di urutan atas diikuti seluruh file workspace dengan fuzzy search.
2. **Command Palette (`Ctrl+Shift+P` / `F1` - No 3)**: Modal pencarian aksi aplikasi berbasis fuzzy search dengan eksekusi langsung via action dispatcher.
3. **Global Search in Files (`Ctrl+Shift+F` - No 1)**: Modal pencarian konten teks di seluruh workspace dengan cuplikan baris, file:baris, dan lompat langsung ke lokasi kecocokan di editor.
4. **In-Editor Find (`Ctrl+F` - No 22)**: Search bar di editor buffer aktif dengan hitungan match `[X/N]`, navigasi Next/Prev, dan highlighting kecocokan secara real-time.

## Files Affected & Execution Steps
1. **`internal/ui/dialog/quickopen.go` (Upgrade / replace `switcher.go`)**:
   - Buat `QuickOpen` dialog yang mendukung scan file workspace (skip `.git`, `vendor`, `node_modules`).
   - Tampilkan tab aktif bertanda `[Open]` dan file workspace lainnya dengan path relatif.
   - Fuzzy filter match score.
2. **`internal/ui/dialog/palette.go` (New)**:
   - Definisikan `CommandPaletteItem { ID, Title, Category, Shortcut }`.
   - Input text filter fuzzy, render daftar aksi, return action ID saat dipilih.
3. **`internal/ui/dialog/globalsearch.go` (New)**:
   - Dialog pencarian teks global workspace.
   - Input query text, jalankan pemindaian teks pada file yang valid.
   - Tampilkan daftar match `RelPath:Line:Col` + baris preview teks.
   - Saat dipilih, kembalikan target file, baris, dan kolom untuk dibuka di editor.
4. **`internal/editor/buffer.go` & `internal/editor/search.go`**:
   - Tambahkan state pencarian di `Buffer`: `SearchQuery`, `SearchMatches []MatchPos`, `ActiveMatchIdx`.
   - Method `SetSearchQuery(q string)`, `FindNext()`, `FindPrev()`, `ClearSearch()`.
   - Update `Buffer.Render()` untuk mewarnai rune yang cocok dengan highlight style.
5. **`internal/app/model.go` & `internal/app/view.go`**:
   - Registrasikan shortcut: `Ctrl+P` (Quick Open), `Ctrl+Shift+P` / `F1` (Command Palette), `Ctrl+Shift+F` (Global Search), `Ctrl+F` (In-Editor Find).
   - Update `Update()` untuk menangani event input dan navigasi masing-masing dialog.
   - Update `View()` untuk merender overlay dialog dan search bar editor.
   - Update jump cursor logic saat membuka hasil global search atau navigasi match editor.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu Bar (Row 0) | Tertekan jika modal overlay meluap | Gunakan `placeCenter` dengan background dialog ternormalisasi dan ukuran clamped |
| Editor Pane (Row 2 to Height-2) | In-Editor search bar memakan baris | Gunakan floating panel 1-2 baris atau baris di bagian atas/bawah buffer tanpa menggeser gutter |
| Status Bar (Row Height-1) | Overflow saat menampilkan toast match | Potong pesan toast dengan `ansi.Truncate` tepat 1 baris |

## UI / Keymap Flow
- `Ctrl+P`: Buka Quick Open workspace. Ketik nama file -> Enter untuk buka/pindah tab. Esc untuk tutup.
- `Ctrl+Shift+P` / `F1`: Buka Command Palette. Ketik nama aksi -> Enter untuk eksekusi. Esc untuk tutup.
- `Ctrl+Shift+F`: Buka Global Search. Ketik query pencarian teks -> Enter pada list hasil untuk buka file pada baris kecocokan. Esc untuk tutup.
- `Ctrl+F`: Buka Find bar di buffer aktif. Ketik kata -> Enter / Down ke match berikutnya, Shift+Enter / Up ke match sebelumnya, Esc untuk keluar.

## Tasks
- [x] Task 1: Implementasi `QuickOpen` dialog (scan workspace + open tabs + fuzzy match) menggantikan `switcher.go`
- [x] Task 2: Implementasi `CommandPalette` dialog (`Ctrl+Shift+P` / `F1`) dan integrasi dispatcher aksi
- [x] Task 3: Implementasi In-Editor Find di `internal/editor/` (`Ctrl+F`, highlight match, next/prev navigation)
- [x] Task 4: Implementasi `GlobalSearch` dialog (`Ctrl+Shift+F`) untuk pencarian teks seluruh workspace
- [x] Task 5: Integrasi event handling, shortcut global, dan overlay rendering di `internal/app/`
- [x] Task 6: Unit tests & regression tests untuk QuickOpen, CommandPalette, GlobalSearch, dan In-Editor Find
- [x] Verification: go vet ./... ✓, go test -v -race ./... ✓

## Manual Test Checklist
- [x] Buka `Ctrl+P`: Ketik sebagian nama file di folder dalam workspace -> pilih -> file terbuka di tab editor.
- [x] Buka `Ctrl+Shift+P` / `F1`: Ketik "Toggle Sidebar" atau "Save" -> Enter -> aksi tereksekusi.
- [x] Di editor aktif, tekan `Ctrl+F`: Ketik kata yang ada di buffer -> match ter-highlight, tekan Enter untuk lompat ke kecocokan berikutnya.
- [x] Tekan `Ctrl+Shift+F`: Ketik kata -> muncul daftar file dan baris kecocokan -> Enter -> file terbuka dan kursor berada di baris kecocokan.

## Acceptance Criteria
- [x] Quick Open (`Ctrl+P`) dapat mencari file di seluruh workspace dan tab yang aktif.
- [x] Command Palette (`Ctrl+Shift+P`) menampilkan semua action menu dan dapat dieksekusi secara fuzzy.
- [x] Global Search (`Ctrl+Shift+F`) memindai isi file workspace dan melompat ke posisi baris/kolom.
- [x] In-Editor Search (`Ctrl+F`) menyorot kecocokan di buffer dan mendukung navigasi Next/Prev.
- [x] `go vet ./...` dan `go test -v -race ./...` lulus 100% tanpa race condition.

## Decisions
- Meng-upgrade `BufferSwitcher` menjadi `QuickOpen` agar shortcut `Ctrl+P` tetap konsisten dan mempertahankan fungsionalitas tab switcher yang sudah ada sambil menambahkan file workspace.
- Menambahkan fallback shortcut `F1` untuk Command Palette karena beberapa emulator terminal memetakan `Ctrl+Shift+P` secara berbeda.
- Menambahkan fallback shortcut `Ctrl+F` di Global Search dialog jika terminal memetakan `Ctrl+Shift+F` sebagai `ctrl+f` saat ditekan dengan modifier shift.

## Blockers
- None.

## Next
- Tunggu konfirmasi user untuk review manual Improvement 2.

## Improvement 2 (2026-11-12 — Menu Bar & Dedicated Function Keys for Palette & Global Search)
### Ground Truth Findings
1. Sebagian besar emulator terminal (misalnya xterm, Linux virtual console, dsb) tidak mengirim sequence escape unik untuk kombinasi tombol `Ctrl+Shift+P` dan `Ctrl+Shift+F`.
2. Menu Bar saat ini (`internal/ui/menu.go`) hanya memiliki menu `File`, `Setting`, dan `About`. Belum ada kategori menu `Search` terdedikasi di baris 0 (Top Menu).
3. Tombol fungsi standar (`F1` s/d `F12`) didukung secara universal oleh hampir seluruh terminal tanpa ketergantungan pada modifier `Shift` atau `Ctrl`.

### Plan
1. Tambahkan kategori menu `Search` (`Alt+R`) di `internal/ui/menu.go` dengan item:
   - `Command Palette...` (`F1`) -> `search.palette`
   - `Quick Open File...` (`Ctrl+P`) -> `search.quick_open`
   - `Find in Current File...` (`Ctrl+F`) -> `search.find`
   - `Find in Files (Global)...` (`F4`) -> `search.global`
2. Tambahkan `Quick Open...` (`Ctrl+P`) di menu `File`.
3. Update `HandleKey` di `internal/ui/menu.go` untuk menangani `Alt+R` (membuka menu `Search`).
4. Update `internal/app/model.go` agar tombol `F4` langsung memicu `openGlobalSearch()`.
5. Update `renderHelpModal()` di `internal/app/view.go` untuk memperjelas shortcut `F1` dan `F4`.
6. Verifikasi `go vet ./...` dan `go test -v -race ./...`.

### Tasks
- [x] Tambahkan kategori menu `Search` dan itemnya di `internal/ui/menu.go`
- [x] Tambahkan handler shortcut `Alt+R` di `internal/ui/menu.go`
- [x] Tambahkan shortcut `F4` untuk Global Search di `internal/app/model.go`
- [x] Update help modal shortcut di `internal/app/view.go`
- [x] Verifikasi `go vet ./...` dan `go test -v -race ./...`


## Improvement 1 (2026-11-12 — Floating Overlay & 80% Modal Width)
### Ground Truth Findings
1. `placeCenter()` di `internal/app/view.go` menggunakan `lipgloss.Place()` yang mengabaikan `baseView` dan mengisi seluruh area terminal dengan background kosong/blank.
2. `overlayAt()` di `internal/app/view.go` sudah mendukung slicing ANSI-safe baris per baris.
3. Modal dialog `QuickOpen`, `CommandPalette`, dan `GlobalSearch` memiliki ukuran lebar fixed (60 atau 70 kolom) dan tidak merespons perubahan ukuran terminal secara dinamis.

### Plan
1. Ubah `placeCenter` atau buat fungsi `overlayCenter(base, modal string, totalW, totalH int, topRatio float64) string` menggunakan `overlayAt`.
   - Hitung `atX = (totalW - modalW) / 2`
   - Hitung `atY = (totalH - modalH) / 3` (atau 1/3 untuk QuickOpen/Palette, dan 1/2 untuk centered dialogs).
   - Pasang modal di atas `baseView` sehingga editor dan komponen latar belakang tetap tampak di sekeliling modal.
2. Update `QuickOpen`, `CommandPalette`, dan `GlobalSearch` agar mendukung dynamic width:
   - Tambahkan method `SetWidth(w int)` di `QuickOpen`, `CommandPalette`, dan `GlobalSearch`.
   - Pada saat inisialisasi / `WindowSizeMsg` / saat dialog dibuka: set lebar modal ke `int(float64(m.Width) * 0.8)` (dibatasi minimal 50 dan maksimal `m.Width - 4`).
   - Sesuaikan lebar input filter `Filter.Width` dan item rendering agar mengisi 80% lebar layar secara proporsional.
3. Jalankan `go vet ./...` dan `go test -race ./...`.

### Tasks
- [x] Implementasi dynamic width `SetWidth(w int)` di `QuickOpen`, `CommandPalette`, dan `GlobalSearch`
- [x] Ubah `placeCenter` / `overlayCenter` di `internal/app/view.go` agar menggunakan `overlayAt` sehingga editor di background tetap terlihat
- [x] Update `internal/app/model.go` untuk memperbarui lebar dialog saat window resize dan saat dibuka
- [x] Verifikasi `go vet ./...` dan `go test -v -race ./...`

