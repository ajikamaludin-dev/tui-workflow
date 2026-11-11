# Panduan Kontribusi (Contributing Guide) — tui-workflow

Terima kasih atas ketertarikan Anda untuk berkontribusi pada **tui-workflow**!

Proyek ini bertujuan menghadirkan *Modern, High-Performance Terminal User Interface (TUI) Workflow Environment* berbasis Go dan ekosistem Charmbracelet. Untuk menjaga kualitas, stabilitas, dan performa terminal 60 FPS, seluruh kontributor wajib mengikuti panduan ini.

---

## 1. Kode Etik & Standar Teknis

Sebelum mengirimkan kontribusi, pastikan Anda memahami dan mematuhi aturan arsitektur berikut:

### A. Zero Terminal Scroll Overflow Mandate (Single-Line Guarantee)
- Komponen baris statis (**Top Menu**, **Tab Bar**, **Status Bar**) **wajib tepat 1 baris tinggi**.
- Dilarang keras membiarkan baris meluap melebihi lebar terminal atau memuat newline ekstra (`\n`). Luapan baris akan memicu *offscreen scroll* di terminal yang merusak posisi UI.
- Selalu batasi komponen baris dengan `MaxHeight(1)` dan potong aman dengan ANSI truncator.

### B. ANSI-Aware String Manipulation
- **Dilarang keras melakukan byte/string slicing mentah** (`str[start:end]`) pada string ber-ANSI sequences. Slicing mentah memutus sequence ANSI di tengah byte dan membocorkan kode escape ke layar.
- Gunakan helper parser ANSI seperti `ansi.Cut` atau `ansi.Truncate`.

### C. Clean Teardown & Alt-Screen Safety
- **Dilarang memanggil `os.Exit(0)` langsung** dari dalam event handler atau menu action saat aplikasi berjalan.
- Selalu gunakan `tea.Quit` agar Bubble Tea dapat membersihkan alternate screen buffer, kursor, dan mouse tracking.

### D. File Safety & Binary Guard
- Batas maksimal ukuran file yang dibuka adalah **10 MB**.
- Selalu periksa 512 byte pertama untuk mendeteksi null byte (`0x00`). Tolak file biner dengan peringatan status bar.

### E. Target Kinerja
- **Latensi Render:** < 16 ms per frame (target 60 FPS).
- **Konsumsi RAM Idle:** < 30 MB.
- Hindari kalkulasi berat sinkron di dalam method `Update()` atau `View()`. Gunakan caching (Chroma token cache, Glamour markdown hash SHA-256).

---

## 2. Struktur Proyek

Pahami pembagian paket sebelum mengubah atau menambahkan modul:

```text
tui-workflow/
├── cmd/tui-workflow/   # Entry point aplikasi (main.go)
├── internal/
│   ├── app/            # Root model Bubble Tea, layout builder, global dispatcher
│   ├── editor/         # Line buffer engine, tab manager, autosave logic
│   ├── explorer/       # File tree explorer & filesystem operations
│   ├── git/            # go-git/v5 wrapper, split diff engine, git sidebar
│   ├── markdown/       # Glamour renderer dengan smart caching
│   ├── syntax/         # Chroma syntax highlighting tokenizer
│   └── ui/             # Palette warna, menu bar, status bar, dialog modals
├── task/               # Tracking task pengembangan internal
├── LICENSE             # Lisensi MIT
├── PRD.md              # Spesifikasi kebutuhan produk (Product Requirement Document)
└── README.md           # Dokumentasi umum proyek
```

---

## 3. Persiapan Lingkungan Pengembangan

Pastikan dependensi berikut terpasang di sistem Anda:

- **Go 1.24+**
- Git
- Terminal emulator yang mendukung ANSI escape sequences & mouse motion tracking (misal: Linux Terminal, macOS Terminal/iTerm2, Windows Terminal).

### Langkah Setup

1. **Fork & Clone repository:**
   ```bash
   git clone https://github.com/<username>/tui-workflow.git
   cd tui-workflow
   ```

2. **Unduh dependensi Go:**
   ```bash
   go mod download
   ```

3. **Jalankan aplikasi di development:**
   ```bash
   go run ./cmd/tui-workflow
   ```

---

## 4. Alur Kerja Kontribusi

1. **Buat Branch Baru:**
   Gunakan nama branch deskriptif:
   ```bash
   git checkout -b feat/nama-fitur
   # atau
   git checkout -b fix/deskripsi-bug
   ```

2. **Tulis Kode Berkualitas:**
   - Format kode menggunakan `gofmt` atau `goimports`:
     ```bash
     gofmt -s -w .
     ```
   - Tulis unit test untuk logika baru (terutama di `internal/editor/`, `internal/git/`, `internal/explorer/`).
   - Hapus kode mati, komentar TODO tanpa konteks, atau debug print.

3. **Verifikasi Wajib Sebelum Commit:**
   Setiap perubahan wajib lulus uji linter dan test suite:
   ```bash
   # 1. Static analysis (wajib 0 error & 0 warning)
   go vet ./...

   # 2. Test suite dengan race detector (wajib 100% PASS)
   go test -v -race ./...
   ```

4. **Kirim Pull Request (PR):**
   - Berikan deskripsi PR yang jelas: apa masalahnya, apa solusinya, dan bagaimana cara mengujinya secara manual di terminal.
   - Cantumkan konfirmasi bahwa `go vet` dan `go test -v -race` telah lolos.

---

## 5. Melaporkan Masalah (Bug Report)

Jika menemukan bug atau crash, sertakan informasi berikut saat membuat issue:
- Sistem Operasi & Terminal Emulator yang digunakan.
- Versi Go (`go version`).
- Langkah-langkah reproduksi masalah secara runtut.
- Tangkapan layar (*screenshot*) atau rekaman terminal bila relevan (terutama untuk *rendering glitch* atau *layout overflow*).

---

## 6. Lisensi

Dengan berkontribusi pada proyek ini, Anda setuju bahwa seluruh kontribusi kode Anda dilisensikan di bawah lisensi [MIT](LICENSE).
