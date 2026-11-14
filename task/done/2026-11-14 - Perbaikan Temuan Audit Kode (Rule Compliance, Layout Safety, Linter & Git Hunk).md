---
title: Perbaikan Temuan Audit Kode (Rule Compliance, Layout Safety, Linter & Git Hunk)
status: done
started: 2026-11-14
completed: 2026-11-14
branch: dev
priority: high
---

# Perbaikan Temuan Audit Kode (Rule Compliance, Layout Safety, Linter & Git Hunk)

## Objective
Menyelesaikan dan memperbaiki 5 temuan audit kode hasil inspeksi menyeluruh pada codebase `tui-workflow`: sinkronisasi help modal (Rule 11), pencegahan terminal scroll overflow pada Top Menu Bar (Rule 4.1), pemotongan string ANSI yang aman pada Markdown viewer (Rule 4.2), dukungan parsing JSON Biome pada linter bridge, serta pembetulan logika safety guard hunk-level Git untuk file ambigu/terhapus.

---

## Ground Truth Findings

Berdasarkan verifikasi fakta Phase 0 pada seluruh paket `internal/` dan `cmd/`:

1. **Pintasan Keyboard Hilang di Help Modal (`internal/ui/dialog/help.go:19-110` vs `internal/app/view.go:312-380`)**:
   - Fitur word jump (`Alt+Left`/`Ctrl+Left`, `Alt+Right`/`Ctrl+Right`), word block selection (`Shift+Alt+Left/Right`, `Shift+Ctrl+Left/Right`), dan word delete (`Alt+Backspace`, `Ctrl+Backspace`) telah diimplementasikan di `internal/editor/buffer.go`, `internal/terminal/terminal.go`, `internal/ui/menu.go`, serta Command Palette `internal/app/model.go`.
   - Namun, `HelpDialog.Lines` pada `internal/ui/dialog/help.go` tidak memuat pintasan ini. Saat `?` ditekan, aplikasi memanggil `m.HelpDialog.Render(m.Height)`, sehingga pop-up cheatsheet menampilkan daftar pintasan lama yang usang. Fallback `renderHelpModal()` di `view.go` memiliki sebagian teks tetapi tidak pernah dieksekusi saat `m.HelpDialog != nil`. Hal ini melanggar **Rule 11 (Shortcut Synchronization & Help Modal Update Mandate)**.

2. **Potensi Overflow Baris Top Menu (`internal/ui/menu.go:280-310`, `internal/ui/theme.go:66-70`)**:
   - `TopMenuStyle` pada `internal/ui/theme.go` tidak memiliki pembatas `.MaxHeight(1)`.
   - Pada `MenuBar.Render()`, pemanggilan `CurrentTheme.TopMenuStyle.Width(m.Width).Render(renderedBar)` tidak membungkus teks akhir dengan `ansi.Truncate(renderedBar, m.Width, "")`. Jika terminal memiliki lebar sempit (< 60 kolom) atau kategori menu meluap, Lipgloss akan melakukan word-wrap menjadi 2 baris atau lebih. Ini menyebabkan Baris 0 terdorong keluar layar (*offscreen scroll*), melanggar **Rule 4.1 (Zero Terminal Scroll Overflow Mandate)**.

3. **ANSI String Slicing Mentah pada Markdown Truncate (`internal/markdown/renderer.go:235-251`)**:
   - Fungsi `truncateAnsi(s string, width int)` di `internal/markdown/renderer.go` melakukan pemotongan karakter dengan mengiris rune slice mentah: `string(runes[:i])`.
   - Output Glamour memuat escape sequence ANSI 24-bit TrueColor / 256-color. Pemotongan berbasis rune memotong urutan ANSI di tengah byte (misal `\x1b[38;2;...`), menyebabkan karakter angka bocor ke layar terminal dan merusak tampilan. Hal ini melanggar **Rule 4.2 (ANSI-Aware String Manipulation Mandate)**.

4. **Inkompatibilitas Parser Linter Biome (`internal/linter/bridge.go:101-118`)**:
   - `RunLinterCmd` mendukung eksekusi `biome lint --reporter=json targetPath`, tetapi output stdout-nya langsung diparsing menggunakan `ParseESLintJSON(outBuf.Bytes(), targetPath)`.
   - Struktur JSON Biome (`{"diagnostics": [{"category": "...", "message": [...], "location": {...}}]}`) tidak cocok dengan schema ESLint (`[{"filePath": "...", "messages": [...]}]`). Akibatnya, `json.Unmarshal` selalu gagal dan mengembalikan 0 diagnostik tanpa ada pesan error yang jelas.

