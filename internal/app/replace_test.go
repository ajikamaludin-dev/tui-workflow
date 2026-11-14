package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tui-workflow/internal/search"
)

func makeSearchViewFixture(t *testing.T) (*Model, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m, err := NewModelWithPath(tmpDir, false)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	return &m, tmpDir
}

func TestDiskReplacePreservesMode(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "mode.txt")
	if err := os.WriteFile(target, []byte("foo\nFOO\nbar foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	snap := []search.SearchMatch{
		{FilePath: target, RelPath: "mode.txt", LineNumber: 1, ColNumber: 1},
		{FilePath: target, RelPath: "mode.txt", LineNumber: 2, ColNumber: 1},
		{FilePath: target, RelPath: "mode.txt", LineNumber: 3, ColNumber: 5},
	}
	job := search.BuildReplaceJob(
		search.ReplaceRequest{Scope: search.ReplaceScopeAll},
		search.Query{Pattern: "foo"},
		"X",
		snap,
	)
	occ, files, err := executeReplaceJob(m, job)
	if err != nil {
		t.Fatalf("executeReplaceJob error: %v", err)
	}
	if occ != 3 {
		t.Fatalf("expected 3 occurrences (CI), got %d", occ)
	}
	if files != 1 {
		t.Fatalf("expected 1 file, got %d", files)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "X\nX\nbar X\n" {
		t.Fatalf("content = %q", string(content))
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestDiskReplaceSkipsBinary(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "bin.bin")
	if err := os.WriteFile(target, []byte("foo\x00foo inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := []search.SearchMatch{{FilePath: target, RelPath: "bin.bin", LineNumber: 1, ColNumber: 1}}
	job := search.BuildReplaceJob(
		search.ReplaceRequest{Scope: search.ReplaceScopeFile, FilePath: target},
		search.Query{Pattern: "foo"},
		"X",
		snap,
	)
	occ, _, err := executeReplaceJob(m, job)
	if err != nil {
		t.Fatal(err)
	}
	if occ != 0 {
		t.Fatalf("expected 0 occurrences for binary file, got %d", occ)
	}
	content, _ := os.ReadFile(target)
	if string(content) != "foo\x00foo inside" {
		t.Fatal("binary file must remain untouched")
	}
}

func TestOpenTabBufferPreferredOverDisk(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "open.txt")
	_ = os.WriteFile(target, []byte("foo one\n"), 0o644)
	m.TabManager.OpenFile(target)
	activeTab := m.TabManager.ActiveTab()
	if activeTab == nil {
		t.Fatal("expected active tab")
	}
	m.AutoSave = false // make the test independent of the user's config
	// Externally modify disk to something else
	_ = os.WriteFile(target, []byte("foo two\n"), 0o644)

	snap := []search.SearchMatch{{FilePath: target, RelPath: "open.txt", LineNumber: 1, ColNumber: 1}}
	job := search.BuildReplaceJob(
		search.ReplaceRequest{Scope: search.ReplaceScopeFile, FilePath: target},
		search.Query{Pattern: "foo"},
		"REPLACED",
		snap,
	)
	occ, files, err := executeReplaceJob(m, job)
	if err != nil {
		t.Fatal(err)
	}
	if occ != 1 || files != 1 {
		t.Fatalf("expected 1 occurrence in 1 file, got %d in %d", occ, files)
	}
	// Buffer must contain the replacement; disk must NOT have been touched.
	if got := strings.Join(runeSliceToStrings(activeTab.Buffer.Lines), "\n"); !strings.Contains(got, "REPLACED") {
		t.Fatalf("buffer must contain replacement, got %q", got)
	}
	disk, _ := os.ReadFile(target)
	if strings.Contains(string(disk), "REPLACED") {
		t.Fatal("disk must remain untouched until the buffer is saved")
	}
}

func TestPendingReplaceClearedOnCancel(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "cancel.txt")
	_ = os.WriteFile(target, []byte("foo\n"), 0o644)
	m.SearchView.FindInput.SetValue("foo")
	m.SearchView.ReplaceInput.SetValue("X")
	m.SearchView.DoSearch()

	// Simulate a replace-all request → opens confirm modal
	nm, _ := m.handleSearchReplaceRequest(search.ReplaceRequest{Scope: search.ReplaceScopeAll}) 
	mo := nm
	if !mo.ConfirmDialog.Active {
		t.Fatal("expected ConfirmReplace modal to be active")
	}
	if mo.ConfirmDialog.Payload == nil {
		t.Fatal("expected payload snapshot to be stored")
	}

	// Press Esc → canceled, payload must be cleared
	newM, _ := mo.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	mo = newM.(Model)
	if mo.ConfirmDialog.Active {
		t.Fatal("expected confirm dialog to close on Esc")
	}
	if mo.ConfirmDialog.Payload != nil {
		t.Fatal("payload must be cleared on cancel")
	}
}

func TestPendingReplaceConfirmExecutes(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "confirm.txt")
	_ = os.WriteFile(target, []byte("foo and foo\n"), 0o644)
	m.SearchView.FindInput.SetValue("foo")
	m.SearchView.ReplaceInput.SetValue("X")
	m.SearchView.DoSearch()

	nm, _ := m.handleSearchReplaceRequest(search.ReplaceRequest{Scope: search.ReplaceScopeAll}) 
	mo := nm
	// Confirm dialog defaults to Cancel (Selected=1); press left to select Replace (0)
	newM, _ := mo.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	mo = newM.(Model)
	newM2, _ := mo.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	mo = newM2.(Model)

	if mo.ConfirmDialog.Active {
		t.Fatal("expected confirm dialog to be closed after confirm")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "X and X\n" {
		t.Fatalf("content = %q, want %q", string(content), "X and X\n")
	}
}

func runeSliceToStrings(lines [][]rune) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out
}

func TestReplaceAllUsesFrozenQueryAndSnapshot(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	target := filepath.Join(tmpDir, "frozen.txt")
	_ = os.WriteFile(target, []byte("foo foo\nbar\n"), 0o644)
	m.SearchView.FindInput.SetValue("foo")
	m.SearchView.ReplaceInput.SetValue("X")
	m.SearchView.DoSearch()

	nm, _ := m.handleSearchReplaceRequest(search.ReplaceRequest{Scope: search.ReplaceScopeAll})
	mo := nm
	// User edits the find query while the confirm modal is open (simulated via direct mutation)
	mo.SearchView.FindInput.SetValue("bar")

	// Confirm Replace (Selected=1 Cancel → left, enter)
	newM, _ := mo.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	mo = newM.(Model)
	newM2, _ := mo.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	mo = newM2.(Model)

	_ = mo
	content, _ := os.ReadFile(target)
	if string(content) != "X X\nbar\n" {
		t.Fatalf("frozen snapshot replace wrote %q, want %q", string(content), "X X\nbar\n")
	}
}

func TestReplaceWithAutoSaveTrueSavesBuffer(t *testing.T) {
	m, tmpDir := makeSearchViewFixture(t)
	m.AutoSave = true
	target := filepath.Join(tmpDir, "autosave.txt")
	_ = os.WriteFile(target, []byte("foo\n"), 0o644)
	m.TabManager.OpenFile(target)

	snap := []search.SearchMatch{{FilePath: target, RelPath: "autosave.txt", LineNumber: 1, ColNumber: 1}}
	job := search.BuildReplaceJob(
		search.ReplaceRequest{Scope: search.ReplaceScopeFile, FilePath: target},
		search.Query{Pattern: "foo"},
		"X",
		snap,
	)
	occ, files, err := executeReplaceJob(m, job)
	if err != nil {
		t.Fatal(err)
	}
	if occ != 1 || files != 1 {
		t.Fatalf("expected 1 occurrence in 1 file, got %d in %d", occ, files)
	}
	content, _ := os.ReadFile(target)
	if string(content) != "X\n" {
		t.Fatalf("autosave must persist buffer, disk = %q", string(content))
	}
}
