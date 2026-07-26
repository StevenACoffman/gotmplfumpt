// Package format implements the Go-template-to-formatted-Go pipeline.
//
// Format(text) parses the text as a Go template, replaces each {{…}}
// action with a sentinel, runs gofumpt on the resulting Go, then
// substitutes the original action bytes back. When gofumpt rejects the
// stubbed Go, Format falls back to a tiling-driven brace-depth indenter so
// the output is at least idempotent.
package format

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/parse"
	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
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

	// Format opaque {{define}} bodies before the main pipeline, which treats
	// them as verbatim. Re-parse only to confirm a reflowed body still yields
	// a valid template (reflowing keeps the delimiters, so this cannot fail in
	// practice); the parse tree itself is no longer needed past this point.
	if formatted := formatDefineBodies(text); formatted != text {
		text = formatted
		if _, err := parse.Parse(text); err != nil {
			return "", fmt.Errorf("parse template: %w", err)
		}
	}

	// Build the tiling once and share it with both paths. A template that
	// parse.Parse accepted always tiles (both require balanced, terminated
	// delimiters), so this error is a should-not-happen invariant violation;
	// surface it rather than hide it.
	til, err := tiling.ScanTiling(text)
	if err != nil {
		return "", fmt.Errorf("tile template: %w", err)
	}
	if out, ok := formatViaGofumpt(til); ok {
		return out, nil
	}
	return tilingIndent(til), nil
}

// formatViaGofumpt is the primary path: stub → gofumpt → verify → restore.
// Returns (formatted, true) on success; (_, false) when any step fails so the
// caller can fall back.
func formatViaGofumpt(til tiling.Tiling) (string, bool) {
	formatted, err := formatGo([]byte(til.Stub()))
	if err != nil {
		return "", false
	}
	// Assert gofumpt left every sentinel intact and in order (so no sentinel
	// crossed a block boundary); a mismatch means restoration would corrupt
	// the template, so fall back. This is a cheaper check than reparsing.
	if err := til.VerifyFormatted(string(formatted)); err != nil {
		return "", false
	}
	out, err := til.Restore(string(formatted))
	if err != nil {
		return "", false
	}
	return out, true
}
