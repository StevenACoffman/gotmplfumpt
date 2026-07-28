// verify_whitebox_test.go drives the unexported renderEquivalent directly. Its
// divergence branch is unreachable through VerifyFormatPreservesRender — a real
// format.Format is render-preserving by design and cannot produce a
// counterexample — so a hand-crafted divergent pair is the only way to exercise
// it.
//
//nolint:testpackage // white-box test of the unexported renderEquivalent.
package render

import (
	"strings"
	"testing"
)

func TestRenderEquivalent(t *testing.T) {
	t.Parallel()

	// Same code, different whitespace: gofumpt normalizes both, so equivalent.
	if err := renderEquivalent(
		"package main\nvar X=1\n",
		"package main\n\nvar X = 1\n",
		nil, nil,
	); err != nil {
		t.Errorf("whitespace-only difference should be equivalent, got %v", err)
	}

	// Different code: gofumpt cannot reconcile it, so a divergence is reported.
	err := renderEquivalent(
		"package main\n\nvar X = 1\n",
		"package main\n\nvar X = 2\n",
		nil, nil,
	)
	if err == nil {
		t.Fatal("want a divergence error, got nil")
	}
	if !strings.Contains(err.Error(), "changed the rendered output") {
		t.Errorf("error %q, want it to report the changed output", err)
	}
}
