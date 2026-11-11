---
title: Persistent Config, Open File/Folder, and Git Tab Enhancements
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Persistent Config, Open File/Folder, and Git Tab Enhancements

## Ground Truth Findings
1. **Config Persistence**: Saat ini konfigurasi belum disimpan ke disk; nilai `AutoSave` (default `true`) dan `ShowHidden` (default `false` di `internal/explorer/tree.go:39`) berada in-memory. Belum ada package `internal/config`.
2. **Hidden Files**: `Tree.ShowHidden` diinisialisasi `false` di `internal/explorer/tree.go:39`.
3. **Menu File**: `internal/ui/menu.go:38` baru menyediakan `New File`, `Save File`, `Close Tab`, dan `Exit`. Belum ada `Open File` dan `Open Folder`.
4. **Git Truncation Bug**: Di `internal/git/view.go:420, 461, 502` dan `internal/git/diff.go:276`, fungsi `truncateOrPad` memotong string ber-ANSI menggunakan penghitungan visual width per rune (`runes[:i]`). Karakter sequence ANSI escape (misal `\x1b[...m` dari `formatBadge` dan `hashBadge`) termakan sebagai visual width sehingga string nama file (`item.Name`) dan pesan commit (`commit.Message`) terpotong/hilang dari tampilan.
5. **Git New File Diff**: Di `internal/app/model.go:285, 652`, ketika file berstatus untracked (`?`) dipilih/diklik, aplikasi memanggil `m.openDiff(path)` yang membuka side-by-side diff kosong di HEAD alih-alih membuka file langsung di editor.
6. **Git Commit & Push**: `internal/git/view.go` hanya memiliki textinput commit tanpa tombol visual. Commit hanya bisa dipicu via `Ctrl+Enter`. Belum ada method `Push()` di `internal/git/repo.go`. User mengonfirmasi preferensi push via `go-git` + eksekusi langsung.
7. **Status Bar Branch**: `internal/ui/statusbar.go` dan `internal/git/repo.go` belum mengekstrak atau menampilkan nama branch git aktif (` main`).

## Objective & Approach
1. Buat package `internal/config` untuk memuat dan menyimpan konfigurasi di `~/.config/tui-workflow/config.json`.
2. Ubah default `ShowHidden` menjadi `true` dan simpan setiap perubahan toggle ke file config.
3. Tambahkan menu `Open File` (`Ctrl+O`) dan `Open Folder` (`Ctrl+K`) pada menu `[File]` beserta dialog input path.
4. Perbaiki `truncateOrPad` agar menggunakan `ansi.Truncate` yang ANSI-aware sehingga nama file untracked dan commit message tampil utuh tanpa terpotong kode ANSI.
5. Pada Git view, jika file berstatus untracked (`?` / `U`) diklik/Enter, langsung buka file di editor buffer (`m.openFile`), bukan diff viewer.
6. Tambahkan tombol visual `[ Commit ]` dan `[ Push ]` di bawah input box Git sidebar yang dapat diklik mouse, dan sediakan method `r.r.Push()` di `repo.go`.
7. Tambahkan indikator nama branch aktif pada status bar bawah.

## Plan
1. **`internal/config/config.go`**:
   - Definisikan `struct Config` (`ShowHidden bool`, `AutoSave bool`, `LastDir string`).
   - Fungsi `GetConfigPath()`, `LoadConfig() (*Config, error)`, `SaveConfig(cfg *Config) error`.
   - Default: `ShowHidden: true`, `AutoSave: true`.
2. **`internal/explorer/tree.go`**:
   - Jadikan `ShowHidden: true` default saat inisialisasi `NewTree`.
3. **`internal/ui/menu.go`**:
   - Tambahkan item `Open File` (ActionID: `file.open`, Shortcut: `Ctrl+O`) dan `Open Folder` (ActionID: `file.open_folder`, Shortcut: `Ctrl+K`) pada kategori `File`.
4. **`internal/ui/dialog/input.go`**:
   - Tambahkan jenis dialog `InputOpenFile` dan `InputOpenFolder`.
