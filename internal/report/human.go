package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/adamsjack711-ux/driftcheck/internal/diff"
)

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiRed     = "\x1b[31m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
)

type humanPrinter struct {
	w    io.Writer
	opts Options
}

func (h humanPrinter) c(code, s string) string {
	if !h.opts.Color {
		return s
	}
	return code + s + ansiReset
}

// RenderHuman writes the terminal report.
func RenderHuman(w io.Writer, r *Report, opts Options) {
	h := humanPrinter{w: w, opts: opts}

	if opts.Verbose {
		if r.RulesFile != "" {
			fmt.Fprintf(w, "%s\n\n", h.c(ansiDim, "rules: "+r.RulesFile))
		} else {
			fmt.Fprintf(w, "%s\n\n", h.c(ansiDim, "rules: (none found — built-in defaults)"))
		}
	}

	for _, fe := range r.Errors {
		fmt.Fprintf(w, "%s %s: %s\n", h.c(ansiRed, "ERROR"), fe.Name, fe.Err)
	}
	if len(r.Errors) > 0 {
		fmt.Fprintln(w)
	}

	if r.DirA != "" {
		h.renderUnmatched(r)
	}

	for i, pair := range r.Pairs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		h.renderPair(pair)
	}

	h.renderGrandTotal(r)
}

func (h humanPrinter) renderUnmatched(r *Report) {
	w := h.w
	if len(r.OnlyInA) > 0 {
		fmt.Fprintf(w, "%s (%d)\n", h.c(ansiBold+ansiRed, "CONFIG FILES ONLY IN "+r.DirA), len(r.OnlyInA))
		for _, f := range r.OnlyInA {
			fmt.Fprintf(w, "  %s %s\n", h.c(ansiRed, "-"), f)
		}
		fmt.Fprintln(w)
	}
	if len(r.OnlyInB) > 0 {
		fmt.Fprintf(w, "%s (%d)\n", h.c(ansiBold+ansiGreen, "CONFIG FILES ONLY IN "+r.DirB), len(r.OnlyInB))
		for _, f := range r.OnlyInB {
			fmt.Fprintf(w, "  %s %s\n", h.c(ansiGreen, "+"), f)
		}
		fmt.Fprintln(w)
	}
}

func (h humanPrinter) renderPair(pair FilePair) {
	w := h.w
	fmt.Fprintf(w, "%s\n", h.c(ansiBold, fmt.Sprintf("%s <-> %s", pair.NameA, pair.NameB)))

	for _, warn := range pair.WarningsA {
		fmt.Fprintf(w, "  %s %s: %s\n", h.c(ansiYellow, "warning"), pair.NameA, warn)
	}
	for _, warn := range pair.WarningsB {
		fmt.Fprintf(w, "  %s %s: %s\n", h.c(ansiYellow, "warning"), pair.NameB, warn)
	}

	groups := groupEntries(pair.Result.Entries)
	pathWidth := maxPathWidth(pair.Result.Entries)

	h.renderGroup(groups[diff.MissingInB], fmt.Sprintf("MISSING IN %s", pair.NameB), ansiRed, "-", pathWidth,
		func(e diff.Entry) string { return displayValue(e.A, e.Secret, h.opts) })
	h.renderGroup(groups[diff.MissingInA], fmt.Sprintf("MISSING IN %s", pair.NameA), ansiGreen, "+", pathWidth,
		func(e diff.Entry) string { return displayValue(e.B, e.Secret, h.opts) })
	h.renderGroup(groups[diff.ValueDrift], "VALUE DRIFT", ansiYellow, "~", pathWidth,
		func(e diff.Entry) string {
			return fmt.Sprintf("%s -> %s", displayValue(e.A, e.Secret, h.opts), displayValue(e.B, e.Secret, h.opts))
		})
	h.renderGroup(groups[diff.TypeDrift], "TYPE DRIFT", ansiMagenta, "!", pathWidth,
		func(e diff.Entry) string {
			return fmt.Sprintf("%s (%s) -> %s (%s)",
				displayValue(e.A, e.Secret, h.opts), e.A.Kind,
				displayValue(e.B, e.Secret, h.opts), e.B.Kind)
		})

	ignored := groups["ignored"]
	if len(ignored) > 0 {
		if h.opts.Verbose {
			h.renderGroup(ignored, "IGNORED (expected drift)", ansiDim, ".", pathWidth,
				func(e diff.Entry) string {
					return fmt.Sprintf("%s -> %s", displayValue(e.A, e.Secret, h.opts), displayValue(e.B, e.Secret, h.opts))
				})
		} else {
			fmt.Fprintf(w, "\n  %s\n", h.c(ansiDim, fmt.Sprintf("%d drift(s) matched ignore rules (--verbose to list)", len(ignored))))
		}
	}

	if h.opts.Verbose {
		h.renderGroup(groups[diff.Same], "SAME", ansiDim, "=", pathWidth,
			func(e diff.Entry) string { return displayValue(e.A, e.Secret, h.opts) })
	}

	c := pair.Result.Counts()
	fmt.Fprintf(w, "\n  %s\n", summaryLine(c))
}

