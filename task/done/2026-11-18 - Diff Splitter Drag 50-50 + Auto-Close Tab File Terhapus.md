---
title: Diff Splitter Drag 50/50 + Auto-Close Tab File Terhapus
status: done
started: 2026-11-18
completed: 2026-11-18
branch: dev
priority: medium
---

# Diff Splitter Drag 50/50 + Auto-Close Tab File Terhapus

## Objective
(1) Pengguna bisa menggeser (drag) pembatas vertikal pada Git Side-by-Side Diff Tab untuk mengubah rasio lebar kiri/kanan dari 50/50 secara bebas. (2) Ketika file pendukung sebuah tab sudah tidak ada di disk (terhapus secara internal maupun eksternal), tab tersebut ditutup otomatis — dengan pengaman untuk buffer dirty agar perubahan belum tersimpan tidak hilang tanpa konfirmasi.

## Ground Truth Findings
Terverifikasi dari kode & dokumen (bukan asumsi):

**Diff rendering**
- `internal/git/diff.go:451` — `DiffView` punya `Result`, `ScrollOffset`, `Hunks`, `ActiveHunk`, `vlCache`, `lastTextWidth`. Tidak ada field rasio.
- `internal/git/diff.go:680` — `DiffView.Render(width, height, isFocused)` meng-hardcode `colWidth := (width - 1) / 2` (pembatas `│` = 1 kolom) dan satu `textWidth` untuk **kedua** sisi.
- `internal/git/diff.go:498` — `visualCache(textWidth)` me-wrap LEFT dan RIGHT memakai **satu** lebar yang sama; cache di-key oleh `(Width, RowCount)`.
- `internal/git/diff.go:557,590,627` — `maxScrollOffset()`, `ActiveLine()`, `ensureActiveHunkVisible()` bergantung pada `dv.textWidth()` (= `lastTextWidth` yang di-set saat `Render`). Jika lebar kiri≠kanan, keempat tempat ini harus memakai pasangan lebar.
- `internal/git/diff.go:470-495` — `wrapDiffText` sudah ANSI-aware via `ansi.StringWidth`; `Render` sudah memakai `ansi.Truncate` + padding `ansi.StringWidth`. Marker `▌` / `│` tidak boleh dipotong byte (Rule ANSI).

**Pola drag yang sudah ada (direuse, tidak ditulis ulang)**
- `internal/app/model.go:68` — `ResizingSidebar bool`; `internal/app/model_actions.go:49` — `setSidebarWidth()` dengan clamp.
- `internal/app/mouse_sidebar.go:10-55` — pola hover `col-resize` → click start → motion → release; `internal/app/update_mouse.go:19` & `internal/app/mouse_release.go:32` adalah dispatcher/routing-nya.
- `internal/app/model_actions.go:70` — `setMousePointer(shape)` (escape sequence `\x1b]22;...`).

**Geometri layout**
- `internal/app/view.go:30-43` — Baris 0 = menu, Baris 1 = tab bar, body mulai Baris 2, Baris terakhir = status bar.
- `internal/app/view.go:286-294` — `renderEditor(width, height)` memanggil `DiffView.Render(width, height-1, ...)` dengan `width = m.Width - (SidebarVisible ? SidebarWidth : 0)`.
- `editorStartX` dihitung di `internal/app/update_mouse.go:29-32` (setelah dispatcher drag lama).

