// fallback_test.go drives the unexported reindentByDepth helper.
//
//nolint:testpackage // white-box test of unexported helper.
package format

import "testing"

// TestReindentByDepth exercises the brace-counting fallback indenter
// across Go braces, template branches, strings/comments/runes, and
// define blocks. Each case also asserts idempotence (running the
// transform twice produces the same output as once).
func TestReindentByDepth(t *testing.T) {
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
			got := reindentByDepth(tc.in)
			if got != tc.want {
				t.Errorf("reindentByDepth: got %q, want %q", got, tc.want)
			}
			// Idempotency.
			got2 := reindentByDepth(got)
			if got2 != got {
				t.Errorf("not idempotent: first %q, second %q", got, got2)
			}
		})
	}
}
