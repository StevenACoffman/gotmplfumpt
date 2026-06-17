// verify_test.go drives the unexported computeShape/shapesEqual helpers
// directly. The structural shape is part of the Format invariant chain,
// so white-box tests catch regressions earlier than golden tests.
//
//nolint:testpackage // white-box test of unexported helpers.
package format

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

func TestStructuralShape(t *testing.T) {
	cases := map[string]struct {
		a, b      string
		wantEqual bool
	}{
		"identical": {
			a:         `{{ if .X }}a{{ end }}`,
			b:         `{{ if .X }}a{{ end }}`,
			wantEqual: true,
		},
		"whitespace-only diff": {
			a:         `{{ if .X }}a{{ end }}`,
			b:         `{{if .X}}a{{end}}`,
			wantEqual: true,
		},
		"different action count": {
			a:         `{{ .X }} {{ .Y }}`,
			b:         `{{ .X }}`,
			wantEqual: false,
		},
		"branch vs action": {
			a:         `{{ if .X }}a{{ end }}`,
			b:         `{{ .X }}a{{ .Y }}a{{ .Z }}`,
			wantEqual: false,
		},
		"range vs if": {
			a:         `{{ range .X }}a{{ end }}`,
			b:         `{{ if .X }}a{{ end }}`,
			wantEqual: false,
		},
		"else branch presence": {
			a:         `{{ if .X }}a{{ else }}b{{ end }}`,
			b:         `{{ if .X }}a{{ end }}`,
			wantEqual: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ra, err := parse.Parse(tc.a)
			if err != nil {
				t.Fatalf("parse a: %v", err)
			}
			rb, err := parse.Parse(tc.b)
			if err != nil {
				t.Fatalf("parse b: %v", err)
			}
			got := shapesEqual(computeShape(ra), computeShape(rb))
			if got != tc.wantEqual {
				t.Errorf("shapesEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.wantEqual)
			}
		})
	}
}
