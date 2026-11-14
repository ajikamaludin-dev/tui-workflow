---
title: Rekomendasi Review — Bubble Tea v2 Migration, PRD v0.2.0 & Refactor internal/app
status: done
started: 2026-11-14
completed: 2026-11-15
branch: dev
priority: high
---

# Rekomendasi Review — Bubble Tea v2 Migration, PRD v0.2.0 & Refactor `internal/app`

## Objective
Menerjemahkan `review.md` menjadi aksi konkret: **(A) upgrade penuh stack charm.land ke v2** (bubbletea, lipgloss, bubbles, glamour, x/ansi latest), (B) refactor bertahap `internal/app` agar tidak menjadi God Object, (C) hardening terminal/git & LSP documentation, (D) sinkronisasi PRD v0.2.0 + docs (termasuk TODO.md dibersihkan) — ditulis terakhir agar sekali tulis mencerminkan realita pasca-v2.

> **Catatan status (2026-11-14):** Track TODO dari versi plan sebelumnya **dibuang atas konfirmasi user** — user menyatakan semua item TODO.md sudah diperbaiki. Konsekuensi: `TODO.md` dicentang & dikosongkan di Track D, tanpa verifikasi ulang di plan ini. (Catatan netral: pada saat penyusunan plan, kode/git di repo ini belum menunjukkan semua perbaikan tersebut; plan ini tidak mengklaim verifikasi, hanya mencatat bahwa user yang mengonfirmasi.)
>
> **Scope change (2026-11-14, user):** Bubble Tea v2 **tidak lagi hanya dicatat** — user meminta upgrade-nya ditangani di plan ini. Keputusan baru: full-stack v2, migration sebagai Track pertama.

## Ground Truth Findings (Phase 0 — terverifikasi 2026-11-14)

> Catatan: isi `review.md` hanya 14 baris (3 catatan kritis + 3 penilaian). Tidak ada temuan lain di luar itu.

### Ecosystem v2 (riset web, 2026-11-14)
- Modul baru vanity domain: `charm.land/bubbletea/v2` (v2.x), `charm.land/lipgloss/v2` (v2.1.0, rilis 2026-11-09), `charm.land/bubbles/v2` (v2.2.1), `charm.land/glamour/v2` (v2.0.1). **`x/ansi` TIDAK pindah** — tetap `github.com/charmbracelet/x/ansi` (pakai latest).
- Sumber kebenaran API: [UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/bubbletea/raw/refs/heads/main/UPGRADE_GUIDE_V2.md) (migrasi kursor/key/mouse/view) + `UPGRADE_GUIDE_V2.md` di repo bubbles & lipgloss.
- Perubahan breaking utama: `View() string` → `View() tea.View`; `tea.KeyMsg` (struct) → `tea.KeyPressMsg`; `msg.Type/Runes/Alt` → `msg.Code/Text/Mod`; space `" "` → `"space"`; mouse jadi interface (`MouseClickMsg`/`MouseReleaseMsg`/`MouseWheelMsg`/`MouseMotionMsg`, akses `msg.Mouse().X/Y`), konstanta button `tea.MouseButtonLeft` → `tea.MouseLeft` dst.; program options (`WithAltScreen`, `WithMouseCellMotion`, …) & toggle commands (`EnterAltScreen`, dst.) **dihapus** → jadi field deklaratif di `tea.View` (`v.AltScreen`, `v.MouseMode`, `v.ReportFocus`, dst.); `tea.WindowSize()` → `tea.RequestWindowSize`; `tea.Sequentially` → `tea.Sequence` (di repo tidak dipakai).
- `tea.WindowSizeMsg` tetap ada (dipakai test & `model.go:483`).
- Chroma (`alecthomas/chroma/v2`) & go-git **tidak terpengaruh**.

