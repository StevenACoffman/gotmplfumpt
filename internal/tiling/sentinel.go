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

// sentinel returns the placeholder for the non-literal slice at tiling index
// id. Action slices become an identifier; all other non-literal slices become
// a block comment.
func sentinel(prefix string, id int, typ SliceType) string {
	if typ == Action {
		return fmt.Sprintf("%s_a%d_", prefix, id)
	}
	return fmt.Sprintf("/*%s_b%d*/", prefix, id)
}
