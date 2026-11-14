---
title: Search & Replace Rework (Sidebar, Engine, Find Bar)
status: review
started: 2026-11-15
completed:
branch:
priority: high
---

# Search & Replace Rework (Sidebar, Engine, Find Bar)

## Objective
Rework search & replace menjadi VSCode-style: sidebar Search yang rapi (input wrap, grouping per file, include/exclude, replace per-file & per-match), engine terpadu untuk sidebar + editor + Find Bar, performa async/debounced, dan replace aman (confirmation + buffer undo / disk mode-preserving).

## Plan
Detail (task breakdown, interfaces, steps, review focus): `docs/superpowers/plans/2026-11-15-search-replace-rework.md`
Design doc (approved): `docs/superpowers/specs/2026-11-15-search-replace-rework-design.md`

Ringkasan task:
1. Engine `internal/search/engine.go` (+ tests)
2. Async walker `internal/search/walk.go` (+ tests)
3. Editor buffer ke engine + MatchAt O(1)-ish + ReplaceMatchAt
4. Find Bar: Alt+C/W/X toggles + badges
5. Sidebar SearchView rewrite (textareas, grouping, toggles, debounce, HandleResults, DoSearchWord tetap)
6. Replace execution + ConfirmReplace modal (buffer path / disk path, count occurrences)
7. Hapus Global Search modal; F4/Ctrl+Shift+F → openSearchSidebar()
8. Mouse hit-testing + shortcut sync (help, menu, palette, README, PRD)
9. Verification: go vet, go test -race, wc -l budget, grep globalsearch

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Sidebar (28 col) | Chrome terlalu tinggi → list hilang | Chrome ≤ 11 baris; Path/Exclude 1 baris; match list dapat sisa tinggi |
| Top Menu / Tab Bar / Status Bar | Tidak berubah | Tidak ada baris baru di luar sidebar |
| ANSI | Potong string ANSI | ansi.Truncate/StringWidth, tanpa raw slicing |
| internal/app budget | keys_git.go/model_modals.go bertambah | Logic berat dipindah ke internal/search & internal/app/replace.go |

## UI / Keymap Flow
- Alt+3 → sidebar Search baru: Find textarea (2 baris) + toggle row `[Aa][ab|][.*]` + Replace textarea (2 baris) + Path (1) + Exclude (1) + `[Find] [Replace All]` + hasil digroup per file `▸ path (N) [Replace]`.
- Tab/Shift+Tab: Find → Replace → Path → Exclude → list. ↑/↓ pilih match. Enter buka file line:col.
- Alt+Enter = replace match; Alt+Shift+Enter = replace in file; [Replace All] = konfirmasi global (ConfirmReplace).
- Alt+C / Alt+W / Alt+X = case / whole-word / regex. F4 & Ctrl+Shift+F = fokus sidebar Search (modal dihapus). Ctrl+Enter tetap git commit.
- Ctrl+F / Ctrl+H Find Bar: tambah badge toggle + Alt+C/W/X.

## Tasks
- [x] Task 1: Engine internal/search/engine.go + tests
- [x] Task 2: Async walker internal/search/walk.go + tests
- [x] Task 3: Editor buffer → engine, MatchAt O(1)-ish, ReplaceMatchAt
- [x] Task 4: Find Bar toggles + badges
- [x] Task 5: Sidebar SearchView rewrite
- [x] Task 6: Replace execution + ConfirmReplace modal
- [x] Task 7: Hapus Global Search modal; F4 reroute
- [x] Task 8: Mouse + shortcut sync (help/menu/palette/README/PRD)
- [x] Task 9: Verification (vet, test -race, budgets)

## Manual Test Checklist
- [ ] Alt+3: layout baru (4 textarea wrap, toggles, grup per file, tombol Replace per file)
- [ ] Live search saat mengetik, tidak freeze; "500+" saat truncated
- [ ] Path: internal/ menyaring; Exclude: *.md menyaring; exclude kosong tetap menyembunyikan .git
- [ ] Alt+C/W/X toggle; regex invalid → hint inline, tidak crash
- [ ] Enter buka file; Alt+Enter replace 1 match; Alt+Shift+Enter replace 1 file (konfirmasi); Replace All (konfirmasi)
- [ ] File terbuka di tab dirty: replace masuk buffer (Ctrl+Z undo); AutoSave off → toast stale
- [ ] F4 buka sidebar Search (bukan modal); Shift+F12 tetap berfungsi; Ctrl+Enter tetap commit

## Acceptance Criteria
- [ ] Semua tasks selesai; go vet bersih; go test -race ./... hijau
- [ ] Satu engine dipakai sidebar + editor + Find Bar
- [ ] Replace: count occurrence benar, file mode terjaga, buffer/disk path sesuai aturan, selalu via konfirmasi (kecuali per-match)
- [ ] Shortcut baru tercantum di help (?), menu, palette, README, PRD
- [ ] wc -l internal/app/*.go ≤ 500 untuk semua non-test

## Decisions
- Ruling eksekusi: tidak ada git commit (branch dev bersih untuk review user)
- Ruling test: TestMarkdownPreviewCopyAndPaste gagal (headless tty, no clipboard) — pre-existing, di luar scope; TestTerminalCloseConfirmationWithRunningProcess flaky (5/5 pass in isolation) — unrelated; keduanya dicatat di ledger
- Approach B (unified engine) — dipilih user; A meninggalkan duplikasi, C menambah dependensi eksternal
- Replace safety D: konfirmasi + buffer path (undo) untuk file terbuka, disk path (mode asli) untuk sisanya
- F4/Ctrl+Shift+F → sidebar Search; Global Search modal dihapus (disetujui user)
- Ctrl+Enter tetap git commit; per-match = Alt+Enter; per-file = Alt+Shift+Enter
- Include/exclude = gitignore-style globs; kosong = semua/default ignores
- AutoSave on → buffer tersimpan pasca replace; off → toast stale
- Debounce 300ms; cap 500 match ("500+")
- Tidak ada git commit (butuh persetujuan user)

## Blockers
- (tidak ada)

## Next
Menunggu approval user untuk eksekusi task. Setelah approve: status in_progress, eksekusi per plan (executing-plans), update checklist bertahap.
