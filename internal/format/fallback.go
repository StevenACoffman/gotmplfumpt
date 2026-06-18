// fallback.go implements the brace-counting fallback indenter (Strategy
// B). When gofumpt rejects the stubbed Go, or when the structural verify
// fails, we still want to emit a deterministic, idempotent format —
// indented according to Go brace nesting plus template branch nesting.
//
// The approach: take the AST printer's flat output, strip every line's
// leading whitespace, then re-add an indent computed by walking the
// text line-by-line and tracking depth from a small state machine that
// understands Go braces (outside strings/runes/comments) and template
// open/close actions. {{ define }} bodies are emitted verbatim because
// they declare a separate template namespace.
//
// "Strip and re-add" is a pure function — running it on its own output
// produces the same result, so the fallback is idempotent by
// construction.

package format

import (
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

// depthScanner tracks lexer state that persists across lines (block
// comments, raw strings, template comments, define blocks) plus the
// running depth.
type depthScanner struct {
	depth             int
	defineDepth       int
	inBlockComment    bool
	inRawString       bool
	inTemplateComment bool
}

// lineWalker scans one line, tracking whether the cursor is inside a Go
// string/rune/comment/raw-string, a template comment, or a template
// action/define, and emitting brace deltas via step().
type lineWalker struct {
	line              string
	pos               int
	defineDepth       int
	inDoubleString    bool
	inRune            bool
	inLineComment     bool
	inBlockComment    bool
	inRawString       bool
	inTemplateComment bool // inside {{/* ... */}}, including across lines
	inAction          bool // inside {{ ... }}
}

// fallbackFormat produces an idempotent format without gofumpt. Used
// when the gofumpt path can't be taken: the stubbed Go didn't parse, or
// the structural verify caught a corruption.
//
// Requires: root produced by parse.Parse.
// Ensures:  output is idempotent; indent applied is
//
//	`goBraceDepth + templateBranchDepth` at each line.
func fallbackFormat(root parse.Node) string {
	return reindentByDepth(root.String())
}

// reindentByDepth strips each line's leading whitespace and reapplies a
// tab indent equal to the combined Go-brace + template-branch depth at
// that line. Lines that are empty after stripping stay empty. Lines
// inside a {{ define }} block, or inside a multi-line {{/* … */}}
// template comment, are emitted verbatim — both declare content whose
// indentation is deliberate and shouldn't be reflowed by the brace
// counter.
func reindentByDepth(s string) string {
	lines := strings.Split(s, "\n")
	scanner := newDepthScanner()
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		insideDefineAtStart := scanner.defineDepth > 0
		insideTemplateCommentAtStart := scanner.inTemplateComment
		pre, post := scanner.deltaForLine(strings.TrimLeft(raw, " \t"))
		insideDefineAtEnd := scanner.defineDepth > 0
		insideTemplateCommentAtEnd := scanner.inTemplateComment
		if insideDefineAtStart || insideDefineAtEnd ||
			insideTemplateCommentAtStart || insideTemplateCommentAtEnd {
			out = append(out, raw)
			continue
		}
		stripped := strings.TrimLeft(raw, " \t")
		if stripped == "" {
			out = append(out, "")
			continue
		}
		scanner.depth += pre
		if scanner.depth < 0 {
			scanner.depth = 0
		}
		indent := strings.Repeat("\t", scanner.depth)
		out = append(out, indent+stripped)
		scanner.depth += post
		if scanner.depth < 0 {
			scanner.depth = 0
		}
	}
	return strings.Join(out, "\n")
}

func newDepthScanner() *depthScanner { return &depthScanner{} }

