package parse_test

import (
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

func FuzzParseString(f *testing.F) {
	samples := []string{
		`{{}}`,
		`{{.}}`,
		`{{.Field}}`,
		`package main` + "\n\n" + `func {{ .Name }}() {}` + "\n",
		`{{ range .Items }}var _ = {{ . }}{{ end }}`,
		`{{.Field | printf "%q"}}`,
		`{{if .}}return 1{{else}}return 0{{end}}`,
		`{{/* generated */}}` + "\n" + `package {{ .Pkg }}` + "\n",
		`{{- $x := .Y -}}` + "\n" + `var V = {{ $x }}` + "\n",
	}
	for _, s := range samples {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		root, err := parse.Parse(s)
		if err != nil {
			return
		}
		out := root.String()
		root2, err := parse.Parse(out)
		if err != nil {
			t.Fatalf("failed to parse output: %v", err)
		}
		out2 := root2.String()
		if out != out2 {
			t.Fatalf("round trip failure: %q != %q", out, out2)
		}
	})
}
