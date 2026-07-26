// adjacent_test.go covers the post-parse adjacency analysis pass and
// the printer's behavior when control actions are flagged as adjacent
// to a prior action. White-box (package parse) so it can drive the
// unexported markAdjacency and inspect unexported PrevAdjacent fields.
//
//nolint:testpackage // white-box test of unexported helpers.
package parse

import (
	"strings"
	"testing"
)

// TestPrinterPreservesAdjacency renders a parsed template back to source
// and asserts that two control actions that sat back-to-back in the
// input remain back-to-back in the output. The fallback path in
// internal/format relies on this; without it, gotmplfumpt would insert
// stray newlines between adjacent {{...}} actions when the gofumpt
// pipeline can't run (e.g., fragment templates lacking a package
// clause).
func TestPrinterPreservesAdjacency(t *testing.T) {
	cases := map[string]struct {
		input       string
		mustContain []string
		mustNot     []string
	}{
		"adjacent opens and ends": {
			input: "{{ range .A }}{{ range . }}\nbody\n{{ end }}{{ end }}\n",
			mustContain: []string{
				"{{ range .A }}{{ range . }}",
				"{{ end }}{{ end }}",
			},
		},
		"adjacent if/else/end": {
			input: "{{ if .A }}{{ else }}{{ end }}\n",
			mustContain: []string{
				"{{ if .A }}{{ else }}{{ end }}",
			},
		},
		"action then range stays adjacent": {
			input: "{{ .X }}{{ range . }}\nb\n{{ end }}\n",
			mustContain: []string{
				"{{ .X }}{{ range . }}",
			},
		},
		"separator between control actions stays separated": {
			input: "{{ range .A }} {{ range . }}\nbody\n{{ end }} {{ end }}\n",
			mustNot: []string{
				"{{ range .A }}{{ range . }}",
				"{{ end }}{{ end }}",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			checkPrinterCase(t, tc.input, tc.mustContain, tc.mustNot)
		})
	}
}

// checkPrinterCase parses input, renders it, and asserts the contain /
// must-not-contain substring sets. Split out of the test loop so the
// table walker stays inside the cognitive-complexity budget.
func checkPrinterCase(t *testing.T, input string, mustContain, mustNot []string) {
	t.Helper()
	root, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := root.String()
	for _, w := range mustContain {
		if !strings.Contains(out, w) {
			t.Errorf("output missing substring %q\n--- output ---\n%s", w, out)
		}
	}
	for _, w := range mustNot {
		if strings.Contains(out, w) {
			t.Errorf("output unexpectedly contains %q\n--- output ---\n%s", w, out)
		}
	}
}

// TestMarkAdjacency drives the analysis pass directly and asserts the
// PrevAdjacent flag on each control node. Inputs are kept small so
// branch shapes are obvious from reading the source string.
func TestMarkAdjacency(t *testing.T) {
	cases := map[string]struct {
		input string
		// adjacency lists the expected PrevAdjacent value for each
		// control action in printer-visit order:
		//   outer-open, [inner-open, inner-else..., inner-end,]
		//   outer-else..., outer-end.
		adjacency []bool
	}{
		"first action never adjacent": {
			input:     "{{ range . }}body{{ end }}",
			adjacency: []bool{false, false},
		},
		"nested back-to-back opens and ends": {
			input:     "{{ range .A }}{{ range . }}body{{ end }}{{ end }}",
			adjacency: []bool{false, true, false, true},
		},
		"if/else/end fully adjacent": {
			input:     "{{ if .A }}{{ else }}{{ end }}",
			adjacency: []bool{false, true, true},
		},
		"text between opens breaks adjacency": {
			input:     "{{ range .A }} {{ range . }}body{{ end }}{{ end }}",
			adjacency: []bool{false, false, false, true},
		},
		"action precedes range adjacently": {
			input:     "{{ .X }}{{ range . }}body{{ end }}",
			adjacency: []bool{true, false},
		},
		"comment precedes range adjacently": {
			input:     "{{/* hi */}}{{ range . }}body{{ end }}",
			adjacency: []bool{true, false},
		},
		"trim markers do not alter byte positions": {
			input:     "{{- range .A -}}{{- range . -}}body{{- end -}}{{- end -}}",
			adjacency: []bool{false, true, false, true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := collectAdjacency(root)
			if len(got) != len(tc.adjacency) {
				t.Fatalf("got %d flags, want %d: %v vs %v",
					len(got), len(tc.adjacency), got, tc.adjacency)
			}
			for i, want := range tc.adjacency {
				if got[i] != want {
					t.Errorf("flag[%d] = %v, want %v (all: %v)",
						i, got[i], want, got)
				}
			}
		})
	}
}

// collectAdjacency walks the parsed tree in printer-visit order and
// returns the PrevAdjacent flag of each BranchNode, ElseNode, and
// EndNode encountered.
func collectAdjacency(n Node) []bool {
	var out []bool
	var visit func(n Node)
	visit = func(n Node) {
		switch n := n.(type) {
		case *ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				visit(c)
			}
		case *BranchNode:
			out = append(out, n.PrevAdjacent)
			visit(n.List)
			for _, e := range n.Elses {
				out = append(out, e.PrevAdjacent)
				visit(e.List)
			}
			if n.End != nil {
				out = append(out, n.End.PrevAdjacent)
			}
		}
	}
	visit(n)
	return out
}
