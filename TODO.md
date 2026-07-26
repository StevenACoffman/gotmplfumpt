# TODO

Outstanding work, most actionable first. Items marked _(discovered)_ surfaced
while replacing the stub/restore heuristics with the `internal/tiling` source
model; items marked _(design)_ come from the SQLFluff source-map comparison.

## Architecture — finish routing everything through the tiling

The tiling replaced the heuristics on the **gofumpt path** only. The fallback
path is still a parallel universe with its own scanner and its own adjacency
pass, so the SQLFluff "one invariant, one source of truth" win holds for the
primary path but not globally. These two items close that gap; B depends on A.

- [ ] **A. Route the fallback path through the tiling.** _(design)_ Highest-
      leverage remaining work. `fallbackFormat` still indents from
      `root.String()` (the AST printer) plus its own `lineWalker` brace scanner
      and `reindentByDepth` — it never sees the tiling. Build the tiling once and
      drive the brace-depth indent from its typed slices instead, so the primary
      and fallback paths share one scan of the source. Location:
      `internal/format/fallback.go`, `internal/format/format.go`.

- [ ] **B. Delete `parse/adjacent.go` and collapse the duplicate scanners.**
      _(design)_ Depends on A. The gofumpt path's adjacency is now
      `precededByLiteral` (a read on the tiling), but the fallback path still
      recomputes it via `markAdjacency` — two adjacency computations, and four
      independent `{{…}}` scanners total (`tiling/scan.go`, `format/fallback.go`
      `lineWalker`, `parse/lex.go`, `parse/adjacent.go`). Once the fallback runs
      on the tiling (A), `markAdjacency`/`PrevAdjacent` and the printer's
      adjacency suppression become dead, and the fallback scanner folds into the
      tiling scanner. Location: `internal/parse/adjacent.go`,
      `internal/parse/node.go`, `internal/format/fallback.go`.

## Correctness follow-ups

- [ ] **Reindent corrupts a balanced multi-line raw string inside an action.**
      _(discovered)_ `reindentContinuation`'s guard skips only when the action's
      raw has an _odd_ number of backticks, so an action like
      `` {{ printf `a\nb` }} `` (balanced) is reindented, inserting indentation
      _into_ the raw string's value. Pre-existing behavior, ported verbatim.
      Fix: track raw-string spans within the action and skip continuation lines
      that fall inside one, rather than counting backticks over the whole raw.
      Location: `internal/tiling/reindent.go`.

- [ ] **Decide whether opaque `define` bodies should be truly verbatim.**
      _(discovered)_ An opaque define block is currently reindented to its
      sentinel column, stripping interior indentation (e.g. eventgen-publish IN
      `\t{{- if` becomes OUT `{{- if`). This is faithful to the pre-tiling tool,
      so it was preserved — but "opaque" arguably implies verbatim. If we skip
      reindent for `Define` slices, regenerate the eventgen-* golden outputs and
      note the behavior change. Location: `internal/tiling/stubrestore.go`
      (`Restore`) — guard reindent on `s.Type != Define`.

- [ ] **Lock in the trim-marker whitespace behavior with a fixture.**
      _(discovered)_ The tiling emits literals verbatim from source, so
      whitespace the old `TextNode`-based stub dropped before a `{{-` marker
      (e.g. `A  {{- .X}}`) is now preserved. No golden fixture exercises this.
      Add `internal/format/testdata/golden/{in,out}/trim-marker-whitespace.*`
      to pin the (more correct) behavior and confirm it is intended.

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
