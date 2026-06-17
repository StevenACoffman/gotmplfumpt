// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// eof is the sentinel rune returned by lexer.next at end of input.
const eof = -1

// Delimiter literals.
const (
	leftDelim    = "{{"
	rightDelim   = "}}"
	leftComment  = "/*"
	rightComment = "*/"
)

// Trim markers. If the action begins "{{- " rather than "{{", all
// space/tab/newlines preceding the action are trimmed; conversely if it
// ends " -}}" the leading spaces of the following text are trimmed.
const (
	trimMarker    = '-'
	trimMarkerLen = Pos(1 + 1) // marker plus space before or after.
)

// itemType identifiers, ordered so that all keywords sit above
// itemKeyword.
const (
	itemError        itemType = iota // error occurred; value is text of error
	itemBool                         // boolean constant
	itemChar                         // printable ASCII character; grab bag for comma etc.
	itemCharConstant                 // character constant
	itemComment                      // comment text
	itemComplex                      // complex constant (1+2i); imaginary is just a number
	itemAssign                       // equals ('=') introducing an assignment
	itemDeclare                      // colon-equals (':=') introducing a declaration
	itemEOF
	itemField      // alphanumeric identifier starting with '.'
	itemIdentifier // alphanumeric identifier not starting with '.'
	itemLeftDelim  // left action delimiter
	itemLeftParen  // '(' inside action
	itemNumber     // simple number, including imaginary
	itemPipe       // pipe symbol
	itemRawString  // raw quoted string (includes quotes)
	itemRightDelim // right action delimiter
	itemRightParen // ')' inside action
	itemSpace      // run of spaces separating arguments
	itemString     // quoted string (includes quotes)
	itemText       // plain text
	itemVariable   // variable starting with '$', such as '$' or  '$1' or '$hello'
	// Keywords appear after all the rest.
	itemKeyword // used only to delimit the keywords
	itemDot     // the cursor, spelled '.'
	itemElse    // else keyword
	itemEnd     // end keyword
	itemIf      // if keyword
	itemNil     // the untyped nil constant, easiest to treat as a keyword
	itemBranch  // some branch-y keyword (with, block, define, range)
)

// key maps reserved words to their itemType.
//
//nolint:gochecknoglobals // immutable lookup table.
var key = map[string]itemType{
	".":      itemDot,
	"block":  itemBranch, // cheap hack -- also slightly wrong, b/c block doesn't accept "else"
	"define": itemBranch, // cheap hack -- also wrong, b/c define doesn't accept "else"
	"else":   itemElse,
	"end":    itemEnd,
	"if":     itemIf,
	"range":  itemBranch,
	"nil":    itemNil,
	"with":   itemBranch,
}

// item represents a token or text string returned from the scanner.
type item struct {
	val  string
	pos  Pos
	line int
	trim trim
	typ  itemType
}

// itemType identifies the type of lex items.
type itemType int

// stateFn represents the state of the scanner as a function that returns
// the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	input           string
	item            item
	pos             Pos
	start           Pos
	parenDepth      int
	line            int
	startLine       int
	atEOF           bool
	insideAction    bool
	commentLeftTrim bool
}

// trim records the left/right trim markers attached to a delimiter.
type trim struct {
	left, right bool
}

// String renders the item for diagnostic messages.
func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case i.typ > itemKeyword:
		return fmt.Sprintf("<%s>", i.val)
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

// leftDelim returns the rendered left delimiter for this trim state.
func (t trim) leftDelim() string {
	if t.left {
		return "{{- "
	}
	return "{{ "
}

// rightDelim returns the rendered right delimiter for this trim state.
func (t trim) rightDelim() string {
	if t.right {
		return " -}}"
	}
	return " }}"
}

// rightDelimNoSpace returns the rendered right delimiter without a
// leading space, used when the close sits on its own line.
func (t trim) rightDelimNoSpace() string {
	if t.right {
		return "-}}"
	}
	return "}}"
}

