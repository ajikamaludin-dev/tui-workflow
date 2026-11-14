package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Client represents a JSON-RPC 2.0 stdio LSP client connected to a language server.
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	reqID     int64
	pendingMu sync.Mutex
	pending   map[int64]chan *Response
	isClosed  bool
	closeMu   sync.Mutex
	doneChan  chan struct{}
}

// StartClient launches the language server process and starts the message loop.
func StartClient(binaryPath string, args ...string) (*Client, error) {
	cmd := exec.Command(binaryPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe error: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("stdout pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdoutPipe.Close()
		return nil, fmt.Errorf("start server error: %w", err)
	}

	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdoutPipe),
		pending:  make(map[int64]chan *Response),
		doneChan: make(chan struct{}),
	}

	go c.listenLoop()
	return c, nil
}

// Close gracefully terminates the language server.
func (c *Client) Close() error {
	c.closeMu.Lock()
	if c.isClosed {
		c.closeMu.Unlock()
		return nil
	}
	c.isClosed = true
	c.closeMu.Unlock()

	close(c.doneChan)
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

// listenLoop reads JSON-RPC frames from stdout.
func (c *Client) listenLoop() {
	for {
		select {
		case <-c.doneChan:
			return
		default:
		}

		contentLength := 0
		for {
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				// Header finished, body begins
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					val, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
					contentLength = val
				}
			}
		}

		if contentLength <= 0 {
			continue
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			return
		}

		// Try parsing as Response
		var resp Response
		if err := json.Unmarshal(body, &resp); err == nil && resp.ID != 0 {
			c.pendingMu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.pendingMu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}
}

// sendRaw writes a formatted JSON-RPC message to stdin.
func (c *Client) sendRaw(payload []byte) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.isClosed {
		return fmt.Errorf("client closed")
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(payload); err != nil {
		return err
	}
	return nil
}

// Call sends a request and waits for a response with a timeout.
func (c *Client) Call(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := atomic.AddInt64(&c.reqID, 1)

	var rawParams json.RawMessage
	if params != nil {
		bytes, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		rawParams = bytes
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	respChan := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respChan
	c.pendingMu.Unlock()

	if err := c.sendRaw(payload); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-c.doneChan:
		return nil, fmt.Errorf("client closed")
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("lsp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	}
}

// Notify sends a notification without expecting a response.
func (c *Client) Notify(method string, params interface{}) error {
	var rawParams json.RawMessage
	if params != nil {
		bytes, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = bytes
	}

	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	}

	payload, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return c.sendRaw(payload)
}

// PathToURI converts a file system path to file:// URI.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

// URIToPath converts a file:// URI to file system path.
func URIToPath(uri string) string {
	clean := strings.TrimPrefix(uri, "file://")
	return filepath.FromSlash(clean)
}

// Initialize performs LSP handshake.
func (c *Client) Initialize(ctx context.Context, rootDir string) error {
	params := InitializeParams{
		ProcessID: 0,
		RootURI:   PathToURI(rootDir),
		RootPath:  rootDir,
	}
	_, err := c.Call(ctx, "initialize", params)
	if err != nil {
		return err
	}
	return c.Notify("initialized", map[string]interface{}{})
}

// DidOpen notifies server that a file is opened.
func (c *Client) DidOpen(filePath, langID, content string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        PathToURI(filePath),
			LanguageID: langID,
			Version:    1,
			Text:       content,
		},
	}
	return c.Notify("textDocument/didOpen", params)
}

// DidChange notifies server that document content changed.
func (c *Client) DidChange(filePath string, version int, content string) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     PathToURI(filePath),
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: content},
		},
	}
	return c.Notify("textDocument/didChange", params)
}

// Completion queries code completions at (line0, col0).
func (c *Client) Completion(ctx context.Context, filePath string, line0, col0 int) ([]CompletionItem, error) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: PathToURI(filePath)},
		Position:     Position{Line: line0, Character: col0},
	}

	resp, err := c.Call(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	// Result can be []CompletionItem or CompletionList
	var list CompletionList
	if err := json.Unmarshal(resp.Result, &list); err == nil && len(list.Items) > 0 {
		return list.Items, nil
	}

	var items []CompletionItem
	if err := json.Unmarshal(resp.Result, &items); err == nil {
		return items, nil
	}

	return nil, nil
}

// Definition queries symbol definition location at (line0, col0).
func (c *Client) Definition(ctx context.Context, filePath string, line0, col0 int) ([]Location, error) {
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: PathToURI(filePath)},
		Position:     Position{Line: line0, Character: col0},
	}

	resp, err := c.Call(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	// Result can be Location or []Location
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err == nil && loc.URI != "" {
		return []Location{loc}, nil
	}

	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err == nil {
		return locs, nil
	}

	return nil, nil
}
