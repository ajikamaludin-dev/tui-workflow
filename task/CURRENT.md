---
title: Git Sidebar Right-Click Context Menu (Stage/Unstage, Open File, Copy Full Path)
status: planning
started: 2026-11-15
completed:
branch: dev
priority: medium
---

# Git Sidebar Right-Click Context Menu (Stage/Unstage, Open File, Copy Full Path)

## Objective
Klik kanan pada baris file di section **Staged Changes** / **Changes** pada Git sidebar membuka context menu berisi: Stage/Unstage (adaptif per section), Open File (selalu editor), Open Diff, Copy Full Path, dan Discard Changes (khusus Changes) — setara ergonomi context menu file explorer & tab bar yang sudah ada.

## Context & Prior Task
Task sebelumnya (GitHub Release Automation, 2026-11-15) ditandai `done` dan diarsipkan oleh instruksi eksplisit user **meskipun Task 7 (branch `main` + rilis pertama, butuh approval git menulis) dan beberapa acceptance criteria belum terverifikasi/tercentang**. Sisa pekerjaan release **tidak dibawa ke task ini**. Catatan audit: `task/done/2026-11-15 - GitHub Release Automation & Linux Binary Distribution.md`.

## Scope (keputusan user, sesi 2026-11-15)
- Item yang dapat menu: **hanya baris file** di section Staged Changes & Changes. Section header, commit input, tombol commit/push, dan History (commit) **tidak** dapat menu.
- **Open File**: selalu membuka working-tree file di editor tab (bukan diff), apapun statusnya. Untuk status `D` (deleted) item dinonaktifkan.
- Isi menu: 3 aksi utama (Stage/Unstage, Open File, Copy Full Path) **+ Open Diff + Discard Changes**.
- Section Staged: `Discard Changes` **disembunyikan/dinonaktifkan** (discard bekerja pada working tree, bukan index); penggantinya `Unstage`.
- Tidak ada keybinding/shortcut baru → help modal (`?`), command palette, dan `internal/ui/menu.go` tidak berubah. Dokumen (PRD §4.2.B, §5 mouse matrix; README Git section) diperbarui sesuai Rule 11 semangat dokumentasi mouse.

## Ground Truth Findings
- **Context menu infra** (`internal/ui/contextmenu.go`): `ContextMenu.OpenCustom(x, y, items, screenW, screenH)` dengan `ContextMenuItem{ID, Label, Shortcut, Disabled}`; render overlay di atas UI; `HandleMouse` sudah menangani hover/click/close; dispatch via `handleContextMenuAction(action string)` di `internal/app/palette_context.go:30`.
- **Pembukaan menu explorer** (pola yang diikuti): `handleSidebarRightClick` (`mouse_sidebar.go:52`) → set `m.ContextMenuTabIdx = -1` → `openExplorerContextMenu(...)` (`palette_context.go:14`). Action handler explorer di `handleContextMenuAction` memakai pola clipboard (`clipboard.WriteAll` + `ShowToast`) untuk `explorer_copy_path` (`palette_context.go:210`).
- **Right-click saat ini di Git sidebar tidak ditangani**: `handleSidebarRightClick` hanya untuk `SidebarView == SidebarFiles`; right-click git jatuh ke `handleEditorRightClick` dan tidak memunculkan menu.
- **Hit-testing git** (`internal/git/view.go`): `ItemClickMap map[int]GitClickTarget` diisi saat `Render` dengan baris absolut (termasuk `ScrollOffset`); `GitClickTarget{Type, Index, Path}`; type item file = `ClickItemStaged` / `ClickItemChanges` (baris hint `Hunk x/y` memakai `ClickNone`; header seksi `ClickSectionStaged/Changes`; history `ClickItemHistory`). `HandleMouseClick` (`view.go:634`) sudah memakai `actualLine := gv.ScrollOffset + relY`.
- **Path git**: `GitFileStatus.Path` **relatif terhadap repo root** (`w.Filesystem.Root()` dari `OpenRepo`, `repo.go:82`; `GitRepo.RootPath` dipakai konsisten di seluruh `repo.go`). Kode existing `mouse_sidebar.go:235` memakai `filepath.Join(m.RootDir, targetFile.Path)` — **salah untuk workspace di subdirektori repo**; diperbaiki di task ini.
- **Klik kiri di item git** (`mouse_sidebar.go:201-241`): klik `[+]`/`[-]` stage/unstage; klik nama → untuk status `?`/`U` membuka file via `filepath.Join(m.RootDir, ...)`, selain itu `m.openDiff(path, isStaged)`. `openFile` = `internal/app/model_actions.go:313` (TabManager.OpenFile + toast); `openDiff` = `internal/app/model_diff.go:120` (guard file biner + batas ukuran sudah ada — aman untuk status `D`).
- **Discard flow** existing: klik `[x]` di Changes → `ConfirmDialog.Open(dialog.ConfirmDiscardGit, "Discard Changes", ..., targetFile.Path)` (`mouse_sidebar.go:224-231`); handler konfirmasi membuka via `internal/ui/dialog/confirm.go` (`ConfirmDiscardGit`, `ConfirmDiscardGitHunk`).
- **Model state**: `internal/app/model.go:58` `ContextMenuTabIdx int` (sentinel `-1` = bukan tab), di-init `-1` (`model.go:174`). Pola ini dipakai explorer; kita tambah field setara untuk git.
- **Dispatch action**: mouse → `mouse_modal.go:19`, keyboard → `keys_modal.go:29` — keduanya memanggil `handleContextMenuAction`.
- **Test pattern git**: `internal/app/git_binary_test.go` (pakai `NewModel()`, repo git asli di cwd); `tab_context_test.go` (pola test context menu; clipboard headless — `watcher_sidebar_preview_test.go:264` memanggil `handleContextMenuAction("copy")` tanpa assert clipboard).
- **File budget** (Rule 12, ≤500 baris non-test): `palette_context.go` 312 → ~385; `mouse_sidebar.go` 271 → ~330; `model.go` 240 → ~242. Aman.
- **Verifikasi wajib** akhir task: `go vet ./...`, `go test -v -race ./...`, `wc -l internal/app/*.go`.

