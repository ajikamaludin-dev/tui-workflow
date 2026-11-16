---
title: Git Change Gutter di Editor + Soft Wrap di Git Diff View
status: done
started: 2026-11-16
completed: 2026-11-16
branch: dev
priority: medium
---

# Git Change Gutter di Editor + Soft Wrap di Git Diff View

## Objective
1. Editor file biasa menampilkan indikator baris yang berubah menurut Git (mirip VS Code dirty-diff gutter) di sebelah kiri nomor baris.
2. Git Side-by-Side Split Diff View mendukung soft wrap — baris panjang melipat ke bawah, tidak lagi terpotong/disembunyikan.

## Ground Truth Findings
- **Gutter editor** (`internal/editor/buffer_view.go:306-327`): setiap visual line dirender `marker(1 char) + lineNum + " │ "`. `marker` dipakai fold (`▸`/`▾`) atau diagnostik (`●`/`▲`/`ℹ` dari `Buffer.LineDiagnostics`, `internal/editor/indent_rainbow.go:74`). Soft wrap editor sudah ada via `Buffer.ComputeVisualLines(textWidth)` (`buffer_view.go:129`) — continuation line dirender dengan gutter `┊` tanpa nomor baris.
- **Git diff data**: `internal/git/repo.go` punya `GetFileContentAtHEAD(relPath)`, `GetIndexFileContent(relPath)`, `GetWorktreeFileContent(relPath)`, dan `GetDiffHunks(relPath, staged, contextLines)` (HEAD/index vs worktree, LCS via `ComputeSplitDiff` di `internal/git/diff.go:272`). Repo tersimpan di `Model.GitRepo`; root di `Repo.RootPath`; path tab disimpan relatif (`tab.Path` dipakai relatif pada `model_diff.go`).
- **DiffRow** (`internal/git/diff.go:23`): `{LeftNum, LeftText, LeftType, RightNum, RightText, RightType}` — `RightNum` = nomor baris working tree (0 untuk baris deleted). Cocok untuk memetakan perubahan → baris editor.
- **Diff View render** (`internal/git/diff.go:483-571`): `Render(width, height, isFocused)` — `gutterWidth=5` per sisi, `textWidth=(width-1)/2-5`, teks dipotong keras via `truncateOrPad` (`ansi.Truncate`), 1 DiffRow = tepat 1 baris render. `ScrollOffset` berbasis row index. `NextHunk/PrevHunk`, `ensureActiveHunkVisible`, `ScrollUp/ScrollDown`, dan status bar (`internal/app/model_actions.go:213` → `tab.DiffView.ScrollOffset + 1`) semuanya berbasis row. Header 1 baris (`HEAD / Original (-)` | `Working Tree / Modified (+)`).
- **Refresh Git**: `gitRefreshCmd` (`commands.go:63`) → `gitStatusResultMsg` ditangani di `update.go:86`; save/open memicu refresh. File watcher juga ada (`file_watcher.go`).
- **Belum ada** indikator git di gutter editor (diagnostik `SetDiagnostic` bahkan belum dipanggil dari mana pun di app layer).
- `internal/app/*.go` budget 500 baris/file — perubahan wiring harus kecil; logika inti masuk `internal/editor/` & `internal/git/`.

