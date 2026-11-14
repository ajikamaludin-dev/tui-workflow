package snippet

import (
	"path/filepath"
	"strings"
)

// Snippet represents a reusable code template with tab-stops.
type Snippet struct {
	Prefix      string   // Trigger keyword, e.g. "rafce"
	Description string   // Human-readable description
	Body        []string // Multi-line template body with $1, ${1:default}, $0
	Languages   []string // Supported languages / extensions (empty = all)
}

// Built-in snippets catalog (React, JS/TS, Go)
var DefaultSnippets = []Snippet{
	// React: rafce
	{
		Prefix:      "rafce",
		Description: "React Arrow Function Component with Export",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"import React from 'react';",
			"",
			"export const ${1:ComponentName} = () => {",
			"  return (",
			"    <div>",
			"      ${2:content}$0",
			"    </div>",
			"  );",
			"};",
			"",
			"export default ${1:ComponentName};",
		},
	},
	// React: rfce
	{
		Prefix:      "rfce",
		Description: "React Function Component with Export",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"import React from 'react';",
			"",
			"export function ${1:ComponentName}() {",
			"  return (",
			"    <div>",
			"      ${2:content}",
			"    </div>",
			"  );",
			"}",
			"",
			"export default ${1:ComponentName};",
		},
	},
	// React: rcc
	{
		Prefix:      "rcc",
		Description: "React Class Component with Export",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"import React, { Component } from 'react';",
			"",
			"export class ${1:ComponentName} extends Component {",
			"  render() {",
			"    return (",
			"      <div>",
			"        ${2:content}",
			"      </div>",
			"    );",
			"  }",
			"}",
			"",
			"export default ${1:ComponentName};",
		},
	},
	// React: useState
	{
		Prefix:      "useState",
		Description: "React useState hook",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"const [${1:state}, set${2:State}] = useState(${3:initialState});",
		},
	},
	// React: useEffect
	{
		Prefix:      "useEffect",
		Description: "React useEffect hook",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"useEffect(() => {",
			"  $0",
			"}, [${1:dependencies}]);",
		},
	},
	// React: useCallback
	{
		Prefix:      "useCallback",
		Description: "React useCallback hook",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"useCallback(() => {",
			"  $0",
			"}, [${1:dependencies}]);",
		},
	},
	// React: useMemo
	{
		Prefix:      "useMemo",
		Description: "React useMemo hook",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"useMemo(() => {",
			"  return ${1:value};",
			"}, [${2:dependencies}]);",
		},
	},
	// React: useRef
	{
		Prefix:      "useRef",
		Description: "React useRef hook",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"const ${1:ref} = useRef(${2:null});",
		},
	},
	// JS: console.log
	{
		Prefix:      "clg",
		Description: "console.log()",
		Languages:   []string{".jsx", ".tsx", ".js", ".ts"},
		Body: []string{
			"console.log(${1:object});",
		},
	},
	// Go: err != nil
	{
		Prefix:      "errn",
		Description: "if err != nil { return err }",
		Languages:   []string{".go"},
		Body: []string{
			"if err != nil {",
			"  return ${1:err}",
			"}",
		},
	},
	// Go: func
	{
		Prefix:      "fn",
		Description: "func Name() { ... }",
		Languages:   []string{".go"},
		Body: []string{
			"func ${1:name}(${2:params}) ${3:error} {",
			"  $0",
			"}",
		},
	},
}

// FindSnippet searches for a matching snippet by prefix and file extension.
func FindSnippet(prefix, filename string) *Snippet {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, s := range DefaultSnippets {
		if s.Prefix != prefix {
			continue
		}
		if len(s.Languages) == 0 {
			return &s
		}
		for _, lang := range s.Languages {
			if lang == ext {
				return &s
			}
		}
	}
	return nil
}

// BaseFilename extracts clean component name from filename (e.g. "UserProfile.jsx" -> "UserProfile")
func BaseFilename(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return "Component"
	}
	// Sanitize to valid identifier
	var sb strings.Builder
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else if i == 0 {
			sb.WriteString("Comp")
		}
	}
	res := sb.String()
	if res == "" {
		return "Component"
	}
	return res
}
