// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Terminal AST node types: identifiers, variables, dots, nils, fields,
// chains, bools, numbers, strings. Split out of node.go so that file
// stays under the 1000-SLOC hard limit.

package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// IdentifierNode holds an identifier.
type IdentifierNode struct {
	tr *Tree
	NodeType
	Ident string
	Pos
	Line int
}

// VariableNode holds a list of variable names, possibly with chained field
// accesses. The dollar sign is part of the (first) name.
type VariableNode struct {
	tr *Tree
	NodeType
	Ident []string
	Pos
	Line int
}

// DotNode holds the special identifier '.'.
type DotNode struct {
	tr *Tree
	NodeType
	Pos
	Line int
}

// NilNode holds the special identifier 'nil' representing an untyped nil
// constant.
type NilNode struct {
	tr *Tree
	NodeType
	Pos
	Line int
}

// FieldNode holds a field (identifier starting with '.'). The names may
// be chained ('.x.y'). The period is dropped from each ident.
type FieldNode struct {
	tr *Tree
	NodeType
	Ident []string
	Pos
	Line int
}

// ChainNode holds a term followed by a chain of field accesses
// (identifier starting with '.'). The names may be chained ('.x.y').
// The periods are dropped from each ident.
type ChainNode struct {
	tr   *Tree
	Node Node
	NodeType
	Field []string
	Pos
	Line int
}

// BoolNode holds a boolean constant.
type BoolNode struct {
	tr *Tree
	NodeType
	Pos
	Line int
	True bool
}

// NumberNode holds a number: signed or unsigned integer, float, or
// complex. The value is parsed and stored under all the types that can
// represent the value. This simulates in a small amount of code the
// behavior of Go's ideal constants.
type NumberNode struct {
	tr *Tree
	NodeType
	Text       string
	Complex128 complex128
	Int64      int64
	Uint64     uint64
	Float64    float64
	Pos
	Line      int
	IsInt     bool
	IsUint    bool
	IsFloat   bool
	IsComplex bool
}

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	tr *Tree
	NodeType
	Quoted string
	Text   string
	Pos
	Line int
}

// NewIdentifier returns a new IdentifierNode with the given identifier
// name.
func NewIdentifier(ident string) *IdentifierNode {
	return &IdentifierNode{NodeType: NodeIdentifier, Ident: ident}
}

// SetPos sets the position. NewIdentifier is a public method so we can't
// modify its signature. Chained for convenience.
func (i *IdentifierNode) SetPos(pos Pos) *IdentifierNode {
	i.Pos = pos
	return i
}

// SetTree sets the parent tree for the node. NewIdentifier is a public
// method so we can't modify its signature. Chained for convenience.
func (i *IdentifierNode) SetTree(t *Tree) *IdentifierNode {
	i.tr = t
	return i
}

// SetLine sets the line number. NewIdentifier is a public method so we
// can't modify its signature. Chained for convenience.
func (i *IdentifierNode) SetLine(line int) *IdentifierNode {
	i.Line = line
	return i
}

// String returns the identifier name.
func (i *IdentifierNode) String() string {
	return i.Ident
}

func (i *IdentifierNode) lineNumber() int     { return i.Line }
func (i *IdentifierNode) writeTo(sb *printer) { _, _ = sb.WriteString(i.String()) }
func (i *IdentifierNode) tree() *Tree         { return i.tr }

func (t *Tree) newVariable(pos Pos, ident string, line int) *VariableNode {
	return &VariableNode{
		tr:       t,
		NodeType: NodeVariable,
		Pos:      pos,
		Line:     line,
		Ident:    strings.Split(ident, "."),
	}
}

// String renders the variable and its field chain.
func (v *VariableNode) String() string {
	sb := newPrinter()
	v.writeTo(sb)
	return sb.String()
}

func (v *VariableNode) lineNumber() int { return v.Line }

func (v *VariableNode) writeTo(sb *printer) {
	for i, id := range v.Ident {
		if i > 0 {
			_ = sb.WriteByte('.')
		}
		_, _ = sb.WriteString(id)
	}
}

func (v *VariableNode) tree() *Tree { return v.tr }

func (t *Tree) newDot(pos Pos, line int) *DotNode {
	return &DotNode{tr: t, NodeType: NodeDot, Pos: pos, Line: line}
}

// Type overrides the embedded NodeType for API compatibility.
func (d *DotNode) Type() NodeType { return NodeDot }

// String renders the dot.
func (d *DotNode) String() string { return "." }

func (d *DotNode) lineNumber() int     { return d.Line }
func (d *DotNode) writeTo(sb *printer) { _, _ = sb.WriteString(d.String()) }
func (d *DotNode) tree() *Tree         { return d.tr }

func (t *Tree) newNil(pos Pos, line int) *NilNode {
	return &NilNode{tr: t, NodeType: NodeNil, Pos: pos, Line: line}
}

