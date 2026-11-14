package linter

import (
	"path/filepath"
	"sync"
)

// Store maintains diagnostics for workspace files thread-safely.
type Store struct {
	mu   sync.RWMutex
	data map[string][]Diagnostic
}

// NewStore creates a new diagnostics store.
func NewStore() *Store {
	return &Store{
		data: make(map[string][]Diagnostic),
	}
}

// Set stores diagnostics for a file.
func (s *Store) Set(filePath string, diags []Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanPath := filepath.Clean(filePath)
	s.data[cleanPath] = diags
}

// Clear removes diagnostics for a file.
func (s *Store) Clear(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanPath := filepath.Clean(filePath)
	delete(s.data, cleanPath)
}

// Get returns diagnostics for a file.
func (s *Store) Get(filePath string) []Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cleanPath := filepath.Clean(filePath)
	return s.data[cleanPath]
}

// LineDiagnostics returns a map of 0-based physical line numbers to gutter marker string.
func (s *Store) LineDiagnostics(filePath string) map[int]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cleanPath := filepath.Clean(filePath)
	diags := s.data[cleanPath]
	if len(diags) == 0 {
		return nil
	}

	res := make(map[int]string)
	for _, d := range diags {
		line0 := d.Line - 1
		if line0 < 0 {
			line0 = 0
		}
		// Error takes precedence over warning
		existing, ok := res[line0]
		if !ok || d.Severity == SeverityError {
			res[line0] = d.Marker()
		}
		_ = existing
	}
	return res
}

// DiagnosticAt returns the first diagnostic at the given 1-based line number.
func (s *Store) DiagnosticAt(filePath string, line1Based int) *Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cleanPath := filepath.Clean(filePath)
	diags := s.data[cleanPath]
	for _, d := range diags {
		if d.Line == line1Based {
			diagCopy := d
			return &diagCopy
		}
	}
	return nil
}
