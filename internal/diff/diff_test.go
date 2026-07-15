package diff

import (
	"testing"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
	"github.com/adamsjack711-ux/driftcheck/internal/rules"
)

func entryByPath(t *testing.T, r Result, path string) Entry {
	t.Helper()
	for _, e := range r.Entries {
		if e.Path == path {
			return e
		}
	}
	t.Fatalf("no entry for path %q", path)
	return Entry{}
}

func TestCompareClassification(t *testing.T) {
	a := model.Tree{
		"same":       model.StringVal("x"),
		"only_in_a":  model.StringVal("gone"),
		"changed":    model.IntVal(30),
		"type_drift": model.StringVal("8080"),
		"both":       model.BoolVal(true),
	}
	b := model.Tree{
		"same":       model.StringVal("x"),
		"only_in_b":  model.StringVal("new"),
		"changed":    model.IntVal(45),
		"type_drift": model.IntVal(8080),
		"both":       model.BoolVal(false),
	}

	r := Compare(a, b, rules.Default())

	cases := map[string]DriftType{
		"same":       Same,
		"only_in_a":  MissingInB,
		"only_in_b":  MissingInA,
		"changed":    ValueDrift,
		"type_drift": TypeDrift,
		"both":       ValueDrift,
	}
	for path, want := range cases {
		if got := entryByPath(t, r, path).Type; got != want {
			t.Errorf("%s: got %s, want %s", path, got, want)
		}
	}

	c := r.Counts()
	if c.Drift() != 5 || c.Same != 1 {
		t.Errorf("counts = %+v, want drift 5 same 1", c)
	}
}

func TestTypeDriftVariants(t *testing.T) {
	cases := []struct {
		name string
		a, b model.Value
		want DriftType
	}{
		{"string vs int same digits", model.StringVal("8080"), model.IntVal(8080), TypeDrift},
		{"string vs bool", model.StringVal("true"), model.BoolVal(true), TypeDrift},
		{"int vs float same magnitude", model.IntVal(8080), model.FloatVal(8080), TypeDrift},
		{"string vs int different digits", model.StringVal("8080"), model.IntVal(9090), ValueDrift},
		{"null vs string null", model.Null(), model.StringVal("null"), TypeDrift},
		{"same ints", model.IntVal(1), model.IntVal(1), Same},
	}
	for _, c := range cases {
		r := Compare(model.Tree{"k": c.a}, model.Tree{"k": c.b}, rules.Default())
		if got := entryByPath(t, r, "k").Type; got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestSecretDetection(t *testing.T) {
	a := model.Tree{
		"API_KEY":          model.StringVal("aaa"),
		"db.password":      model.StringVal("hunter2"),
		"AUTH_TOKEN":       model.StringVal("t1"),
		"CLIENT_SECRET":    model.StringVal("s1"),
		"PASSWD":           model.StringVal("p"),
		"MONKEY":           model.StringVal("not secret"),
		"TIMEOUT":          model.StringVal("30"),
		"PRIVATE_KEY_PATH": model.StringVal("also flagged, name-based"),
	}
	r := Compare(a, model.Tree{}, rules.Default())

	wantSecret := map[string]bool{
		"API_KEY": true, "db.password": true, "AUTH_TOKEN": true,
		"CLIENT_SECRET": true, "PASSWD": true, "PRIVATE_KEY_PATH": true,
		"MONKEY": false, "TIMEOUT": false,
	}
	for path, want := range wantSecret {
		if got := entryByPath(t, r, path).Secret; got != want {
			t.Errorf("secret(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestIgnoreRules(t *testing.T) {
	cfg := mustRules(t, `
ignore:
  - DATABASE_URL
  - "features.*"
`)
	a := model.Tree{
		"DATABASE_URL":     model.StringVal("postgres://dev"),
		"features.a":       model.BoolVal(true),
		"features.deep.b":  model.BoolVal(true),
		"REAL_DRIFT":       model.StringVal("x"),
		"features_similar": model.StringVal("not ignored: no dot"),
	}
	b := model.Tree{
		"DATABASE_URL":     model.StringVal("postgres://prod"),
		"features.a":       model.BoolVal(false),
		"features.deep.b":  model.BoolVal(false),
		"REAL_DRIFT":       model.StringVal("y"),
		"features_similar": model.StringVal("changed"),
	}
	r := Compare(a, b, cfg)

	wantIgnored := map[string]bool{
		"DATABASE_URL": true, "features.a": true, "features.deep.b": true,
		"REAL_DRIFT": false, "features_similar": false,
	}
	for path, want := range wantIgnored {
		if got := entryByPath(t, r, path).Ignored; got != want {
			t.Errorf("ignored(%s) = %v, want %v", path, got, want)
		}
	}

	c := r.Counts()
	if c.Drift() != 2 || c.Ignored != 3 {
		t.Errorf("counts = %+v, want drift 2 ignored 3", c)
	}
}

func mustRules(t *testing.T, yaml string) *rules.Rules {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/.driftcheck.yaml"
	if err := writeFile(path, yaml); err != nil {
		t.Fatal(err)
	}
	cfg, err := rules.Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
