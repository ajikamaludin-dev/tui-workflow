package editor

import (
	"strings"
	"testing"
)

func TestCountLeadingSpaces(t *testing.T) {
	tests := []struct {
		input       string
		wantSpaces  int
		wantHaveTab bool
	}{
		{"", 0, false},
		{"abc", 0, false},
		{"  abc", 2, false},
		{"    abc", 4, false},
		{"\tabc", 4, true},
		{"  \tabc", 6, true},
	}

	for _, tt := range tests {
		gotSpaces, gotTab := CountLeadingSpaces([]rune(tt.input))
		if gotSpaces != tt.wantSpaces || gotTab != tt.wantHaveTab {
			t.Errorf("CountLeadingSpaces(%q) = (%d, %v), want (%d, %v)",
				tt.input, gotSpaces, gotTab, tt.wantSpaces, tt.wantHaveTab)
		}
	}
}

func TestRenderIndentRune(t *testing.T) {
	// 4 spaces (aligned): level 0 guide at 0, spaces at 1, 2, 3
	res0, ok0 := RenderIndentRune(0, 4, ' ')
	if !ok0 || !strings.Contains(res0, "│") {
		t.Errorf("RenderIndentRune(0, 4) want vertical guide, got %q", res0)
	}

	res1, ok1 := RenderIndentRune(1, 4, ' ')
	if !ok1 || res1 != " " {
		t.Errorf("RenderIndentRune(1, 4) want plain space, got %q", res1)
	}

	// 3 spaces (misaligned): error guard
	resErr, okErr := RenderIndentRune(0, 3, ' ')
	if !okErr || !strings.Contains(resErr, " ") {
		t.Errorf("RenderIndentRune(0, 3) want error highlight, got %q", resErr)
	}
}

func TestBufferDiagnostics(t *testing.T) {
	buf := NewBuffer("line 1\nline 2\nline 3")
	buf.SetDiagnostic(1, "●")

	if buf.LineDiagnostics[1] != "●" {
		t.Errorf("LineDiagnostics[1] = %v, want ●", buf.LineDiagnostics[1])
	}

	rendered := buf.Render(40, 5, "test.js", true)
	if !strings.Contains(rendered, "●") {
		t.Errorf("Rendered buffer should contain diagnostic marker ●, got:\n%s", rendered)
	}

	buf.ClearDiagnostics()
	if len(buf.LineDiagnostics) != 0 {
		t.Errorf("ClearDiagnostics() failed, got len %d", len(buf.LineDiagnostics))
	}
}
