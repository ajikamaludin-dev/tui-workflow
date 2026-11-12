package formatter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTimeout is the maximum duration for a formatter CLI execution.
const DefaultTimeout = 4 * time.Second

// ToolCandidate defines a CLI formatter candidate with arguments and optional local path.
type ToolCandidate struct {
	Name      string
	Cmd       string
	Args      []string
	LocalPath string // Relative to workspace root, e.g. "./node_modules/.bin/biome"
}

// FormatRequest holds all parameters for a format operation.
type FormatRequest struct {
	Content         string
	FilePath        string
	WorkspaceDir    string
	CustomOverrides map[string][]string // extension -> ["command", "args..."]
}

// FormatResult holds the formatted output and the tool that successfully formatted it.
type FormatResult struct {
	Formatted string
	ToolName  string
}

// DetectExtension extracts the extension, prioritizing multi-part extensions like .blade.php.
func DetectExtension(filePath string) string {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".blade.php") {
		return ".blade.php"
	}
	return strings.ToLower(filepath.Ext(filePath))
}

// GetFallbackChain returns the list of candidate tools for a given extension.
func GetFallbackChain(ext, filePath string) []ToolCandidate {
	switch ext {
	case ".jsx", ".tsx", ".js", ".ts":
		return []ToolCandidate{
			{
				Name:      "biome (local)",
				Cmd:       "./node_modules/.bin/biome",
				Args:      []string{"format", "--stdin-file-path", filePath},
				LocalPath: "./node_modules/.bin/biome",
			},
			{
				Name:      "prettier (local)",
				Cmd:       "./node_modules/.bin/prettier",
				Args:      []string{"--stdin-filepath", filePath},
				LocalPath: "./node_modules/.bin/prettier",
			},
			{
				Name: "biome",
				Cmd:  "biome",
				Args: []string{"format", "--stdin-file-path", filePath},
			},
			{
				Name: "prettier",
				Cmd:  "prettier",
				Args: []string{"--stdin-filepath", filePath},
			},
			{
				Name: "deno",
				Cmd:  "deno",
				Args: []string{"fmt", "-"},
			},
		}

	case ".blade.php":
		return []ToolCandidate{
			{
				Name: "blade-formatter",
				Cmd:  "blade-formatter",
				Args: []string{"--stdin"},
			},
			{
				Name:      "pint (local)",
				Cmd:       "./vendor/bin/pint",
				Args:      []string{"-q", "-"},
				LocalPath: "./vendor/bin/pint",
			},
			{
				Name: "pint",
				Cmd:  "pint",
				Args: []string{"-q", "-"},
			},
		}

	case ".php":
		return []ToolCandidate{
			{
				Name:      "pint (local)",
				Cmd:       "./vendor/bin/pint",
				Args:      []string{"-q", "-"},
				LocalPath: "./vendor/bin/pint",
			},
			{
				Name: "pint",
				Cmd:  "pint",
				Args: []string{"-q", "-"},
			},
			{
				Name: "php-cs-fixer",
				Cmd:  "php-cs-fixer",
				Args: []string{"fix", "--rules=@PSR12", "-q", "-"},
			},
		}

	case ".go":
		return []ToolCandidate{
			{
				Name: "gofmt",
				Cmd:  "gofmt",
				Args: nil,
			},
			{
				Name: "goimports",
				Cmd:  "goimports",
				Args: nil,
			},
		}

	case ".py":
		return []ToolCandidate{
			{
				Name: "ruff",
				Cmd:  "ruff",
				Args: []string{"format", "-"},
			},
			{
				Name: "black",
				Cmd:  "black",
				Args: []string{"-q", "-"},
			},
			{
				Name: "autopep8",
				Cmd:  "autopep8",
				Args: []string{"-"},
			},
		}

	case ".rs":
		return []ToolCandidate{
			{
				Name: "rustfmt",
				Cmd:  "rustfmt",
				Args: nil,
			},
		}

	case ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx", ".java", ".cs":
		return []ToolCandidate{
			{
				Name: "clang-format",
				Cmd:  "clang-format",
				Args: nil,
			},
		}

	case ".sh", ".bash":
		return []ToolCandidate{
			{
				Name: "shfmt",
				Cmd:  "shfmt",
				Args: nil,
			},
		}

	case ".html", ".css", ".scss", ".less", ".json", ".yaml", ".yml", ".md", ".markdown":
		return []ToolCandidate{
			{
				Name:      "biome (local)",
				Cmd:       "./node_modules/.bin/biome",
				Args:      []string{"format", "--stdin-file-path", filePath},
				LocalPath: "./node_modules/.bin/biome",
			},
			{
				Name:      "prettier (local)",
				Cmd:       "./node_modules/.bin/prettier",
				Args:      []string{"--stdin-filepath", filePath},
				LocalPath: "./node_modules/.bin/prettier",
			},
			{
				Name: "biome",
				Cmd:  "biome",
				Args: []string{"format", "--stdin-file-path", filePath},
			},
			{
				Name: "prettier",
				Cmd:  "prettier",
				Args: []string{"--stdin-filepath", filePath},
			},
		}

	case ".lua":
		return []ToolCandidate{
			{
				Name: "stylua",
				Cmd:  "stylua",
				Args: []string{"-"},
			},
		}

	case ".sql":
		return []ToolCandidate{
			{
				Name: "sql-formatter",
				Cmd:  "sql-formatter",
				Args: nil,
			},
		}

	default:
		return nil
	}
}

