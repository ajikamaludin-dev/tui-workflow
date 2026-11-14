# tui-workflow Architecture & Technical Specifications

Dokumen ini mendokumentasikan arsitektur sistem, modularitas, event loop, dan alur kerja teknis dari **tui-workflow** — *Modern, High-Performance Terminal User Interface Workflow Environment*.

---

## 1. Arsitektur 4-Zona Antarmuka (Layout Zones)

Antarmuka `tui-workflow` dibagi secara deterministik menjadi 4 zona layout yang menjamin kestabilan visual tanpa *terminal flickering* dan tanpa *offscreen scroll*:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Row 0: Top Menu Bar (1 baris statis)                                   │
├──────────────────────────┬─────────────────────────────────────────────┤
│ Col 1: Sidebar (28 cols) │ Col 2: Editor Pane (lebar adaptif)          │
│ • Header (1 baris)       │ • Tab Bar (1 baris statis)                  │
│ • Refresh Row (1 baris)  │ • Editor Body / Terminal / Markdown / Diff  │
│ • Body Viewport          │   (Tinggi Pane = Height - 2,                │
│   (Tinggi = Height - 5)  │    Tinggi Body = Height - 3)                │
│ • Footer Tabs (1 baris)  │                                             │
├──────────────────────────┴─────────────────────────────────────────────┤
│ Row Height-1: Status Bar (1 baris statis)                              │
└────────────────────────────────────────────────────────────────────────┘
```

### Aturan Keras 1 Baris (Single-Line Guarantee)
- **Top Menu Bar** (`internal/ui/menu.go`), **Tab Bar** (`internal/editor/manager.go`), dan **Status Bar** (`internal/ui/statusbar.go`) wajib dibatasi dengan `MaxHeight(1)` dan dipotong aman dengan `ansi.Truncate(str, width, "")`.
- Pelanggaran batas tinggi 1 baris akan memicu terminal auto-scroll, menyebabkan Baris 0 terdorong ke atas dan menghilang dari layar (*layout corruption*).

---

## 2. Siklus Event & The Elm Architecture (Bubble Tea)

`tui-workflow` mengadopsi pola The Elm Architecture (`Model`, `Init`, `Update`, `View`) yang diorkestrasi oleh Bubble Tea v2 (`charm.land/bubbletea/v2`):

```text
               ┌───────────────────────┐
               │    Terminal Input     │
               │ (Keys, Mouse, Resize) │
               └───────────┬───────────┘
                           │
                           ▼
               ┌───────────────────────┐
               │     Model.Update()    │
               │  - Global shortcuts   │
               │  - Active modal check │
               │  - Mouse hit-testing  │
               │  - Active component   │
               └───────────┬───────────┘
                           │
             ┌─────────────┴─────────────┐
             ▼                           ▼
      State Mutated                  tea.Cmd
             │                   (Async Goroutine)
             │                           │
             ▼                           ▼
      ┌─────────────┐             ┌─────────────┐
      │ Model.View()│             │   tea.Msg   │
      └──────┬──────┘             └──────┬──────┘
             │                           │
             ▼                           │
      Rendered ANSI String ──────────────┘
