package config

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the user config directory for the package so save/load
// tests never touch the user's real config.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tui-workflow-test-config-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: failed to create temp config dir:", err)
		os.Exit(1)
	}
	prevXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	if prevXDG == "" {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
	} else {
		_ = os.Setenv("XDG_CONFIG_HOME", prevXDG)
	}
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
