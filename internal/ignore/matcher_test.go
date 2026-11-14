package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcherDefaults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui_ignore_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	m := NewMatcher(tempDir)

	// .git, node_modules, and vendor should be ignored by default
	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{".git", true, true},
		{".git/config", false, true},
		{"node_modules", true, true},
		{"node_modules/axios/index.js", false, true},
		{"vendor", true, true},
		{"vendor/autoload.php", false, true},
		{"src/app.js", false, false},
		{"main.go", false, false},
		{"public", true, false},
	}

	for _, tt := range tests {
		fullPath := filepath.Join(tempDir, tt.path)
		got := m.Matches(fullPath, tt.isDir)
		if got != tt.want {
			t.Errorf("Matches(%q, isDir=%v) = %v; want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestMatcherWithGitignore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tui_ignore_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	gitignoreContent := `
# Comment line
/storage/tmp
*.log
!important.log
dist/
.env*
!.env.example
`
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewMatcher(tempDir)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"storage/tmp", true, true},
		{"storage/tmp/cache.txt", false, true},
		{"app.log", false, true},
		{"important.log", false, false},
		{"dist", true, true},
		{".env", false, true},
		{".env.production", false, true},
		{".env.example", false, false},
		{"storage/logs", true, false},
		{"app/Http/Kernel.php", false, false},
	}

	for _, tt := range tests {
		fullPath := filepath.Join(tempDir, tt.path)
		got := m.Matches(fullPath, tt.isDir)
		if got != tt.want {
			t.Errorf("Matches(%q, isDir=%v) = %v; want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}
