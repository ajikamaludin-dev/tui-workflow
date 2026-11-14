package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DetectLanguageServer identifies the appropriate language server for a file extension.
func DetectLanguageServer(rootDir, ext string) (bin string, args []string, langID string) {
	ext = strings.ToLower(ext)
	switch ext {
	case ".js", ".mjs", ".cjs":
		langID = "javascript"
	case ".jsx":
		langID = "javascriptreact"
	case ".ts":
		langID = "typescript"
	case ".tsx":
		langID = "typescriptreact"
	case ".go":
		langID = "go"
	case ".py":
		langID = "python"
	default:
		return "", nil, ""
	}

	// 1. JS / TS / React
	if strings.HasPrefix(langID, "javascript") || strings.HasPrefix(langID, "typescript") {
		localTS := filepath.Join(rootDir, "node_modules", ".bin", "typescript-language-server")
		if info, err := os.Stat(localTS); err == nil && !info.IsDir() {
			return localTS, []string{"--stdio"}, langID
		}
		if p, err := exec.LookPath("typescript-language-server"); err == nil {
			return p, []string{"--stdio"}, langID
		}
		if p, err := exec.LookPath("biome"); err == nil {
			return p, []string{"lsp-proxy"}, langID
		}
		if p, err := exec.LookPath("vtsls"); err == nil {
			return p, []string{"--stdio"}, langID
		}
	}

	// 2. Go
	if langID == "go" {
		if p, err := exec.LookPath("gopls"); err == nil {
			return p, []string{"serve"}, langID
		}
	}

	// 3. Python
	if langID == "python" {
		if p, err := exec.LookPath("pyright-langserver"); err == nil {
			return p, []string{"--stdio"}, langID
		}
		if p, err := exec.LookPath("pylsp"); err == nil {
			return p, nil, langID
		}
	}

	return "", nil, langID
}

// Manager coordinates language server lifecycles across the workspace.
type Manager struct {
	rootDir string
	mu      sync.Mutex
	clients map[string]*Client // Keyed by langID
}

// NewManager creates an LSP client manager for the workspace.
func NewManager(rootDir string) *Manager {
	return &Manager{
		rootDir: rootDir,
		clients: make(map[string]*Client),
	}
}

// GetClient returns or starts an LSP client for the given file extension.
func (m *Manager) GetClient(ext string) (*Client, string, error) {
	bin, args, langID := DetectLanguageServer(m.rootDir, ext)
	if bin == "" {
		return nil, langID, nil // No server available, graceful degradation
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	clientGroup := langID
	if strings.HasPrefix(langID, "javascript") || strings.HasPrefix(langID, "typescript") {
		clientGroup = "js-ts"
	}

	if c, ok := m.clients[clientGroup]; ok && !c.isClosed {
		return c, langID, nil
	}

	// Start new client
	client, err := StartClient(bin, args...)
	if err != nil {
		return nil, langID, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = client.Initialize(ctx, m.rootDir)
	m.clients[clientGroup] = client
	return client, langID, nil
}

// CloseAll shuts down all active language servers.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		_ = c.Close()
	}
	m.clients = make(map[string]*Client)
}
