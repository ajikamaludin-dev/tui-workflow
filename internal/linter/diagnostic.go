package linter

// Severity indicates the importance of a diagnostic issue.
type Severity int

const (
	SeverityInfo Severity = iota + 1
	SeverityWarning
	SeverityError
)

// Diagnostic represents a single linting or type check issue.
type Diagnostic struct {
	FilePath string   `json:"filePath"`
	Line     int      `json:"line"`     // 1-based line number
	Col      int      `json:"col"`      // 1-based column number
	Severity Severity `json:"severity"` // 1: Info, 2: Warning, 3: Error
	Message  string   `json:"message"`
	RuleID   string   `json:"ruleId"`
	Source   string   `json:"source"` // e.g. "eslint", "biome"
}

// Marker returns a colored visual dot symbol for gutter rendering.
func (d Diagnostic) Marker() string {
	switch d.Severity {
	case SeverityError:
		return "\x1b[38;2;224;108;117m●\x1b[0m" // Red ●
	case SeverityWarning:
		return "\x1b[38;2;229;192;123m▲\x1b[0m" // Yellow ▲
	default:
		return "\x1b[38;2;97;175;239mℹ\x1b[0m" // Blue ℹ
	}
}
