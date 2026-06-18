// Package tmplfunc offers a small set of text/template helpers tailored to
// code-generation templates that want their rendered Go to be gofumpt-clean
// without a post-gofumpt step.
//
// The primary helper is PadRight, which lets a template right-pad a string
// to a known width — useful for aligning the identifier column of
// consecutive const items or the key column of struct-literal entries,
// which gofumpt would otherwise re-align after the fact.
//
// Typical use:
//
//	import (
//	    "text/template"
//	    "github.com/StevenACoffman/gotmplfumpt/tmplfunc"
//	)
//
//	t := template.New("").Funcs(tmplfunc.FuncMap()).Parse(src)
//
// And in the template, after computing a max width:
//
//	{{- $max := 0 -}}
//	{{- range .Items -}}
//	  {{- if gt (len .Name) $max }}{{ $max = len .Name }}{{ end -}}
//	{{- end -}}
//	{{- range .Items }}
//	{{ padRight .Name $max }} = "{{ .Value }}"
//	{{- end }}
package tmplfunc

import (
	"strings"
	"text/template"
)

// PadRight returns s right-padded with spaces to width n. If len(s) >= n
// the input is returned unchanged. n values < 0 are treated as 0.
//
// Requires: nothing.
// Ensures:  result begins with s; len(result) == max(len(s), n).
func PadRight(s string, n int) string {
	if n <= len(s) {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// FuncMap returns the helper set as a text/template.FuncMap suitable for
// (*Template).Funcs. The same set is returned on every call; callers who
// add their own entries should merge into a fresh map.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"padRight": PadRight,
	}
}
