// indent_test.go is a white-box test: tilingIndent is unexported and the
// test drives it directly against the tiling, so it lives in package format.
//
//nolint:testpackage // white-box test of the unexported fallback indenter.
package format

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestTilingIndent exercises the fallback indenter across Go braces, template
// branches, strings/comments/runes, define blocks, and template comments,
// including the two fixtures that actually reach the fallback
// (define-block, adjacent-actions-fragment). Each case also asserts
// idempotency.
func TestTilingIndent(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		want string
	}{
		"flat": {
			in:   "package main\n",
			want: "package main\n",
		},
		"go braces indent body": {
			in:   "func F() {\nreturn\n}\n",
			want: "func F() {\n\treturn\n}\n",
		},
		"template branch indent": {
			in:   "{{ if .X }}\na\n{{ end }}\n",
			want: "{{ if .X }}\n\ta\n{{ end }}\n",
		},
		"branch inside func body adds depths": {
			in:   "func F() {\n{{ if .X }}\nreturn 1\n{{ end }}\n}\n",
			want: "func F() {\n\t{{ if .X }}\n\t\treturn 1\n\t{{ end }}\n}\n",
		},
		"nested ranges indent body by template depth": {
			in:   "{{ range .Events }}{{ range . }}\ntype {{ .Name }} struct{}\n{{ end }}{{ end }}\n",
			want: "{{ range .Events }}{{ range . }}\n\t\ttype {{ .Name }} struct{}\n{{ end }}{{ end }}\n",
		},
		"define body is verbatim": {
			in:   "{{ define \"x\" }}\npackage main\n\nfunc F() {}\n{{ end }}\n",
			want: "{{ define \"x\" }}\npackage main\n\nfunc F() {}\n{{ end }}\n",
		},
		"string literal braces ignored": {
			in:   "s := \"a{b}c\"\nreturn\n",
			want: "s := \"a{b}c\"\nreturn\n",
		},
		"line comment braces ignored": {
			in:   "func F() {\n// { } { }\nreturn\n}\n",
			want: "func F() {\n\t// { } { }\n\treturn\n}\n",
		},
		"block comment braces ignored": {
			in:   "func F() {\n/* { } */ x := 1\nreturn\n}\n",
			want: "func F() {\n\t/* { } */ x := 1\n\treturn\n}\n",
		},
		"rune literal brace ignored": {
			in:   "func F() {\nr := '{'\nreturn\n}\n",
			want: "func F() {\n\tr := '{'\n\treturn\n}\n",
		},
		"single-line template comment": {
			in:   "{{/* one-liner */}}\nreturn\n",
			want: "{{/* one-liner */}}\nreturn\n",
		},
		"multi-line template comment preserved": {
			in:   "{{/* line1\n     line2 */}}\nreturn\n",
			want: "{{/* line1\n     line2 */}}\nreturn\n",
		},
		"template comment in func body": {
			in:   "func F() {\n{{/* note */}}\nreturn\n}\n",
			want: "func F() {\n\t{{/* note */}}\n\treturn\n}\n",
		},
		"trim-marked template comment": {
			in:   "{{- /* trimmed\nbody */ -}}\nx\n",
			want: "{{- /* trimmed\nbody */ -}}\nx\n",
		},
		"template comment with braces inside ignored": {
			in:   "func F() {\n{{/* foo { bar } baz */}}\nreturn\n}\n",
			want: "func F() {\n\t{{/* foo { bar } baz */}}\n\treturn\n}\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := indentSource(t, tc.in); got != tc.want {
				t.Errorf("tilingIndent: got %q, want %q", got, tc.want)
			}
			// Idempotency: re-indenting the output is a fixed point.
			once := indentSource(t, tc.in)
			if twice := indentSource(t, once); twice != once {
				t.Errorf("not idempotent: first %q, second %q", once, twice)
			}
		})
	}
}

// indentSource scans src into a tiling and runs the fallback indenter.
func indentSource(t *testing.T, src string) string {
	t.Helper()
	til, err := tiling.ScanTiling(src)
	if err != nil {
		t.Fatalf("ScanTiling(%q): %v", src, err)
	}
	return tilingIndent(til)
}
