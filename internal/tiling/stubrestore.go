// stubrestore.go maps a tiling to and from the stub handed to gofumpt. Stub
// replaces every non-literal slice with a sentinel and emits literals
// verbatim; Restore walks the tiling and the formatted stub in lockstep,
// splicing each slice's original source bytes back over its sentinel. The two
// are inverse functions: for an identity formatter, Restore(Stub(t)) == t.Src.

package tiling

import (
	"fmt"
	"strings"
)

// Stub renders the tiling as Go source with each non-literal slice replaced
// by a sentinel and each literal emitted verbatim. The verbatim literals are
// what gofumpt reflows; the sentinels survive gofumpt unchanged.
//
// Ensures: every non-literal slice contributes exactly one sentinel, in order.
func (t Tiling) Stub() string {
	var b strings.Builder
	b.Grow(len(t.Src))
	for i, s := range t.Slices {
		if s.Type == Literal {
			b.WriteString(t.Raw(s))
			continue
		}
		b.WriteString(sentinel(t.prefix, i, s.Type))
	}
	return b.String()
}

// Restore reconstructs formatted source from a gofumpt-formatted stub. It
// finds each non-literal slice's sentinel in order and writes that slice's
// original source bytes in its place. The text between the previous sentinel
// and this one — the "gap", which gofumpt owns — is kept only when a Literal
// slice precedes this one in the tiling. When two non-literal slices are
// adjacent in source, any whitespace gofumpt inserted between their sentinels
// is dropped, which subsumes the live package's tightenAdjacentSentinels pass.
// A multi-line slice's continuation lines are realigned to the column gofumpt
// placed its sentinel at (see reindentContinuation).
//
// Requires: formatted is the gofumpt output of t.Stub(); every sentinel
// appears exactly once. Ensures: for an identity formatter, the result
// equals t.Src.
func (t Tiling) Restore(formatted string) (string, error) {
	var b strings.Builder
	b.Grow(len(formatted))
	cur := 0
	for i, s := range t.Slices {
		if s.Type == Literal {
			continue
		}
		needle := sentinel(t.prefix, i, s.Type)
		off := strings.Index(formatted[cur:], needle)
		if off < 0 {
			return "", fmt.Errorf(
				"tiling: sentinel %q for slice %d not found in formatted output",
				needle,
				i,
			)
		}
		if precededByLiteral(t.Slices, i) {
			b.WriteString(formatted[cur : cur+off])
		}
		raw := t.Raw(s)
		if strings.Contains(raw, "\n") {
			raw = reindentContinuation(formatted, cur+off, raw)
		}
		b.WriteString(raw)
		cur += off + len(needle)
	}
	b.WriteString(formatted[cur:])
	return b.String(), nil
}

// precededByLiteral reports whether the slice at index i is immediately
// preceded by a Literal slice. Because ScanTiling omits zero-length literal
// runs, a false result means slice i was adjacent to the previous action in
// source.
func precededByLiteral(slices []RawSlice, i int) bool {
	return i > 0 && slices[i-1].Type == Literal
}