// groupEntries buckets by drift type, routing ignored drift to its own bucket.
func groupEntries(entries []diff.Entry) map[DriftGroupKey][]diff.Entry {
	groups := map[DriftGroupKey][]diff.Entry{}
	for _, e := range entries {
		key := DriftGroupKey(e.Type)
		if e.Ignored && e.Type != diff.Same {
			key = "ignored"
		}
		groups[key] = append(groups[key], e)
	}
	return groups
}

// DriftGroupKey is a diff.DriftType or the synthetic "ignored" bucket.
type DriftGroupKey = diff.DriftType

func (h humanPrinter) renderGroup(entries []diff.Entry, title, color, marker string, pathWidth int, value func(diff.Entry) string) {
	if len(entries) == 0 {
		return
	}
	w := h.w
	fmt.Fprintf(w, "\n%s (%d)\n", h.c(ansiBold+color, title), len(entries))
	for _, e := range entries {
		path := displayPath(e.Path)
		fmt.Fprintf(w, "  %s %-*s  %s\n", h.c(color, marker), pathWidth, path, value(e))
	}
}

func displayPath(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

func maxPathWidth(entries []diff.Entry) int {
	max := 0
	for _, e := range entries {
		if n := len(displayPath(e.Path)); n > max {
			max = n
		}
	}
	if max > 60 {
		max = 60
	}
	return max
}

func summaryLine(c diff.Counts) string {
	var parts []string
	if c.MissingInB > 0 {
		parts = append(parts, fmt.Sprintf("%d missing in B", c.MissingInB))
	}
	if c.MissingInA > 0 {
		parts = append(parts, fmt.Sprintf("%d missing in A", c.MissingInA))
	}
	if c.ValueDrift > 0 {
		parts = append(parts, fmt.Sprintf("%d value", c.ValueDrift))
	}
	if c.TypeDrift > 0 {
		parts = append(parts, fmt.Sprintf("%d type", c.TypeDrift))
	}
	drift := c.Drift()
	head := fmt.Sprintf("%d drift(s)", drift)
	if len(parts) > 0 {
		head += " (" + strings.Join(parts, ", ") + ")"
	}
	return fmt.Sprintf("Summary: %s, %d ignored, %d identical", head, c.Ignored, c.Same)
}

func (h humanPrinter) renderGrandTotal(r *Report) {
	if r.DirA == "" {
		return // single pair already printed its summary
	}
	w := h.w
	total := r.TotalDrift()
	fmt.Fprintln(w)
	if total == 0 && len(r.Errors) == 0 {
		fmt.Fprintf(w, "%s no unexpected drift across %d file pair(s)\n", h.c(ansiGreen, "OK:"), len(r.Pairs))
		return
	}
	fmt.Fprintf(w, "%s %d total drift(s) across %d file pair(s), %d unmatched file(s), %d error(s)\n",
		h.c(ansiBold, "TOTAL:"), total, len(r.Pairs), len(r.OnlyInA)+len(r.OnlyInB), len(r.Errors))
}
