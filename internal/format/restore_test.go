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