// Type overrides the embedded NodeType for API compatibility.
func (n *NilNode) Type() NodeType { return NodeNil }

// String renders the literal "nil".
func (n *NilNode) String() string { return "nil" }

func (n *NilNode) lineNumber() int     { return n.Line }
func (n *NilNode) writeTo(sb *printer) { _, _ = sb.WriteString(n.String()) }
func (n *NilNode) tree() *Tree         { return n.tr }

func (t *Tree) newField(pos Pos, ident string, line int) *FieldNode {
	return &FieldNode{
		tr:       t,
		NodeType: NodeField,
		Pos:      pos,
		Line:     line,
		Ident:    strings.Split(ident[1:], "."), // [1:] drops leading period.
	}
}

// String renders the field chain with a leading dot per element.
func (f *FieldNode) String() string {
	sb := newPrinter()
	f.writeTo(sb)
	return sb.String()
}

func (f *FieldNode) lineNumber() int { return f.Line }

func (f *FieldNode) writeTo(sb *printer) {
	for _, id := range f.Ident {
		_ = sb.WriteByte('.')
		_, _ = sb.WriteString(id)
	}
}

func (f *FieldNode) tree() *Tree { return f.tr }

func (t *Tree) newChain(pos Pos, node Node, line int) *ChainNode {
	return &ChainNode{tr: t, NodeType: NodeChain, Pos: pos, Line: line, Node: node}
}

// Add adds the named field (which should start with a period) to the end
// of the chain.
func (c *ChainNode) Add(field string) {
	if field == "" || field[0] != '.' {
		panic("no dot in field")
	}
	field = field[1:] // Remove leading dot.
	if field == "" {
		panic("empty field")
	}
	c.Field = append(c.Field, field)
}

// String renders the chain term followed by its field chain.
func (c *ChainNode) String() string {
	sb := newPrinter()
	c.writeTo(sb)
	return sb.String()
}

func (c *ChainNode) lineNumber() int { return c.Line }

func (c *ChainNode) writeTo(sb *printer) {
	if _, ok := c.Node.(*PipeNode); ok {
		_ = sb.WriteByte('(')
		c.Node.writeTo(sb)
		_ = sb.WriteByte(')')
	} else {
		c.Node.writeTo(sb)
	}
	for _, field := range c.Field {
		_ = sb.WriteByte('.')
		_, _ = sb.WriteString(field)
	}
}

func (c *ChainNode) tree() *Tree { return c.tr }

func (t *Tree) newBool(pos Pos, value bool, line int) *BoolNode {
	return &BoolNode{tr: t, NodeType: NodeBool, Pos: pos, Line: line, True: value}
}

// String renders "true" or "false".
func (b *BoolNode) String() string {
	if b.True {
		return "true"
	}
	return "false"
}

func (b *BoolNode) lineNumber() int     { return b.Line }
func (b *BoolNode) writeTo(sb *printer) { _, _ = sb.WriteString(b.String()) }
func (b *BoolNode) tree() *Tree         { return b.tr }

func (t *Tree) newNumber(pos Pos, text string, typ itemType, line int) (*NumberNode, error) {
	n := &NumberNode{tr: t, NodeType: NodeNumber, Pos: pos, Line: line, Text: text}
	switch typ {
	case itemCharConstant:
		return newNumberFromChar(n, text)
	case itemComplex:
		return newNumberFromComplex(n, text)
	case itemError, itemBool, itemChar, itemComment, itemAssign, itemDeclare,
		itemEOF, itemField, itemIdentifier, itemLeftDelim, itemLeftParen,
		itemNumber, itemPipe, itemRawString, itemRightDelim, itemRightParen,
		itemSpace, itemString, itemText, itemVariable, itemKeyword,
		itemDot, itemElse, itemEnd, itemIf, itemNil, itemBranch:
		// Fall through to the generic numeric parse path below.
	}
	return parseGenericNumber(n, text)
}

// parseGenericNumber tries imaginary, then unsigned, signed, and float.
func parseGenericNumber(n *NumberNode, text string) (*NumberNode, error) {
	if text != "" && text[len(text)-1] == 'i' && numberAsImaginary(n, text) {
		return n, nil
	}
	numberAsUint(n, text)
	numberAsInt(n, text)
	if err := numberAsFloat(n, text); err != nil {
		return nil, err
	}
	if !n.IsInt && !n.IsUint && !n.IsFloat {
		return nil, fmt.Errorf("illegal number syntax: %q", text)
	}
	return n, nil
}