**File watcher & tab lifecycle**
- `internal/app/file_watcher.go:17-39` — `checkWatchedTab`: pada `os.IsNotExist` hanya menampilkan toast + menghapus baseline (`WatchedHash`, `WatchedSize`, `LastModTime`), **tab tetap terbuka selamanya**. Dipanggil tiap 1 detik (`internal/app/commands.go:332`, `internal/app/update.go:100`).
- `internal/app/file_watcher.go:41-76` — file muncul lagi / berubah → clean tab di-`ReloadFromDisk`; dirty tab → `ConfirmExternalChange`.
- `internal/editor/tab.go:17-39` — `Tab` punya `Path`, `Buffer`, `DiffView`, `IsPinned`, `WatchedHash/Sz`; tidak ada flag "deleted".
- `internal/editor/manager.go:216` — `CloseTab(idx)` (tidak mempedulikan pin; proteksi pin ada di aksi bulk).
- `internal/editor/manager.go:131-159` — `OpenDiff` **mengganti** `t.DiffView` dengan `git.NewDiffView(...)` saat membuka ulang diff yang sama → rasio harus dipindahkan agar tidak reset.
- `internal/app/dialog_handlers.go:169-205` — `ConfirmUnsavedChanges` dengan `ClosingTabIdx >= 0` sudah menyediakan Save / Discard / Cancel. Bisa direuse untuk kasus dirty+terhapus (Save akan merecreate file).
- `internal/app/dialog_handlers.go:157-167` — delete via explorer (`ConfirmDelete`) menghapus file; watcher akan menangkap dalam ≤1 detik → tidak perlu duplikasi logika close di handler ini.

**Batasan & aturan**
- `internal/app/resize.go:10-11` — minimal terminal 80×24; diff Render sudah return "" untuk `width < 20 || height < 2`.
- `internal/git/diff_test.go:182` — `dv.visualCache(10)` dipanggil langsung (test harus ikut diubah saat signature berubah).
- Rule 11 (shortcut sync): fitur ini **mouse-only**, tidak menambah shortcut keyboard → tidak ada perubahan help modal/menu/command palette. Hanya PRD + README yang perlu sinkron.
- Rule 12: `wc -l internal/app/*.go` maks 500/file; file baru `mouse_diff.go` jauh di bawah batas.

## Objective & Approach
- **Rasio sebagai milik `DiffView`**: tambah `SplitRatio float64` (default 0.5) + `ColumnWidths(width)` + `SetSplitRatio(r)` di `internal/git/diff.go`. Semua perhitungan geometri (`Render`, `maxScrollOffset`, `ActiveLine`, hunk nav) memakai satu helper yang sama agar scroll tidak "loncat".
- **Cache wrap dua sisi**: `visualCache(leftW, rightW)` — LEFT dibungkus dengan lebar kiri, RIGHT dengan lebar kanan; jumlah baris visual per row tetap `max(leftSegs, rightSegs)`. Kunci cache = `(leftW, rightW, rowCount)`.
- **Drag mengikuti pola sidebar resize**: hit-test kolom pembatas yang eksak + Baris 2..Height-2 (Rule hit-testing presisi), pointer `col-resize` saat hover, click → `ResizingDiff`, motion → update rasio, release → clear flag. Tidak ada baris baru → zero flicker / single-line guarantee terjaga.
- **Auto-close aman**: watcher mengumpulkan tab clean yang file-nya hilang lalu menutupnya **setelah** loop selesai (hindari mutasi slice saat iterasi). Tab dirty ditandai `DeletedFromDisk`, dialog `ConfirmUnsavedChanges` dibuka **sekali** (Save merecreate file / Discard menutup / Cancel membatalkan). Tab pinned dilewati (kontrak pin dilindungi dari penutupan otomatis) dengan toast peringatan.

