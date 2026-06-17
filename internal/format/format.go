// Package format implements the Go-template-to-formatted-Go pipeline.
//
// Format(text) parses the text as a Go template, replaces each {{…}}
// action with a sentinel, runs gofumpt on the resulting Go, then
// substitutes the original action bytes back. When gofumpt rejects the
// stubbed Go, Format falls back to the parser's own AST printer so the
// output is at least idempotent.
package format

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
)

// Format formats a Go template source string. The interface is deliberately
// minimal: no options, no modes.
//
// Returns an error only for genuine parse failures of the template itself.
// gofumpt rejection of the stubbed Go is handled internally by falling
// back to the AST printer; the caller never sees that.
func Format(text string) (string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	root, err := parse.Parse(text)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	if root.HasIgnoreAll() {
		return text, nil
	}

	if out, ok := formatViaGofumpt(root, text); ok {
		return out, nil
	}
	return fallbackFormat(root), nil
}

// formatViaGofumpt is the primary path: stub → gofumpt → restore →
// structural verify. Returns (formatted, true) on success; (_, false)
// when any step fails so the caller can fall back.
func formatViaGofumpt(root parse.Node, text string) (string, bool) {
	stub := stubGo(root, text)
	formatted, err := formatGo([]byte(stub.Go))
	if err != nil {
		return "", false
	}
	out, err := restore(string(formatted), stub.Entries, stub.Prefix)
	if err != nil {
		return "", false
	}
	// Structural sanity check: re-parse the output and compare the
	// action sequence to the input. A mismatch means restoration lost
	// or duplicated an action; fall back rather than emit corrupted
	// output.
	reparsed, err := parse.Parse(out)
	if err != nil {
		return "", false
	}
	if !shapesEqual(computeShape(root), computeShape(reparsed)) {
		return "", false
	}
	return out, true
}
