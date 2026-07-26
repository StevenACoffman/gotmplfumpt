// indent.go is the fallback formatter, used when gofumpt rejects the stub
// (the template isn't valid whole-file Go). It re-applies a tab indent equal
// to the combined template-block depth (read from the tiling's typed slices)
// and Go-brace depth (from a scan of the Literal slices only — the tiling has
// already carved actions out, so the Go scan never meets a "{{"). Actions and
// literals are kept verbatim; Define blocks and multi-line comments pass
// through untouched. "Strip leading whitespace, re-add depth tabs" is a pure
// function of structure, so the output is idempotent by construction.

package format

import (
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// depthIndenter walks the source once, merging template-structure events from
// the tiling with a Go-brace scan of literal text, and emits indented lines.
type depthIndenter struct {
	src    string
	slices []tiling.RawSlice
	i      int // scan cursor into src
	si     int // index of the slice containing i

	out   []string // finished lines, joined with "\n" at the end
	depth int      // running combined depth at the current line's start

	// current-line accumulation
	lineStart int
	linePre   int  // delta from leading close tokens (applied before indent)
	linePost  int  // delta from the rest of the line (applied after indent)
	leading   bool // still in the line's leading zone (no open/content yet)
	verbatim  bool // line overlaps a Define or multi-line Comment slice

	// Go lexical state; inRawString and inBlockComment persist across lines.
	inRawString    bool
	inBlockComment bool
	inString       bool
	inRune         bool
	inLineComment  bool
}

// tilingIndent re-indents til.Src by combined template-block + Go-brace depth,
// keeping every slice's bytes verbatim.
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
	case tiling.BlockOpen, tiling.BlockClose, tiling.BlockMid:
		if ix.i == s.Start {
			ix.blockDelta(s.Type)
		}
		ix.i++
	case tiling.Action:
		ix.i++
	}
}

// blockDelta applies a control tag's depth change at its start: an opening tag
// indents what follows; a closing tag dedents this line; an {{else}} dedents
// its own line to the block level but keeps the following body indented.
func (ix *depthIndenter) blockDelta(t tiling.SliceType) {
	switch t {
	case tiling.BlockOpen:
		ix.addDelta(+1, true)
	case tiling.BlockClose:
		ix.addDelta(-1, false)
	case tiling.BlockMid:
		ix.addDelta(-1, false)
		ix.addDelta(+1, true)
	case tiling.Literal, tiling.Action, tiling.Comment, tiling.Define:
		// Not control tags; blockDelta is only called for the three above.
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
// string/comment substate or records a bracket delta.
func (ix *depthIndenter) literalStart(c byte) {
	if !ix.openGoSubstate(c) {
		if delta, ok := goBracketDelta(c); ok {
			ix.addDelta(delta, delta > 0)
		}
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

// goBracketDelta returns the depth delta for a single Go bracket byte and
// whether c is one.
func goBracketDelta(c byte) (delta int, ok bool) {
	switch c {
	case '{', '(', '[':
		return +1, true
	case '}', ')', ']':
		return -1, true
	}
	return 0, false
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
// (interpreted string, rune, line comment never cross a newline).
func (ix *depthIndenter) startLine(at int) {
	ix.lineStart = at
	ix.linePre = 0
	ix.linePost = 0
	ix.leading = true
	ix.verbatim = false
	ix.inString = false
	ix.inRune = false
	ix.inLineComment = false
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
	ix.out = append(ix.out, strings.Repeat("\t", ix.depth)+stripped)
	ix.depth = clampDepth(ix.depth + ix.linePost)
}

func clampDepth(d int) int {
	if d < 0 {
		return 0
	}
	return d
}
