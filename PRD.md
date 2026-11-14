# Product Requirement Document (PRD)
## Project: `tui-workflow`
**Version:** v0.1.1  
**Status:** Approved  
**Language/Framework:** Go (1.24+), Bubble Tea Ecosystem 
## 1. Executive Summary
`tui-workflow` adalah aplikasi terminal modern (TUI) berbasis Go yang mengadopsi tata letak mirip Visual Studio Code. Aplikasi ditujukan untuk developer yang menginginkan editor teks cepat, ringan, keyboard-driven sekaligus mouse-friendly langsung di dalam terminal tanpa dependensi berat (Electron/GUI), dengan tetap mempertahankan kejelasan visual dari antarmuka multi-kolom, top menu bar, git source control, markdown preview terintegrasi, dan multi-tab editor.

---

## 2. Core Architecture & Tech Stack

| Layer | Technology | Version | Purpose |
|---|---|---|---|
| **Runtime** | Go | 1.24+ | Core execution runtime & concurrency |
| **TUI Core Engine** | `github.com/charmbracelet/bubbletea` | v1.3+ | Elm-based Model-Update-View architecture, event loop, mouse tracking (`tea.WithMouseCellMotion()`) |
| **Styling & Layout** | `github.com/charmbracelet/lipgloss` | v1.1+ | Flexible box model, borders, colors, horizontal/vertical join |
| **UI Components** | `github.com/charmbracelet/bubbles` | v1.0+ | Textinput, viewport, keymaps, help system |
| **Syntax Highlighting** | `github.com/alecthomas/chroma/v2` | v2.27+ | Terminal truecolor/256-color code syntax tokenizer & styler (integrated with custom line buffer) |
| **Markdown Renderer** | `github.com/charmbracelet/glamour` | v0.8+ | In-terminal rendered Markdown styling & preview engine |
| **Git Engine** | `github.com/go-git/go-git/v5` | v5.13+ | Pure Go Git implementation for status, diff, staging, commit, and log history |
| **File System I/O** | Standard Library (`os`, `path/filepath`, `io`) | Go 1.24 | Safe cross-platform directory traversal and file I/O |
| **PTY & Terminal Emulation** | `github.com/creack/pty`, `github.com/hinshun/vt10x` | v1.1+, v0.0+ | Pseudo-terminal allocation, shell process spawning, ANSI/VT100 screen emulation |

---

## 3. Layout Wireframe

Layout terminal dibagi menjadi 4 zona: Baris 0 (Top Menu), Kolom 1 (Sidebar: Files / Git), Kolom 2 (Editor Pane), dan Baris Bawah (Status Bar).

