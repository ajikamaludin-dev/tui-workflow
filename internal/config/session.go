package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SessionSchemaVersion = 1

type SessionTab struct {
	Path            string `json:"path"`
	Type            string `json:"type"` // file|markdown
	CursorRow       int    `json:"cursor_row"`
	CursorCol       int    `json:"cursor_col"`
	ScrollRow       int    `json:"scroll_row"`
	ScrollCol       int    `json:"scroll_col"`
	MarkdownPreview bool   `json:"markdown_preview,omitempty"`
}

type SessionSidebar struct {
	Visible bool   `json:"visible"`
	Width   int    `json:"width"`
	View    string `json:"view"` // files|git|search
}

type WorkspaceSession struct {
	Version   int            `json:"version"`
	RootDir   string         `json:"root_dir"`
	SavedAt   time.Time      `json:"saved_at"`
	OpenTabs  []SessionTab   `json:"open_tabs,omitempty"`
	ActiveTab string         `json:"active_tab,omitempty"`
	Sidebar   SessionSidebar `json:"sidebar"`
	Focus     string         `json:"focus"` // editor|sidebar|menu
}

type SessionIndexEntry struct {
	File    string    `json:"file"`
	RootDir string    `json:"root_dir"`
	SavedAt time.Time `json:"saved_at,omitempty"`
	Version int       `json:"version,omitempty"`
}

type SessionIndex struct {
	Version    int                          `json:"version,omitempty"`
	Workspaces map[string]SessionIndexEntry `json:"workspaces,omitempty"`
}

func canonicalRoot(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = filepath.Clean(resolved)
	}
	return abs
}

func workspaceSessionFileName(root string) string {
	root = canonicalRoot(root)
	sum := sha1.Sum([]byte(root))
	base := filepath.Base(root)
	base = strings.ReplaceAll(base, string(filepath.Separator), "_")
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "workspace"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	return fmt.Sprintf("session-%s-%s.json", base, hex.EncodeToString(sum[:8]))
}

func ensureSessionIndex(idx *SessionIndex) *SessionIndex {
	if idx == nil {
		return &SessionIndex{Version: SessionSchemaVersion, Workspaces: map[string]SessionIndexEntry{}}
	}
	if idx.Version <= 0 {
		idx.Version = SessionSchemaVersion
	}
	if idx.Workspaces == nil {
		idx.Workspaces = map[string]SessionIndexEntry{}
	}
	return idx
}

func sessionFilePathForRoot(root string) (string, string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", "", err
	}
	name := workspaceSessionFileName(root)
	return filepath.Join(dir, name), name, nil
}

func LoadWorkspaceSession(root string) (*WorkspaceSession, error) {
	root = canonicalRoot(root)
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	idx := ensureSessionIndex(cfg.SessionIndex)
	entry, ok := idx.Workspaces[root]
	dir, _ := GetConfigDir()

	tryPaths := make([]string, 0, 3)
	if ok && entry.File != "" && dir != "" {
		tryPaths = append(tryPaths, filepath.Join(dir, filepath.Base(entry.File)))
	}
	if p, _, err := sessionFilePathForRoot(root); err == nil && p != "" {
		dup := false
		for _, existing := range tryPaths {
			if filepath.Clean(existing) == filepath.Clean(p) {
				dup = true
				break
			}
		}
		if !dup {
			tryPaths = append(tryPaths, p)
		}
	}
	// fallback: scan config dir for session-*-<hash>.json matching root hash
	if dir != "" {
		if _, name, err := sessionFilePathForRoot(root); err == nil && name != "" {
			p := filepath.Join(dir, name)
			dup := false
			for _, existing := range tryPaths {
				if filepath.Clean(existing) == filepath.Clean(p) {
					dup = true
					break
				}
			}
			if !dup {
				tryPaths = append(tryPaths, p)
			}
		}
	}

	var data []byte
	var readErr error
	foundPath := ""
	for _, p := range tryPaths {
		if p == "" {
			continue
		}
		b, e := os.ReadFile(p)
		if e == nil {
			data = b
			readErr = nil
			foundPath = p
			break
		}
		readErr = e
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		// try next candidate on transient/read errors
		continue
	}
	if data == nil {
		if readErr == nil || errors.Is(readErr, os.ErrNotExist) {
			return nil, nil
		}
		return nil, readErr
	}

	var sess WorkspaceSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	if sess.Version <= 0 {
		sess.Version = SessionSchemaVersion
	}
	if sess.RootDir == "" {
		sess.RootDir = root
	} else {
		sess.RootDir = canonicalRoot(sess.RootDir)
	}
	// self-heal index if file found via fallback path
	if dir != "" && foundPath != "" {
		_ = os.MkdirAll(dir, 0o755)
		if cfg.SessionIndex == nil {
			cfg.SessionIndex = &SessionIndex{Version: SessionSchemaVersion, Workspaces: map[string]SessionIndexEntry{}}
		}
		cfg.SessionIndex = ensureSessionIndex(cfg.SessionIndex)
		cfg.SessionIndex.Workspaces[root] = SessionIndexEntry{
			File:    filepath.Base(foundPath),
			RootDir: root,
			SavedAt: sess.SavedAt,
			Version: sess.Version,
		}
		pruneSessionIndex(cfg.SessionIndex, dir)
		_ = SaveConfig(cfg)
	}
	return &sess, nil
}

