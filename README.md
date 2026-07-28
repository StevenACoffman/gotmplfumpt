# Gotmplfumpt - Go Codegen Templates Formatter

This is a formatter for Go templates that emit Go code.

It parses the template with the [text/template/parse](https://pkg.go.dev/text/template/parse) grammar (Go 1.20.4, see license below), partitions the source into a gap-free set of typed slices, replaces each `{{ ... }}` action with a syntactically-valid Go sentinel (an identifier for value actions, a comment for control tags such as `{{if}}`/`{{end}}`), runs [gofumpt](https://github.com/mvdan/gofumpt) on the result, verifies that gofumpt left every sentinel intact and in order, and restores the original actions in place — so the output is gofumpt-compliant where the underlying Go is gofumpt-compliant.

Most codegen templates are fragments rather than whole files, so when gofumpt rejects a stub we retry it two ways before giving up: wrapped in a synthetic `package` clause, and with any declaration-level action (one alone on its line, such as `{{ reserveImport ... }}`) held as a comment. We try each form only after the previous one fails to parse, so a retry reaches more templates but never changes one that already formats. Whatever gofumpt still cannot parse takes the fallback.

- We have no options.
- We use tabs for indentation (gofumpt does).
- We support `{{/* gotmplfumpt-ignore-all */}}`, `{{/* gotmplfumpt-ignore-start */}}` and `{{/* gotmplfumpt-ignore-end */}}` to skip regions.
- Control tags (`{{if}}`, `{{range}}`, `{{with}}`, `{{else}}`, `{{end}}`) don't add indentation on either path: they render to no Go braces, so their bodies stay at the surrounding Go level, and the template matches the shape of the Go it renders to.
- We format `define` block bodies: a body that is standalone Go is reformatted by gofumpt (wrapped in a synthetic package first if it lacks one); a body that contains template actions is indented by its own structure.
- When gofumpt still rejects the stubbed Go (for example, the template splits a Go statement across actions), we fall back to an indent pass driven by the same source model: indentation follows the literal Go braces alone, with control tags invisible, so the fallback matches the primary path. Output is still idempotent in that case.
- We don't auto-add trailing newlines.
- We care about idempotency: if you find an input that formats differently on a second pass, file a bug report.

## Known Limitations

- Actions that emit half a Go statement (`{{ if .X }}a, b := {{ end }} f()`) take the fallback path.
- The tool preserves verbatim any action inside a Go string literal (gofumpt doesn't reformat string bodies).
- Templates without a `package` clause are fragments. We wrap a declaration-list fragment in a synthetic `package` (holding any leading declaration-level actions as comments) so it still reaches gofumpt; only what gofumpt cannot parse either way takes the fallback.
- On the fallback path, control-tag lines (`{{if}}`/`{{range}}`/`{{end}}`) are placed at the surrounding Go-brace depth. They emit nothing — and usually carry `{{-` trim markers that delete their leading whitespace at render time — so the render can't dictate where they go; a `{{range}}` wrapping switch cases can end up deeper than the `case` labels inside it. This is cosmetic: the emitted Go still matches the render. Higher fidelity comes from routing more templates onto the gofumpt path, not from growing the fallback (see `TODO.md`).

## Install

If you have [Go](https://go.dev/doc/install) installed, you can install from source:

```text
go install github.com/StevenACoffman/gotmplfumpt@latest
```

For installers, see [releases](https://github.com/StevenACoffman/gotmplfumpt/releases).

## Usage

To use this as a CLI tool, you can run:

```text
usage: gotmplfumpt [flags] [path ...]

  -d	   display diffs instead of rewriting files
  -l	   list files whose formatting differs from gotmplfumpt's
  -w	   write result to (source) file instead of stdout
  -version print version information and exit
```

Without flags, `gotmplfumpt` prints the formatted output to stdout. When you point it at a directory, it processes all Go-template files recursively. Recognized suffixes: `.tpl.go`, `.go.tpl`, `.gotmpl.go`, `.tmpl.go`, `.go.tmpl`, `.gotmpl`. It also reads from stdin when you supply no paths.

## Producing Gofumpt-Clean Output

`gotmplfumpt` formats the **template source**. The **rendered Go** is only as gofumpt-clean as the template makes it. The two common holdouts are alignment cases — consecutive `const` items and struct-literal keys — because gofumpt right-pads the shorter name in a group to align columns, and a template that emits one item per iteration can't pre-compute the max width without help.

The `tmplfunc` subpackage offers a `padRight` helper for exactly this. Install it once in your codegen's `FuncMap`:

```go
import (
	"text/template"

	"github.com/StevenACoffman/gotmplfumpt/tmplfunc"
)

t := template.New("").Funcs(tmplfunc.FuncMap()).Parse(src)
```

Then in the template, compute the max width in a first pass and pad in the second:

```text
{{- $max := 0 -}}
{{- range .Items -}}
  {{- if gt (len .Name) $max }}{{ $max = len .Name }}{{ end -}}
{{- end -}}
const (
{{- range .Items }}
	{{ padRight .Name $max }} = "{{ .Value }}"
{{- end }}
)
```

Renders to:

```go
const (
	Alpha    = "1"
	BetaLong = "2"
	C        = "3"
)
```

gofumpt sees this as a fixed point and makes no changes.

## Recommended Related Tools

- [templatecheck](https://github.com/jba/templatecheck)

### CI

To verify that CI keeps every file formatted, use the `-l` flag:

```text
gotmplfumpt -l . | grep . && exit 1
```

Or use `-d` to display the diffs:

```text
gotmplfumpt -d .
```

In a GitHub Actions, you may want to add something like these steps to your workflow:

```yaml
steps:
  - name: Install gotmplfumpt
    run: go install github.com/StevenACoffman/gotmplfumpt@latest
  - name: Check go template formatting
    run: diff <(gotmplfumpt -d layouts) <(printf '')
```

## Why?

**Note:** it is *easy* to render a Go template into a buffer and then format the result with `gofumpt`:

```go

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateFileData{Capabilities: caps}); err != nil {
		fmt.Println(err)
		return
	}
	formatted, err := format.Source(buf.Bytes(), format.Options{})

```

The motivations for wanting to format codegen `*.gotmpl` template source files are:

- Humans find it simpler to read and maintain a Go template file that matches the shape of the Go code it renders to after code generation. An improvement or bug fix in the rendered output `*.go` can then be backported to the codegen `*.gotmpl` template source file.
- Further, static analysis of rendered `*.go` files is standard practice, although generated files are often exempted from analysis despite being as prone to bugs as any other Go. I want at least machine-assisted tooling that can backport static-analysis suggestions from those `*.go` files to their codegen `*.gotmpl` template source file.
- Ideally this work can extend to syntax-aware static analysis on the `*.gotmpl` template source files themselves.

## Lineage

- This is a fork of [gotmplfmt](https://github.com/gohugoio/gotmplfmt) which was for HTML templates.
- That was a fork of [gotmplfmt](https://github.com/josharian/gotmplfmt).
- That was derived from the `text/template/parse` package in Go standard library 1.20.4

## License

See the LICENSE file for the license terms.

This code is based on code from the Go standard library. The BSD-ish license for that code is:

```text
Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```
