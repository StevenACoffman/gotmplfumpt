package tmplfunc_test

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/StevenACoffman/gotmplfumpt/tmplfunc"
)

func TestPadRight(t *testing.T) {
	cases := map[string]struct {
		s    string
		n    int
		want string
	}{
		"shorter than width":     {"foo", 5, "foo  "},
		"exact width":            {"foo", 3, "foo"},
		"longer than width":      {"hello", 3, "hello"},
		"empty string with zero": {"", 0, ""},
		"empty string padded":    {"", 4, "    "},
		"negative width":         {"abc", -1, "abc"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tmplfunc.PadRight(tc.s, tc.n)
			if got != tc.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

// TestFuncMap exercises the helper end-to-end through a parsed template,
// confirming both that the FuncMap registration works and that the
// rendered output is what callers expect.
func TestFuncMap(t *testing.T) {
	const src = `{{- $max := 0 -}}` +
		`{{- range . }}{{ if gt (len .Name) $max }}{{ $max = len .Name }}{{ end }}{{ end -}}` +
		"\n" +
		`{{- range . }}` +
		"\n" +
		`{{ padRight .Name $max }} = "{{ .Value }}"` +
		`{{- end }}` +
		"\n"
	type item struct {
		Name, Value string
	}
	data := []item{
		{"Alpha", "1"},
		{"BetaLong", "2"},
		{"C", "3"},
	}
	tmpl, err := template.New("").Funcs(tmplfunc.FuncMap()).Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	want := "\n" +
		`Alpha    = "1"` + "\n" +
		`BetaLong = "2"` + "\n" +
		`C        = "3"` + "\n"
	if got != want {
		t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	// Spot-check key presence.
	if !strings.Contains(got, "Alpha    =") {
		t.Errorf("padding not applied; got: %q", got)
	}
}
