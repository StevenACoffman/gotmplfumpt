// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse builds parse trees for Go templates using the text/template
// grammar. It is a fork of the standard library's text/template/parse,
// trimmed and adapted for gotmplfumpt's formatting needs.
package parse

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// Mode flags. ParseComments adds {{/* … */}} comments to the AST;
// SkipFuncCheck disables function-presence checks (unused here but
// preserved for API compatibility with text/template/parse).
const (
	// ParseComments adds comment nodes to the AST.
	ParseComments Mode = 1 << iota
	// SkipFuncCheck disables function-presence checks.
	SkipFuncCheck
)

// errUnreachable is panicked from code paths that follow a call to
// (*Tree).errorf, which itself panics — the explicit panic exists only to
// satisfy linters that require explicit returns.
var errUnreachable = errors.New("parse: unreachable")

// Mode is a set of flags (or 0) controlling parser behavior.
type Mode uint

// Tree is the representation of a single parsed template.
type Tree struct {
	lex        *lexer
	Root       *ListNode
	text       string
	token      [3]item
	peekCount  int
	actionLine int
}

// Parse parses the given template source and returns the root list node.
// Returns the error from (*Tree).Parse if parsing fails.
func Parse(text string) (*ListNode, error) {
	t := new(Tree)
	if err := t.Parse(text); err != nil {
		return nil, err
	}
	return t.Root, nil
}

// ErrorContext returns a textual representation of the location of the
// node in the input text. The receiver is only used when the node does
// not have a pointer to the tree inside, which can occur in old code.
func (t *Tree) ErrorContext(n Node) (location, context string) {
	pos := int(n.Position())
	tree := n.tree()
	if tree == nil {
		tree = t
	}
	text := tree.text[:pos]
	byteNum := strings.LastIndex(text, "\n")
	if byteNum == -1 {
		byteNum = pos // On first line.
	} else {
		byteNum++ // After the newline.
		byteNum = pos - byteNum
	}
	lineNum := 1 + strings.Count(text, "\n")
	context = n.String()
	return fmt.Sprintf("%d:%d", lineNum, byteNum), context
}

// Parse parses the template definition string to construct a
// representation of the template for formatting.
func (t *Tree) Parse(text string) (err error) {
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		castErr, ok := e.(error)
		if !ok {
			panic(e)
		}
		err = castErr
	}()
	t.lex = lex(text)
	t.text = text
	t.parse()
	return nil
}

