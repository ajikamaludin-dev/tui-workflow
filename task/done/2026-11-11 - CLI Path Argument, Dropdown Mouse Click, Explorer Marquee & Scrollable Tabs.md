---
title: CLI Path Argument, Dropdown Mouse Click, Explorer Marquee & Scrollable Tabs
status: done
started: 2026-11-11
completed: 2026-11-11
branch: main
priority: high
---

# CLI Path Argument, Dropdown Mouse Click, Explorer Marquee & Scrollable Tabs

## Ground Truth Findings
1. **CLI Argument (`cmd/tui-workflow/main.go` & `internal/app/model.go`)**:
   - `main.go` memanggil `app.NewModel()` tanpa argumen CLI.
   - `internal/app/model.go:37-75` mem-hardcode root path `.` dan membuka `PRD.md` jika ada. Belum mendukung argument path seperti `code /path/to/project`.
2. **Dropdown Menu Mouse Click (`internal/ui/menu.go`)**:
   - `internal/ui/theme.go:67-70` mendefinisikan `DropdownStyle` dengan border rounded.
   - Border atas berada di row `Y = 1`. Item pertama dirender pada `Y = 2`.
   - `internal/ui/menu.go:193-205` menghitung `dropStartY := 1` dan `clickedItem := msg.Y - dropStartY`. Klik di `Y = 2` salah menghitung index menjadi `1` ("Save File"), sedangkan klik di `Y = 5` (item terakhir) out-of-bounds dan dropdown hanya tertutup tanpa aksi. Lebar dropdown juga dihitung manual bukan dari lebar box ter-render.
3. **Explorer Long Filename Overflow (`internal/explorer/tree.go`)**:
   - `internal/explorer/tree.go:240-275` tidak melakukan truncate pada `lineText` jika lebih panjang dari `width`. String meluap dan ter-wrap ke 3 baris di terminal (`/tmp/pi-clipboard-1ec94ca7-6428-45a1-bd6c-fdfd311e8137.png`), melanggar aturan single-line tree.
   - Belum ada mekanisme marquee animasi untuk nama file yang terpilih (*selected*).
4. **Editor Tab Bar Truncation & Scrolling (`internal/editor/manager.go`)**:
   - `internal/editor/manager.go:219-225` mengecilkan `maxTabTitleLen` secara agresif hingga 6 karakter (`READ..`, `AGEN..`) jika tab bertambah banyak.
   - Tab bar menghitung ulang `startIdx` setiap frame tanpa menyimpan `TabScrollOffset`, tidak mendukung scroll wheel mouse pada tab bar, dan tombol `<` / `>` tidak memiliki click hit-testing.

## Objective & Approach
1. **CLI Path Argument**: Dukung argumen path opsional pada binary CLI (contoh `tui-workflow /path/to/folder` atau `tui-workflow file.go`). Validasi path keberadaan file/folder. Jika folder, buka folder di explorer & git. Jika file, buka folder parent di explorer & buka file di editor buffer.
2. **Dropdown Mouse Click**: Perbaiki koordinat hit-testing pada `internal/ui/menu.go`. Sesuaikan `dropStartY = 2` (karena border di Y=1) dan hitung lebar box dropdown dari `lipgloss.Width(box)`. Klik item dropdown mengeksekusi action dengan tepat.
3. **Explorer Long Name Truncation & Marquee**:
   - Terapkan `ansi.Truncate(..., width, "")` dengan single-line guarantee ketat.
   - Jika node tidak terpilih: potong dengan elipsis `...` jika melebihi lebar kolom.
   - Jika node terpilih (*active/selected*) & teks melebihi lebar: aktifkan marquee scroll horizontal via `tea.Tick` (offset bergeser berkala dan loop mulus) tanpa merusak batas 1 baris.
4. **Scrollable Tabs & Fixed Character Limit**:
   - Kunci batas karakter nama tab ke angka tetap yang wajar (16 karakter), jangan dipaksa ciut ke 6 karakter.
   - Simpan `TabScrollOffset` di `TabManager`. Tambahkan scrolling via mouse wheel di tab bar (Row 1) dan klik pada tombol `<` / `>`.
   - Auto-scroll viewport tab saat active tab berpindah agar selalu terlihat.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu (Row 0) | Salah hitung koordinat dropdown merusak row lain | Dropdown tetap dirender via overlay at `Y=1`, hit-test mouse Y=2..N diperbaiki |
