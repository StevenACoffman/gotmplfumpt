// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package parse

import (
	"strings"
)

// Directive comment markers.
const (
	directiveIgnoreAll   = "gotmplfumpt-ignore-all"
	directiveIgnoreStart = "gotmplfumpt-ignore-start"
	directiveIgnoreEnd   = "gotmplfumpt-ignore-end"
)

// NodeType values identify the kind of a parse tree node.
const (
	NodeText       NodeType = iota // Plain text.
	NodeAction                     // A non-control action such as a field evaluation.
	NodeBool                       // A boolean constant.
	NodeChain                      // A sequence of field accesses.
	NodeCommand                    // An element of a pipeline.
	NodeDot                        // The cursor, dot.
	nodeElse                       // An else action. Not added to tree.
	nodeEnd                        // An end action. Not added to tree.
	NodeField                      // A field or method name.
	NodeIdentifier                 // An identifier; always a function name.
	NodeBranch                     // A branch-y action.
	NodeList                       // A list of Nodes.
	NodeNil                        // An untyped nil constant.
	NodeNumber                     // A numerical constant.
	NodePipe                       // A pipeline of commands.
	NodeString                     // A string constant.
	NodeTemplate                   // A template invocation action.
	NodeVariable                   // A $ variable.
	NodeComment                    // A comment.
)

// indentTab is a precomputed cache of tab strings, indexed by depth.
// Avoids strings.Repeat allocations during printing.
//
//nolint:gochecknoglobals // immutable cache; populated via function literal.
var indentTab = func() [16]string {
	var t [16]string
	for i := range t {
		t[i] = strings.Repeat("\t", i)
	}
	return t
}()

// Node is an element in the parse tree. The interface contains an
// unexported method so that only types local to this package can satisfy
// it.
type Node interface {
	Type() NodeType
	String() string
	Position() Pos // byte position of start of node in full original input
	tree() *Tree
	writeTo(*printer)
}

// NodeType identifies the type of a parse tree node.
type NodeType int

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

// liner is implemented by nodes that store their source line number.
type liner interface {
	lineNumber() int
}

// printer renders a parsed template AST back to source. Its
// responsibilities:
//
//   - Track template branch nesting (branchDepth) so {{if}}/{{end}} pairs
//     align consistently.
//   - Carry the multi-line pipe alignment column (prefix) so that
//     {{ dict\n "a" "b"\n }} stays vertically aligned to the action's
//     opening column.
//   - Remember whether the current branch is a one-liner so {{ else }}
//     and {{ end }} stay on the same line.
//   - Track right-trim pending so that text following a "-}}" delimiter
//     is trimmed.
//
// Text between template actions is emitted verbatim — the format package
// is what ultimately reflows the Go code via gofumpt.
type printer struct {
	*strings.Builder
	prefix           string
	depth            int
	branchDepth      int
	inOneLiner       bool
	rightTrimPending bool
	isLastInList     bool
}

// ListNode holds a sequence of nodes.
type ListNode struct {
	tr *Tree
	NodeType
	Nodes []Node
	Pos
}

// TextNode holds plain text.
type TextNode struct {
	tr *Tree
	NodeType
	Text string
	Pos
	Line int
}

// CommentNode holds a comment.
type CommentNode struct {
	tr *Tree
	NodeType
	Text string
	Pos
	Trim trim
	Line int
}

// PipeNode holds a pipeline with optional declaration.
type PipeNode struct {
	tr *Tree
	NodeType
	Decl []*VariableNode
	Cmds []*CommandNode
	Pos
	Line     int
	IsAssign bool
}

// ActionNode holds an action (something bounded by delimiters). Control
// actions have their own nodes; ActionNode represents simple ones such as
// field evaluations and parenthesized pipelines.
type ActionNode struct {
	tr *Tree
	NodeType
	Pipe *PipeNode
	Pos
	Trim trim
	Line int
}

// CommandNode holds a command (a pipeline inside an evaluating action).
type CommandNode struct {
	tr *Tree
	NodeType
	Args []Node
	Pos
	Trim trim
	Line int
}

// EndNode represents an {{end}} action.
type EndNode struct {
	tr *Tree
	NodeType
	Pos
	Trim trim
	Line int
}

