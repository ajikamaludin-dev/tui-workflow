package explorer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tui-workflow/internal/ui"
)

type Tree struct {
	RootPath      string
	RootNode      *Node
	FlatNodes     []*Node
	SelectedIndex int
	ScrollOffset  int
	ShowHidden    bool
	MarqueeOffset int
}

func NewTree(rootPath string) *Tree {
	cleanPath, err := filepath.Abs(rootPath)
	if err != nil {
		cleanPath = rootPath
	}

	rootName := filepath.Base(cleanPath)
	rootNode := NewNode(cleanPath, rootName, true, 0)
	rootNode.IsExpanded = true

	t := &Tree{
		RootPath:      cleanPath,
		RootNode:      rootNode,
		FlatNodes:     make([]*Node, 0),
		SelectedIndex: 0,
		ScrollOffset:  0,
		ShowHidden:    true,
	}

	_ = t.LoadChildren(t.RootNode)
	t.Flatten()
	return t
}

func (t *Tree) LoadChildren(parent *Node) error {
	entries, err := os.ReadDir(parent.Path)
	if err != nil {
		return err
	}

	parent.Children = make([]*Node, 0)
	for _, entry := range entries {
		childPath := filepath.Join(parent.Path, entry.Name())
		childNode := NewNode(childPath, entry.Name(), entry.IsDir(), parent.Depth+1)
		parent.Children = append(parent.Children, childNode)
	}

	parent.SortChildren()
	return nil
}

func (t *Tree) Flatten() {
	// Remember selected path if any
	var selectedPath string
	if t.SelectedIndex >= 0 && t.SelectedIndex < len(t.FlatNodes) {
		selectedPath = t.FlatNodes[t.SelectedIndex].Path
	}

	t.FlatNodes = make([]*Node, 0)
	t.flattenRecursive(t.RootNode)

	// Restore selected index
	if selectedPath != "" {
		for i, n := range t.FlatNodes {
			if n.Path == selectedPath {
				t.SelectedIndex = i
				return
			}
		}
	}

	if t.SelectedIndex >= len(t.FlatNodes) {
		t.SelectedIndex = len(t.FlatNodes) - 1
	}
	if t.SelectedIndex < 0 {
		t.SelectedIndex = 0
	}
}

func (t *Tree) flattenRecursive(node *Node) {
	if node != t.RootNode {
		if node.IsHidden && !t.ShowHidden {
			return
		}
		t.FlatNodes = append(t.FlatNodes, node)
	}

	if node.IsExpanded {
		for _, child := range node.Children {
			t.flattenRecursive(child)
		}
	}
}

func (t *Tree) SelectedNode() *Node {
	if len(t.FlatNodes) == 0 || t.SelectedIndex < 0 || t.SelectedIndex >= len(t.FlatNodes) {
		return nil
	}
	return t.FlatNodes[t.SelectedIndex]
}

func (t *Tree) MoveUp() {
	if t.SelectedIndex > 0 {
		t.SelectedIndex--
		t.MarqueeOffset = 0
		t.adjustScroll()
	}
}

func (t *Tree) MoveDown() {
	if t.SelectedIndex < len(t.FlatNodes)-1 {
		t.SelectedIndex++
		t.MarqueeOffset = 0
		t.adjustScroll()
	}
}

func (t *Tree) ToggleExpand() {
	node := t.SelectedNode()
	if node == nil || !node.IsDir {
		return
	}

	if node.IsExpanded {
		node.IsExpanded = false
	} else {
		node.IsExpanded = true
		_ = t.LoadChildren(node)
	}
	t.Flatten()
}

func (t *Tree) CollapseOrParent() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	if node.IsDir && node.IsExpanded {
		node.IsExpanded = false
		t.Flatten()
		return
	}

	// Move to parent
	parentPath := filepath.Dir(node.Path)
	for i, n := range t.FlatNodes {
		if n.Path == parentPath {
			t.SelectedIndex = i
			t.adjustScroll()
			return
		}
	}
}

func (t *Tree) ToggleHidden() {
	t.ShowHidden = !t.ShowHidden
	t.Flatten()
}

func (t *Tree) Refresh() {
	expandedMap := make(map[string]bool)
	t.collectExpanded(t.RootNode, expandedMap)

	t.RootNode.Children = make([]*Node, 0)
	_ = t.LoadChildren(t.RootNode)
	t.restoreExpanded(t.RootNode, expandedMap)
	t.Flatten()
}

func (t *Tree) collectExpanded(node *Node, m map[string]bool) {
	if node.IsDir && node.IsExpanded {
		m[node.Path] = true
		for _, child := range node.Children {
			t.collectExpanded(child, m)
		}
	}
}