// deltaForLine scans line, advancing scanner state for any cross-line
// constructs and returns the brace delta split into:
//
//   - pre:  delta from leading close-tokens (`}`/`)`/`]` or `{{end}}`)
//     that should reduce depth BEFORE the line is indented;
//   - post: the remaining delta, applied AFTER indenting.
//
// Tokens inside Go string/rune/comment regions, template actions, and
// {{ define }} blocks are not counted toward depth.
func (s *depthScanner) deltaForLine(line string) (pre, post int) {
	leadingZone := true
	walker := lineWalker{
		line:              line,
		inBlockComment:    s.inBlockComment,
		inRawString:       s.inRawString,
		inTemplateComment: s.inTemplateComment,
		defineDepth:       s.defineDepth,
	}
	for walker.pos < len(walker.line) {
		change, isOpenLike := walker.step()
		if change == 0 {
			continue
		}
		if leadingZone && change < 0 && !isOpenLike {
			pre += change
		} else {
			leadingZone = false
			post += change
		}
		if change > 0 || isOpenLike {
			leadingZone = false
		}
	}
	s.inBlockComment = walker.inBlockComment
	s.inRawString = walker.inRawString
	s.inTemplateComment = walker.inTemplateComment
	s.defineDepth = walker.defineDepth
	return pre, post
}

// step advances pos by one logical token, returns (delta, isOpenLike).
// delta is -1 for a depth-decreasing token (`}`/`)`/`]` or `{{end}}`),
// +1 for a depth-increasing token (`{`/`(`/`[` or `{{if/range/with}}`),
// 0 otherwise. isOpenLike marks tokens that are increases (so they exit
// the leading-close zone in deltaForLine).
func (w *lineWalker) step() (int, bool) {
	switch {
	case w.inTemplateComment:
		return w.consumeTemplateCommentSegment()
	case w.inBlockComment:
		return w.consumeBlockCommentSegment()
	case w.inRawString:
		return w.consumeRawStringSegment()
	case w.inDoubleString:
		return w.consumeDoubleStringSegment()
	case w.inRune:
		return w.consumeRuneSegment()
	case w.inLineComment:
		// Line comments end at line boundary.
		w.pos = len(w.line)
		return 0, false
	case w.inAction:
		return w.consumeActionSegment()
	}
	return w.consumeNormal()
}

// consumeNormal handles one rune of input outside any sub-state.
// Dispatches by leading-byte class to keep complexity low.
func (w *lineWalker) consumeNormal() (int, bool) {
	r := w.line[w.pos]
	// Always interpret action delimiters, even inside a define, so we
	// can detect the closing {{end}}.
	if r == '{' && w.peekIs("{{") {
		return w.openTemplateAction()
	}
	if w.defineDepth > 0 {
		w.pos++
		return 0, false
	}
	if delta, ok := w.consumeGoBracket(r); ok {
		return delta, delta > 0
	}
	if w.consumeGoSubstate(r) {
		return 0, false
	}
	w.pos++
	return 0, false
}

// consumeGoBracket returns the depth delta for a single bracket
// character ({}/()/[]). ok==false means r isn't a bracket.
func (w *lineWalker) consumeGoBracket(r byte) (delta int, ok bool) {
	switch r {
	case '{', '(', '[':
		w.pos++
		return +1, true
	case '}', ')', ']':
		w.pos++
		return -1, true
	}
	return 0, false
}

// consumeGoSubstate transitions into a string/rune/comment substate if
// r begins one. Returns true when the cursor advanced.
func (w *lineWalker) consumeGoSubstate(r byte) bool {
	switch {
	case r == '/' && w.peekIs("//"):
		w.inLineComment = true
		w.pos += 2
		return true
	case r == '/' && w.peekIs("/*"):
		w.inBlockComment = true
		w.pos += 2
		return true
	case r == '"':
		w.inDoubleString = true
		w.pos++
		return true
	case r == '`':
		w.inRawString = true
		w.pos++
		return true
	case r == '\'':
		w.inRune = true
		w.pos++
		return true
	}
	return false
}

// openTemplateAction is called at a "{{" prefix; it classifies the
// action's keyword (if any) and consumes through the matching "}}",
// returning the brace delta to apply for the action as a whole. A
// "{{/* … */}}" template comment is recognized here (after any trim
// marker) and dispatched to the cross-line template-comment state.
func (w *lineWalker) openTemplateAction() (int, bool) {
	w.pos += 2
	for w.pos < len(w.line) && (w.line[w.pos] == '-' || w.line[w.pos] == ' ' || w.line[w.pos] == '\t') {
		w.pos++
	}
	if w.peekIs("/*") {
		w.pos += 2
		w.inTemplateComment = true
		return 0, false
	}
	delta, openLike := w.classifyActionKeyword()
	w.inAction = true
	return delta, openLike
}