// ElseNode represents an {{else}}, {{else if}}, or {{else with}} action.
// Does not appear in the final tree.
type ElseNode struct {
	tr *Tree
	NodeType
	Pipe    *PipeNode
	List    *ListNode
	Keyword string
	Pos
	Trim trim
	Line int
}

// BranchNode is the common representation of if, range, and with.
type BranchNode struct {
	tr *Tree
	NodeType
	Pipe    *PipeNode
	List    *ListNode
	End     *EndNode
	Keyword string
	Elses   []*ElseNode
	Pos
	Trim trim
	Line int
}

// Position returns the byte position.
func (p Pos) Position() Pos { return p }

// Type returns the NodeType. Provides a default implementation suitable
// for embedding in concrete Node types.
func (t NodeType) Type() NodeType { return t }

func indent(level int) string {
	if level < len(indentTab) {
		return indentTab[level]
	}
	return strings.Repeat("\t", level)
}

func newPrinter() *printer {
	return &printer{Builder: new(strings.Builder)}
}

// WritePrefix writes the current prefix string followed by the depth
// indent. Used during multi-line pipe formatting to align continuations
// with the action's opening column.
func (p *printer) WritePrefix() {
	_, _ = p.WriteString(p.prefix)
	_, _ = p.WriteString(indent(p.depth))
}

// writeAction writes a template action: {{ keyword pipe }} or {{ pipe }}.
// It handles multi-line pipes by computing a prefix from the current
// output line and placing the closing delimiter on its own indented line.
// keyword is empty for plain actions (ActionNode) and "if"/"with"/etc.
// for branches.
func (p *printer) writeAction(keyword string, pipe *PipeNode, tr trim) {
	p.setActionPrefix()
	_, _ = p.WriteString(tr.leftDelim())
	if keyword != "" {
		_, _ = p.WriteString(keyword)
	}
	if len(pipe.Cmds) == 0 {
		_, _ = p.WriteString(tr.rightDelim())
		return
	}
	if keyword != "" {
		_ = p.WriteByte(' ')
	}
	p.writePipeWithMaybeNewline(pipe, tr)
}

// setActionPrefix records the printer's current column so multi-line
// pipes (e.g. {{ dict\n "a" "b"\n }}) can vertically align continuation
// lines.
func (p *printer) setActionPrefix() {
	s := p.String()
	lastNL := strings.LastIndexByte(s, '\n')
	afterNL := s
	if lastNL >= 0 {
		afterNL = s[lastNL+1:]
	}
	if strings.TrimLeft(afterNL, " \t") == "" {
		p.prefix = afterNL
	} else {
		p.prefix = ""
	}
}

// writePipeWithMaybeNewline writes the pipe body. If the body grew a
// newline (multi-line pipe), the closing delimiter goes on its own
// indented line.
func (p *printer) writePipeWithMaybeNewline(pipe *PipeNode, tr trim) {
	onOwnLine := p.prefix != ""
	before := strings.Count(p.String(), "\n")
	p.depth = 1
	pipe.writeTo(p)
	p.depth = 0
	cur := p.String()
	after := strings.Count(cur, "\n")
	if before != after && cur[len(cur)-1] != '`' {
		_, _ = p.WriteString("\n")
		if onOwnLine {
			p.WritePrefix()
		}
		_, _ = p.WriteString(tr.rightDelimNoSpace())
		return
	}
	_, _ = p.WriteString(tr.rightDelim())
}

func (p *printer) writeBranchIndent() {
	if p.branchDepth == 0 {
		return
	}
	s := p.String()
	if s == "" || s[len(s)-1] == '\n' {
		_, _ = p.WriteString(indent(p.branchDepth))
	}
}

// writeControlIndent forces a newline if the output doesn't already end
// with one, then indents to the current branch level. Used by template
// control structures ({{end}}, {{else}}, {{if}}, {{range}}, {{with}})
// which must always start on their own line, except inside a one-liner
// branch.
func (p *printer) writeControlIndent() {
	if p.inOneLiner {
		return
	}
	s := p.String()
	if s != "" && s[len(s)-1] != '\n' {
		_ = p.WriteByte('\n')
	}
	if p.branchDepth > 0 {
		_, _ = p.WriteString(indent(p.branchDepth))
	}
}

func lineno(n Node) int {
	return n.(liner).lineNumber()
}

