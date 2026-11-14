package symbol

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

func init() {
	register(goExtractor{})
	register(jsExtractor{})
	register(phpExtractor{})
	register(jsonExtractor{})
	register(pyExtractor{})
	register(mdExtractor{})
}

func lineColOf(content []byte, offset int) (line, col int) {
	line = 1
	col = 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return
}

// ---------- Go (AST, akurat untuk deklarasi top-level) ----------

type goExtractor struct{}

func (goExtractor) Extensions() []string { return []string{".go"} }

func (goExtractor) Extract(path string, content []byte) []Symbol {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		return goRegexFallback(path, content)
	}
	var out []Symbol
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := KindFunc
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = KindMethod
				// sertakan receiver type agar unik dibaca
				if t := recvTypeName(d.Recv.List[0].Type); t != "" {
					name = t + "." + d.Name.Name
				}
			}
			pos := fset.Position(d.Name.Pos())
			out = append(out, Symbol{Name: name, Kind: kind, FilePath: path, Line: pos.Line, Col: pos.Column})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					pos := fset.Position(s.Name.Pos())
					out = append(out, Symbol{Name: s.Name.Name, Kind: KindType, FilePath: path, Line: pos.Line, Col: pos.Column})
				case *ast.ValueSpec:
					k := KindVar
					if d.Tok == token.CONST {
						k = KindConst
					}
					for _, n := range s.Names {
						if n.Name == "_" {
							continue
						}
						pos := fset.Position(n.Pos())
						out = append(out, Symbol{Name: n.Name, Kind: k, FilePath: path, Line: pos.Line, Col: pos.Column})
					}
				}
			}
		}
	}
	return out
}

func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return ""
}

func goRegexFallback(path string, content []byte) []Symbol {
	var out []Symbol
	re := regexp.MustCompile(`(?m)^\s*func\s+(?:\(\s*\w+\s+\*?\w+\s*\)\s*)?(\w+)\s*\(`)
	for _, loc := range re.FindAllSubmatchIndex(content, -1) {
		ns, ne := loc[2], loc[3]
		line, col := lineColOf(content, ns)
		out = append(out, Symbol{Name: string(content[ns:ne]), Kind: KindFunc, FilePath: path, Line: line, Col: col})
	}
	return out
}

// ---------- JavaScript / TypeScript ----------

type jsExtractor struct{}

func (jsExtractor) Extensions() []string {
	return []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
}