5. **Inversi Logika Safety Guard Git Hunk Action (`internal/app/model.go:3874-3885`)**:
   - Pada `isUnsafeForHunkAction(relPath string)`:
     ```go
     info, err := os.Stat(abs)
     if err != nil {
         // untracked/delete/rename-conflict-like ambiguity: keep safe fallback
         return false
     }
     ```
   - Ketika file telah dihapus di working tree atau path ambigu, `os.Stat(abs)` menghasilkan error. Fungsi saat ini mengembalikan `false` (dianggap *safe* untuk hunk action). Ini menyebabkan `applyDiffHunkAction` tetap mencoba membaca file yang tidak ada dan gagal saat runtime. Sesuai PRD Bagian 4.7.1, kasus ambigu/deleted wajib mengembalikan `true` agar dialihkan ke aksi file-level yang aman.

---

## Plan

### Pendekatan Teknis:
1. **Perbaikan `internal/ui/dialog/help.go`**:
   - Tambahkan entri shortcut baru pada slice `Lines` di `NewHelpDialog()`:
     - Word jump: `Alt/Ctrl+Panah Kiri/Kanan`
     - Word block selection: `Shift+Alt+Panah Kiri/Kanan`
     - Delete word left: `Alt/Ctrl+Backspace`
     - Navigasi shell terminal: Word jump & word delete pada PTY.
   - Sinkronkan kategori agar konsisten dengan `renderHelpModal()` di `internal/app/view.go` dan cheatsheet PRD/README.

2. **Perbaikan `internal/ui/menu.go` & `internal/ui/theme.go`**:
   - Tambahkan `.MaxHeight(1)` pada `t.TopMenuStyle` di `internal/ui/theme.go`.
   - Di `MenuBar.Render()` (`internal/ui/menu.go`), pastikan string akhir dipotong aman menggunakan `ansi.Truncate(renderedBar, m.Width, "")` sebelum dirender oleh Lipgloss style, serta pastikan `TopMenuStyle.Width(m.Width).MaxHeight(1).Render(...)` selalu tepat 1 baris.

3. **Perbaikan `internal/markdown/renderer.go`**:
   - Ganti implementasi `truncateAnsi(s, width)` dengan `ansi.Truncate(s, width, "")` dari pustaka `github.com/charmbracelet/x/ansi`.
   - Hapus kode slicing rune manual yang rentan memotong sequence escape ANSI.

4. **Perbaikan `internal/linter/bridge.go` & `internal/linter/diagnostic.go`**:
   - Definisikan tipe data JSON untuk schema laporan Biome (`BiomeReport`, `BiomeDiagnostic`, `BiomeLocation`, dll.).
   - Buat fungsi `ParseBiomeJSON(rawOutput []byte, targetPath string) []Diagnostic`.
   - Pada `RunLinterCmd`, jika `linterType == "biome"`, gunakan `ParseBiomeJSON`, jika `eslint` gunakan `ParseESLintJSON`.
   - Tambahkan unit test untuk `ParseBiomeJSON` di `internal/linter/linter_test.go`.

5. **Perbaikan `internal/app/model.go`**:
   - Pada fungsi `isUnsafeForHunkAction`, ubah kondisi `if err != nil` menjadi `return true` sehingga file yang terhapus/tidak dapat di-stat dikategorikan sebagai *unsafe* untuk operasi hunk parsial dan diarahkan ke aksi file-level.

---

## Terminal & Layout Impact Analysis

| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| **Row 0 (Top Menu Bar)** | Menu meluap pada terminal sempit menyebabkan baris 0 terdorong ke atas (*offscreen scroll*). | Terapkan `.MaxHeight(1)` pada `TopMenuStyle` dan `ansi.Truncate(renderedBar, m.Width, "")` pada `MenuBar.Render()`. |
| **Column 2 (Markdown Viewer)** | Teks terpotong secara mentah merusak escape sequence warna dan membocorkan kode ANSI. | Gunakan `ansi.Truncate` bawaan `charmbracelet/x/ansi` yang memahami escape codes visual. |
| **Modal Overlays (Help Dialog)** | Teks daftar shortcut bertambah panjang dan meluap dari batas tinggi layar. | `HelpDialog` sudah memiliki viewport scrolling internal (`MaxRows` adaptif + scroll up/down j/k). Penambahan baris tidak mengganggu layout modal. |
| **Status Bar & Diff Pane** | Aksi hunk pada file terhapus menimbulkan error toast atau inkonsistensi git index. | Pengalihan aman ke file-level action (`return true` pada `isUnsafeForHunkAction`). |

