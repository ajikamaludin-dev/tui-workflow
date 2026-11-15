---
title: Configurable Tab/Indent Width (1-4) untuk Indent Rainbow & Indentasi Global
status: done
started: 2026-11-15
completed: 2026-11-15
branch: dev
priority: medium
---

# Configurable Tab/Indent Width (1-4)

## Objective
Membuat lebar indentasi/tab dapat dikonfigurasi 1, 2, 3, atau 4 spasi (default 4), berlaku global: indent rainbow, rendering tab char, Tab key, indent/outdent selection. Di-set via menu `[Setting]` (cycle) + command palette. Persist di `config.json`.

## Ground Truth Findings
- `internal/editor/buffer_view.go:36-37`: `const TabWidth = 4` — dipakai `runeVisualWidth` (lebar char `\t`), `RowColFromVisual`, dll.
- `internal/editor/indent_rainbow.go:33,51-61`: `CountLeadingSpaces` & `RenderIndentRune` pakai `TabWidth` untuk hitung level, offset, dan deteksi misaligned indent (`leadingSpaces % TabWidth != 0`).
- `internal/editor/buffer.go:633-735`: `IndentCurrentLine` (l.641), `IndentSelection` (l.665) insert `"    "` hardcoded; `OutdentSelection`/`OutdentCurrentLine` hapus maks 4 spasi hardcoded; cursor adjust `+= 4` / `-= 4`.
- `internal/app/keys_editor.go:221`: tombol Tab → `buf.InsertString("    ")` hardcoded.
- `internal/git/diff.go:574`: `strings.ReplaceAll(s, "\t", "    ")` untuk tampilan diff.
- `internal/config/config.go`: `Config` struct + `DefaultConfig()` (AutoSave, SoftWrap, FormatOnSave, SidebarWidth...) dan `LoadConfig()` dengan clamp/normalisasi (pola: `if cfg.SidebarWidth <= 0 { cfg.SidebarWidth = 30 }`).
- Startup: `internal/app/model.go` (`New()` ~l.154-189) load `cfg`, set `m.Config`, `m.SoftWrap`, `m.TabManager.SetSoftWrap(cfg.SoftWrap)` — belum ada setter untuk tab width.
- UI settings pattern: `internal/app/model_actions.go` `toggleSoftWrap()` → flip bool, apply, toast, `config.SaveConfig`. Palette: `internal/app/palette_actions.go` switch `case "setting.softwrap", "view.soft_wrap":`. Palette items: `internal/app/model_modals.go` (`{ID, Title, Category: "Settings", Shortcut}`). Menu: `internal/ui/menu.go` Setting section (~l.113-120) — **label static** (MenuBar dibangun sekali di model.go l.154), jadi label dinamis menampilkan nilai butuh rebuild menu; dipilih label statis.
- `ComputeVisualLines` (buffer_view.go l.117) dihitung ulang setiap render, tidak ada cache yang perlu di-invalidate saat width berubah.
- Tests: `internal/editor/indent_rainbow_test.go`, `internal/app/code_assistance_test.go`, `internal/app/features_test.go` — tidak ada assert TabWidth ≠ 4; default tetap 4 → tidak pecah.
- Tidak ada shortcut keyboard baru → aturan Shortcut Sync (AGENTS.md §4.11) tidak berlaku; tapi PRD.md + README.md perlu dicatat (fitur user-facing, lihat PRD l.212 & l.263-267 pola dokumentasi setting).

## Objective & Approach
Ubah `const TabWidth = 4` → `var TabWidth = 4` + `editor.SetTabWidth(w int)` (clamp 1-4). Semua pemakai existing otomatis ikut. Hardcoded `"    "` / `4` diganti `TabWidth`. Config baru `tab_width` (default 4, clamp saat load). Menu Setting: item cycle (1→2→3→4→1) + toast "Tab Width: N". Command palette: entry baru `setting.tab_width`. Tidak ada shortcut baru.

