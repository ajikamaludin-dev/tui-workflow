package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	enabled bool
	logFile *os.File
	out     io.Writer = io.Discard
)

// Init initializes the debug logger. If enabled is true, logs are appended to filePath.
// If filePath is empty, "debug.log" in current working directory is used.
func Init(debugMode bool, filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	if !debugMode {
		enabled = false
		out = io.Discard
		return nil
	}

	if filePath == "" {
		filePath = "debug.log"
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file %q: %w", filePath, err)
	}

	logFile = f
	out = f
	enabled = true

	// Write session start marker
	_, _ = fmt.Fprintf(out, "\n=== [%s] DEBUG SESSION STARTED ===\n", time.Now().Format("2006-01-02 15:04:05.000"))
	return nil
}

// IsEnabled returns true if debug logging is active.
func IsEnabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return enabled
}

// Debugf writes a formatted log entry if debug logging is active.
func Debugf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !enabled || out == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(out, "[%s] %s\n", ts, msg)
}

// Close closes the underlying debug log file if open.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		_, _ = fmt.Fprintf(out, "=== [%s] DEBUG SESSION ENDED ===\n", time.Now().Format("2006-01-02 15:04:05.000"))
		_ = logFile.Close()
		logFile = nil
	}
	enabled = false
	out = io.Discard
}
