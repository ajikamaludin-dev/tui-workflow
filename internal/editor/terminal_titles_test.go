package editor

import "testing"

func terminalNames(tm *TabManager) []string {
	var names []string
	for _, tab := range tm.Tabs {
		if tab.Type == TabTerminal {
			names = append(names, tab.Name)
		}
	}
	return names
}

func TestRenumberTerminalTabs(t *testing.T) {
	tm := NewTabManager()
	tm.OpenTerminal("term:1", "Terminal", nil)
	tm.OpenTerminal("term:2", "Terminal", nil)
	tm.OpenTerminal("term:3", "Terminal", nil)

	got := terminalNames(tm)
	want := []string{"Terminal", "Terminal - 2", "Terminal - 3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after open: got %v, want %v", got, want)
		}
	}

	// Closing the first terminal renumbers the remaining ones.
	tm.CloseTab(0)
	got = terminalNames(tm)
	want = []string{"Terminal", "Terminal - 2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after close: got %v, want %v", got, want)
		}
	}
}
