---
title: Unblock LSP & Integrate Intelligent Code Assistance Suite in PRD
status: done
started: 2026-11-14
completed: 2026-11-14
branch: main
priority: high
---

# Unblock LSP & Integrate Intelligent Code Assistance Suite in PRD

## Objective
Mengintegrasikan 3 fase kapabilitas (Indent Rainbow, Snippet Engine, Linter Diagnostics) dan membuka blokir scope LSP (Language Server Protocol) menjadi 1 spesifikasi terpadu di `PRD.md` dan `AGENTS.md`.

## Ground Truth Findings
1. **PRD.md Saat Ini**:
   - `PRD.md` v0.1.1 mendefinisikan fitur editor di Bagian 4.3 (Tab Bar, Modeless Buffer, Markdown, Split Diff, Formatter CLI, PTY Terminal, In-Memory Symbol Index).
   - Di `AGENTS.md` Bagian 3, `LSP (Language Server Protocol) atau auto-complete engine kompleks` masih tercantum di daftar `OUT — Jangan dikerjakan di v1`.
2. **Kebutuhan Fitur Terpadu**:
   - **Indent Rainbow**: Visual tint per level indentasi dan deteksi error indentasi di editor buffer.
   - **React/ES7 Snippet Engine**: Template ekspansi instan (`rafce`, `rfce`, `useState`, dll.) via `Tab` / `Enter`.
   - **Linter & Diagnostics Bridge**: Background CLI worker (ESLint / Biome) dengan visual warning dots di gutter & status bar.
   - **LSP Client Engine (Unblocked)**: JSON-RPC 2.0 stdio client untuk auto-import, floating completion popup, dan Go to Definition berbasis language server.

## Plan
1. **Perbarui `PRD.md`**:
   - Buka blokir LSP pada arsitektur & scope.
   - Tambahkan sub-bab baru di Bagian 4.3: `#### H. Intelligent Code Assistance & LSP Integration Suite` yang mencakup:
     - 1. Visual Indentation Guide & Indent Rainbow.
     - 2. React & Component Snippet Expander.
     - 3. Workspace Linter & Diagnostics Bridge (ESLint / Biome CLI).
     - 4. Generic LSP Client Engine (JSON-RPC 2.0 stdio: Completion Popup, Auto Import, Go to Definition).
   - Perbarui Blueprint Struktur Direktori (Bagian 6) untuk modul `internal/lsp`, `internal/snippet`, `internal/linter`.
   - Perbarui Roadmap (Bagian 7) memasukkan fase integrasi Intelligent Code Assistance & LSP.
2. **Perbarui `AGENTS.md`**:
   - Hapus pembatasan LSP dari bagian `OUT — Jangan dikerjakan di v1` agar diizinkan secara resmi.
   - Tambahkan panduan zero-latency & non-blocking untuk arsitektur LSP dan snippet.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu (Row 0) | Overlap jika ada menu baru | Tidak ada perubahan baris menu; tetap 1 baris statis. |
| Column 1 (Sidebar) | Diagnostic panel / marker meluap | Diagnostic terintegrasi di status bar & gutter editor, sidebar tetap adaptif. |
| Column 2 (Editor Pane) | Floating completion popup & rainbow indent pecah/flicker | Indent rainbow di-render inline saat pemrosesan baris; floating popup menggunakan safe ANSI overlay clamping. |
| Status Bar (Bottom Row) | Teks diagnostik error/warning meluap melebihi lebar layar | Teks dipotong aman dengan `ansi.Truncate(msg, width, "")` menjamin tepat 1 baris. |

## UI / Keymap Flow
- **Indent Rainbow**: Otomatis aktif di buffer, indentasi valid diberi warna bertingkat, indentasi keliru diberi latar merah.
- **Snippet Engine**: Ketik prefix (misal `rafce`) ➔ tekan `Tab` ➔ blok kode komponen terbentang dengan kursor di tab-stop pertama.
- **Linter Diagnostics**: Muncul penanda `●` (merah) / `▲` (kuning) di gutter editor; status bar menampilkan detail pesan linter saat kursor berada di baris tersebut.
- **LSP Completion**: Popup apung muncul saat mengetik identifier ➔ navigasi `Up`/`Down` ➔ `Enter` / `Tab` untuk menyisipkan teks + auto import header.
- **LSP Go to Definition**: `F12` / `Alt+Click` melompat ke file & baris deklarasi target.

## Tasks
- [x] Susun draf revisi spesifikasi terpadu di `PRD.md` (Bagian 2, 4.3.H, 6, 7).
- [x] Buka blokir LSP di `AGENTS.md` (Bagian 3: Scope v1).
- [x] Verifikasi konsistensi dokumen & bebas konflik aturan.
- [x] Verification: go vet ./... ✓, go test ./... ✓

## Manual Test Checklist
- [x] Dokumen `PRD.md` memuat spesifikasi lengkap untuk Indent Rainbow, Snippet, Linter, dan LSP.
- [x] Dokumen `AGENTS.md` mengonfirmasi LSP telah di-unblock dari larangan scope.

## Acceptance Criteria
- [x] 3 fase rekomendasi (Indent Rainbow, Snippet, ESLint) + LSP terintegrasi dalam 1 bab utuh di `PRD.md`.
- [x] Larangan LSP di `AGENTS.md` dicabut secara resmi.

## Decisions
- Menggabungkan Indent Rainbow, Snippet, Linter CLI, dan LSP ke dalam satu modul terkoordinasi (Intelligent Code Assistance Suite) untuk menyatukan pengalaman VS Code di terminal.

## Blockers
- None.

## Next
- Tunggu approval user untuk menerapkan perubahan ke `PRD.md` dan `AGENTS.md`.
