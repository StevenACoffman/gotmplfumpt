// verify_test.go is a white-box test: it builds exact sentinel strings from the
// unexported prefix to construct reordered/duplicated/deleted stubs.
//
//nolint:testpackage // white-box test of unexported sentinel helpers.
package tiling

import (
	"strings"
	"testing"
)

// TestVerifyFormatted checks the block-order guarantee: a faithful stub passes,
// and reordering, duplicating, or deleting a sentinel is reported.
func TestVerifyFormatted(t *testing.T) {
	t.Parallel()
	// {{if .A}}z{{end}} tiles to BlockOpen(0), Literal(1), BlockClose(2).
	til, err := ScanTiling("{{if .A}}z{{end}}")
	if err != nil {
		t.Fatalf("ScanTiling: %v", err)
	}
	stub := til.Stub()
	open := til.sentinelFor(0)
	closeS := til.sentinelFor(2)

	if err := til.VerifyFormatted(stub); err != nil {
		t.Errorf("faithful stub reported: %v", err)
	}
	if err := til.VerifyFormatted(swap(stub, open, closeS)); err == nil {
		t.Error("reorder not detected")
	}
	if err := til.VerifyFormatted(stub + open); err == nil {
		t.Error("duplicate not detected")
	}
	if err := til.VerifyFormatted(strings.Replace(stub, open, "", 1)); err == nil {
		t.Error("deletion not detected")
	}
}

// swap exchanges the first occurrence of a and b within s.
func swap(s, a, b string) string {
	const sentinel = "\x00"
	s = strings.Replace(s, a, sentinel, 1)
	s = strings.Replace(s, b, a, 1)
	return strings.Replace(s, sentinel, b, 1)
}
