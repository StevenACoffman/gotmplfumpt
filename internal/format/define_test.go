// define_test.go is a white-box test of the unexported define-body helpers.
//
//nolint:testpackage // white-box test of unexported helpers.
package format

import "testing"

// TestFormatGoFragment covers the cascade: whole-file direct, package-less
// wrapped, and the refusals (empty, contains a template action, not Go).
func TestFormatGoFragment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		core   string
		want   string
		wantOK bool
	}{
		{
			name:   "whole file with package",
			core:   "package main\nfunc   F(){}",
			want:   "package main\n\nfunc F() {}",
			wantOK: true,
		},
		{
			name:   "package-less declaration is wrapped",
			core:   "func   F(){}",
			want:   "func F() {}",
			wantOK: true,
		},
		{
			name:   "multiple package-less declarations",
			core:   "type T struct{ X int }\nfunc   F(){}",
			want:   "type T struct{ X int }\n\nfunc F() {}",
			wantOK: true,
		},
		{
			name:   "template action is refused",
			core:   "var x = []T{{1}}", // contains "{{"
			want:   "var x = []T{{1}}",
			wantOK: false,
		},
		{
			name:   "empty is refused",
			core:   "",
			want:   "",
			wantOK: false,
		},
		{
			name:   "non-declaration go is refused",
			core:   "return 1",
			want:   "return 1",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := formatGoFragment(tt.core)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("formatGoFragment(%q) = (%q, %v), want (%q, %v)",
					tt.core, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestReformatDefineBlock checks that a Go body is reflowed while surrounding
// whitespace and the delimiters are preserved, and that a template-fragment
// body is left untouched.
func TestReformatDefineBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "go body reflowed, surrounding newlines kept",
			raw:  "{{ define \"x\" }}\nfunc   F(){}\n{{ end }}",
			want: "{{ define \"x\" }}\nfunc F() {}\n{{ end }}",
		},
		{
			name: "template-fragment body untouched",
			raw:  "{{define \"x\"}}{{ .Y }}{{end}}",
			want: "{{define \"x\"}}{{ .Y }}{{end}}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := reformatDefineBlock(tt.raw); got != tt.want {
				t.Errorf("reformatDefineBlock(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestSplitSurroundingSpace checks the whitespace split used to preserve a
// define body's leading and trailing layout.
func TestSplitSurroundingSpace(t *testing.T) {
	t.Parallel()
	lead, core, trail := splitSurroundingSpace("\n\tfunc F(){}\n")
	if lead != "\n\t" || core != "func F(){}" || trail != "\n" {
		t.Errorf("got (%q, %q, %q)", lead, core, trail)
	}
	if l, c, tr := splitSurroundingSpace("  \t "); c != "" || l+tr != "  \t " {
		t.Errorf("all-space split = (%q, %q, %q), want empty core", l, c, tr)
	}
}
