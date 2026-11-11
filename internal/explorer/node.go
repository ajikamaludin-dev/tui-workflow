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
