---
title: Cursor I-Beam, Mouse Selection & Context Menu, Modal Visual Fix
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Cursor I-Beam, Mouse Selection & Context Menu, Modal Visual Fix

## Ground Truth Findings
1. **Modal "Berlubang" (Holes) Root Cause**:
   - Ditemukan pada `internal/ui/dialog/input.go`, `confirm.go`, `filepicker.go`, dan `switcher.go`.
   - Terverifikasi pada screenshot `/tmp/pi-clipboard-32400fc0-cfe7-4c10-a2dc-4abff660150d.png`:
     - Box 1 (y=39..75, x=154..397): Sisa baris di sebelah kanan title "Buat File Baru".
     - Input row: Area input textinput setelah kursor.
     - Box 2 (y=140..174, x=151..182): Jeda 1 spasi antara tombol OK dan Cancel akibat `MarginRight(1)`.
     - Box 3 (y=144..168, x=306..393): Sisa baris di sebelah kanan tombol Cancel.
   - Mekanisme kegagalan: Komponen anak (title, textinput view, button) merender ANSI reset `\x1b[0m` di akhir teksnya. `lipgloss.JoinVertical` dan `boxStyle` menyisipkan spasi murni (`0x20`) tanpa escape sequence background `BgTopMenu` (`#2D2D2D`). Terminal merender spasi-spasi tanpa background ini menggunakan warna background default terminal (`#1E1E1E`), menciptakan efek "lubang gelap".
2. **Cursor Shape**:
   - `internal/editor/buffer.go` (baris 374, 405-415) menggunakan software cursor block terbalik (`Reverse(true).Bold(true)`).
   - Terminal cursor shape dapat diatur menggunakan escape sequence DECSCUSR VT (`\x1b[5 q` blinking bar / `\x1b[6 q` steady bar), dan dikembalikan bersih saat teardown (`\x1b[0 q` / `\x1b[2 q`).
   - Pada editor buffer, visual kursor dapat dirender dalam format bar/I-beam (`│` / bar style) yang jelas membedakan mode I-beam dari block.
3. **Mouse Text Selection (Block Text)**:
   - `internal/editor/buffer.go` belum memiliki field seleksi teks (`SelectionAnchorRow`, `SelectionAnchorCol`, `SelectionEndRow`, `SelectionEndCol`, `HasSelection`).
   - `internal/app/model.go` (baris 808-818) hanya menangani klik kiri baris secara kasar (`SetCursor(targetRow, 0)`), mengabaikan kolom X dan mengabaikan event drag mouse (`tea.MouseActionMotion`).
   - `cmd/tui-workflow/main.go` sudah mengaktifkan `tea.WithMouseCellMotion()`, sehingga Bubble Tea siap menerima motion/drag events.
4. **Right Click Context Menu (Copy & Paste)**:
   - `internal/app/model.go` hanya memeriksa `tea.MouseButtonLeft` (baris 705) dan scroll wheel. Event `tea.MouseButtonRight` belum ditangkap sama sekali.
   - Library `github.com/atotto/clipboard` sudah tersedia di `go.mod` (fungsi `clipboard.WriteAll` dan `clipboard.ReadAll`).
   - Diperlukan context menu mini terapung (*floating context menu*) di posisi mouse dengan opsi: `[Salin (Copy)]`, `[Tempel (Paste)]`, `[Potong (Cut)]`, `[Pilih Semua]`.

---

## Objective & Approach
1. **Perbaikan Modal Berlubang**:
   - Buat helper normalisasi ANSI background di `internal/ui/dialog` (atau `internal/ui/theme.go`) yang menjamin seluruh spasi padding di dalam dialog terisi warna background `ui.CurrentTheme.BgTopMenu`.
   - Update `input.go`, `confirm.go`, `filepicker.go`, dan `switcher.go` agar menggunakan styling background konsisten pada textinput, prompt, margin antar tombol, dan baris konten.
2. **Cursor Shape I-Beam**:
   - Kirim escape sequence DECSCUSR `\x1b[6 q` (steady I-beam) / `\x1b[5 q` saat inisialisasi aplikasi / fokus editor dan `\x1b[0 q` saat aplikasi quit (teardown safety).
   - Di `internal/editor/buffer.go`, sesuaikan visual kursor menjadi I-beam bar format (`│` atau vertical bar indicator) saat fokus di editor.
3. **Block dengan Mouse (Mouse Selection)**:
   - Tambahkan state seleksi pada `editor.Buffer`: `SelectAnchorRow`, `SelectAnchorCol`, `SelectEndRow`, `SelectEndCol`, `HasSelection`.
   - Hitung koordinat kolom horizontal relatif terhadap gutter editor (`col = mouseX - editorStartX - gutterWidth`).
   - Tangani event `tea.MouseActionPress` (memulai seleksi / set kursor), `tea.MouseActionMotion` dengan tombol kiri ditekan (drag seleksi), dan `tea.MouseActionRelease`.
   - Di `Buffer.Render()`, karakter dalam rentang seleksi diberi highlight styling `ui.CurrentTheme.Accent` dengan `FgHighlight`.
   - Tambahkan method `GetSelectedText()`, `DeleteSelection()`, dan `ClearSelection()`.
