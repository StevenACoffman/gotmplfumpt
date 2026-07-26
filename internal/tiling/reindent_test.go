package tiling_test

import "testing"

// TestRestoreReindentsOwnLineAction checks that a multi-line action sitting on
// its own indented line has its continuation lines realigned to the column
// gofumpt placed the sentinel at.
func TestRestoreReindentsOwnLineAction(t *testing.T) {
	t.Parallel()
	// The action is indented one tab; its second line is flush left in source.
	const src = "func F() {\n\t{{ dict\n\"a\" 1 }}\n}\n"
	// A formatter that preserves the stub verbatim (the action's own line is
	// already at one tab), so restore must reindent the continuation to one tab.
	got := roundTrip(t, src, func(stub string) string { return stub })
	const want = "func F() {\n\t{{ dict\n\t\"a\" 1 }}\n}\n"
	if got != want {
		t.Errorf("reindent own-line action = %q, want %q", got, want)
	}
}

// TestRestoreLeavesInlineActionUnchanged checks the guard: when the action is
// inline (non-whitespace precedes it on the line), its continuation lines are
// left as-authored, since gofumpt does not own that column.
func TestRestoreLeavesInlineActionUnchanged(t *testing.T) {
	t.Parallel()
	const src = "var C = {{ dict\n\"a\" 1 }}\n"
	got := roundTrip(t, src, func(stub string) string { return stub })
	if got != src {
		t.Errorf("inline action changed = %q, want identity %q", got, src)
	}
}
