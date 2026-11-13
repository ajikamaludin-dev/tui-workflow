---
title: P0 - Workspace/Session Persistence & Git Hunk-Level Diff Actions
status: review
started: 2026-11-13
completed:
branch: dev
priority: high
---

# P0 - Workspace/Session Persistence & Git Hunk-Level Diff Actions

## Objective
Menyelesaikan dua pekerjaan P0 sesuai `PRD.md` v0.1.1 dan keputusan D1–D5: (1) **Workspace/Session Persistence** metadata-only per-workspace dengan startup priority argumen CLI, dan (2) **Git Hunk-Level Diff Actions** (`stage-hunk`, `unstage-hunk`, `discard-hunk`) dengan UX lengkap di Diff Tab + Git Sidebar, aman, non-regresif, dan terdokumentasi (Rule 11).

## Plan
Eksekusi dilakukan end-to-end pada codebase:
- **P0-A (D1–D3):** Index session `~/.config/tui-workflow/config.json` + state per-workspace `~/.config/tui-workflow/sessions/*.json` (versioned, atomic, backward-compatible). Capture/restore metadata aman (tab path-backed valid, active tab, cursor/scroll, markdown preview, sidebar visible/width/view, focus) tanpa dirty buffer/PTY/diff transien; validasi file (exists, non-binary, ≤10MB) dengan skip/fallback non-blocking. Startup: argumen CLI eksplisit diprioritaskan, tanpa arg → restore session workspace, fallback perilaku lama deterministik. Persist via debounce + trigger state relevan/quit.
- **P0-B (D4–D5):** Fondasi hunk berbasis diff terhitung (`DiffHunk`/range) dan operasi repo `StageHunk`, `UnstageHunk`, `DiscardHunk` dengan fallback aman file-level untuk kasus ambigu/tidak terjamin (partial apply gagal, adjacent/context mismatch, CRLF sensitif, untracked/delete/rename/conflict, binary/oversize) tanpa korupsi index/worktree. UX & navigasi/seleksi hunk aktif di **Diff Tab** dan **Sidebar Git**, 3 aksi konsisten (keyboard+mouse/palette/menu), indikator state, toast/refresh jelas, `discard-hunk` selalu konfirmasi modal. Sinkron Rule 11: help modal, top menu, command palette, README, PRD.
- **Verifikasi:** `go vet ./...` dan `go test -v -race ./...` hijau; subset race `internal/app, internal/git, internal/config, internal/ui/dialog` lulus.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Status Bar / Toast | Spam notifikasi persist/hunk | Toast ringkas, debounce persist, error/warning non-blocking |
| Diff Tab (editor pane) | Overflow indikator/navigasi hunk, lag render | Indikator ringkas 1-baris, navigasi berbasis index hunk, reuse diff yang sudah dihitung |
| Git Sidebar (kolom 1) | Kepadatan info hunk/aksi | Tampilan hunk/aksi ringkas per file terpilih, clamp scroll/focus, fallback file-level jelas |
| Startup/Restore flow | Blokir startup atau crash saat file invalid | Validasi + skip non-blocking, fallback deterministik, no dirty restore |

## UI / Keymap Flow
- **Session restore:** Buka tanpa arg → pulihkan tab/state workspace terakhir (valid only). Buka dengan arg path eksplisit → prioritas arg, fallback aman.
- **Sidebar Git:** `+/-` stage/unstage file-level; `[`/`]` navigasi hunk aktif; `s/u` stage/unstage hunk aktif; `x` discard hunk aktif (unstaged, modal konfirmasi); `Enter` buka diff; refresh status/diff konsisten.
- **Diff Tab:** `n`/`]` next hunk, `N`/`[` prev hunk, `s`/`+` stage hunk (unstaged diff), `u`/`-` unstage hunk (staged diff), `x` discard hunk (unstaged diff, konfirmasi), scroll & indikator hunk aktif.
- **Rule 11 sinkron:** Help modal (`?`), Top Menu (`Git` menu), Command Palette (`git.hunk_*`), `README.md`, `PRD.md`.

## Tasks
- [x] P0-A: index `config.json` + session file per-workspace (schema/versioned/atomic/backward-compatible) dan wiring config/app.
- [x] P0-A: capture/restore metadata session (tabs aktif/cursor/scroll/preview/sidebar/focus) + startup priority D3 + persist debounce/trigger & quit-save.
- [x] P0-B: fondasi hunk (`DiffHunk`, hitung hunk/range, patch selection/apply helpers) + repo ops `StageHunk`/`UnstageHunk`/`DiscardHunk` + fallback aman.
- [x] P0-B: wiring UI/aksi hunk di Diff Tab + Sidebar Git, konfirmasi discard, refresh diff/status, command palette/menu integration.
- [x] Rule 11: update help modal, menu, command palette, `README.md`, `PRD.md`.
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓ (subset race app/git/config/dialog ✓).

