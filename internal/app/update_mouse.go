package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	_, isClick := msg.(tea.MouseClickMsg)
	_, isMotion := msg.(tea.MouseMotionMsg)
	_, isRelease := msg.(tea.MouseReleaseMsg)
	_, isWheel := msg.(tea.MouseWheelMsg)

	// Intercept mouse clicks for active modals
	if nm, cmd, handled := m.handleModalMouse(msg, mouse, isClick); handled {
		return nm, cmd
	}

	if nm, cmd, handled := m.handleSidebarResizeMouse(msg, mouse, isClick, isMotion, isRelease); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.handleTabDragMouse(msg, mouse, isMotion, isRelease); handled {
		return nm, cmd
	}

	editorStartX := 0
	if m.SidebarVisible {
		editorStartX = m.SidebarWidth
	}

	// Update mouse pointer shape: col-resize at border, text over editor, default elsewhere
	if m.SidebarVisible && mouse.X == m.SidebarWidth-1 && mouse.Y >= 1 && mouse.Y < m.Height-1 {
		m.setMousePointer("col-resize")
	} else if mouse.X >= editorStartX && mouse.Y >= 2 && mouse.Y < m.Height-1 {
		m.setMousePointer("text")
	} else {
		m.setMousePointer("default")
	}

	// Right-click handling
	if nm, cmd, handled := m.handleRightClickMouse(msg, mouse, editorStartX, isClick); handled {
		return nm, cmd
	}

	// Drag mouse inside editor buffer area to select text
	if nm, cmd, handled := m.handleMotionMouse(msg, mouse, editorStartX, isMotion); handled {
		return nm, cmd
	}

	// Release mouse inside editor area
	if nm, cmd, handled := m.handleReleaseMouse(msg, mouse, editorStartX, isRelease); handled {
		return nm, cmd
	}

	// Forward mouse to menu bar
	if nm, cmd, handled := m.handleMenuBarMouse(msg, mouse); handled {
		return nm, cmd
	}

	// Mouse scroll wheel on Tab Bar (Row 1): scroll tabs horizontally
	if nm, cmd, handled := m.handleTabBarWheelMouse(msg, mouse, editorStartX); handled {
		return nm, cmd
	}

	// Mouse scroll wheel in sidebar (Row 2 to Height-2)
	if nm, cmd, handled := m.handleSidebarWheelMouse(msg, mouse, editorStartX); handled {
		return nm, cmd
	}

	// Mouse scroll wheel in editor
	if nm, cmd, handled := m.handleEditorFocusMouse(msg, mouse, editorStartX, isWheel); handled {
		return nm, cmd
	}

	// ShowHelp / ShowAbout mouse handling
	if nm, cmd, handled := m.handleHelpAboutMouse(msg, mouse, isClick); handled {
		return nm, cmd
	}

	// Mouse click handling
	if nm, cmd, handled := m.handleClickFallbackMouse(msg, mouse, editorStartX, isClick); handled {
		return nm, cmd
	}

	return m, nil
}