func (t *Tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

func (t *Tree) backup() {
	t.peekCount++
}

// backup2 backs the input stream up two tokens. The zeroth token is
// already there.
func (t *Tree) backup2(t1 item) {
	t.token[1] = t1
	t.peekCount = 2
}

// backup3 backs the input stream up three tokens. The zeroth token is
// already there.
func (t *Tree) backup3(t2, t1 item) { // Reverse order: we're pushing back.
	t.token[1] = t1
	t.token[2] = t2
	t.peekCount = 3
}

func (t *Tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

func (t *Tree) nextNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	return token
}

func (t *Tree) peekNonSpace() item {
	token := t.nextNonSpace()
	t.backup()
	return token
}

func (t *Tree) errorf(format string, args ...any) {
	t.Root = nil
	format = fmt.Sprintf("template: %d: %s", t.token[0].line, format)
	panic(fmt.Errorf(format, args...))
}

func (t *Tree) error(err error) {
	t.errorf("%s", err)
}

func (t *Tree) expect(expected itemType, context string) item {
	token := t.nextNonSpace()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

func (t *Tree) unexpected(token item, context string) {
	if token.typ == itemError {
		extra := ""
		if t.actionLine != 0 && t.actionLine != token.line {
			extra = fmt.Sprintf(" in action started at :%d", t.actionLine)
			if strings.HasSuffix(token.val, " action") {
				extra = extra[len(" in action"):] // avoid "action in action"
			}
		}
		t.errorf("%s%s", token, extra)
	}
	t.errorf("unexpected %s in %s", token, context)
}

// parse is the top-level parser, essentially the same as itemList except
// it also parses {{define}} actions. It runs to EOF.
func (t *Tree) parse() {
	t.Root = t.newList(t.peek().pos)
	for t.peek().typ != itemEOF {
		if t.peek().typ == itemLeftDelim {
			delim := t.next()
			t.nextNonSpace()
			t.backup2(delim)
		}
		n := t.textOrAction()
		switch n.Type() {
		case nodeEnd, nodeElse:
			t.errorf("unexpected %s", n)
		case NodeText, NodeAction, NodeBool, NodeChain, NodeCommand,
			NodeDot, NodeField, NodeIdentifier, NodeBranch, NodeList,
			NodeNil, NodeNumber, NodePipe, NodeString, NodeTemplate,
			NodeVariable, NodeComment:
			t.Root.append(n)
		}
	}
}

// itemList parses textOrAction* and terminates at {{end}} or {{else}},
// which is returned separately.
func (t *Tree) itemList() (list *ListNode, next Node) {
	list = t.newList(t.peekNonSpace().pos)
	for t.peekNonSpace().typ != itemEOF {
		n := t.textOrAction()
		switch n.Type() {
		case nodeEnd, nodeElse:
			return list, n
		case NodeText, NodeAction, NodeBool, NodeChain, NodeCommand,
			NodeDot, NodeField, NodeIdentifier, NodeBranch, NodeList,
			NodeNil, NodeNumber, NodePipe, NodeString, NodeTemplate,
			NodeVariable, NodeComment:
		}
		list.append(n)
	}
	t.errorf("unexpected EOF")
	panic(errUnreachable) // errorf panics; this is for the linter.
}

// textOrAction parses text | comment | action.
func (t *Tree) textOrAction() (n Node) {
	token := t.nextNonSpace()
	switch token.typ {
	case itemText:
		return t.newText(token.pos, token.val, token.line)
	case itemLeftDelim:
		t.actionLine = token.line
		defer t.clearActionLine()
		return t.action(token.trim)
	case itemComment:
		return t.newComment(token.pos, token.val, token.trim, token.line)
	case itemError, itemBool, itemChar, itemCharConstant, itemComplex,
		itemAssign, itemDeclare, itemEOF, itemField, itemIdentifier,
		itemLeftParen, itemNumber, itemPipe, itemRawString,
		itemRightDelim, itemRightParen, itemSpace, itemString,
		itemVariable, itemKeyword, itemDot, itemElse, itemEnd, itemIf,
		itemNil, itemBranch:
		t.unexpected(token, "input")
	}
	return nil
}

func (t *Tree) clearActionLine() {
	t.actionLine = 0
}

// action parses one of: control, command ("|" command)*.
// Left delim is past. First word could be a keyword such as range.
func (t *Tree) action(trim trim) (n Node) {
	switch token := t.nextNonSpace(); token.typ {
	case itemElse:
		return t.elseControl(trim)
	case itemEnd:
		return t.endControl(trim)
	case itemIf, itemBranch:
		return t.branchControl(token.val, trim)
	case itemError, itemBool, itemChar, itemCharConstant, itemComment,
		itemComplex, itemAssign, itemDeclare, itemEOF, itemField,
		itemIdentifier, itemLeftDelim, itemLeftParen, itemNumber,
		itemPipe, itemRawString, itemRightDelim, itemRightParen,
		itemSpace, itemString, itemText, itemVariable, itemKeyword,
		itemDot, itemNil:
		// Fall through to generic pipeline handling below.
	}
	t.backup()
	token := t.peek()
	pipe, endtok := t.pipeline("command", itemRightDelim)
	trim.right = endtok.trim.right
	return t.newAction(token.pos, token.line, pipe, trim)
}

// pipeline parses: declarations? command ('|' command)*.
func (t *Tree) pipeline(context string, end itemType) (*PipeNode, item) {
	token := t.peekNonSpace()
	pipe := t.newPipeline(token.pos, token.line, nil)
	t.parsePipelineDecls(pipe, context)
	for {
		token := t.nextNonSpace()
		if token.typ == end {
			t.checkPipeline(pipe)
			return pipe, token
		}
		if isPipelineOperand(token.typ) {
			t.backup()
			pipe.append(t.command())
			continue
		}
		t.unexpected(token, context)
	}
}

// parsePipelineDecls parses any leading $var declarations or assignments
// in a pipeline. Loops to handle `$k, $v := range` (two declarations).
func (t *Tree) parsePipelineDecls(pipe *PipeNode, context string) {
	for {
		if !t.maybeParseOneDecl(pipe, context) {
			return
		}
	}
}

// maybeParseOneDecl tries to parse one $var declaration or assignment.
// Returns true if it consumed a declaration AND a second declaration may
// follow (the `$k, $v := range` case). Returns false when the next token
// is not a variable, when assignment finishes the decl list, or when no
// more declarations are allowed.
func (t *Tree) maybeParseOneDecl(pipe *PipeNode, context string) bool {
	v := t.peekNonSpace()
	if v.typ != itemVariable {
		return false
	}
	t.next()
	// Three-token look-ahead: in "$x foo" we need to read "foo" (as
	// opposed to ":=") to know that $x is an argument variable rather
	// than a declaration. Remember the adjacent token so we can push it
	// back if necessary.
	tokenAfterVariable := t.peek()
	next := t.peekNonSpace()
	switch {
	case next.typ == itemAssign, next.typ == itemDeclare:
		pipe.IsAssign = next.typ == itemAssign
		t.nextNonSpace()
		pipe.Decl = append(pipe.Decl, t.newVariable(v.pos, v.val, v.line))
		return false
	case next.typ == itemChar && next.val == ",":
		t.nextNonSpace()
		pipe.Decl = append(pipe.Decl, t.newVariable(v.pos, v.val, v.line))
		return t.acceptSecondRangeDecl(pipe, context)
	case tokenAfterVariable.typ == itemSpace:
		t.backup3(v, tokenAfterVariable)
	default:
		t.backup2(v)
	}
	return false
}

// acceptSecondRangeDecl reports whether the parser should loop to take a
// second range-declaration variable. It errors out for non-range or
// over-allocated decl lists.
func (t *Tree) acceptSecondRangeDecl(pipe *PipeNode, context string) bool {
	if context == "range" && len(pipe.Decl) < 2 {
		switch t.peekNonSpace().typ {
		case itemVariable, itemRightDelim, itemRightParen:
			return true
		case itemError, itemBool, itemChar, itemCharConstant,
			itemComment, itemComplex, itemAssign, itemDeclare,
			itemEOF, itemField, itemIdentifier, itemLeftDelim,
			itemLeftParen, itemNumber, itemPipe, itemRawString,
			itemSpace, itemString, itemText, itemKeyword,
			itemDot, itemElse, itemEnd, itemIf, itemNil,
			itemBranch:
			t.errorf("range can only initialize variables")
		}
	}
	t.errorf("too many declarations in %s", context)
	return false
}

// isPipelineOperand reports whether typ may begin a pipeline command
// operand.
func isPipelineOperand(typ itemType) bool {
	switch typ {
	case itemBool, itemCharConstant, itemComplex, itemDot, itemField,
		itemIdentifier, itemNumber, itemNil, itemRawString, itemString,
		itemVariable, itemLeftParen:
		return true
	case itemError, itemChar, itemComment, itemAssign, itemDeclare,
		itemEOF, itemLeftDelim, itemPipe, itemRightDelim, itemRightParen,
		itemSpace, itemText, itemKeyword, itemElse, itemEnd, itemIf,
		itemBranch:
		return false
	}
	return false
}

func (t *Tree) checkPipeline(pipe *PipeNode) {
	// Allow empty pipelines — this is a formatter, not a validator.
	if len(pipe.Cmds) == 0 {
		return
	}
	// Only the first command of a pipeline can start with a non-executable operand.
	for i, c := range pipe.Cmds[1:] {
		switch c.Args[0].Type() {
		case NodeBool, NodeDot, NodeNil, NodeNumber, NodeString:
			t.errorf("non executable command in pipeline stage %d", i+2)
		case NodeText, NodeAction, NodeChain, NodeCommand, NodeField,
			NodeIdentifier, NodeBranch, NodeList, NodePipe, NodeTemplate,
			NodeVariable, NodeComment, nodeElse, nodeEnd:
		}
	}
}

// branchControl parses an if/range/with action and its else/end tail.
func (t *Tree) branchControl(keyword string, trim trim) Node {
	pipe, tok := t.pipeline(keyword, itemRightDelim)
	trim.right = tok.trim.right
	b := &BranchNode{
		tr:       t,
		NodeType: NodeBranch,
		Keyword:  keyword,
		Pos:      pipe.Position(),
		Line:     pipe.Line,
		Pipe:     pipe,
		Trim:     trim,
	}
	var next Node
	b.List, next = t.itemList()
Elses:
	for {
		switch n := next.(type) {
		case *ElseNode:
			n.List, next = t.itemList()
			b.Elses = append(b.Elses, n)
		case *EndNode:
			b.End = n
			break Elses
		}
	}
	return b
}

func (t *Tree) endControl(trim trim) Node {
	token := t.expect(itemRightDelim, "end")
	trim.right = token.trim.right
	return t.newEnd(token.pos, trim, token.line)
}

func (t *Tree) elseControl(trim trim) Node {
	var token item
	var pipe *PipeNode
	var keyword string
	peek := t.peekNonSpace()
	if peek.typ == itemIf || peek.typ == itemBranch {
		keyword = peek.val
		token = t.next() // Consume the "if"/"with"/etc. token.
		var eoptok item
		pipe, eoptok = t.pipeline("else "+keyword, itemRightDelim)
		trim.right = eoptok.trim.right
	} else {
		token = t.expect(itemRightDelim, "else")
		trim.right = token.trim.right
	}
	return t.newElse(token.pos, token.line, keyword, pipe, trim)
}

// command parses space-separated arguments up to a pipeline character or
// right delimiter. We consume the pipe character but leave the right
// delim to terminate the action.
func (t *Tree) command() *CommandNode {
	tok := t.peekNonSpace()
	cmd := t.newCommand(tok.pos, tok.line)
	for {
		t.peekNonSpace() // skip leading spaces.
		operand := t.operand()
		if operand != nil {
			cmd.append(operand)
		}
		token := t.next()
		switch token.typ {
		case itemSpace:
			continue
		case itemRightDelim, itemRightParen:
			t.backup()
		case itemPipe:
			// nothing here; break loop below
		case itemError, itemBool, itemChar, itemCharConstant, itemComment,
			itemComplex, itemAssign, itemDeclare, itemEOF, itemField,
			itemIdentifier, itemLeftDelim, itemLeftParen, itemNumber,
			itemRawString, itemString, itemText, itemVariable, itemKeyword,
			itemDot, itemElse, itemEnd, itemIf, itemNil, itemBranch:
			t.unexpected(token, "operand")
		}
		break
	}
	if len(cmd.Args) == 0 {
		t.errorf("empty command")
	}
	return cmd
}

// operand parses a term possibly followed by field accesses. A nil
// return means the next item is not an operand.
func (t *Tree) operand() Node {
	node := t.term()
	if node == nil {
		return nil
	}
	if t.peek().typ != itemField {
		return node
	}
	chainTok := t.peek()
	chain := t.newChain(chainTok.pos, node, chainTok.line)
	for t.peek().typ == itemField {
		chain.Add(t.next().val)
	}
	// Compatibility with original API: If the term is of type NodeField
	// or NodeVariable, just put more fields on the original.
	// Otherwise, keep the Chain node.
	switch node.Type() {
	case NodeField:
		return t.newField(chain.Position(), chain.String(), chainTok.line)
	case NodeVariable:
		return t.newVariable(chain.Position(), chain.String(), chainTok.line)
	case NodeBool, NodeString, NodeNumber, NodeNil, NodeDot:
		t.errorf("unexpected . after term %q", node.String())
	case NodeText, NodeAction, NodeChain, NodeCommand, NodeIdentifier,
		NodeBranch, NodeList, NodePipe, NodeTemplate, NodeComment,
		nodeElse, nodeEnd:
		// keep the chain.
	}
	return chain
}

// term parses a simple expression: literal, function identifier, dot,
// .Field, $, or parenthesized pipeline. A nil return means the next
// item is not a term. The case-per-token-type switch mirrors
// text/template's term() and is intrinsically branchy.
//
//nolint:cyclop // mirrors text/template's term() switch.
func (t *Tree) term() Node {
	switch token := t.nextNonSpace(); token.typ {
	case itemIdentifier:
		return NewIdentifier(token.val).SetTree(t).SetPos(token.pos).SetLine(token.line)
	case itemDot:
		return t.newDot(token.pos, token.line)
	case itemNil:
		return t.newNil(token.pos, token.line)
	case itemVariable:
		return t.useVar(token.pos, token.val, token.line)
	case itemField:
		return t.newField(token.pos, token.val, token.line)
	case itemBool:
		return t.newBool(token.pos, token.val == "true", token.line)
	case itemCharConstant, itemComplex, itemNumber:
		number, err := t.newNumber(token.pos, token.val, token.typ, token.line)
		if err != nil {
			t.error(err)
		}
		return number
	case itemLeftParen:
		pipe, _ := t.pipeline("parenthesized pipeline", itemRightParen)
		return pipe
	case itemString, itemRawString:
		s, err := strconv.Unquote(token.val)
		if err != nil {
			t.error(err)
		}
		return t.newString(token.pos, token.val, s, token.line)
	case itemError, itemChar, itemComment, itemAssign, itemDeclare,
		itemEOF, itemLeftDelim, itemPipe, itemRightDelim, itemRightParen,
		itemSpace, itemText, itemKeyword, itemElse, itemEnd, itemIf,
		itemBranch:
		// Not a term.
	}
	t.backup()
	return nil
}

// useVar returns a node for a variable reference.
func (t *Tree) useVar(pos Pos, name string, line int) Node {
	return t.newVariable(pos, name, line)
}
