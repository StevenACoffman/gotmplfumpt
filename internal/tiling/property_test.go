package tiling_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// TestGoldenFixturesPreserveStructure runs every existing formatter fixture
// through scan → stub → restore (identity formatter) and asserts the restored
// output re-scans to the same slice-type sequence as the source. This is the
// right property once Restore applies reindentation: it may rewrite whitespace
// (e.g. an opaque define body realigns to its sentinel column, matching the
// live formatter) but must never lose, duplicate, or reclassify a construct.
func TestGoldenFixturesPreserveStructure(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "format", "testdata", "golden", "in")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("golden fixtures unavailable: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			assertStructurePreserved(t, string(b))
		})
	}
}

// FuzzStructurePreserved asserts the same property over arbitrary input: any
// string that scans to a valid tiling must survive stub/restore with its
// slice-type sequence intact. A scan error is not a candidate and is skipped.
func FuzzStructurePreserved(f *testing.F) {
	seeds := []string{
		"package main\n",
		"a {{ .X }} b",
		`{{ printf "}}" }}x`,
		"{{if .A}}{{if .B}}z{{end}}{{end}}",
		"{{if .A}}x{{- else -}}y{{end}}",
		"{{/* a }} b */}}c",
		`{{define "x"}}q{{end}}`,
		"\t{{ dict\n\"a\" 1 }}\n",
		"{{",
		"}}",
		"{{}}{{}}",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		til, err := tiling.ScanTiling(src)
		if err != nil {
			return
		}
		out, err := til.Restore(til.Stub())
		if err != nil {
			t.Fatalf("Restore after successful scan of %q: %v", src, err)
		}
		restored, err := tiling.ScanTiling(out)
		if err != nil {
			t.Fatalf("re-scan of restored output failed for %q: %v", src, err)
		}
		if !sameTypeSequence(til, restored) {
			t.Fatalf("structure changed\n src: %q\n out: %q", src, out)
		}
	})
}

// assertStructurePreserved is the non-fuzz form used by the golden test.
func assertStructurePreserved(t *testing.T, src string) {
	t.Helper()
	til, err := tiling.ScanTiling(src)
	if err != nil {
		t.Fatalf("ScanTiling: %v", err)
	}
	out, err := til.Restore(til.Stub())
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	restored, err := tiling.ScanTiling(out)
	if err != nil {
		t.Fatalf("re-scan of restored output: %v", err)
	}
	if !sameTypeSequence(til, restored) {
		t.Errorf("structure changed after round-trip\n src: %q\n out: %q", src, out)
	}
}

// sameTypeSequence reports whether two tilings have identical slice-type
// sequences (ignoring the byte spans, which reindentation may shift).
func sameTypeSequence(a, b tiling.Tiling) bool {
	if len(a.Slices) != len(b.Slices) {
		return false
	}
	for i := range a.Slices {
		if a.Slices[i].Type != b.Slices[i].Type {
			return false
		}
	}
	return true
}
