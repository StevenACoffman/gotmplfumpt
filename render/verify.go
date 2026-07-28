package render

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/rogpeppe/go-internal/diff"

	"github.com/StevenACoffman/gotmplfumpt/internal/format"
)

// VerifyFormatPreservesRender checks that formatting src with gotmplfumpt does
// not change what it renders: it formats src, renders both the original and the
// formatted template with data, runs gofumpt on each render, and returns nil
// only when the two are byte-identical.
//
// The comparison is after gofumpt on purpose. gotmplfumpt reflows template
// layout, so the raw renders differ in whitespace by design; the guarantee that
// matters is that the generated Go is the same once canonicalized (code
// generators gofmt their output anyway). A difference that survives gofumpt is a
// real gotmplfumpt bug.
//
// Requires: funcs covers every function src calls, and — like Diagnose — the
// template renders a complete Go file (gofumpt rejects a bare fragment). funcs
// are executed twice, once per template, so they should be free of caller-
// visible side effects or the two renders may differ spuriously.
//
// The returned error distinguishes the causes a caller acts on differently:
// "format template" (gotmplfumpt could not format src), an "original:" error
// (src or data is broken), a "reformatted:" error (gotmplfumpt produced a
// template that no longer renders — a formatter bug), and a diff (the reformat
// changed the output — also a formatter bug).
func VerifyFormatPreservesRender(src string, funcs template.FuncMap, data any) error {
	formatted, err := format.Format(src)
	if err != nil {
		return fmt.Errorf("format template: %w", err)
	}
	return renderEquivalent(src, formatted, funcs, data)
}

// renderEquivalent returns nil when templates a and b render to byte-identical
// Go after gofumpt, under the same funcs and data. On divergence it returns an
// error carrying a unified diff; on a render failure it names which side failed.
func renderEquivalent(a, b string, funcs template.FuncMap, data any) error {
	ga, err := gofumptRender(a, funcs, data)
	if err != nil {
		return fmt.Errorf("original: %w", err)
	}
	gb, err := gofumptRender(b, funcs, data)
	if err != nil {
		return fmt.Errorf("reformatted: %w", err)
	}
	if !bytes.Equal(ga, gb) {
		return fmt.Errorf("reformatting changed the rendered output:\n%s",
			diff.Diff("original.go", ga, "reformatted.go", gb))
	}
	return nil
}

// gofumptRender renders src with funcs and data and returns the gofumpt'd output.
// A render that is not valid Go is reported as such.
func gofumptRender(src string, funcs template.FuncMap, data any) ([]byte, error) {
	rendered, err := execTemplate(src, funcs, data)
	if err != nil {
		return nil, err
	}
	out, err := format.GoSource(rendered)
	if err != nil {
		return nil, fmt.Errorf("render is not valid Go: %w", err)
	}
	return out, nil
}