---

## UI / Keymap Flow
1. Menekan `?` membuka modal cheatsheet bantuan yang kini menampilkan seluruh pintasan kata (`Alt+Left/Right`, `Shift+Alt+Left/Right`, `Alt+Backspace`) dan navigasi kata pada terminal shell PTY.
2. Membuka preview markdown (`Ctrl+M`) pada file yang memuat styling kompleks atau baris panjang tidak lagi memunculkan artifact escape code ANSI saat terminal dipersempit.
3. Menjalankan linter pada proyek berbasis Biome menghasilkan indikator dot diagnostik (`●`, `▲`) pada gutter nomor baris editor dan pesan linter di status bar saat kursor berada di baris bermasalah.

---

## Tasks
- [x] 1. Perbarui daftar shortcut pada `internal/ui/dialog/help.go` (Word jump, Word selection, Word delete, Terminal CLI word navigation) dan sinkronkan dengan `internal/app/view.go`.
- [x] 2. Terapkan Single-Line Guarantee dan `ansi.Truncate` pada `internal/ui/theme.go` dan `internal/ui/menu.go`.
- [x] 3. Ganti pemotongan ANSI pada `internal/markdown/renderer.go` dengan `ansi.Truncate`.
- [x] 4. Implementasikan `ParseBiomeJSON` di `internal/linter/bridge.go` dan tambahkan pengujian di `internal/linter/linter_test.go`.
- [x] 5. Perbaiki logika safety guard `isUnsafeForHunkAction` pada `internal/app/model.go` untuk file ambigu/deleted.
- [x] 6. Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓.

---

## Manual Test Checklist
- [x] Tekan `?` di terminal: Verifikasi shortcut `Shift+Alt+Panah`, `Alt+Left/Right`, `Alt+Backspace` tampil pada kategori "Editor & Buffer" dan "Terminal & Shell (PTY)".
- [x] Ubah ukuran jendela terminal menjadi kecil (lebar ~60–80): Verifikasi Top Menu Bar tetap tepat 1 baris tanpa mendorong tampilan ke bawah.
- [x] Buka file markdown berformat kompleks dan tekan `Ctrl+M`: Verifikasi tidak ada karakter ANSI bocor (`38;2;...`) saat di-scroll atau dipersempit.
- [x] Buka file JS/TS yang memiliki kesalahan linter di workspace Biome: Verifikasi gutter menampilkan indikator error/warning.
- [x] Buka Git Diff pada file yang terhapus: Verifikasi aksi hunk dicegah dengan toast peringatan aman untuk menggunakan aksi file-level.

---

## Acceptance Criteria
- [x] Seluruh pintasan keyboard terdaftar lengkap pada pop-up dialog bantuan shortcut (`?`).
- [x] Top Menu Bar dijamin tepat 1 baris (`MaxHeight(1)`) dan dipotong via `ansi.Truncate`.
- [x] `truncateAnsi` pada markdown viewer 100% bebas dari rune slicing mentah pada string ber-ANSI.
- [x] Linter bridge berhasil mem-parse output Biome JSON dan memunculkan diagnostik di editor.
- [x] `isUnsafeForHunkAction` mengembalikan `true` untuk file ambigu atau tidak ditemukan.
- [x] `go vet ./...` menghasilkan 0 error dan 0 warning.
- [x] `go test -v -race ./...` lulus 100% dan bebas race condition.

---

## Decisions
- Menggunakan `ansi.Truncate` standar `charmbracelet/x/ansi` alih-alih slicing rune manual untuk menjamin integritas urutan byte ANSI.
- Menambahkan parser JSON Biome langsung ke dalam paket `internal/linter` tanpa menambah dependensi pihak ketiga baru.
- Mempertahankan `isUnsafeForHunkAction` sebagai gerbang keamanan fallback file-level agar worktree dan git index tidak terkorupsi pada skenario file terhapus atau biner.

---

## Blockers
- Tidak ada.

---

## Next
- Task selesai dan terverifikasi. Siap untuk task berikutnya.
