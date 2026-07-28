// define.go formats the bodies of {{define}}…{{end}} blocks, which the main
// pipeline treats as opaque (their bodies are Go-code fragments handed through
// verbatim). Each pure-Go body is run through gofumpt — directly when it is a
// whole file, otherwise wrapped in a synthetic package and unwrapped — so a
// define body is formatted like the Go it emits instead of passing through
// ugly. Bodies that contain a template action ({{…}}) are left untouched:
// they are not standalone Go, and their delimiters must not be reflowed.

package format

import (
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// wrapPackage is prepended to a package-less Go fragment to make it a
// formattable file; the exact prefix is stripped back off afterward.
const wrapPackage = "package gotmplfumpt\n"

// formatDefineBodies returns text with each {{define}} block's body formatted.
// Non-define spans are re-emitted verbatim, so with no defines (or no
// change) the result is byte-identical to text.
//
// Requires: text parses as a Go template. Ensures: template structure is
// unchanged (only Go-code bodies are reflowed); idempotent.
func formatDefineBodies(text string) string {
	til, err := tiling.ScanTiling(text)
	if err != nil {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, s := range til.Slices {
		if s.Type == tiling.Define {
			b.WriteString(reformatDefineBlock(til.Raw(s)))
		} else {
			b.WriteString(til.Raw(s))
		}
	}
	return b.String()
}

// reformatDefineBlock formats the body of one {{define}}…{{end}} block. The
// body lies between the opening tag's closing "}}" and the matching
// "{{end}}"'s opening "{{". In valid template source those are the only
// delimiters at the top level of the block, so first-"}}"/last-"{{" bracket
// the body exactly. The body's surrounding whitespace is preserved; only its
// core is reflowed.
func reformatDefineBlock(raw string) string {
	open := strings.Index(raw, "}}")
	closeIdx := strings.LastIndex(raw, "{{")
	if open < 0 || closeIdx < open+2 {
		return raw
	}
	formatted, ok := formatDefineBody(raw[open+2 : closeIdx])
	if !ok {
		return raw
	}
	return raw[:open+2] + formatted + raw[closeIdx:]
}

// formatDefineBody formats a define block's body. A pure-Go body is reflowed by
// gofumpt with its surrounding whitespace preserved; any other body — a
// template fragment — is re-indented by its own template/brace structure via
// the tiling indenter. ok is false only when neither applies (the fragment
// does not even tile), leaving the body verbatim.
func formatDefineBody(body string) (string, bool) {
	lead, core, trail := splitSurroundingSpace(body)
	if out, ok := formatGoFragment(core); ok {
		return lead + out + trail, true
	}
	til, err := tiling.ScanTiling(body)
	if err != nil {
		return body, false
	}
	return tilingIndent(til), true
}

// formatGoFragment formats a Go fragment with gofumpt, returning ok=false when
// it cannot. A fragment containing a template action is refused: it is not
// standalone Go, and reflowing it could corrupt "{{"/"}}" delimiters. Whole
// files (with a package clause) format directly; package-less declaration
// lists are wrapped in a synthetic package and unwrapped.
func formatGoFragment(core string) (string, bool) {
	if core == "" || strings.Contains(core, "{{") {
		return core, false
	}
	if out, err := formatGo([]byte(core)); err == nil {
		return strings.TrimSuffix(string(out), "\n"), true
	}
	out, err := formatGo([]byte(wrapPackage + core))
	if err != nil {
		return core, false
	}
	// A define body is spliced between "}}" and "{{", so it drops gofumpt's
	// file-final newline; the stub path keeps it.
	s := strings.TrimSuffix(stripWrapPackage(string(out)), "\n")
	return s, true
}

// splitSurroundingSpace splits s into its leading whitespace run, its core,
// and its trailing whitespace run.
func splitSurroundingSpace(s string) (lead, core, trail string) {
	i := 0
	for i < len(s) && isSpaceByte(s[i]) {
		i++
	}
	j := len(s)
	for j > i && isSpaceByte(s[j-1]) {
		j--
	}
	return s[:i], s[i:j], s[j:]
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