5. **`internal/git/repo.go` & `internal/git/diff.go` & `internal/git/view.go`**:
   - Tambahkan method `GetBranchName() (string, error)` dan `Push() error` pada `Repo`.
   - Perbaiki fungsi `truncateOrPad` / pemotongan teks ANSI di `diff.go` dan `view.go` menggunakan `ansi.Truncate`.
   - Perbarui tampilan History agar menampilkan commit message di samping/bawah hash secara bersih.
   - Tambahkan rendering tombol `[ Commit ]` dan `[ Push ]` di bawah commit message input box dengan hit-testing koordinat mouse (`ClickCommitButton`, `ClickPushButton`).
6. **`internal/ui/statusbar.go`**:
   - Tambahkan field `Branch string` pada `StatusBar` dan render badge ` <branch>` / `git:<branch>` jika ada.
7. **`internal/app/model.go`**:
   - Inisialisasi dan sinkronkan `Config` saat startup dan perubahan konfigurasi.
   - Handle action menu `file.open` dan `file.open_folder`.
   - Implementasikan penggantian folder aktif di `handleInputConfirmed`: perbarui `RootDir`, re-create `Explorer` & `GitView`, bersihkan tab yang ada.
   - Di Git click / key Enter handler: buka langsung via `openFile` jika status `?` atau `U`.
   - Handle klik tombol `[ Commit ]` dan `[ Push ]`, tampilkan status toast notification.
   - Sinkronkan `GitBranch` ke `StatusBar`.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu (`[File]`) | Penambahan item menambah tinggi dropdown | Lebar dan tinggi dropdown sudah terikat ke ukuran items dinamis; batas `MaxHeight` tetap 1 baris di baris 0 |
| Sidebar Col 1 (Git) | Penambahan tombol `[ Commit ] [ Push ]` menambah baris di sidebar | Hitung `currentY` secara akurat pada `ItemClickMap`, batasi render sidebar agar tidak overflow tinggi terminal |
| Status Bar Bawah | Penambahan badge branch nama dapat melebihi lebar terminal | Gunakan `ansi.StringWidth` dan prioritaskan pemotongan file info secara aman dengan `ansi.Truncate` |

## UI / Keymap Flow
1. **Menu File**:
   - Tekan `Alt+F` -> Pilih `Open File` (`Ctrl+O`) -> Muncul dialog input path -> Ketik path file -> File terbuka di tab editor.
   - Tekan `Alt+F` -> Pilih `Open Folder` (`Ctrl+K`) -> Muncul dialog input path folder -> Folder diganti, tree explorer & git view di-refresh.
2. **Git Sidebar**:
   - File baru bertanda `?` tampil dengan nama lengkap yang tidak terpotong.
   - Klik file baru `?` -> Langsung terbuka di tab editor aktif (tidak ada tab split diff).
   - Commit box memiliki tombol `[ Commit ]` dan `[ Push ]`.
   - Klik `[ Commit ]` -> Melakukan git commit dari staged changes.
   - Klik `[ Push ]` -> Menjalankan `go-git` push ke remote upstream, hasil dilaporkan lewat toast status bar.
   - Riwayat Git menampilkan hash singkat dan pesan commit.
3. **Status Bar**:
   - Menampilkan `git: <branch_name>` di sisi kiri status bar.

## Tasks
- [x] Implementasi package `internal/config` (load/save `~/.config/tui-workflow/config.json`)
- [x] Jadikan `ShowHidden: true` default dan persisten
- [x] Tambahkan menu `Open File` dan `Open Folder` serta dialog pendukung di `internal/ui` dan `internal/app`
- [x] Perbaiki ANSI-aware string truncation di Git changes & history view
- [x] Buka file langsung untuk file untracked/new file di Git view (tanpa diff)
- [x] Tambahkan tombol `[ Commit ]` dan `[ Push ]` di Git view beserta method `Push()` di `repo.go`
- [x] Tambahkan informasi Git branch aktif di status bar bawah
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Jalankan aplikasi, periksa apakah file tersembunyi (dotfiles) langsung tampak di explorer.
- [ ] Tekan `Alt+S` atau ubah pengaturan, periksa file `~/.config/tui-workflow/config.json` terbuat dan terupdate.
- [ ] Buka menu `Alt+F` -> `Open Folder`, masukkan path baru, pastikan explorer dan git berganti root folder.
- [ ] Buka tab Git (`Alt+2`), periksa nama file baru bertanda `?` tidak terpotong.
- [ ] Klik file `?`, pastikan file terbuka langsung di editor tanpa split diff tab.
- [ ] Periksa Git commit history, pastikan commit message tampil mendampingi hash.
- [ ] Periksa tombol `[ Commit ]` dan `[ Push ]` di bawah kotak pesan commit.
- [ ] Periksa status bar bawah, pastikan nama branch aktif tampil rapi.