// consumeTemplateCommentSegment consumes input until the closing "*/}}"
// (optionally with right-trim marker "*/ -}}"). If the close isn't on
// this line, the cross-line inTemplateComment state stays set so the
// caller emits subsequent lines verbatim.
func (w *lineWalker) consumeTemplateCommentSegment() (int, bool) {
	idx := strings.Index(w.line[w.pos:], "*/")
	if idx < 0 {
		w.pos = len(w.line)
		return 0, false
	}
	// Advance past "*/" then skip any trim marker and the closing "}}".
	w.pos += idx + 2
	for w.pos < len(w.line) && (w.line[w.pos] == '-' || w.line[w.pos] == ' ' || w.line[w.pos] == '\t') {
		w.pos++
	}
	if w.peekIs("}}") {
		w.pos += 2
		w.inTemplateComment = false
		return 0, false
	}
	// "*/" was inside the comment body, not the closer. Keep scanning.
	return w.consumeTemplateCommentSegment()
}

// classifyActionKeyword peeks at the action's first word and returns
// the brace delta the action contributes. {{ define }} is tracked
// separately via defineDepth — its body is opaque to the brace counter,
// so it contributes no depth delta.
func (w *lineWalker) classifyActionKeyword() (int, bool) {
	rest := w.line[w.pos:]
	switch {
	case startsWithWord(rest, "define"):
		w.defineDepth++
		return 0, false
	case startsWithWord(rest, "end"):
		if w.defineDepth > 0 {
			w.defineDepth--
			return 0, false
		}
		return -1, false
	}
	if w.defineDepth > 0 {
		return 0, false
	}
	switch {
	case startsWithWord(rest, "if"),
		startsWithWord(rest, "range"),
		startsWithWord(rest, "with"),
		startsWithWord(rest, "block"):
		return +1, true
	}
	return 0, false
}

func (w *lineWalker) consumeActionSegment() (int, bool) {
	if idx := strings.Index(w.line[w.pos:], "}}"); idx >= 0 {
		w.pos += idx + 2
		w.inAction = false
	} else {
		w.pos = len(w.line)
	}
	return 0, false
}

func (w *lineWalker) consumeBlockCommentSegment() (int, bool) {
	if idx := strings.Index(w.line[w.pos:], "*/"); idx >= 0 {
		w.pos += idx + 2
		w.inBlockComment = false
	} else {
		w.pos = len(w.line)
	}
	return 0, false
}

func (w *lineWalker) consumeRawStringSegment() (int, bool) {
	if idx := strings.IndexByte(w.line[w.pos:], '`'); idx >= 0 {
		w.pos += idx + 1
		w.inRawString = false
	} else {
		w.pos = len(w.line)
	}
	return 0, false
}

func (w *lineWalker) consumeDoubleStringSegment() (int, bool) {
	for w.pos < len(w.line) {
		c := w.line[w.pos]
		if c == '\\' && w.pos+1 < len(w.line) {
			w.pos += 2
			continue
		}
		if c == '"' {
			w.pos++
			w.inDoubleString = false
			return 0, false
		}
		w.pos++
	}
	return 0, false
}

func (w *lineWalker) consumeRuneSegment() (int, bool) {
	for w.pos < len(w.line) {
		c := w.line[w.pos]
		if c == '\\' && w.pos+1 < len(w.line) {
			w.pos += 2
			continue
		}
		if c == '\'' {
			w.pos++
			w.inRune = false
			return 0, false
		}
		w.pos++
	}
	return 0, false
}

func (w *lineWalker) peekIs(s string) bool {
	return strings.HasPrefix(w.line[w.pos:], s)
}

// startsWithWord reports whether s begins with word, followed by a
// non-identifier byte (or end of string).
func startsWithWord(s, word string) bool {
	if !strings.HasPrefix(s, word) {
		return false
	}
	if len(s) == len(word) {
		return true
	}
	return !isIdentByte(s[len(word)])
}

func isIdentByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