## Objective & Approach
Satu pendekatan: **perluas infrastruktur context menu existing** (tidak membuat komponen baru). Right-click di baris item git → buka `ui.ContextMenu` dengan item adaptif; semua aksi me-reuse handler/fungsi yang sudah ada. Alternatif ditolak: modal git khusus (duplikasi UI, tidak konsisten), keybinding-only (tidak menjawab permintaan), palette-only (bukan permintaan).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Sidebar body (Git view) | Overlay context menu bisa overflow lebar terminal | `OpenCustom` sudah clamp X/Y ke batas layar (`contextmenu.go:62-77`) |
| Top Menu / Tab Bar / Status Bar | Tidak tersentuh | — |
| ANSI handling | Tidak ada slicing string ANSI; render menu dipegang `contextmenu.go` | reuse apa adanya |

## UI / Keymap Flow
Tidak ada keymap baru. Alur:

1. User klik kanan baris file (kolom `X < SidebarWidth`, `Y >= 3` dan `< Height-2`) di Git sidebar.
2. Baris terseleksi (StagedIdx/ChangesIdx di-set, `FocusArea` ikut, `CommitInput` blur) — mirror klik kiri.
3. Menu muncul (clamp ke layar):
   - Section Changes → `[ Stage ]` ; Section Staged → `[ Unstage ]`
   - `[ Open File ]` — buka file di editor tab; **disabled** bila status `D`
   - `[ Open Diff ]` — buka side-by-side diff tab
   - `[ Copy Full Path ]` — clipboard + toast "Full path disalin: …"
   - `[ Discard Changes ]` — hanya Changes; buka modal konfirmasi `ConfirmDiscardGit`
4. Klik kiri di luar menu / Esc → tutup (perilaku existing). Klik item → aksi jalan, menu tertutup.

## Tasks
- [ ] 1. `internal/app/model.go`: tambah field `ContextMenuGitPath string`, `ContextMenuGitStaged bool`, `ContextMenuGitStatus string` + init di constructor (dekat `ContextMenuTabIdx: -1`).
- [ ] 2. `internal/app/mouse_sidebar.go`: di `handleSidebarRightClick`, tambah cabang `SidebarView == SidebarGit` setelah blok Files: guard region (Y 3..Height-3), `relLine := mouse.Y - 3`, `actualLine := GitView.ScrollOffset + relLine`, lookup `GitView.ItemClickMap[actualLine]`; hanya `ClickItemStaged`/`ClickItemChanges` → set seleksi (StagedIdx/ChangesIdx, FocusArea, CommitInput.Blur), set state menu git, panggil `openGitContextMenu`. Target lain (header/history/hunk hint) → `return m, nil, false` (fall-through).
- [ ] 3. `internal/app/palette_context.go`: tambah `openGitContextMenu(absX, absY int, target git.GitClickTarget, fileStatus *git.GitFileStatus)` di samping `openExplorerContextMenu`: item adaptif Stage/Unstage sesuai section; `Open File` disabled bila `Status == "D"`; `Discard Changes` hanya untuk Changes; set `m.ContextMenuTabIdx = -1`.
- [ ] 4. `internal/app/palette_context.go`: tambah cases di `handleContextMenuAction`:
  - `git_stage` → `m.GitRepo.StageFile(path)` + `GitView.Refresh()` (guard `m.GitRepo != nil`)
  - `git_unstage` → `m.GitRepo.UnstageFile(path)` + `Refresh()`
  - `git_open_file` → `m.openFile(filepath.Join(m.GitRepo.RootPath, path))`; `m.Focus = FocusEditor`
  - `git_open_diff` → `m.openDiff(path, isStaged)`
  - `git_copy_path` → join `RootPath` + path, `filepath.Abs`, `clipboard.WriteAll`, toast (pola `explorer_copy_path`); error → toast merah
  - `git_discard` → `m.ConfirmDialog.Open(dialog.ConfirmDiscardGit, "Discard Changes", …, path)` dengan nama file di pesan (pola `mouse_sidebar.go:224`)
  - Reset state menu git (`ContextMenuGitPath = ""`) di akhir dispatch (atau di `ContextMenu.Close()` call sites yang ada — pastikan case baru membersihkan).
