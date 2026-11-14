package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPathToURIAndBack(t *testing.T) {
	path := "/home/user/project/src/App.jsx"
	uri := PathToURI(path)
	if !strings.HasPrefix(uri, "file://") {
		t.Errorf("PathToURI(%s) want file:// prefix, got %s", path, uri)
	}

	back := URIToPath(uri)
	if back != path {
		t.Errorf("URIToPath(%s) = %s, want %s", uri, back, path)
	}
}

func TestProtocolSerialization(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "textDocument/completion",
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal request error: %v", err)
	}

	if !strings.Contains(string(bytes), `"method":"textDocument/completion"`) {
		t.Errorf("Serialized request missing method: %s", string(bytes))
	}
}

func TestCompletionPopupNavigation(t *testing.T) {
	popup := NewCompletionPopup()
	items := []CompletionItem{
		{Label: "useState", Kind: CompletionKindFunction, Detail: "React Hook"},
		{Label: "useEffect", Kind: CompletionKindFunction, Detail: "React Hook"},
		{Label: "useMemo", Kind: CompletionKindFunction, Detail: "React Hook"},
	}

	popup.SetItems(items, 10, 5, "use")
	if !popup.Active {
		t.Fatalf("popup should be active")
	}

	if popup.Selected().Label != "useState" {
		t.Errorf("Selected() want useState, got %s", popup.Selected().Label)
	}

	popup.MoveDown()
	if popup.Selected().Label != "useEffect" {
		t.Errorf("Selected() after MoveDown want useEffect, got %s", popup.Selected().Label)
	}

	popup.MoveUp()
	if popup.Selected().Label != "useState" {
		t.Errorf("Selected() after MoveUp want useState, got %s", popup.Selected().Label)
	}

	rendered := popup.Render(80, 24)
	if !strings.Contains(rendered, "useState") || !strings.Contains(rendered, "React Hook") {
		t.Errorf("Render() missing items, got:\n%s", rendered)
	}
}
