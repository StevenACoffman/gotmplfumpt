// gofumpt.go runs gofumpt's format.Source on stubbed Go and returns the
// formatted bytes. Failures are returned as wrapped errors; callers decide
// whether to fall back.

package format

import (
	"fmt"

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
