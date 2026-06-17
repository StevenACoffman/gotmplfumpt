# Gotmplfumpt - Go Codegen Templates Formatter

This is a formatter for Go templates that emit Go code. It parses the template with the [text/template/parse](https://pkg.go.dev/text/template/parse) grammar (Go 1.20.4, see license below), substitutes each `{{ ... }}` action with a syntactically-valid Go sentinel, runs [gofumpt](https://github.com/mvdan/gofumpt) on the result, and restores the original actions in place — so the output is gofumpt-compliant where the underlying Go is gofumpt-compliant.

- We have no options.
- We use tabs for indentation (gofumpt does).
- We support `{{/* gotmplfumpt-ignore-all */}}`, `{{/* gotmplfumpt-ignore-start */}}` and `{{/* gotmplfumpt-ignore-end */}}` to skip regions.
- `define` blocks are emitted verbatim — their bodies are formatted as separate Go code if they parse standalone.
- When gofumpt rejects the stubbed Go (e.g., the template emits a fragment rather than a whole file, or splits a Go statement across actions), we fall back to a brace-counting indent pass. Output is still idempotent in that case.
- We don't auto-add trailing newlines.
- We care about idempotency: if you find an input that formats differently on a second pass, please report it as a bug.

## Known Limitations

- Actions that emit half a Go statement (`{{ if .X }}a, b := {{ end }} f()`) hit the fallback path.
- Actions inside Go string literals are preserved verbatim (gofumpt doesn't reformat string bodies).
- Templates without a `package` clause render as fragments — the fallback path handles them.

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

Without flags, `gotmplfumpt` prints the formatted output to stdout. When given a directory, it processes all Go-template files recursively. Recognized suffixes: `.tpl.go`, `.go.tpl`, `.gotmpl.go`, `.tmpl.go`, `.go.tmpl`, `.gotmpl`. It also reads from stdin when no paths are given.

### CI

To check that all files are formatted in CI, you can use the `-l` flag:

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

````go

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateFileData{Capabilities: caps}); err != nil {
		fmt.Println(err)
		return
	}
	formatted, err := format.Source(buf.Bytes(), format.Options{})```

````

The motivations for wanting to format codegen `*.gotmpl` template source files are:

- It’s easier for humans to read and maintain a Go template file that is formatted like the Go code it will render out to after code generation. An improvement or bug fix in the concrete rendered output *.go can be easily “backported” to the codegen*.gotmpl template source file
- Further, Static analysis of rendered *.go files is standard practice, although generated files are often exempted from analysis despite them being just as prone to bugs. I would like to at least have machine assisted tooling that can backport static analysis suggested fixes from those*.go files to their codegen \*.gotmpl template source file.
- Ideally this could eventually be extended to do syntax aware static analysis on the \*.gotmpl template source files themselves.

## Lineage

This is a fork of [gotmplfmt](https://github.com/gohugoio/gotmplfmt) which was for HTML templates.
That was a fork of [gotmplfmt](https://github.com/josharian/gotmplfmt).
That was derived from the `text/template/parse` package in Go standard library 1.20.4

## License

For the license for this code, please see the LICENSE file.

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
