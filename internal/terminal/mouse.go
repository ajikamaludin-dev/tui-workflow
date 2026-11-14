package terminal

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/hinshun/vt10x"
)

// Terminal Ctrl Keys & Mouse Protocol constants
const (
	mouseBtnLeftCb    = 0
	mouseBtnMiddleCb  = 1
	mouseBtnRightCb   = 2
	mouseWheelUpCb    = 64
	mouseWheelDownCb  = 65
	mouseWheelLeftCb  = 66
	mouseWheelRightCb = 67

	mouseMotionFlag = 32
	mouseShiftFlag  = 4
	mouseAltFlag    = 8
	mouseCtrlFlag   = 16
)

func mouseButtonCb(b tea.MouseButton) (int, bool) {
	switch b {
	case tea.MouseLeft:
		return mouseBtnLeftCb, true
	case tea.MouseMiddle:
		return mouseBtnMiddleCb, true
	case tea.MouseRight:
		return mouseBtnRightCb, true
	case tea.MouseWheelUp:
		return mouseWheelUpCb, true
	case tea.MouseWheelDown:
		return mouseWheelDownCb, true
	case tea.MouseWheelLeft:
		return mouseWheelLeftCb, true
	case tea.MouseWheelRight:
		return mouseWheelRightCb, true
	default:
		return 0, false
	}
}

// MouseForward encodes a Bubble Tea mouse event into the escape sequence that an
// inner application (vim, htop, lazygit, etc.) expects on its PTY stdin, honoring the
// DEC mode mouse tracking flags the inner app requested (9 / 1000 / 1002 / 1003 / 1006).
//
// mode must be the raw vt10x mouse mode flag (see TerminalView.MouseReporting).
// relX and relY are 0-based coordinates relative to the terminal body origin.
// Returns nil when the event must NOT be forwarded (e.g. motion while the app only
// requested button reporting, or release/motion in X10 mode 9).
func MouseForward(mode vt10x.ModeFlag, msg tea.MouseMsg, relX, relY int) []byte {
	if mode&vt10x.ModeMouseMask == 0 {
		return nil
	}

	m := msg.Mouse()
	base, ok := mouseButtonCb(m.Button)
	if !ok {
		return nil
	}

	isWheel := base >= mouseWheelUpCb
	_, isMotion := msg.(tea.MouseMotionMsg)
	_, isRelease := msg.(tea.MouseReleaseMsg)

	if isMotion {
		// 1000 (button press only) does not report motion; 1002 reports drag
		// motion (MouseModeCellMotion provides this); 1003 reports all
		// motion (MouseModeAllMotion provides hover motion).
		if mode&(vt10x.ModeMouseMotion|vt10x.ModeMouseMany) == 0 {
			return nil
		}
		base |= mouseMotionFlag
	}

	cb := base
	if m.Mod.Contains(tea.ModShift) {
		cb += mouseShiftFlag
	}
	if m.Mod.Contains(tea.ModAlt) {
		cb += mouseAltFlag
	}
	if m.Mod.Contains(tea.ModCtrl) {
		cb += mouseCtrlFlag
	}

	x := relX + 1
	y := relY + 1
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}

	// SGR extended encoding (1006) — preferred whenever the inner app enabled it
	if mode&vt10x.ModeMouseSgr != 0 {
		final := "M"
		if isRelease && !isWheel {
			final = "m"
			cb &^= mouseMotionFlag
		}
		return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%s", cb, x, y, final))
	}

	// Legacy X10 encoding (single-byte coordinates, max 223 cols)
	encX := x + 32
	encY := y + 32
	if encX > 255 {
		encX = 255
	}
	if encY > 255 {
		encY = 255
	}

	// mode 9 (X10 compat): button presses only, no motion, no release, no wheel
	if mode&vt10x.ModeMouseX10 != 0 &&
		mode&(vt10x.ModeMouseButton|vt10x.ModeMouseMotion|vt10x.ModeMouseMany) == 0 {
		if isMotion || isRelease || isWheel {
			return nil
		}
		return []byte{0x1b, '[', 'M', byte(cb + 32), byte(encX), byte(encY)}
	}

	// 1000 / 1002 / 1003 legacy release: button sentinel 3
	if isRelease && !isWheel {
		return []byte{0x1b, '[', 'M', byte(3 + 32), byte(encX), byte(encY)}
	}
	return []byte{0x1b, '[', 'M', byte(cb + 32), byte(encX), byte(encY)}
}

// mouseRelevantMask keeps the reporting bits (9/1000/1002/1003) plus the SGR
// extended-encoding bit (1006), which vt10x excludes from ModeMouseMask.
const mouseRelevantMask = vt10x.ModeMouseMask | vt10x.ModeMouseSgr

// MouseReporting returns the mouse tracking mode requested by the application
// running inside the PTY: the reporting bits plus the SGR (1006) bit.
func (t *TerminalView) MouseReporting() vt10x.ModeFlag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.vt == nil {
		return 0
	}
	return t.vt.Mode() & mouseRelevantMask
}

// WantsMouse reports whether the inner application requested any mouse reporting.
func (t *TerminalView) WantsMouse() bool {
	return t.MouseReporting() != 0
}

// SGRMouse reports whether the inner application enabled SGR extended (1006) encoding.
func (t *TerminalView) SGRMouse() bool {
	return t.MouseReporting()&vt10x.ModeMouseSgr != 0
}

// ForwardMouse encodes msg (a Bubble Tea mouse event, coordinates relative to the
// terminal body origin) and writes it into the PTY. Returns true when bytes were
// forwarded, false when the event was dropped (mode filtering) or the PTY is closed.
func (t *TerminalView) ForwardMouse(msg tea.MouseMsg, relX, relY int) bool {
	bytes := MouseForward(t.MouseReporting(), msg, relX, relY)
	if len(bytes) == 0 {
		return false
	}
	_, err := t.Write(bytes)
	return err == nil
}
