package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Disabled mode
	_ = Init(false, "")
	if IsEnabled() {
		t.Errorf("expected logger to be disabled")
	}
	Debugf("this should not be written")

	// Enabled mode
	if err := Init(true, logPath); err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}
	if !IsEnabled() {
		t.Errorf("expected logger to be enabled")
	}

	Debugf("testing hello %d", 42)
	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	str := string(content)
	if !strings.Contains(str, "DEBUG SESSION STARTED") {
		t.Errorf("expected session start marker, got: %s", str)
	}
	if !strings.Contains(str, "testing hello 42") {
		t.Errorf("expected log message, got: %s", str)
	}
	if !strings.Contains(str, "DEBUG SESSION ENDED") {
		t.Errorf("expected session end marker, got: %s", str)
	}
}
