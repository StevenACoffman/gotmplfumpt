// indent.go is the fallback formatter, used when gofumpt rejects the stub
// (the template isn't valid whole-file Go). It re-applies a tab indent equal to
// the Go-brace depth alone, scanned from the Literal slices only (the tiling has
// already carved actions out, so the Go scan never meets a "{{"). Control tags
// are indentation-invisible — they render to no Go braces, so they contribute no
// depth, matching the gofumpt path. Actions and literals are kept verbatim;
// Define blocks and multi-line comments pass through untouched. "Strip leading
// whitespace, re-add depth tabs" is a pure function of structure, so the output
// is idempotent by construction.

package format

import (
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// brace is one open Go bracket the indenter has seen but not yet closed. line
// is the source line (0-based) it opened on; incremented records whether
// opening it raised the indent — true only for the first still-open bracket a
// line contributes, so several brackets opened on one line count as one level.
// kind is the opener byte ('{', '(', '['); isSwitch is set only for a '{' that
// opens a switch/select body, so its case/default labels can be dedented.
type brace struct {
	line        int
	kind        byte
	incremented bool
	isSwitch    bool
}

// depthIndenter walks the source once, tracking Go-brace depth from a scan of
// the literal text, and emits each line re-indented to that depth.
type depthIndenter struct {
	src    string
	slices []tiling.RawSlice
	i      int // scan cursor into src
	si     int // index of the slice containing i

	out     []string // finished lines, joined with "\n" at the end
	depth   int      // running indent depth at the current line's start
	stack   []brace  // open Go brackets, innermost last
	curLine int      // 0-based index of the line being scanned

	// current-line accumulation
	lineStart int
	linePre   int  // delta from leading close tokens (applied before indent)
	linePost  int  // delta from the rest of the line (applied after indent)
	leading   bool // still in the line's leading zone (no open/content yet)
	verbatim  bool // line overlaps a Define or multi-line Comment slice
	inSwitch  bool // line starts directly inside a switch/select body

	// Go lexical state; inRawString and inBlockComment persist across lines.
	inRawString    bool
	inBlockComment bool
	inString       bool
	inRune         bool
	inLineComment  bool
}

// tilingIndent re-indents til.Src by Go-brace depth, keeping every slice's
// bytes verbatim.
//
// Requires: til satisfies the tiling invariant (ScanTiling guarantees it).
// Ensures:  output is idempotent; Define and multi-line Comment slices are
//
//	emitted verbatim; every other line is stripped and re-indented
//	by its depth, with leading close tokens dedenting their line.
func tilingIndent(til tiling.Tiling) string {
	ix := &depthIndenter{src: til.Src, slices: til.Slices}
	return ix.run()
}

func (ix *depthIndenter) run() string {
	ix.startLine(0)
	for ix.i < len(ix.src) {
		ix.sync()
		if ix.src[ix.i] == '\n' {
			ix.flush(ix.i)
			ix.i++
			ix.curLine++
			ix.startLine(ix.i)
			continue
		}
		ix.step()
	}
	ix.flush(len(ix.src))
	return strings.Join(ix.out, "\n")
}

// sync advances si so slices[si] is the slice containing i.
func (ix *depthIndenter) sync() {
	for ix.si < len(ix.slices) && ix.i >= ix.slices[ix.si].Stop {
		ix.si++
	}
}

// cur returns the slice containing the cursor. The tiling is gap-free, so for
// any i < len(src) there is always one.
func (ix *depthIndenter) cur() tiling.RawSlice { return ix.slices[ix.si] }

// step consumes the token at the cursor, advancing i by its length and
// recording any depth or verbatim effect.
func (ix *depthIndenter) step() {
	s := ix.cur()
	switch s.Type {
	case tiling.Literal:
		ix.literalByte()
	case tiling.Define:
		ix.verbatim = true
		ix.i++
	case tiling.Comment:
		if strings.Contains(ix.src[s.Start:s.Stop], "\n") {
			ix.verbatim = true
		}
		ix.i++
	case tiling.Action, tiling.BlockOpen, tiling.BlockMid, tiling.BlockClose:
		// Control tags are indentation-invisible: {{if}}/{{range}}/{{with}}/
		// {{else}}/{{end}} render to no Go braces, so their bodies belong at the
		// surrounding Go level, exactly as the gofumpt path already produces.
		// Actions likewise never change depth. Advance past the byte.
		ix.i++
	}
}

// literalByte scans one byte of Go literal text, tracking string/comment
// state so brackets inside them are not counted, and recording bracket depth
// deltas. It advances i past the byte(s) it consumes.
func (ix *depthIndenter) literalByte() {
	c := ix.src[ix.i]
	switch {
	case ix.inRawString:
		if c == '`' {
			ix.inRawString = false
		}
		ix.i++
	case ix.inBlockComment:
		if c == '*' && ix.peek(1) == '/' {
			ix.inBlockComment = false
			ix.i += 2
			return
		}
		ix.i++
	case ix.inLineComment:
		ix.i++ // ends at newline, handled by run
	case ix.inString:
		ix.consumeEscaped('"', &ix.inString)
	case ix.inRune:
		ix.consumeEscaped('\'', &ix.inRune)
	default:
		ix.literalStart(c)
	}
}

// literalStart handles a byte outside any Go string/comment: it opens a
// string/comment substate or records a bracket.
func (ix *depthIndenter) literalStart(c byte) {
	if !ix.openGoSubstate(c) {
		ix.recordBracket(c)
	}
	ix.i++
}

// openGoSubstate enters a Go string, rune, or comment substate if c begins
// one, consuming the extra byte of a two-byte comment introducer. Returns
// true when a substate was entered.
func (ix *depthIndenter) openGoSubstate(c byte) bool {
	switch {
	case c == '`':
		ix.inRawString = true
	case c == '"':
		ix.inString = true
	case c == '\'':
		ix.inRune = true
	case c == '/' && ix.peek(1) == '/':
		ix.inLineComment = true
		ix.i++
	case c == '/' && ix.peek(1) == '*':
		ix.inBlockComment = true
		ix.i++
	default:
		return false
	}
	return true
}

// recordBracket updates the bracket stack for one Go bracket byte and folds its
// indent effect into the current line's pre/post split. An opener raises the
// indent only when it is the first still-open bracket its line contributes, so
// a line opening several brackets indents its body one level — matching gofmt,
// not one level per bracket. A closer dedents by one only if the opener it
// matches had raised the indent, keeping the two balanced (and idempotent). A
// close with an empty stack is an unbalanced fragment and is ignored. Bytes
// that are not brackets have no effect.
func (ix *depthIndenter) recordBracket(c byte) {
	switch c {
	case '{', '(', '[':
		incremented := len(ix.stack) == 0 || ix.stack[len(ix.stack)-1].line != ix.curLine
		ix.stack = append(ix.stack, brace{
			line:        ix.curLine,
			kind:        c,
			incremented: incremented,
			isSwitch:    c == '{' && ix.opensSwitchBody(),
		})
		if incremented {
			ix.addDelta(+1, true)
		} else {
			ix.leading = false
		}
	case '}', ')', ']':
		if len(ix.stack) == 0 {
			return
		}
		top := ix.stack[len(ix.stack)-1]
		ix.stack = ix.stack[:len(ix.stack)-1]
		if top.incremented {
			ix.addDelta(-1, false)
		}
	}
}

// opensSwitchBody reports whether the text before the cursor on the current
// line begins with the keyword switch or select — the statements whose block
// dedents its case/default labels. Called from recordBracket with ix.i at the
// candidate '{', so ix.src[ix.lineStart:ix.i] is that '{'s line prefix.
func (ix *depthIndenter) opensSwitchBody() bool {
	prefix := strings.TrimLeft(ix.src[ix.lineStart:ix.i], " \t")
	return startsWithWord(prefix, "switch") || startsWithWord(prefix, "select")
}

// enclosingSwitch reports whether the innermost still-open '{' opened a
// switch/select body. Open parens and brackets are skipped: a case/default
// label is a direct child of the switch block, not of any expression nested
// inside it.
func (ix *depthIndenter) enclosingSwitch() bool {
	for k := len(ix.stack) - 1; k >= 0; k-- {
		if ix.stack[k].kind == '{' {
			return ix.stack[k].isSwitch
		}
	}
	return false
}

// isCaseLabel reports whether a stripped line begins a case or default label.
// Both are keywords, so a whole-word match cannot collide with an identifier.
func isCaseLabel(stripped string) bool {
	return startsWithWord(stripped, "case") || startsWithWord(stripped, "default")
}

// startsWithWord reports whether s begins with word as a whole token: the byte
// following word (if any) must not be an identifier byte, so "switch" matches
// "switch x {" and "default" matches "default:" but neither matches a longer
// identifier like "switchboard".
func startsWithWord(s, word string) bool {
	if !strings.HasPrefix(s, word) {
		return false
	}
	return len(s) == len(word) || !isIdentByte(s[len(word)])
}

// isIdentByte reports whether c can appear in a Go identifier.
func isIdentByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// consumeEscaped advances through a Go interpreted-string or rune literal
// byte, honoring backslash escapes and clearing the flag at the close quote.
func (ix *depthIndenter) consumeEscaped(quote byte, flag *bool) {
	c := ix.src[ix.i]
	switch {
	case c == '\\' && ix.i+1 < len(ix.src):
		ix.i += 2
	case c == quote:
		*flag = false
		ix.i++
	default:
		ix.i++
	}
}

func (ix *depthIndenter) peek(n int) byte {
	if ix.i+n < len(ix.src) {
		return ix.src[ix.i+n]
	}
	return 0
}

// addDelta folds a depth change into the current line's pre/post split: a
// close token in the leading zone dedents this line (pre); anything else
// adjusts the next line (post). An open token or non-close ends the leading
// zone.
func (ix *depthIndenter) addDelta(change int, openLike bool) {
	if ix.leading && change < 0 && !openLike {
		ix.linePre += change
	} else {
		ix.leading = false
		ix.linePost += change
	}
	if change > 0 || openLike {
		ix.leading = false
	}
}

// startLine resets per-line accumulation and the within-line Go substates
// (interpreted string, rune, line comment never cross a newline). It snapshots
// whether the line begins directly inside a switch/select body — computed here,
// before the line's own brackets are scanned, so a case/default label is judged
// against the block it lives in.
func (ix *depthIndenter) startLine(at int) {
	ix.lineStart = at
	ix.linePre = 0
	ix.linePost = 0
	ix.leading = true
	ix.verbatim = false
	ix.inString = false
	ix.inRune = false
	ix.inLineComment = false
	ix.inSwitch = ix.enclosingSwitch()
}

// flush emits the current line [lineStart, end): verbatim if it overlaps a
// Define/multi-line-Comment slice, otherwise stripped and re-indented by
// depth. Empty lines stay empty.
func (ix *depthIndenter) flush(end int) {
	line := ix.src[ix.lineStart:end]
	if ix.verbatim {
		ix.out = append(ix.out, line)
		return
	}
	stripped := strings.TrimLeft(line, " \t")
	if stripped == "" {
		ix.out = append(ix.out, "")
		return
	}
	ix.depth = clampDepth(ix.depth + ix.linePre)
	display := ix.depth
	if ix.inSwitch && isCaseLabel(stripped) {
		// A case/default label sits one level shallower than the statements it
		// introduces. Dedent the label's own line only; the running depth is
		// unchanged, so the case body stays at block-body level.
		display = clampDepth(display - 1)
	}
	ix.out = append(ix.out, strings.Repeat("\t", display)+stripped)
	ix.depth = clampDepth(ix.depth + ix.linePost)
}

func clampDepth(d int) int {
	if d < 0 {
		return 0
	}
	return d
}
