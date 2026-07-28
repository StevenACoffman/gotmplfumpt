# TODO

Outstanding work, most actionable first. Items marked _(discovered)_ surfaced
while replacing the stub/restore heuristics with the `internal/tiling` source
model; items marked _(design)_ come from the SQLFluff source-map comparison;
items marked _(gqlgen)_ surfaced by running gotmplfumpt over the gqlgen codegen
templates (`github.com/99designs/gqlgen`).

## Fallback Indenter — Make It Gofmt-Faithful and Indentation-Invisible

The fallback path (`internal/format/indent.go`, `tilingIndent`) diverges from
both gofmt and the tool's own "control tags are indentation-invisible" design
principle. All three items below surfaced by running gotmplfumpt over the gqlgen
templates, which are Go _fragments_ and so hit the fallback exclusively. None of
the three affects rendered code — gqlgen re-runs gofmt on generated output — but
each degrades already-correct template source, which is the one thing a
formatter must never do.

- [x] **1. `case`/`default` labels over-indent inside `switch`/`select`.**
      _(gqlgen)_ _(done)_ The indenter tracked only brackets, so after `switch x {`
      it left `case`/`default` labels at the block-body depth instead of dedenting
      them one level to the `switch` level — the label and the statements it
      introduces then sat at the same column. Fix: each stack frame now records
      its bracket `kind` and whether a `{` opened a `switch`/`select` body
      (`opensSwitchBody`); `startLine` snapshots `enclosingSwitch()`, and `flush`
      dedents a `case`/`default` label line (`isCaseLabel`) by one against the
      _display_ depth only, so the case body stays at block-body level. Goto
      labels are lexically ambiguous with composite-literal keys and are left
      unhandled. Covered by white-box `indent_test.go` cases (including a
      `caseCount` non-label guard) and the `switch-case` golden.

- [x] **2. Lines that leave more than one bracket open over-indent.**
      _(gqlgen)_ _(done)_ `goBracketDelta` counted `{`, `(`, and `[` each as ±1,
      so the indenter tracked _net unclosed brackets_ and pushed a body two
      levels for `srv.Use(extension.Foo{`. Replaced by a bracket stack: each
      opener increments the indent only when it is the first still-open bracket
      its line contributes (`incremented := stack empty || top.line != curLine`),
      and its matching closer dedents only if it did — one step per continuing
      line, balanced and idempotent. Covered by the two-bracket and
      open-and-close-on-one-line `indent_test.go` cases.

- [x] **3. Control tags add a Go indent level, contradicting the design principle.**
      _(gqlgen)_ _(done)_ `blockDelta` added +1 for `{{if}}`/`{{range}}`/`{{with}}`
      (with a matching `{{else}}` dedent), so the fallback indented control-tag
      bodies even though the primary gofumpt path — and the "Control-tag actions
      are indentation-invisible" principle below — does not. Made the fallback
      invisible too: `BlockOpen`/`BlockMid`/`BlockClose` now advance like an
      `Action` and contribute zero depth; only literal Go brackets drive
      indentation, so the fallback matches the render and the gofumpt path.
      Deleted `blockDelta` and the `{{else}}` special-case; updated the
      `adjacent-actions-fragment`, `eventgen-test`, and `trim-marker-whitespace`
      goldens and the affected `indent_test.go`/`define_test.go` cases.

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

### Which Templates Fall Back, and Why _(Gqlgen, Measured)_

A probe over all 20 gqlgen `.gotpl` templates found that **every one** falls
back for a single reason: gofumpt's `format.Source` requires a `package` clause
and these are all fragments. The errors split two ways: a package-less
_declaration list_ (`expected 'package', found 'func'/'var'`) and a leading
_top-level action_ that stubs to a bare identifier (`found __gtmpl_aN_`). The
two levers below target exactly those shapes.

- [x] **Lever 1 — Synthetic-package wrap in the main pipeline.** _(done)_
      `formatStub` (`internal/format/gofumpt.go`) tries the bare stub, then on a
      parse error retries wrapped in `package gotmplfumpt\n` and strips the prefix
      back (`stripWrapPackage`, now shared with `formatGoFragment`); the trailing
      newline is kept, matching the bare gofumpt path. `formatViaGofumpt` calls it
      in place of `formatGo`. Measured on gqlgen: interface, object, directives_
      moved onto the byte-for-byte gofumpt path — object now gets gofumpt's
      struct-key column alignment (`Field:  field,`) the fallback could not — and
      all templates stay idempotent. `adjacent-actions-fragment` flipped to the
      gofumpt path (golden updated; fallback nested-range coverage retained by
      the `indent_test.go` white-box case and the `switch-case` fixture). New
      `formatStub` unit test and `package-less-decls` golden prove the lift.

- [x] **Lever 2 — Comment-stub standalone actions.** _(done)_ An action alone on
      its source line (`{{reserveImport "fmt"}}`, `{{.OriginalSource}}`) renders
      to nothing or a whole declaration; an _identifier_ stub is invalid at
      declaration position, a _comment_ stub is valid and more render-faithful.
      `ScanTiling` now marks such actions `RawSlice.Standalone`; the sentinel form
      moved into one place, `Tiling.sentinelFor` (Stub/Restore/VerifyFormatted all
      route through it), and `Tiling.WithStandaloneComments()` returns a copy that
      emits the comment form for standalone actions. `formatViaGofumpt` extracts
      `tryFormat` and runs it twice — default, then with standalone comments — so
      the comment form is used **only on a retry after the identifier stub fails**,
      which makes regression impossible. Measured on gqlgen: 7 newly lifted (args,
      generated, input, type, federation, requires, server) onto the byte-for-byte
      gofumpt path — server now gets `":"+port` operator spacing, reserveImports
      restored verbatim. `api!.gotpl` parses when commented but fails the stricter
      `VerifyFormatted`/`Restore` round-trip, so it safely falls back (the net
      working). All 20 templates idempotent; no non-lifted template changed.
      New `standalone_test.go` (marking + form) and `reserveimport-led` golden.

