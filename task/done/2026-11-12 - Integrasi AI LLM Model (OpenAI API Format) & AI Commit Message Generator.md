---
title: Integrasi AI LLM Model (OpenAI API Format) & AI Commit Message Generator
status: done
started: 2026-11-12
completed: 2026-11-12
branch: dev
priority: high
---

# Integrasi AI LLM Model (OpenAI API Format) & AI Commit Message Generator

## Ground Truth Findings
1. **Config Persistence (`internal/config/config.go`)**:
   - `Config` struct menyimpan pengaturan user ke `~/.config/tui-workflow/config.json`.
   - Perlu ditambah field `AI AIConfig` (`provider_url`, `api_key`, `models`, `default_model`) dengan backward-compatibility.
2. **Git Workflow (`internal/git/`)**:
   - `internal/git/repo.go` punya `GetStatus()`, `GetFileContentAtHEAD()`, dan `Commit()`.
   - Belum ada method `GetStagedDiff()` untuk mengekstrak diff perubahan staged dalam format unified text.
   - `internal/git/view.go` punya `CommitInput textinput.Model`, `GitFocusCommitInput`, serta status loading spinner.
3. **Modal & Dialog Architecture (`internal/ui/dialog/`)**:
   - Komponen modal terisolasi di `internal/ui/dialog/` (misal `input.go`, `branch.go`).
   - Butuh modal baru `AISettingsDialog` (`internal/ui/dialog/aisettings.go`) untuk edit 4 field: Provider URL, API Key, Available Models, dan Default Model.
4. **Shortcut & Navigation (`internal/app/`, `internal/ui/menu.go`)**:
   - Shortcut `Ctrl+G` sudah terpakai untuk Go-to-Line di Editor.
   - Shortcut aman untuk AI commit generator: `Alt+G` (atau tombol mouse `[✨ AI]` di samping input commit di sidebar Git).
   - Shortcut untuk konfigurasi AI: `Alt+I` (atau menu `Setting -> AI Settings...`).
   - Wajib update `renderHelpModal()` di `internal/app/view.go`, `MenuBar` di `internal/ui/menu.go`, dan `CommandPaletteItem` di `internal/app/model.go`.

## Dual-Lens Evaluation
- **Business & UX Lens**:
  - Pengguna tidak perlu mengetik commit message secara manual dari perubahan kompleks.
  - Kompatibel dengan semua provider format OpenAI (OpenAI, OpenRouter, Ollama lokal, Groq, vLLM, DeepSeek).
  - Feedback visual jelas: status loading/spinner selama request berjalan async, toast error informatif jika API key salah / koneksi gagal.
- **Senior Technical Lens**:
  - Zero-lag & non-blocking: pemanggilan API dilakukan via async `tea.Cmd` goroutine, UI tetap 60 FPS.
  - Zero external dependency: gunakan `net/http` standar Go, JSON payload standar OpenAI `/v1/chat/completions`.
  - Zero-regression ke editor line buffer dan git core.
  - Diff truncation safety: potong diff jika > 30.000 karakter agar tidak melebihi konteks window atau buang token sia-sia.

## Objective & Approach
1. Buat package `internal/ai` untuk HTTP client OpenAI-compatible (`/v1/chat/completions`) yang mengirim prompt diff dan menerima commit message.
2. Update `internal/config/config.go` dengan `AIConfig` dan default values.
3. Buat modal `AISettingsDialog` di `internal/ui/dialog/aisettings.go` untuk konfigurasi user friendly.
4. Tambah `GetStagedDiff()` di `internal/git/repo.go` untuk menyusun unified diff dari file staged.
5. Integrasikan ke `internal/git/view.go` (tombol `[✨ AI]` & visual indicator) dan `internal/app/model.go` (`Alt+G` trigger, `Alt+I` settings, tea.Cmd async handler).
6. Update Menu Bar, Command Palette, dan Help Modal (`?`).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Sidebar Git (Kolom 1) | Penambahan tombol `[✨ AI]` berpotensi overflow jika lebar sempit | Format ringkas `[✨ AI]` pada baris input commit, clamping dengan `ansi.Truncate` |
| Status Bar (Baris Bawah) | Pesan proses AI (generating/error) overflow | Gunakan toast message standar dengan auto-dismiss timer |
| Overlay Modal | Modal AI Settings tampil di tengah terminal | Pastikan `overlayCenter` menghormati lebar min 60 karakter dan tinggi proporsional |

## UI / Keymap Flow
1. **Konfigurasi AI (`Alt+I` atau Menu `Setting -> AI Settings...`)**:
   - Muncul modal 4 field input: Provider URL, API Key, Models, Default Model.
   - `Tab` / `Shift+Tab` navigasi antar field dan tombol `[ Save ]` / `[ Cancel ]`.
   - `Enter` pada Save atau `Ctrl+S` menyimpan ke `config.json`. `Esc` membatalkan.
2. **Generate Commit Message (`Alt+G` atau Klik `[✨ AI]` di Git Sidebar)**:
   - Validasi staged files. Jika kosong, toast "Tidak ada perubahan di staging area".
   - Jika ada, status loading "🤖 Menghasilkan commit message via AI..." + spinner aktif.
   - Hasil otomatis dimasukkan ke `CommitInput` di Git Sidebar.
   - User tinggal tekan `Ctrl+Enter` untuk finalisasi commit.

