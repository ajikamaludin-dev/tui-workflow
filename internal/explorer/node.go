package explorer

import (
	"sort"
	"strings"
)

type Node struct {
	Path       string
	Name       string
	IsDir      bool
	IsExpanded bool
	Depth      int
	Children   []*Node
	IsHidden   bool
}

func NewNode(path, name string, isDir bool, depth int) *Node {
	return &Node{
		Path:       path,
		Name:       name,
		IsDir:      isDir,
		IsExpanded: false,
		Depth:      depth,
		Children:   make([]*Node, 0),
		IsHidden:   strings.HasPrefix(name, "."),
	}
}

func (n *Node) SortChildren() {
	sort.Slice(n.Children, func(i, j int) bool {
		// Directories first
		if n.Children[i].IsDir != n.Children[j].IsDir {
			return n.Children[i].IsDir
		}
		// Alphabetical case-insensitive
		return strings.ToLower(n.Children[i].Name) < strings.ToLower(n.Children[j].Name)
	})
}

// MatchesFilter checks if node or any of its children match filter substring.
func (n *Node) MatchesFilter(q string) bool {
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(n.Name), q) {
		return true
	}
	if n.IsDir {
		for _, child := range n.Children {
			if child.MatchesFilter(q) {
				return true
			}
		}
	}
	return false
}
