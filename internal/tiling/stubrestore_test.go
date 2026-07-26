package tiling_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestStubRestoreIdentity checks the inverse property directly: with the stub
// used as its own "formatted" output (an identity formatter), Restore
// reproduces the source exactly.
func TestStubRestoreIdentity(t *testing.T) {
	t.Parallel()
	srcs := []string{
		"package main\n",
		"a {{ .X }} b",
		`{{ printf "}}" }}x`,
		"{{if .A}}{{if .B}}z{{end}}{{end}}",
		"{{if .A}}x{{- else -}}y{{end}}",
		"{{/* a }} b */}}c",
		`{{define "x"}}q{{end}}`,
		"", // empty
	}
	for _, src := range srcs {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			got := roundTrip(t, src, func(stub string) string { return stub })
			if got != src {
				t.Errorf("Restore(Stub(%q)) = %q, want identity", src, got)
			}
		})
	}
}

// TestRestoreDropsInsertedWhitespace checks the tightenAdjacentSentinels
// subsumption: when a formatter inserts whitespace between the sentinels of
// two source-adjacent actions, Restore discards it, because no Literal slice
// sits between them in the tiling.
func TestRestoreDropsInsertedWhitespace(t *testing.T) {
	t.Parallel()
	const src = "{{if .A}}{{if .B}}z{{end}}{{end}}"
	// A formatter that inserts a space between two back-to-back block-comment
	// sentinels, the way gofumpt spaces consecutive /* */ comments. Only
	// sentinel-to-sentinel boundaries are perturbed; boundaries with real
	// literal text are left alone.
	got := roundTrip(t, src, func(stub string) string {
		return strings.ReplaceAll(stub, "*//*", "*/ /*")
	})
	if got != src {
		t.Errorf("Restore dropped-whitespace = %q, want %q", got, src)
	}
}

// TestRestoreKeepsReflowedLiteral checks the complement: whitespace a
// formatter changes inside a real Literal slice is kept, because that gap is
// preceded by a Literal in the tiling.
func TestRestoreKeepsReflowedLiteral(t *testing.T) {
	t.Parallel()
	const src = "{{ .A }}   x{{ .B }}"
	got := roundTrip(t, src, func(stub string) string {
		// Collapse the literal run "   x" as a formatter might.
		return strings.Replace(stub, "   x", " x", 1)
	})
	const want = "{{ .A }} x{{ .B }}"
	if got != want {
		t.Errorf("Restore reflowed-literal = %q, want %q", got, want)
	}
}

// roundTrip scans src, stubs it, applies format to the stub, and restores.
func roundTrip(t *testing.T, src string, format func(stub string) string) string {
	t.Helper()
	til, err := tiling.ScanTiling(src)
	if err != nil {
		t.Fatalf("ScanTiling(%q): %v", src, err)
	}
	out, err := til.Restore(format(til.Stub()))
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	return out
}
