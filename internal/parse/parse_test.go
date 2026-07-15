package parse

import (
	"testing"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

func TestParseEnv(t *testing.T) {
	data := []byte(`
# comment
PORT=8080
export DEBUG=true
NAME="hello world"
LITERAL='no $expansion'
ESCAPED="line1\nline2"
INLINE=value # trailing comment
EMPTY=
URL=https://example.com/#anchor
BAD LINE WITHOUT EQUALS
PORT=9090
`)
	tree, warnings, err := parseEnv(data)
	if err != nil {
		t.Fatalf("parseEnv: %v", err)
	}

	want := map[string]string{
		"PORT":    "9090", // duplicate: later wins
		"DEBUG":   "true",
		"NAME":    "hello world",
		"LITERAL": "no $expansion",
		"ESCAPED": "line1\nline2",
		"INLINE":  "value",
		"EMPTY":   "",
		"URL":     "https://example.com/#anchor",
	}
	if len(tree) != len(want) {
		t.Errorf("got %d keys, want %d: %v", len(tree), len(want), tree.SortedPaths())
	}
	for k, w := range want {
		v, ok := tree[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if v.Kind != model.KindString || v.Str != w {
			t.Errorf("%s = %v (%s), want %q", k, v.Canonical(), v.Kind, w)
		}
	}
	// One warning for the bad line, one for the duplicate.
	if len(warnings) != 2 {
		t.Errorf("warnings = %v, want 2 entries", warnings)
	}
}

func TestParseJSONTypes(t *testing.T) {
	data := []byte(`{
		"port": 8080,
		"portStr": "8080",
		"ratio": 0.5,
		"whole": 3.0,
		"on": true,
		"nothing": null,
		"nested": {"deep": {"key": "v"}},
		"list": ["a", 2, false]
	}`)
	tree, _, err := parseJSON(data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	cases := []struct {
		path string
		kind model.Kind
		want string
	}{
		{"port", model.KindInt, "8080"},
		{"portStr", model.KindString, "8080"},
		{"ratio", model.KindFloat, "0.5"},
		{"whole", model.KindFloat, "3"},
		{"on", model.KindBool, "true"},
		{"nothing", model.KindNull, "null"},
		{"nested.deep.key", model.KindString, "v"},
		{"list[0]", model.KindString, "a"},
		{"list[1]", model.KindInt, "2"},
		{"list[2]", model.KindBool, "false"},
	}
	for _, c := range cases {
		v, ok := tree[c.path]
		if !ok {
			t.Errorf("missing path %q in %v", c.path, tree.SortedPaths())
			continue
		}
		if v.Kind != c.kind || v.Canonical() != c.want {
			t.Errorf("%s = %s (%s), want %s (%s)", c.path, v.Canonical(), v.Kind, c.want, c.kind)
		}
	}
}

func TestParseJSONMalformed(t *testing.T) {
	if _, _, err := parseJSON([]byte(`{"a": `)); err == nil {
		t.Error("truncated JSON should error")
	}
	if _, _, err := parseJSON([]byte(`{"a": 1} trailing`)); err == nil {
		t.Error("trailing garbage should error")
	}
}

func TestParseYAMLTypes(t *testing.T) {
	data := []byte(`
server:
  port: 8080
  host: "0.0.0.0"
  timeout: 30.5
features:
  new_checkout: true
  legacy: null
hosts:
  - a.example.com
  - b.example.com
`)
	tree, _, err := parseYAML(data)
	if err != nil {
		t.Fatalf("parseYAML: %v", err)
	}
	cases := []struct {
		path string
		kind model.Kind
		want string
	}{
		{"server.port", model.KindInt, "8080"},
		{"server.host", model.KindString, "0.0.0.0"},
		{"server.timeout", model.KindFloat, "30.5"},
		{"features.new_checkout", model.KindBool, "true"},
		{"features.legacy", model.KindNull, "null"},
		{"hosts[0]", model.KindString, "a.example.com"},
		{"hosts[1]", model.KindString, "b.example.com"},
	}
	for _, c := range cases {
		v, ok := tree[c.path]
		if !ok {
			t.Errorf("missing path %q in %v", c.path, tree.SortedPaths())
			continue
		}
		if v.Kind != c.kind || v.Canonical() != c.want {
			t.Errorf("%s = %s (%s), want %s (%s)", c.path, v.Canonical(), v.Kind, c.want, c.kind)
		}
	}
}

func TestParseYAMLMalformedAndMultiDoc(t *testing.T) {
	if _, _, err := parseYAML([]byte("key: [unclosed")); err == nil {
		t.Error("malformed YAML should error")
	}
	if _, _, err := parseYAML([]byte("a: 1\n---\nb: 2\n")); err == nil {
		t.Error("multi-doc YAML should error")
	}
	tree, _, err := parseYAML([]byte(""))
	if err != nil || len(tree) != 0 {
		t.Errorf("empty YAML: tree=%v err=%v, want empty tree, nil error", tree, err)
	}
}

func TestParseTOMLTypes(t *testing.T) {
	data := []byte(`
title = "app"
[server]
port = 8080
ratio = 0.25
enabled = true
[[workers]]
name = "w1"
[[workers]]
name = "w2"
`)
	tree, _, err := parseTOML(data)
	if err != nil {
		t.Fatalf("parseTOML: %v", err)
	}
	cases := []struct {
		path string
		kind model.Kind
		want string
	}{
		{"title", model.KindString, "app"},
		{"server.port", model.KindInt, "8080"},
		{"server.ratio", model.KindFloat, "0.25"},
		{"server.enabled", model.KindBool, "true"},
		// arrays of tables with a unique "name" field key by identity
		{"workers[name=w1].name", model.KindString, "w1"},
		{"workers[name=w2].name", model.KindString, "w2"},
	}
	for _, c := range cases {
		v, ok := tree[c.path]
		if !ok {
			t.Errorf("missing path %q in %v", c.path, tree.SortedPaths())
			continue
		}
		if v.Kind != c.kind || v.Canonical() != c.want {
			t.Errorf("%s = %s (%s), want %s (%s)", c.path, v.Canonical(), v.Kind, c.want, c.kind)
		}
	}
}

func TestParseTOMLMalformed(t *testing.T) {
	if _, _, err := parseTOML([]byte(`key = `)); err == nil {
		t.Error("malformed TOML should error")
	}
}

func TestDetectFormat(t *testing.T) {
	cases := map[string]Format{
		".env":           FormatEnv,
		".env.production": FormatEnv,
		"dev.env":        FormatEnv,
		"config.json":    FormatJSON,
		"config.yaml":    FormatYAML,
		"config.yml":     FormatYAML,
		"config.toml":    FormatTOML,
		"config.YAML":    FormatYAML,
		"README.md":      FormatUnknown,
		"binary":         FormatUnknown,
	}
	for name, want := range cases {
		if got := DetectFormat(name); got != want {
			t.Errorf("DetectFormat(%q) = %q, want %q", name, got, want)
		}
	}
}