### Dampak kode repo (grep 2026-11-14)
- File mengimpor bubbletea: **57**; lipgloss: **28**; bubbles: **11** (`textinput` ×10, `textarea` ×1); glamour: **1** (`internal/markdown/renderer.go`).
- `View() string` hanya 1: `internal/app/view.go:15` (`func (m Model) View() string`).
- `tea.KeyMsg`: 154 referensi (kode + test); `tea.KeyRunes`: 32 (banyak di test: `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(...), Alt: bool}`); `msg.Type`: 7; `msg.Alt`: 9; `msg.Runes`: beberapa di `internal/terminal/terminal.go:389` (raw key forward ke PTY).
- Key constants dipakai: `tea.KeyEnter` (29), `tea.KeyF*` (17 — substring match, perlu inventaris), `tea.KeyEsc` (14), `tea.KeyTab` (13), `tea.KeyDown/Up/Right/Left`, `tea.KeyBackspace`, `tea.KeyShiftTab`, `tea.KeyCtrl*` (banyak), `tea.KeySpace` (2), `tea.KeyPgUp/PgDown/Insert/Home/End/Delete/Null/CtrlBackslash` dst. — perlu inventaris persis saat eksekusi.
- Mouse: `tea.MouseMsg` 72 referensi; `MouseAction/MouseButton/MouseEvent` 211 referensi; hit-testing manual (menu bar, tab bar, sidebar, tree, editor tabs, stage buttons, modal, terminal pane) ada di `internal/app/update_mouse.go` (1047 baris), `internal/ui/` (menu/dialog), `internal/terminal/` (forward ke PTY).
- Program options hanya di `cmd/tui-workflow/main.go:47-48` (`WithAltScreen`, `WithMouseCellMotion`). Tidak ada `WithReportFocus`/`WithoutBracketedPaste`/`SetWindowTitle` aktif.
- `lipgloss.AdaptiveColor`/`CompleteColor`/`ColorProfile` **tidak dipakai** — risiko v2 color API kecil. API Style/Color v2 hampir identik untuk pemakaian repo ini (NewStyle, Width/Height, JoinHorizontal/Vertical, MaxWidth/MaxHeight(1), Border, AdaptiveColor tidak dipakai).
- go.sum memuat lipgloss `v1.0.0` **dan** pseudo-version (inkonsistensi); go.mod memuat `lipgloss v1.1.1-0.20250404...`. Stack v2 akan membersihkan ini.
- `gofmt`/linter: tidak ada config khusus; verifikasi standar = `go vet` + `go test -race`.
- Test yang meng-instantiate `tea.KeyMsg{...}`: `internal/ui/menu_test.go`, `internal/ui/dialog/find_test.go`, `internal/terminal/terminal.go`+test, `internal/app/terminal_tab_test.go`, dst. — harus ditulis ulang ke `tea.KeyPressMsg`.
- Commit terbaru `3c2373f` (mouse reporting fix) dan `52b3314` (terminal process detection, vertical centering) — area mouse paling sensitif, prioritas smoke test pasca-migration.
- `task/CURRENT.md`: working tree bersih; branch belum diset; **tidak ada commit otomatis** — versioning di tangan user (aturan repo: no git writes tanpa approval).

## Objective & Approach
Urutan: **kode v2 dulu (A) → refactor (B) → hardening (C) → docs terakhir (D)** — docs sekali tulis, langsung mencerminkan realita v2. Migration = kerja mekanis besar tapi harus teliti di 3 titik sensitif: `View()` declarative fields, key message overhaul, dan mouse interface + hit-testing. Semua perubahan mengikuti aturan keras repo (Rule 1–3, 11) — tidak ada perubahan arsitektur Elm, tidak ada perubahan layout.

## Track A — Bubble Tea v2 Full-Stack Migration (kode dulu)