## Files Affected & Execution Steps
1. `internal/config/config.go` — tambah `TabWidth int json:"tab_width,omitempty"`; `DefaultConfig()` set 4; `LoadConfig()` clamp: `<1 || >4 → 4`.
2. `internal/editor/buffer_view.go` — `const TabWidth = 4` → `var TabWidth = 4`; tambah `func SetTabWidth(w int)` (clamp 1-4; no-op bila sama; set var). Update komentar.
3. `internal/editor/buffer.go` — `IndentCurrentLine` & `IndentSelection`: `strings.Repeat(" ", TabWidth)`, prealloc `len(line)+TabWidth`, `CursorCol += TabWidth` (dan anchor col), komentar "up to TabWidth". `OutdentSelection` & `OutdentCurrentLine`: loop `spaces < TabWidth`, adjust `SelectAnchorCol`/`CursorCol` dengan `-= TabWidth` (OutdentSelection pakai nilai variabel spaces sebenarnya sudah benar memakai loop, tapi penyesuaian cursor selection masih hardcode — samakan).
4. `internal/app/keys_editor.go:221` — `buf.InsertString(strings.Repeat(" ", editor.TabWidth))` (pastikan import `strings` & `editor` ada; bila belum, tambah).
5. `internal/git/diff.go:574` — ganti `"    "` dengan `strings.Repeat(" ", editor.TabWidth)` (cek import cycle `internal/git` → `internal/editor`; bila bermasalah, fallback biarkan 4 spasi + catat di Decisions).
6. `internal/app/model.go` — setelah cfg di-load di `New()`: `editor.SetTabWidth(cfg.TabWidth)` (mirror pola `m.TabManager.SetSoftWrap(cfg.SoftWrap)`).
7. `internal/app/palette_actions.go` — tambah `case "setting.tab_width":` (cycle 1→2→3→4, `editor.SetTabWidth`, `m.Config.TabWidth = ...`, `config.SaveConfig`, toast "Tab Width: N spasi").
8. `internal/app/model_modals.go` — tambah `{ID: "setting.tab_width", Title: "Tab Width (Cycle 1-4)", Category: "Settings", Shortcut: ""}` di dekat entry setting lainnya.
9. `internal/ui/menu.go` — Setting section: tambah `{Label: "Cycle Tab Width 1-4", Shortcut: "", ActionID: "setting.tab_width"}` (setelah Toggle Indent Rainbow).
10. `PRD.md` — update bagian settings/indent rainbow (l.263-267 area) + config key `tab_width`; `README.md` — tambah ke tabel setting bila ada.
11. Verifikasi: `wc -l internal/app/*.go` (<500/file), `go vet ./...`, `go test -v -race ./...`.

## Terminal & Layout Impact Analysis
No layout impact. Perubahan rendering lebar tab/indent kolom dalam editor pane; tidak menyentuh tinggi baris (Top Menu/Tab Bar/Status Bar tetap 1 baris). Lebar kolom teks berubah sesuai width → soft-wrap otomatis ikut karena `ComputeVisualLines` recompute per frame.

| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Editor Pane | Visual lines bergeser saat width berubah | Recompute per-frame; tidak ada cache stale |
| Editor Pane | Misaligned indent detection ikut width baru | Konsisten: semua pakai `TabWidth` var |
| Status Bar | Toast "Tab Width: N" | Pola toast existing |
| Top Menu / Tab Bar / Sidebar | Tidak tersentuh | — |

## UI / Keymap Flow
Tidak ada keybinding baru. Menu `[Setting]` → item baru "Cycle Tab Width 1-4" → klik/Enter → width cycle 1→2→3→4→1, toast `Tab Width: N spasi`. Command palette `Ctrl+P` → ketik "Tab Width" → entry "Tab Width (Cycle 1-4)". Persist otomatis ke `config.json` (`tab_width`).

## Tasks
- [x] `internal/config/config.go`: field `TabWidth`, default 4, clamp load
- [x] `internal/editor/buffer_view.go`: var TabWidth + SetTabWidth (clamp 1-4)
- [x] `internal/editor/buffer.go`: Indent/Outdent pakai TabWidth
- [x] `internal/app/keys_editor.go`: Tab key insert `TabWidth` spasi
- [x] `internal/git/diff.go`: FALLBACK — tetap 4 spasi (import cycle `internal/git` → `internal/editor` karena `internal/editor` mengimpor `internal/git`)
- [x] `internal/app/model.go`: SetTabWidth saat startup
- [x] `internal/app/palette_actions.go`: action `setting.tab_width` (cycle + toast + save)
- [x] `internal/app/model_modals.go`: palette item Settings
- [x] `internal/ui/menu.go`: item menu `[Setting]`
- [x] `PRD.md` + `README.md`: dokumentasi setting `tab_width`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓, `wc -l internal/app/*.go` ✓

## Manual Test Checklist
- [ ] Buka file dengan indent, `[Setting] ➔ Cycle Tab Width 1-4` → toast "Tab Width: 2 spasi", render indent rainbow level berubah, tab char selebar 2 kolom
- [ ] Cycle terus 2→3→4→1 (wrap) — tidak pernah nilai di luar 1-4
- [ ] Tab key insert spasi sesuai width aktif; Indent (Tab) / Outdent (Shift+Tab) selection menyesuaikan
- [ ] Restart app → width tersimpan (config.json `tab_width`)
- [ ] Command palette `Ctrl+P` → "Tab Width" → cycle jalan
- [ ] Indent misaligned (mis. 3 spasi saat width 4) tetap highlight error; width 3 → 3 spasi jadi aligned