// lex creates a new scanner for the input string.
func lex(input string) *lexer {
	return &lexer{
		input:     input,
		line:      1,
		startLine: 1,
	}
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.atEOF = true
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += Pos(w)
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune.
func (l *lexer) backup() {
	if !l.atEOF && l.pos > 0 {
		r, w := utf8.DecodeLastRuneInString(l.input[:l.pos])
		l.pos -= Pos(w)
		if r == '\n' {
			l.line--
		}
	}
}

// thisItem returns the item at the current input point with the
// specified type and advances the input.
func (l *lexer) thisItem(t itemType) item {
	i := item{
		typ:  t,
		pos:  l.start,
		val:  l.input[l.start:l.pos],
		line: l.startLine,
	}
	l.start = l.pos
	l.startLine = l.line
	return i
}

// emit passes the trailing text as an item back to the parser.
func (l *lexer) emit(t itemType) stateFn {
	return l.emitItem(l.thisItem(t))
}

// emitItem passes the specified item to the parser.
func (l *lexer) emitItem(i item) stateFn {
	l.item = i
	return nil
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.line += strings.Count(l.input[l.start:l.pos], "\n")
	l.start = l.pos
	l.startLine = l.line
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing back
// a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...any) stateFn {
	l.item = item{
		typ:  itemError,
		pos:  l.start,
		val:  fmt.Sprintf(format, args...),
		line: l.startLine,
	}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	l.item = item{typ: itemEOF, pos: l.pos, val: "EOF", line: l.startLine}
	state := lexText
	if l.insideAction {
		state = lexInsideAction
	}
	for {
		state = state(l)
		if state == nil {
			return l.item
		}
	}
}

// atRightDelim reports whether the lexer is at a right delimiter,
// possibly preceded by a trim marker.
func (l *lexer) atRightDelim() (delim, trimSpaces bool) {
	if hasRightTrimMarker(l.input[l.pos:]) &&
		strings.HasPrefix(l.input[l.pos+trimMarkerLen:], rightDelim) {
		return true, true
	}
	if strings.HasPrefix(l.input[l.pos:], rightDelim) {
		return true, false
	}
	return false, false
}

// atTerminator reports whether the input is at a valid termination
// character to appear after an identifier.
func (l *lexer) atTerminator() bool {
	r := l.peek()
	if isSpace(r) {
		return true
	}
	switch r {
	case eof, '.', ',', '|', ':', ')', '(':
		return true
	}
	return strings.HasPrefix(l.input[l.pos:], rightDelim)
}

// lexText scans until an opening action delimiter, "{{".
func lexText(l *lexer) stateFn {
	if x := strings.Index(l.input[l.pos:], leftDelim); x >= 0 {
		if x > 0 {
			l.pos += Pos(x)
			l.line += strings.Count(l.input[l.start:l.pos], "\n")
			i := l.thisItem(itemText)
			if i.val != "" {
				return l.emitItem(i)
			}
		}
		return lexLeftDelim
	}
	l.pos = Pos(len(l.input))
	if l.pos > l.start {
		l.line += strings.Count(l.input[l.start:l.pos], "\n")
		return l.emit(itemText)
	}
	return l.emit(itemEOF)
}

// lexLeftDelim scans the left delimiter, which is known to be present,
// possibly with a trim marker.
func lexLeftDelim(l *lexer) stateFn {
	l.pos += Pos(len(leftDelim))
	trimSpace := hasLeftTrimMarker(l.input[l.pos:])
	afterMarker := Pos(0)
	if trimSpace {
		afterMarker = trimMarkerLen
	}
	if strings.HasPrefix(l.input[l.pos+afterMarker:], leftComment) {
		l.commentLeftTrim = trimSpace
		l.pos += afterMarker
		l.ignore()
		return lexComment
	}
	i := l.thisItem(itemLeftDelim)
	i.trim.left = trimSpace
	l.insideAction = true
	l.pos += afterMarker
	l.ignore()
	l.parenDepth = 0
	return l.emitItem(i)
}

// lexComment scans a comment. The left comment marker is known to be
// present.
func lexComment(l *lexer) stateFn {
	l.pos += Pos(len(leftComment))
	x := strings.Index(l.input[l.pos:], rightComment)
	if x < 0 {
		return l.errorf("unclosed comment")
	}
	l.pos += Pos(x + len(rightComment))
	delim, trimSpace := l.atRightDelim()
	if !delim {
		return l.errorf("comment ends before closing delimiter")
	}
	i := l.thisItem(itemComment)
	i.trim.left = l.commentLeftTrim
	i.trim.right = trimSpace
	if trimSpace {
		l.pos += trimMarkerLen
	}
	l.pos += Pos(len(rightDelim))
	l.ignore()
	return l.emitItem(i)
}

// lexRightDelim scans the right delimiter, which is known to be present,
// possibly with a trim marker.
func lexRightDelim(l *lexer) stateFn {
	_, trimSpace := l.atRightDelim()
	if trimSpace {
		l.pos += trimMarkerLen
		l.ignore()
	}
	l.pos += Pos(len(rightDelim))
	i := l.thisItem(itemRightDelim)
	i.trim.right = trimSpace
	l.insideAction = false
	return l.emitItem(i)
}

// lexInsideAction scans the elements inside action delimiters.
func lexInsideAction(l *lexer) stateFn {
	delim, _ := l.atRightDelim()
	if delim {
		if l.parenDepth == 0 {
			return lexRightDelim
		}
		return l.errorf("unclosed left paren")
	}
	r := l.next()
	if state := lexPunctOrEOF(l, r); state != nil {
		return state
	}
	if state := lexLiteralStart(l, r); state != nil {
		return state
	}
	return lexCatchAll(l, r)
}

// lexPunctOrEOF handles EOF and single-rune operators (=, :, |, etc.).
// Returns nil if r is not one of these.
func lexPunctOrEOF(l *lexer, r rune) stateFn {
	switch r {
	case eof:
		return l.errorf("unclosed action")
	case '=':
		return l.emit(itemAssign)
	case ':':
		if l.next() != '=' {
			return l.errorf("expected :=")
		}
		return l.emit(itemDeclare)
	case '|':
		return l.emit(itemPipe)
	case '(':
		l.parenDepth++
		return l.emit(itemLeftParen)
	case ')':
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right paren")
		}
		return l.emit(itemRightParen)
	}
	if isSpace(r) {
		l.backup() // Put space back in case we have " -}}".
		return lexSpace
	}
	return nil
}

