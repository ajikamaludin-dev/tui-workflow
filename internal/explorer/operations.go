package explorer

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateFile(parentDir, fileName string) (string, error) {
	if fileName == "" {
		return "", fmt.Errorf("file name cannot be empty")
	}

	targetPath := filepath.Join(parentDir, fileName)
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("file '%s' already exists", fileName)
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	_ = f.Close()

	return targetPath, nil
}

func CreateFolder(parentDir, folderName string) (string, error) {
	if folderName == "" {
		return "", fmt.Errorf("folder name cannot be empty")
	}

	targetPath := filepath.Join(parentDir, folderName)
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("folder '%s' already exists", folderName)
	}

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	return targetPath, nil
}

func RenamePath(oldPath, newName string) (string, error) {
	if newName == "" {
		return "", fmt.Errorf("new name cannot be empty")
	}

	parent := filepath.Dir(oldPath)
	newPath := filepath.Join(parent, newName)

	if oldPath == newPath {
		return oldPath, nil
	}

	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("item with name '%s' already exists", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to rename: %w", err)
	}

	return newPath, nil
}

func DeletePath(path string) error {
	if path == "" || path == "." || path == "/" {
		return fmt.Errorf("cannot delete root directory")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
