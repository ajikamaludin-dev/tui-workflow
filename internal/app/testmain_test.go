package app

import (
	"fmt"
	"os"
	"testing"
)

// TestMain redirects the user config directory to an isolated temp dir for the
// whole package. Tests in this package build full Model instances whose
// handlers persist config.json and session state on disk; without isolation
// they would overwrite the user's real ~/.config/tui-workflow/config.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tui-workflow-test-config-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: failed to create temp config dir:", err)
		os.Exit(1)
	}
	prev := os.Getenv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: failed to set XDG_CONFIG_HOME:", err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	if prev == "" {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
	} else {
		_ = os.Setenv("XDG_CONFIG_HOME", prev)
	}
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
