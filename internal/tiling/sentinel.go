// sentinel.go chooses the placeholder strings that stand in for non-literal
// slices in the stub. Two forms mirror the live format package: an identifier
// for output-producing actions (a valid Go expression) and a block comment
// for control tags and comments (valid in any position). Both embed the
// slice's tiling index, and both are self-delimiting — an identifier ends
// with "_" and a comment with "*/" — so one sentinel is never a prefix of
// another (e.g. …_a1_ is not found inside …_a12_).

package tiling

import (
	"fmt"
	"hash/fnv"
	"strings"
)

const sentinelBase = "__gtmpl"

// uniquePrefix returns an identifier prefix that does not occur in src, so no
// sentinel built from it can collide with source text. The default is used
// unless src already mentions it, in which case a source-hash suffix (then a
// counter, for the pathological case) makes it unique.
func uniquePrefix(src string) string {
	if !strings.Contains(src, sentinelBase) {
		return sentinelBase
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(src))
	for i := range len(src) + 1 {
		cand := fmt.Sprintf("%s_x%08x_%d", sentinelBase, h.Sum32(), i)
		if !strings.Contains(src, cand) {
			return cand
		}
	}
	// Unreachable: len(src)+1 distinct candidates cannot all be substrings of
	// a string of length len(src).
	return fmt.Sprintf("%s_x%08x_z", sentinelBase, h.Sum32())
}

// sentinelFor returns the placeholder for the non-literal slice at tiling index
// i. An output-producing Action becomes an identifier so it is a valid Go
// expression; every other non-literal slice becomes a block comment, valid in
// any position. The one exception: when commentStandalone is set, a standalone
// Action (alone on its source line, so at a statement/declaration boundary) also
// takes the comment form — an identifier is invalid at declaration position,
// but a comment is not. Stub, Restore, and VerifyFormatted all route through
// here, so the three agree on the form for every slice.
func (t Tiling) sentinelFor(i int) string {
	s := t.Slices[i]
	if s.Type == Action && (!t.commentStandalone || !s.Standalone) {
		return actionSentinel(t.prefix, i)
	}
	return commentSentinel(t.prefix, i)
}

// actionSentinel is the identifier placeholder for an output-producing action.
func actionSentinel(prefix string, id int) string {
	return fmt.Sprintf("%s_a%d_", prefix, id)
}

// commentSentinel is the block-comment placeholder for a control tag, template
// comment, define, or a standalone action held at declaration position.
func commentSentinel(prefix string, id int) string {
	return fmt.Sprintf("/*%s_b%d*/", prefix, id)
}