- [x] **A1. Baseline hijau:** `go vet ./... && go test -race ./...` PASS dulu — simpan sebagai pembanding. (Tanpa git writes.)
- [x] **A2. Dependency bump:** `go get charm.land/bubbletea/v2 charm.land/lipgloss/v2 charm.land/bubbles/v2 charm.land/glamour/v2 github.com/charmbracelet/x/ansi@latest` → `go mod tidy`. Cek tidak ada `github.com/charmbracelet/bubbletea` / `charmbracelet/lipgloss` v1 tersisa di `go.mod` (`go mod graph | grep -E "charmbracelet/(bubbletea|lipgloss|bubbles|glamour) "` kosong; `charm.land/*/v2` muncul).
- [x] **A3. Import path sweep (mekanis):** `charmbracelet/bubbletea` → `charm.land/bubbletea/v2` (57 file), `charmbracelet/lipgloss` → `charm.land/lipgloss/v2` (28 file), `charmbracelet/bubbles/...` → `charm.land/bubbles/v2/...` (11 file), `charmbracelet/glamour` → `charm.land/glamour/v2` (1 file). x/ansi tetap path lama (bump latest).
- [x] **A4. `View()` → `tea.View`:** `internal/app/view.go:15` — kembalikan `tea.NewView(content)` (atau `tea.View` + `SetContent`); set field deklaratif: `v.AltScreen = true`, `v.MouseMode = tea.MouseModeCellMotion`, `v.ReportFocus` sesuai perilaku v1 (cek apakah v1 mengaktifkan focus events — jika ya set `true`).
- [x] **A5. `cmd/tui-workflow/main.go`:** hapus `tea.WithAltScreen()` & `tea.WithMouseCellMotion()` dari `tea.NewProgram(...)` (sudah digantikan field View); pertahankan urutan init lain; pastikan teardown via `tea.Quit` (Rule 3) — `p.Run()` sudah dipakai, tidak ada `Start()`.
- [x] **A6. Key messages (per-domain, bukan blind replace):**
  - `case tea.KeyMsg:` → `case tea.KeyPressMsg:` (di `Update()` handlers: `internal/app/update_keys.go`, `model.go`, `internal/ui/*`, `internal/terminal/*`, dialogs).
  - `msg.Type` → `msg.Code`; `msg.Runes` (`[]rune`) → `msg.Text` (`string`) — konversi: perbandingan rune tunggal `len(msg.Text) == 1 && rune(msg.Text[0]) == 'x'` atau `msg.Text == "x"`; konstruksi test `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}` → `tea.KeyPressMsg{Code: 'a', Text: "a"}` (+`Mod` untuk Alt: `tea.ModAlt`).
  - `msg.Alt` → `msg.Mod.Contains(tea.ModAlt)`.
  - `case " "` (space) → `case "space"`; `tea.KeySpace` konstanta → string/rune sesuai konteks switch.
  - Inventaris & adaptasi semua `tea.KeyEnter/Esc/Tab/F*/Backspace/ShiftTab/PgUp/PgDown/Insert/Home/End/Delete/Ctrl*/Null/CtrlBackslash` — jika konstanta v1 hilang, pakai `msg.String()` matching (sudah jadi pola dominan repo, e.g. `update_keys.go:614`) atau `msg.Code` + `msg.Mod`.
  - **`internal/terminal/terminal.go:389`** (raw forward ke PTY) — paling sensitif: pastikan byte streaming ke PTY tetap faithful: printable chars via `msg.Text`, control bytes via `msg.Code`+`Mod` → mapping ke byte ASCII (Ctrl+C → 0x03 dst.), Backspace (0x7f), Escape sequences, Tab, Enter, dan Alt prefix (ESC + key). Ini satu-satunya tempat perilaku bisa berubah halus — tulis/ jalankan `terminal_test.go`.