| Sidebar (Files Tree) | Nama file panjang meluap menyebabkan line wrapping | Strict `ansi.Truncate(line, width, "")` wajib 1 baris per node; marquee hanya menggeser karakter horizontal |
| Tab Bar (Row 1) | Tab meluap atau push baris editor ke bawah | Single-line guarantee dijaga via `MaxHeight(1)` dan pemotongan tabs yang berada di luar view window |

## UI / Keymap Flow
- CLI: `$ tui-workflow [path]`
- Mouse Dropdown: Klik `[File]` -> Klik `New File` -> Modal New File terbuka; Klik `Exit` -> konfirmasi/keluar.
- Explorer: Navigasi kursor ke file bernama panjang -> teks bergerak menyamping (marquee) otomatis; saat pindah kursor kembali terpotong `...`.
- Tab Bar: Buka banyak tab -> nama tab tetap terbaca (`2026-11-11-In..`) -> scroll wheel pada tab bar atau klik `<` / `>` menggeser tab horizontal.

## Files Affected & Execution Steps
1. `cmd/tui-workflow/main.go`:
   - Baca `os.Args[1]`. Validasi filesystem.
   - Panggil `app.NewModelWithPath(targetPath)`.
2. `internal/app/model.go`:
   - Implementasikan `NewModelWithPath(path string)`.
   - Tambahkan handling `MarqueeTickMsg` untuk animasi file tree.
   - Tangani mouse wheel pada `Row 1 && msg.X >= editorStartX` untuk scroll tab bar.
   - Tangani klik `<` dan `>` pada tab bar untuk scroll tab.
3. `internal/ui/menu.go` & `internal/ui/menu_test.go`:
   - Koreksi hit-test `HandleMouse`: item Y dimulai dari 2, lebar dihitung dari dimensi box ter-render.
   - Tambahkan unit test klik mouse pada dropdown menu.
4. `internal/explorer/tree.go` & `internal/explorer/tree_test.go`:
   - Tambahkan `MarqueeOffset int` dan metode `TickMarquee()`.
   - Format nama node: truncate `...` untuk inactive node; marquee slice loop untuk active node.
   - Potong seluruh line node dengan `ansi.Truncate(lineText, width, "")`.
   - Tambahkan unit test truncation dan marquee.
5. `internal/editor/manager.go` & `internal/editor/manager_test.go`:
   - Tetapkan `maxTabTitleLen = 16` tanpa reduksi dinamis ke 6 karakter.
   - Tambahkan `TabScrollOffset`, `ScrollLeft()`, `ScrollRight()`.
   - Tambahkan hit region untuk `<` dan `>` di `RenderTabBar` dan `HandleTabClick`.
   - Tambahkan unit test scrollable tab bar dan click navigation.

## Tasks
- [x] Task 1: CLI path argument support (`cmd/tui-workflow/main.go` & `internal/app/model.go`)
- [x] Task 2: Dropdown menu mouse click fix (`internal/ui/menu.go` & test)
- [x] Task 3: Explorer long name truncation & marquee animation (`internal/explorer/tree.go` & test)
- [x] Task 4: Scrollable tab bar with fixed character limit (`internal/editor/manager.go` & test)
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [x] Jalankan `./tui-workflow internal/editor` -> explorer terbuka di folder `internal/editor`.
- [x] Jalankan `./tui-workflow PRD.md` -> editor langsung membuka `PRD.md`.
- [x] Klik menu `[File]` lalu klik `New File` -> dialog input muncul.
- [x] Di explorer, arahkan kursor ke file ber-nama panjang -> nama file marquee geser samping, tidak membungkus ke baris baru.
- [x] Buka 10+ file -> judul tab tidak ciut ke 4-6 karakter, klik `<` / `>` atau scroll mouse pada tab bar menggeser daftar tab.

## Acceptance Criteria
- [x] Binary menerima parameter folder/file opsional.
- [x] Item dropdown dapat diklik mouse dengan akurat.
- [x] Explorer tree strictly 1 baris per node; long file name memiliki elipsis dan marquee saat aktif.
- [x] Tab bar scrollable (mouse scroll & click `<`/`>`) dengan judul tab tetap proporsional.
- [x] `go vet ./...` 0 error, `go test -v -race ./...` 100% PASS.

## Decisions
- Marquee refresh interval: ~250ms via `tea.Tick` agar animasi halus tanpa membebani CPU.
- Tab title limit: 16 karakter agar informatif dan muat banyak tab.
- Dropdown mouse hit-test: Menggunakan exact boundary box `RenderDropdown()` (Y=2..N, X=startX..startX+width).

## Blockers
- None.

## Next
- Tunggu konfirmasi review dari user. Setelah oke, arsipkan ke `task/done/`.