## Manual Test Checklist
- [ ] **Session restore:** buka workspace (tanpa arg) → tutup → buka lagi tanpa arg: tab path-backed, tab aktif, cursor/scroll, mode preview markdown, sidebar visible/width/view, dan fokus pulih; file invalid/hilang/biner/>10MB dilewati tanpa crash.
- [ ] **Startup priority:** jalankan dengan argumen path file/folder eksplisit → argumen diprioritaskan; perilaku fallback lama tetap benar.
- [ ] **Persist trigger:** pindah tab/fokus/sidebar/preview lalu restart singkat → state terbaru tersimpan (debounce, tanpa lag/flicker).
- [ ] **Sidebar Git hunks:** pada file changes, navigasi hunk `[`/`]`, `s` stage hunk aktif, lihat pindah ke Staged; `u` unstage hunk; `x` discard hunk unstaged memunculkan konfirmasi dan hanya membuang hunk terpilih; status/diff refresh konsisten.
- [ ] **Diff Tab hunks:** buka diff staged/unstaged, navigasi `n`/`]`/`N`/`[`, `s`/`+` stage, `u`/`-` unstage, `x` discard (unstaged, konfirmasi) bekerja pada hunk aktif dengan indikator/toast jelas.
- [ ] **Fallback aman:** kasus ambigu/untracked/delete/rename/conflict/partial-apply-tidak-aman tidak korup index/worktree dan memberi feedback/fallback file-level yang jelas.
- [ ] **Rule 11 UI/docs:** `?` (help), menu `Git`, Command Palette menampilkan shortcut/aksi hunk; `README.md` & `PRD.md` sinkron.
- [ ] **Regresi & kualitas:** `go vet ./...` dan `go test -v -race ./...` tetap hijau; tidak ada flicker/overflow 1-baris, dirty buffer tidak ikut dipersist, tidak ada mutasi git terlarang.

## Acceptance Criteria
- [x] Session workspace persist/restore metadata-only (D1–D3) deterministik, aman, backward-compatible, dan non-blocking.
- [x] `stage-hunk`/`unstage-hunk`/`discard-hunk` tersedia di Diff Tab & Sidebar Git (D4–D5) dengan fallback aman, konfirmasi discard, dan refresh konsisten.
- [x] Shortcut/menu/command palette/help/docs tersinkron (Rule 11).
- [x] `go vet ./...` dan `go test -v -race ./...` lulus.
- [ ] Konfirmasi review/manual test oleh user.

## Decisions
- **D1:** `~/.config/tui-workflow/config.json` sebagai index; session per-workspace di `~/.config/tui-workflow/sessions/*.json` (versioned, atomic, migrasi/backward-compatible).
- **D2:** Metadata saja (tabs path-backed valid, active tab, cursor/scroll, markdown preview, sidebar visible/width/view, focus). Tanpa isi buffer dirty, tanpa PTY/diff transien. Restore tervalidasi + skip/fallback non-blocking.
- **D3:** Startup priority A — argumen CLI eksplisit diprioritaskan; tanpa arg restore session workspace; fallback perilaku lama deterministik & tanpa regresi.
- **D4:** Hunk ops `stage-hunk`/`unstage-hunk`/`discard-hunk` lengkap berbasis hunk terpilih, correctness-first dengan fallback aman file-level untuk edge case/ambigu; tanpa korupsi index/worktree; `discard-hunk` selalu konfirmasi.
- **D5:** UX entrypoint B — Diff Tab + Sidebar Git, navigasi/seleksi hunk aktif, aksi konsisten, indikator/toast/refresh jelas, Rule 11 sinkron.
- **Implementasi:** Persist session dipicu via debounce pada perubahan state relevan dan saat quit; operasi hunk memakai patch berbasis unified-diff selection dari hunk terhitung dengan fallback aman saat apply tidak terjamin.

## Blockers
- Tidak ada blocker teknis saat ini. Perlu konfirmasi manual oleh user di terminal untuk perilaku interaktif (session restore UX, mouse/keyboard hunk flow, dan fallback edge cases nyata).

## Next
1. User melakukan manual test checklist di terminal (terutama session restore & hunk actions pada repo nyata).
2. Jika ada temuan/regresi: tambahkan `## Improvement N` di task ini, perbaiki, dan verifikasi ulang.
3. Setelah user konfirmasi OK: set `completed`, ubah `status: done`, arsipkan ke `task/done/YYYY-MM-DD - P0 - Workspace-Session Persistence & Git Hunk-Level Diff Actions.md` (mv byte-identical), update `task/LOG.md` dan `task/done/INDEX.md`, lalu reset `task/CURRENT.md` ke template kosong.
