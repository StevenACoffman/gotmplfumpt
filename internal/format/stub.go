// Package-internal: stub.go renders a parsed Go template AST as a string
// of syntactically-valid Go where each template action is replaced by a
// sentinel. The result is fed to gofumpt; sentinels are then mapped back
// to the original raw action source by restore.go.
//
// Sentinel scheme:
//
//	{{ X }}                       → __gtmpl_N         (identifier)
//	{{ if/range/with/define X }}  → /*GTMPL_OPEN_N*/
//	{{ else }} / {{ else if X }}  → /*GTMPL_MID_N*/
//	{{ end }}                     → /*GTMPL_CLOSE_N*/
//	{{/* comment */}}             → /*GTMPL_COMMENT_N*/
//
// Why comment-based sentinels for branches: a block statement `{ … }` is
// only valid in Go's statement context, not at top level. Comments are
// valid in every position, so the template's branch nesting need not
// correspond to a Go brace level. gofumpt indents based on actual Go
// braces in the TextNode contents — the user's existing indentation
// inside a {{ if }} block passes through, and gofumpt may further
// normalize it according to brace nesting it sees.

package format

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

// sentinelKind classifies how an entry should be matched and restored.
const (
	kindAction          sentinelKind = iota // {{ X }}
	kindBranchOpen                          // {{ if/range/with X }}
	kindBranchMid                           // {{ else }} or {{ else if X }}
	kindBranchClose                         // {{ end }}
	kindTemplateComment                     // {{/* … */}}
	kindDefineBlock                         // {{ define "x" }} … {{ end }} (whole block, opaque)
)

// sentinelKind classifies how an entry should be matched and restored.
type sentinelKind int

// sentinelEntry holds the original raw source for one template action and
// the kind of sentinel that replaced it in the stub.
type sentinelEntry struct {
	Raw          string // exact bytes of the original action including {{…}} and trim markers
	Kind         sentinelKind
	PrevAdjacent bool // true iff this action's "{{" immediately followed the previous action's "}}" in source
}

// stubResult is the output of stubGo.
type stubResult struct {
	Entries map[int]sentinelEntry // sentinel ID → original action
	Go      string                // stub Go source
	Prefix  string                // identifier prefix for action sentinels, e.g. "__gtmpl"
}

// stubBuilder accumulates output and the sentinel map during walk.
type stubBuilder struct {
	entries    map[int]sentinelEntry
	src        string
	prefix     string
	out        strings.Builder
	nextID     int
	prevEndPos int // byte position just past the previous action's "}}"; -1 means none yet
}

// stubGo walks the parse AST and emits stub Go source.
//
// Requires: root produced by parse.Parse(src).
// Ensures:  every action in root appears exactly once in Entries; Go
//
//	contains every key from Entries (encoded according to its
//	Kind) exactly once.
func stubGo(root parse.Node, src string) stubResult {
	prefix := uniqueSentinelPrefix(src)
	b := &stubBuilder{
		src:        src,
		prefix:     prefix,
		entries:    map[int]sentinelEntry{},
		prevEndPos: -1,
	}
	b.walk(root)
	return stubResult{Go: b.out.String(), Entries: b.entries, Prefix: prefix}
}

func (b *stubBuilder) nextSentinelID() int {
	id := b.nextID
	b.nextID++
	return id
}

// recordSpan records an action whose source spans src[start:end). It sets
// PrevAdjacent when start equals the previous action's end position
// (meaning the two actions had no whitespace between them in source).
func (b *stubBuilder) recordSpan(kind sentinelKind, start, end int) int {
	id := b.nextSentinelID()
	b.entries[id] = sentinelEntry{
		Kind:         kind,
		Raw:          b.src[start:end],
		PrevAdjacent: start == b.prevEndPos,
	}
	b.prevEndPos = end
	return id
}

// emit writes the stub form of a sentinel with the given kind and ID.
func (b *stubBuilder) emit(id int, kind sentinelKind) {
	switch kind {
	case kindAction:
		_, _ = fmt.Fprintf(&b.out, "%s_%d", b.prefix, id)
	case kindBranchOpen:
		_, _ = fmt.Fprintf(&b.out, "/*GTMPL_OPEN_%d*/", id)
	case kindBranchMid:
		_, _ = fmt.Fprintf(&b.out, "/*GTMPL_MID_%d*/", id)
	case kindBranchClose:
		_, _ = fmt.Fprintf(&b.out, "/*GTMPL_CLOSE_%d*/", id)
	case kindTemplateComment:
		_, _ = fmt.Fprintf(&b.out, "/*GTMPL_COMMENT_%d*/", id)
	case kindDefineBlock:
		_, _ = fmt.Fprintf(&b.out, "/*GTMPL_DEFINE_%d*/", id)
	}
}

