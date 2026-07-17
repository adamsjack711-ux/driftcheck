package model

import "testing"

func TestFlattenNested(t *testing.T) {
	doc := map[string]any{
		"a": map[string]any{
			"b": 1,
			"c": []any{"x", map[string]any{"d": true}},
		},
		"dotted.key": "v",
	}
	tree := Flatten(doc)

	cases := map[string]string{
		"a.b":         "1",
		"a.c[0]":      "x",
		"a.c[1].d":    "true",
		`dotted\.key`: "v",
	}
	for path, want := range cases {
		v, ok := tree[path]
		if !ok {
			t.Errorf("missing path %q in %v", path, tree.SortedPaths())
			continue
		}
		if v.Canonical() != want {
			t.Errorf("%s = %q, want %q", path, v.Canonical(), want)
		}
	}
	if len(tree) != len(cases) {
		t.Errorf("got %d leaves, want %d: %v", len(tree), len(cases), tree.SortedPaths())
	}
}

func TestFlattenBareScalar(t *testing.T) {
	tree := Flatten("just a string")
	v, ok := tree[""]
	if !ok || v.Str != "just a string" {
		t.Errorf("bare scalar should flatten to root path, got %v", tree)
	}
}

func TestLastSegment(t *testing.T) {
	cases := map[string]string{
		"API_KEY":       "API_KEY",
		"db.password":   "password",
		"a.b.c":         "c",
		"hosts[0]":      "hosts",
		"a.list[3]":     "list",
		`dotted\.key`:   "dotted.key",
		`a.dotted\.key`: "dotted.key",
	}
	for path, want := range cases {
		if got := LastSegment(path); got != want {
			t.Errorf("LastSegment(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestValueCanonicalAndEqual(t *testing.T) {
	if FloatVal(8080).Canonical() != "8080" {
		t.Errorf("float 8080 canonical = %q, want 8080", FloatVal(8080).Canonical())
	}
	if IntVal(8080).Equal(FloatVal(8080)) {
		t.Error("int and float with same magnitude must not be Equal (that's type drift)")
	}
	if !StringVal("x").Equal(StringVal("x")) {
		t.Error("identical strings should be Equal")
	}
}
