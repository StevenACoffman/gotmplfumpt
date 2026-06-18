// restore.go substitutes original template-action source bytes back into
// gofumpt-formatted Go. Each sentinel string is unique (incorporates its
// integer ID), so substitution is a deterministic per-entry replace.

package format

import (
	"fmt"
	"strings"
)

// restore replaces every sentinel in formatted with the corresponding
// Raw bytes from entries.
//
// Requires: every sentinel ID in entries was emitted into formatted
//
//	exactly once during stubGo.
//
// Ensures:  on nil error, the returned string contains no sentinel
//
//	markers and every entry's Raw appears at the position the
//	sentinel held. Multi-line Raw values are re-indented so
//	continuation lines align with the column gofumpt placed the
//	sentinel at.
func restore(formatted string, entries map[int]sentinelEntry, prefix string) (string, error) {
	formatted = tightenAdjacentSentinels(formatted, entries, prefix)
	out := formatted
	for id, entry := range entries {
		needle := sentinelString(id, entry.Kind, prefix)
		idx := strings.Index(out, needle)
		if idx < 0 {
			return "", fmt.Errorf("sentinel %q not found in formatted output", needle)
		}
		replacement := entry.Raw
		if strings.Contains(replacement, "\n") {
			replacement = reindentContinuation(out, idx, replacement)
		}
		out = out[:idx] + replacement + out[idx+len(needle):]
	}
	return out, nil
}

// tightenAdjacentSentinels removes any whitespace gofumpt inserted
// between two sentinels whose source actions were adjacent (the
// previous action's "}}" was immediately followed by the next
// action's "{{" with no whitespace between).
//
// gofumpt may add a space, newline, or blank line between consecutive
// /* */ block comments. Without this pass, two adjacent template
// actions like "{{ range A }}{{ range B }}" would become
// "{{- range A }} {{- range B }}" after a round-trip — an unintended
// whitespace insertion that the original source explicitly avoided.
func tightenAdjacentSentinels(
	formatted string,
	entries map[int]sentinelEntry,
	prefix string,
) string {
	out := formatted
	for id, entry := range entries {
		if !entry.PrevAdjacent {
			continue
		}
		prev, ok := entries[id-1]
		if !ok {
			continue
		}
		prevS := sentinelString(id-1, prev.Kind, prefix)
		currS := sentinelString(id, entry.Kind, prefix)
		if prevS == "" || currS == "" {
			continue
		}
		idx := strings.Index(out, prevS)
		if idx < 0 {
			continue
		}
		afterPrev := idx + len(prevS)
		j := afterPrev
		for j < len(out) && isHSpaceOrNewline(out[j]) {
			j++
		}
		if !strings.HasPrefix(out[j:], currS) {
			continue
		}
		out = out[:afterPrev] + out[j:]
	}
	return out
}

func isHSpaceOrNewline(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// reindentContinuation rewrites the 2..N lines of raw to start at the
// same column as the sentinel sitting at sentinelIdx in formatted. The
// first line is unchanged (it lands at the sentinel's column by
// construction).
//
// When the sentinel's line has non-whitespace before it (the action
// sits inline, not on its own line), or when raw contains an unclosed
// backtick raw-string literal, indentation can't be safely rewritten
// and raw is returned unchanged.
func reindentContinuation(formatted string, sentinelIdx int, raw string) string {
	lineStart := strings.LastIndexByte(formatted[:sentinelIdx], '\n') + 1
	prefix := formatted[lineStart:sentinelIdx]
	if !isAllSpaceOrTab(prefix) {
		return raw
	}
	if hasUnclosedBacktick(raw) {
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

// hasUnclosedBacktick reports whether s contains an odd number of
// backticks — implying a raw-string literal spans lines and re-indenting
// would corrupt its content.
func hasUnclosedBacktick(s string) bool {
	return strings.Count(s, "`")%2 != 0
}

// sentinelString returns the string form of a sentinel as emitted by
// stubBuilder.emit, so restore can look for the exact bytes.
func sentinelString(id int, kind sentinelKind, prefix string) string {
	switch kind {
	case kindAction:
		return fmt.Sprintf("%s_%d", prefix, id)
	case kindBranchOpen:
		return fmt.Sprintf("/*GTMPL_OPEN_%d*/", id)
	case kindBranchMid:
		return fmt.Sprintf("/*GTMPL_MID_%d*/", id)
	case kindBranchClose:
		return fmt.Sprintf("/*GTMPL_CLOSE_%d*/", id)
	case kindTemplateComment:
		return fmt.Sprintf("/*GTMPL_COMMENT_%d*/", id)
	case kindDefineBlock:
		return fmt.Sprintf("/*GTMPL_DEFINE_%d*/", id)
	}
	return ""
}
