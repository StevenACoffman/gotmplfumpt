// scan.go builds a Tiling from raw template source. It is a delimiter
// scanner over the source bytes, the Go analog of slicing from a lexer token
// stream (SPEC §8.2) rather than reconstructing from the parse AST — whose
// TextNode text is already whitespace-trimmed and whose delimiters are
// discarded. Literal slices are therefore verbatim source substrings, so trim
// markers and inter-action whitespace round-trip exactly.

package tiling

import (
	"fmt"
	"strings"
)

// ScanTiling partitions src into a gap-free tiling of typed slices. In valid
// Go-template source every "{{" is an action delimiter (a literal "{{" must
// be written {{"{{"}}), so the scanner can treat every "{{" as an action
// start. Zero-length literal runs between adjacent actions are not emitted,
// which is what lets restore distinguish "source has whitespace here" from
// "these actions are adjacent" without a separate adjacency pass.
//
// Requires: src parses as a Go text/template (callers parse first). Ensures:
// the returned tiling satisfies Check; an unterminated action is an error.
func ScanTiling(src string) (Tiling, error) {
	var slices []RawSlice
	litStart := 0
	for i := 0; i < len(src); {
		if !strings.HasPrefix(src[i:], "{{") {
			i++
			continue
		}
		if i > litStart {
			slices = append(slices, RawSlice{Type: Literal, Start: litStart, Stop: i})
		}
		end, typ, ok := scanAction(src, i)
		if !ok {
			return Tiling{}, fmt.Errorf("tiling: unterminated action at byte %d", i)
		}
		if typ == Define {
			end, ok = defineBlockEnd(src, i)
			if !ok {
				return Tiling{}, fmt.Errorf("tiling: unterminated define block at byte %d", i)
			}
		}
		slices = append(slices, RawSlice{Type: typ, Start: i, Stop: end})
		i = end
		litStart = end
	}
	if litStart < len(src) {
		slices = append(slices, RawSlice{Type: Literal, Start: litStart, Stop: len(src)})
	}
	t := Tiling{Src: src, Slices: slices, prefix: uniquePrefix(src)}
	if err := t.Check(); err != nil {
		return Tiling{}, err
	}
	return t, nil
}

// defineBlockEnd returns the offset just past the {{end}} that closes the
// {{define}} block opening at open. The whole block becomes one opaque slice:
// its body is passed through verbatim and never handed to gofumpt, because
// define bodies are frequently Go fragments that do not stand alone. Depth
// counts nested block-opening tags so an inner {{end}} does not close the
// block. ok is false if no matching {{end}} is found.
func defineBlockEnd(src string, open int) (end int, ok bool) {
	depth := 0
	for i := open; i < len(src); {
		if !strings.HasPrefix(src[i:], "{{") {
			i++
			continue
		}
		aEnd, typ, aOK := scanAction(src, i)
		if !aOK {
			return len(src), false
		}
		switch typ {
		case Define, BlockOpen:
			depth++
		case BlockClose:
			depth--
			if depth == 0 {
				return aEnd, true
			}
		case Literal, Action, BlockMid, Comment:
			// Neither opens nor closes a block.
		}
		i = aEnd
	}
	return len(src), false
}

// scanAction returns the offset just past the closing "}}" of the action
// beginning at the "{{" at open, plus its classified type. ok is false when
// the action is unterminated.
func scanAction(src string, open int) (end int, typ SliceType, ok bool) {
	body := open + 2 // past "{{"
	kw := skipLeftTrimAndSpace(src, body)
	if strings.HasPrefix(src[kw:], "/*") {
		end, ok = commentEnd(src, kw+2)
		return end, Comment, ok
	}
	end, ok = actionEnd(src, body)
	return end, classify(leadingWord(src, kw)), ok
}

// actionEnd scans from the action interior at i to the offset just past the
// first "}}" that is not inside a string, raw-string, or char literal.
func actionEnd(src string, i int) (end int, ok bool) {
	for i < len(src) {
		switch src[i] {
		case '"':
			i = skipQuoted(src, i+1, '"', true)
		case '\'':
			i = skipQuoted(src, i+1, '\'', true)
		case '`':
			i = skipQuoted(src, i+1, '`', false)
		case '}':
			if strings.HasPrefix(src[i:], "}}") {
				return i + 2, true
			}
			i++
		default:
			i++
		}
	}
	return len(src), false
}

// skipQuoted returns the offset just past the closing quote of a literal that
// opened at i (the byte after the opening quote). When escapes is true a
// backslash escapes the next byte (interpreted and char literals); raw
// strings pass escapes false.
func skipQuoted(src string, i int, quote byte, escapes bool) int {
	for i < len(src) {
		switch {
		case escapes && src[i] == '\\' && i+1 < len(src):
			i += 2
		case src[i] == quote:
			return i + 1
		default:
			i++
		}
	}
	return len(src)
}

// commentEnd returns the offset just past the "}}" closing a {{/* … */}}
// comment whose body begins at afterOpen (just past "/*"). The bytes between
// "*/" and "}}" are only whitespace and an optional trim marker, so the first
// "}}" after "*/" is the true close.
func commentEnd(src string, afterOpen int) (end int, ok bool) {
	star := strings.Index(src[afterOpen:], "*/")
	if star < 0 {
		return len(src), false
	}
	rest := afterOpen + star + 2
	rd := strings.Index(src[rest:], "}}")
	if rd < 0 {
		return len(src), false
	}
	return rest + rd + 2, true
}

// classify maps an action's leading keyword to its slice type. A pipeline
// that does not begin with a control keyword is an output-producing Action.
func classify(word string) SliceType {
	switch word {
	case "if", "range", "with", "block":
		return BlockOpen
	case "else":
		return BlockMid
	case "end":
		return BlockClose
	case "define":
		return Define
	default:
		return Action
	}
}

// skipLeftTrimAndSpace advances past a left trim marker ("- " — a dash
// immediately followed by whitespace) and any following whitespace, so the
// keyword can be read regardless of trimming.
func skipLeftTrimAndSpace(src string, i int) int {
	if i+1 < len(src) && src[i] == '-' && isSpace(src[i+1]) {
		i++
	}
	for i < len(src) && isSpace(src[i]) {
		i++
	}
	return i
}

// leadingWord returns the run of ASCII letters at i (empty if none). Template
// control keywords are all lowercase ASCII.
func leadingWord(src string, i int) string {
	j := i
	for j < len(src) && isLetter(src[j]) {
		j++
	}
	return src[i:j]
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
