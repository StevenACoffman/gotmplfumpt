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
// non-whitespace before it (the action is inline, not on its own line), raw is
// returned unchanged. A continuation line that begins inside a Go raw-string
// literal is left verbatim — its leading whitespace is string content, not
// indentation, so rewriting it would change the string's value.
func reindentContinuation(formatted string, sentinelIdx int, raw string) string {
	lineStart := strings.LastIndexByte(formatted[:sentinelIdx], '\n') + 1
	prefix := formatted[lineStart:sentinelIdx]
	if !isAllSpaceOrTab(prefix) {
		return raw
	}
	lines := strings.Split(raw, "\n")
	insideString := stringLineMask(raw)
	for i := 1; i < len(lines); i++ {
		if insideString[i] {
			continue
		}
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

// stringLineMask reports, for each line of raw (split on "\n"), whether that
// line begins inside a Go string, raw-string, or char literal — where the
// leading whitespace is literal content that must not be re-indented. It
// records exactly one entry per newline (so the result aligns with
// strings.Split(raw, "\n")), which a skip-to-close scan cannot guarantee when a
// literal itself contains a newline.
func stringLineMask(raw string) []bool {
	mask := []bool{false} // line 0 always begins in code context
	var inInterp, inRaw, inRune, escaped bool
	for i := range len(raw) {
		c := raw[i]
		switch {
		case c == '\n':
			mask = append(mask, inAnyString(inInterp, inRaw, inRune))
			escaped = false
		case escaped:
			escaped = false
		case inRaw:
			inRaw = c != '`'
		case inInterp:
			escaped, inInterp = updateStringState(c, '"', inInterp)
		case inRune:
			escaped, inRune = updateStringState(c, '\'', inRune)
		default:
			inInterp, inRaw, inRune = c == '"', c == '`', c == '\''
		}
	}
	return mask
}

// inAnyString reports whether any string-literal state is open.
func inAnyString(interp, raw, char bool) bool { return interp || raw || char }

// updateStringState advances one byte inside an escaped (interpreted-string or
// char) literal delimited by quote: a backslash escapes the next byte, and the
// unescaped quote closes the literal. Returns the new (escaped, open) state.
func updateStringState(c, quote byte, open bool) (escaped, stillOpen bool) {
	switch c {
	case '\\':
		return true, open
	case quote:
		return false, false
	default:
		return false, open
	}
}
