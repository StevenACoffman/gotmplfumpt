// Package render diagnoses what gofumpt would change in a template's rendered
// output and points each change back at the template source where it can prove
// the mapping. It is a best-effort reporter, never a rewriter: the caller
// supplies the data, Diagnose renders and formats, and the caller decides what
// to do with the findings.
//
// The hard part of relating rendered Go back to template source is the absence
// of a source map from text/template. Diagnose sidesteps it by claiming only
// mappings it can prove — a rendered line whose whitespace-collapsed text equals
// a template line's — and honestly flagging everything else (alignment on
// templated values, inserted blank lines) as unmapped.
package render

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/rogpeppe/go-internal/diff"

	"github.com/StevenACoffman/gotmplfumpt/internal/format"
)

const (
	// TopLevel means the line is not inside any control body.
	TopLevel Enclosing = iota
	// InLoop means the line is inside a {{range}} or {{with}} body, so the
	// change applies to every iteration.
	InLoop
	// InConditional means the line is inside a {{if}}/{{else}} body, so it
	// renders only for some data.
	InConditional
)

// Enclosing classifies the template control-flow context of a mapped line.
type Enclosing int

// Finding is one change gofumpt would make to the rendered output, with a
// best-effort pointer back into the template source.
type Finding struct {
	// RenderedLine is the 1-based line in the rendered output where the change
	// starts.
	RenderedLine int
	// Before is the rendered text gofumpt replaces; After is its replacement.
	// Either may span multiple lines (joined with "\n").
	Before string
	After  string
	// TemplateLine is the 1-based line in src the change maps to, or 0 when the
	// change could not be proven to correspond to a literal template line.
	TemplateLine int
	// Enclosing is the control-flow context of a mapped line (TopLevel when
	// unmapped).
	Enclosing Enclosing
	// Note explains the mapping or why it is absent.
	Note string
}

// Report is the result of diagnosing one template against one set of data.
type Report struct {
	// Clean is true when the rendered output is already gofumpt-clean; then Diff
	// is empty and Findings is nil.
	Clean bool
	// Diff is the unified diff of the rendered output against its gofumpt'd form.
	Diff string
	// Findings holds one entry per changed hunk, in source order.
	Findings []Finding
}

// String returns the name of the enclosing context.
func (e Enclosing) String() string {
	switch e {
	case TopLevel:
		return "TopLevel"
	case InLoop:
		return "InLoop"
	case InConditional:
		return "InConditional"
	default:
		return fmt.Sprintf("Enclosing(%d)", int(e))
	}
}

// Diagnose renders src with data and reports what gofumpt would change in the
// output, mapping each change back to a template line where it can prove the
// mapping and flagging the rest. It never edits the template.
//
// Requires: funcs covers every function src calls.
// Ensures:  a Finding's TemplateLine is non-zero only when src's line at that
//
//	number is literal (contains no action); on Report.Clean the render is
//	already gofumpt-clean and Findings is empty.
//
// The template is expected to render a complete Go file (a package clause and
// balanced declarations); gofumpt, like go/format, rejects a bare fragment. A
// fragment render therefore returns the "not valid Go" error below rather than
// a report — diagnosing a fragment in isolation is not meaningful, since it only
// renders correctly inside its surrounding file.
//
// It returns an error when src does not parse, does not execute with data, or
// renders to invalid Go (gofumpt rejects it — itself a useful diagnosis).
func Diagnose(src string, funcs template.FuncMap, data any) (Report, error) {
	rendered, err := execTemplate(src, funcs, data)
	if err != nil {
		return Report{}, err
	}
	formatted, err := format.GoSource(rendered)
	if err != nil {
		return Report{}, fmt.Errorf("rendered output is not valid Go: %w", err)
	}
	if bytes.Equal(rendered, formatted) {
		return Report{Clean: true}, nil
	}
	diffText := string(diff.Diff("rendered.go", rendered, "gofumpt.go", formatted))
	return Report{Diff: diffText, Findings: findings(diffText, src)}, nil
}

// execTemplate parses src with funcs and executes it against data, returning the
// rendered bytes. Parse and execute failures are wrapped so callers (and their
// tests) can tell the two apart.
func execTemplate(src string, funcs template.FuncMap, data any) ([]byte, error) {
	tmpl, err := template.New("").Funcs(funcs).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}