```text
┌─ [File] [Setting] [About] ────────────────────────────────────────────────────────────────────────┐
├─ [ 1:Files ] [ 2:Git ] ──────────┬─ Tab Bar: [main.go*] [Diff: prd.go] [README.md] [x] ───────────┤
│ ▼ tui-workflow/                  │ 1 package main                                                 │
│   ▶ cmd/                         │ 2                                                              │
│     main.go                      │ 3 import (                                                     │
│   ▼ internal/                    │ 4     "fmt"                                                    │
│     ▶ explorer/                  │ 5 )                                                            │
│     ▼ editor/                    │ 6                                                              │
│       buffer.go                  │ 7 func main() {                                                │
│       tabs.go                    │ 8     fmt.Println("tui-workflow running")                      │
│       diff.go                    │ 9 }                                                            │
│   go.mod                         │                                                                │
│   PRD.md                         │                                                                │
├──────────────────────────────────┴────────────────────────────────────────────────────────────────┤
│ [AutoSave: ON] | internal/editor/buffer.go | Ln 7, Col 5 | Go | UTF-8 | Alt+F: Menu | ?: Help      │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **Top Menu Bar:** Tinggi 1 baris di paling atas terminal (`Row 0`).
- **Lebar Kolom 1 (Sidebar):** 28–32 kolom terminal (dapat di-toggle dengan `Ctrl+b`).
- **Lebar Kolom 2 (Editor Pane):** Dinamis mengisi sisa terminal (`TerminalWidth - SidebarWidth`).
- **Tinggi Konten (Sidebar & Editor):** `TerminalHeight - 3` (Row 0 Menu Bar, Row 1 Tab Bar, Baris Akhir Status Bar).
- **Minimum Terminal Dimensions:** `80 x 24`. Jika ukuran terminal lebih kecil, aplikasi menampilkan Overlay Warning Screen.

---

## 4. Functional Specifications

### 4.1. Top Menu Bar (Row 0)
1. **Item Menu & Shortcut:**
   - `[ File ]` (`Alt+f`):
     - `New File` (`a`) ➔ Membuka modal dialog buat file baru.
     - `New Terminal Tab` (`Ctrl+Shift+\` / `Ctrl+T`) ➔ Membuka tab terminal interaktif baru.
     - `Save File` (`Ctrl+s`) ➔ Simpan buffer aktif ke disk.
     - `Close Tab` (`Ctrl+w`) ➔ Tutup tab aktif (dengan konfirmasi jika dirty).
     - `Exit` (`Ctrl+q`) ➔ Tutup aplikasi (dengan konfirmasi jika ada unsaved tabs).
   - `[ Edit ]` (`Alt+e`):
     - `Undo` (`Ctrl+z`) ➔ Batalkan perubahan teks terakhir.
     - `Redo` (`Ctrl+y`) ➔ Ulangi perubahan teks yang dibatalkan.
     - `Find...` (`Ctrl+f`) ➔ Buka Find Bar pada buffer aktif.
     - `Find & Replace...` (`Ctrl+h`) ➔ Buka Find & Replace bar pada buffer aktif.
     - `Go to Line...` (`Ctrl+g`) ➔ Lompat ke baris & kolom tertentu.
     - `Toggle Comment` (`Ctrl+/`) ➔ Toggle komentar baris/seleksi sesuai sintaks file.
   - `[ Search ]` (`Alt+r`):
     - `Command Palette...` (`F1`) ➔ Buka Command Palette (semua perintah).
     - `Quick Open File...` (`Ctrl+p`) ➔ Buka Quick Open workspace switcher.
     - `Find in Current File...` (`Ctrl+f`) ➔ Buka Find bar buffer aktif.
     - `Find & Replace...` (`Ctrl+h`) ➔ Buka Find & Replace bar buffer aktif.
     - `Find in Files (Global)...` (`F4`) ➔ Buka pencarian global di workspace.
     - `Go to Line...` (`Ctrl+g`) ➔ Lompat ke baris buffer aktif.
   - `[ Setting ]` (`Alt+t`):
     - `Toggle Sidebar` (`Ctrl+b`) ➔ Sembunyikan / tampilkan sidebar.
     - `Toggle Soft Wrap` (`Alt+z`) ➔ Nyalakan / matikan soft-wrap editor.
     - `Toggle Auto-Save` (`Alt+s`) ➔ Bergantian ON / OFF.
     - `Toggle Hidden Files` (`.`) ➔ Sembunyikan / tampilkan dotfiles di explorer.
     - `Switch Git Branch...` (`Alt+b`) ➔ Membuka modal dialog pilih branch Git.
     - `Keyboard Shortcuts` (`?`) ➔ Membuka modal cheatsheet shortcut.
   - `[ About ]` (`Alt+a`):
     - Membuka modal dialog info aplikasi (Nama, Versi v0.1.1, Tech Stack Go + Bubble Tea + Lipgloss, Lisensi).
2. **Interaksi Dropdown:**
   - Klik mouse pada judul menu untuk membuka dropdown.
   - Navigasi keyboard `Up` / `Down` dan `Enter` untuk memilih item.
   - Tombol `Esc` atau klik di luar dropdown untuk menutup (dismiss).

### 4.2. Sidebar Column 1: Multi-View (Files, Git & Search)
Sidebar memiliki tab bar navigasi di baris paling bawah untuk beralih tampilan: `[ 📁 Files ]`, `[ 🌿 Git ]`, dan `[ 🔍 Search ]`.
- Beralih tampilan via keyboard: `Alt+1` (Files) / `Alt+2` (Git) / `Alt+3` (Search & Replace), atau klik mouse pada tab footer di baris bawah.

#### A. View 1: File Explorer (Tree View & CRUD)
1. **Hierarchical Tree & Navigasi:**
   - Root folder dibaca secara rekursif lazy loading.
   - Folder dapat di-expand (`▼`) dan di-collapse (`▶`).
   - Indentasi 2 spasi per level kedalaman.
   - Navigasi: `j`/`Down`, `k`/`Up`, `Enter`/`Right` (buka file/expand), `Left` (collapse/parent), `r` (refresh disk), `.` (toggle hidden).
2. **Operasi File & Folder (CRUD):**
   - `a`: Buat file baru ➔ Membuka **Modal Dialog Input** di tengah layar untuk nama file relatif terhadap folder aktif.
   - `A`: Buat folder baru ➔ Membuka **Modal Dialog Input** untuk nama folder.
   - `F2` (atau `r` pada toolbar): Ubah nama (Rename) file/folder ➔ Membuka **Modal Dialog Input** berisi nama lama untuk diubah.
   - `d`: Hapus file/folder ➔ Membuka **Modal Konfirmasi Hapus** `[Delete] [Cancel]`.
   - `/`: Filter realtime pohon file (tekan Enter pada input kosong untuk membersihkan filter).
   - Klik Kanan pada node file/folder: Membuka Context Menu Explorer (New File, New Folder, Rename, Delete, Copy Relative Path).
3. **Interaksi Mouse:**
   - Klik pada folder: Toggle expand / collapse.
   - Klik pada file: Buka file ke Editor Tab.

#### B. View 2: Git Source Control (go-git backend)
1. **Komponen Tampilan Git:**
   - **Commit Input Box:** Textarea 1–3 baris di bagian atas dengan placeholder "Message (Ctrl+Enter to commit)".
   - **Staged Changes List:** Daftar file yang sudah di-stage. Menampilkan ikon status `(M)`, `(A)`, `(D)`.
   - **Changes List (Unstaged):** Daftar file modified / untracked di working directory.
   - **Commit History List:** Log 10 commit terakhir (hash pendek, pesan commit, author, waktu relatif).
2. **Aksi & Shortcut Git:**
   - `+` atau klik ikon `+`: Stage file (`git add`).
   - `-` atau klik ikon `-`: Unstage file (`git reset`).
   - `+` / `-`: Stage / unstage seluruh perubahan file terpilih (file-level).
   - `[` / `]`: Navigasi hunk aktif; `s` stage hunk, `u` unstage hunk, `x` discard hunk (unstaged, konfirmasi).
   - `Enter`: Buka Side-by-Side Diff Tab file terpilih.
   - `Alt+b` atau klik header branch: Buka modal dialog pemilihan dan perpindahan branch Git lokal.
   - `Alt+g` atau klik tombol `[✨ AI]`: Hasilkan pesan commit otomatis menggunakan LLM (OpenAI API format) dari staged changes.
   - `Alt+i` (atau menu Setting > AI Configuration): Konfigurasi endpoint OpenAI API, API Key, daftar model, dan model default.
   - `Enter` atau klik file pada daftar Git: Buka **Git Side-by-Side Split Diff Tab** di Kolom 2.
   - `Ctrl+Enter`: Jalankan commit jika pesan tidak kosong dan ada staged changes.
   - Klik tombol `[Push]`: Mengirim commit lokal ke remote repository (dengan loading indicator di status bar).

### 4.3. Column 2: Multi-Tab File Editor & Viewers

#### A. Tab Bar Manager
1. **Manajemen Tab:**
   - Mendukung multi-buffer: file teks biasa, file Markdown, dan Git Diff tab.
   - Active tab diberi highlight warna kontras.
   - Indikator dirty buffer: Tanda asterisk (`*`) jika ada perubahan belum disimpan.
   - Tombol tutup (`[x]`) pada tiap tab yang dapat diklik mouse.
2. **Tab Overflow Handling:**
   - Jika total panjang tab melebihi lebar kolom editor, nama file pada tab otomatis di-shrink / truncate dengan elipsis (`main.go` ➔ `ma...`).
   - Quick Buffer Switcher (`Ctrl+p`): Membuka fuzzy modal dropdown untuk beralih tab dengan cepat.
3. **Navigasi Tab:**
   - `Ctrl+n`: Tab berikutnya.
   - `Ctrl+p`: Tab sebelumnya / buffer switcher.
   - `Ctrl+w`: Tutup tab aktif.

#### B. Modeless Text Editor Buffer
1. **Model Interaksi (VS Code Style):**
   - Editor bersifat **modeless** (tidak menggunakan modal Vim NORMAL/INSERT). Pengguna langsung mengetik teks.
   - Navigasi kursor menggunakan tombol panah (`Left`, `Right`, `Up`, `Down`), `Home`, `End`, `PageUp`, `PageDown`.
   - Tombol `Tab`: Menyisipkan indentasi (default 4 spasi atau karakter tab) ke dalam buffer teks.
2. **Custom Line Buffer & Syntax Highlighting:**
   - Dibangun dengan struktur custom line-based buffer (`[][]rune` atau `[]string`) untuk performa tinggi.
   - Integrasi tokenisasi syntax highlighting real-time via `alecthomas/chroma/v2` saat teks diketik.
   - Line numbers gutter di sisi kiri dengan warna muted dan separator vertikal.
   - **Line Wrapping:** Fitur **Soft Wrap** aktif secara default (baris panjang melipat visual ke bawah tanpa menyisipkan karakter `\n` fisik).
   - **Multi-Cursor Editing (VSCode-Style):**
     - Menambah kursor pada kemunculan kata berikutnya via `Ctrl+Shift+D` (*Add Next Occurrence*).
     - Menambah kursor di atas / bawah baris aktif via `Ctrl+Alt+Up` / `Ctrl+Alt+Down`.
     - Menambah kursor bebas di posisi klik mouse via `Ctrl+Alt+Click`.
     - Penggabungan multi-cursor kembali ke single primary cursor via tombol `Esc`.
     - Operasi pengetikan karakter, penghapusan (`Backspace`/`Delete`), dan seleksi berlaku serentak pada semua kursor secara stabil (diurutkan bottom-to-top).
   - **Code Folding (Indent-Based):**
     - Melipat atau membuka blok kode berdasarkan indentasi baris via `za` atau `Ctrl+Shift+[` (fold) dan `Ctrl+Shift+]` (unfold).
     - Melipat semua blok kode terluar via `Ctrl+K Ctrl+0` / `zM` (*Fold All*), dan membuka seluruh lipatan via `Ctrl+K Ctrl+J` / `zR` (*Unfold All*).
     - Kolom 0 pada gutter nomor baris menampilkan penanda lipatan visual `▸` (blok terlipat) atau `▾` (blok terbuka) yang dapat diklik langsung dengan mouse untuk toggle fold/unfold.
     - Baris tersembunyi (*hidden lines*) dilewati secara otomatis dalam rendering tampilan editor dan perhitungan posisi kursor.
3. **Interaksi Mouse di Editor:**
   - Klik mouse di area buffer teks langsung memindahkan kursor ke posisi baris dan kolom yang diklik.
   - `Alt+Click` pada simbol/kata: Melompat langsung ke definisi (*Go to Definition*).
   - `Ctrl+Alt+Click`: Menambahkan multi-cursor di titik koordinat yang diklik.
   - Klik pada marker kolom 0 gutter: Melipat atau membuka blok kode (*Toggle Fold*).
   - Mouse wheel (scroll up / down) menggeser viewport secara vertikal.

#### C. Markdown Dual-Mode Viewer
1. **Deteksi File:** Berlaku untuk file berekstensi `.md`, `.markdown`.
2. **Mode Transisi In-Place (`Ctrl+m`):**
   - **Edit Mode:** Buffer teks biasa dengan Chroma syntax highlighting.
   - **Rendered Preview Mode:** Viewport yang dirender menggunakan engine `glamour` dengan format terminal truecolor/ASCII modern.
   - Status bar menampilkan tag `[PREVIEW]` saat preview aktif.
   - Scroll wheel dan keyboard navigasi tetap berfungsi untuk membaca dokumen pada mode preview.

#### D. Git Side-by-Side Split Diff Viewer
1. **Pemicu:** Dibuka otomatis saat memilih file di daftar Git Staged / Changes.
2. **Tampilan Side-by-Side:**
   - Kolom editor dibagi 2 secara vertikal:
     - **Sisi Kiri:** Versi HEAD / Index (Original) dengan highlight baris merah (`-`).
     - **Sisi Kanan:** Versi Working Tree (Modified) dengan highlight baris hijau (`+`).
   - Line number gutter independen pada kedua sisi.
   - Scrolling tersinkronisasi (synchronized vertical scroll) antara sisi kiri dan kanan.

#### E. Document Code Formatter Engine (CLI Integration)
1. **Pemicu:** Shortcut `Ctrl+Shift+I` / `Alt+Shift+F`, Top Menu `[File] ➔ Format Document`, Command Palette (`F1`).
2. **Format on Save:** Opsi toggle via `[Setting] ➔ Toggle Format on Save` atau opsi config `format_on_save`.
3. **Smart Fallback Chain:**
   - React / JS / TS (`.jsx`, `.tsx`, `.js`, `.ts`): Local `./node_modules/.bin/biome` / `prettier` ➔ Global `biome` ➔ `prettier` ➔ `deno fmt`.
   - Blade (`.blade.php`): `blade-formatter` ➔ Local/Global `pint`.
   - PHP (`.php`): Local `./vendor/bin/pint` ➔ Global `pint` ➔ `php-cs-fixer`.
   - Go (`.go`): `gofmt` ➔ `goimports`.
   - Python (`.py`): `ruff` ➔ `black` ➔ `autopep8`.
   - Rust (`.rs`): `rustfmt`.
   - C / C++ / Java / C#: `clang-format`.
   - Shell (`.sh`, `.bash`): `shfmt`.
   - HTML / CSS / JSON / YAML / Markdown: Biome ➔ Prettier.
   - Lua (`.lua`): `stylua`.
   - SQL (`.sql`): `sql-formatter`.
4. **Piping & Non-Blocking:** Mengalirkan buffer via STDIN/STDOUT dengan timeout 4s tanpa flickering atau blocking UI.
5. **Undo Support:** Hasil format dapat dibatalkan seketika menggunakan `Ctrl+Z` via `ReplaceContentWithUndo`.

#### F. Integrated PTY Terminal Tab
1. **Pemicu:** Shortcut `Ctrl+Shift+\` atau `Ctrl+T`, Top Menu `[File] ➔ New Terminal Tab`, Command Palette (`terminal.new`).
2. **Pseudo-Terminal (PTY) Allocation:**
   - Membuka instance shell interaktif pengguna (`$SHELL`, default `/bin/bash` atau `/bin/sh`) menggunakan `creack/pty`.
   - Mengemulasikan VT100/Xterm state machine menggunakan `hinshun/vt10x` secara real-time.
   - Mendukung penuh TUI app bertingkat (*nested alternate screen*) seperti code agent (`pi`, `claude`), system monitor (`btop`, `htop`), `git`, `python`, `node`, dll.
3. **Focus & Keystroke Isolation:**
   - Keystroke di-forward secara mentah (*raw byte streaming*) ke PTY saat tab aktif dan fokus berada di Editor/Terminal Pane.
   - Shortcut `Ctrl+\` / `Ctrl+` ` / `F6`: Beralih fokus antara Terminal dan Sidebar.
   - Shortcut `Alt+W` / `Ctrl+Shift+W` atau mengetik `exit`: Menutup tab terminal secara bersih (*clean teardown* proses shell & PTY descriptor).
   - Shortcut `Ctrl+Shift+P` / `F1`: Tetap dapat membuka Command Palette secara global.