// isAllWhitespace reports whether s contains only spaces and tabs.
func isAllWhitespace(s string) bool {
	for i := range len(s) {
		if s[i] != ' ' && s[i] != '\t' {
			return false
		}
	}
	return true
}

// rawNodeStart returns the byte position of the '{{' that opens n's
// action, or -1 if it can't be found. n.Position() is interior to {{…}}
// for Action/Branch/Comment nodes, so we walk back to the opening
// delimiter.
func rawNodeStart(n Node, src string) int {
	pos := int(n.Position())
	if pos <= 0 || pos > len(src) {
		return -1
	}
	return strings.LastIndex(src[:pos], "{{")
}

// findActionEnd returns the byte position just past the '}}' that closes
// the action containing start, or -1 if not found. A '-}}' trim marker is
// before '}}' so it is included in the returned slice.
func findActionEnd(src string, start int) int {
	if start < 0 || start >= len(src) {
		return -1
	}
	idx := strings.Index(src[start:], "}}")
	if idx < 0 {
		return -1
	}
	return start + idx + 2
}

// rawNodeEnd returns the byte position just past the closing '}}' of n's
// final action (the action itself for Action/Comment, the End for
// Branch), or -1 if it can't be determined.
func rawNodeEnd(n Node, src string) int {
	switch nn := n.(type) {
	case *ActionNode:
		return findActionEnd(src, int(nn.Position()))
	case *CommentNode:
		return findActionEnd(src, int(nn.Position()))
	case *BranchNode:
		if nn.End == nil {
			return -1
		}
		return findActionEnd(src, int(nn.End.Position()))
	}
	return -1
}

func (t *Tree) newList(pos Pos) *ListNode {
	return &ListNode{tr: t, NodeType: NodeList, Pos: pos}
}

// HasIgnoreAll reports whether the first comment in the list is a
// gotmplfumpt-ignore-all directive.
func (l *ListNode) HasIgnoreAll() bool {
	for _, n := range l.Nodes {
		if c, ok := n.(*CommentNode); ok {
			return strings.Contains(c.Text, directiveIgnoreAll)
		}
		if _, ok := n.(*TextNode); ok {
			continue
		}
		break
	}
	return false
}

// String renders the list as source.
func (l *ListNode) String() string {
	p := newPrinter()
	l.writeTo(p)
	return p.String()
}

func (l *ListNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}

func (l *ListNode) tree() *Tree { return l.tr }

func (l *ListNode) writeTo(sb *printer) {
	if l == nil {
		return
	}
	for i := 0; i < len(l.Nodes); i++ {
		sb.isLastInList = i == len(l.Nodes)-1
		n := l.Nodes[i]
		if skipTo, handled := tryWriteIgnoreBlock(l, sb, i); handled {
			i = skipTo
			continue
		}
		// A pending right-trim only consumes immediately adjacent text;
		// if the next sibling is a non-text node, the trim has nothing
		// to do and must not leak into that node's body.
		if _, isText := n.(*TextNode); !isText {
			sb.rightTrimPending = false
		}
		n.writeTo(sb)
	}
}

// tryWriteIgnoreBlock writes the verbatim source from {{/*
// gotmplfumpt-ignore-start */}} to its matching ignore-end directive if
// the node at index i is the start directive. Returns the index of the
// end directive and true on success; otherwise (0, false).
func tryWriteIgnoreBlock(l *ListNode, sb *printer, i int) (int, bool) {
	c, ok := l.Nodes[i].(*CommentNode)
	if !ok || !strings.Contains(c.Text, directiveIgnoreStart) {
		return 0, false
	}
	for j := i + 1; j < len(l.Nodes); j++ {
		c2, ok := l.Nodes[j].(*CommentNode)
		if !ok || !strings.Contains(c2.Text, directiveIgnoreEnd) {
			continue
		}
		writeIgnoreBlockBody(l, sb, c, c2)
		return j, true
	}
	return 0, false
}

func writeIgnoreBlockBody(l *ListNode, sb *printer, start, end *CommentNode) {
	src := l.tr.text
	cStart := rawNodeStart(start, src)
	c2End := rawNodeEnd(end, src)
	if cStart >= 0 && c2End > cStart && c2End <= len(src) {
		// Preserve user-set indentation for the ignore block.
		lineStart := strings.LastIndexByte(src[:cStart], '\n') + 1
		userIndent := src[lineStart:cStart]
		if isAllWhitespace(userIndent) {
			_, _ = sb.WriteString(userIndent)
		}
		_, _ = sb.WriteString(src[cStart:c2End])
		return
	}
	// Defensive fallback: format the start and end directives normally.
	start.writeTo(sb)
	end.writeTo(sb)
}

