package format_test

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/StevenACoffman/gotmplfumpt/internal/format"
)

const goldenWritePerm = 0o600

// update is set via -update to overwrite golden outputs.
var update = flag.Bool("update", false, "update the golden files")

// TestGolden walks every fixture in testdata/golden/in, formats it, and
// compares against the matching file under testdata/golden/out. The
// per-file body is delegated to runGoldenCase to keep complexity bounded.
func TestGolden(t *testing.T) {
	if *update {
		t.Log("Updating golden files...")
	}
	// testdata is a special directory name the Go tool excludes from
	// builds, so fixture files ending in .go don't get compiled.
	goldenDir := filepath.Join("testdata", "golden")
	goldenDirIn := filepath.Join(goldenDir, "in")
	goldenDirOut := filepath.Join(goldenDir, "out")

	if *update {
		if err := os.RemoveAll(goldenDirOut); err != nil {
			t.Fatalf("remove golden out: %v", err)
		}
		if err := os.MkdirAll(goldenDirOut, 0o755); err != nil {
			t.Fatalf("mkdir golden out: %v", err)
		}
	}

	walkFn := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		baseName := strings.TrimPrefix(path, goldenDirIn+string(os.PathSeparator))
		t.Run(path, func(t *testing.T) {
			runGoldenCase(t, path, filepath.Join(goldenDirOut, baseName))
		})
		return nil
	}
	if err := filepath.Walk(goldenDirIn, walkFn); err != nil {
		t.Fatalf("walk golden/in: %v", err)
	}
}

// runGoldenCase loads one fixture, formats it (both unix and windows
// line endings), and compares against the golden output.
func runGoldenCase(t *testing.T, inputPath, goldenPath string) {
	t.Helper()
	b, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	unix := toUnixLineEndings(string(b))
	checkFormatted(t, unix, goldenPath)
	if !*update {
		checkFormatted(t, toWindowsLineEndings(unix), goldenPath)
	}
}

// checkFormatted runs Format on input, compares to the golden file (or
// overwrites it when -update is set), then asserts idempotency.
func checkFormatted(t *testing.T, input, goldenPath string) {
	t.Helper()
	tryParseGoTextTemplate(t, input)
	output, err := format.Format(input)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if *update {
		if err := os.WriteFile(goldenPath, []byte(output), goldenWritePerm); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if output != toUnixLineEndings(string(expected)) {
		t.Errorf("output mismatch\nGot:\n%s\nExpected:\n%s", output, expected)
	}
	output2, err := format.Format(output)
	if err != nil {
		t.Fatalf("re-format: %v", err)
	}
	if output != output2 {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", output, output2)
	}
	tryParseGoTextTemplate(t, output)
}

// tryParseGoTextTemplate validates that text parses as a Go text/template.
// Common Go-template helper names that code-generation templates often
// reference are registered as stubs so the parser doesn't reject them;
// the formatter never executes the template, so the stubs never run.
func tryParseGoTextTemplate(t *testing.T, text string) {
	t.Helper()
	fn := func() string { return "" }
	funcMap := template.FuncMap{
		"append":  fn,
		"default": fn,
		"dict":    fn,
		"first":   fn,
		"where":   fn,
	}
	if _, err := template.New("").Funcs(funcMap).Parse(text); err != nil {
		t.Fatal("Error parsing template:", err)
	}
}

func toUnixLineEndings(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func toWindowsLineEndings(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}
