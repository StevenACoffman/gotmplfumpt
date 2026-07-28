// gofumpt.go runs gofumpt's format.Source on stubbed Go and returns the
// formatted bytes. Failures are returned as wrapped errors; callers decide
// whether to fall back.

package format

import (
	"fmt"
	"strings"

	"mvdan.cc/gofumpt/format"
)

// formatGo runs gofumpt on src.
//
// Ensures: on nil error, the returned bytes are valid Go and gofumpt-canonical.
//
//	on error, the original src is unchanged.
func formatGo(src []byte) ([]byte, error) {
	out, err := format.Source(src, format.Options{
		LangVersion: "go1.25",
	})
	if err != nil {
		return nil, fmt.Errorf("gofumpt: %w", err)
	}
	return out, nil
}

// formatStub runs gofumpt on a tiling stub. A stub that is already a whole file
// (has a package clause) formats directly; a package-less fragment — the common
// case for code-generation templates — is retried wrapped in a synthetic
// package and unwrapped, so declaration-list templates reach the gofumpt path
// instead of the fallback.
//
// Requires: stub is Tiling.Stub() output (sentinels in place, no "{{").
// Ensures:  on ok, the result is gofumpt's formatting of the stub with any
//
//	synthetic package removed (trailing newline kept), ready for
//	VerifyFormatted/Restore; on !ok, neither attempt parsed as Go.
func formatStub(stub string) (string, bool) {
	if out, err := formatGo([]byte(stub)); err == nil {
		return string(out), true
	}
	out, err := formatGo([]byte(wrapPackage + stub))
	if err != nil {
		return "", false
	}
	return stripWrapPackage(string(out)), true
}

// stripWrapPackage removes the synthetic package clause that wrapPackage
// prepends, plus the blank line gofumpt inserts after it, returning the
// formatted fragment body. Any trailing file-final newline is left in place.
func stripWrapPackage(out string) string {
	s := strings.TrimPrefix(out, wrapPackage)
	return strings.TrimPrefix(s, "\n")
}
