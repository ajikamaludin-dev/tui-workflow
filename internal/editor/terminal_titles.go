package editor

import "fmt"

// RenumberTerminalTabs assigns sequential titles to all open terminal tabs in
// tab order: the first terminal is "Terminal", subsequent terminals are
// "Terminal - 2", "Terminal - 3", and so on.
func (tm *TabManager) RenumberTerminalTabs() {
	n := 0
	for _, t := range tm.Tabs {
		if t.Type != TabTerminal {
			continue
		}
		n++
		if n == 1 {
			t.Name = "Terminal"
			continue
		}
		t.Name = fmt.Sprintf("Terminal - %d", n)
	}
}