var jsPatterns = []struct {
	re   *regexp.Regexp
	kind SymbolKind
	grp  int
}{
	{regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s*\*?\s*(\w+)`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*(?:async\s+)?function\s*\*?\s*(\w+)`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?class\s+(\w+)`), KindClass, 1},
	{regexp.MustCompile(`(?m)^\s*class\s+(\w+)`), KindClass, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|\w+)\s*=>`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|\w+)\s*=>`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?function`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?function`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+(?:interface|type)\s+(\w+)`), KindType, 1},
	{regexp.MustCompile(`(?m)^\s*(?:interface|type)\s+(\w+)`), KindType, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+const\s+(\w+)`), KindConst, 1},
	{regexp.MustCompile(`(?m)^\s*export\s+(?:let|var)\s+(\w+)`), KindVar, 1},
}

func (jsExtractor) Extract(path string, content []byte) []Symbol {
	seen := map[string]bool{}
	var out []Symbol
	for _, p := range jsPatterns {
		for _, loc := range p.re.FindAllSubmatchIndex(content, -1) {
			ns, ne := loc[2*p.grp], loc[2*p.grp+1]
			name := string(content[ns:ne])
			key := name + ":" + string(p.kind)
			if seen[key] {
				continue
			}
			seen[key] = true
			line, col := lineColOf(content, ns)
			out = append(out, Symbol{Name: name, Kind: p.kind, FilePath: path, Line: line, Col: col})
		}
	}
	return out
}

// ---------- Python ----------

type pyExtractor struct{}

func (pyExtractor) Extensions() []string { return []string{".py"} }

var pyPatterns = []struct {
	re   *regexp.Regexp
	kind SymbolKind
}{
	{regexp.MustCompile(`(?m)^(\s*)def\s+(\w+)\s*\(`), KindFunc},
	{regexp.MustCompile(`(?m)^(\s*)class\s+(\w+)\s*[:\(]`), KindClass},
}

func (pyExtractor) Extract(path string, content []byte) []Symbol {
	var out []Symbol
	for _, p := range pyPatterns {
		for _, loc := range p.re.FindAllSubmatchIndex(content, -1) {
			indent := content[loc[2]:loc[3]]
			ns, ne := loc[4], loc[5]
			kind := p.kind
			if kind == KindFunc && len(indent) > 0 {
				kind = KindMethod
			}
			line, col := lineColOf(content, ns)
			out = append(out, Symbol{Name: string(content[ns:ne]), Kind: kind, FilePath: path, Line: line, Col: col})
		}
	}
	return out
}

// ---------- Markdown headings (berguna untuk "Go to Symbol") ----------

type mdExtractor struct{}

func (mdExtractor) Extensions() []string { return []string{".md", ".markdown"} }

var mdHeadingRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*#*\s*$`)

func (mdExtractor) Extract(path string, content []byte) []Symbol {
	var out []Symbol
	for _, loc := range mdHeadingRe.FindAllSubmatchIndex(content, -1) {
		ns, ne := loc[4], loc[5]
		line, col := lineColOf(content, ns)
		name := strings.TrimSpace(string(content[ns:ne]))
		out = append(out, Symbol{Name: name, Kind: KindHeading, FilePath: path, Line: line, Col: col})
	}
	return out
}

// ---------- PHP ----------
type phpExtractor struct{}

func (phpExtractor) Extensions() []string { return []string{".php", ".phtml"} }

var phpPatterns = []struct {
	re   *regexp.Regexp
	kind SymbolKind
	grp  int
}{
	{regexp.MustCompile(`(?m)^\s*(?:(?:final|abstract|readonly)\s+)*class\s+(\w+)`), KindClass, 1},
	{regexp.MustCompile(`(?m)^\s*interface\s+(\w+)`), KindClass, 1},
	{regexp.MustCompile(`(?m)^\s*trait\s+(\w+)`), KindClass, 1},
	{regexp.MustCompile(`(?m)^\s*(?:(?:public|protected|private|static|final|abstract)\s+)*function\s+(\w+)\s*\(`), KindFunc, 1},
	{regexp.MustCompile(`(?m)^\s*const\s+([A-Za-z0-9_]+)`), KindConst, 1},
	{regexp.MustCompile(`(?m)^\s*define\s*\(\s*['"](\w+)['"]`), KindConst, 1},
}

func (phpExtractor) Extract(path string, content []byte) []Symbol {
	seen := map[string]bool{}
	var out []Symbol
	for _, p := range phpPatterns {
		for _, loc := range p.re.FindAllSubmatchIndex(content, -1) {
			ns, ne := loc[2*p.grp], loc[2*p.grp+1]
			name := string(content[ns:ne])
			key := name + ":" + string(p.kind)
			if seen[key] {
				continue
			}
			seen[key] = true
			line, col := lineColOf(content, ns)
			out = append(out, Symbol{Name: name, Kind: p.kind, FilePath: path, Line: line, Col: col})
		}
	}
	return out
}

// ---------- JSON (depth <= 10) ----------
type jsonExtractor struct{}

func (jsonExtractor) Extensions() []string { return []string{".json"} }

func (jsonExtractor) Extract(path string, content []byte) []Symbol {
	var out []Symbol
	n := len(content)
	i := 0
	type containerFrame struct {
		hasKey bool
	}
	var containerStack []containerFrame
	var keyPathStack []string
	pendingKey := ""

	skipWS := func() {
		for i < n {
			ch := content[i]
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				i++
			} else if ch == '/' && i+1 < n && content[i+1] == '/' {
				i += 2
				for i < n && content[i] != '\n' {
					i++
				}
			} else if ch == '/' && i+1 < n && content[i+1] == '*' {
				i += 2
				for i+1 < n && !(content[i] == '*' && content[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				}
			} else {
				break
			}
		}
	}

	for i < n {
		skipWS()
		if i >= n {
			break
		}
		ch := content[i]
		switch ch {
		case '{', '[':
			hasKey := false
			if pendingKey != "" {
				keyPathStack = append(keyPathStack, pendingKey)
				hasKey = true
				pendingKey = ""
			}
			containerStack = append(containerStack, containerFrame{hasKey: hasKey})
			i++
		case '}', ']':
			pendingKey = ""
			if len(containerStack) > 0 {
				top := containerStack[len(containerStack)-1]
				containerStack = containerStack[:len(containerStack)-1]
				if top.hasKey && len(keyPathStack) > 0 {
					keyPathStack = keyPathStack[:len(keyPathStack)-1]
				}
			}
			i++
		case '"':
			startOffset := i + 1
			i++
			escaped := false
			for i < n {
				if escaped {
					escaped = false
					i++
					continue
				}
				if content[i] == '\\' {
					escaped = true
					i++
					continue
				}
				if content[i] == '"' {
					break
				}
				i++
			}
			endOffset := i
			if i < n && content[i] == '"' {
				i++
			}
			val := string(content[startOffset:endOffset])

			peek := i
			for peek < n && (content[peek] == ' ' || content[peek] == '\t' || content[peek] == '\n' || content[peek] == '\r') {
				peek++
			}
			if peek < n && content[peek] == ':' {
				i = peek + 1 // skip ':'
				depth := len(containerStack)
				if depth <= 10 {
					line, col := lineColOf(content, startOffset)
					out = append(out, Symbol{
						Name:     val,
						Kind:     KindKey,
						FilePath: path,
						Line:     line,
						Col:      col,
					})

					if len(keyPathStack) > 0 {
						fullPath := append(append([]string(nil), keyPathStack...), val)
						dotted := strings.Join(fullPath, ".")
						if dotted != val {
							out = append(out, Symbol{
								Name:     dotted,
								Kind:     KindKey,
								FilePath: path,
								Line:     line,
								Col:      col,
							})
						}
					}
				}

				skipWS()
				if i < n && (content[i] == '{' || content[i] == '[') {
					pendingKey = val
				} else {
					pendingKey = ""
				}
			} else {
				pendingKey = ""
			}
		default:
			pendingKey = ""
			i++
		}
	}
	return out
}

var _ = bytes.IndexByte
var _ = json.Valid
