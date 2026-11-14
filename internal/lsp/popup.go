package lsp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

// CompletionPopup manages the floating autocomplete dropdown UI.
type CompletionPopup struct {
	Active        bool
	Items         []CompletionItem
	SelectedIndex int
	ScrollOffset  int
	X, Y          int // Screen coordinates
	Prefix        string
}

// NewCompletionPopup creates a new inactive completion popup.
func NewCompletionPopup() *CompletionPopup {
	return &CompletionPopup{
		Active: false,
	}
}

// SetItems sets items and resets selection.
func (p *CompletionPopup) SetItems(items []CompletionItem, x, y int, prefix string) {
	p.Items = items
	p.SelectedIndex = 0
	p.ScrollOffset = 0
	p.X = x
	p.Y = y
	p.Prefix = prefix
	p.Active = len(items) > 0
}

// MoveDown advances selection down.
func (p *CompletionPopup) MoveDown() {
	if !p.Active || len(p.Items) == 0 {
		return
	}
	if p.SelectedIndex < len(p.Items)-1 {
		p.SelectedIndex++
		if p.SelectedIndex >= p.ScrollOffset+7 {
			p.ScrollOffset = p.SelectedIndex - 6
		}
	}
}

// MoveUp advances selection up.
func (p *CompletionPopup) MoveUp() {
	if !p.Active || len(p.Items) == 0 {
		return
	}
	if p.SelectedIndex > 0 {
		p.SelectedIndex--
		if p.SelectedIndex < p.ScrollOffset {
			p.ScrollOffset = p.SelectedIndex
		}
	}
}

// Selected returns the currently selected completion item.
func (p *CompletionPopup) Selected() *CompletionItem {
	if !p.Active || p.SelectedIndex < 0 || p.SelectedIndex >= len(p.Items) {
		return nil
	}
	return &p.Items[p.SelectedIndex]
}

// Close hides the popup.
func (p *CompletionPopup) Close() {
	p.Active = false
	p.Items = nil
}

// KindIcon returns a compact symbol representing the completion kind.
func KindIcon(k CompletionItemKind) string {
	switch k {
	case CompletionKindFunction, CompletionKindMethod:
		return "\x1b[38;2;198;120;221mƒ\x1b[0m" // Purple ƒ
	case CompletionKindVariable, CompletionKindField, CompletionKindProperty:
		return "\x1b[38;2;97;175;239mv\x1b[0m" // Blue v
	case CompletionKindClass, CompletionKindStruct, CompletionKindInterface:
		return "\x1b[38;2;229;192;123mc\x1b[0m" // Yellow c
	case CompletionKindKeyword:
		return "\x1b[38;2;224;108;117mk\x1b[0m" // Red k
	case CompletionKindModule:
		return "\x1b[38;2;86;182;194mm\x1b[0m" // Cyan m
	case CompletionKindSnippet:
		return "\x1b[38;2;152;195;121ms\x1b[0m" // Green s
	default:
		return "\x1b[38;2;171;178;191m•\x1b[0m" // Gray •
	}
}

// Render formats the completion list into a floating ANSI box.
func (p *CompletionPopup) Render(maxW, maxH int) string {
	if !p.Active || len(p.Items) == 0 {
		return ""
	}

	popupW := 38
	if maxW > 50 && popupW > maxW-4 {
		popupW = maxW - 4
	}

	visibleRows := 7
	if visibleRows > len(p.Items) {
		visibleRows = len(p.Items)
	}

	selectedStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.Accent).
		Foreground(ui.CurrentTheme.FgHighlight).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(ui.CurrentTheme.FgPrimary)

	var lines []string
	end := p.ScrollOffset + visibleRows
	if end > len(p.Items) {
		end = len(p.Items)
	}

	for i := p.ScrollOffset; i < end; i++ {
		item := p.Items[i]
		icon := KindIcon(item.Kind)
		label := item.Label
		detail := item.Detail

		// Format row: icon + label + detail
		availLabel := popupW - 6
		if detail != "" && availLabel > 15 {
			availLabel = 18
		}
		labelTrunc := ansi.Truncate(label, availLabel, "…")

		var rowStr string
		if detail != "" {
			availDetail := popupW - 4 - ansi.StringWidth(labelTrunc) - 3
			if availDetail > 0 {
				detTrunc := ansi.Truncate(detail, availDetail, "")
				rowStr = fmt.Sprintf(" %s %s \x1b[38;2;92;99;112m%s\x1b[0m", icon, labelTrunc, detTrunc)
			} else {
				rowStr = fmt.Sprintf(" %s %s", icon, labelTrunc)
			}
		} else {
			rowStr = fmt.Sprintf(" %s %s", icon, labelTrunc)
		}

		padded := ansi.Truncate(rowStr, popupW-2, "")
		if w := ansi.StringWidth(padded); w < popupW-2 {
			padded += strings.Repeat(" ", popupW-2-w)
		}

		if i == p.SelectedIndex {
			lines = append(lines, selectedStyle.Render(padded))
		} else {
			lines = append(lines, normalStyle.Render(padded))
		}
	}

	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme.BorderColor).
		Background(ui.CurrentTheme.BgPrimary).
		Width(popupW)

	return boxStyle.Render(content)
}
