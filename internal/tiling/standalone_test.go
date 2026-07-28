// standalone_test.go is a white-box test of the standalone-action marking and
// the comment-form sentinel selection it drives.
//
//nolint:testpackage // white-box test of unexported Standalone/sentinelFor.
package tiling

import "testing"

// TestStandaloneMarking checks that ScanTiling flags exactly the Action slices
// that are alone on their source line.
func TestStandaloneMarking(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src  string
		want []bool // expected Standalone per Action slice, in source order
	}{
		"lone action is standalone": {
			"{{ reserveImport \"x\" }}\nvar y int\n",
			[]bool{true},
		},
		"indented lone action is standalone":  {"\t{{ .Impl }}\n", []bool{true}},
		"action at EOF with no newline":       {"{{ .Impl }}", []bool{true}},
		"action with leading code is not":     {"x := {{ .Y }}\n", []bool{false}},
		"action with trailing text is not":    {"{{ .A }} z\n", []bool{false}},
		"adjacent actions are not standalone": {"{{ .A }}{{ .B }}\n", []bool{false, false}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			til, err := ScanTiling(tc.src)
			if err != nil {
				t.Fatalf("ScanTiling(%q): %v", tc.src, err)
			}
			got := actionStandaloneFlags(til)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d actions, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("action %d Standalone = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// actionStandaloneFlags collects the Standalone flag of each Action slice in order.
func actionStandaloneFlags(til Tiling) []bool {
	var flags []bool
	for _, s := range til.Slices {
		if s.Type == Action {
			flags = append(flags, s.Standalone)
		}
	}
	return flags
}

// TestSentinelForCommentStandalone checks that WithStandaloneComments switches a
// standalone action to the comment form, leaves a non-standalone action as an
// identifier, and does not mutate the receiver.
func TestSentinelForCommentStandalone(t *testing.T) {
	t.Parallel()
	til, err := ScanTiling("{{ reserveImport \"x\" }}\nx := {{ .Y }}\n{{ if .Z }}\nq\n{{ end }}\n")
	if err != nil {
		t.Fatalf("ScanTiling: %v", err)
	}
	// slice 0: standalone Action; 2: in-expression Action; 4: BlockOpen.
	standalone, inExpr, block := 0, 2, 4
	if til.Slices[inExpr].Type != Action || til.Slices[block].Type != BlockOpen {
		t.Fatalf("fixture drifted: slices = %+v", til.Slices)
	}
	ident := func(i int) string { return actionSentinel(til.prefix, i) }
	comment := func(i int) string { return commentSentinel(til.prefix, i) }

	assertForm(
		t,
		"default standalone action is an identifier",
		til.sentinelFor(standalone),
		ident(standalone),
	)

	commented := til.WithStandaloneComments()
	assertForm(
		t,
		"commented standalone action is a comment",
		commented.sentinelFor(standalone),
		comment(standalone),
	)
	assertForm(
		t,
		"non-standalone action stays an identifier",
		commented.sentinelFor(inExpr),
		ident(inExpr),
	)
	assertForm(t, "control tag is always a comment", commented.sentinelFor(block), comment(block))
	assertForm(t, "receiver is not mutated", til.sentinelFor(standalone), ident(standalone))
}

// assertForm fails when got != want, naming the case.
func assertForm(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", name, got, want)
	}
}