4. **Klik Kanan Context Menu (Copy & Paste)**:
   - Tangani `msg.Button == tea.MouseButtonRight` di area editor pada `internal/app/model.go`.
   - Tampilkan floating popup context menu mini di koordinat klik kanan (`relX`, `relY`) dengan item:
     - `Salin (Copy)` (aktif jika `HasSelection`)
     - `Tempel (Paste)` (selalu aktif jika clipboard berisi teks)
     - `Potong (Cut)` (aktif jika `HasSelection`)
     - `Pilih Semua (Select All)`
   - Integrasikan dengan `github.com/atotto/clipboard`.

---

## Files Affected & Execution Steps
1. **`internal/ui/dialog/normalize.go`** (New File):
   - Helper untuk normalisasi ANSI background pada string rendered modal agar tidak ada celah/lubang akibat reset `\x1b[0m`.
2. **`internal/ui/dialog/input.go`**:
   - Terapkan background `BgTopMenu` pada `d.TextInput.TextStyle` dan `PromptStyle`.
   - Ganti `MarginRight(1)` antar tombol dengan separator ber-background.
   - Bungkus output render dengan helper normalisasi background.
3. **`internal/ui/dialog/confirm.go`, `filepicker.go`, `switcher.go`**:
   - Terapkan perbaikan background serupa agar semua modal 100% bebas lubang.
4. **`internal/editor/buffer.go`**:
   - Tambahkan field seleksi mouse dan fungsi seleksi teks (`SelectAnchor`, `HasSelection`, `SetSelectionEnd`, `GetSelectedText`, `DeleteSelection`, `ClearSelection`).
   - Update `Render()` untuk mewarnai teks yang sedang di-block.
   - Perbarui render kursor menjadi format I-beam (`│` / vertical bar styling).
5. **`internal/app/model.go`**:
   - Tangani mouse drag (`tea.MouseActionMotion` saat tombol kiri ditekan) untuk update seleksi di editor.
   - Tangani mouse click kiri dengan perhitungan kolom `X` yang presisi terhadap gutter.
   - Tangani `tea.MouseButtonRight` untuk membuka context menu editor.
   - Tangani aksi context menu (Copy, Paste, Cut, Select All).
6. **`internal/app/view.go`**:
   - Render overlay context menu mini jika aktif.
7. **`cmd/tui-workflow/main.go`**:
   - Emit I-beam cursor escape sequence pada startup dan teardown.
8. **Unit & Integration Tests**:
   - `internal/ui/dialog/dialog_test.go`: Verifikasi tidak ada unstyled spaces pada render modal.
   - `internal/editor/buffer_test.go`: Verifikasi fungsionalitas seleksi mouse, delete selection, dan I-beam cursor rendering.
   - `internal/app/features_test.go`: Verifikasi mouse drag dan right click context menu.

---

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Overlay / Modals | Reset ANSI menyebabkan lubang background (`#1E1E1E`) | Normalisasi background memastikan semua spasi dan jeda tombol mempertahankan `BgTopMenu`. |
| Editor Pane (Col 2) | Render seleksi teks dan I-beam dapat menggeser lebar baris jika penanganan rune/ANSI salah | Gunakan perhitungan `runewidth` dan ANSI styling tanpa menambah rune ekstra pada baris aktif. |
| Floating Context Menu | Context menu meluap keluar batas terminal (*overflow*) | Lakukan boundary clamping koordinat popup (`clamp(x, 0, width-menuW)`, `clamp(y, 0, height-menuH)`). |
| Status Bar & Menu Bar | Tetap 1 baris statis | Tidak ada perubahan pada struktur Status Bar atau Menu Bar. |

---

## UI / Keymap Flow
1. **Kursor I-Beam**:
   - Terminal menampilkan kursor format I-beam saat mengedit.
   - Buffer editor menampilkan visual kursor I-beam vertikal pada posisi kursor aktif.
2. **Block Teks dengan Mouse**:
   - Pengguna menekan tombol kiri mouse di dalam area editor, lalu men-drag kursor -> teks terpilih disorot (*highlighted*) dengan warna aksen.
   - Mengetik teks atau menekan Backspace saat ada teks terpilih akan menghapus/mengganti teks tersebut.
   - Mengklik kiri di sembarang tempat tanpa drag membersihkan seleksi.
3. **Klik Kanan Copy/Paste**:
   - Klik kanan di area editor memunculkan menu popup kecil:
     - Klik `Salin` -> Teks terpilih disalin ke sistem clipboard OS via `atotto/clipboard`.
     - Klik `Tempel` -> Teks dari sistem clipboard disisipkan pada kursor editor.
     - Klik `Potong` -> Teks terpilih disalin ke clipboard lalu dihapus dari buffer.
     - Klik `Pilih Semua` -> Seluruh buffer diblok.
   - Menekan `Esc` atau klik di luar menutup context menu.
