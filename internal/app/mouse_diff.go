package app

import (
	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/git"
)

// activeDiffView returns the DiffView of the active tab when it is a Git
// side-by-side diff tab, otherwise nil.
func (m Model) activeDiffView() *git.DiffView {
	tab := m.TabManager.ActiveTab()
	if tab == nil || !tab.IsDiff || tab.DiffView == nil {
		return nil
	}
	return tab.DiffView
}

// diffEditorStartX is the absolute column where the editor pane begins.
func (m Model) diffEditorStartX() int {
	if m.SidebarVisible {
		return m.SidebarWidth
	}
	return 0
}

// diffSeparatorAbsX returns the absolute screen column of the diff separator,
// or -1 when there is no active diff tab.
func (m Model) diffSeparatorAbsX() int {
	dv := m.activeDiffView()
	if dv == nil {
		return -1
	}
	startX := m.diffEditorStartX()
	editorWidth := m.Width - startX
	left, _ := dv.ColumnWidths(editorWidth)
	return startX + left
}

// diffHit reports whether a coordinate lies on the draggable separator of an
// active diff tab. The separator spans the diff body rows only (row 2 through
// Height-2) and exactly one column, so horizontal hit-testing is precise.
func (m Model) diffHit(x, y int) bool {
	if m.ResizingDiff {
		return true
	}
	if y < 2 || y >= m.Height-1 {
		return false
	}
	sepX := m.diffSeparatorAbsX()
	return sepX >= 0 && x == sepX
}

// handleDiffResizeStart starts a drag when the user clicks the separator, and
// shows the col-resize pointer when merely hovering it.
func (m Model) handleDiffResizeStart(msg tea.MouseMsg, mouse tea.Mouse, isClick bool) (tea.Model, tea.Cmd, bool) {
	if m.diffHit(mouse.X, mouse.Y) {
		m.setMousePointer("col-resize")
		if isClick && mouse.Button == tea.MouseLeft {
			m.ResizingDiff = true
			return m, nil, true
		}
	}
	return m, nil, false
}

// handleDiffResizing updates the split ratio from the horizontal mouse
// position while a drag is in progress. The ratio is derived from the
// separator offset within the editor pane; ColumnWidths applies min-width
// clamping so both sides stay readable.
func (m Model) handleDiffResizing(msg tea.MouseMsg, mouse tea.Mouse, isMotion, isRelease bool) (tea.Model, tea.Cmd, bool) {
	if !m.ResizingDiff {
		return m, nil, false
	}
	dv := m.activeDiffView()
	if dv == nil {
		m.ResizingDiff = false
		m.setMousePointer("default")
		return m, nil, true
	}
	if isRelease {
		m.ResizingDiff = false
		return m, nil, true
	}
	if isMotion {
		startX := m.diffEditorStartX()
		editorWidth := m.Width - startX
		if editorWidth > 1 {
			relX := mouse.X - startX
			if relX < 0 {
				relX = 0
			}
			if relX > editorWidth-1 {
				relX = editorWidth - 1
			}
			dv.SetSplitRatio(float64(relX) / float64(editorWidth-1))
		}
		m.setMousePointer("col-resize")
		return m, nil, true
	}
	return m, nil, false
}

// handleReleaseDiffResize ends a diff drag regardless of where the pointer is
// released, so the col-resize cursor never sticks.
func (m Model) handleReleaseDiffResize(msg tea.MouseMsg, mouse tea.Mouse) (tea.Model, tea.Cmd, bool) {
	if m.ResizingDiff {
		m.ResizingDiff = false
		m.setMousePointer("default")
		return m, nil, true
	}
	return m, nil, false
}
