package tiling_test

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// span is the (type, raw text) of one slice, the readable projection of a
// RawSlice used to assert scanner output.
type span struct {
	typ tiling.SliceType
	raw string
}

// TestScanTiling asserts the scanner's typed partition for representative
// inputs, including the cases the current Index("}}") approach mishandles:
// a "}}" inside a string literal, and adjacent actions with no literal
// between them.
func TestScanTiling(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []span
	}{
		{
			name: "literal only",
			src:  "package main\n",
			want: []span{{tiling.Literal, "package main\n"}},
		},
		{
			name: "action between literals",
			src:  "a {{ .X }} b",
			want: []span{
				{tiling.Literal, "a "},
				{tiling.Action, "{{ .X }}"},
				{tiling.Literal, " b"},
			},
		},
		{
			name: "close delim inside string is not the end",
			src:  `{{ printf "}}" }}x`,
			want: []span{
				{tiling.Action, `{{ printf "}}" }}`},
				{tiling.Literal, "x"},
			},
		},
		{
			name: "adjacent actions have no literal between",
			src:  "{{if .A}}{{if .B}}z{{end}}{{end}}",
			want: []span{
				{tiling.BlockOpen, "{{if .A}}"},
				{tiling.BlockOpen, "{{if .B}}"},
				{tiling.Literal, "z"},
				{tiling.BlockClose, "{{end}}"},
				{tiling.BlockClose, "{{end}}"},
			},
		},
		{
			name: "else and trim markers",
			src:  "{{if .A}}x{{- else -}}y{{end}}",
			want: []span{
				{tiling.BlockOpen, "{{if .A}}"},
				{tiling.Literal, "x"},
				{tiling.BlockMid, "{{- else -}}"},
				{tiling.Literal, "y"},
				{tiling.BlockClose, "{{end}}"},
			},
		},
		{
			name: "comment with delims in body",
			src:  "{{/* a }} b */}}c",
			want: []span{
				{tiling.Comment, "{{/* a }} b */}}"},
				{tiling.Literal, "c"},
			},
		},
		{
			name: "define block is one opaque slice",
			src:  `{{define "x"}}q{{end}}`,
			want: []span{
				{tiling.Define, `{{define "x"}}q{{end}}`},
			},
		},
		{
			name: "define with nested block coalesces to its own end",
			src:  `a{{define "x"}}{{if .Y}}z{{end}}{{end}}b`,
			want: []span{
				{tiling.Literal, "a"},
				{tiling.Define, `{{define "x"}}{{if .Y}}z{{end}}{{end}}`},
				{tiling.Literal, "b"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tiling.ScanTiling(tt.src)
			if err != nil {
				t.Fatalf("ScanTiling(%q): %v", tt.src, err)
			}
			assertSpans(t, got, tt.want)
		})
	}
}

// assertSpans checks that the tiling's slices match want in type and raw text.
func assertSpans(t *testing.T, got tiling.Tiling, want []span) {
	t.Helper()
	if len(got.Slices) != len(want) {
		t.Fatalf("slice count = %d, want %d\ngot: %s", len(got.Slices), len(want), formatSpans(got))
	}
	for i, s := range got.Slices {
		if s.Type != want[i].typ || got.Raw(s) != want[i].raw {
			t.Errorf("slice %d = (%s, %q), want (%s, %q)",
				i, s.Type, got.Raw(s), want[i].typ, want[i].raw)
		}
	}
}

// formatSpans renders a tiling's slices for failure messages.
func formatSpans(t tiling.Tiling) string {
	var b []byte
	for _, s := range t.Slices {
		b = append(b, '\n')
		b = append(b, s.Type.String()...)
		b = append(b, ' ')
		b = append(b, []byte(t.Raw(s))...)
	}
	return string(b)
}
