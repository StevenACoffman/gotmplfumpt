package tiling_test

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestCheck verifies the invariant passes for a gap-free tiling and fails
// for the ways a tiling can be malformed: a gap, short coverage, and an
// inverted slice.
func TestCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		tiling  tiling.Tiling
		wantErr bool
	}{
		{
			name: "contiguous",
			tiling: tiling.Tiling{
				Src: "abcd",
				Slices: []tiling.RawSlice{
					{Type: tiling.Literal, Start: 0, Stop: 2},
					{Type: tiling.Action, Start: 2, Stop: 4},
				},
			},
		},
		{
			name:   "empty source, no slices",
			tiling: tiling.Tiling{Src: "", Slices: nil},
		},
		{
			name: "gap between slices",
			tiling: tiling.Tiling{
				Src: "abcd",
				Slices: []tiling.RawSlice{
					{Type: tiling.Literal, Start: 0, Stop: 1},
					{Type: tiling.Action, Start: 2, Stop: 4},
				},
			},
			wantErr: true,
		},
		{
			name: "does not reach end",
			tiling: tiling.Tiling{
				Src: "abcd",
				Slices: []tiling.RawSlice{
					{Type: tiling.Literal, Start: 0, Stop: 3},
				},
			},
			wantErr: true,
		},
		{
			name: "inverted slice",
			tiling: tiling.Tiling{
				Src: "abcd",
				Slices: []tiling.RawSlice{
					{Type: tiling.Literal, Start: 0, Stop: 4},
					{Type: tiling.Action, Start: 4, Stop: 2},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.tiling.Check()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Check() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