// newNumberFromChar handles a Go character constant like 'x' or '\n'.
func newNumberFromChar(n *NumberNode, text string) (*NumberNode, error) {
	r, _, tail, err := strconv.UnquoteChar(text[1:], text[0])
	if err != nil {
		return nil, fmt.Errorf("unquote char %q: %w", text, err)
	}
	if tail != "'" {
		return nil, fmt.Errorf("malformed character constant: %s", text)
	}
	if r < 0 {
		return nil, fmt.Errorf("invalid character constant: %s", text)
	}
	n.Int64 = int64(r)
	n.IsInt = true
	n.Uint64 = uint64(r) // r ≥ 0; safe widen.
	n.IsUint = true
	n.Float64 = float64(r) // odd but those are the rules.
	n.IsFloat = true
	return n, nil
}

// newNumberFromComplex parses a complex literal like "1+2i".
func newNumberFromComplex(n *NumberNode, text string) (*NumberNode, error) {
	// fmt.Sscan can parse the pair, so let it do the work.
	if _, err := fmt.Sscan(text, &n.Complex128); err != nil {
		return nil, fmt.Errorf("sscan complex %q: %w", text, err)
	}
	n.IsComplex = true
	n.simplifyComplex()
	return n, nil
}

// numberAsImaginary tries to parse text as a zero-real-part imaginary
// constant ("3i", "0i"). Returns true if parsed.
func numberAsImaginary(n *NumberNode, text string) bool {
	f, err := strconv.ParseFloat(text[:len(text)-1], 64)
	if err != nil {
		return false
	}
	n.IsComplex = true
	n.Complex128 = complex(0, f)
	n.simplifyComplex()
	return true
}

// numberAsUint tries to parse text as a Go unsigned integer literal.
// On success populates IsUint and Uint64; otherwise leaves n unchanged.
func numberAsUint(n *NumberNode, text string) {
	u, err := strconv.ParseUint(text, 0, 64) // will fail for -0; fixed in numberAsInt.
	if err == nil {
		n.IsUint = true
		n.Uint64 = u
	}
}

// numberAsInt tries to parse text as a Go signed integer literal.
func numberAsInt(n *NumberNode, text string) {
	i, err := strconv.ParseInt(text, 0, 64)
	if err == nil {
		n.IsInt = true
		n.Int64 = i
		if i == 0 {
			n.IsUint = true // in case of -0.
			// numberAsUint has already populated Uint64 (or set it to 0).
		}
	}
}

// numberAsFloat fills the float/integer fields from a Go float literal.
// Returns an error only when text looks like an integer that doesn't fit
// in int64; non-numeric text is silently ignored.
func numberAsFloat(n *NumberNode, text string) error {
	if n.IsInt {
		n.IsFloat = true
		n.Float64 = float64(n.Int64)
		return nil
	}
	if n.IsUint {
		n.IsFloat = true
		n.Float64 = float64(n.Uint64)
		return nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil //nolint:nilerr // non-numeric text isn't a parse error here.
	}
	// A "float" that looks like an integer is one too big for int64.
	if !strings.ContainsAny(text, ".eEpP") {
		return fmt.Errorf("integer overflow: %q", text)
	}
	n.IsFloat = true
	n.Float64 = f
	if !n.IsInt && float64(int64(f)) == f {
		n.IsInt = true
		n.Int64 = int64(f)
	}
	if !n.IsUint && f >= 0 && float64(uint64(f)) == f {
		n.IsUint = true
		n.Uint64 = uint64(f)
	}
	return nil
}

// String returns the original textual representation of the number.
func (n *NumberNode) String() string { return n.Text }

func (n *NumberNode) lineNumber() int { return n.Line }

// simplifyComplex pulls out any other types that are represented by the
// complex number. These all require that the imaginary part be zero.
func (n *NumberNode) simplifyComplex() {
	n.IsFloat = imag(n.Complex128) == 0
	if !n.IsFloat {
		return
	}
	n.Float64 = real(n.Complex128)
	n.IsInt = float64(int64(n.Float64)) == n.Float64
	if n.IsInt {
		n.Int64 = int64(n.Float64)
	}
	n.IsUint = n.Float64 >= 0 && float64(uint64(n.Float64)) == n.Float64
	if n.IsUint {
		n.Uint64 = uint64(n.Float64)
	}
}

func (n *NumberNode) writeTo(sb *printer) { _, _ = sb.WriteString(n.String()) }
func (n *NumberNode) tree() *Tree         { return n.tr }

func (t *Tree) newString(pos Pos, orig, text string, line int) *StringNode {
	return &StringNode{tr: t, NodeType: NodeString, Pos: pos, Line: line, Quoted: orig, Text: text}
}

// String returns the original quoted representation of the string.
func (s *StringNode) String() string { return s.Quoted }

func (s *StringNode) lineNumber() int     { return s.Line }
func (s *StringNode) writeTo(sb *printer) { _, _ = sb.WriteString(s.String()) }
func (s *StringNode) tree() *Tree         { return s.tr }
