// verify.go provides a structural-shape comparison between two parse
// trees. Used by Format as a post-restoration sanity check: if the
// restored output has a different number or kind of template actions
// than the input, something went wrong and we fall back.

package format

import "github.com/StevenACoffman/gotmplfumpt/internal/parse"

const (
	shapeAction shapeKind = iota
	shapeComment
	shapeBranchOpen
	shapeBranchMid // else / else if
	shapeBranchEnd
)

// structuralShape is a flat fingerprint of a parse tree: the ordered
// list of action kinds visited by an in-order walk. Two trees have the
// same structuralShape when they declare the same template-action
// sequence, regardless of TextNode contents, field-access paths, or
// whitespace differences.
type structuralShape []shapeAtom

// shapeAtom records one action's contribution to the shape.
type shapeAtom struct {
	keyword string // for BranchNode; empty for non-branch nodes
	kind    shapeKind
}

// shapeKind classifies an action by AST node type.
type shapeKind uint8

// computeShape walks n and returns its structural shape.
//
// Requires: n is non-nil and produced by parse.Parse.
// Ensures:  the result is deterministic; two trees that differ only in
//
//	TextNode contents or field-identifier strings produce the
//	same shape.
func computeShape(n parse.Node) structuralShape {
	s := make(structuralShape, 0, 8)
	appendShape(&s, n)
	return s
}

func appendShape(s *structuralShape, n parse.Node) {
	if n == nil {
		return
	}
	switch n := n.(type) {
	case *parse.ListNode:
		appendListShape(s, n)
	case *parse.TextNode:
		// Text contributes no shape.
	case *parse.ActionNode:
		*s = append(*s, shapeAtom{kind: shapeAction})
	case *parse.CommentNode:
		*s = append(*s, shapeAtom{kind: shapeComment})
	case *parse.BranchNode:
		appendBranchShape(s, n)
	}
}

func appendListShape(s *structuralShape, n *parse.ListNode) {
	if n == nil {
		return
	}
	for _, c := range n.Nodes {
		appendShape(s, c)
	}
}

func appendBranchShape(s *structuralShape, n *parse.BranchNode) {
	*s = append(*s, shapeAtom{kind: shapeBranchOpen, keyword: n.Keyword})
	appendShape(s, n.List)
	for _, e := range n.Elses {
		*s = append(*s, shapeAtom{kind: shapeBranchMid, keyword: e.Keyword})
		appendShape(s, e.List)
	}
	if n.End != nil {
		*s = append(*s, shapeAtom{kind: shapeBranchEnd, keyword: n.Keyword})
	}
}

// shapesEqual reports whether two structuralShapes describe the same
// action sequence.
func shapesEqual(a, b structuralShape) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
