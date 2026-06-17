// Package main is the gotmplfumpt CLI entry point: a formatter for Go
// templates that emit Go code. It walks the given paths (or stdin), parses
// each file as a Go template, runs the stub-and-gofumpt pipeline, and
// writes the result back depending on the flag combination.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/rogpeppe/go-internal/diff"

	"github.com/StevenACoffman/gotmplfumpt/internal/format"
)

// templateSuffixes lists the filename suffixes recognized as Go-emitting
// templates. They are compound suffixes (e.g. ".tpl.go"), so we match via
// strings.HasSuffix rather than filepath.Ext.
//
//nolint:gochecknoglobals // constant slice; Go has no const slice literal.
var templateSuffixes = []string{
	".tpl.go",
	".go.tpl",
	".gotmpl.go",
	".tmpl.go",
	".go.tmpl",
	".gotmpl",
}

// Version info populated via -ldflags by release builds. Section 18 of the
// project's CLI guidelines permits package-level mutable Version variables
// as the one exception to the "no global state" rule, since the Go linker
// can only override `var`, not `const` or local variables.
//
//nolint:gochecknoglobals // ldflags target; documented exception.
var (
	commit = "none"
	tag    = "(devel)"
	date   = "unknown"
)

// processOpts groups the per-file knobs so we don't thread four bools
// through every function.
type processOpts struct {
	stdout   io.Writer
	write    bool
	list     bool
	showDiff bool
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	// stop() is called explicitly (not deferred) before os.Exit because
	// os.Exit bypasses deferred calls.
	code := realMain(ctx)
	stop()
	os.Exit(code)
}

// realMain returns the exit code. Kept separate from main so signal
// teardown can run before os.Exit.
func realMain(ctx context.Context) int {
	err := run(ctx, os.Args, os.Getenv, os.Stdin, os.Stdout, os.Stderr)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		// flag.ContinueOnError already printed usage to stderr.
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "gotmplfumpt: %s\n", err)
		return 1
	}
}

// run is the program. It parses flags, dispatches work, and returns any
// error to main. Tests drive it directly with injected I/O.
//
// Returns flag.ErrHelp unwrapped when the user requests help; main detects
// that with errors.Is and exits zero.
func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	_ = ctx    // reserved for future cancellation hooks
	_ = getenv // reserved for future env-driven knobs

	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: gotmplfumpt [flags] [path ...]\n\n")
		flags.PrintDefaults()
	}
	var (
		writeFlag = flags.Bool("w", false,
			"write result to (source) file instead of stdout")
		listFlag = flags.Bool("l", false,
			"list files whose formatting differs from gotmplfumpt's")
		diffFlag = flags.Bool("d", false,
			"display diffs instead of rewriting files")
		versionFlag = flags.Bool("version", false,
			"print version information and exit")
	)
	if err := flags.Parse(args[1:]); err != nil {
		// Wrap with %w so callers can still detect flag.ErrHelp via
		// errors.Is, while wrapcheck sees an explicitly-wrapped error.
		return fmt.Errorf("parse flags: %w", err)
	}

	if *versionFlag {
		initVersionInfo()
		_, _ = fmt.Fprintf(stdout, "gotmplfumpt %s (commit: %s, date: %s)\n", tag, commit, date)
		return nil
	}

	opts := processOpts{
		stdout:   stdout,
		write:    *writeFlag,
		list:     *listFlag,
		showDiff: *diffFlag,
	}

	if flags.NArg() == 0 {
		if opts.write {
			return errors.New("cannot use -w with standard input")
		}
		if opts.list {
			return errors.New("cannot use -l with standard input")
		}
		return processReader(stdin, stdout)
	}

	for _, arg := range flags.Args() {
		if err := processPath(arg, opts); err != nil {
			return err
		}
	}
	return nil
}

func processPath(path string, opts processOpts) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		if err := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if isTemplateFile(p) {
				return processFile(p, opts)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		return nil
	}
	return processFile(path, opts)
}

func processFile(path string, opts processOpts) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := format.Format(string(src))
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	switch {
	case opts.list:
		return emitListLine(opts.stdout, path, string(src), out)
	case opts.showDiff:
		return emitDiff(opts.stdout, path, src, out)
	case opts.write:
		return writeIfChanged(path, string(src), out)
	default:
		return emitStdout(opts.stdout, out)
	}
}

func emitListLine(w io.Writer, path, src, out string) error {
	if out == src {
		return nil
	}
	_, _ = fmt.Fprintln(w, path)
	return nil
}

func emitDiff(w io.Writer, path string, src []byte, out string) error {
	if out == string(src) {
		return nil
	}
	d := diff.Diff(path+".orig", src, path, []byte(out))
	if _, err := w.Write(d); err != nil {
		return fmt.Errorf("write diff: %w", err)
	}
	return nil
}

func writeIfChanged(path, src, out string) error {
	if out == src {
		return nil
	}
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func emitStdout(w io.Writer, out string) error {
	if _, err := io.WriteString(w, out); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}

func processReader(r io.Reader, w io.Writer) error {
	src, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	out, err := format.Format(string(src))
	if err != nil {
		return fmt.Errorf("format stdin: %w", err)
	}
	if _, err := io.WriteString(w, out); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}

func isTemplateFile(path string) bool {
	lower := strings.ToLower(path)
	for _, s := range templateSuffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

func initVersionInfo() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			date = s.Value
		}
	}
}
