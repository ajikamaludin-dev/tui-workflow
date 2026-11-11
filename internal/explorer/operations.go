package explorer

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateFile(parentDir, fileName string) (string, error) {
	if fileName == "" {
		return "", fmt.Errorf("nama file tidak boleh kosong")
	}

	targetPath := filepath.Join(parentDir, fileName)
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("file '%s' sudah ada", fileName)
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori induk: %w", err)
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("gagal membuat file: %w", err)
	}
	_ = f.Close()

	return targetPath, nil
}

func CreateFolder(parentDir, folderName string) (string, error) {
	if folderName == "" {
		return "", fmt.Errorf("nama folder tidak boleh kosong")
	}

	targetPath := filepath.Join(parentDir, folderName)
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("folder '%s' sudah ada", folderName)
	}

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat folder: %w", err)
	}

	return targetPath, nil
}

func RenamePath(oldPath, newName string) (string, error) {
	if newName == "" {
		return "", fmt.Errorf("nama baru tidak boleh kosong")
	}

	parent := filepath.Dir(oldPath)
	newPath := filepath.Join(parent, newName)

	if oldPath == newPath {
		return oldPath, nil
	}

	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("item dengan nama '%s' sudah ada", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("gagal mengubah nama: %w", err)
	}

	return newPath, nil
}

func DeletePath(path string) error {
	if path == "" || path == "." || path == "/" {
		return fmt.Errorf("tidak dapat menghapus root directory")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path tidak ditemukan: %w", err)
	}

	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
