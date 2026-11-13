# tui-workflow Architecture & Technical Specifications

Dokumen ini mendokumentasikan arsitektur sistem, modularitas, event loop, dan alur kerja teknis dari **tui-workflow** — *Modern, High-Performance Terminal User Interface Workflow Environment*.

---

## 1. Arsitektur 4-Zona Antarmuka (Layout Zones)

Antarmuka `tui-workflow` dibagi secara deterministik menjadi 4 zona layout yang menjamin kestabilan visual tanpa *terminal flickering* dan tanpa *offscreen scroll*:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Row 0: Top Menu Bar (1 baris statis)                                  │
├──────────────────────────┬─────────────────────────────────────────────┤
│ Col 1: Sidebar (28 cols) │ Col 2: Editor Pane (lebar adaptif)          │
│ • Header (1 baris)       │ • Tab Bar (1 baris statis)                  │
│ • Refresh Row (1 baris)  │ • Editor Body / Terminal / Markdown / Diff  │
│ • Body Viewport          │   (Tinggi Pane = Height - 2,                │
│   (Tinggi = Height - 5)  │    Tinggi Body = Height - 3)                │
│ • Footer Tabs (1 baris)  │                                             │
├──────────────────────────┴─────────────────────────────────────────────┤
│ Row Height-1: Status Bar (1 baris statis)                             │
└────────────────────────────────────────────────────────────────────────┘
```

### Aturan Keras 1 Baris (Single-Line Guarantee)
- **Top Menu Bar** (`internal/ui/menu.go`), **Tab Bar** (`internal/editor/manager.go`), dan **Status Bar** (`internal/ui/statusbar.go`) wajib dibatasi dengan `MaxHeight(1)` dan dipotong aman dengan `ansi.Truncate(str, width, "")`.
- Pelanggaran batas tinggi 1 baris akan memicu terminal auto-scroll, menyebabkan Baris 0 terdorong ke atas dan menghilang dari layar (*layout corruption*).

---

## 2. Siklus Event & The Elm Architecture (Bubble Tea)

`tui-workflow` mengadopsi pola The Elm Architecture (`Model`, `Init`, `Update`, `View`) yang diorkestrasi oleh Bubble Tea v1.3+:

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
- `model.go`: Mendefinisikan root `Model`, inisialisasi state, event dispatcher global, dan interceptor mouse/keyboard modal.
- `view.go`: Menggabungkan 4 zona layout dan merender overlay modal dengan helper `overlayCenter` dan `overlayTopCenter` berbasis ANSI trimming.

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
