// verify.go checks that gofumpt left the stub's sentinels intact: each appears
// exactly once and in the original order. A control-tag sentinel marks a block
// boundary, so order preservation is exactly "no sentinel moved across a block
// boundary"; the Block field lets a violation name which boundary was crossed.
// This replaces a full reparse-and-compare of the restored output with an
// O(n·k) scan of the formatted stub (n = len, k = number of sentinels).

package tiling

import (
	"fmt"
	"strings"
)

// VerifyFormatted reports an error if the gofumpt-formatted stub reordered,
// dropped, or duplicated any sentinel relative to the tiling. On a nil error,
// Restore will reconstruct the original template structure.
//
// Requires: formatted is gofumpt's output for t.Stub().
func (t Tiling) VerifyFormatted(formatted string) error {
	prevPos, prevBlock := -1, -1
	for i, s := range t.Slices {
		if s.Type == Literal {
			continue
		}
		needle := t.sentinelFor(i)
		if n := strings.Count(formatted, needle); n != 1 {
			return fmt.Errorf("tiling: sentinel for slice %d (block %d) occurs %d times, want 1",
				i, s.Block, n)
		}
		pos := strings.Index(formatted, needle)
		if pos < prevPos {
			return reorderError(i, prevBlock, s.Block)
		}
		prevPos, prevBlock = pos, s.Block
	}
	return nil
}

// reorderError describes an out-of-order sentinel, distinguishing a move across
// a block boundary from a reorder within one block.
func reorderError(slice, prevBlock, block int) error {
	if block != prevBlock {
		return fmt.Errorf(
			"tiling: gofumpt moved slice %d's sentinel across a block boundary (block %d -> block %d)",
			slice,
			prevBlock,
			block,
		)
	}
	return fmt.Errorf("tiling: gofumpt reordered slice %d's sentinel within block %d", slice, block)
}
