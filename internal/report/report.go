// Package report renders comparison results for humans (grouped,
// color-coded) and for CI (--json).
package report

import (
	"github.com/adamsjack711-ux/driftcheck/internal/diff"
	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

// FilePair is one compared pair plus everything needed to render it.
type FilePair struct {
	NameA, NameB string
	Result       diff.Result
	WarningsA    []string // non-fatal parse warnings (e.g. skipped .env lines)
	WarningsB    []string
}

// FileError is a file that could not be loaded or parsed.
type FileError struct {
	Name string `json:"file"`
	Err  string `json:"error"`
}

// Report is the full outcome of a run: one pair for `compare`, many for
// `compare-dir`, plus files that exist on only one side of a dir comparison.
type Report struct {
	Pairs     []FilePair
	OnlyInA   []string // relative paths present only under dirA
	OnlyInB   []string
	Errors    []FileError
	DirA      string // set for compare-dir
	DirB      string
	RulesFile string // path of the rules file that applied, "" if defaults
}

// TotalDrift sums unignored drift across all pairs, plus unmatched files in a
// dir comparison — a config file that exists in staging but not prod is drift.
func (r *Report) TotalDrift() int {
	n := len(r.OnlyInA) + len(r.OnlyInB)
	for _, p := range r.Pairs {
		n += p.Result.Counts().Drift()
	}
	return n
}

// TotalWarnings counts non-fatal parse warnings across all pairs. Under
// --strict these fail the run: a file that is mostly skipped lines can
// otherwise compare "clean" while being garbage.
func (r *Report) TotalWarnings() int {
	n := 0
	for _, p := range r.Pairs {
		n += len(p.WarningsA) + len(p.WarningsB)
	}
	return n
}

// CategoryTotals sums drift by category across the whole report, for
// --fail-on filtering.
type CategoryTotals struct {
	Missing int // keys missing on either side
	Value   int
	Type    int
	Files   int // files present on only one side of a dir comparison
}

func (r *Report) Categories() CategoryTotals {
	var t CategoryTotals
	t.Files = len(r.OnlyInA) + len(r.OnlyInB)
	for _, p := range r.Pairs {
		c := p.Result.Counts()
		t.Missing += c.MissingInA + c.MissingInB
		t.Value += c.ValueDrift
		t.Type += c.TypeDrift
	}
	return t
}

// Options controls rendering.
type Options struct {
	Verbose     bool // include same-value keys and ignored drift
	ShowSecrets bool // render secret values instead of [redacted]
	Color       bool
}

// redacted is what secret values render as unless --show-secrets is set.
const redacted = "[redacted]"

func displayValue(v *model.Value, secret bool, opts Options) string {
	if v == nil {
		return ""
	}
	if secret && !opts.ShowSecrets {
		return redacted
	}
	return v.Display()
}