func (t *Tree) newText(pos Pos, text string, line int) *TextNode {
	return &TextNode{tr: t, NodeType: NodeText, Pos: pos, Line: line, Text: text}
}

// String returns the verbatim text.
func (t *TextNode) String() string { return t.Text }

func (t *TextNode) lineNumber() int { return t.Line }

func (t *TextNode) writeTo(sb *printer) {
	text := t.Text
	if sb.rightTrimPending {
		text = strings.TrimLeft(text, " \t\n\r")
		sb.rightTrimPending = false
	}
	_, _ = sb.WriteString(text)
}

func (t *TextNode) tree() *Tree { return t.tr }

func (t *Tree) newComment(pos Pos, text string, tr trim, line int) *CommentNode {
	return &CommentNode{tr: t, NodeType: NodeComment, Pos: pos, Line: line, Text: text, Trim: tr}
}

// String renders the comment as source.
func (c *CommentNode) String() string {
	sb := newPrinter()
	c.writeTo(sb)
	return sb.String()
}

func (c *CommentNode) lineNumber() int { return c.Line }

func (c *CommentNode) writeTo(sb *printer) {
	sb.writeBranchIndent()
	if c.Trim.left {
		_, _ = sb.WriteString("{{- ")
	} else {
		_, _ = sb.WriteString("{{")
	}
	_, _ = sb.WriteString(c.Text)
	if c.Trim.right {
		_, _ = sb.WriteString(" -}}")
	} else {
		_, _ = sb.WriteString("}}")
	}
}

func (c *CommentNode) tree() *Tree { return c.tr }

func (t *Tree) newPipeline(pos Pos, line int, vars []*VariableNode) *PipeNode {
	return &PipeNode{tr: t, NodeType: NodePipe, Pos: pos, Line: line, Decl: vars}
}

// String renders the pipeline as source.
func (p *PipeNode) String() string {
	sb := newPrinter()
	p.writeTo(sb)
	return sb.String()
}

func (p *PipeNode) lineNumber() int { return p.Line }

func (p *PipeNode) append(command *CommandNode) {
	p.Cmds = append(p.Cmds, command)
}

func (p *PipeNode) writeTo(sb *printer) {
	if len(p.Decl) > 0 {
		for i, v := range p.Decl {
			if i > 0 {
				_, _ = sb.WriteString(", ")
			}
			v.writeTo(sb)
		}
		if p.IsAssign {
			_, _ = sb.WriteString(" = ")
		} else {
			_, _ = sb.WriteString(" := ")
		}
	}
	for i, c := range p.Cmds {
		if i > 0 {
			_, _ = sb.WriteString(" | ")
		}
		c.writeTo(sb)
	}
}

func (p *PipeNode) tree() *Tree { return p.tr }

func (t *Tree) newAction(pos Pos, line int, pipe *PipeNode, trim trim) *ActionNode {
	return &ActionNode{tr: t, NodeType: NodeAction, Pos: pos, Line: line, Pipe: pipe, Trim: trim}
}

// String renders the action as source.
func (a *ActionNode) String() string {
	sb := newPrinter()
	a.writeTo(sb)
	return sb.String()
}

func (a *ActionNode) lineNumber() int { return a.Line }

func (a *ActionNode) writeTo(sb *printer) {
	sb.writeBranchIndent()
	sb.writeAction("", a.Pipe, a.Trim)
}

func (a *ActionNode) tree() *Tree { return a.tr }

func (t *Tree) newCommand(pos Pos, line int) *CommandNode {
	return &CommandNode{tr: t, NodeType: NodeCommand, Pos: pos, Line: line}
}

// String renders the command as source.
func (c *CommandNode) String() string {
	sb := newPrinter()
	c.writeTo(sb)
	return sb.String()
}

func (c *CommandNode) lineNumber() int { return c.Line }

func (c *CommandNode) append(arg Node) {
	c.Args = append(c.Args, arg)
}

