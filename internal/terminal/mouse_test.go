package terminal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hinshun/vt10x"
)

func clickMsg(btn tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: btn}
}

func releaseMsg(btn tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: btn}
}

func motionMsg(btn tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: btn}
}

func wheelMsg(btn tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: btn}
}

func TestMouseForward_NoModeRequested(t *testing.T) {
	msg := clickMsg(tea.MouseLeft, 3, 2)
	if got := MouseForward(0, msg, 3, 2); got != nil {
		t.Fatalf("expected nil when inner app requested no mouse mode, got %q", got)
	}
}

func TestMouseForward_ReportsMouseModes(t *testing.T) {
	term := &TerminalView{
		vt:   vt10x.New(vt10x.WithSize(10, 3)),
		cols: 10,
		rows: 3,
	}
	mode := term.MouseReporting()
	if mode != 0 {
		t.Fatalf("expected no mouse mode before inner app enables it, got %v", mode)
	}
	term.vt.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	mode = term.MouseReporting()
	if mode&vt10x.ModeMouseButton == 0 {
		t.Fatalf("expected ModeMouseButton after 1000h, got %v", mode)
	}
	if mode&vt10x.ModeMouseSgr == 0 {
		t.Fatalf("expected ModeMouseSgr after 1006h, got %v", mode)
	}
	if !term.WantsMouse() {
		t.Fatal("expected WantsMouse to be true")
	}
	if !term.SGRMouse() {
		t.Fatal("expected SGRMouse to be true")
	}
}

func TestMouseForward_SGR(t *testing.T) {
	mode := vt10x.ModeMouseButton | vt10x.ModeMouseMotion | vt10x.ModeMouseSgr

	tests := []struct {
		name     string
		msg      tea.MouseMsg
		expected string
	}{
		{"left press", clickMsg(tea.MouseLeft, 3, 2), "\x1b[<0;3;2M"},
		{"right press", clickMsg(tea.MouseRight, 3, 2), "\x1b[<2;3;2M"},
		{"middle press", clickMsg(tea.MouseMiddle, 3, 2), "\x1b[<1;3;2M"},
		{"release", releaseMsg(tea.MouseLeft, 3, 2), "\x1b[<0;3;2m"},
		{"drag motion", motionMsg(tea.MouseLeft, 3, 2), "\x1b[<32;3;2M"},
		{"wheel up", wheelMsg(tea.MouseWheelUp, 3, 2), "\x1b[<64;3;2M"},
		{"wheel down", wheelMsg(tea.MouseWheelDown, 3, 2), "\x1b[<65;3;2M"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MouseForward(mode, tt.msg, 2, 1)
			if string(got) != tt.expected {
				t.Fatalf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMouseForward_SGRWithModifiers(t *testing.T) {
	mode := vt10x.ModeMouseButton | vt10x.ModeMouseSgr
	msg := tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft, Mod: tea.ModCtrl | tea.ModShift}
	got := MouseForward(mode, msg, 0, 0)
	// ctrl (16) + shift (4) + left (0) = 20
	if string(got) != "\x1b[<20;1;1M" {
		t.Fatalf("got %q, want %q", got, "\x1b[<20;1;1M")
	}
}

func TestMouseForward_MotionDroppedWithoutMotionMode(t *testing.T) {
	mode := vt10x.ModeMouseButton | vt10x.ModeMouseSgr // 1000 only
	msg := motionMsg(tea.MouseLeft, 1, 1)
	if got := MouseForward(mode, msg, 1, 1); got != nil {
		t.Fatalf("motion must be dropped in 1000 mode, got %q", got)
	}
}

func TestMouseForward_LegacyX10(t *testing.T) {
	// 1000 without 1006 → legacy encoding
	mode := vt10x.ModeMouseButton

	got := MouseForward(mode, clickMsg(tea.MouseLeft, 3, 2), 2, 1)
	want := []byte{0x1b, '[', 'M', byte(0 + 32), byte(3 + 32), byte(2 + 32)}
	if string(got) != string(want) {
		t.Fatalf("press got %v, want %v", got, want)
	}

	got = MouseForward(mode, releaseMsg(tea.MouseLeft, 3, 2), 2, 1)
	want = []byte{0x1b, '[', 'M', byte(3 + 32), byte(3 + 32), byte(2 + 32)}
	if string(got) != string(want) {
		t.Fatalf("release got %v, want %v", got, want)
	}
}

func TestMouseForward_Mode9PressOnly(t *testing.T) {
	mode := vt10x.ModeMouseX10
	press := MouseForward(mode, clickMsg(tea.MouseLeft, 1, 1), 1, 1)
	if len(press) != 6 {
		t.Fatalf("expected X10 press encoding, got %v", press)
	}
	if got := MouseForward(mode, releaseMsg(tea.MouseLeft, 1, 1), 1, 1); got != nil {
		t.Fatalf("mode 9 must drop release, got %v", got)
	}
	if got := MouseForward(mode, motionMsg(tea.MouseLeft, 1, 1), 1, 1); got != nil {
		t.Fatalf("mode 9 must drop motion, got %v", got)
	}
}

func TestMouseForward_WheelInMode1000(t *testing.T) {
	mode := vt10x.ModeMouseButton | vt10x.ModeMouseSgr
	got := MouseForward(mode, wheelMsg(tea.MouseWheelUp, 5, 5), 5, 5)
	if string(got) != "\x1b[<64;6;6M" {
		t.Fatalf("wheel up got %q", got)
	}
}

func TestMouseForward_SGRWithoutReportingMode(t *testing.T) {
	// SGR bit alone (no 9/1000/1002/1003) must not enable reporting
	got := MouseForward(vt10x.ModeMouseSgr, clickMsg(tea.MouseLeft, 1, 1), 1, 1)
	if got != nil {
		t.Fatalf("expected nil for SGR-only mode, got %q", got)
	}
}