func (t *Tree) restoreExpanded(node *Node, m map[string]bool) {
	if node.IsDir && m[node.Path] {
		node.IsExpanded = true
		_ = t.LoadChildren(node)
		for _, child := range node.Children {
			t.restoreExpanded(child, m)
		}
	}
}

func (t *Tree) adjustScroll() {
	// Scroll offset adjusted dynamically during render
}

func (t *Tree) HandleClick(relativeLine int) (action string, node *Node) {
	targetIdx := t.ScrollOffset + relativeLine
	if targetIdx < 0 || targetIdx >= len(t.FlatNodes) {
		return "", nil
	}

	t.SelectedIndex = targetIdx
	t.MarqueeOffset = 0
	node = t.FlatNodes[targetIdx]

	if node.IsDir {
		t.ToggleExpand()
		return "toggle", node
	}

	return "open", node
}

func (t *Tree) TickMarquee() {
	t.MarqueeOffset++
}

func (t *Tree) ResetMarquee() {
	t.MarqueeOffset = 0
}

func (t *Tree) SelectedNameExceeds(width int) bool {
	node := t.SelectedNode()
	if node == nil {
		return false
	}
	prefixWidth := (node.Depth - 1) * 2 + 2
	availForName := width - prefixWidth
	return ansi.StringWidth(node.Name) > availForName
}

func (t *Tree) ScrollUp(delta int) {
	t.ScrollOffset -= delta
	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}
	if t.SelectedIndex < t.ScrollOffset {
		t.SelectedIndex = t.ScrollOffset
	}
}

func (t *Tree) ScrollDown(delta int, height int) {
	maxScroll := len(t.FlatNodes) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	t.ScrollOffset += delta
	if t.ScrollOffset > maxScroll {
		t.ScrollOffset = maxScroll
	}
	if t.SelectedIndex < t.ScrollOffset {
		t.SelectedIndex = t.ScrollOffset
	}
}

func (t *Tree) Render(width, height int, focused bool) string {
	if len(t.FlatNodes) == 0 {
		return lipgloss.NewStyle().
			Foreground(ui.CurrentTheme.FgSecondary).
			Padding(1, 1).
			Render("(Folder Kosong)")
	}

	// Ensure selected node is in viewport
	if t.SelectedIndex < t.ScrollOffset {
		t.ScrollOffset = t.SelectedIndex
	} else if t.SelectedIndex >= t.ScrollOffset+height {
		t.ScrollOffset = t.SelectedIndex - height + 1
	}

	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}

	var lines []string
	endIdx := t.ScrollOffset + height
	if endIdx > len(t.FlatNodes) {
		endIdx = len(t.FlatNodes)
	}

	for i := t.ScrollOffset; i < endIdx; i++ {
		node := t.FlatNodes[i]
		indent := strings.Repeat("  ", node.Depth-1)
		if indent < "" {
			indent = ""
		}
		var icon string
		if node.IsDir {
			if node.IsExpanded {
				icon = "▼ "
			} else {
				icon = "▶ "
			}
		} else {
			icon = "  "
		}

		prefix := indent + icon
		prefixWidth := ansi.StringWidth(prefix)
		availForName := width - prefixWidth
		if availForName < 1 {
			availForName = 1
		}

		var displayName string
		nameWidth := ansi.StringWidth(node.Name)

		if i == t.SelectedIndex {
			if nameWidth > availForName {
				// Marquee horizontally for selected item
				loopText := node.Name + "   "
				loopRunes := []rune(loopText)
				start := t.MarqueeOffset % len(loopRunes)
				var nameRunes []rune
				for k := 0; k < availForName; k++ {
					nameRunes = append(nameRunes, loopRunes[(start+k)%len(loopRunes)])
				}
				displayName = string(nameRunes)
			} else {
				displayName = node.Name
			}
		} else {
			if nameWidth > availForName {
				// Truncate with ... for inactive items
				displayName = ansi.Truncate(node.Name, availForName, "...")
			} else {
				displayName = node.Name
			}
		}

		lineText := prefix + displayName
		lineWidth := ansi.StringWidth(lineText)
		if lineWidth < width {
			lineText += strings.Repeat(" ", width-lineWidth)
		} else if lineWidth > width {
			lineText = ansi.Truncate(lineText, width, "")
		}

		if i == t.SelectedIndex {
			style := lipgloss.NewStyle().
				Background(ui.CurrentTheme.BorderColor).
				Foreground(ui.CurrentTheme.FgHighlight).
				Bold(true)
			if focused {
				style = style.Background(ui.CurrentTheme.Accent)
			}
			lines = append(lines, style.Render(lineText))
		} else {
			style := lipgloss.NewStyle().
				Foreground(ui.CurrentTheme.FgPrimary)
			if node.IsDir {
				style = style.Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
			} else if node.IsHidden {
				style = style.Foreground(ui.CurrentTheme.FgSecondary)
			}
			lines = append(lines, style.Render(lineText))
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}
