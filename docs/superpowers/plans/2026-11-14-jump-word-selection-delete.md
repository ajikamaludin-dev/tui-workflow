# Word Jump Selection and Word Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan seleksi block text per kata (word jump selection) dan hapus kata dengan backspace di Editor, serta word jump dan word deletion di Terminal PTY.
**Architecture:** Menambahkan keyboard selection API dan `DeleteWordLeft` pada `Buffer`, menyambungkan modifier keyboard di dispatcher `model.go`, serta memetakan modifier `Alt`/`Ctrl` ke sequence Readline di `terminal.KeyToBytes`.
**Tech Stack:** Go 1.24+, Bubble Tea v1.3.4, Lipgloss v1.0.0
**Spec:** `docs/superpowers/specs/2026-11-14-jump-word-selection-delete-design.md`

## Global Constraints
- Go 1.24+ standard library & existing project dependencies only.
- Strict ANSI safety and single-line layout clamping rules from `AGENTS.md`.
- `go vet ./...` must return 0 errors.
- `go test -v -race ./...` must pass 100%.

## Review Focus
- Boundary check pada awal dan akhir baris saat seleksi per kata melintasi batas baris (`CursorRow` dan `CursorCol`).
- Seleksi saat kursor bergerak mundur (`SelectAnchorCol > CursorCol`) dinormalisasi dengan benar oleh `NormalizedSelection`.
- Undo/redo stack merekam `DeleteWordLeft` secara atomik sehingga satu kali `Ctrl+Z` mengembalikan seluruh kata yang terhapus.
- `KeyToBytes` tidak merusak tombol arrow atau backspace normal tanpa modifier.
- Tombol navigasi non-shift (`Left`, `Right`, dsb.) selalu membersihkan selection (`ClearSelection`).

---

### Task 1: Buffer Selection & Word Delete API in `internal/editor/buffer.go`
**Files:**
- Modify: `internal/editor/buffer.go`
- Test: `internal/editor/buffer_test.go`
**Interfaces:**
- Produces:
  - `(b *Buffer) SelectWordLeft()`
  - `(b *Buffer) SelectWordRight()`
  - `(b *Buffer) SelectLeft()`
  - `(b *Buffer) SelectRight()`
  - `(b *Buffer) SelectUp()`
  - `(b *Buffer) SelectDown()`
  - `(b *Buffer) DeleteWordLeft()`
- [ ] **Step 1: Write failing tests in `internal/editor/buffer_test.go`**
  - Test `TestBufferSelectWord`: verifikasi `SelectWordLeft` dan `SelectWordRight` mengeset anchor dan `HasSelection = true`.
  - Test `TestBufferDeleteWordLeft`: verifikasi penghapusan kata ke kiri dan integritas undo stack.
- [ ] **Step 2: Run test verify fails**
  Run: `go test -v -run "TestBufferSelectWord|TestBufferDeleteWordLeft" ./internal/editor`
- [ ] **Step 3: Implement selection methods and `DeleteWordLeft` in `internal/editor/buffer.go`**
- [ ] **Step 4: Run test verify passes**
  Run: `go test -v -run "TestBufferSelectWord|TestBufferDeleteWordLeft" ./internal/editor`

---

### Task 2: Terminal PTY Word Jump & Backspace in `internal/terminal/terminal.go`
**Files:**
- Modify: `internal/terminal/terminal.go`
- Test: `internal/terminal/terminal_test.go`
**Interfaces:**
- Modifies: `KeyToBytes(msg tea.KeyMsg) []byte`
- [ ] **Step 1: Write failing tests in `internal/terminal/terminal_test.go`**
  - Test `KeyToBytes` dengan `Alt+Left`, `Ctrl+Left`, `Alt+Right`, `Ctrl+Right`, `Alt+Backspace`, `Ctrl+Backspace`.
- [ ] **Step 2: Run test verify fails**
  Run: `go test -v -run "TestKeyToBytes" ./internal/terminal`
- [ ] **Step 3: Implement sequence mapping in `KeyToBytes`**
- [ ] **Step 4: Run test verify passes**
  Run: `go test -v -run "TestKeyToBytes" ./internal/terminal`

---

### Task 3: Key Dispatching in `internal/app/model.go`
**Files:**
- Modify: `internal/app/model.go`
- Test: `internal/app/model_test.go`
- [ ] **Step 1: Add key cases in `model.go` editor switch-case**
  - `shift+alt+left`, `alt+shift+left`, `shift+ctrl+left`, `ctrl+shift+left`
  - `shift+alt+right`, `alt+shift+right`, `shift+ctrl+right`, `ctrl+shift+right`
  - `shift+left`, `shift+right`, `shift+up`, `shift+down`
  - `alt+backspace`, `ctrl+backspace`
- [ ] **Step 2: Write integration test in `internal/app/model_test.go` or `search_features_test.go`**
- [ ] **Step 3: Run test verify passes**
  Run: `go test -v ./internal/app`

---

### Task 4: Shortcut Synchronization & Documentation
**Files:**
- Modify: `internal/app/view.go` (Help Modal `?`)
- Modify: `internal/app/model.go` (Command Palette items)
- Modify: `README.md`
- Modify: `PRD.md`
- [ ] **Step 1: Update Help Modal (`renderHelpModal`) with new shortcuts**
- [ ] **Step 2: Update Command Palette items**
- [ ] **Step 3: Update README.md and PRD.md tables**

---

### Task 5: Verification Suite
- [ ] **Step 1: Run `go vet ./...`**
- [ ] **Step 2: Run `go test -v -race ./...`**
