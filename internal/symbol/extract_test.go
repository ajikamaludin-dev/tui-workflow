package symbol

import (
	"testing"
)

func TestExtractGo(t *testing.T) {
	src := `package main

type MyStruct struct{}

const Pi = 3.14
var Counter = 0

func Hello() {}

func (m *MyStruct) DoWork() {}
`
	e := extractorFor(".go")
	if e == nil {
		t.Fatal("no extractor for .go")
	}
	syms := e.Extract("main.go", []byte(src))
	names := map[string]SymbolKind{}
	for _, s := range syms {
		names[s.Name] = s.Kind
	}

	expected := map[string]SymbolKind{
		"MyStruct":        KindType,
		"Pi":              KindConst,
		"Counter":         KindVar,
		"Hello":           KindFunc,
		"MyStruct.DoWork": KindMethod,
	}

	for expName, expKind := range expected {
		kind, ok := names[expName]
		if !ok {
			t.Errorf("expected Go symbol %q not found", expName)
		} else if kind != expKind {
			t.Errorf("symbol %q expected kind %s, got %s", expName, expKind, kind)
		}
	}
}

func TestExtractJS(t *testing.T) {
	src := `export function greet(name) {}
const calculateTotal = (a, b) => a + b;
export class UserComponent {}
export interface UserProps {}
export type ID = string;
export const VERSION = "1.0.0";
`
	e := extractorFor(".tsx")
	if e == nil {
		t.Fatal("no extractor for .tsx")
	}
	syms := e.Extract("app.tsx", []byte(src))
	names := map[string]SymbolKind{}
	for _, s := range syms {
		names[s.Name] = s.Kind
	}

	expected := map[string]SymbolKind{
		"greet":          KindFunc,
		"calculateTotal": KindFunc,
		"UserComponent":  KindClass,
		"UserProps":      KindType,
		"ID":             KindType,
		"VERSION":        KindConst,
	}

	for expName, expKind := range expected {
		kind, ok := names[expName]
		if !ok {
			t.Errorf("expected JS/TS symbol %q not found", expName)
		} else if kind != expKind {
			t.Errorf("symbol %q expected kind %s, got %s", expName, expKind, kind)
		}
	}
}

func TestExtractPHP(t *testing.T) {
	src := `<?php
class DatabaseConnection {
    const TIMEOUT = 30;
    public function connect() {}
}
interface QueryInterface {}
trait LoggerTrait {}
function standaloneFunc() {}
define("APP_ENV", "testing");
`
	e := extractorFor(".php")
	if e == nil {
		t.Fatal("no extractor for .php")
	}
	syms := e.Extract("db.php", []byte(src))
	names := map[string]SymbolKind{}
	for _, s := range syms {
		names[s.Name] = s.Kind
	}

	expected := map[string]SymbolKind{
		"DatabaseConnection": KindClass,
		"TIMEOUT":            KindConst,
		"connect":            KindFunc,
		"QueryInterface":     KindClass,
		"LoggerTrait":        KindClass,
		"standaloneFunc":     KindFunc,
		"APP_ENV":            KindConst,
	}

	for expName, expKind := range expected {
		kind, ok := names[expName]
		if !ok {
			t.Errorf("expected PHP symbol %q not found", expName)
		} else if kind != expKind {
			t.Errorf("symbol %q expected kind %s, got %s", expName, expKind, kind)
		}
	}
}

func TestExtractJSONDepth(t *testing.T) {
	// JSON with depth 1 to 12
	src := `{
  "d1": {
    "d2": {
      "d3": {
        "d4": {
          "d5": {
            "d6": {
              "d7": {
                "d8": {
                  "d9": {
                    "d10": {
                      "d11": {
                        "d12": "too deep"
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`
	e := extractorFor(".json")
	if e == nil {
		t.Fatal("no extractor for .json")
	}
	syms := e.Extract("config.json", []byte(src))
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
		if s.Kind != KindKey {
			t.Errorf("JSON symbol %s expected KindKey, got %s", s.Name, s.Kind)
		}
	}

	// d1 through d10 should exist
	for i := 1; i <= 10; i++ {
		key := "d" + string(rune('0'+i/10*10+i%10))
		if i == 10 {
			key = "d10"
		}
		if !names[key] {
			t.Errorf("expected JSON key %q at depth %d to be indexed", key, i)
		}
	}

	// d11 and d12 should NOT exist (depth > 10)
	if names["d11"] {
		t.Errorf("key d11 at depth 11 should not be indexed")
	}
	if names["d12"] {
		t.Errorf("key d12 at depth 12 should not be indexed")
	}

	// Dotted path check for nested keys
	if !names["d1.d2"] {
		t.Errorf("expected dotted key d1.d2 to be indexed")
	}
}

func TestExtractJSONArrayOfObjects(t *testing.T) {
	// JSON with arrays of objects (like settings-version.json)
	src := `{
  "appliedAt": "2026-11-11",
  "log": [
    {
      "action": "skipped",
      "from": "path/a"
    },
    {
      "action": "migrated",
      "nested": {
        "ok": true
      }
    }
  ]
}`
	e := extractorFor(".json")
	if e == nil {
		t.Fatal("no extractor for .json")
	}
	syms := e.Extract("test.json", []byte(src))
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}

	expected := []string{"appliedAt", "log", "action", "log.action", "from", "log.from", "nested", "log.nested", "ok", "log.nested.ok"}
	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("expected JSON symbol %q not found in extracted: %v", exp, names)
		}
	}
}

func TestExtractMarkdown(t *testing.T) {
	src := `# Title
Some text
## Installation
More text
### Configuration Options
`
	e := extractorFor(".md")
	if e == nil {
		t.Fatal("no extractor for .md")
	}
	syms := e.Extract("README.md", []byte(src))
	if len(syms) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(syms))
	}
	if syms[0].Name != "Title" || syms[0].Kind != KindHeading {
		t.Errorf("heading 0 mismatch: %+v", syms[0])
	}
	if syms[1].Name != "Installation" || syms[1].Kind != KindHeading {
		t.Errorf("heading 1 mismatch: %+v", syms[1])
	}
	if syms[2].Name != "Configuration Options" || syms[2].Kind != KindHeading {
		t.Errorf("heading 2 mismatch: %+v", syms[2])
	}
}