4. **Window Resizing & Anti-Overflow:**
   - Sinkronisasi dimensi ukuran terminal via `TIOCSWINSZ` (`pty.Setsize`) dan `vt10x.Resize` saat resize terminal terjadi.
   - Pemotongan aman berbasis visual width (`ansi.Truncate`) menjamin output baris terminal tidak pernah meluap (*zero terminal scroll overflow*).

#### G. Symbol Navigation & Code Intelligence (Path B — In-Memory Symbol Index)
1. **In-Memory Symbol Index:**
   - Pemindaian dan pengindeksan simbol seluruh workspace secara non-blocking saat workspace dimuat.
   - Pembaruan inkremental otomatis (*zero stale state*) saat file disimpan, dibuat, diubah nama (*rename*), atau dihapus.
   - Ekstraktor simbol berbasis parser/AST & regex ringan:
     - **Go (`.go`):** AST parsing deklarasi fungsi, method (dengan receiver type), struct, interface, type alias, konstanta, dan variabel.
     - **JavaScript / TypeScript / React (`.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, `.cjs`):** Functions, arrow functions, classes, interfaces, types, exported constants.
     - **PHP (`.php`, `.phtml`):** Classes, interfaces, traits, functions, methods, constants, dan `define()`.
     - **JSON (`.json`):** Keys bersarang (*nested keys*) hingga kedalaman **depth ≤ 10** sebagai simbol `KindKey`, termasuk jalur bertitik (*dotted path*).
     - **Markdown (`.md`, `.markdown`):** Headings level 1–6 (`#{1,6}`) sebagai simbol `KindHeading`.
2. **Alur Navigasi Simbol:**
   - **Go to Definition (`F12` / `Alt+Click`):** Mengambil identitas simbol di bawah kursor, mencari di indeks simbol, dan melompat langsung ke file, baris, dan kolom target (membuka tab editor baru jika berada di file berbeda).
   - **Find References (`Shift+F12`):** Mencari seluruh kemunculan simbol di seluruh workspace berbasis batas kata (*word-boundary*), menampilkan hasil di sidebar Search, dan memfokuskan daftar kecocokan untuk navigasi cepat.
   - **Go to Symbol (`Ctrl+Shift+O`):** Dialog pencarian cepat simbol seluruh workspace dengan filter pencarian real-time dan navigasi instan.

### 4.4. Auto-Save & Dialog Unsaved Changes

#### A. Auto-Save on Focus Change
1. **Pemicu (Triggers):**
   - Perpindahan tab editor aktif (berpindah ke tab file lain).
   - Perpindahan fokus pane dari Editor ke Sidebar.
2. **Konfigurasi & Toggle:**
   - Dapat di-toggle ON/OFF kapan saja menggunakan shortcut `Alt+s` atau melalui Top Menu `[ Setting ] ➔ Toggle Auto-Save`.
   - Status bar menampilkan indikator `[AutoSave: ON]` atau `[AutoSave: OFF]`.
3. **Penanganan Error:**
   - Jika penyimpanan otomatis gagal (permission denied, disk full), dirty flag (`*`) tetap dipertahankan dan error toast merah non-blocking muncul di status bar.

#### B. Modal Dialog Unsaved Changes
1. **Pemicu:**
   - Menutup tab yang dirty via `Ctrl+w` atau klik `[x]` saat Auto-Save OFF atau gagal.
   - Menutup aplikasi via `Ctrl+q` saat masih ada satu atau lebih buffer dirty.
2. **Bentuk Tampilan:**
   - Modal popup di tengah layar dengan pesan: *"File `<name>` memiliki perubahan yang belum disimpan. Simpan sebelum menutup?"*
   - Opsi tombol: `[ Save ]`, `[ Discard ]`, `[ Cancel ]`.
   - Navigasi tombol via `Left`/`Right` + `Enter`, atau klik mouse langsung pada tombol.

### 4.5. Status Bar & Global Controls

1. **Indikator Status Bar (Baris Bawah):**
   - `[AutoSave: ON/OFF]`
   - Path file aktif relatif terhadap root project.
   - Posisi Kursor: `Ln X, Col Y` (disembunyikan saat mode Markdown Preview).
   - Tipe File & Syntax (contoh: `Go`, `Markdown`, `JSON`, `Git Diff`).
   - File Encoding: `UTF-8`.
   - Quick Help Shortcut: `Alt+F: Menu | Ctrl+E: Focus | ?: Help`.
2. **Navigasi Global:**
   - `Ctrl+e` atau `F6`: Beralih fokus antara Sidebar (Column 1) dan Editor Buffer (Column 2).
   - `Ctrl+b`: Toggle sembunyikan / tampilkan Sidebar (maksimalkan ruang editor).
   - `Ctrl+s`: Simpan manual buffer aktif.
   - `Ctrl+q`: Keluar aplikasi (dengan safety check unsaved buffer).
3. **Safety Guards:**
   - **Tolak File Biner:** Cek null bytes (`0x00`) pada 512 byte pertama file. Jika terdeteksi biner, batalkan pembukaan dan munculkan toast error: *"File biner tidak didukung"*.
   - **Batas Ukuran File:** Maksimal `10 MB`. Jika file lebih dari 10 MB, tolak pembukaan dengan toast: *"Ukuran file melebihi batas 10MB"*.
   - **Terminal Size Guard:** Jika dimensi terminal `< 80 x 24`, render **Overlay Warning Screen** penuh dengan teks *"Ukuran terminal terlalu kecil (Min: 80x24, Saat ini: WxH). Silakan perbesar jendela terminal."*



### 4.6. Workspace/Session Persistence (P0, Metadata-only)
1. **Storage (D1):** `~/.config/tui-workflow/config.json` sebagai index config/session (mapping root kanonik workspace → entry). Setiap workspace memiliki file session terpisah `~/.config/tui-workflow/session-<id>.json` (versioned, atomic write, deterministic, backward-compatible).
2. **Data (D2):** Hanya metadata UI/workspace: tab file/markdown path-backed valid, active tab, cursor/scroll, status preview markdown, visibilitas/lebar/view sidebar, dan focus. Tidak menyimpan isi buffer dirty, PTY terminal, atau diff transien.
3. **Startup Priority (D3):** Jika argumen CLI path eksplisit diberikan → restore session diutamakan. Tanpa argumen → restore session workspace terakhir; fallback ke perilaku default sebelumnya (termasuk auto-open `PRD.md` bila relevan) secara deterministik dan tanpa regresi.
4. **Keamanan/Validasi:** File yang tidak ada, biner, atau >10MB dilewati saat restore dengan fallback non-blocking. Persist dilakukan saat event relevan (switch/close tab, resize/sidebar/focus, save, exit) dengan debounce.

### 4.7. Git Hunk-Level Diff Actions (P0)
1. **Scope (D4):** `stage-hunk`, `unstage-hunk`, dan `discard-hunk` berbasis unified hunk terpilih. Fallback aman ke aksi file-level untuk kasus ambigu/tidak terjamin (partial apply gagal, adjacent/context mismatch, CRLF sensitif, file untracked/delete/rename/conflict, binary/oversize). Tidak ada korupsi index/worktree.
2. **UX (D5):** Navigasi/seleksi hunk aktif tersedia di **Diff Tab** dan **Git Sidebar**.
   - **Sidebar Git:** `[`/`]` navigasi hunk, `s` stage hunk (Changes), `u` unstage hunk (Staged), `x` discard hunk (Changes, konfirmasi), `Enter` buka diff, `+`/`-` aksi file-level.
   - **Diff Tab:** `n`/`]` next hunk, `N`/`[` prev hunk, `s`/`+` stage hunk, `u`/`-` unstage hunk, `x` discard hunk (unstaged, konfirmasi), indikator hunk aktif & status.
3. **Integritas:** `discard-hunk` selalu memerlukan modal konfirmasi. Setelah aksi, status Git dan tampilan diff direfresh konsisten. Shortcut/menu/command palette/help/docs disinkronkan (Rule 11).


---

## 5. Ringkasan Interaksi Mouse (Mouse Support Matrix)

Program diinisialisasi dengan `tea.WithMouseCellMotion()`.

| Komponen UI | Aksi Mouse | Hasil / Respon |
|---|---|---|
| **Top Menu Bar** | Left-Click pada `[File]`, `[Setting]`, `[About]` | Membuka dropdown menu yang bersangkutan |
| **Dropdown Item** | Left-Click pada opsi menu | Menjalankan aksi menu dan menutup dropdown |
| **Sidebar Tab Header** | Left-Click pada `[ 1:Files ]` / `[ 2:Git ]` | Berpindah antara tampilan File Explorer dan Git View |
| **File Explorer Tree** | Left-Click pada Folder node | Expand / Collapse subtree (`▼` / `▶`) |
| **File Explorer Tree** | Left-Click pada File node | Membuka file ke tab editor |
| **Git Staged/Changes** | Left-Click pada tombol `+` / `-` | Stage / Unstage file |
| **Git File Item** | Left-Click pada nama file | Membuka Side-by-Side Diff Tab |
| **Tab Bar** | Left-Click pada tab item | Mengaktifkan tab tersebut |
| **Tab Bar** | Left-Click pada tombol `[x]` tab | Menutup tab (trigger modal unsaved jika dirty) |
| **Editor Buffer** | Left-Click pada teks buffer | Memindahkan posisi kursor ke baris & kolom yang diklik |
| **Editor / Preview / Diff** | Mouse Wheel Up / Down | Scroll vertikal konten viewport |
| **Modal Dialog** | Left-Click pada tombol `[Save]`, `[Cancel]`, dll. | Mengeksekusi opsi dialog |

---

## 6. Directory Structure Blueprint

```text
tui-workflow/
├── cmd/
│   └── tui-workflow/
│       └── main.go              # Entry point aplikasi & Bubble Tea runner
├── internal/
│   ├── app/
│   │   ├── model.go             # Root Tea Model, event router & state machine
│   │   ├── view.go              # Lipgloss multi-column & top-menu layout builder
│   │   ├── keymap.go            # Global keybindings & shortcut definitions
│   │   └── resize.go            # Terminal size validation (< 80x24 overlay)
│   ├── ui/
│   │   ├── theme.go             # Palet warna Lipgloss (VS Code Dark theme)
│   │   ├── menu.go              # Top Menu Bar component & dropdown handler
│   │   ├── statusbar.go         # Status bar component & toast notification
│   │   └── dialog/              # Modal dialog components
│   │       ├── input.go         # Textinput dialog (buat file/folder, rename)
│   │       ├── confirm.go       # Konfirmasi unsaved changes & hapus file
│   │       ├── about.go         # About app info modal
│   │       └── switcher.go      # Quick buffer switcher modal (Ctrl+p)
│   ├── explorer/
│   │   ├── node.go              # Hierarchical tree node data structure
│   │   ├── tree.go              # Explorer navigation & selection model
│   │   └── operations.go        # Create, rename, delete file/dir FS operations
│   ├── git/
│   │   ├── repo.go              # go-git repository abstraction & status loader
│   │   ├── view.go              # Git sidebar component (Commit, Stage, History)
│   │   └── diff.go              # Side-by-side split diff calculator & styler
│   ├── editor/
│   │   ├── manager.go           # Tabs controller, active pointer, dirty tracker
│   │   ├── buffer.go            # Custom line-based buffer, cursor, soft-wrap
│   │   ├── tab.go               # Tab item struct (File, Markdown, Diff)
│   │   └── autosave.go          # Auto-save triggers & focus change watcher
│   ├── syntax/
│   │   └── highlighter.go       # Chroma integration, token lexer & caching
│   ├── formatter/
│   │   ├── formatter.go         # Smart fallback CLI formatters (Prettier, Biome, Pint, Gofmt)
│   │   └── formatter_test.go    # Unit testing & pipe verification
│   ├── markdown/
│   │   └── preview.go           # Glamour rendered preview viewport & toggle
│   ├── ai/
│   │   └── client.go            # OpenAI-compatible API client for AI commit message
│   ├── config/
│   │   └── config.go            # User configuration loader & persistence (~/.config/tui-workflow/config.json)
│   ├── logger/
│   │   └── logger.go            # Debug logging facility
│   ├── search/
│   │   └── search.go            # Global workspace text search & replace view
│   └── terminal/
│       └── terminal.go          # Embedded PTY terminal subshell runner
├── docs/
│   └── architecture.md
├── PRD.md
├── go.mod
└── go.sum
```

---

## 7. Phased Implementation Roadmap

- **Fase 1: Foundation & Shell Layout**
  - Inisialisasi Go module (`go mod init tui-workflow`).
  - Instal dependensi utama: `bubbletea`, `lipgloss`, `bubbles`, `chroma/v2`, `glamour`, `go-git/v5`.
  - Bangun Root Model dengan Top Menu Bar, 2 Kolom (Sidebar + Editor placeholder), dan Status Bar.
  - Implementasi Terminal Size Guard (`< 80x24` warning overlay) dan `tea.WithMouseCellMotion()`.

- **Fase 2: File Explorer & File CRUD**
  - Implementasi tree lazy-loading recursive filesystem scanner.
  - Render expandable nodes dengan mouse click & keyboard navigation.
  - Implementasi Modal Dialog Input untuk CRUD: buat file (`a`), buat folder (`A`), rename (`r`), dan hapus (`d`).

- **Fase 3: Modeless Editor & Syntax Highlighting**
  - Implementasi custom line-based buffer dengan cursor navigation dan soft-wrap.
  - Integrasi Chroma syntax highlighter real-time saat mengetik.
  - Multi-tab manager dengan auto-truncate judul tab dan Quick Buffer Switcher (`Ctrl+p`).
  - Implementasi Auto-Save on Focus Change (toggle `Alt+s`) dan Modal Unsaved Changes confirmation.

- **Fase 4: Git Source Control & Side-by-Side Diff**
  - Integrasi `go-git/v5`: read status, staged, unstaged, and recent commit history.
  - Implementasi sidebar tab switcher `[ 1:Files | 2:Git ]` (`Alt+1`/`Alt+2`).
  - Staging/unstaging file dan commit box (`Ctrl+Enter`).
  - Implementasi Git Side-by-Side Split Diff tab di kolom editor.

- **Fase 5: Markdown Dual-Mode & Top Menu Bar**
  - Implementasi Glamour in-place toggle (`Ctrl+m`) pada tab `.md`.
  - Implementasi Top Menu Bar (`[File]`, `[Setting]`, `[About]`) dengan dropdown interaktif dan shortcut `Alt`.
  - Guard tolak file biner dan batas ukuran file 10MB.

- **Fase 6: Testing, Refinement & Polish**
  - Unit test untuk file operations, buffer manipulation, git status parser.
  - Benchmarking rendering performance pada terminal besar.
  - Edge case testing: rapid resize, empty repository, permission errors.

---

## 8. ATURAN PENGEMBANGAN WAJIB: DECISION GATE (ASK_USER)

> ⚠️ **MANDATORY GOVERNANCE RULE FOR IMPLEMENTATION**:
>
> Selama proses pengembangan (development) oleh Orchestrator maupun Delegate (`worker`, `planner`, `reviewer`):
> 
> 1. **Zero-Assumption on Ambiguity:** Jika ditemukan skenario teknis, perubahan kontrak komponen, trade-off arsitektural, atau kebutuhan fitur baru yang belum didefinisikan secara eksplisit di dalam PRD ini, pelaksana **DILARANG KERAS** berasumsi, menebak preferensi, atau memutuskan secara sepihak.
> 2. **Wajib Memanggil `ask_user`:** Setiap ada pilihan arsitektural, percabangan logika, atau keputusan penting saat koding, Orchestrator **WAJIB** memanggil tool `ask_user` untuk mempresentasikan konteks, opsi, dan meminta keputusan langsung dari user sebelum menulis kode implementasi terkait.
> 3. **Konfirmasi Eksekusi:** Setiap transisi fase atau pendelegasian tugas ke sub-agent wajib dikonfirmasi terlebih dahulu kepada user.

