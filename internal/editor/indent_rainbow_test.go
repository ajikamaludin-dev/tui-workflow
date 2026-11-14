package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

	// Tab character indentation tests
	resTab0, okTab0 := RenderIndentRune(0, 4, '\t')
	if !okTab0 || !strings.Contains(resTab0, "│") || !strings.HasSuffix(resTab0, "   ") {
		t.Errorf("RenderIndentRune(0, 4, '\\t') want vertical guide with 3 trailing spaces, got %q", resTab0)
	}

	resTab1, okTab1 := RenderIndentRune(4, 8, '\t')
	if !okTab1 || !strings.Contains(resTab1, "│") || !strings.HasSuffix(resTab1, "   ") {
		t.Errorf("RenderIndentRune(4, 8, '\\t') want level-1 vertical guide with 3 trailing spaces, got %q", resTab1)
	}
}

func TestBuffer_IndentRainbow_TabRendering(t *testing.T) {
	content := "package main\n\tvar a = 1\n\t\tif a > 0 {\n\t\t\treturn\n\t\t}"
	buf := NewBuffer(content)
	buf.IndentRainbow = true

	rendered := buf.Render(80, 10, "main.go", false)
	plain := ansi.Strip(rendered)

	// In line 1 ("\tvar a = 1"), "var" must NOT directly follow "│" without spaces
	if strings.Contains(plain, "│var") {
		t.Errorf("Rendered buffer contains '│var' without spacing after indent guide")
	}
	if !strings.Contains(plain, "│   var") {
		t.Errorf("Rendered buffer should contain '│   var', got:\n%s", plain)
	}

	// In line 2 ("\t\tif a > 0 {"), the space after 'if' must NOT be turned into a guide bar
	if strings.Contains(plain, "if│") || strings.Contains(plain, "if |") {
		t.Errorf("Rendered buffer corrupted space after 'if' with indent guide, got:\n%s", plain)
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
