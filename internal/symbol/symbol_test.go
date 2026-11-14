package symbol

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSymbolIndexLifecycle(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	fileA := filepath.Join(tempDir, "file_a.go")
	contentA := []byte(`package main
func FunctionA() {}
`)
	if err := os.WriteFile(fileA, contentA, 0644); err != nil {
		t.Fatal(err)
	}

	// Create an ignored dir
	gitDir := filepath.Join(tempDir, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	_ = os.WriteFile(filepath.Join(gitDir, "ignored.go"), []byte("package git\nfunc Ignored() {}\n"), 0644)

	idx := NewIndex(tempDir)
	if err := idx.Build(); err != nil {
		t.Fatalf("build index failed: %v", err)
	}

	if idx.Count() != 1 {
		t.Fatalf("expected 1 symbol, got %d", idx.Count())
	}

	defs := idx.Definitions("FunctionA")
	if len(defs) != 1 || defs[0].Name != "FunctionA" {
		t.Fatalf("expected FunctionA definition, got %+v", defs)
	}

	if len(idx.Definitions("Ignored")) != 0 {
		t.Error("symbol in .git should not be indexed")
	}

	// UpdateFile
	contentAUpdated := []byte(`package main
func FunctionA() {}
func FunctionB() {}
`)
	if err := os.WriteFile(fileA, contentAUpdated, 0644); err != nil {
		t.Fatal(err)
	}
	idx.UpdateFile(fileA)

	if idx.Count() != 2 {
		t.Fatalf("expected 2 symbols after update, got %d", idx.Count())
	}
	if len(idx.Definitions("FunctionB")) != 1 {
		t.Fatal("expected FunctionB definition after update")
	}

	// RenameFile
	fileB := filepath.Join(tempDir, "file_b.go")
	if err := os.Rename(fileA, fileB); err != nil {
		t.Fatal(err)
	}
	idx.RenameFile(fileA, fileB)

	defs = idx.Definitions("FunctionA")
	if len(defs) != 1 || defs[0].FilePath != fileB {
		t.Fatalf("expected FunctionA to point to %s, got %+v", fileB, defs)
	}

	// RemoveFile
	_ = os.Remove(fileB)
	idx.RemoveFile(fileB)

	if idx.Count() != 0 {
		t.Fatalf("expected 0 symbols after remove, got %d", idx.Count())
	}
	if len(idx.Definitions("FunctionA")) != 0 {
		t.Fatal("expected no definitions for FunctionA after remove")
	}
}

func TestSymbolIndexConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "test.go")
	_ = os.WriteFile(file, []byte("package main\nfunc TestFunc() {}\n"), 0644)

	idx := NewIndex(tempDir)
	_ = idx.Build()

	var wg sync.WaitGroup
	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = idx.Definitions("TestFunc")
				_ = idx.AllSymbols()
				_ = idx.Count()
			}
		}()
	}
	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				idx.UpdateFile(file)
			}
		}()
	}
	wg.Wait()
}