## Acceptance Criteria
- [ ] `tab_width` 1/2/3/4 di config.json dihormati & default 4 bila absent/invalid
- [ ] Indent rainbow, tab rendering, Tab key, indent/outdent semuanya konsisten memakai width yang sama
- [ ] Cycle via menu Setting & command palette berfungsi + persist
- [ ] `go vet ./...` bersih, `go test -v -race ./...` PASS, `wc -l internal/app/*.go` semua <500

## Decisions
- `internal/git/diff.go` `truncateOrPad` tetap expand tab ke 4 spasi — `internal/editor` mengimpor `internal/git` (tab.go, manager.go), sehingga `internal/git` tidak boleh mengimpor `internal/editor` (import cycle). Display-only; tidak memengaruhi buffer.
- Width global (bukan indent-rainbow-only) — dipilih user; semua indentasi (tab char, Tab key, indent/outdent, rainbow) ikut satu setting
- `var TabWidth` + `SetTabWidth` di package editor — paling minimal; semua call site existing otomatis ikut tanpa ubah signature; trade-off: package-level mutable state (konsisten dengan gaya codebase saat ini)
- Label menu statis "Cycle Tab Width 1-4" — MenuBar dibangun sekali; nilai ditampilkan via toast setelah cycle
- Nama config key `tab_width` — konsisten dengan konsep TabWidth

## Blockers
- Import cycle potensial `internal/git` → `internal/editor` di `diff.go` — solusi: fallback biarkan 4 spasi untuk diff view bila cycle terjadi

## Improvement 1 (2026-11-15 — Tab Width Picker Modal)
Analisis fakta:
- User feedback (m00245): cycle terlalu lambat untuk memilih width; tujuan utama adalah indent rainbow (flutter width 2 tidak boleh memicu red block misaligned indent).
- User memilih via ask_user: scope tetap GLOBAL; UI diganti jadi modal picker seperti Branch Switcher tapi hanya 4 opsi.

Perubahan:
- `internal/ui/dialog/tabwidth.go` baru: `TabWidthPicker` (Active/Selected/Options []int{1,2,3,4}/Current/Width 44). Update: `↑/k/ctrl+p` & `↓/j/ctrl+n`, angka `1`-`4` langsung pick+apply, `Enter` apply, `Esc` batal (chosen=0). Mouse: klik luar = batal, wheel = pindah, klik row (relY 3..6) = pilih+apply. Render: rounded border accent, header `📐 Tab Width (1-4)`, row ✓ = current, selected = accent bg, hint `[↑/↓:Select  1-4:Pick  Enter:Apply  Esc:Cancel]`.
- `internal/app/model_actions.go`: helper `applyTabWidth(w int)` (clamp 1-4, `editor.SetTabWidth`, save `Config.TabWidth`, toast).
- `internal/app/model.go`: field `TabWidthPicker` + init `NewTabWidthPicker()`.
- `internal/app/model_modals.go`: `openTabWidthPicker()`; palette title `Tab Width (Cycle 1-4)` → `Tab Width...`.
- `internal/app/keys_modal.go`: handler setelah Branch Switcher (0.81) — apply bila closed && chosen >= 1.
- `internal/app/mouse_modal.go`: block setelah Branch Switcher mouse — startX/startY centering (`(Height-modalH)/3`), HandleMouse, apply bila chosen >= 1.
- `internal/app/view.go`: overlay chain `TabWidthPicker.Active` → `overlayTopCenter`.
- `internal/app/palette_actions.go`: `setting.tab_width` cycle dihapus → `m.openTabWidthPicker(); return nil`.
- `internal/ui/menu.go`: label menu → `Tab Width...`.
- `PRD.md`, `README.md`: dokumentasi cycle → picker.
- Verifikasi: `go build ./...` ✓, `go vet ./...` ✓ (0 error), `go test -race ./...` ✓ all packages ok, `wc -l internal/app/*.go` non-test max palette_actions.go 463 (<500). mouse_modal.go 363, keys_modal.go 175, internal/ui/dialog/tabwidth.go 223.

Manual Test Checklist (Improvement 1):
- [ ] `[Setting] ➔ Tab Width...` membuka modal picker; `↑/↓` berpindah, `Enter` apply, toast `Tab Width: N spasi`
- [ ] Menekan `1`/`2`/`3`/`4` langsung apply tanpa pindah-pindah
- [ ] Klik baris picker / scroll wheel berfungsi; klik di luar modal membatalkan
- [ ] `Esc` membatalkan tanpa mengubah width
- [ ] Flutter file width 2: tidak ada red block misaligned indent; Tab insert 2 spasi
- [ ] Restart aplikasi: width tersimpan via config `tab_width`

## Next
Menunggu verifikasi manual user (checkout daftar Manual Test Checklist terbaru), lalu task selesai → arsip ke `task/done/`.
