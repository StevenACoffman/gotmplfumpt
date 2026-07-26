package tiling_test

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestScannerAgreesWithParser cross-checks the scanner against the real
// text/template parser as an independent oracle: the number of non-literal
// slices the scanner produces must equal the number of delimited constructs
// ({{…}} pairs) the parser finds. This catches a scanner that splits an
// action wrongly or misses one, using a code path with no shared logic.
func TestScannerAgreesWithParser(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"package main\n",
		"a {{ .X }} b",
		`{{ printf "}}" }}x`,
		"{{if .A}}x{{else}}y{{end}}",
		"{{if .A}}x{{else if .B}}y{{else}}z{{end}}",
		"{{range .Items}}{{.}}{{end}}",
		"{{with .X}}a{{end}}",
		"{{/* c */}}q",
		`{{define "x"}}q{{end}}`,
		`{{block "b" .}}x{{end}}`,
		`{{template "t" .}}`,
	}
	for _, src := range inputs {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			root, err := parse.Parse(src)
			if err != nil {
				t.Fatalf("parse.Parse: %v", err)
			}
			til, err := tiling.ScanTiling(src)
			if err != nil {
				t.Fatalf("ScanTiling: %v", err)
			}
			want := countConstructs(root)
			if got := nonLiteralCount(til); got != want {
				t.Errorf("non-literal slices = %d, parser constructs = %d", got, want)
			}
		})
	}
}

// nonLiteralCount returns the number of slices that become sentinels.
func nonLiteralCount(t tiling.Tiling) int {
	n := 0
	for _, s := range t.Slices {
		if s.Type != tiling.Literal {
			n++
		}
	}
	return n
}

// countConstructs counts the non-literal slices the scanner should produce for
// an AST: one per action and comment; one per if/range/with/block branch plus
// one for each else clause and its closing end; and exactly one per define
// block, which the scanner treats as a single opaque slice (so its body is not
// recursed). This parser keeps {{else if}} as a single flat else clause, so the
// non-define count equals the source's delimiter-pair count.
func countConstructs(n parse.Node) int {
	switch n := n.(type) {
	case *parse.ListNode:
		if n == nil {
			return 0
		}
		sum := 0
		for _, c := range n.Nodes {
			sum += countConstructs(c)
		}
		return sum
	case *parse.ActionNode:
		return 1
	case *parse.CommentNode:
		return 1
	case *parse.BranchNode:
		if n.Keyword == "define" {
			return 1 // opaque whole-block slice
		}
		count := 1 + countConstructs(n.List)
		for _, e := range n.Elses {
			count += 1 + countConstructs(e.List)
		}
		if n.End != nil {
			count++
		}
		return count
	default:
		return 0
	}
}
