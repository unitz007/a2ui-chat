package a2ui

import (
	"fmt"
)

// Node is a component placed in its surface's tree: its scope (where its relative paths read from) and its children,
// templates expanded for the data as it is now.
type Node struct {
	Component
	Scope    Scope
	Children []*Node
	// Tabs are a Tabs component's titles, one for each child.
	Tabs []string
}

// maxDepth bounds a tree, so a component that is its own child can't loop.
const maxDepth = 32

// Tree is the surface as it is now, from its root ("root"); nil while the root hasn't come (a stream still arriving).
func (s *Surface) Tree() *Node {
	return s.node("root", Scope{}, 0, map[string]bool{})
}

func (s *Surface) node(id string, sc Scope, depth int, on map[string]bool) *Node {
	key := fmt.Sprintf("%s@%s", id, sc.Base)
	if depth > maxDepth || on[key] {
		return nil
	}
	c, ok := s.Component(id)
	if !ok {
		return nil
	}
	on[key] = true
	defer delete(on, key)
	n := &Node{Component: c, Scope: sc}
	add := func(childID string, csc Scope) {
		if ch := s.node(childID, csc, depth+1, on); ch != nil {
			n.Children = append(n.Children, ch)
		}
	}
	switch c.Type {
	case "Card", "Button":
		if id, ok := c.Props["child"].(string); ok {
			add(id, sc)
		}
	case "Modal":
		for _, k := range []string{"trigger", "content"} {
			if id, ok := c.Props[k].(string); ok {
				add(id, sc)
			}
		}
	case "Tabs":
		tabs, _ := c.Props["tabs"].([]any)
		for _, t := range tabs {
			tab, _ := t.(map[string]any)
			if id, ok := tab["child"].(string); ok {
				before := len(n.Children)
				add(id, sc)
				if len(n.Children) > before {
					n.Tabs = append(n.Tabs, s.String(tab["title"], sc))
				}
			}
		}
	case "Row", "Column", "List":
		switch ch := c.Props["children"].(type) {
		case []any:
			for _, x := range ch {
				if id, ok := x.(string); ok {
					add(id, sc)
				}
			}
		case map[string]any: // a template: one child for each item of a list in the data model
			tpl, _ := ch["componentId"].(string)
			path, _ := ch["path"].(string)
			items, _ := s.Get(sc.abs(path)).([]any)
			for i := range items {
				add(tpl, sc.Item(path, i))
			}
		}
	}
	return n
}

// Walk visits a tree depth first, a node before its children; returning false skips the node's children.
func (n *Node) Walk(visit func(*Node) bool) {
	if n == nil || !visit(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(visit)
	}
}