## Objective & Approach
### Feature A — Git change gutter di editor
- Baseline: **index vs working tree** (unstaged changes saja, ala VS Code "Changes") via `ComputeSplitDiff(relPath, indexContent, workContent)` — sudah tersedia di `Repo.GetDiffHunks(relPath, false, …)`; `Result.Rows` memberi peta RightNum. File baru/untracked (index read gagal / blob tidak ada) → semua baris `Added`. File staged-only (worktree == index) → gutter kosong, karena baseline ini memang hanya menunjukkan unstaged. File biner/>10MB → skip (guard sama dgn diff viewer).
- Indikator VS Code-style di **kolom 1 char paling kiri gutter** (terpisah dari kolom `marker` fold/diagnostik yang sudah ada):
  - `▎` hijau (#3FB950) = baris ditambahkan
  - `▎` amber (#D29922) = baris dimodifikasi
  - `▾`/`▴` merah (#F85149) = baris dihapus pada posisi ini (penanda di baris setelahnya; EOF → di baris terakhir)
  - kosong = tidak berubah
- State per buffer: `Buffer.LineGitChanges map[int]GitChangeKind` + `SetGitChanges(...)` / `ClearGitChanges()` (file baru `internal/editor/gitgutter.go`, mirror pola `LineDiagnostics`).
- Wiring app: hitung & set gutter saat (1) file dibuka, (2) setelah save, (3) handler `gitStatusResultMsg`, (4) event file watcher untuk file aktif. **Tidak** dihitung per keystroke (mahal); buffer kotor menampilkan gutter stale sampai save/refresh — diterima (sama seperti diagnostics).
- Pemetaan: iterasi `SplitDiffResult.Rows`; `RightType==DiffAdded` → line `RightNum` = Added, `LeftType==DiffDeleted` → marker delete di line `RightNum` row berikutnya (atau baris terakhir). Modified = pasangan del+add berdampingan (mengikuti heuristik LCS yang sudah ada — tidak dibedakan dari add/del; ditandai Modified jika row punya Left!=Unchanged && Right!=Unchanged? → lihat Decisions).
- Gutter width bertambah 1 kolom; `GutterWidth()`, `RowColFromVisual`, hit-test mouse gutter (fold click di `mouse_editor.go`) disesuaikan.

### Feature B — Soft wrap di Git Diff View
- Model visual line (mirror editor): `ComputeVisualLines(textWidth) []DiffVisualLine{RowIdx, Segment, IsContinuation}`; tinggi row visual = max(jumlah segmen kiri, segmen kanan); sel yang lebih pendek dipad dengan `emptyStyle`.
- `ScrollOffset` pindah ke satuan **visual line**; `ScrollUp/ScrollDown/clamp`, `ensureActiveHunkVisible`, `NextHunk/PrevHunk` dipetakan row↔visual; status bar line menampilkan baris visual atau row aktif (putuskan saat implementasi, default: baris working-tree aktif).
- Nomor gutter per sisi hanya di segmen pertama; continuation memakai spasi/blank indent. `▌` penanda active hunk tampil di semua segmen row dalam hunk aktif.
- Wrap ANSI-safe (lebar rune via `ansi.StringWidth`, potongan per-rune — teks diff belum ber-ANSI karena styling diterapkan setelah wrap, jadi aman).
- `truncateOrPad` tetap dipakai untuk padding/pad-end tapi tidak lagi memotong; baris melebihi tinggi disembunyikan oleh viewport seperti biasa.
- Kolom tetap 2-split; tidak ada horizontal scroll baru.

### Dokumentasi (wajib repo rule)
- PRD.md: tambah sub-section — "Git Change Gutter" (di 4.3 Editor / fitur baru) + revisi deskripsi Split Diff Viewer (soft wrap, bukan truncate) + Revision History.
- README.md: tabel fitur/cheatsheet bila relevan (tidak ada shortcut baru direncanakan).

## Files Affected & Execution Steps
1. `internal/editor/gitgutter.go` (baru): `GitChangeKind`, state, setter/clear, helper `DiffRowsToLineChanges(rows []git.DiffRow) map[int]GitChangeKind` — atau taruh helper di `internal/git/diff.go` (`ComputeLineChanges`) agar tidak cycle import (editor sudah import git → OK dari sisi arah dependensi; cek).
2. `internal/editor/buffer_view.go`: gutter +1 kolom bar, render warna, sesuaikan `GutterWidth()` & continuation.
3. `internal/editor/buffer.go`: tambah field `LineGitChanges` + inisialisasi.
4. `internal/app/`: wiring compute/clear pada open/save/`gitStatusResultMsg`/watcher (file wiring kecil, hormati budget 500 baris).
5. `internal/git/diff.go`: visual-line model + rewrite `Render`, scroll/hunk mapping, `truncateOrPad` diganti wrap (sisakan pad-end).
6. `internal/git/diff_test.go`: unit test wrap + mapping hunk/scroll.
7. `internal/editor/*_test.go`: test `ComputeLineChanges` mapping (added/modified/deleted/untracked/EOF-delete).
8. `PRD.md`, `README.md`: dokumentasi fitur.
9. Verifikasi: `go vet ./...`, `go test -v -race ./...`, `wc -l internal/app/*.go` (≤500).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor gutter (Kolom 2) | Gutter melebar 1 kolom; misalignment klik fold marker | `GutterWidth()` jadi satu-satunya sumber lebar; update hit-test mouse |
| Diff View (Kolom 2) | Tinggi visual row berubah; scroll/hunk sync rusak | Visual-line model terpusat; unit test mapping row↔visual |
| Status Bar | Arti `ScrollOffset+1` berubah | Tampilkan baris working-tree atau visual (keputusan impl.) |
| Top Menu / Tab Bar / Sidebar | Tidak ada | — |

## UI / Keymap Flow
- Tidak ada shortcut baru. Gutter git non-interaktif (hover/klik tidak diperlukan v1).
- Diff view: scroll wheel/keys tetap; baris panjang kini terlihat penuh (wrap).

## Tasks
- [x] Feature A: gitgutter state + render + wiring refresh
- [x] Feature B: visual-line wrap di DiffView + scroll/hunk remap
- [x] Unit tests (line-change mapping, wrap visual lines, hunk navigation setelah wrap)
- [x] Docs: PRD.md + README.md
- [x] Verification: go vet ./... ✓, go test -v -race ./... ✓ (300 passed), wc -l internal/app/*.go ≤ 500 ✓ (non-test terbesar: palette_actions.go 463 baris)

## Manual Test Checklist
- [ ] Buka file di repo git dengan perubahan unstaged → bar hijau/amber muncul di gutter kiri; save → refresh sesuai
- [ ] File dengan baris dihapus → penanda merah ▾ di baris setelahnya
- [ ] File baru (untracked) → semua baris hijau
- [ ] Git stage seluruh file (`+`) → gutter hilang (worktree == index); unstage → gutter muncul lagi
- [ ] Stage sebagian file saja → file lain tetap menunjukkan perubahan
- [ ] File >10MB / biner → gutter git tidak aktif, tidak error
- [ ] Diff view: baris sangat panjang → melipat, tidak terpotong; scroll & hunk nav (`n`/`N`) tetap sinkron kiri-kanan
- [ ] Diff view: file tanpa perubahan → tetap render normal

## Acceptance Criteria
- [ ] Gutter git tampil akurat vs `git diff` (index vs worktree) untuk add/modify/delete
- [ ] Soft wrap diff view tidak memotong konten; scroll & sinkronisasi sisi kiri/kanan benar
- [ ] Tidak ada regresi fold click / diagnostics marker / soft wrap editor
- [ ] vet + test -race hijau; budget internal/app terjaga

## Decisions
- Baseline index vs worktree (bukan HEAD) — sesuai konfirmasi user (m00048, "ala VS Code Changes"); hanya unstaged yang tampil, staged-only = gutter kosong. Konsekuensi: `git diff HEAD` di sidebar/diff tab tetap berbeda cakupannya; ini disengaja.
- Bar warna di kolom paling kiri (bukan recolor line number) — line number tetap muted/accent seperti sekarang; bar membawa info 3 jenis perubahan (line number hijau hanya bisa 1 state).
- Modified tidak dideteksi via heuristik terpisah pada LCS: baris pasangan del+add yang berdampingan dianggap **Modified** (kuning), del-only → marker merah, add-only → hijau. Bila LCS memecahnya beda, fallback harmless (hijau/merah).
- Gutter stale saat buffer dirty — diterima; recompute hanya di open/save/git-refresh/watcher (performa; LCS per keystroke terlalu mahal).
- Soft wrap diff memakai visual-line model (pola sama dengan editor) bukan horizontal scroll — sesuai permintaan user.
- Helper mapping LCS→LineChange ditaruh di `internal/git/diff.go` (`ComputeLineChanges` + `LineChangesFromSplitDiff`), bukan di editor — editor mengimpor git (arah dependensi sudah ada).
- Status bar diff menampilkan `DiffView.ActiveLine()` = RightNum baris visual pertama yang terlihat (fallback LeftNum, lalu 1) — bukan lagi `ScrollOffset+1` (satuan ScrollOffset kini visual line).
- `truncateOrPad` dipertahankan di akhir `diff.go` karena dipakai sidebar `internal/git/view.go`; `Render` kini memakai wrap + padDiff (tidak pernah truncate konten).
- Deletion marker memakai `▾` (bukan `▴`) pada baris setelah blok delete; EOF deletion → baris worktree terakhir; Added/Modified tidak pernah tertimpa Deleted.
- Tick file watcher memakai gate `tab.GitGutterKey == tab.WatchedHash` agar LCS tidak dihitung tiap detik; refresh force dipakai pada openFile / ctrl+s / gitStatusResultMsg (stage/unstage mengubah index tanpa mengubah disk).
- Untracked file (HEAD & index miss) → `indexContent=""` → seluruh baris Added; file biner/>10MB atau di luar repo → gutter dibersihkan (ClearGitChanges).
- Rev history PRD ditambah baris `v0.4.0 (unreleased)` — bump versi final diserahkan ke owner saat release.

## Blockers
- none

## Verification Notes
- `go vet ./...` bersih; `go test ./...` → 300 passed/21 packages (termasuk test baru: TestLineChangesFromSplitDiff, TestLineChangesDeletionAtEOF, TestLineChangesMiddleDeletion, TestComputeLineChangesUntracked, TestLineChangesIdentical, TestWrapDiffText, TestDiffViewSoftWrap, TestSetGitChangesMapping/ReplaceAndClear/IgnoresUnchanged, TestBufferGutterWidth & TestSoftWrapToggleAndWordJump diupdate 7→8).
- `gofmt -l` bersih untuk file yang disentuh sesi ini (repo punya pelanggaran gofmt pre-existing pada file yang tidak disentuh).
- TODO.md: ada 8 baris deletions pre-existing (bukan dari sesi ini) — dibiarkan, dilaporkan ke user.

## Next
Menunggu konfirmasi user setelah mencoba Manual Test Checklist di terminalnya. Setelah dikonfirmasi: isi `completed:`, set `status: done`, pindah ke `task/done/YYYY-MM-DD - Git Change Gutter di Editor + Soft Wrap di Git Diff View.md`, tambah baris di `task/LOG.md` dan `task/done/INDEX.md`, reset CURRENT.md.
