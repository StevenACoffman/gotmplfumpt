package tiling_test

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestScanTilingBlock checks the monotonic block-region id: slices in the same
// flat region share a Block, and it increments at each BlockOpen/BlockClose.
func TestScanTilingBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []int // Block per slice, in order
	}{
		{
			name: "literal only is all block 0",
			src:  "package main\n",
			want: []int{0},
		},
		{
			name: "nested blocks increment at each boundary",
			src:  "{{if .A}}{{if .B}}{{end}}{{end}}",
			want: []int{0, 1, 2, 3},
		},
		{
			name: "literals inherit the surrounding region",
			src:  "a{{if .A}}b{{end}}c",
			// literal a (0), if (0→1), literal b (1), end (1→2), literal c (2)
			want: []int{0, 0, 1, 1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			til, err := tiling.ScanTiling(tt.src)
			if err != nil {
				t.Fatalf("ScanTiling(%q): %v", tt.src, err)
			}
			got := make([]int, len(til.Slices))
			for i, s := range til.Slices {
				got[i] = s.Block
			}
			if !equalInts(got, tt.want) {
				t.Errorf("Block ids = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
