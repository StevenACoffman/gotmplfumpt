package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// runFlagCase is one row in TestRunFlags' table.
type runFlagCase struct {
	verdict verdictFn
	stdin   string
	args    []string
}

// verdictFn validates the outcome of one run() invocation.
type verdictFn func(t *testing.T, err error, stdout, stderr *bytes.Buffer)

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testscripts",
		Setup: func(_ *testscript.Env) error {
			return nil
		},
	})
}

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"gotmplfumpt": main,
	})
}

// TestRunFlags exercises run() directly with injected I/O. Cases share
// the runOneFlagCase driver; per-case verdicts live in named helpers so
// the table itself stays simple.
func TestRunFlags(t *testing.T) {
	cases := map[string]runFlagCase{
		"help returns flag.ErrHelp": {
			args:    []string{"gotmplfumpt", "-h"},
			verdict: verifyHelp,
		},
		"version writes to stdout": {
			args:    []string{"gotmplfumpt", "-version"},
			verdict: verifyVersion,
		},
		"write flag forbidden with stdin": {
			args:    []string{"gotmplfumpt", "-w"},
			verdict: expectErrContaining("cannot use -w"),
		},
		"list flag forbidden with stdin": {
			args:    []string{"gotmplfumpt", "-l"},
			verdict: expectErrContaining("cannot use -l"),
		},
		"stdin passes through Format": {
			args:    []string{"gotmplfumpt"},
			stdin:   "hello",
			verdict: verifyStdinPassthrough,
		},
		"unknown flag returns error": {
			args:    []string{"gotmplfumpt", "-no-such-flag"},
			verdict: verifyUnknownFlag,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			runOneFlagCase(t, tc)
		})
	}
}

func runOneFlagCase(t *testing.T, tc runFlagCase) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		tc.args,
		emptyGetenv,
		strings.NewReader(tc.stdin),
		&stdout, &stderr,
	)
	tc.verdict(t, err, &stdout, &stderr)
}

func verifyHelp(t *testing.T, err error, _, stderr *bytes.Buffer) {
	t.Helper()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr.String(), "usage: gotmplfumpt") {
		t.Errorf("stderr did not include usage line: %q", stderr.String())
	}
}

func verifyVersion(t *testing.T, err error, stdout, _ *bytes.Buffer) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "gotmplfumpt ") {
		t.Errorf("expected version line on stdout, got %q", stdout.String())
	}
}

func verifyStdinPassthrough(t *testing.T, err error, stdout, _ *bytes.Buffer) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "hello" {
		t.Errorf("expected stdout %q, got %q", "hello", stdout.String())
	}
}

func verifyUnknownFlag(t *testing.T, err error, _, _ *bytes.Buffer) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Errorf("unknown flag returned flag.ErrHelp; expected a distinct error")
	}
}

// expectErrContaining returns a verdict that fails unless err contains
// substr.
func expectErrContaining(substr string) verdictFn {
	return func(t *testing.T, err error, _, _ *bytes.Buffer) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), substr) {
			t.Errorf("expected error containing %q, got %v", substr, err)
		}
	}
}

func emptyGetenv(string) string { return "" }

// TestIsTemplateFile checks the compound-suffix matcher against the six
// recognized extensions plus a handful of negative cases.
func TestIsTemplateFile(t *testing.T) {
	cases := map[string]struct {
		path string
		want bool
	}{
		"tpl.go":              {"emit.tpl.go", true},
		"go.tpl":              {"emit.go.tpl", true},
		"gotmpl.go":           {"emit.gotmpl.go", true},
		"tmpl.go":             {"emit.tmpl.go", true},
		"go.tmpl":             {"emit.go.tmpl", true},
		"gotmpl":              {"emit.gotmpl", true},
		"uppercase":           {"emit.TPL.GO", true},
		"mixed case":          {"emit.Tpl.Go", true},
		"with dir":            {"a/b/c.tpl.go", true},
		"html dropped":        {"page.html", false},
		"htm dropped":         {"page.htm", false},
		"plain go":            {"main.go", false},
		"plain tpl":           {"x.tpl", false},
		"plain tmpl":          {"x.tmpl", false},
		"empty":               {"", false},
		"suffix-only":         {".tpl.go", true},
		"unrelated extension": {"notes.txt", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isTemplateFile(tc.path)
			if got != tc.want {
				t.Errorf("isTemplateFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