// lexLiteralStart handles tokens whose first rune signals a literal
// (string, raw string, char, variable, dot/field, number). Returns nil
// if r is not one of these starts.
func lexLiteralStart(l *lexer, r rune) stateFn {
	switch r {
	case '"':
		return lexQuote
	case '`':
		return lexRawQuote
	case '$':
		return lexVariable
	case '\'':
		return lexChar
	case '.':
		return lexDotOrNumber(l)
	}
	if r == '+' || r == '-' || ('0' <= r && r <= '9') {
		l.backup()
		return lexNumber
	}
	return nil
}

// lexDotOrNumber resolves '.' as either a field accessor or the start of
// a number. The dot has already been consumed.
func lexDotOrNumber(l *lexer) stateFn {
	if l.pos < Pos(len(l.input)) {
		next := l.input[l.pos]
		if next < '0' || '9' < next {
			return lexField
		}
	}
	l.backup()
	return lexNumber
}

// lexCatchAll handles identifiers and printable-ASCII characters; falls
// through to an error.
func lexCatchAll(l *lexer, r rune) stateFn {
	if isAlphaNumeric(r) {
		l.backup()
		return lexIdentifier
	}
	if r <= unicode.MaxASCII && unicode.IsPrint(r) {
		return l.emit(itemChar)
	}
	return l.errorf("unrecognized character in action: %#U", r)
}

// lexSpace scans a run of space characters. The first space is known to
// be present.
func lexSpace(l *lexer) stateFn {
	var numSpaces int
	for isSpace(l.peek()) {
		l.next()
		numSpaces++
	}
	// Be careful about a trim-marked closing delimiter, which has a
	// minus after a space.
	if hasRightTrimMarker(l.input[l.pos-1:]) &&
		strings.HasPrefix(l.input[l.pos-1+trimMarkerLen:], rightDelim) {
		l.backup() // Before the space.
		if numSpaces == 1 {
			return lexRightDelim
		}
	}
	return l.emit(itemSpace)
}

