package parse_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

// TestLexInsideActionPunctuation regresses a bug where the refactored
// lexInsideAction dispatcher treated a nil return from a sub-handler as
// "didn't match" rather than "handled and emitted." After lexPunctOrEOF
// emitted a token for `:=`, `=`, `|`, `(`, or `)`, the dispatcher fell
// through to subsequent handlers, which emitted a bogus second token
// with empty val, surfacing as `unexpected "" in operand`.
func TestLexInsideActionPunctuation(t *testing.T) {
	cases := []string{
		`{{ $x := .Name }}`,
		`{{ $x := title .Name }}`,
		`{{ $x = .Name }}`,
		`{{ .A | .B }}`,
		`{{ (.A) }}`,
		`{{ range $i, $v := .Items }}{{ . }}{{ end }}`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := parse.Parse(src); err != nil {
				t.Errorf("parse %q: %v", src, err)
			}
		})
	}
}

// TestLexInsideActionRealFile catches the original report: a template
// with `{{ $eventName := title .Name }}` on its own line, sitting inside
// a function-body context.
func TestLexInsideActionRealFile(t *testing.T) {
	src := strings.Join([]string{
		"// Code generated, do not edit.",
		"package x",
		"",
		"import (",
		`	"example/pkg"`,
		")",
		`{{ range .Events }}{{ range . }}`,
		`{{ $eventName := title .Name }}`,
		`{{ range .Columns }}{{ if .EnumOptions }}`,
		"{{ end }}{{ end }}",
		"{{ end }}{{ end }}",
	}, "\n")
	if _, err := parse.Parse(src); err != nil {
		t.Errorf("parse failed: %v\nsrc:\n%s", err, src)
	}
}