func (c *CommandNode) writeTo(sb *printer) {
	if len(c.Args) == 0 {
		return
	}
	var prevLine int
	for i, arg := range c.Args {
		line := lineno(arg)
		if i > 0 {
			if line > prevLine {
				_, _ = sb.WriteString("\n")
				sb.WritePrefix()
			} else {
				_ = sb.WriteByte(' ')
			}
		}
		prevLine = line
		if pipe, ok := arg.(*PipeNode); ok {
			_ = sb.WriteByte('(')
			before := strings.Count(sb.String(), "\n")
			pipe.writeTo(sb)
			after := strings.Count(sb.String(), "\n")
			if before != after {
				_, _ = sb.WriteString("\n")
				sb.WritePrefix()
			}
			_ = sb.WriteByte(')')
			continue
		}
		arg.writeTo(sb)
	}
}

func (c *CommandNode) tree() *Tree { return c.tr }

func (t *Tree) newEnd(pos Pos, trim trim, line int) *EndNode {
	return &EndNode{tr: t, NodeType: nodeEnd, Pos: pos, Line: line, Trim: trim}
}

// String renders the {{end}} action.
func (e *EndNode) String() string {
	sb := newPrinter()
	e.writeTo(sb)
	return sb.String()
}

func (e *EndNode) lineNumber() int { return e.Line }

func (e *EndNode) writeTo(sb *printer) {
	sb.writeControlIndent()
	_, _ = sb.WriteString(e.Trim.leftDelim())
	_, _ = sb.WriteString("end")
	_, _ = sb.WriteString(e.Trim.rightDelim())
	if e.Trim.right {
		sb.rightTrimPending = true
	}
}

func (e *EndNode) tree() *Tree { return e.tr }

func (t *Tree) newElse(pos Pos, line int, keyword string, pipe *PipeNode, trim trim) *ElseNode {
	return &ElseNode{
		tr:       t,
		NodeType: nodeElse,
		Pos:      pos,
		Line:     line,
		Keyword:  keyword,
		Pipe:     pipe,
		Trim:     trim,
	}
}

// Type overrides the embedded NodeType to expose the (unexported)
// nodeElse value for API compatibility.
func (e *ElseNode) Type() NodeType { return nodeElse }

// String renders the {{else}} action as source.
func (e *ElseNode) String() string {
	sb := newPrinter()
	e.writeTo(sb)
	return sb.String()
}

func (e *ElseNode) lineNumber() int { return e.Line }

func (e *ElseNode) writeTo(sb *printer) {
	sb.writeControlIndent()
	_, _ = sb.WriteString(e.Trim.leftDelim())
	_, _ = sb.WriteString("else")
	if e.Pipe != nil {
		_, _ = sb.WriteString(" ")
		_, _ = sb.WriteString(e.Keyword)
		if len(e.Pipe.Cmds) > 0 {
			_ = sb.WriteByte(' ')
			e.Pipe.writeTo(sb)
		}
	}
	_, _ = sb.WriteString(e.Trim.rightDelim())
	sb.branchDepth++
	e.List.writeTo(sb)
	sb.branchDepth--
}

func (e *ElseNode) tree() *Tree { return e.tr }

// String renders the branch as source.
func (b *BranchNode) String() string {
	sb := newPrinter()
	b.writeTo(sb)
	return sb.String()
}

func (b *BranchNode) lineNumber() int { return b.Line }

func (b *BranchNode) writeTo(sb *printer) {
	// One-liner branches (open and end on the same source line) stay on
	// a single line; multi-line branches put control structures on their
	// own lines.
	savedOneLiner := sb.inOneLiner
	if lineno(b) == lineno(b.End) {
		sb.inOneLiner = true
	}
	if sb.inOneLiner {
		sb.writeBranchIndent()
	} else {
		sb.writeControlIndent()
	}
	sb.writeAction(b.Keyword, b.Pipe, b.Trim)
	indentBody := !sb.inOneLiner && b.Keyword != "define"
	if indentBody {
		sb.branchDepth++
	}
	b.List.writeTo(sb)
	if indentBody {
		sb.branchDepth--
	}
	for _, e := range b.Elses {
		e.writeTo(sb)
	}
	b.End.writeTo(sb)
	sb.inOneLiner = savedOneLiner
}

func (b *BranchNode) tree() *Tree { return b.tr }
