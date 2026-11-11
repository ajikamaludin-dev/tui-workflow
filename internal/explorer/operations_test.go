package explorer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui-workflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Create File
	createdFile, err := CreateFile(tempDir, "hello.txt")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if _, err := os.Stat(createdFile); os.IsNotExist(err) {
		t.Errorf("Created file does not exist on disk")
	}

	// 2. Duplicate Create File should fail
	_, err = CreateFile(tempDir, "hello.txt")
	if err == nil {
		t.Errorf("Expected error when creating duplicate file, got nil")
	}

	// 3. Create Folder
	createdDir, err := CreateFolder(tempDir, "subfolder")
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}
	if info, err := os.Stat(createdDir); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("Created folder does not exist or is not a directory")
	}

	// 4. Rename Path
	renamedFile, err := RenamePath(createdFile, "renamed.txt")
	if err != nil {
		t.Fatalf("RenamePath failed: %v", err)
	}
	if filepath.Base(renamedFile) != "renamed.txt" {
		t.Errorf("Expected renamed filename 'renamed.txt', got '%s'", filepath.Base(renamedFile))
	}
	if _, err := os.Stat(renamedFile); os.IsNotExist(err) {
		t.Errorf("Renamed file does not exist on disk")
	}

	// 5. Delete Path
	err = DeletePath(renamedFile)
	if err != nil {
		t.Fatalf("DeletePath failed: %v", err)
	}
	if _, err := os.Stat(renamedFile); !os.IsNotExist(err) {
		t.Errorf("File should be deleted, but still exists")
	}
}
