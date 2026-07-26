// reindent.go realigns the continuation lines of a multi-line action after
// gofumpt has placed its sentinel at some column. gofumpt owns the column of
// the sentinel token; the restored action's second and later lines must start
// at that column, or a multi-line pipeline would land at the wrong indent.
// This is the one restore-side heuristic the tiling model does not eliminate —
// gofumpt genuinely owns literal layout — but it is keyed deterministically off
// the sentinel's position rather than recovered by string search.

package tiling

import "strings"

// reindentContinuation rewrites lines 2..N of raw to start at the same column
// as the sentinel at sentinelIdx in formatted. Line 1 is unchanged; it already
// sits at the sentinel's column by construction. When the sentinel's line has
// non-whitespace before it (the action is inline, not on its own line) or when
// raw contains an unbalanced backtick (a raw-string literal spans lines and
// re-indenting would corrupt it), raw is returned unchanged.
func reindentContinuation(formatted string, sentinelIdx int, raw string) string {
	lineStart := strings.LastIndexByte(formatted[:sentinelIdx], '\n') + 1
	prefix := formatted[lineStart:sentinelIdx]
	if !isAllSpaceOrTab(prefix) || hasUnclosedBacktick(raw) {
		return raw
	}
	lines := strings.Split(raw, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = prefix + strings.TrimLeft(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// isAllSpaceOrTab reports whether s contains only space and tab bytes.
func isAllSpaceOrTab(s string) bool {
	for i := range len(s) {
		if s[i] != ' ' && s[i] != '\t' {
			return false
		}
	}
	return true
}

// hasUnclosedBacktick reports whether s contains an odd number of backticks,
// implying a raw-string literal spans lines and re-indenting would corrupt it.
func hasUnclosedBacktick(s string) bool {
	return strings.Count(s, "`")%2 != 0
}