- [x] **Keep the fallback for the residue.** _(done — accepted)_ After both
      levers, 10 of 20 gqlgen templates reach the gofumpt path; the other 10 stay
      fallback for reasons that should not be forced: actions that emit a
      _declaration fragment_ into a grammar that rejects a placeholder
      (`var ( {{.FunctionImpl}} )` in internal, split func signatures in field,
      the func-name/params actions in resolver/stubs/models), templates that are
      not Go at all (`test.gotpl` is literally `this is my test package`), and
      `api!.gotpl`, whose comment stub parses but does not survive the sentinel
      round-trip. The three-bug fix already made the fallback a sound
      approximation for all of these.

## Design Principles

- **Control-tag actions are indentation-invisible (match the render).**
  `{{if}}`/`{{range}}`/`{{with}}`/`{{else}}`/`{{end}}` produce no Go braces when
  rendered, so gotmplfumpt does not indent their bodies as if they did. Block
  tags stub to comments (not `if true {`), and indentation follows only the
  literal Go braces — so the template matches the shape of the Go it renders to.
  Both paths now agree: the gofumpt path via comment sentinels, the fallback via
  a Go-brace-only indenter (`internal/format/indent.go`).
  (Define bodies are the deliberate exception: they have no single render shape,
  being invoked elsewhere, so they are indented by their own structure.)

  _Residual (fallback only, cosmetic):_ a control-tag line is placed at the
  surrounding Go-brace depth, but it emits nothing and usually carries a `{{-`
  trim marker that deletes its leading whitespace at render time, so the render
  cannot dictate its column — a `{{range}}` wrapping switch cases can sit deeper
  than the `case` labels inside it. The emitted Go still matches the render;
  only the invisible tag's own column is arbitrary. Aligning it would re-couple
  the indenter to template structure (the coupling removed for the invisibility
  fix), so leave it. Higher fidelity is the "Reduce Fallbacks" item above, not a
  smarter fallback.

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
      source.** _(design)_ This needs a rendered↔source map, which
      `text/template` does not provide: the executor emits bytes with no source
      positions, and the projection is many-to-one (`{{range}}` bodies emit N
      times), lossy (values become concrete bytes), branch-shadowed (data hides
      `{{else}}` arms), and whitespace-shifted (`{{-`/`-}}`). Building a sound map
      means forking the executor or an SQLFluff-style instrumented trace — large,
      and still cannot place a change that lives inside a loop body.
      One distinction decides whether this is worth it: backporting **gofumpt
      formatting** is low value, because the data-less stub→restore pipeline
      applies every data-independent formatting change without a map, and the one
      class it misses — width-dependent column alignment — is not expressible as
      static template text anyway (that is what the `padRight` helper is for). So
      a render→backport of formatting would mostly re-derive what stubbing already
      does, plus the cases that cannot be backported. Backporting
      **static-analysis / bug findings** (a missing nil check) is the version with
      real payoff: those are data-independent and worth surfacing. Same hard map,
      very different value — pursue it for findings, not for formatting.

- [x] **Render diagnostic (`render.Diagnose`).** _(done)_ A best-effort reporter,
      not a rewriter: render a template with caller-supplied data, run gofumpt on
      the result, and report what gofumpt would change — each hunk mapped back to
      a template line where a whitespace-collapsed literal line matches uniquely,
      and flagged unmappable (likely on templated / data-dependent output)
      otherwise, with the enclosing `{{range}}`/`{{if}}` noted so the caller knows
      a mapped change may repeat or be conditional. It sidesteps the source-map
      problem by claiming only mappings it can prove (a literal line present
      verbatim in the source) and flagging the rest. Public package `render`;
      never edits the template; expects a template that renders a complete Go
      file (a fragment render is reported as "not valid Go"). See `render/`.

- [x] **Render-equivalence check (`render.VerifyFormatPreservesRender`).**
      _(done)_ `func VerifyFormatPreservesRender(src string, funcs
      template.FuncMap, data any) error`. Formats the template with the data-less
      pipeline, renders both the original and the formatted template with the
      caller's data, gofumpts both, and returns nil unless the two render to
      byte-identical Go — a survivor of gofumpt is a real gotmplfumpt bug. Uses
      data for what data is good at — verifying a reformat is render-preserving —
      rather than the intractable backport, so it needs no source map. The error
      distinguishes cannot-format, a broken original, a reformat that no longer
      renders (formatter bug), and a diff. In package `render`; see `render/`.

## Done (This Effort)

- [x] Gap-free typed source tiling with an asserted invariant (`Check`,
      SQLFluff base.py:195-207).
- [x] String-literal-aware source scanner (fixes `Index("}}")` splitting on a
      `}}` inside a string).
- [x] Self-delimiting sentinels (no `_a1_`-inside-`_a12_` prefix collision).
- [x] Opaque `define` coalescing and multi-line reindent in the tiling.
- [x] Swapped `format.formatViaGofumpt` onto the tiling; deleted the old
      `stub.go`/`restore.go` (684 lines). Golden outputs unchanged.
