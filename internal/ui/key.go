package ui

import tea "charm.land/bubbletea/v2"

// KeyName canonicalizes a key-press message for string-based matching.
//
// In Bubble Tea v2 msg.String() returns the printable text and DROPS modifiers
// (alt+i → "i"), while msg.Keystroke() always carries modifiers but lowercases
// the base key. This helper combines both: chord combos with Ctrl/Alt use
// Keystroke() ("alt+i", "ctrl+shift+i"); everything else (plain printable keys,
// shift+printable like "?" and "M", and modifier-less special keys) uses
// String() ("?", "a", "tab", "enter").
func KeyName(msg tea.KeyPressMsg) string {
	if msg.Mod.Contains(tea.ModCtrl) || msg.Mod.Contains(tea.ModAlt) {
		return msg.Keystroke()
	}
	return msg.String()
}