## Acceptance Criteria
- [x] File konfigurasi tersimpan di `~/.config/tui-workflow/config.json` dan memuat setting saat startup.
- [x] Hidden files aktif secara default.
- [x] Menu `Open File` dan `Open Folder` bekerja menggantikan folder/membuka file.
- [x] Bug nama file baru dan bug commit history hilang (nama dan commit message terlihat jelas).
- [x] File baru (`?`) dari Git view dibuka langsung di editor.
- [x] Tombol commit & push tersedia di Git view.
- [x] Status bar menampilkan nama branch git aktif.
- [x] Seluruh test `go test -v -race ./...` dan `go vet ./...` lulus 100%.

## Decisions
- Konfigurasi disimpan dalam format JSON di path standar OS (`os.UserConfigDir()/tui-workflow/config.json`).
- Push git dieksekusi menggunakan library `go-git/v5` sesuai pilihan user.
- Truncation baris git diganti menggunakan library `ansi.Truncate` demi mematuhi aturan keras Zero Terminal Flickering dan ANSI-Aware String Manipulation.

## Blockers
- None.

## Next
- Eksekusi Improvement 1: File & Folder Picker Dialog serta mouse handling pada seluruh modal dialog.

## Improvement 1 (2026-11-12 — Interactive File & Folder Picker with Mouse Support)

### Ground Truth Findings
1. **Modal Mouse Event Leak**: Di `internal/app/model.go:160-200`, `tea.MouseMsg` tidak dicegat saat modal dialog aktif (`InputDialog`, `ConfirmDialog`, `Switcher`). Event mouse jatuh ke baris tab bar (Y=1) atau buffer editor (Y>=2).
2. **Open File & Open Folder UX**: `InputDialog` hanya berupa text input satu baris, menyulitkan pengguna bernavigasi tanpa mengingat path absolut/relatif secara manual.
3. **No File Picker Component**: Saat ini belum ada komponen modal picker direktori yang interaktif di `internal/ui/dialog`.

### Objective & Approach
1. Buat komponen baru `internal/ui/dialog/filepicker.go` (`FilePicker`):
   - Mendukung mode `PickerModeFile` (Open File) dan `PickerModeFolder` (Open Folder).
   - Tampilkan Current Path, real-time search filter input, daftar navigasi scrollable (`..`, folder, file dengan icon).
   - Dukung mouse click: klik item untuk select/double-click masuk folder, klik tombol action `[ Buka File ]` / `[ Buka Folder Ini ]` dan `[ Batal ]`.
   - Dukung mouse wheel scroll list.
2. Tambahkan mouse hit-testing pada `InputDialog` dan `ConfirmDialog` agar tombol `[ OK ]`, `[ Cancel ]`, `[ Save ]`, `[ Discard ]` dapat diklik langsung oleh mouse.
3. Di `internal/app/model.go`:
   - Alihkan `file.open` dan `file.open_folder` ke `m.FilePicker.Open(...)`.
   - Intersep `tea.MouseMsg` di awal loop saat ada modal aktif (`FilePicker`, `InputDialog`, `ConfirmDialog`, `Switcher`) agar klik tidak tembus ke background.
   - Sambungkan pemilihan file/folder ke `m.openFile` dan workspace switch.

### Tasks
- [x] Buat `internal/ui/dialog/filepicker.go` beserta unit test di `filepicker_test.go`
- [x] Tambahkan mouse handling pada `InputDialog` dan `ConfirmDialog` di `internal/ui/dialog/`
- [x] Integrasikan `FilePicker` ke dalam `internal/app/model.go` dan `internal/app/view.go`
- [x] Cegat `tea.MouseMsg` saat modal aktif di `internal/app/model.go`
- [x] Unit testing e2e navigasi mouse & keyboard picker di `internal/app/features_test.go`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓
