package format_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/format"
)

// TestFormatRoundTrip exercises the full Format pipeline on small
// Go-emitting templates and asserts:
//  1. Format returns no error.
//  2. The output is idempotent: formatting it again is a no-op.
//  3. The output preserves every {{ ... }} action body that appeared in
//     the input (textual equality of trimmed action contents).
func TestFormatRoundTrip(t *testing.T) {
	cases := map[string]string{
		"plain func": `package main

func F() {}
`,
		"action in identifier slot": `package main

func {{ .Name }}() {}
`,
		"if in body": `package main

func F() {
{{ if .HasBody }}
println("x")
{{ end }}
}
`,
		"package action": `package {{ .PkgName }}
`,
		"action in string": `package main

var S = "hello {{ .Name }}"
`,
		"with template comment": `package main

{{/* generated */}}
func F() {}
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			runRoundTripCase(t, src)
		})
	}
}

// runRoundTripCase is the per-case body of TestFormatRoundTrip, lifted out
// to keep that test's cognitive complexity bounded.
func runRoundTripCase(t *testing.T, src string) {
	t.Helper()
	out, err := format.Format(src)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	out2, err := format.Format(out)
	if err != nil {
		t.Fatalf("Format (second pass): %v", err)
	}
	if out != out2 {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
	}
	for _, body := range actionBodies(src) {
		if !strings.Contains(out, body) {
			t.Errorf("output missing original action %q\nout:\n%s", body, out)
		}
	}
}

// actionBodies returns the verbatim {{…}} actions found in src.
func actionBodies(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		start := strings.Index(src[i:], "{{")
		if start < 0 {
			break
		}
		start += i
		end := strings.Index(src[start:], "}}")
		if end < 0 {
			break
		}
		out = append(out, src[start:start+end+2])
		i = start + end + 2
	}
	return out
}
