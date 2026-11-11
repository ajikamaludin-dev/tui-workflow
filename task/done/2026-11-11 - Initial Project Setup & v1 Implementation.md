---
title: Initial Project Setup & v1 Implementation
status: done
started: 2026-11-11
completed: 2026-11-11
branch: main
priority: high
---

# Initial Project Setup & v1 Implementation

## Objective
Implement all core requirements of PRD v1.1.0 for tui-workflow, covering layout shell, modeless editor, explorer CRUD, git source control, markdown preview, and test suites.

## Plan
Direct phased implementation from Phase 1 to Phase 6 in accordance with PRD specifications.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Top Menu Bar (Row 0) | Offscreen scroll if height > 1 | MaxHeight(1) & ANSI-aware truncate |
| Middle Columns | Sizing conflict during sidebar toggle | Dynamic width allocation (28 cols sidebar, remainder editor) |
| Status Bar | Line overflow causing offscreen scroll | Dynamic priority truncation + MaxHeight(1) |

## Tasks
- [x] Phase 1: Setup & Layout Shell (Top Menu, Sidebar, Editor Pane, Status Bar)
- [x] Phase 2: File Explorer & CRUD (Tree, operations, modal dialogs)
- [x] Phase 3: Modeless Text Editor & Chroma Syntax Highlighting
- [x] Phase 4: Git Source Control & Side-by-Side Split Diff (go-git/v5)
- [x] Phase 5: Live Markdown Preview (Glamour engine with SHA-256 caching)
- [x] Phase 6: Polish, Profiling, E2E Test Suite, and Bug Triage
- [x] Verification: go vet ./... ✓, go test -v -race ./... ✓

## Manual Test Checklist
- [x] Verify top menu bar dropdowns open and close smoothly via mouse click and Alt+F/T/A.
- [x] Verify file explorer CRUD operations (new file 'a', new folder 'A', rename 'F2', delete 'd').
- [x] Verify multi-tab editor with typing, syntax highlighting, and unsaved changes confirmation.
- [x] Verify Git sidebar stage/unstage buttons and split diff view.
- [x] Verify Markdown preview toggle via Ctrl+M.

## Acceptance Criteria
- [x] All PRD v1.1.0 requirements implemented and passing.
- [x] Terminal minimum resolution guard (80x24) functioning with overlay.
- [x] 100% test pass with -race flag and 0 vet warnings.

## Decisions
- Used tea.WithMouseCellMotion() for full mouse interactions.
- Maintained 1-row tab bar guarantee with horizontal scrolling window to prevent layout shifts.
- Implemented 10MB limit and 512B null byte check to safeguard against binary files in editor and diff.

## Next
All initial v1 requirements completed. Ready for future task development.