- [x] **A7. Mouse messages (paling berisiko, Rule: hit-testing manual):**
  - Interface: `case tea.MouseMsg:` tetap valid, tapi akses data via `m := msg.Mouse()` (X/Y/Mod/Button).
  - Split per tipe: `tea.MouseClickMsg` (press), `tea.MouseReleaseMsg`, `tea.MouseWheelMsg`, `tea.MouseMotionMsg` — rewrite switch di `internal/app/update_mouse.go` + jalur mouse di `internal/ui/` (menu bar hit-test, dialog modals) + forward mouse ke PTY di `internal/terminal/` (jika terminal tab aktif → forward raw mouse report ke PTY dengan encoding SGR 1006 sesuai `MouseMode`).
  - Rename konstanta: `tea.MouseButtonLeft/Right/Middle/Wheel*` → `tea.MouseLeft/Right/Middle/WheelUp/WheelDown/WheelLeft/WheelRight`.
  - **Hit-testing logic (relX/relY vs posisi komponen) TIDAK berubah** — hanya accessor & tipe message; verifikasi setiap handler: menu bar, sidebar tabs (`[ 1:Files | 2:Git ]`), tree nodes, editor tabs + tombol `[×]`, git stage buttons, tab drag-drop, modal dialogs, terminal pane, context menu.
  - Fokus: klik terminal tab vs hijack oleh app handler (regression `3c2373f`) — pastikan terminal tetap menerima raw mouse saat focused.
