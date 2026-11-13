# tui-workflow ⚡

> **Modern, High-Performance Terminal User Interface (TUI) Workflow Environment**  
> Dibangun dengan **Go 1.24+** dan ekosistem **Charmbracelet (Bubble Tea, Lipgloss, Bubbles, Glamour, Chroma, go-git/v5)**.

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![PRD Version](https://img.shields.io/badge/PRD-v0.1.1-blueviolet)](PRD.md)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](https://github.com)

---

## 📌 Gambaran Umum (Overview)

`tui-workflow` adalah aplikasi terminal modern berbasis Go yang mengadopsi fleksibilitas antarmuka IDE grafis (seperti VS Code) ke dalam terminal. Dirancang agar **sangat cepat**, **ringan** (< 30 MB RAM), **keyboard-driven**, dan **mouse-friendly**, memungkinkan developer mengelola file, menyunting kode dengan syntax highlighting real-time, meninjau perubahaan Git secara side-by-side, dan melihat preview Markdown secara langsung tanpa meninggalkan terminal.

---

## ✨ Fitur Utama (Key Features)

### 1. 🖥️ Layout 4 Zona Adaptif
- **Baris 0 (Top Menu Bar):** Menu dropdown interaktif `[File]`, `[Setting]`, dan `[About]` yang dapat diakses via keyboard (`Alt+F`, `Alt+T`, `Alt+A`) maupun klik mouse.
- **Kolom 1 (Sidebar Multifungsi):** Lebar 28 kolom yang dapat di-toggle (`Ctrl+B`) dengan tab switcher `[ 1:Files | 2:Git ]` (`Alt+1` / `Alt+2`).
- **Kolom 2 (Editor & Diff Pane):** Area kerja fleksibel dengan multi-tab bar, nomor baris, dan penanda status perubahan buffer.
- **Baris Bawah (Status Bar):** Menampilkan badge Auto-Save, nama file, posisi kursor (`Ln/Col`), deteksi bahasa sintaksis, encoding UTF-8, dan pesan toast notifikasi.
- **Terminal Guard:** Pengaman batas minimum terminal **80 × 24 karakter** dengan overlay peringatan informatif saat layar terlalu kecil.

### 2. 📝 Modeless Text Editor & Syntax Highlighting
- **Modeless Editing:** Pendekatan intuitif ala VS Code tanpa kerumitan modal Vim (ketik langsung tanpa menekan `i`).
- **Line-based Buffer Engine:** Navigasi kursor presisi (panah, `Home`, `End`, `Page Up`, `Page Down`), soft-wrap baris panjang, dan indentasi 4 spasi via `Tab`.
- **Real-Time Chroma Highlighting:** Tokenisasi sintaksis langsung saat mengetik untuk berbagai bahasa populer (Go, Markdown, JSON, YAML, Shell, Python, JavaScript, TypeScript, HTML, CSS, SQL, Rust, C/C++).
- **Auto-Save on Focus Change:** Otomatis menyimpan buffer saat berpindah tab atau fokus (`Alt+S` untuk toggle on/off).

### 3. 📑 Multi-Tab Manager & Quick Buffer Switcher
- **Single-Line Guarantee:** Tab bar dijamin selalu 1 baris, dilengkapi auto-truncate judul dan scrolling window horizontal `<` / `>` saat tab melebihi lebar layar.
- **Quick Buffer Switcher (`Ctrl+P`):** Modal pencarian cepat untuk berpindah antar tab buffer aktif dengan filter realtime.
- **Unsaved Changes Guard:** Modal konfirmasi keamanan saat menutup tab buffer yang belum tersimpan (`[Save]`, `[Discard]`, `[Cancel]`).

### 4. 📖 Live Markdown Preview (Glamour Engine)
- **Dual-Mode In-Place Toggle:** Beralih instan antara mode editor raw dan tampilan rendered Glamour via pintasan `Ctrl+M` / `Alt+M` atau tombol `[Preview]` / `[Raw]` di pojok kanan tab bar.
- **Smart Caching:** SHA-256 hash + terminal width caching untuk performa rendering kilat tanpa re-parsing dokumen yang tidak berubah.
- **Viewport Scrolling:** Navigasi scrolling preview via keyboard maupun mouse wheel.

### 5. 🌿 Git Source Control & Side-by-Side Split Diff
- **Native go-git/v5 Engine:** Integrasi langsung membaca status repositori tanpa dependensi binary git eksternal.
- **Sidebar Git View:** Menampilkan collapsible sections *Staged Changes*, *Working Tree Changes*, dan riwayat commit (*Commit History*).
- **Interactive Staging:** Tombol `[+]` untuk stage (`git add`) dan `[-]` untuk unstage file via klik mouse atau tombol keyboard.
- **Commit Box:** Input commit message dengan tombol shortcut `Ctrl+Enter` untuk melakukan commit langsung.
- **Side-by-Side Split Diff Tab:** Tab diff khusus di editor yang membandingkan versi HEAD vs Working Tree secara berdampingan dengan sinkronisasi scroll baris dan pewarnaan diff jelas (merah/hijau).

### 6. 🛡️ File Safety Guards
- **Binary File Rejection:** Pengecekan null byte (`0x00`) pada 512 byte pertama untuk menolak file biner (ELF, gambar, database) agar terminal tidak rusak/crash.
- **File Size Limit:** Batas keamanan ukuran file maksimum **10 MB** untuk mencegah out-of-memory.

### 7. 🖱️ Ergonomi Mouse Penuh (`tea.WithMouseCellMotion`)
- Klik pada menu bar dan item dropdown.
- Klik untuk beralih tab sidebar (`1:Files` / `2:Git`).
- Klik expand/collapse dan buka file pada File Explorer.
- Klik tab editor untuk aktivasi dan tombol `[×]` untuk menutup tab.
- Klik tombol `[+]` dan `[-]` pada Git sidebar.
- Mouse wheel scrolling pada editor buffer, markdown preview, dan split diff view.

### 8. 💻 Integrated Pseudo-Terminal (PTY) Tab
- **Full PTY Shell:** Membuka sesi shell interaktif langsung di tab editor menggunakan `creack/pty` dan emulasi VT100/Xterm via `hinshun/vt10x`.
- **Dukungan Coding Agent & TUI:** Berjalan mulus menjalankan TUI nested dan coding agent seperti `pi`, `claude`, `btop`, `git`, `python`, `node`.
- **Zero Scroll Overflow:** Clamping visual width dengan `ansi.Truncate` menjamin render terminal tidak pernah meluap dari batas terminal.
- **Isolasi Keystroke & Safe Teardown:** Aliran input raw terisolasi di tab terminal aktif tanpa konflik dengan editor; teardown bersih mematikan proses shell tanpa zombie process.

---

## ⌨️ Daftar Pintasan Keyboard (Keybindings)

### Global Shortcuts
| Pintasan | Aksi |
|---|---|
| `Ctrl + Q` | Keluar dari aplikasi (dengan konfirmasi jika ada buffer kotor) |
| `Ctrl + Shift + \` / `Ctrl + T` | Buka tab terminal terintegrasi baru |
| `Ctrl + \` | Pindah fokus antara Terminal dan Sidebar |
| `Ctrl + B` | Sembunyikan / Tampilkan Sidebar (Toggle Sidebar) |
| `Ctrl + E` / `F6` | Pindah fokus antara Sidebar dan Editor |
| `Alt + 1` | Beralih Sidebar ke Tampilan File Explorer |
| `Alt + 2` | Beralih Sidebar ke Tampilan Git Source Control |
| `Alt + 3` | Beralih Sidebar ke Tampilan Cari & Ganti (Search & Replace) |
| `Alt + [` / `Alt + ]` | Persempit / Perlebar lebar Sidebar |
| `Alt + F` | Buka Menu Top Bar **File** |
| `Alt + E` | Buka Menu Top Bar **Edit** |
| `Alt + R` | Buka Menu Top Bar **Search** |
| `Alt + T` | Buka Menu Top Bar **Setting** |
| `Alt + A` | Buka Menu Top Bar **About** |
| `F1` / `Ctrl + Shift + P` | Buka Command Palette (semua perintah dan aksi) |
| `Ctrl + P` | Quick Open File di Workspace |
| `Ctrl + O` | Buka File Baru dari Disk (File Picker modal) |
| `Ctrl + K` | Buka Direktori / Folder Baru di Workspace |
| `F4` | Cari Teks Global di Seluruh Workspace (Global Search modal) |
| `Alt + S` | Toggle Auto-Save on Focus Change (ON / OFF) |
| `Alt + Z` | Toggle Soft-Wrap pada Editor |
| `Alt + I` | Konfigurasi Model AI (OpenAI API Format: Provider URL, API Key, Model) |
| `?` | Tampilkan Modal Panduan Pintasan Keyboard |
| `Esc` | Tutup menu dropdown, modal dialog, atau kembali ke editor |

### Text Editor
| Pintasan | Aksi |
|---|---|
| `Ctrl + S` | Simpan buffer aktif ke disk (opsional auto-format jika format_on_save aktif) |
| `Ctrl + Shift + I` / `Alt + Shift + F` | Format dokumen aktif menggunakan CLI formatter (Prettier, Biome, Pint, Gofmt, dll) |
| `Ctrl + W` | Tutup tab aktif |
| `Ctrl + N` | Pindah ke tab berikutnya secara berurutan |
| `Ctrl + Z` | Batalkan perubahan teks terakhir (Undo) |
| `Ctrl + Y` / `Ctrl + Shift + Z` | Ulangi perubahan teks yang dibatalkan (Redo) |
| `Ctrl + F` | Buka pencarian teks di buffer aktif (Find Bar) |
| `Ctrl + H` | Buka pencarian dan penggantian teks di buffer aktif (Find & Replace) |
| `Ctrl + G` | Lompat ke baris dan kolom tertentu (Go to Line / Column) |
| `Tab` / `Shift + Tab` | Indentasi / Outdentasi 4 spasi (mendukung seleksi multi-baris) |
| `Ctrl + /` (`Ctrl + _`) | Toggle komentar baris atau blok seleksi otomatis sesuai bahasa |
| `Ctrl + M` / `Alt + M` | Toggle mode Live Markdown Preview (Glamour) vs Raw Editor |
| `Ctrl + A` / `Ctrl + C` / `Ctrl + V` | Pilih Semua / Salin / Tempel Teks Clipboard |
| `Panah` / `Home` / `End` | Navigasi kursor per karakter dan baris |
| `Page Up` / `Page Down` | Navigasi kursor per halaman |

### File Explorer
| Pintasan | Aksi |
|---|---|
| `j` / `Down` | Sorot node berikutnya |
| `k` / `Up` | Sorot node sebelumnya |
| `Enter` / `l` | Buka file ke tab editor / Buka-tutup folder |
| `h` | Tutup folder atau lompat ke parent folder |
| `a` | Buat file baru pada direktori yang disorot |
| `A` | Buat folder baru pada direktori yang disorot |
| `F2` | Ubah nama file / folder terpilih (Rename) |
| `d` | Hapus file / folder terpilih (dengan konfirmasi dialog) |
| `/` | Filter daftar file secara instan (real-time tree filter) |
| `.` | Toggle tampilkan / sembunyikan hidden files (dotfiles) |

### Git Source Control
| Pintasan | Aksi |
|---|---|
| `Up` / `Down` | Navigasi daftar file atau riwayat commit |
| `Enter` / `Space` | Stage / Unstage file yang sedang dipilih (`+` / `-`) |
| `d` / klik `[x]` | Buang semua perubahan lokal pada file terpilih (Discard Changes) |
| `Alt + B` | Buka pemilih branch Git lokal (Branch Switcher modal) |
| `Tab` | Pindah fokus ke kotak input pesan commit |
| `Alt + G` / klik `[✨ AI]` | Hasilkan pesan commit otomatis menggunakan AI dari perubahan staged |
| `Ctrl + Enter` | Lakukan Commit dengan pesan yang diketik |

### Terminal Tab (PTY)
| Pintasan | Aksi |
|---|---|
| `Ctrl + Shift + \` / `Ctrl + T` | Buka tab terminal terintegrasi baru |
| `Ctrl + \` / `Ctrl + \`` / `F6` | Pindah fokus antara Terminal dan Sidebar |
| `Alt + W` / `Ctrl + Shift + W` | Tutup tab terminal aktif |
| `Ctrl + N` | Pindah ke tab berikutnya |
| `Ctrl + Shift + P` / `F1` | Buka Command Palette |
| `exit` | Tutup shell dan tutup tab terminal otomatis |

---

## 🚀 Instalasi & Menjalankan

### Kebutuhan Sistem (Prerequisites)
- **Go 1.24** atau versi yang lebih baru terpasang di sistem.
- Terminal modern yang mendukung ANSI escape sequences dan mouse tracking (misal: Kitty, Alacritty, WezTerm, iTerm2, Windows Terminal, GNOME Terminal).

### 1. Clone Repositori
```bash
git clone https://github.com/your-username/tui-workflow.git
cd tui-workflow
```

### 2. Unduh Dependensi
```bash
go mod tidy
```

### 3. Jalankan Aplikasi
```bash
# Menjalankan langsung pada direktori saat ini
go run ./cmd/tui-workflow

# Atau jalankan dengan argumen direktori kerja spesifik
go run ./cmd/tui-workflow /path/to/project
```

### 4. Kompilasi Binary Mandiri (Build)
```bash
go build -o tui-workflow ./cmd/tui-workflow
./tui-workflow
```

Atau gunakan `Makefile`:

```bash
# Kompilasi ke bin/tui-workflow
make build

# Pasang ke ~/.local/bin (folder home user)
make install

# Hapus binary hasil build
make clean
```

`make install` menaruh binary di `$(HOME)/.local/bin/tui-workflow` (dapat diubah via `make install INSTALL_DIR=/path/lain`). Pastikan direktori tersebut ada di `PATH` agar perintah `tui-workflow` bisa dipanggil dari mana saja. Jalankan `make help` untuk daftar target lengkap (`build`, `install`, `uninstall`, `run`, `test`, `vet`, `clean`).

---

## 🧪 Pengujian & Kualitas Kode (Test Suite)

Aplikasi dilengkapi dengan test suite menyeluruh yang mencakup unit test komponen, simulasi memory leak (tab churn), dan pengujian integrasi End-to-End headless Bubble Tea:

```bash
# Menjalankan seluruh pengujian dengan deteksi race condition
go test -v -race ./...

# Validasi linter standar Go
go vet ./...
```

---

## 📂 Struktur Proyek (Directory Structure)

```text
tui-workflow/
├── cmd/
│   └── tui-workflow/
│       └── main.go              # Titik masuk aplikasi (Program initialization)
├── docs/
│   └── architecture.md          # Spesifikasi teknis arsitektur 4-zona & event loop
├── internal/
│   ├── ai/                      # Integrasi model AI (OpenAI API Format commit messages)
│   ├── app/
│   │   ├── e2e_test.go          # End-to-End headless test suite
│   │   ├── keymap.go            # Konstanta fokus & pemetaan pintasan
│   │   ├── model.go             # Model Bubble Tea utama (Root Model)
│   │   ├── resize.go            # Pengecekan ukuran layar minimum (80x24)
│   │   └── view.go              # Perakitan layout utama 4-zona & overlays
│   ├── config/                  # Konfigurasi aplikasi & persistensi JSON
│   ├── editor/
│   │   ├── autosave.go          # Logika auto-save pada pergantian fokus
│   │   ├── buffer.go            # Custom line-based text buffer engine
│   │   ├── leak_test.go         # Uji performa & memory leak tab churn
│   │   ├── manager.go           # Multi-tab bar manager & horizontal scroll
│   │   └── tab.go               # Struktur tab (editor, diff, markdown, terminal)
│   ├── explorer/
│   │   ├── node.go              # Struktur node tree direktori & file
│   │   ├── operations.go        # Logika filesystem CRUD (Create, Rename, Delete)
│   │   └── tree.go              # Komponen navigasi treeview explorer & marquee
│   ├── formatter/               # Integrasi CLI Code Formatter (gofmt, prettier, shfmt)
│   ├── git/
│   │   ├── diff.go              # Engine komputasi LCS Side-by-Side Split Diff
│   │   ├── repo.go              # Wrapper API go-git/v5
│   │   └── view.go              # Komponen Sidebar Git view & commit box
│   ├── logger/                  # Debug & event logger
│   ├── markdown/
│   │   └── renderer.go          # Glamour Markdown live preview dengan cache
│   ├── search/                  # Global Workspace Search engine
│   ├── syntax/
│   │   └── highlighter.go       # Chroma real-time tokenizer & detector
│   ├── terminal/
│   │   ├── terminal.go          # Embedded subshell PTY pseudoterminal manager
│   │   └── terminal_test.go     # Test suite subshell PTY terminal
│   └── ui/
│       ├── contextmenu.go       # Interactive pop-up context menu (Editor, Tab, Explorer)
│       ├── dialog/
│       │   ├── aisettings.go    # Modal konfigurasi endpoint AI
│       │   ├── branch.go        # Modal Branch Switcher Git
│       │   ├── confirm.go       # Modal konfirmasi (Delete & Unsaved changes)
│       │   ├── filepicker.go    # Modal navigasi file picker
│       │   ├── findbar.go       # Floating search & replace bar
│       │   ├── globalsearch.go  # Modal pencarian teks global workspace
│       │   ├── help.go          # Modal interaktif bantuan & panduan pintasan keyboard
│       │   ├── input.go         # Modal input teks (New file, folder, rename)
│       │   ├── normalize.go     # Normalisasi background ANSI modal dialog
│       │   ├── palette.go       # Modal Command Palette (F1 / Ctrl+Shift+P)
│       │   ├── quickopen.go     # Modal Quick Open File Workspace (Ctrl+P)
│       │   └── switcher.go      # Modal Quick Buffer Switcher
│       ├── menu.go              # Top Menu Bar component & dropdown handler
│       ├── statusbar.go         # Status bar bawah interaktif 1 baris
│       └── theme.go             # Skema warna palet VS Code Dark & styling
├── CONTRIBUTING.md               # Panduan kontribusi proyek
├── go.mod
├── go.sum
├── LICENSE                      # Lisensi MIT
├── PRD.md                       # Dokumen Spesifikasi Requirement Produk
└── README.md                    # Dokumentasi Proyek
```

---

## 🤝 Kontribusi (Contributing)

Tertarik berkontribusi? Baca [Panduan Kontribusi](CONTRIBUTING.md) untuk memahami standar teknis, arsitektur TUI 60 FPS, dan alur pengiriman Pull Request.

---

## 📄 Lisensi (License)

Proyek ini dilisensikan di bawah lisensi [MIT](LICENSE). Silakan gunakan, modifikasi, dan distribusikan secara bebas.