// resolveExecutable checks if tool can be executed, returning the full path.
func resolveExecutable(candidate ToolCandidate, workspaceDir string) (string, bool) {
	if candidate.LocalPath != "" && workspaceDir != "" {
		localTarget := filepath.Join(workspaceDir, filepath.FromSlash(candidate.LocalPath))
		if info, err := os.Stat(localTarget); err == nil && !info.IsDir() {
			// Check if executable (on unix)
			if info.Mode()&0111 != 0 || filepath.Ext(localTarget) == ".exe" {
				return localTarget, true
			}
		}
	}

	binPath, err := exec.LookPath(candidate.Cmd)
	if err == nil {
		return binPath, true
	}

	return "", false
}

// Format formats the provided content according to file type, custom overrides, and fallback chains.
func Format(req FormatRequest) (*FormatResult, error) {
	ext := DetectExtension(req.FilePath)
	if ext == "" {
		return nil, errors.New("cannot format: file has no extension")
	}

	// 1. Check custom override from config
	if req.CustomOverrides != nil {
		if customCmd, ok := req.CustomOverrides[ext]; ok && len(customCmd) > 0 {
			cmdName := customCmd[0]
			var args []string
			for _, arg := range customCmd[1:] {
				if arg == "{file}" {
					args = append(args, req.FilePath)
				} else {
					args = append(args, arg)
				}
			}

			binPath, err := exec.LookPath(cmdName)
			if err != nil {
				return nil, fmt.Errorf("custom formatter '%s' not found in PATH", cmdName)
			}

			formatted, err := runCLI(binPath, args, req.Content, req.WorkspaceDir)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", cmdName, err)
			}
			return &FormatResult{
				Formatted: formatted,
				ToolName:  cmdName,
			}, nil
		}
	}

	// 2. Fallback Chain
	candidates := GetFallbackChain(ext, req.FilePath)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no formatter registered for '%s' files", ext)
	}

	var triedNames []string
	for _, cand := range candidates {
		binPath, ok := resolveExecutable(cand, req.WorkspaceDir)
		if !ok {
			triedNames = append(triedNames, cand.Name)
			continue
		}

		formatted, err := runCLI(binPath, cand.Args, req.Content, req.WorkspaceDir)
		if err != nil {
			// If it was found and executed but failed (e.g. syntax error in code), report the error
			return nil, fmt.Errorf("%s: %w", cand.Name, err)
		}

		return &FormatResult{
			Formatted: formatted,
			ToolName:  cand.Name,
		}, nil
	}

	return nil, fmt.Errorf("formatter not found in PATH (tried: %s)", strings.Join(triedNames, ", "))
}

// runCLI executes a formatter binary via stdin/stdout piping with timeout.
func runCLI(binPath string, args []string, input string, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	cmd.Stdin = strings.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("timed out after 4s")
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			// Return clean first 1-2 lines of error
			lines := strings.Split(errMsg, "\n")
			if len(lines) > 2 {
				errMsg = lines[0] + " " + lines[1]
			}
			return "", errors.New(errMsg)
		}
		return "", err
	}

	return stdout.String(), nil
}
