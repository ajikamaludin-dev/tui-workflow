---
title: File Watcher, Adjustable Sidebar Width, and Markdown Preview Copy/Paste
status: in_progress
started: 2026-11-12
completed: 
branch: dev
priority: high
---

# File Watcher, Adjustable Sidebar Width, and Markdown Preview Copy/Paste

## Ground Truth Findings
- **File Watching (`internal/editor/tab.go`, `internal/app/model.go`)**:
  - `Tab` merekam `Path`, `Buffer`, `MarkdownViewer`. Belum ada `LastModTime time.Time` untuk tracking perubahan file di filesystem.
  - Bubble Tea loop di `internal/app/model.go` memiliki `marqueeTickCmd` (250ms) dan `gitLoadingTickCmd` (80ms). Polling stat file setiap ~1 detik (`1000ms`) via `tea.Tick` sangat hemat CPU/RAM (< 10 µs per tick untuk open tabs) dan 100% kompatibel lintas OS tanpa ketergantungan library CGO eksternal.
  - Auto-reload aman dieksekusi jika buffer bersih (`!Buffer.IsDirty`). Jika kotor (`Buffer.IsDirty`), munculkan toast peringatan konflik tanpa menimpa buffer user.
- **Sidebar Width (`internal/app/model.go`, `internal/app/view.go`, `internal/config/config.go`)**:
  - `Model.SidebarWidth` sudah ada dengan nilai awal `30`, namun bersifat hardcoded saat runtime.
  - `Config` di `internal/config/config.go` saat ini memuat `ShowHidden`, `AutoSave`, `SoftWrap`, `LastDir`. Belum ada field `SidebarWidth int`.
  - Border kanan sidebar terletak tepat di koordinat `X = m.SidebarWidth - 1`. Belum ada penanganan mouse drag di border atau keybinding shortcut penyesuaian lebar (`Alt+[` dan `Alt+]`).
- **Markdown Preview Copy/Paste (`internal/markdown/renderer.go`, `internal/app/model.go`)**:
  - `MarkdownViewer` merender via Glamour ke `cachedLines []string` dengan word wrapping.
  - Belum ada state seleksi (`SelectionStart`, `SelectionEnd`, `HasSelection`) dan visual highlighting baris terpilih pada rendered lines.
  - Pada mode `MarkdownPreview`, keypress `Ctrl+C`, `Ctrl+V`, `Ctrl+A` dan mouse drag selection belum ditangani. Context menu editor belum terintegrasi untuk aksi preview markdown.

## Dual-Lens Evaluation
1. **Business & UX Lens**:
   - *File Watcher*: User yang menggunakan `git checkout`, `git pull`, atau generator file di terminal luar tidak perlu lagi menutup dan membuka ulang tab secara manual.
   - *Adjustable Sidebar*: Nama file atau struktur folder yang panjang sering terpotong jika lebar sidebar statis 30 kolom; user dapat memperluas atau mempersempit via mouse drag atau shortcut cepat `Alt+[` / `Alt+]`.
   - *Markdown Preview Copy/Paste*: User yang sedang membaca preview dokumen markdown dapat langsung menyeleksi dan menyalin (`Ctrl+C`) teks/kode bersih tanpa beralih ke editor mode mentah, serta menempel (`Ctrl+V`) konten ke dokumen.
2. **Senior Technical Lens**:
   - *Zero-Lag & Zero-Flicker*: File stat checking hanya memeriksa tab yang terbuka (`O(N)` dengan N = jumlah tab aktif, tipikal < 20).
   - *Single-Line Guarantee*: Penyesuaian lebar sidebar wajib mempertahankan batas `minSidebarWidth` (15) dan `maxSidebarWidth` (`m.Width - 25`) agar editor pane tidak remuk (`width < 10`).
   - *ANSI Safety*: Copy text dari Glamour viewer wajib melalui `ansi.Strip` agar escape code ANSI styling tidak bocor ke clipboard sistem.

