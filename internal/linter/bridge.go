package linter

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LintFinishedMsg is sent when a linter run finishes.
type LintFinishedMsg struct {
	FilePath    string
	Diagnostics []Diagnostic
	Err         error
}

// ESLintMessage represents a single message in ESLint's JSON format.
type ESLintMessage struct {
	RuleID   string `json:"ruleId"`
	Severity int    `json:"severity"` // 1: warning, 2: error
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// ESLintFileReport represents a file entry in ESLint's JSON format.
type ESLintFileReport struct {
	FilePath string          `json:"filePath"`
	Messages []ESLintMessage `json:"messages"`
}

// FindLinterExecutable returns the path to eslint or biome in workspace or PATH.
func FindLinterExecutable(rootDir string) (string, string) {
	// 1. Local workspace node_modules
	localESLint := filepath.Join(rootDir, "node_modules", ".bin", "eslint")
	if info, err := os.Stat(localESLint); err == nil && !info.IsDir() {
		return localESLint, "eslint"
	}
	localBiome := filepath.Join(rootDir, "node_modules", ".bin", "biome")
	if info, err := os.Stat(localBiome); err == nil && !info.IsDir() {
		return localBiome, "biome"
	}

	// 2. PATH lookups
	if p, err := exec.LookPath("eslint"); err == nil {
		return p, "eslint"
	}
	if p, err := exec.LookPath("biome"); err == nil {
		return p, "biome"
	}

	return "", ""
}

// ParseESLintJSON parses raw ESLint JSON output into Diagnostics.
func ParseESLintJSON(rawOutput []byte, targetPath string) []Diagnostic {
	var reports []ESLintFileReport
	if err := json.Unmarshal(rawOutput, &reports); err != nil {
		return nil
	}

	var res []Diagnostic
	for _, rep := range reports {
		for _, msg := range rep.Messages {
			sev := SeverityWarning
			if msg.Severity == 2 {
				sev = SeverityError
			}
			res = append(res, Diagnostic{
				FilePath: targetPath,
				Line:     msg.Line,
				Col:      msg.Column,
				Severity: sev,
				Message:  msg.Message,
				RuleID:   msg.RuleID,
				Source:   "eslint",
			})
		}
	}
	return res
}

// RunLinterCmd executes the linter in the background as a tea.Cmd.
func RunLinterCmd(rootDir, targetPath string) tea.Cmd {
	return func() tea.Msg {
		bin, linterType := FindLinterExecutable(rootDir)
		if bin == "" {
			return LintFinishedMsg{FilePath: targetPath, Diagnostics: nil}
		}

		ext := strings.ToLower(filepath.Ext(targetPath))
		if ext != ".js" && ext != ".jsx" && ext != ".ts" && ext != ".tsx" && ext != ".mjs" && ext != ".cjs" {
			return LintFinishedMsg{FilePath: targetPath, Diagnostics: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var cmd *exec.Cmd
		if linterType == "eslint" {
			cmd = exec.CommandContext(ctx, bin, "--format", "json", targetPath)
		} else {
			cmd = exec.CommandContext(ctx, bin, "lint", "--reporter=json", targetPath)
		}
		cmd.Dir = rootDir

		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf

		_ = cmd.Run() // ESLint returns exit code 1 if errors found, which is normal

		diags := ParseESLintJSON(outBuf.Bytes(), targetPath)
		return LintFinishedMsg{
			FilePath:    targetPath,
			Diagnostics: diags,
		}
	}
}
