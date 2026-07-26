// adjacent.go implements the post-parse pass that marks control-action
// nodes whose opening "{{" immediately followed the previous action's
// closing "}}" in source. The printer reads these flags to suppress its
// otherwise unconditional newline + indent before each control action,
// so a round-trip of "{{ range A }}{{ range B }}…{{ end }}{{ end }}"
// preserves the back-to-back layout the author wrote.
//
// Adjacency is a pure function of the AST and the original source
// bytes: no I/O, no globals, no inter-call state.

package parse

import "strings"

// adjacencyMarker carries the walk's only mutable state: the byte
// offset just past the most recently observed "}}". A value of -1
// means no action has been seen yet (the first action can never be
// adjacent).
type adjacencyMarker struct {
	src          string
	lastDelimEnd int
}

// markAdjacency walks root in printer order and sets PrevAdjacent on
// every BranchNode, EndNode, and ElseNode whose "{{" sits immediately
// after the previous action's "}}" in src.
//
// The tracker advances through every action kind (BranchNode, EndNode,
// ElseNode, ActionNode, CommentNode) so that "{{ X }}{{ range . }}"
// also marks the BranchNode adjacent, not only control-to-control
// pairings. Plain ActionNode and CommentNode never emit forced
// newlines, so they don't carry the flag themselves.
//
// Requires: src is the source string that produced root via Parse.
// Ensures:  PrevAdjacent is true on exactly those control nodes whose
//
//	opening "{{" byte-offset equals the byte just past the
//	previous action's "}}".
func markAdjacency(root Node, src string) {
	m := &adjacencyMarker{src: src, lastDelimEnd: -1}
	m.visit(root)
}

func (m *adjacencyMarker) visit(n Node) {
	if n == nil {
		return
	}
	switch n := n.(type) {
	case *ListNode:
		m.visitList(n)
	case *TextNode:
		// Text sits between actions but never carries a delimiter, so
		// it neither sets PrevAdjacent on anything nor advances the
		// tracker — but its presence breaks adjacency for the next
		// action because that action's "{{" no longer touches the
		// previous "}}".
		m.lastDelimEnd = -1
	case *ActionNode:
		m.markAction(n.Position(), nil)
	case *CommentNode:
		m.markAction(n.Position(), nil)
	case *BranchNode:
		m.visitBranch(n)
	}
}

func (m *adjacencyMarker) visitList(n *ListNode) {
	if n == nil {
		return
	}
	for _, c := range n.Nodes {
		m.visit(c)
	}
}

func (m *adjacencyMarker) visitBranch(n *BranchNode) {
	m.markAction(n.Position(), &n.PrevAdjacent)
	m.visit(n.List)
	for _, e := range n.Elses {
		m.markAction(e.Position(), &e.PrevAdjacent)
		m.visit(e.List)
	}
	if n.End != nil {
		m.markAction(n.End.Position(), &n.End.PrevAdjacent)
	}
}

// markAction sets *flag (if non-nil) when the action whose interior
// position is pos has its "{{" immediately after the previous action's
// "}}"; then advances lastDelimEnd past this action's "}}".
//
// flag is nil for action kinds that don't carry an adjacency field
// (ActionNode, CommentNode); they still participate in the walk so
// they correctly update lastDelimEnd.
func (m *adjacencyMarker) markAction(pos Pos, flag *bool) {
	p := int(pos)
	if p < 0 || p > len(m.src) {
		m.lastDelimEnd = -1
		return
	}
	leftIdx := strings.LastIndex(m.src[:p], "{{")
	if leftIdx < 0 {
		m.lastDelimEnd = -1
		return
	}
	if flag != nil && m.lastDelimEnd >= 0 && leftIdx == m.lastDelimEnd {
		*flag = true
	}
	rightOff := strings.Index(m.src[p:], "}}")
	if rightOff < 0 {
		m.lastDelimEnd = -1
		return
	}
	m.lastDelimEnd = p + rightOff + 2
}