## Objective & Approach
1. **Task 20 (File Watcher)**:
   - Tambahkan `LastModTime time.Time` pada `editor.Tab`. Inisialisasi saat `openFile` dan perbarui saat save buffer.
   - Buat `fileWatchTickCmd()` (interval 1s). Di handler `FileWatchTickMsg`: periksa `os.Stat` seluruh tab file terbuka.
   - Jika modtime lebih baru dan `!Buffer.IsDirty`: baca ulang konten dari disk, perbarui buffer, dan tampilkan toast `"File '<nama>' dimuat ulang dari disk"`.
   - Jika modtime lebih baru dan `Buffer.IsDirty`: tampilkan toast peringatan konflik `"Peringatan: File '<nama>' diubah dari luar"`.
   - Jika file terhapus (`os.IsNotExist`): tampilkan toast `"File '<nama>' telah dihapus dari disk"`.
2. **Task 24 (Adjustable Sidebar Width)**:
   - Tambahkan `SidebarWidth int` pada `config.Config` (default 30) dengan batas aman [15, width-20].
   - Tambahkan keybinding: `Alt+[` (persempit 2 kol), `Alt+]` (perlebar 2 kol).
   - Tangani mouse drag pada border sidebar (`msg.X == m.SidebarWidth - 1 || msg.X == m.SidebarWidth`): drag mouse untuk resize interaktif secara halus.
   - Simpan nilai `SidebarWidth` ke file konfigurasi saat diubah.
3. **Task 21 (Markdown Preview Copy/Paste)**:
   - Tambahkan state seleksi (`SelectionStartRow`, `SelectionEndRow`, `HasSelection`) dan helper (`SelectAll`, `ClearSelection`, `GetSelectedText`) pada `markdown.MarkdownViewer`.
   - Berikan styling highlight visual pada baris preview yang terseleksi di `MarkdownViewer.Render`.
   - Di `internal/app/model.go`:
     - Dukung mouse drag pada area preview untuk memilih baris teks.
     - Dukung `Ctrl+C`: salin teks terseleksi (bersih tanpa ANSI) ke clipboard sistem via `clipboard.WriteAll`.
     - Dukung `Ctrl+A`: seleksi seluruh baris preview.
     - Dukung `Ctrl+V`: paste teks dari clipboard ke dalam buffer dokumen markdown terkait dengan toast feedback.
     - Integrasikan ke klik kanan Context Menu (Salin, Tempel, Pilih Semua).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Zona 2 (Sidebar) | Kolom terlalu sempit / lebar merusak layout | Batasi clamp `SidebarWidth` antara 15 dan `m.Width - 20` |
| Zona 3 (Editor Pane) | Perubahan `SidebarWidth` mempengaruhi viewport editor | Re-evaluasi `editorWidth = m.Width - m.SidebarWidth` secara dinamis di setiap render |
| Baris 4 (Status Bar) | Toast notification bertabrakan | Gunakan timeout toast standar (2-3 detik) non-blocking |

## UI / Keymap Flow
- **Shortcut Baru**:
  - `Alt+[` : Kurangi lebar sidebar sebesar 2 kolom.
  - `Alt+]` : Tambah lebar sidebar sebesar 2 kolom.
  - `Ctrl+C` (di Markdown Preview) : Salin teks/blok terseleksi ke clipboard.
  - `Ctrl+A` (di Markdown Preview) : Seleksi semua teks di preview.
  - `Ctrl+V` (di Markdown Preview) : Tempel isi clipboard ke dalam buffer dokumen markdown.
- **Mouse Action**:
  - Drag mouse pada garis batas kolom sidebar (`X = SidebarWidth - 1`): Mengubah lebar sidebar secara dinamis.
  - Drag mouse di dalam Markdown Preview: Menyeleksi baris teks rendered.
  - Klik kanan di Markdown Preview: Membuka context menu (Salin / Tempel / Pilih Semua).