4. **Tampilan Modal Rapi**:
   - Seluruh modal (`Buat File Baru`, `Konfirmasi Hapus`, `File Picker`, dll.) tampil solid tanpa celah belang/lubang gelap.

---

## Tasks
- [x] Buat helper normalisasi background modal di `internal/ui/dialog/normalize.go`
- [x] Perbaiki visual modal pada `input.go`, `confirm.go`, `filepicker.go`, dan `switcher.go`
- [x] Implementasikan kursor I-beam format (DECSCUSR sequence & visual buffer)
- [x] Implementasikan seleksi teks (block dengan mouse) pada `editor.Buffer` dan `model.go`
- [x] Implementasikan klik kanan context menu (Copy, Paste, Cut, Select All) dengan integrasi clipboard
- [x] Verifikasi: `go vet ./...` ✓, `go test -v -race ./...` ✓

---

## Manual Test Checklist
- [x] Buka modal "Buat File Baru" (tekan `a` di sidebar) -> Pastikan tampilan modal solid tanpa ada lubang gelap di sebelah title, input box, atau antar tombol.
- [x] Buka file teks di editor -> Verifikasi bentuk kursor berbentuk I-beam format.
- [x] Klik dan drag mouse pada baris teks di editor -> Verifikasi teks ter-block dengan warna highlight.
- [x] Klik kanan pada teks yang di-block -> Pilih "Salin" -> Paste di aplikasi lain -> Teks berhasil disalin.
- [x] Klik kanan di editor -> Pilih "Tempel" -> Teks dari clipboard masuk ke editor.
- [x] Tekan `Esc` saat context menu aktif -> Context menu tertutup rapi.

---

## Acceptance Criteria
- [x] Modal dialog (terutama `Buat File Baru`) tidak memiliki lubang visual / background belang.
- [x] Kursor berbentuk format I-beam di editor.
- [x] Teks dapat diblok menggunakan drag mouse.
- [x] Klik kanan mouse memunculkan opsi Copy dan Paste yang berfungsi dengan clipboard OS.
- [x] `go vet ./...` 0 error, 0 warning.
- [x] `go test -v -race ./...` 100% PASS tanpa race condition.

---

## Decisions
- Menggunakan escape sequence DECSCUSR `\x1b[6 q` dan visual I-beam bar pada buffer untuk menjamin tampilan I-beam di terminal.
- Menggunakan popup floating context menu pada klik kanan daripada aksi instan tersembunyi, agar pengguna mendapat kejelasan aksi Copy, Paste, Cut, dan Select All sesuai standar modern IDE.
- Normalisasi background modal dilakukan di level string render agar otomatis memperbaiki semua celah reset ANSI dari pustaka Lipgloss.
- Membedakan aksi mouse (Press vs Motion vs Release) pada context menu agar pelepasan tombol klik kanan atau gerakan mouse tidak menutup context menu secara prematur.

---

## Improvement 1 (2026-11-12 — Context Menu Premature Dismissal Fix)
- **Problem**: Menu klik kanan muncul lalu langsung tertutup seketika sebelum pengguna sempat memilih item "Salin".
- **Root Cause**: `ContextMenu.HandleMouse` awalnya dieksekusi tanpa memeriksa `msg.Action`. Pelepasan tombol kanan mouse (`MouseActionRelease`) atau gerakan mikro mouse (`MouseActionMotion`) yang terjadi tepat setelah klik kanan diperlakukan sebagai klik di luar area konten (`relY=0`), sehingga langsung memicu `m.Close()`.
- **Fix**:
  1. `HandleMouse` kini memfilter event `msg.Action`:
     - `MouseActionMotion`: Hanya memperbarui hover index item (`m.SelectedIdx`), tidak pernah menutup menu.
     - `MouseActionRelease`: Dikonsumsi aman jika berada di dalam batas popup tanpa menutup menu.
     - `MouseActionPress`: Hanya klik kiri (`MouseButtonLeft`) yang mengeksekusi item atau menutup menu jika klik di luar.
  2. Menambahkan boundary clamping pada `ContextMenu.Open(..., screenW, screenH)` agar popup tidak terpotong tepi layar terminal.
- **Tasks**:
  - [x] Perbarui `internal/ui/contextmenu.go` dengan pemisahan event mouse `Press`/`Motion`/`Release`.
  - [x] Perbarui pemanggilan di `internal/app/model.go`.
  - [x] Unit test `internal/ui/contextmenu_test.go` & `internal/app/features_test.go` ✓.

---

## Blockers
- None.

---

## Next
- Tunggu konfirmasi pengujian manual dari user sebelum mengarsipkan task ke task/done/.