- [x] **A8. Tests rewrite:** semua `tea.KeyMsg{Type: tea.KeyRunes, ...}` di `menu_test.go`, `find_test.go`, `terminal_test.go`, `terminal_tab_test.go`, `e2e_test.go`, `features_test.go`, `git_binary_test.go`, `modal_mouse_test.go`, `tab_context_test.go`, `watcher_sidebar_preview_test.go` → `tea.KeyPressMsg`; klik/scroll konstruksi `tea.MouseMsg{...}` → `tea.MouseClickMsg{Mouse: tea.Mouse{...}}` dst.; `tea.WindowSizeMsg` tetap.
- [x] **A9. Lipgloss v2 fallout:** perbaiki compile error minimal (Style API hampir sama); pastikan `lipgloss.Width/Height/JoinHorizontal/JoinVertical/MaxWidth/MaxHeight` dipakai sesuai v2; tidak ada penggunaan fitur v1 yang dihapus (cek docs v2 bila compile error muncul).
- [x] **A10. Bubbles v2 fallout:** ikuti `UPGRADE_GUIDE_V2.md` di repo bubbles (textinput: fokus/placeholder API; textarea); hanya file `quickopen.go`, `input.go`, `globalsearch.go`, `palette.go`, `filepicker.go`, `aisettings.go`, `branch.go`, `findbar.go`, `switcher.go`, `internal/search/view.go`, `internal/git/view.go`.
- [x] **A11. Glamour v2:** update import di `internal/markdown/renderer.go`; SHA-256 cache (konten + lebar viewport) tidak berubah; hash behavior diverifikasi oleh test yang ada.
- [x] **A12. Verifikasi mekanis:** `go vet ./...` bersih; `go test -v -race ./...` hijau; `grep -r "charmbracelet/bubbletea\|charmbracelet/lipgloss\|charmbracelet/bubbles\|charmbracelet/glamour" --include=*.go .` kosong (selain x/ansi).
- [ ] **A13. Smoke fungsional (Rule 1–3, Rule 11):** alt-screen tidak tersisa setelah quit (teardown bersih), Baris 0 (Top Menu) tidak pernah terdorong offscreen (MaxHeight(1) masih efektif), zero flicker, klik menu/tab/sidebar/tree menghasilkan aksi yang sama seperti v1, scroll wheel di preview markdown & terminal, terminal tab mouse fix (`3c2373f`) tetap bekerja, `Ctrl+Shift+\` / `Ctrl+T` buka terminal (interaksi dengan catatan TODO user — jangan diubah).

## Track B — Refactor `internal/app` bertahap (behavior-neutral)

*(Setelah v2 hijau; file sizes bisa bergeser sedikit — ukur ulang saat eksekusi.)*

Urutan aman: ekstrak file dulu (pure move), baru perbaiki coupling. Tidak ada perubahan logika/render.

- [x] **B1.** Tulis peta domain & budget di `docs/architecture.md` (atau section baru): tabel file → domain → target maks baris (usul: **≤500 baris per file**; `model.go` dikecualikan dengan batas 1200 → diturunkan bertahap).
- [x] **B2.** Split `update_keys.go` → `keys_global.go`, `keys_editor.go`, `keys_search.go`, `keys_git.go`, `keys_terminal.go`, `keys_modal.go`. Pertahankan signature `handleKey` publik yang dipanggil dari `Update()`.
- [x] **B3.** Split `update_mouse.go` → `mouse_menu.go`, `mouse_tabbar.go`, `mouse_sidebar.go`, `mouse_editor.go`, `mouse_modal.go`, `mouse_terminal.go`.
- [x] **B4.** Split `palette_actions.go` → `palette_actions_git.go`, `palette_actions_terminal.go`, `palette_actions_file.go`, `palette_actions_setting.go`, `palette_actions_ai.go`.
- [x] **B5.** Split `view.go` bila >500 baris non-layout; `model.go` terakhir (pisahkan inisialisasi modal ke `model_modals.go`).
- [x] **B6.** **AGENTS.md Rule 12 (usulan baru): "Internal App Package Budget"** — wajib: (1) file di `internal/app/` ≤500 baris, (2) handler per-domain di file terpisah mengikuti pola `dialog_handlers.go`/`navigation.go`/`resize.go`, (3) dilarang menambah `case` baru ke switch raksasa tanpa ekstraksi, (4) PR/commit yang melewati budget harus memecah file terlebih dahulu, (5) cek `wc -l internal/app/*.go` di verification suite.
- [x] **B7.** Verifikasi: `go vet ./...` bersih, `go test -race ./...` hijau.

## Track C — Hardening terminal/git & LSP documentation

- [x] **C1. Terminal & git = pembeda utama → stabilisasi.** Audit semua jalur close terminal & quit aplikasi agar konsisten memanggil `HasRunningProcess` (tab `[×]`, `Ctrl+W` di terminal tab, `Ctrl+Q`, menu `File ➔ Exit`, palette `app.exit`, `Alt+W`/`Ctrl+Shift+W`). Audit jalur: `dialog_handlers.go`, `model.go`, `palette_actions.go`, `update_keys.go` (pasca-split: `keys_*.go`) — catat jalur yang belum dijaga, tambahkan jika ada yang lolos. Tambah/verifikasi test `HasRunningProcess` (unix) di `internal/terminal/`; jalankan test git hunk (stage/unstage/discard) & pastikan hijau.
- [x] **C2. LSP hand-rolled — dokumentasikan, jangan rewrite.** Tambah checklist audit di `docs/lsp-audit-checklist.md`: position encoding (utf-16), incremental document sync, capability negotiation, timeout/crash recovery. Status: "v1-acceptable, known debt".
- [ ] ~~C3. Bubble Tea v2 catat saja~~ → **digantikan Track A** (diupgrade penuh di plan ini).
- [x] **C3-baru.** Pastikan CLAUDE/AGENTS/PRD tidak lagi menyebut bubbletea v1 sebagai stack — diserahkan ke Track D; di C cukup pastikan kode v2 tidak menyisakan TODO migrasi terpendam (grep `v1\|TODO.*migrat` di kode).

## Track D — PRD v0.2.0 & Docs Sync (terakhir)

- [x] **D1.** Header PRD: `Version: v0.2.0`, `Status: Approved`; section **Revision History** (v0.1.1 → v0.2.0: rekap fitur yang sudah masuk realita — AI commit/LLM config, PTY terminal, multi-cursor, symbol index, formatter, session persistence, tab pinning/drag-drop, word jump — **dan migrasi stack ke charm.land v2**).
- [x] **D2.** Tech stack table PRD diupdate ke versi v2 aktual (dari `go.mod` pasca-A2): `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, `charm.land/glamour/v2`, `chroma/v2 v2.27+`, `go-git/v5`. Hapus klaim `bubbles v1.0+`/`lipgloss v1.1+` yang usang.
- [x] **D3.** Tambah section **Known Risks & Technical Debt**: (1) LSP hand-rolled (audit checklist → C2), (2) `internal/app` ~6.3k→pasca-split (B), (3) v2 API churn baru (lipgloss/bubbles v2 masih berkembang — pin versi, jangan `go get -u` buta), (4) vt10x/PTY tidak ter-cover v2 changes (API `creack/pty` stabil).
- [x] **D4.** Rule 11 audit penuh: bandingkan SEMUA shortcut di PRD vs `menu.go` vs `view.go` (help modal `?`) vs `model.go` (Command Palette items) — perbaiki docs; binding tidak berubah kecuali user meminta.
- [x] **D5.** `TODO.md`: centang seluruh 5 item & kosongkan/normalisasi (sesuai konfirmasi user — semua TODO sudah diperbaiki).
- [x] **D6.** `AGENTS.md`: "PRD.md (v0.1.1)" → v0.2.0; stack table di section 2 diupdate ke versi v2 aktual; tambah Rule 12 (dari B6); Rule 11 tetap.
- [x] **D7.** `docs/architecture.md`: update stack + ukuran/struktur package `internal/app` pasca-split.
- [x] **D8.** `README.md`: update tabel versi & (bila ada) sebutan bubbletea v1.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Baris 0 (Top Menu) | Renderer v2 berbeda — risiko Baris 0 offscreen jika ada overflow baru | Rule 1 tetap: MaxHeight(1) + ANSI truncate; smoke test eksplisit (A13, Manual Test) |
| Kolom 2 (Editor) | Terminal pane: raw key/mouse forward harus identik byte-level | A6/A7 + `terminal_test.go`; verifikasi interaktif |
| Baris Bawah (Status Bar) | Tidak berubah; statusar `MaxHeight(1)` tetap | — |
| Modal (`?` help) | Hanya teks sync di D4 (jika ada selisih) | ANSI-safe edit |
| Refactor (B) | Pure move, tidak ada perubahan render | `go test -race` hijau di setiap langkah |
| v2 (A) | Perilaku renderer/mouse bisa halus berbeda | A12 + A13 + visual smoke; jika regresi ditemukan: fix di A, jangan biarkan lolos |

## UI / Keymap Flow
- **Tidak ada perubahan keybinding/mouse behavior yang disengaja.** Semua input path dipertahankan — hanya tipe message & accessor yang berubah. `?` help, menu, Command Palette disinkronkan di D4 (docs-only).
- Interaksi mouse: coords hit-testing sama; tipe event dimigrasikan.

## Tasks (ringkasan eksekusi)
- [x] A1–A13 (v2 full-stack migration + smoke) — A13 smoke menunggu user
- [x] B1–B7 (refactor internal/app + Rule 12)
- [x] C1–C3-baru (hardening terminal/git + LSP checklist)
- [x] D1–D8 (PRD v0.2.0 + TODO.md + AGENTS/README/architecture sync)
- [x] Verification: `go vet ./...` bersih ✓, `go test -race -count=1 ./...` hijau ✓ (216 passed, 20 packages)
- [ ] **Tidak ada git writes** (commit/push) tanpa approval eksplisit user

## Manual Test Checklist
- [ ] Aplikasi start di alt-screen, quit → terminal bersih (cursor kembali, tidak ada mouse tracking tersisa, shell prompt normal)
- [ ] Klik Top Menu `[File]`/`[Setting]`/`[About]` membuka dropdown; `Alt+F/T/A` juga
- [ ] Klik sidebar `[ 1:Files | 2:Git ]`, tree fold/expand, editor tab switch + `[×]` tutup, drag-drop reorder tab
- [ ] Git stage `[+]`/`[-]` per file & per hunk tampil benar
- [ ] Scroll wheel: preview markdown scroll; terminal pane scroll; editor scroll
- [ ] Baris 0 Top Menu selalu terlihat saat terminal dikecilkan 80×24 dan di-resize
- [ ] Terminal tab: ketik shell (termasuk `vim`/tmux interaktif), `Ctrl+C`, `Backspace`, `Alt+key`, arrow keys, Tab — semua sampai ke shell dengan benar; mouse klik di terminal tab tidak terhijack app handler (regression `3c2373f` tetap fix)
- [ ] `Ctrl+Shift+\` / `Ctrl+T` membuka terminal tab baru
- [ ] Setelah refactor (B): smoke test buka file, edit, save, git commit, terminal, markdown preview — normal; `wc -l internal/app/*.go` tidak ada file >500 baris (kecuali `model.go` dengan batas tercatat)
- [ ] PRD v0.2.0 menampilkan Revision History + Known Risks; stack table = versi v2; `TODO.md` tercentang/kosong; `AGENTS.md` & README sinkron

## Acceptance Criteria
- [ ] `go mod graph` tidak memuat `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `charmbracelet/bubbles`, `charmbracelet/glamour` v1 (x/ansi path lama boleh)
- [ ] `go vet ./...` 0 error; `go test -v -race ./...` 100% PASS
- [ ] Smoke A13 + Manual Test Checklist lolos semua
- [ ] `TODO.md` tercentang/dikosongkan sesuai konfirmasi user
- [ ] PRD.md v0.2.0 (Revision History + Known Risks + stack v2); AGENTS.md/README/architecture.md sinkron
- [ ] `internal/app/*.go` ≤500 baris (kecuali `model.go` dengan batas tercatat)
- [ ] AGENTS.md memiliki Rule 12 anti-God-class
- [ ] Rule 11 shortcut-sync terpenuhi di semua docs

## Decisions
- **Bubble Tea v2 DI-UPGRADE PENUH di plan ini** (revisi keputusan sebelumnya "catat saja", atas permintaan user 2026-11-14). Scope: `bubbletea` + `lipgloss` + `bubbles` + `glamour` → `charm.land/*/v2` + `x/ansi` latest. Alasan: hindari dua lipgloss di satu binary; bubbles v1 tidak kompatibel dengan tea v2 message types; glamour v2 memutus dependency ke lipgloss v1.
- **Urutan: migration → refactor → hardening → docs** — docs sekali tulis, mencerminkan realita v2 (pilihan user).
- **`View()` deklaratif** menggantikan program options (`WithAltScreen`/`WithMouseCellMotion`) — idiom v2, perilaku identik.
- **Raw PTY key forward (`terminal.go`) diperlakukan khusus** — satu-satunya tempat byte-level fidelity kritis; diprioritaskan di A6/A7 dan di-cover test.
- **Mouse hit-testing tidak diubah logikanya** — hanya tipe message & accessor; ini area regresi paling mungkin (`3c2373f`).
- **Track TODO dibuang** — user mengonfirmasi semua TODO sudah diperbaiki; `TODO.md` dikosongkan tanpa verifikasi ulang.
- **Refactor = pure move dulu** — tidak ada perubahan logika/render di Track B.
- **`setting.shortcuts` ActionID tidak di-rename** — hanya docs yang disinkronkan.
- **AGENTS.md Rule 12** — agar AI agent berikutnya tidak menumbuhkan God class lagi.
- **Tidak ada git writes otomatis** — commit/push menunggu instruksi eksplisit user.

## Blockers
- A13 (smoke interaktif): menunggu user menjalankan aplikasi — tidak bisa diverifikasi headless (alt-screen teardown, klik, PTY interaktif).
- Flaky test satu kali terdeteksi pada full suite (`terminal_tab_test.go:87`) selama pengembangan; penyebabnya echo child belum tentu reaped saat Alt+W ditekan — sudah diperbaiki dengan polling `HasRunningProcess` sebelum close di test; full suite kemudian hijau 4× berturut-turut (`-count=1`).

## Decisions Tambahan (2026-11-15)
- **Refactor memakai pure move + dispatcher** (urutan `handleKeyMsg`/`handleMouseMsg` dipertahankan persis). `update_keys.go` & `update_mouse.go` dipecah menjadi `keys_*.go` (9 file) dan `mouse_*.go` (10 file). `keys_buffer.go` dipisah lagi menjadi `keys_preview.go` (diff view + markdown preview) agar semua file ≤500 baris.
- **`keys_global.go` hanya berisi dispatcher `handleKeyMsg`** — ctrl+K chord di `keys_editor.go` (`handleCtrlKPending`), editor buffer di `keys_buffer.go`, diff/markdown di `keys_preview.go`.
- **Root key/chord matching via helper baru `internal/ui/key.go` (`ui.KeyName`)**: chord Ctrl/Alt → `msg.Keystroke()`; plain/shift-printable → `msg.String()`. Solusi atas breaking change v2 (`String()` membuang modifier). Tanpa helper ini seluruh shortcut Alt+/*- berhenti bekerja di v2.
- **Editor wheel scroll** (`handleEditorFocusMouse`) kini di-gate `isWheel` (di v1 wheel = press action, sehingga kebetulan bekerja). Perilaku ini sebelumnya "kebetulan benar"; di v2 harus eksplisit.
- **Bubble Tea v2 dimigrasikan penuh** (keputusan user): `bubbletea`/`lipgloss`/`bubbles`/`glamour` → `charm.land/*/v2`, `x/ansi` tetap path lama (v0.11.9). `lipgloss.Color` kini function (bukan type); field/param diganti `color.Color` (stdlib).
- **Test mouse/key message ditulis ulang** ke `MouseClickMsg`/`MouseWheelMsg`/`MouseMotionMsg`/`MouseReleaseMsg` dan `KeyPressMsg{Code, Text, Mod}`.
- **C1 — jalur close/quit diaudit & gap ditutup**: `terminal.close` (palette) & `Ctrl+Shift+W`/`Alt+W` (terminal) dialihkan ke `attemptCloseTab` (guard `HasRunningProcess` + pinned). Bulk close (`tab_close_right/left/all/others`, palette + context menu) diblokir dengan toast bila ada terminal unpinned dengan proses aktif (pola sama dengan guard dirty-buffer yang sudah ada).
- **`TerminalExitMsg` (shell exit natural) sengaja TIDAK di-guard** — terminal auto-close setelah proses benar-benar berakhir.
- **About dialog & welcome screen** di-bump ke v0.2.0 (`view.go`, `view_modals.go`).
- **Rule 11 audit**: selisih docs↔kode yang ditemukan hanya README (`Ctrl+\` didokumentasikan sebagai focus-toggle padahal di kode `Ctrl+\``/F6) — sudah diperbaiki. Sisanya (F1, `Ctrl+Shift+\`, `Ctrl+T`, `Ctrl+Shift+P`) konsisten docs↔kode.
- **AGENTS.md Rule 12 "Internal App Package Budget"** ditambahkan; Rule 11 direferensikan ke file baru (`view_modals.go`, `model_modals.go`).
- **`docs/lsp-audit-checklist.md`** dibuat (C2). Tidak ada rewrite LSP.
- **About/README/PRD/architecture stack** disinkronkan ke v0.2.0 + Charm v2. `docs/architecture.md` menyebut Bubble Tea v1.3+ → diperbarui.
- **README**: badge PRD diperbarui ke v0.2.0.
- **Tidak ada git writes** (commit/push) dilakukan — versi diserahkan ke user.

## Next
Set `status: review` selesai. Menunggu user:
1. Menjalankan aplikasi dan mencoba Manual Test Checklist (alt-screen teardown, klik, PTY interaktif).
2. Jika smoke lolos → pindahkan task ke `done/` (`task/done/2026-11-15 - Bubble Tea v2 Migration, PRD v0.2.0 & Refactor internal/app.md`), update `task/LOG.md` & `task/done/INDEX.md`.
3. Commit (jika user menginginkan) — tidak dilakukan otomatis.
