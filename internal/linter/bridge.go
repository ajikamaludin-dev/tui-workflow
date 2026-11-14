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

	tea "charm.land/bubbletea/v2"
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

// BiomeDiagnosticMessage represents a formatted content item in Biome JSON.
type BiomeDiagnosticMessage struct {
	Content string `json:"content"`
}

// BiomeDiagnosticPosition represents line and column in Biome JSON.
type BiomeDiagnosticPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// BiomeLocation represents code location in Biome JSON output.
type BiomeLocation struct {
	Path struct {
		File string `json:"file"`
	} `json:"path"`
	Start    BiomeDiagnosticPosition `json:"start"`
	Position BiomeDiagnosticPosition `json:"position"`
	Line     int                     `json:"line"`
	Column   int                     `json:"column"`
}

// BiomeDiagnostic represents an individual diagnostic item in Biome JSON.
type BiomeDiagnostic struct {
	Category    string                   `json:"category"`
	Severity    string                   `json:"severity"`
	Description string                   `json:"description"`
	Message     []BiomeDiagnosticMessage `json:"message"`
	Location    BiomeLocation            `json:"location"`
}

// BiomeReport represents top-level structure of Biome's JSON report.
type BiomeReport struct {
	Diagnostics []BiomeDiagnostic `json:"diagnostics"`
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

// ParseBiomeJSON parses raw Biome JSON output into Diagnostics.
func ParseBiomeJSON(rawOutput []byte, targetPath string) []Diagnostic {
	var report BiomeReport
	var diags []BiomeDiagnostic
	if err := json.Unmarshal(rawOutput, &report); err == nil && len(report.Diagnostics) > 0 {
		diags = report.Diagnostics
	} else if err := json.Unmarshal(rawOutput, &diags); err != nil {
		return nil
	}

	var res []Diagnostic
	for _, d := range diags {
		sev := SeverityWarning
		s := strings.ToLower(d.Severity)
		if s == "error" || s == "fatal" {
			sev = SeverityError
		}

		line := d.Location.Start.Line
		if line == 0 {
			line = d.Location.Position.Line
		}
		if line == 0 {
			line = d.Location.Line
		}

		col := d.Location.Start.Column
		if col == 0 {
			col = d.Location.Position.Column
		}
		if col == 0 {
			col = d.Location.Column
		}

		msgText := d.Description
		if msgText == "" && len(d.Message) > 0 {
			var parts []string
			for _, m := range d.Message {
				if m.Content != "" {
					parts = append(parts, m.Content)
				}
			}
			msgText = strings.Join(parts, " ")
		}
		if msgText == "" {
			msgText = d.Category
		}

		fp := targetPath
		if d.Location.Path.File != "" {
			fp = d.Location.Path.File
		}

		res = append(res, Diagnostic{
			FilePath: fp,
			Line:     line,
			Col:      col,
			Severity: sev,
			Message:  msgText,
			RuleID:   d.Category,
			Source:   "biome",
		})
	}
	return res
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

		var diags []Diagnostic
		if linterType == "biome" {
			diags = ParseBiomeJSON(outBuf.Bytes(), targetPath)
		} else {
			diags = ParseESLintJSON(outBuf.Bytes(), targetPath)
		}
		return LintFinishedMsg{
			FilePath:    targetPath,
			Diagnostics: diags,
		}
	}
}