- [ ] 5. Boy Scout: `mouse_sidebar.go:235` — `filepath.Join(m.RootDir, …)` → `filepath.Join(m.GitRepo.RootPath, …)` (dengan nil-guard `m.GitRepo != nil`).
- [ ] 6. Verifikasi toggle: file status `D` di Changes → menu item `Open File` disabled (klik tidak membuka; toast/menu tetap konsisten). Status `A` (staged, untracked/added) → `Open Diff` membuka diff HEAD↔index seperti existing behavior.
- [ ] 7. Tests `internal/app/git_context_test.go` (pola `git_binary_test.go`/`tab_context_test.go`):
  - Right-click pada baris item Changes → menu `Active`, item pertama `git_stage`; seleksi ChangesIdx ter-set.
  - Right-click pada baris header section / History → menu tidak terbuka.
  - `handleContextMenuAction("git_stage")` pada repo temp → file masuk staged; `git_unstage` → sebaliknya.
  - `git_open_file` pada file normal → tab editor terbuka; pada status `D` → tidak membuka tab (item disabled).
  - `git_copy_path` tidak panic headless; path yang dipakai berbasis `RootPath`.
- [ ] 8. Docs: PRD §4.2.B tambah bullet "Klik kanan pada file (Staged/Changes): context menu — Stage/Unstage, Open File, Open Diff, Copy Full Path, Discard Changes"; §5 mouse matrix tambah baris `Git File Item | Right-Click`; README Git section (§5) tambah baris yang sama.
- [ ] 9. Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓, `wc -l internal/app/*.go` ≤ 500 ✓

## Manual Test Checklist
- [ ] Klik kanan file modified di Changes → menu: Stage, Open File, Open Diff, Copy Full Path, Discard Changes
- [ ] Klik kanan file di Staged → menu: Unstage, Open File, Open Diff, Copy Full Path (tanpa Discard)
- [ ] Stage dari menu → file pindah ke section Staged; Unstage dari menu → sebaliknya
- [ ] Open File pada file modified → file terbuka di editor tab (bukan diff)
- [ ] Open File pada file deleted (`D`) → item disabled, tidak membuka tab
- [ ] Copy Full Path → toast path absolut; paste di terminal = path valid (`ls <path>`)
- [ ] Discard Changes → modal konfirmasi muncul; Discard → perubahan hilang; Cancel → batal
- [ ] Klik kanan di section header / History / hunk-hint row → tidak ada menu
- [ ] Workspace dibuka dari **subdirektori** repo → Copy Full Path tetap mengarah ke file yang benar (RootPath)
- [ ] Resize terminal kecil (80x24) → menu tetap di dalam layar

## Acceptance Criteria
- [ ] Right-click di baris file Staged/Changes membuka context menu adaptif sesuai desain di atas
- [ ] Semua 5 aksi bekerja: Stage, Unstage, Open File (editor), Open Diff, Copy Full Path, Discard (Changes only)
- [ ] Status `D` menonaktifkan Open File
- [ ] `go vet ./...` bersih; `go test -race ./...` hijau; semua file `internal/app/*.go` ≤ 500 baris
- [ ] PRD §4.2.B, PRD §5, README Git section terbarui
- [ ] Tidak ada perubahan keymap / help modal / command palette

## Decisions
- Reuse `ui.ContextMenu` + `handleContextMenuAction` (pola explorer) — bukan komponen menu baru.
- Full path berbasis `GitRepo.RootPath`, bukan `m.RootDir` — Path git relatif ke repo root; `RootDir` bisa subdirektori. Sekalian perbaiki call site existing (Boy Scout).
- `Open File` selalu editor (keputusan user), dengan disabled untuk status `D` — file tidak ada di disk.
- `Discard Changes` hanya di Changes; di Staged diganti `Unstage` — discard = working tree, bukan index.
- Tidak ada shortcut baru → Rule 11 help-modal/palette tidak berlaku; hanya dokumen mouse matrix yang diperbarui.
- History commit tidak dapat menu (keputusan user).

## Blockers
- (kosong)

## Next
Tunggu approval user atas plan ini. Setelah approve: status → `in_progress`, eksekusi Task 1–9. Tanya user dulu: eksekusi langsung di sesi ini atau via delegasi?