## Files Affected & Execution Steps
1. `internal/git/diff.go` — tambah `SplitRatio`, helper `ColumnWidths(width) (int,int)`, `SetSplitRatio`; ubah `diffVisualCache` ke dua lebar + kunci cache; ubah `visualCache`, `textWidths`, `maxScrollOffset`, `ActiveLine`, `ensureActiveHunkVisible`, `Render` (header/gutter/content/empty memakai lebar masing-masing). `NewDiffView` default 0.5.
2. `internal/editor/manager.go` — di `OpenDiff`, pindahkan `SplitRatio` lama ke `DiffView` baru saat reuse tab.
3. `internal/editor/tab.go` — tambah `DeletedFromDisk bool` (didokumentasikan).
4. `internal/app/model.go` — tambah `ResizingDiff bool`.
5. `internal/app/mouse_diff.go` (BARU) — `diffSeparatorX()`, `handleDiffResizeStart`, `handleDiffResizing`, `handleReleaseDiffResize`, helper clamp; ≤200 baris.
6. `internal/app/update_mouse.go` — naikkan hitung `editorStartX` sebelum dispatcher drag; sisipkan intercept start+motion diff resize setelah tab-drag; tambah cabang pointer `col-resize` di separator.
7. `internal/app/mouse_release.go` — panggil `handleReleaseDiffResize` pada release klik kiri.
8. `internal/app/file_watcher.go` — refactor `checkWatchedFiles`/`checkWatchedTab`: kumpulkan index tab clean terhapus → tutup setelah loop; dirty → flag + dialog sekali; reset flag saat file muncul lagi; toast; `requestSessionPersist()` setelah perubahan.
9. `internal/git/diff_test.go` — update `visualCache(10)` → `visualCache(10,10)`; test baru: rasio 50/50 default, `ColumnWidths` clamp, lebar kiri≠kanan dirender pas width & baris visual tetap sinkron.
10. `internal/app/diff_resize_test.go` (BARU) — test hit-test separator, drag mengubah rasio, clamp, pointer shape.
11. `internal/app/file_watcher_test.go` — test auto-close clean tab saat file dihapus eksternal; test dirty tab tidak langsung tertutup + dialog muncul sekali; test file muncul lagi mereset flag.
12. `PRD.md` — §4.3.D tambah splitter drag + auto-close; §Revision History entri v0.4.1 (2026-11-18).
13. `README.md` — §5 Split Diff & §7 Ergonomi Mouse + catatan auto-close.
14. `task/CURRENT.md` — status `in_progress` saat approve; checklist dicek bertahap.

## Terminal & Layout Impact
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Kolom 2 (Diff body) | Rasio ekstrem membuat satu sisi lebih sempit dari gutter+teks | `ColumnWidths` clamp ke minimum 10 kolom/sisi; fallback 50/50 jika total < 2×min+1 |
| Baris 0/1/terakhir | Drag atau teks meluap → scroll offscreen | Tidak ada baris baru; total lebar `left+1+right == width` dipertahankan; `ansi.Truncate` |
| ANSI escape | Slicing mentah saat mark `│`/`▌` | Tetap pakai `ansi.StringWidth`/`ansi.Truncate`; tidak ada byte slicing |
| Mouse pointer | `col-resize` menempel setelah drag selesai | Release clear flag; if-chain pointer di-update setiap event |
| Status bar | `Ln` bisa salah saat lebar berubah | `ActiveLine`/scroll memakai `textWidths` yang di-set `Render` (sumber tunggal) |

## UI / Keymap Flow
1. Buka Git Side-by-Side Diff Tab (klik file di sidebar Git / `Enter`).
2. Arahkan mouse ke garis `│` di tengah diff → kursor berubah `col-resize`.
3. Klik kiri + tahan + geser ke kiri/kanan → pembatas mengikuti mouse; teks di-wrap ulang per sisi; scroll sinkron tetap benar.
4. Lepas mouse → rasio tersimpan pada tab diff (bertahan saat hunk di-stage/refresh; ikut dipindahkan saat diff dibuka ulang).
5. Hapus file yang tab-nya sedang terbuka (dari explorer, `rm`, editor lain):
   - Buffer **clean** → tab langsung tertutup, toast "File 'x' dihapus — tab ditutup", session di-persist.
   - Buffer **dirty** → toast peringatan + dialog `Save / Discard / Cancel` (sekali). Save = recreate file + tutup; Discard = tutup tanpa simpan; Cancel = tab tetap terbuka.
   - Tab **pinned** → tidak ditutup otomatis, hanya toast.
