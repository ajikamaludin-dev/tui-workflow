package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHighTabChurnMemory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-churn-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create 50 files with content
	filePaths := make([]string, 50)
	for i := 0; i < 50; i++ {
		p := filepath.Join(tempDir, fmt.Sprintf("file_%02d.txt", i))
		content := fmt.Sprintf("File %d content line 1\nFile %d content line 2\n", i, i)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		filePaths[i] = p
	}

	runtime.GC()
	var mBaseline runtime.MemStats
	runtime.ReadMemStats(&mBaseline)

	tm := NewTabManager()

	// 1. Open 50 tabs sequentially
	for _, p := range filePaths {
		tab, err := tm.OpenFile(p)
		if err != nil {
			t.Fatalf("Failed to open tab '%s': %v", p, err)
		}
		if tab == nil {
			t.Fatalf("Expected non-nil tab")
		}
	}

	if tm.Count() != 50 {
		t.Fatalf("Expected 50 tabs open, got %d", tm.Count())
	}

	// 2. Close all 50 tabs
	for tm.Count() > 0 {
		tm.CloseTab(0)
	}

	if tm.Count() != 0 {
		t.Fatalf("Expected 0 tabs after closing, got %d", tm.Count())
	}

	// 3. Force GC and verify memory usage
	runtime.GC()
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)

	// Max 30 MB idle memory limit per PRD Sec 5.3
	const maxAllowedBytes = 30 * 1024 * 1024
	if mAfter.Alloc > maxAllowedBytes {
		t.Errorf("Memory leak detected: Alloc %d bytes exceeds 30MB limit (%d)", mAfter.Alloc, maxAllowedBytes)
	}
}
