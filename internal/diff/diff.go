// Package diff compares two normalized config trees and classifies every
// key path's drift.
package diff

import (
	"sort"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
	"github.com/adamsjack711-ux/driftcheck/internal/rules"
)

// DriftType classifies one key path's comparison outcome.
type DriftType string

const (
	// Same: present in both files with identical kind and value.
	Same DriftType = "same"
	// MissingInB: present in A, absent from B.
	MissingInB DriftType = "missing_in_b"
	// MissingInA: present in B, absent from A.
	MissingInA DriftType = "missing_in_a"
	// ValueDrift: present in both with different values.
	ValueDrift DriftType = "value_drift"
	// TypeDrift: same canonical value, different type — "8080" vs 8080,
	// "true" vs true. Distinct from ValueDrift because it usually means a
	// quoting/templating bug rather than an intentional config change.
	TypeDrift DriftType = "type_drift"
)

// IsDrift reports whether this outcome counts against the exit code.
func (d DriftType) IsDrift() bool { return d != Same }

// Entry is the comparison result for one key path.
type Entry struct {
	Path    string
	Type    DriftType
	A, B    *model.Value // nil on the missing side
	Secret  bool         // key name matched a secret pattern; redact in output
	Ignored bool         // key matched an ignore rule; excluded from exit code
}

// Result is the full comparison of one file pair.
type Result struct {
	Entries []Entry // sorted by path
}

// Counts tallies entries by classification.
type Counts struct {
	Same       int
	MissingInA int
	MissingInB int
	ValueDrift int
	TypeDrift  int
	Ignored    int // drifting entries suppressed by ignore rules
}

// Drift is the number of unignored drifting entries — the exit-code signal.
func (c Counts) Drift() int {
	return c.MissingInA + c.MissingInB + c.ValueDrift + c.TypeDrift
}

func (r Result) Counts() Counts {
	var c Counts
	for _, e := range r.Entries {
		if e.Type == Same {
			c.Same++
			continue
		}
		if e.Ignored {
			c.Ignored++
			continue
		}
		switch e.Type {
		case MissingInA:
			c.MissingInA++
		case MissingInB:
			c.MissingInB++
		case ValueDrift:
			c.ValueDrift++
		case TypeDrift:
			c.TypeDrift++
		}
	}
	return c
}

// Compare classifies every key path in the union of the two trees.
// Ignore rules and secret patterns come from cfg; secrets are still compared
// on their real values — redaction happens at render time.
func Compare(a, b model.Tree, cfg *rules.Rules) Result {
	paths := map[string]struct{}{}
	for p := range a {
		paths[p] = struct{}{}
	}
	for p := range b {
		paths[p] = struct{}{}
	}

	union := make([]string, 0, len(paths))
	for p := range paths {
		union = append(union, p)
	}
	sort.Strings(union)

	var res Result
	for _, p := range union {
		va, inA := a[p]
		vb, inB := b[p]
		e := Entry{
			Path:    p,
			Secret:  cfg.IsSecret(model.LastSegment(p)),
			Ignored: cfg.IsIgnored(p),
		}
		switch {
		case inA && !inB:
			e.Type, e.A = MissingInB, &va
		case !inA && inB:
			e.Type, e.B = MissingInA, &vb
		default:
			e.A, e.B = &va, &vb
			switch {
			case va.Equal(vb):
				e.Type = Same
			case va.Kind != vb.Kind && va.Canonical() == vb.Canonical():
				e.Type = TypeDrift
			default:
				e.Type = ValueDrift
			}
		}
		res.Entries = append(res.Entries, e)
	}
	return res
}
