# TODO

Outstanding work, most actionable first. Items marked _(discovered)_ surfaced
while replacing the stub/restore heuristics with the `internal/tiling` source
model; items marked _(design)_ come from the SQLFluff source-map comparison.

## Architecture — finish routing everything through the tiling

The tiling replaced the heuristics on the **gofumpt path** first. Item A then
routed the fallback through the tiling too, so both paths now share one scan.
Item B is the remaining cleanup: the AST printer and its adjacency pass are now
dead outside `parse`'s own tests.

- [x] **A. Route the fallback path through the tiling.** _(done)_ The fallback
      (`internal/format/indent.go`, `tilingIndent`) now indents `til.Src` by
      combined template-block depth (from the tiling's typed slices) and
      Go-brace depth (from a scan of the Literal slices), keeping everything
      verbatim. `format.go` builds the tiling once and shares it with both
      paths. Deleted the old `fallback.go` (`reindentByDepth`/`lineWalker`,
      ~398 lines). All golden outputs unchanged.

- [x] **B. Delete `parse/adjacent.go` and the printer's adjacency.** _(done)_
      Deleted `adjacent.go` + its test, removed the `markAdjacency` call and the
      `PrevAdjacent` fields, and reverted the printer to its unconditional
      `writeControlIndent` default (the more idempotent path). The kept printer
      (public `String()` API) round-trips as before, confirmed by
      `FuzzParseString`; all golden outputs unchanged (both adjacency fixtures
      now format via the tiling, not the printer). This collapses the last
      duplicate `{{…}}` scanner outside the `parse/lex.go` lexer.

## Formatter strategy — reduce fallbacks, feed gofumpt richer Go

Keep gofumpt as the Go-layout oracle (the tool's contract chains its output to
gofumpt's byte-for-byte, so replacing gofumpt would mean cloning it exactly).
Instead, do more of the work *around* gofumpt and hand it better stubs.

- [x] **Format opaque `define` bodies via gofumpt-on-a-wrapped fragment.**
      _(done)_ `internal/format/define.go` reflows each PURE-GO define body
      (a whole file directly, a package-less declaration list wrapped in a
      synthetic `package` and unwrapped) before the main pipeline. Bodies
      containing a template action (`{{…}}`) are refused, so their delimiters
      are never reflowed — this leaves template-fragment defines (e.g.
      eventgen) untouched. define-block's golden now shows cleaned-up bodies;
      all others unchanged; idempotent.

- [ ] **Context-sensitive stubbing: map control tags to real Go blocks.**
      _(design)_ Highest-upside experiment. Today `{{if}}` becomes an inert
      `/* comment */`, chosen so template nesting need not match Go brace
      nesting — but then gofumpt never sees the block as an indent scope, which
      forces the reindent hacks and many fallbacks. In **statement context**,
      stub `{{if .X}}…{{end}}` as `if true {…}` and `{{range}}` as
      `for range x {…}` — real Go blocks gofumpt indents natively — then restore
      the delimiters. This is "make the process aware of both languages" done by
      feeding gofumpt richer Go rather than replacing it. Risk: detecting
      statement vs. expression vs. top-level context is itself non-trivial and is
      where a new edge-case tail could grow; gate it behind the context the
      tiling can determine reliably, and fall back to comment sentinels
      otherwise. Location: `internal/tiling/sentinel.go`, `scan.go`.

## Correctness follow-ups

- [x] **Reindent corrupts a balanced multi-line raw string inside an action.**
      _(done)_ Replaced the odd-backtick-count guard with `stringLineMask`, a
      per-line scan that marks any line beginning inside a string, raw-string, or
      char literal so reindent leaves it verbatim. Fuzzing (`FuzzStructurePreserved`)
      caught a deeper case the first cut missed — an interpreted string holding a
      literal newline (`{{"\n"}}`) desynced the mask and panicked; the fix records
      exactly one entry per newline. Regression seed committed under
      `internal/tiling/testdata/fuzz`. `internal/tiling/reindent.go`.

- [ ] **Decide whether reindented template-fragment `define` bodies should be
      verbatim.** _(discovered)_ Scope narrowed by the define-formatting work
      above: pure-Go define bodies are now properly formatted, but a
      template-fragment define body (with `{{…}}`, e.g. eventgen) still takes
      the primary path and is reindented to its sentinel column, stripping
      interior indentation (eventgen-publish IN `\t{{- if` → OUT `{{- if`).
      Decide whether to skip reindent for `Define` slices; if so, regenerate the
      eventgen-* goldens. Location: `internal/tiling/stubrestore.go` (`Restore`)
      — guard reindent on `s.Type != Define`.

- [x] **Lock in the trim-marker whitespace behavior with a fixture.** _(done)_
      Added `trim-marker-whitespace.tpl.go` (a fallback-path fragment where the
      preservation is not masked by gofumpt reindentation): the two spaces
      before `{{- .Tag }}`, which the old `TextNode`-based stub trimmed, survive
      to the output and the golden's idempotency/reparse gates pass.

## Simplification

- [ ] **Downgrade or drop the reparse + `shapesEqual` verify.** _(discovered)_
      With the tiling's structure-preservation property proven (golden + fuzz),
      `formatViaGofumpt`'s full reparse is largely redundant. Consider replacing
      it with an O(n) assertion that every sentinel appears exactly once in
      order, or dropping it. Currently kept as defense-in-depth against gofumpt
      doing something unexpected. Location: `internal/format/verify.go`,
      `internal/format/format.go`.

## Enhancements

- [ ] **`BlockIdx`/`Block` field on `RawSlice`.** _(design)_ Formalize which
      template block owns each slice (SQLFluff `block_idx`, base.py:83). Enables
      a verify step to assert gofumpt never reordered a sentinel across a block
      boundary — a safety property, not cosmetic. Optional. Location:
      `internal/tiling/tiling.go`, `internal/tiling/scan.go`.

## Project direction (from README "Why?")

- [ ] **Syntax-aware static analysis on `.gotmpl` source directly.** The tiling's
      typed slices are a foundation for linting template source without
      rendering.

- [ ] **Backport static-analysis findings from rendered `.go` to template
      source.** _(design)_ Requires a rendered↔source map — the SQLFluff
      direction deliberately NOT built here (gotmplfumpt never renders). Revisit
      only if this becomes a goal; see `scratchpad/` design notes for why the
      render→backport approach is a poor fit for a data-less formatter.

## Done (this effort)

- [x] Gap-free typed source tiling with an asserted invariant (`Check`,
      SQLFluff base.py:195-207).
- [x] String-literal-aware source scanner (fixes `Index("}}")` splitting on a
      `}}` inside a string).
- [x] Self-delimiting sentinels (no `_a1_`-inside-`_a12_` prefix collision).
- [x] Opaque `define` coalescing and multi-line reindent in the tiling.
- [x] Swapped `format.formatViaGofumpt` onto the tiling; deleted the old
      `stub.go`/`restore.go` (684 lines). Golden outputs unchanged.