// lexIdentifier scans an alphanumeric identifier.
func lexIdentifier(l *lexer) stateFn {
	for {
		r := l.next()
		if isAlphaNumeric(r) {
			continue
		}
		l.backup()
		word := l.input[l.start:l.pos]
		if !l.atTerminator() {
			return l.errorf("bad character %#U", r)
		}
		switch {
		case key[word] > itemKeyword:
			return l.emit(key[word])
		case word[0] == '.':
			return l.emit(itemField)
		case word == "true", word == "false":
			return l.emit(itemBool)
		default:
			return l.emit(itemIdentifier)
		}
	}
}

// lexField scans a field: .Alphanumeric. The . has been scanned.
func lexField(l *lexer) stateFn {
	return lexFieldOrVariable(l, itemField)
}

// lexVariable scans a variable: $Alphanumeric. The $ has been scanned.
func lexVariable(l *lexer) stateFn {
	if l.atTerminator() {
		return l.emit(itemVariable)
	}
	return lexFieldOrVariable(l, itemVariable)
}

// lexFieldOrVariable scans a field or variable.
func lexFieldOrVariable(l *lexer, typ itemType) stateFn {
	if l.atTerminator() {
		if typ == itemVariable {
			return l.emit(itemVariable)
		}
		return l.emit(itemDot)
	}
	var r rune
	for {
		r = l.next()
		if !isAlphaNumeric(r) {
			l.backup()
			break
		}
	}
	if !l.atTerminator() {
		return l.errorf("bad character %#U", r)
	}
	return l.emit(typ)
}

// lexChar scans a character constant.
func lexChar(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated character constant")
		case '\'':
			break Loop
		}
	}
	return l.emit(itemCharConstant)
}

// lexNumber scans a number: decimal, octal, hex, float, or imaginary.
func lexNumber(l *lexer) stateFn {
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	if sign := l.peek(); sign == '+' || sign == '-' {
		if !l.scanNumber() || l.input[l.pos-1] != 'i' {
			return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
		}
		return l.emit(itemComplex)
	}
	return l.emit(itemNumber)
}

// digitClass returns the digit-character set for a numeric literal based
// on the optional 0x/0o/0b prefix already consumed via l.accept.
func (l *lexer) digitClass() string {
	if !l.accept("0") {
		return "0123456789_"
	}
	// Leading 0 does not mean octal in floats.
	switch {
	case l.accept("xX"):
		return "0123456789abcdefABCDEF_"
	case l.accept("oO"):
		return "01234567_"
	case l.accept("bB"):
		return "01_"
	}
	return "0123456789_"
}

// scanNumber consumes a Go-style numeric literal at the cursor. Returns
// true on success.
func (l *lexer) scanNumber() bool {
	l.accept("+-") // Optional leading sign.
	digits := l.digitClass()
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	// Decimal-only literals may have an 'e' exponent.
	if len(digits) == 10+1 && l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789_")
	}
	// Hex floats may have a 'p' exponent.
	if len(digits) == 16+6+1 && l.accept("pP") {
		l.accept("+-")
		l.acceptRun("0123456789_")
	}
	l.accept("i")
	// Next thing mustn't be alphanumeric.
	if isAlphaNumeric(l.peek()) {
		l.next()
		return false
	}
	return true
}

// lexQuote scans a quoted string.
func lexQuote(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case '"':
			break Loop
		}
	}
	return l.emit(itemString)
}

// lexRawQuote scans a raw quoted string.
func lexRawQuote(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case eof:
			return l.errorf("unterminated raw quoted string")
		case '`':
			break Loop
		}
	}
	return l.emit(itemRawString)
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

// isAlphaNumeric reports whether r is alphabetic, a digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func hasLeftTrimMarker(s string) bool {
	return len(s) >= 2 && s[0] == trimMarker && isSpace(rune(s[1]))
}

func hasRightTrimMarker(s string) bool {
	return len(s) >= 2 && isSpace(rune(s[0])) && s[1] == trimMarker
}
