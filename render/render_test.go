package render_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gotmplfumpt/render"
)

// mustDiagnose runs Diagnose and fails on an unexpected error.
func mustDiagnose(t *testing.T, src string, data any) render.Report {
	t.Helper()
	rep, err := render.Diagnose(src, nil, data)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	return rep
}

// assertNoActionLineMapped checks the core safety property: a mapped finding
// always points at a literal template line (one with no "{{").
func assertNoActionLineMapped(t *testing.T, src string, rep render.Report) {
	t.Helper()
	lines := strings.Split(src, "\n")
	for _, f := range rep.Findings {
		if f.TemplateLine == 0 {
			continue
		}
		line := lines[f.TemplateLine-1]
		if strings.Contains(line, "{{") {
			t.Errorf("mapped TemplateLine %d is an action line: %q", f.TemplateLine, line)
		}
	}
}

func TestDiagnoseClean(t *testing.T) {
	t.Parallel()
	src := "package {{ .Pkg }}\n\nfunc F() {}\n"
	rep := mustDiagnose(t, src, map[string]string{"Pkg": "main"})
	if !rep.Clean {
		t.Fatalf("want Clean, got Diff:\n%s", rep.Diff)
	}
	if len(rep.Findings) != 0 {
		t.Errorf("clean report should have no findings, got %d", len(rep.Findings))
	}
}

func TestDiagnoseMapsLiteralChange(t *testing.T) {
	t.Parallel()
	// The messy line (x:=1) is literal, so gofumpt's spacing fix maps back to it.
	src := "package {{ .Pkg }}\n\nfunc F() {\nx:=1\n_ = x\n}\n"
	rep := mustDiagnose(t, src, map[string]string{"Pkg": "main"})
	if rep.Clean {
		t.Fatal("want changes, got Clean")
	}
	assertNoActionLineMapped(t, src, rep)

	var mapped *render.Finding
	for i := range rep.Findings {
		if rep.Findings[i].TemplateLine != 0 {
			mapped = &rep.Findings[i]
			break
		}
	}
	if mapped == nil {
		t.Fatalf("want a mapped finding, got %+v", rep.Findings)
	}
	if mapped.Enclosing != render.TopLevel {
		t.Errorf("want TopLevel, got %v", mapped.Enclosing)
	}
	if got := strings.Split(src, "\n")[mapped.TemplateLine-1]; strings.TrimSpace(got) != "x:=1" {
		t.Errorf("mapped to line %d %q, want the x:=1 line", mapped.TemplateLine, got)
	}
}

func TestDiagnoseFlagsDataDependentAlignment(t *testing.T) {
	t.Parallel()
	// Column alignment depends on the rendered value widths, so the changed
	// lines carry action output and cannot map back — the honest flag.
	src := "package main\n\nconst (\n{{ .A }} = \"1\"\n{{ .B }} = \"22\"\n)\n"
	rep := mustDiagnose(t, src, map[string]string{"A": "X", "B": "YY"})
	if rep.Clean {
		t.Fatal("want alignment changes, got Clean")
	}
	assertNoActionLineMapped(t, src, rep)

	unmapped := false
	for _, f := range rep.Findings {
		if f.TemplateLine == 0 && strings.Contains(f.Note, "templated output") {
			unmapped = true
		}
	}
	if !unmapped {
		t.Errorf("want an unmapped data-dependent finding, got %+v", rep.Findings)
	}
}

func TestDiagnoseClassifiesRange(t *testing.T) {
	t.Parallel()
	// x:=1 is literal and unique in the template but sits inside {{range}}.
	src := "package main\n\n{{ range .Fns }}\nfunc {{ . }}() {\nx:=1\n_ = x\n}\n{{ end }}\n"
	rep := mustDiagnose(t, src, map[string][]string{"Fns": {"A", "B"}})
	if rep.Clean {
		t.Fatal("want changes, got Clean")
	}
	assertNoActionLineMapped(t, src, rep)

	inLoop := false
	for _, f := range rep.Findings {
		if f.TemplateLine != 0 && f.Enclosing == render.InLoop {
			inLoop = true
		}
	}
	if !inLoop {
		t.Errorf("want a finding mapped inside the {{range}}, got %+v", rep.Findings)
	}
}

func TestDiagnoseErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src     string
		data    any
		wantSub string
	}{
		"parse error":   {"package main {{ .X", nil, "parse template"},
		"execute error": {"package main {{ .Missing }}", struct{}{}, "execute template"},
		"invalid go":    {"this is not go {{ .X }}", map[string]string{"X": "z"}, "not valid Go"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := render.Diagnose(tc.src, nil, tc.data)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q, want it to contain %q", err, tc.wantSub)
			}
		})
	}
}

func TestVerifyFormatPreservesRender(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src  string
		data any
	}{
		"already clean whole file": {
			"package main\n\nfunc F() {}\n", nil,
		},
		"reformatted but render-preserving": {
			"package main\n\nfunc F() {\nx:={{ .N }}\n_ = x\n}\n",
			map[string]string{"N": "1"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := render.VerifyFormatPreservesRender(tc.src, nil, tc.data); err != nil {
				t.Errorf("want nil, got %v", err)
			}
		})
	}
}

func TestVerifyFormatPreservesRenderErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src     string
		data    any
		wantSub string
	}{
		"cannot format": {"package main {{ .X", nil, "format template"},
		"execute error": {"package main {{ .Missing }}", struct{}{}, "execute template"},
		"invalid go":    {"this is not go\n", nil, "not valid Go"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := render.VerifyFormatPreservesRender(tc.src, nil, tc.data)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q, want it to contain %q", err, tc.wantSub)
			}
		})
	}
}
