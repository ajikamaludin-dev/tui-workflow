package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tui-workflow/internal/editor"
	"tui-workflow/internal/search"
	"tui-workflow/internal/ui/dialog"
)

// handleSearchReplaceRequest processes a replace request emitted by the sidebar
// SearchView (keyboard or mouse). Scope "match" executes immediately (no
// confirmation); scope "file" and "all" open the ConfirmReplace modal.
func (m Model) handleSearchReplaceRequest(req search.ReplaceRequest) (Model, tea.Cmd) {
	sv := m.SearchView
	if sv == nil {
		return m, nil
	}
	repl := sv.ReplaceInput.Value()
	query := sv.CurrentQuery()

	var snapshot []search.SearchMatch
	switch req.Scope {
	case search.ReplaceScopeFile:
		for _, mm := range sv.Matches {
			if mm.FilePath == req.FilePath {
				snapshot = append(snapshot, mm)
			}
		}
	case search.ReplaceScopeMatch:
		for _, mm := range sv.Matches {
			if mm.FilePath == req.FilePath && mm.LineNumber == req.LineNumber && mm.ColNumber == req.ColNumber {
				snapshot = append(snapshot, mm)
				break
			}
		}
	default:
		snapshot = append(snapshot, sv.Matches...)
	}

	job := search.BuildReplaceJob(req, query, repl, snapshot)
	if job.Occurrences() == 0 && req.Scope != search.ReplaceScopeMatch {
		m.StatusBar.ShowToast("No matches to replace", false, 2*time.Second)
		return m, nil
	}

	if req.Scope == search.ReplaceScopeMatch {
		occ, files, err := executeReplaceJob(&m, job)
		if err != nil {
			m.StatusBar.ShowToast("Replace error: "+err.Error(), true, 3*time.Second)
			return m, nil
		}
		if occ == 0 {
			m.StatusBar.ShowToast("Match no longer present (buffer changed?)", true, 2*time.Second)
			return m, nil
		}
		m.StatusBar.ShowToast(fmt.Sprintf("Replaced %d occurrence(s) in %d file(s)", occ, files), false, 2*time.Second)
		return m, nil
	}

	m.ConfirmDialog.Open(dialog.ConfirmReplace, "Replace", job.Describe(), job.Token())
	m.ConfirmDialog.Payload = job
	return m, nil
}

// executeReplaceJob performs the replacement. Files open in tabs go through the
// buffer (undo-able); other files are rewritten on disk in place, preserving the
// original file mode. Returns (occurrences, filesTouched, error).
func executeReplaceJob(m *Model, job search.ReplaceJob) (int, int, error) {
	files := job.Files()
	totalOcc := 0
	filesDone := 0
	touchedBuffers := false

	for _, f := range files {
		tab := findOpenTabByPath(m, f)
		if tab != nil && tab.Buffer != nil {
			n := replaceInBuffer(tab, job)
			if n > 0 {
				totalOcc += n
				filesDone++
				touchedBuffers = true
			}
			continue
		}
		n, err := replaceOnDisk(f, job)
		if err != nil {
			return totalOcc, filesDone, err
		}
		if n > 0 {
			totalOcc += n
			filesDone++
		}
	}

	if touchedBuffers {
		if m.AutoSave {
			_, _ = editor.TriggerAutoSave(m.TabManager)
		} else {
			m.StatusBar.ShowToast("Buffer(s) modified but not saved — search results may be stale", false, 3*time.Second)
		}
	}

	if m.SearchView != nil {
		m.SearchView.DoSearch()
	}
	m.syncStatusBarWithActiveTab()
	return totalOcc, filesDone, nil
}

func findOpenTabByPath(m *Model, path string) *editor.Tab {
	clean := filepath.ToSlash(filepath.Clean(path))
	for _, tab := range m.TabManager.Tabs {
		if tab == nil || tab.Path == "" {
			continue
		}
		if filepath.ToSlash(filepath.Clean(tab.Path)) == clean {
			return tab
		}
	}
	return nil
}

func replaceInBuffer(tab *editor.Tab, job search.ReplaceJob) int {
	b := tab.Buffer
	if job.Scope == search.ReplaceScopeMatch {
		b.SetSearchOptions(job.Query)
		if b.ReplaceMatchAt(job.LineNumber-1, job.ColNumber-1, job.Replacement) {
			return 1
		}
		return 0
	}
	return b.ReplaceAllOptions(job.Query, job.Replacement)
}

func replaceOnDisk(path string, job search.ReplaceJob) (int, error) {
	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil {
		return 0, err
	}
	if info.Size() == 0 || info.Size() > 10*1024*1024 {
		return 0, nil // silently skip ineligible files
	}
	f, err := os.Open(clean)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		_ = f.Close()
		return 0, nil
	}
	if strings.ContainsRune(string(buf[:n]), 0) {
		_ = f.Close()
		return 0, nil
	}
	_ = f.Close()

	content, err := os.ReadFile(clean)
	if err != nil {
		return 0, err
	}
	matcher, err := search.Compile(job.Query)
	if err != nil {
		return 0, err
	}

	switch job.Scope {
	case search.ReplaceScopeMatch:
		return replaceMatchInContent(content, job, matcher, clean, info.Mode().Perm())
	default:
		text := string(content)
		newText, occ := matcher.ReplaceAllInText(text, job.Replacement)
		if occ == 0 {
			return 0, nil
		}
		if err := os.WriteFile(clean, []byte(newText), info.Mode().Perm()); err != nil {
			return 0, err
		}
		return occ, nil
	}
}

func replaceMatchInContent(content []byte, job search.ReplaceJob, matcher *search.Matcher, clean string, mode os.FileMode) (int, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	if job.LineNumber < 1 || job.LineNumber > len(lines) {
		return 0, nil
	}
	idx := job.LineNumber - 1
	lineRunes := []rune(lines[idx])
	for _, m := range matcher.FindInLine(lineRunes, idx) {
		if m.Col == job.ColNumber-1 {
			// Expand $1 group references against the matched text when in regex mode.
			repText := job.Replacement
			if matcher.RegexMode() {
				matched := string(lineRunes[m.Col : m.Col+m.Length])
				newText, _ := matcher.ReplaceAllInText(matched, job.Replacement)
				repText = newText
			}
			repRunes := []rune(repText)
			out := make([]rune, 0, len(lineRunes)-m.Length+len(repRunes))
			out = append(out, lineRunes[:m.Col]...)
			out = append(out, repRunes...)
			out = append(out, lineRunes[m.Col+m.Length:]...)
			lines[idx] = string(out)
			newText := strings.Join(lines, "\n")
			if err := os.WriteFile(clean, []byte(newText), mode); err != nil {
				return 0, err
			}
			return 1, nil
		}
	}
	return 0, nil
}