func (b *stubBuilder) walk(n parse.Node) {
	if n == nil {
		return
	}
	switch n := n.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			b.walk(child)
		}
	case *parse.TextNode:
		_, _ = b.out.WriteString(n.Text)
	case *parse.ActionNode:
		b.emitActionSpan(int(n.Position()), kindAction)
	case *parse.CommentNode:
		b.emitActionSpan(int(n.Position()), kindTemplateComment)
	case *parse.BranchNode:
		b.walkBranch(n)
	}
}

// emitActionSpan records and emits one action whose interior position is
// pos. Falls back to a degraded record (no span info) if the action's
// delimiters can't be located.
func (b *stubBuilder) emitActionSpan(pos int, kind sentinelKind) {
	start, end, ok := actionSpan(pos, b.src)
	if !ok {
		return
	}
	id := b.recordSpan(kind, start, end)
	b.emit(id, kind)
}

func (b *stubBuilder) walkBranch(n *parse.BranchNode) {
	// {{ define "x" }} bodies declare a separate template namespace and
	// may be Go-file fragments (no package clause, partial declarations).
	// Pass the whole block through as one opaque sentinel so gofumpt
	// never sees the body. The body is restored byte-for-byte.
	if n.Keyword == "define" && n.End != nil {
		if start, end, ok := defineSpan(n, b.src); ok {
			id := b.recordSpan(kindDefineBlock, start, end)
			b.emit(id, kindDefineBlock)
			return
		}
	}
	b.emitActionSpan(int(n.Position()), kindBranchOpen)
	b.walk(n.List)
	for _, e := range n.Elses {
		b.emitActionSpan(int(e.Position()), kindBranchMid)
		b.walk(e.List)
	}
	if n.End != nil {
		b.emitActionSpan(int(n.End.Position()), kindBranchClose)
	}
}

// defineSpan returns the byte range [start, end) covering a {{define}}
// branch from its opening "{{" through its matching "{{end}}"'s "}}".
// Returns ok=false if either delimiter can't be located.
func defineSpan(n *parse.BranchNode, src string) (start, end int, ok bool) {
	openPos := int(n.Position())
	if openPos <= 0 || openPos > len(src) {
		return 0, 0, false
	}
	s := strings.LastIndex(src[:openPos], "{{")
	if s < 0 {
		return 0, 0, false
	}
	endPos := int(n.End.Position())
	if endPos < 0 || endPos > len(src) {
		return 0, 0, false
	}
	closeIdx := strings.Index(src[endPos:], "}}")
	if closeIdx < 0 {
		return 0, 0, false
	}
	return s, endPos + closeIdx + 2, true
}

// uniqueSentinelPrefix returns an identifier prefix that does not appear
// in src. The default "__gtmpl" is used unless src itself mentions it,
// in which case a hash-derived suffix is appended.
func uniqueSentinelPrefix(src string) string {
	const base = "__gtmpl"
	if !strings.Contains(src, base) {
		return base
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(src))
	cand := fmt.Sprintf("%s_x%08x", base, h.Sum32())
	if !strings.Contains(src, cand) {
		return cand
	}
	// Pathological: input contains both base AND the hash form. Iterate
	// the suffix until we find a fresh one.
	for i := range 16 {
		cand = fmt.Sprintf("%s_x%08x_%d", base, h.Sum32(), i)
		if !strings.Contains(src, cand) {
			return cand
		}
	}
	return cand
}

// actionSpan returns the byte range [start, end) of the {{…}} action
// whose interior position is pos in src. Returns ok=false when the
// delimiters can't be located.
func actionSpan(pos int, src string) (start, end int, ok bool) {
	if pos <= 0 || pos > len(src) {
		return 0, 0, false
	}
	s := strings.LastIndex(src[:pos], "{{")
	if s < 0 {
		return 0, 0, false
	}
	j := strings.Index(src[s:], "}}")
	if j < 0 {
		return 0, 0, false
	}
	return s, s + j + 2, true
}