6. File muncul lagi → flag di-reset, konten di-reload seperti biasa.

## Tasks
- [x] Ground truth: verifikasi diff/cache/geometry/watcher (SELESAI — lihat Ground Truth Findings)
- [x] Implementasi `DiffView` rasio + `ColumnWidths` + cache dua sisi
- [x] Preserve rasio di `manager.OpenDiff`; field `DeletedFromDisk` & `ResizingDiff`
- [x] Handler drag diff (`mouse_diff.go`) + wiring di `update_mouse.go` / `mouse_release.go`
- [x] Auto-close tab terhapus di `file_watcher.go` (clean / dirty / pinned)
- [x] Update + test baru (`diff_test.go`, `mouse_diff_test.go`, `file_watcher_test.go`)
- [x] Sinkron dokumen: `PRD.md` (§D1 + Revision History v0.4.1) & `README.md`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓ (20 paket PASS), `wc -l internal/app/*.go` ≤500 ✓ (max 481)

## Manual Test Checklist
- [x] Buka diff tab, hover/klik-drag garis `│` → rasio berubah halus, tanpa flicker, tanpa baris tambahan.
- [x] Drag sampai ekstrem → sisi sempit berhenti di batas aman (gutter + teks masih terbaca).
- [x] Scroll wheel & `n`/`]` pada diff setelah drag → tidak ada loncatan/offset salah.
- [x] Stage hunk lalu buka ulang diff → rasio tidak kembali 50/50.
- [x] Hapus file clean yang tab-nya terbuka (dari terminal) → dalam ≤1 detik tab tertutup otomatis + toast.
- [x] Edit file lalu hapus dari terminal → tab tetap ada, dialog Save/Discard/Cancel muncul sekali; pilih Save → file recreates & tab tertutup.
- [x] Pin tab lalu hapus filenya → tab tidak tertutup, muncul toast.
- [x] Buat ulang file yang dihapus → tab normal kembali (atau bisa dibuka ulang tanpa warning lama).

## Acceptance Criteria
- [x] Garis pembatas diff bisa di-drag; rasio tersimpan per tab dan tidak reset oleh refresh hunk.
- [x] Render diff tetap tepat `width` kolom, tidak menambah baris, ANSI bersih.
- [x] Scroll & navigasi hunk konsisten pada rasio apa pun (terbukti oleh test).
- [x] Tab clean untuk file terhapus tertutup otomatis ≤1 detik; dirty tidak hilang tanpa konfirmasi.
- [x] `go vet ./...` dan `go test -v -race ./...` hijau; budget `internal/app` ≤500 baris/file.

## Decisions
- **Rasio disimpan di `DiffView`, bukan `Model`** — geometry milik view; tiap tab punya rasio independen; refresh hunk tidak mengubahnya. Trade-off: `OpenDiff` perlu menyalin rasio saat mengganti `DiffView` (3 baris).
- **Dirty + file hilang memakai `ConfirmUnsavedChanges` yang sudah ada** — direuse daripada dialog baru; Save merecreate file lewat `SaveTab`. Trade-off: teks dialog generik, cukup untuk kasus ini.
- **Mouse-only (tanpa shortcut baru)** — menghindari churn Rule 11 (help/menu/palette). Trade-off: tidak ada reset keyboard ke 50/50.
- **Tab pinned dikecualikan dari auto-close** — mematuhi kontrak pin PRD. Trade-off: tab pinned user yang terhapus tetap terbuka (ada toast).
- **Auto-close lewat watcher (1s), bukan duplikat di `ConfirmDelete`** — satu sumber kebenaran; delete internal & eksternal perlakuan sama.

## Blockers
- Tidak ada.

## Next
Task selesai & diarsipkan ke `task/done/2026-11-18 - Diff Splitter Drag 50-50 + Auto-Close Tab File Terhapus.md` dengan verifikasi manual oleh user dinyatakan aman.
