# TODO

Outstanding work, most actionable first. Items marked _(discovered)_ surfaced
while replacing the stub/restore heuristics with the `internal/tiling` source
model; items marked _(design)_ come from the SQLFluff source-map comparison.

## Architecture — Finish Routing Everything Through the Tiling

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

## Formatter Strategy — Reduce Fallbacks, Feed Gofumpt Richer Go

Keep gofumpt as the Go-layout oracle (the tool's contract chains its output to
gofumpt's byte-for-byte, so replacing gofumpt would mean cloning it exactly).
Instead, do more of the work _around_ gofumpt and hand it better stubs.

- [x] **Format opaque `define` bodies via gofumpt-on-a-wrapped fragment.**
      _(done)_ `internal/format/define.go` reflows each PURE-GO define body
      (a whole file directly, a package-less declaration list wrapped in a
      synthetic `package` and unwrapped) before the main pipeline. Bodies
      containing a template action (`{{…}}`) are refused, so their delimiters
      are never reflowed — this leaves template-fragment defines (for example
      eventgen) untouched. define-block's golden now shows cleaned-up bodies;
      all others unchanged; idempotent.

- [x] **Context-sensitive stubbing: map control tags to real Go blocks.**
      _(won't do)_ The idea was to stub `{{if .X}}…{{end}}` as `if true {…}` in
      statement context so gofumpt indents the body natively. Investigation
      showed this **contradicts the tool's match-render goal**: a `{{if}}`
      renders to no Go braces, so its body belongs at the surrounding Go level —
      exactly what the current comment-sentinel approach already produces. The
      `if-else` fixture confirms it: `return 1` sits at one tab, matching the
      rendered `func F() int { return 1 }`. `if true {` scaffolding would indent
      the body a second level, introducing a phantom brace level absent from the
      render. The comment-sentinel design is correct and intentional, not a
      limitation. See the design principle below.

## Design Principles

- **Control-tag actions are indentation-invisible (match the render).**
  `{{if}}`/`{{range}}`/`{{with}}`/`{{else}}`/`{{end}}` produce no Go braces when
  rendered, so gotmplfumpt does not indent their bodies as if they did. Block
  tags stub to comments (not `if true {`), and indentation follows only the
  literal Go braces — so the template matches the shape of the Go it renders to.
  (Define bodies are the deliberate exception: they have no single render shape,
  being invoked elsewhere, so they are indented by their own structure.)

## Correctness Follow-Ups

- [x] **Reindent corrupts a balanced multi-line raw string inside an action.**
      _(done)_ Replaced the odd-backtick-count guard with `stringLineMask`, a
      per-line scan that marks any line beginning inside a string, raw-string, or
      char literal so reindent leaves it verbatim. Fuzzing (`FuzzStructurePreserved`)
      caught a deeper case the first cut missed — an interpreted string holding a
      literal newline (`{{"\n"}}`) desynced the mask and panicked; the fix records
      exactly one entry per newline. Regression seed committed under
      `internal/tiling/testdata/fuzz`. `internal/tiling/reindent.go`.

- [x] **Reindented template-fragment `define` bodies — resolved.** _(done)_
      Two layers. First, `Restore` now skips reindent for `Define` slices, so an
      opaque body is never flattened to the sentinel column (this also fixes a
      latent bug where that would undo the pre-pass's gofumpt formatting of a
      pure-Go body). Second, the pre-pass then structurally re-indents
      template-fragment bodies via `tilingIndent` (with an `{{else}}` dedent fix
      so `{{if/else/end}}` align), degrading to verbatim if the fragment does not
      tile. The eventgen goldens now show Go bodies indented by depth with
      control tags at column 0.

- [x] **Lock in the trim-marker whitespace behavior with a fixture.** _(done)_
      Added `trim-marker-whitespace.tpl.go` (a fallback-path fragment where the
      preservation is not masked by gofumpt reindentation): the two spaces
      before `{{- .Tag }}`, which the old `TextNode`-based stub trimmed, survive
      to the output, and the golden test's idempotency and reparse gates pass.

## Simplification

- [x] **Downgrade the reparse + `shapesEqual` verify.** _(done)_ Replaced the
      full `parse.Parse(out)` + shape compare with `Tiling.VerifyFormatted`: an
      O(n·k) scan asserting every sentinel occurs exactly once and in order.
      Deleted `internal/format/verify.go`/`verify_test.go`. Bonus correctness —
      it also catches same-kind reorders that the shape compare missed. Golden
      output unchanged.

## Enhancements

- [x] **`Block` field on `RawSlice`.** _(done)_ Added a monotonic block-region
      id (SQLFluff `block_idx` analog), computed in `ScanTiling` and incremented
      at each BlockOpen/BlockClose. `VerifyFormatted` uses it to report whether a
      displaced sentinel crossed a block boundary, and is groundwork for
      future block-aware rules. `internal/tiling/{tiling,scan,verify}.go`.

## Project Direction (From README "Why?")

- [ ] **Syntax-aware static analysis on `.gotmpl` source directly.** The tiling's
      typed slices are a foundation for linting template source without
      rendering.

- [ ] **Backport static-analysis findings from rendered `.go` to template
      source.** _(design)_ Requires a rendered↔source map — the SQLFluff
      direction deliberately NOT built here (gotmplfumpt never renders). Revisit
      only if this becomes a goal; see `scratchpad/` design notes for why the
      render→backport approach is a poor fit for a data-less formatter.

## Done (This Effort)

- [x] Gap-free typed source tiling with an asserted invariant (`Check`,
      SQLFluff base.py:195-207).
- [x] String-literal-aware source scanner (fixes `Index("}}")` splitting on a
      `}}` inside a string).
- [x] Self-delimiting sentinels (no `_a1_`-inside-`_a12_` prefix collision).
- [x] Opaque `define` coalescing and multi-line reindent in the tiling.
- [x] Swapped `format.formatViaGofumpt` onto the tiling; deleted the old
      `stub.go`/`restore.go` (684 lines). Golden outputs unchanged.