## Tasks
- [ ] 1. Perbarui `internal/config/config.go` untuk mendukung field `SidebarWidth` (default 30).
- [ ] 2. Implementasi File Watcher di `internal/editor/tab.go` (`LastModTime`) dan `internal/app/model.go` (`fileWatchTickCmd` + handler reload / alert).
- [ ] 3. Implementasi Adjustable Sidebar Width di `internal/app/model.go` (shortcut `Alt+[` / `Alt+]`, mouse drag di border) dan perbarui `view.go`.
- [ ] 4. Implementasi Seleksi & Copy/Paste di `internal/markdown/renderer.go` (`MarkdownViewer` selection state, ANSI-stripped copy, rendering highlight).
- [ ] 5. Integrasi kontrol `Ctrl+C`, `Ctrl+V`, `Ctrl+A`, context menu, dan mouse selection preview di `internal/app/model.go`.
- [ ] 6. Perbarui `TODO.md` (centang Task 20, 24, 21).
- [ ] 7. Tambahkan unit test & regression test di `internal/editor`, `internal/markdown`, `internal/config`, dan `internal/app`.
- [ ] 8. Verifikasi: `go vet ./...` ✓, `go test -v -race ./...` ✓.

## Manual Test Checklist
- [ ] Buka file teks di editor. Di terminal lain, ubah isi file (`echo "tambahan" >> file.txt`). Verifikasi editor otomatis memuat ulang isi file dan memunculkan toast.
- [ ] Ubah isi buffer tanpa disimpan (dirty). Ubah file dari luar. Verifikasi muncul toast peringatan konflik tanpa menimpa perubahan lokal.
- [ ] Tekan `Alt+]` beberapa kali, verifikasi sidebar melebar. Tekan `Alt+[` beberapa kali, verifikasi sidebar menyempit.
- [ ] Drag border kanan sidebar menggunakan mouse kiri, verifikasi lebar sidebar mengikuti kursor mouse dan ter-clamp dengan aman.
- [ ] Buka file markdown, tekan `Alt+M` (masuk preview). Drag mouse pada teks preview atau tekan `Ctrl+A`. Tekan `Ctrl+C`, verifikasi teks tersalin ke clipboard tanpa karakter ANSI rusak.
- [ ] Tekan `Ctrl+V` saat di preview markdown, verifikasi konten tertempel ke dokumen.
- [ ] Klik kanan pada preview markdown, verifikasi menu konteks Salin/Tempel/Pilih Semua berfungsi.

## Acceptance Criteria
- [ ] File watcher mendeteksi modifikasi file eksternal dan auto-reload jika buffer clean.
- [ ] Buffer dirty terlindungi dari overwrite saat terjadi perubahan eksternal.
- [ ] Lebar sidebar dapat disesuaikan via keyboard (`Alt+[`/`Alt+]`) dan mouse drag pada border.
- [ ] Lebar sidebar persisten di file config.
- [ ] Preview markdown mendukung seleksi teks visual, copy (`Ctrl+C`), select all (`Ctrl+A`), paste (`Ctrl+V`), dan context menu.
- [ ] `go vet ./...` menghasilkan 0 error / warning.
- [ ] `go test -v -race ./...` 100% lulus tanpa race condition atau regression.

## Decisions
- Stat polling 1 detik dipilih dibanding library inotify/fsnotify pihak ketiga untuk menjamin portabilitas 100% tanpa CGO, toleran terhadap atomic-rename editors (vim/nano), dan RAM footprint < 30 MB.
- Copy dari Glamour preview menggunakan pembersihan ANSI escape code (`ansi.Strip`) agar teks hasil paste di aplikasi lain rapi dan bersih.

## Blockers
- None.

## Next
- Tunggu approval user untuk mulai eksekusi pengerjaan.