func SaveWorkspaceSession(root string, sess *WorkspaceSession) error {
	if sess == nil {
		return nil
	}
	root = canonicalRoot(root)
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path, fileName, err := sessionFilePathForRoot(root)
	if err != nil {
		return err
	}
	sess.Version = SessionSchemaVersion
	sess.RootDir = root
	if sess.SavedAt.IsZero() {
		sess.SavedAt = time.Now()
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(path, data, 0o644); err != nil {
		return err
	}

	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}
	cfg.SessionIndex = ensureSessionIndex(cfg.SessionIndex)
	cfg.SessionIndex.Workspaces[root] = SessionIndexEntry{
		File:    fileName,
		RootDir: root,
		SavedAt: sess.SavedAt,
		Version: sess.Version,
	}
	pruneSessionIndex(cfg.SessionIndex, dir)
	return SaveConfig(cfg)
}

func pruneSessionIndex(idx *SessionIndex, dir string) {
	if idx == nil || idx.Workspaces == nil || dir == "" {
		return
	}
	for k, e := range idx.Workspaces {
		if e.File == "" || e.RootDir == "" {
			delete(idx.Workspaces, k)
			continue
		}
		p := filepath.Join(dir, filepath.Base(e.File))
		if _, err := os.Stat(p); err != nil {
			delete(idx.Workspaces, k)
		}
	}
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		// best effort
	}
	if err := tmp.Sync(); err != nil {
		// best effort
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func NormalizeWorkspaceSession(sess *WorkspaceSession) {
	if sess == nil {
		return
	}
	if sess.Sidebar.Width <= 0 {
		sess.Sidebar.Width = 30
	}
	if sess.Sidebar.Width < 15 {
		sess.Sidebar.Width = 15
	}
	switch sess.Sidebar.View {
	case "files", "git", "search":
	default:
		sess.Sidebar.View = "files"
	}
	switch sess.Focus {
	case "editor", "sidebar", "menu":
	default:
		sess.Focus = "editor"
	}
	seen := map[string]bool{}
	out := make([]SessionTab, 0, len(sess.OpenTabs))
	for _, t := range sess.OpenTabs {
		if t.Path == "" {
			continue
		}
		typ := strings.ToLower(t.Type)
		if typ != "file" && typ != "markdown" {
			continue
		}
		p := canonicalRoot(t.Path)
		key := typ + "|" + p
		if seen[key] {
			continue
		}
		seen[key] = true
		t.Path = p
		t.Type = typ
		if t.CursorRow < 0 {
			t.CursorRow = 0
		}
		if t.CursorCol < 0 {
			t.CursorCol = 0
		}
		if t.ScrollRow < 0 {
			t.ScrollRow = 0
		}
		if t.ScrollCol < 0 {
			t.ScrollCol = 0
		}
		out = append(out, t)
	}
	sess.OpenTabs = out
	if sess.ActiveTab != "" {
		act := canonicalRoot(sess.ActiveTab)
		found := false
		for _, t := range sess.OpenTabs {
			if canonicalRoot(t.Path) == act {
				sess.ActiveTab = t.Path
				found = true
				break
			}
		}
		if !found && len(sess.OpenTabs) > 0 {
			sess.ActiveTab = sess.OpenTabs[len(sess.OpenTabs)-1].Path
		} else if !found {
			sess.ActiveTab = ""
		}
	} else if len(sess.OpenTabs) > 0 {
		sess.ActiveTab = sess.OpenTabs[len(sess.OpenTabs)-1].Path
	}
}
