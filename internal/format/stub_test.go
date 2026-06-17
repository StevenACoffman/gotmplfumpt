// stub_test.go intentionally uses the same package as the production code
// (white-box test). It exercises the unexported stubGo and
// uniqueSentinelPrefix helpers — moving them to format_test would force
// either exporting them or introducing a parallel test-only API.
//
//nolint:testpackage // white-box test of unexported helpers.
package format

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

// TestStubGoProducesParseableGo asserts that for representative
// Go-emitting templates, the stub output parses with go/parser. This
// catches sentinel classification mistakes early.
func TestStubGoProducesParseableGo(t *testing.T) {
	cases := map[string]string{
		"simple func name": `package main

func {{ .Name }}() {}
`,
		"package name": `package {{ .PkgName }}
`,
		"if in body": `package main

func F() {
	{{ if .HasBody }}
	println("x")
	{{ end }}
}
`,
		"range in body": `package main

func F() {
	{{ range .Items }}
	_ = "x"
	{{ end }}
}
`,
		"with else": `package main

func F() int {
	{{ if .X }}
	return 1
	{{ else }}
	return 2
	{{ end }}
}
`,
		"action inside string": `package main

func F() string {
	return "hello {{ .Name }}"
}
`,
		"comment": `package main

{{/* doc */}}
func F() {}
`,
		"variable use": `package main

var V = {{ .Value }}
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			runStubCase(t, src)
		})
	}
}

// runStubCase is the per-case body of TestStubGoProducesParseableGo,
// lifted out so the test's cognitive complexity stays bounded.
func runStubCase(t *testing.T, src string) {
	t.Helper()
	root, err := parse.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res := stubGo(root, src)
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "stub.go", res.Go, parser.ParseComments); err != nil {
		t.Errorf("stub did not parse as Go: %v\nstub:\n%s", err, res.Go)
	}
	if want, got := countActionNodes(root), len(res.Entries); got != want {
		t.Errorf("Entries len = %d, want %d", got, want)
	}
}

// countActionNodes returns the number of {{…}} action boundaries in the
// parse tree (action + branch open + each else + each end + comment).
func countActionNodes(n parse.Node) int {
	switch n := n.(type) {
	case *parse.ListNode:
		if n == nil {
			return 0
		}
		total := 0
		for _, c := range n.Nodes {
			total += countActionNodes(c)
		}
		return total
	case *parse.ActionNode, *parse.CommentNode:
		return 1
	case *parse.BranchNode:
		total := 1 // open
		total += countActionNodes(n.List)
		for _, e := range n.Elses {
			total++
			total += countActionNodes(e.List)
		}
		if n.End != nil {
			total++
		}
		return total
	}
	return 0
}

func TestUniqueSentinelPrefix(t *testing.T) {
	cases := map[string]struct {
		src        string
		wantPrefix string
	}{
		"no collision":   {"package main", "__gtmpl"},
		"collision base": {"// __gtmpl is here", "__gtmpl_x"},
		"empty input":    {"", "__gtmpl"},
		"contains chunk": {"x __gtmpl_5 y", "__gtmpl_x"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := uniqueSentinelPrefix(tc.src)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Errorf("uniqueSentinelPrefix(%q) = %q, want prefix %q",
					tc.src, got, tc.wantPrefix)
			}
			if strings.Contains(tc.src, got) {
				t.Errorf("returned prefix %q is present in src", got)
			}
		})
	}
}