```

### Prioritas Dispatcher `Update()`
1. **Modal & Floating Bars Interception**:
   - Jika ada dialog modal aktif (`CommandPalette`, `QuickOpen`, `GlobalSearch`, `BranchSwitcher`, `AISettingsDialog`, `FindBar`, `FilePicker`, `InputDialog`, `ConfirmDialog`, `Switcher`, `HelpModal`, `AboutModal`), event keyboard dan mouse wajib dicegat sepenuhnya agar tidak tembus ke buffer editor atau git widget di background.
2. **Global Shortcuts**:
   - `Ctrl+Q` (Clean Exit dengan `tea.Quit`), `Ctrl+E`/`F6` (Toggle Fokus), `Ctrl+B` (Toggle Sidebar), `Alt+1/2/3` (Sidebar Switch), `F1` (Command Palette), `Ctrl+P` (Quick Open).
3. **Focused Component Routing**:
   - Jika fokus pada `FocusEditor`, delegasikan event ke Tab Aktif (Text Buffer, Terminal PTY, Markdown Viewer, atau Git Split Diff).
   - Jika fokus pada `FocusSidebar`, delegasikan ke Tree Explorer, Git View, atau Search View.

---

## 3. Paket & Modul Inti

### `internal/app` (Orchestrator)
Root package berisi Elm Model, event dispatcher, dan render root. **Batas keras: setiap file maksimal 500 baris.** Pemecahan per-domain dilakukan agar AI agent berikutnya tidak menumbuhkan God Object (lihat AGENTS.md Rule 12).

**Model & Update**
- `model.go`: Definisi root `Model` + inisialisasi (`NewModelWithPath`, `NewModel`, `Init`).
- `update.go`: Entry point `Update(msg)` — routing pesan ke handler fokus.
- `model_actions.go`: Aksi sidebar/tab (resize sidebar, soft-wrap, watcher, close tab, binary guard).
- `model_modals.go`: Pembukaan modal (GoToSymbol, QuickOpen, Command Palette, GlobalSearch, Find/Replace Bar, Go-to-Line, Buffer Switcher, Branch Switcher, AI Settings).
- `model_diff.go`: Aksi hunk (stage/unstage/discard & refresh diff tab).
- `session.go`: Workspace session persistence (90–165).
- `commands.go`: `tea.Cmd` constructors (git refresh/commit/push, formatter, LSP completion, AI commit, terminal output watcher).
- `navigation.go`, `resize.go`, `keymap.go`: Navigasi viewport, helper resize, definisi konstanta key.

**Keyboard handlers (`keys_*.go`)**
- `keys_global.go`: Dispatcher `handleKeyMsg` (urutan tetap: ctrl+K → GoToSymbol → context menu → search modals → modals → terminal → sidebar/git → editor).
- `keys_editor.go`: Helper editing (`handleCtrlSpace`, `handleTab`, `handleShiftTab`, `handleCtrlKPending`, dispatcher `handleEditorKeys`).
- `keys_buffer.go`: Handler buffer modeless plain (autocomplete/snippet/fold/undo/dll).
- `keys_preview.go`: Handler Diff View & Markdown Preview.
- `keys_search.go`: GoToSymbol + Find Bar/Replace + QuickOpen + Command Palette + GlobalSearch.
- `keys_modal.go`: Modal non-search (BranchSwitcher, AI Settings, Switcher, FilePicker, Input/Confirm Dialog, MenuBar, Help/About).
- `keys_terminal.go`: Tab terminal PTY (raw keystroke forward + toggle fokus `Ctrl+\`).
- `keys_git.go`: Sidebar Git/Files (`d`/`x` buang hunk, discard, branch switcher `Alt+B`).

**Mouse handlers (`mouse_*.go`)**
- `update_mouse.go`: Dispatcher `handleMouseMsg` (urutan: modals → resize/drag → right-click → drag-select → release → menu bar → tab bar → sidebar wheel → editor wheel → help/about → click fallback).
- `mouse_modal.go`: Hit-testing modal & context menu + help/about.
- `mouse_sidebar.go`: Resize sidebar (`col-resize` pointer), right-click explorer, scroll sidebar, klik baris refresh/bottom tabs/body.
- `mouse_tabbar.go`: Drag tab, scroll horizontal, right-click tab bar, klik tab (Row 1).
- `mouse_editor.go`: Drag-select (buffer/terminal/markdown preview), klik editor (gutter/fold/selection), klik tab terminal.
- `mouse_terminal.go`: Right-click terminal (forward PTY mouse reporting).
- `mouse_menu.go`, `mouse_statusbar.go`: Klik Top Menu Bar & Status Bar.
- `mouse_click.go`, `mouse_release.go`: Dispatcher fallback klik kiri & release + right-click.

**Rendering**
- `view.go`: `View()` (tea.View), `renderSidebar`, `renderEditor` (4 zona layout).
- `view_overlay.go`: `overlayAt/Center/TopCenter`, `placeCenter` (overlay modal ANSI-aware).
- `view_modals.go`: `renderHelpModal`, `renderAboutModal`, `renderGoToSymbol`.
- `palette_actions.go`: `handleMenuAction` (semua aksi dari Menu Bar & Command Palette).
- `palette_context.go`: `handleContextMenuAction` + `openExplorerContextMenu`.

**Peta Budget (`wc -l internal/app/*.go` target ≤ 500)** — lihat AGENTS.md Rule 12.

### `internal/editor` (Text Buffer & Tab Engine)
- `buffer.go`: Modeless line-based buffer (`[][]rune`) dengan cursor clamping, soft-wrap virtual line mapping, gutter width adaptif, word jump, selection range, caching teks (`cachedText`), dan undo/redo snapshots.
- `manager.go`: Tab Manager yang mendukung multi-tab (`TabFile`, `TabTerminal`, `TabMarkdown`, `TabGitDiff`), scrolling horizontal, penutupan batch, dan konfirmasi dirty buffer.
- `tab.go`: Struktur data tab dan enum tipe tab (`TabType`).

### `internal/syntax` (Chroma v2 Engine)
- `highlighter.go`: Tokenisasi Chroma v2 dengan format ANSI styling.
- **Lazy Viewport-Bounded Parsing**: Hanya mengonversi baris mentah menjadi slice style ANSI per-rune (`[][]string`) saat baris tersebut tampak di jendela viewport, menghindari alokasi memori berlebih pada file besar.

### `internal/git` (In-Memory Git Plumbing)
- `repo.go`: Wrapper aman untuk `go-git/v5` (Status worktree, stage/unstage file, discard changes, commit, dan switch branch).
- `diff.go`: Kalkulasi LCS (Longest Common Subsequence) diff antara buffer lokal dan versi file di `HEAD` untuk tampilan Side-by-Side Split Diff tab.
- `view.go`: Sidebar Source Control view dengan commit box dan AI commit message generator.

### `internal/terminal` (Embedded Subshell PTY)
- `terminal.go`: Pengelola pseudoterminal (`creack/pty`) untuk mengeksekusi shell lokal (`/bin/bash` atau `$SHELL`) langsung di dalam tab editor.

### `internal/ui` & `internal/ui/dialog` (Design System & Modals)
- `theme.go`: Skema warna VS Code Dark (Background `#1e1e1e`, Accent `#007acc`, Foreground `#d4d4d4`).
- `contextmenu.go`: Pop-up context menu interaktif untuk aksi klik kanan pada Tab, Editor, dan File Explorer.
- `normalize.go`: Menghilangkan celah background ANSI bocor pada modal dialog (`NormalizeDialogBackground`).
- `dialog/`: Komponen modal interaktif (`palette.go`, `quickopen.go`, `globalsearch.go`, `branch.go`, `aisettings.go`, `findbar.go`, `filepicker.go`, `input.go`, `confirm.go`, `switcher.go`, `help.go`).

---

## 4. Keamanan File & Pencegahan Crash (Guards)

1. **Binary File Guard**:
   - Membaca 512 byte pertama file; jika ditemukan byte `0x00` (null byte), pembukaan file digagalkan dengan peringatan toast untuk mencegah korupsi terminal atau crash tokenizer.
2. **Large File Guard**:
   - Membatasi ukuran file maksimal **10 MB**. File yang melebihi batas ini ditolak agar tidak menghabiskan RAM terminal.
3. **Clean Teardown Guard**:
   - Larangan keras `os.Exit(0)`. Aplikasi wajib keluar via pengiriman `tea.Quit` agar terminal merestorasi alt-screen, kursor, dan mouse tracking secara bersih.
