package render

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gotmplfumpt/internal/tiling"
)

// hunk is one contiguous change region parsed from a unified diff.
type hunk struct {
	renderedLine int      // 1-based line in the rendered (old) file where it starts
	before       []string // removed (rendered) lines
	after        []string // added (gofumpt) lines
}

// findings turns the unified diff of rendered→gofumpt'd into Findings, mapping
// each hunk back to a template line where a literal match can be proven.
func findings(diffText, src string) []Finding {
	index := collapseIndex(src)
	til, tilOK := scanTiling(src)
	hunks := unifiedHunks(diffText)
	out := make([]Finding, 0, len(hunks))
	for _, h := range hunks {
		f := Finding{
			RenderedLine: h.renderedLine,
			Before:       strings.Join(h.before, "\n"),
			After:        strings.Join(h.after, "\n"),
		}
		if line, ok := mapHunk(h, index); ok {
			f.TemplateLine = line
			f.Enclosing = enclosingAt(til, tilOK, src, line)
			f.Note = mappedNote(f.Enclosing)
		} else {
			f.Note = unmappedNote(h)
		}
		out = append(out, f)
	}
	return out
}

// mapHunk returns the template line a hunk maps to: the first before-line whose
// whitespace-collapsed text occurs on exactly one template line. A template line
// holding an action collapses to a key containing "{{", which no action-free
// rendered line can equal, so a match is always to literal text. ok is false
// when no before-line matches uniquely.
func mapHunk(h hunk, index map[string][]int) (int, bool) {
	for _, line := range h.before {
		key := collapseWS(line)
		if key == "" {
			continue
		}
		if lines := index[key]; len(lines) == 1 {
			return lines[0], true
		}
	}
	return 0, false
}

// collapseIndex maps each template line's whitespace-collapsed text to the
// 1-based line numbers that produce it. Blank lines are skipped.
func collapseIndex(src string) map[string][]int {
	index := make(map[string][]int)
	for i, line := range strings.Split(src, "\n") {
		if key := collapseWS(line); key != "" {
			index[key] = append(index[key], i+1)
		}
	}
	return index
}

// collapseWS trims a line and replaces every run of whitespace with one space,
// so a comparison ignores exactly the indentation and spacing gofumpt reflows.
func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// unifiedHunks parses a unified diff into contiguous change regions. Each run of
// '-'/'+' lines between context lines becomes one hunk, tagged with the old-file
// (rendered) line where it begins.
func unifiedHunks(diffText string) []hunk {
	var hunks []hunk
	var cur *hunk
	oldLine := 0
	flush := func() {
		if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			oldLine = parseHunkOldStart(line)
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			// File headers, not content.
		case strings.HasPrefix(line, "-"):
			if cur == nil {
				cur = &hunk{renderedLine: oldLine}
			}
			cur.before = append(cur.before, line[1:])
			oldLine++
		case strings.HasPrefix(line, "+"):
			if cur == nil {
				cur = &hunk{renderedLine: oldLine}
			}
			cur.after = append(cur.after, line[1:])
		default:
			// Context or blank line.
			flush()
			oldLine++
		}
	}
	flush()
	return hunks
}

// parseHunkOldStart reads the old-file start line from an "@@ -a,b +c,d @@"
// header (the number after the '-'). It returns 0 on a malformed header.
func parseHunkOldStart(header string) int {
	dash := strings.IndexByte(header, '-')
	if dash < 0 {
		return 0
	}
	end := dash + 1
	for end < len(header) && header[end] >= '0' && header[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(header[dash+1 : end])
	return n
}

// scanTiling scans src into a tiling, reporting ok=false if it cannot (so
// enclosing classification degrades to TopLevel rather than failing the run).
func scanTiling(src string) (tiling.Tiling, bool) {
	til, err := tiling.ScanTiling(src)
	return til, err == nil
}

// enclosingAt returns the control-flow context of template line at src's given
// 1-based line, or TopLevel when the tiling is unavailable.
func enclosingAt(til tiling.Tiling, tilOK bool, src string, line int) Enclosing {
	if !tilOK {
		return TopLevel
	}
	return enclosingControl(til, lineStartOffset(src, line))
}

// enclosingControl reports the control body innermost-enclosing the byte offset:
// the keyword of the last {{range}}/{{with}}/{{if}} opened but not yet closed
// before offset. {{else}} does not change nesting, so a line after it is still
// inside its {{if}}.
func enclosingControl(til tiling.Tiling, offset int) Enclosing {
	stack := openControlStack(til, offset)
	if len(stack) == 0 {
		return TopLevel
	}
	return enclosingFor(stack[len(stack)-1])
}

// openControlStack returns the keywords of the control tags open at offset,
// outermost first — a push on each {{range}}/{{if}}/{{with}}, a pop on each
// {{end}}.
func openControlStack(til tiling.Tiling, offset int) []string {
	var stack []string
	for _, s := range til.Slices {
		if s.Start >= offset {
			break
		}
		switch s.Type {
		case tiling.BlockOpen:
			stack = append(stack, controlKeyword(til.Raw(s)))
		case tiling.BlockClose:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case tiling.Literal, tiling.Action, tiling.BlockMid, tiling.Comment, tiling.Define:
			// No effect on control nesting.
		}
	}
	return stack
}

// enclosingFor maps a control keyword to the context it establishes.
func enclosingFor(keyword string) Enclosing {
	switch keyword {
	case "range", "with", "block":
		return InLoop
	case "if":
		return InConditional
	default:
		return TopLevel
	}
}

// controlKeyword returns the leading keyword of a control tag such as
// "{{- range $x := .Y }}" ("range"), skipping the delimiter, an optional trim
// marker, and whitespace.
func controlKeyword(raw string) string {
	s := strings.TrimPrefix(raw, "{{")
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimLeft(s, " \t")
	end := 0
	for end < len(s) && s[end] >= 'a' && s[end] <= 'z' {
		end++
	}
	return s[:end]
}

// lineStartOffset returns the byte offset of the start of the 1-based line in
// src, clamped to len(src).
func lineStartOffset(src string, line int) int {
	off := 0
	for n := 1; n < line; n++ {
		nl := strings.IndexByte(src[off:], '\n')
		if nl < 0 {
			return len(src)
		}
		off += nl + 1
	}
	return off
}

// mappedNote explains a mapped finding, including the caveat its control context
// implies.
func mappedNote(e Enclosing) string {
	switch e {
	case InLoop:
		return "mapped to a template line inside a {{range}}/{{with}} body; the change applies to every iteration"
	case InConditional:
		return "mapped to a template line inside an {{if}} body; the line renders only for some data"
	case TopLevel:
		return "mapped to a top-level template line"
	default:
		return "mapped to a template line"
	}
}

// unmappedNote explains why a hunk has no template line.
func unmappedNote(h hunk) string {
	if len(h.before) == 0 {
		return "gofumpt inserted lines here; no rendered line to map back to the template"
	}
	return "not mapped to a template line; the change is on templated output (likely data-dependent, e.g. column alignment)"
}
