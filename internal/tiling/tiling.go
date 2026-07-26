// Package tiling is a prototype source model for gotmplfumpt's stub/restore
// pipeline. It partitions a Go-template source string into a gap-free,
// non-overlapping sequence of typed slices (a "tiling"), then reconstructs
// the source losslessly from a formatted stub.
//
// The tiling is the analog of SQLFluff's raw_sliced list (base.py): a
// contiguous partition of the source that carries each span's type. Unlike
// SQLFluff, this model never renders the template — it only maps source
// bytes to and from the stub handed to gofumpt. It therefore keeps only the
// source side (RawFileSlice) and drops the entire rendered/templated side
// (TemplatedFileSlice, the trace program counter, control-flow jumps).
//
// Indexing is by byte offset, not rune. The only consumer of the stub is
// gofumpt, which is byte-oriented UTF-8, and text/template/parse positions
// are already byte offsets. SQLFluff's rune-offset decision exists to match a
// downstream Python lexer and does not transfer here.
package tiling

import "fmt"

const (
	// Literal is plain source text between actions. It is emitted verbatim
	// into the stub and reflowed by gofumpt.
	Literal SliceType = iota
	// Action is an output-producing {{ pipeline }}.
	Action
	// BlockOpen is an opening control tag: {{if}}, {{range}}, {{with}}, {{block}}.
	BlockOpen
	// BlockMid is a mid control tag: {{else}} or {{else if}}.
	BlockMid
	// BlockClose is a closing control tag: {{end}}.
	BlockClose
	// Comment is a {{/* … */}} template comment.
	Comment
	// Define is a whole {{define "x"}}…{{end}} block, spanning its opening tag,
	// body, and matching end. The body is opaque — restored verbatim, never
	// handed to gofumpt — because define bodies are often standalone-invalid
	// Go fragments.
	Define
)

// SliceType classifies one span of source. The vocabulary mirrors SQLFluff's
// slice_type strings (tracer.py / base.py); Define is a gotmplfumpt addition
// for {{define}}…{{end}} blocks.
type SliceType uint8

// RawSlice is a typed, half-open interval [Start, Stop) over the source
// string's bytes. A slice carries no identity of its own: its position in a
// Tiling.Slices sequence is its identity, which keeps the type minimal and
// makes the sentinel numbering fall out of the ordering.
type RawSlice struct {
	Type  SliceType
	Start int
	Stop  int
}

// Tiling is a gap-free, non-overlapping partition of Src into typed slices,
// in source order. It is the single source of truth for stub/restore: every
// byte of Src belongs to exactly one slice, so reconstruction is a total
// function of the tiling and the formatted stub.
type Tiling struct {
	Src    string
	Slices []RawSlice
	// prefix is the sentinel prefix chosen for Src, held so Stub and Restore
	// agree without the caller threading it through both. ScanTiling sets it;
	// a hand-built Tiling that never calls Stub/Restore may leave it empty.
	prefix string
}

// String returns the SQLFluff-style name of the slice type.
func (t SliceType) String() string {
	switch t {
	case Literal:
		return "literal"
	case Action:
		return "templated"
	case BlockOpen:
		return "block_start"
	case BlockMid:
		return "block_mid"
	case BlockClose:
		return "block_end"
	case Comment:
		return "comment"
	case Define:
		return "define"
	default:
		return fmt.Sprintf("SliceType(%d)", uint8(t))
	}
}

// Raw returns the source bytes of one slice.
//
// Requires: s is a slice of t (its bounds index into t.Src).
func (t Tiling) Raw(s RawSlice) string { return t.Src[s.Start:s.Stop] }

// Check asserts the tiling invariant: slices are contiguous with no gaps,
// each starts where the previous stopped, the first starts at 0, and the
// last stops at len(Src). This is the Go analog of the consistency check in
// SQLFluff's TemplatedFile.__init__ (base.py:195-207), which raises rather
// than let a mis-tiled file corrupt downstream edits. ScanTiling calls it so
// a malformed tiling never escapes construction.
//
// Ensures: on nil error, the slice lengths sum to len(Src) with no overlaps.
func (t Tiling) Check() error {
	pos := 0
	for i, s := range t.Slices {
		if s.Stop < s.Start {
			return fmt.Errorf("tiling: slice %d (%s) inverted: stop %d < start %d",
				i, s.Type, s.Stop, s.Start)
		}
		if s.Start != pos {
			return fmt.Errorf("tiling: gap before slice %d (%s): start %d != running length %d",
				i, s.Type, s.Start, pos)
		}
		pos = s.Stop
	}
	if pos != len(t.Src) {
		return fmt.Errorf("tiling: covers %d of %d source bytes", pos, len(t.Src))
	}
	return nil
}