## Tasks
- [x] Task 1: Update `internal/config/config.go` & `config_test.go` (tambah `AIConfig`)
- [x] Task 2: Buat package `internal/ai` (`client.go`, `client_test.go`) dengan format OpenAI `/v1/chat/completions`
- [x] Task 3: Tambah method `GetStagedDiff()` di `internal/git/repo.go` & unit test di `repo_test.go`
- [x] Task 4: Buat `AISettingsDialog` di `internal/ui/dialog/aisettings.go` & `aisettings_test.go`
- [x] Task 5: Integrasi UI GitView (`[✨ AI]` button, click handler) di `internal/git/view.go`
- [x] Task 6: Integrasi event handling `Alt+G`, `Alt+I`, async `tea.Cmd`, menu bar, command palette, dan help modal di `internal/app/`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [x] Tekan `Alt+I`: Muncul dialog AI Settings dengan nilai default terisi.
- [x] Ubah Provider URL, API Key, Models, Default Model, klik `[ Save ]`: tersimpan di `config.json`.
- [x] Di Git Sidebar dengan file staged, tekan `Alt+G` atau klik `[✨ AI]`: Muncul loading spinner lalu textinput commit terisi pesan conventional commit.
- [x] Tekan `?`: Tertera shortcut `Alt+G` dan `Alt+I` di modal bantuan.
- [x] Buka Command Palette (`F1`), cari "AI": Muncul opsi generate commit dan AI settings.

## Acceptance Criteria
- [x] Pengaturan AI tersimpan persisten di `config.json`.
- [x] Async call tidak menyebabkan UI lag atau freeze saat generate.
- [x] Error network/API (koneksi putus, API key salah) ditangani anggun via status bar toast tanpa crash.
- [x] Semua tes unit lulus tanpa race condition (`go test -v -race ./...`).
- [x] `go vet ./...` lulus bersih 0 error.

## Decisions
- Menggunakan `Alt+G` untuk generate commit karena `Ctrl+G` sudah dipakai untuk Go-to-Line di Editor.
- Menggunakan `net/http` standar tanpa library pihak ketiga untuk menjaga zero-dependency dan binary footprint ramping.
- Menargetkan perubahan di staging area (staged changes) sesuai semantik git commit standar.
- Diff dibatasi 24.000 karakter (~6.000 token) untuk melindungi latency dan batas payload.

## Blockers
- None.

## Improvement 1 (2026-11-12 — AI Modal UI Fixes, Compact Corner Button, Debug Logger & SSE/JSON Handler)

### Ground Truth & Root Cause Analysis
1. **Modal "Berlubang" & Mouse Unfriendly (`internal/ui/dialog/aisettings.go`)**:
   - `aisettings.go` tidak memanggil `NormalizeDialogBackground`, dan mengatur `Background` eksplisit pada span teks kecil (`titleStyle`, `labelStyle`) yang menyebabkan kontras blok hitam tidak rata dan sisa padding baris kehilangan background saat ANSI reset (`\x1b[0m`).
   - Handler `tea.MouseMsg` di `aisettings.go` masih berupa stub kosong tanpa mapping koordinat klik input dan tombol Save/Cancel.
2. **Posisi Tombol AI di Pojok Kanan Commit Box (`internal/git/view.go`)**:
   - Pengguna meminta tombol AI dipindah ke pojok kanan kotak input commit (sesuai tanda merah di screenshot), bukan berupa tombol terpisah di bawah `Commit`.
   - Tombol AI murni hanya bertindak sebagai "suggest commit message" yang mengisi `CommitInput` tanpa men-trigger aksi commit ke git.
3. **Error JSON Decode `invalid character 'd'` (`internal/ai/client.go`)**:
   - Karakter `'d'` muncul karena respon dimulai dengan `data: ...` (Server-Sent Events / SSE stream) yang terjadi saat router proxy seperti 9router/OneAPI/LiteLLM default ke streaming jika payload tidak menyertakan `"stream": false`.
   - Atau respon HTTP bukan JSON (seperti `default backend - 404`).
   - Solusi: eksplisit kirim `"stream": false`, tambahkan parser fallback untuk format SSE `data: ...`, dan berikan preview raw body saat unmarshal error.
4. **Sistem Logging Debug (`internal/logger/logger.go`)**:
   - Tambahkan flag CLI `--debug` atau env `DEBUG=1` yang mencatat request/response AI, payload diff, header (dengan masking API key), dan HTTP status code ke file `debug.log`.

### Tasks (Improvement 1)
- [x] Buat package `internal/logger/logger.go` & aktifkan via flag `--debug` di `cmd/tui-workflow/main.go`
- [x] Perbaiki visual modal di `internal/ui/dialog/aisettings.go` (hapus background inline terpotong, panggil `NormalizeDialogBackground`)
- [x] Tambahkan interaksi mouse lengkap pada `AISettingsDialog` (klik field 1-4 untuk fokus, klik Simpan/Batal, klik luar untuk close)
- [x] Pindahkan tombol `[✨]` / `[✨ AI]` ke pojok kanan kotak `CommitInput` di `internal/git/view.go`, dan hapus tombol AI terpisah di bawah
- [x] Tambahkan field `Stream: false` di `ChatCompletionRequest`, parser SSE stream `data: ...`, dan debug logging di `internal/ai/`
- [x] Verifikasi `go vet ./...` dan `go test -v -race ./...`

## Next
- Berikan rangkuman perbaikan dan panduan menjalankan dengan opsi `--debug` kepada user.
