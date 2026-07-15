package report

import (
	"encoding/json"
	"io"

	"github.com/adamsjack711-ux/driftcheck/internal/diff"
	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

// jsonReport is the stable machine-readable schema for --json. CI consumers
// key off .summary.drift and .summary.errors; everything else is detail.
type jsonReport struct {
	Pairs    []jsonPair  `json:"pairs"`
	OnlyInA  []string    `json:"files_only_in_a,omitempty"`
	OnlyInB  []string    `json:"files_only_in_b,omitempty"`
	Errors   []FileError `json:"errors"`
	Summary  jsonSummary `json:"summary"`
}

type jsonSummary struct {
	Drift   int `json:"drift"`   // unignored drift count -> exit 1 if > 0
	Ignored int `json:"ignored"` // drift suppressed by ignore rules
	Errors  int `json:"errors"`  // load/parse failures -> exit 2 if > 0
	Pairs   int `json:"pairs"`
}

type jsonPair struct {
	FileA    string      `json:"file_a"`
	FileB    string      `json:"file_b"`
	Drifts   []jsonDrift `json:"drifts"`
	Warnings []string    `json:"warnings,omitempty"`
	Counts   jsonCounts  `json:"counts"`
}

type jsonCounts struct {
	MissingInA int `json:"missing_in_a"`
	MissingInB int `json:"missing_in_b"`
	ValueDrift int `json:"value_drift"`
	TypeDrift  int `json:"type_drift"`
	Ignored    int `json:"ignored"`
	Same       int `json:"same"`
}

type jsonDrift struct {
	Path    string     `json:"path"`
	Type    string     `json:"type"`
	A       *jsonValue `json:"a,omitempty"`
	B       *jsonValue `json:"b,omitempty"`
	Secret  bool       `json:"secret,omitempty"`
	Ignored bool       `json:"ignored,omitempty"`
}

type jsonValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func toJSONValue(v *model.Value, secret bool, opts Options) *jsonValue {
	if v == nil {
		return nil
	}
	jv := &jsonValue{Type: v.Kind.String()}
	if secret && !opts.ShowSecrets {
		jv.Value = redacted
		return jv
	}
	switch v.Kind {
	case model.KindNull:
		jv.Value = nil
	case model.KindBool:
		jv.Value = v.Bool
	case model.KindInt:
		jv.Value = v.Int
	case model.KindFloat:
		jv.Value = v.Float
	case model.KindString:
		jv.Value = v.Str
	}
	return jv
}

// RenderJSON writes the machine-readable report. Same-value entries are
// included only with Verbose, mirroring the human report.
func RenderJSON(w io.Writer, r *Report, opts Options) error {
	out := jsonReport{
		Pairs:   []jsonPair{},
		OnlyInA: r.OnlyInA,
		OnlyInB: r.OnlyInB,
		Errors:  r.Errors,
	}
	if out.Errors == nil {
		out.Errors = []FileError{}
	}

	for _, pair := range r.Pairs {
		jp := jsonPair{
			FileA:  pair.NameA,
			FileB:  pair.NameB,
			Drifts: []jsonDrift{},
		}
		jp.Warnings = append(jp.Warnings, pair.WarningsA...)
		jp.Warnings = append(jp.Warnings, pair.WarningsB...)

		c := pair.Result.Counts()
		jp.Counts = jsonCounts{
			MissingInA: c.MissingInA,
			MissingInB: c.MissingInB,
			ValueDrift: c.ValueDrift,
			TypeDrift:  c.TypeDrift,
			Ignored:    c.Ignored,
			Same:       c.Same,
		}

		for _, e := range pair.Result.Entries {
			if e.Type == diff.Same && !opts.Verbose {
				continue
			}
			jp.Drifts = append(jp.Drifts, jsonDrift{
				Path:    e.Path,
				Type:    string(e.Type),
				A:       toJSONValue(e.A, e.Secret, opts),
				B:       toJSONValue(e.B, e.Secret, opts),
				Secret:  e.Secret,
				Ignored: e.Ignored,
			})
		}
		out.Pairs = append(out.Pairs, jp)
	}

	out.Summary = jsonSummary{
		Drift:   r.TotalDrift(),
		Errors:  len(r.Errors),
		Pairs:   len(r.Pairs),
	}
	for _, pair := range r.Pairs {
		out.Summary.Ignored += pair.Result.Counts().Ignored
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
