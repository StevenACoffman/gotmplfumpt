// restore_test.go uses package format (white-box) so it can drive the
// unexported reindentContinuation helper directly.
//
//nolint:testpackage // white-box test of unexported helpers.
package format

import "testing"

// TestReindentContinuation exercises the column-realignment logic that
// keeps multi-line actions vertically aligned to the column gofumpt
// placed the sentinel at.
func TestReindentContinuation(t *testing.T) {
	cases := map[string]struct {
		formatted   string
		sentinelIdx int
		raw         string
		want        string
	}{
		"single-line raw unchanged": {
			formatted:   "\tNEEDLE\n",
			sentinelIdx: 1,
			raw:         "{{ .X }}",
			want:        "{{ .X }}",
		},
		"two-line raw indents to tab": {
			formatted:   "\tNEEDLE\n",
			sentinelIdx: 1,
			raw:         "{{ dict\n\"a\" 1 }}",
			want:        "{{ dict\n\t\"a\" 1 }}",
		},
		"two-line raw indents to two tabs": {
			formatted:   "\t\tNEEDLE\n",
			sentinelIdx: 2,
			raw:         "{{ dict\n\"a\" 1\n\"b\" 2 }}",
			want:        "{{ dict\n\t\t\"a\" 1\n\t\t\"b\" 2 }}",
		},
		"sentinel not on own line: leave raw alone": {
			formatted:   "x = NEEDLE\n",
			sentinelIdx: 4,
			raw:         "{{ dict\n\"a\" 1 }}",
			want:        "{{ dict\n\"a\" 1 }}",
		},
		"unclosed backtick blocks reindent": {
			formatted:   "\tNEEDLE\n",
			sentinelIdx: 1,
			raw:         "{{ printf `multi\nline` }}",
			// odd number of backticks somewhere in raw → bail.
			// Here, two backticks, so the helper proceeds — flip to
			// genuinely-unclosed to force the bail path.
			want: "{{ printf `multi\n\tline` }}",
		},
		"truly unclosed backtick": {
			formatted:   "\tNEEDLE\n",
			sentinelIdx: 1,
			raw:         "{{ printf `dangling\n}}",
			want:        "{{ printf `dangling\n}}",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := reindentContinuation(tc.formatted, tc.sentinelIdx, tc.raw)
			if got != tc.want {
				t.Errorf("reindentContinuation: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTightenAdjacentSentinels exercises the post-gofumpt pass that
// removes whitespace gofumpt inserted between two sentinels whose source
// actions sat back-to-back. Each case lists the formatted Go that
// gofumpt produced (with sentinels intact) and the desired tightened
// form before restore swaps actions back in.
func TestTightenAdjacentSentinels(t *testing.T) {
	const prefix = "__gtmpl"
	cases := map[string]struct {
		formatted string
		entries   map[int]sentinelEntry
		want      string
	}{
		"space between two adjacent branch opens": {
			formatted: "/*GTMPL_OPEN_0*/ /*GTMPL_OPEN_1*/\n",
			entries: map[int]sentinelEntry{
				0: {Kind: kindBranchOpen, PrevAdjacent: false},
				1: {Kind: kindBranchOpen, PrevAdjacent: true},
			},
			want: "/*GTMPL_OPEN_0*//*GTMPL_OPEN_1*/\n",
		},
		"newline between two adjacent branch closes": {
			formatted: "/*GTMPL_CLOSE_0*/\n/*GTMPL_CLOSE_1*/\n",
			entries: map[int]sentinelEntry{
				0: {Kind: kindBranchClose, PrevAdjacent: false},
				1: {Kind: kindBranchClose, PrevAdjacent: true},
			},
			want: "/*GTMPL_CLOSE_0*//*GTMPL_CLOSE_1*/\n",
		},
		"non-adjacent pair untouched": {
			formatted: "/*GTMPL_OPEN_0*/ /*GTMPL_OPEN_1*/\n",
			entries: map[int]sentinelEntry{
				0: {Kind: kindBranchOpen, PrevAdjacent: false},
				1: {Kind: kindBranchOpen, PrevAdjacent: false},
			},
			want: "/*GTMPL_OPEN_0*/ /*GTMPL_OPEN_1*/\n",
		},
		"action identifier adjacent to branch open": {
			formatted: "__gtmpl_0 /*GTMPL_OPEN_1*/\n",
			entries: map[int]sentinelEntry{
				0: {Kind: kindAction, PrevAdjacent: false},
				1: {Kind: kindBranchOpen, PrevAdjacent: true},
			},
			want: "__gtmpl_0/*GTMPL_OPEN_1*/\n",
		},
		"no inserted whitespace stays put": {
			formatted: "/*GTMPL_OPEN_0*//*GTMPL_OPEN_1*/\n",
			entries: map[int]sentinelEntry{
				0: {Kind: kindBranchOpen, PrevAdjacent: false},
				1: {Kind: kindBranchOpen, PrevAdjacent: true},
			},
			want: "/*GTMPL_OPEN_0*//*GTMPL_OPEN_1*/\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tightenAdjacentSentinels(tc.formatted, tc.entries, prefix)
			if got != tc.want {
				t.Errorf("tightenAdjacentSentinels: got %q, want %q", got, tc.want)
			}
		})
	}
}
