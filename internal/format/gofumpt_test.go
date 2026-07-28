// gofumpt_test.go is a white-box test of the unexported formatStub helper.
//
//nolint:testpackage // white-box test of the unexported stub formatter.
package format

import (
	"strings"
	"testing"
)

// TestFormatStub covers the three ways a stub can format: a whole file
// (formatted directly), a package-less declaration list (wrapped in a synthetic
// package and unwrapped so it reaches gofumpt instead of the fallback), and a
// fragment that is invalid Go either way.
func TestFormatStub(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in       string
		wantOK   bool
		wantHas  string // substring the formatted result must contain
		wantMiss string // substring it must not contain
	}{
		"whole file formats directly": {
			in:      "package main\nfunc  F( ){}\n",
			wantOK:  true,
			wantHas: "func F() {}",
		},
		"package-less declaration list is wrapped and unwrapped": {
			in:       "func  F( ){\nx:=1\n_ = x\n}\n",
			wantOK:   true,
			wantHas:  "x := 1",
			wantMiss: "package", // synthetic wrapper is stripped back off
		},
		"bare identifier is invalid even wrapped": {
			in:     "notADecl\n",
			wantOK: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := formatStub(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("formatStub ok = %v, want %v (got %q)", ok, tc.wantOK, got)
			}
			if ok {
				assertSubstrings(t, tc.in, got, tc.wantHas, tc.wantMiss)
			}
		})
	}
}

// assertSubstrings checks that got contains has (when non-empty) and omits miss
// (when non-empty), reporting against the original input for a readable failure.
func assertSubstrings(t *testing.T, in, got, has, miss string) {
	t.Helper()
	if has != "" && !strings.Contains(got, has) {
		t.Errorf("formatStub(%q) = %q, want it to contain %q", in, got, has)
	}
	if miss != "" && strings.Contains(got, miss) {
		t.Errorf("formatStub(%q) = %q, want it to omit %q", in, got, miss)
	}
}
