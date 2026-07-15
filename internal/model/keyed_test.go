package model

import "testing"

func TestFlattenKeyedList(t *testing.T) {
	doc := map[string]any{
		"env": []any{
			map[string]any{"name": "LOG_LEVEL", "value": "info"},
			map[string]any{"name": "TIMEOUT", "value": "30"},
		},
	}
	tree := Flatten(doc)
	for _, path := range []string{
		"env[name=LOG_LEVEL].name", "env[name=LOG_LEVEL].value",
		"env[name=TIMEOUT].name", "env[name=TIMEOUT].value",
	} {
		if _, ok := tree[path]; !ok {
			t.Errorf("missing keyed path %q in %v", path, tree.SortedPaths())
		}
	}
}

func TestKeyedListInsertionDoesNotMisalign(t *testing.T) {
	mk := func(names ...string) Tree {
		var items []any
		for _, n := range names {
			items = append(items, map[string]any{"name": n, "value": "v-" + n})
		}
		return Flatten(map[string]any{"env": items})
	}
	a := mk("LOG_LEVEL", "TIMEOUT", "REGION")
	b := mk("NEW_FLAG", "LOG_LEVEL", "TIMEOUT", "REGION")

	// Every path in a must exist identically in b: insertion at the head
	// must not shift anything.
	for p, va := range a {
		vb, ok := b[p]
		if !ok {
			t.Errorf("path %q vanished after insertion", p)
			continue
		}
		if !va.Equal(vb) {
			t.Errorf("path %q changed value after insertion: %s -> %s", p, va.Canonical(), vb.Canonical())
		}
	}
	if len(b) != len(a)+2 { // NEW_FLAG contributes .name and .value
		t.Errorf("b has %d leaves, want %d", len(b), len(a)+2)
	}
}

func TestKeyedListFallsBackToPositional(t *testing.T) {
	// Duplicate identities: order might be meaningful, stay positional.
	dup := Flatten(map[string]any{"l": []any{
		map[string]any{"name": "x"},
		map[string]any{"name": "x"},
	}})
	if _, ok := dup["l[0].name"]; !ok {
		t.Errorf("duplicate identities should fall back to indices: %v", dup.SortedPaths())
	}

	// Scalar lists: positional (command args semantics).
	scalars := Flatten(map[string]any{"args": []any{"--verbose", "--out", "x"}})
	if _, ok := scalars["args[0]"]; !ok {
		t.Errorf("scalar lists should stay positional: %v", scalars.SortedPaths())
	}

	// Mixed candidate: "key" works when "name" is absent.
	keyed := Flatten(map[string]any{"l": []any{
		map[string]any{"key": "a", "v": 1},
		map[string]any{"key": "b", "v": 2},
	}})
	if _, ok := keyed["l[key=a].v"]; !ok {
		t.Errorf("'key' identity should apply when 'name' absent: %v", keyed.SortedPaths())
	}
}

func TestFlattenEmptyContainers(t *testing.T) {
	tree := Flatten(map[string]any{
		"emptyMap":  map[string]any{},
		"emptyList": []any{},
		"full":      map[string]any{"a": 1},
	})
	if v, ok := tree["emptyMap"]; !ok || v.Kind != KindEmptyMap {
		t.Errorf("empty map should be a leaf, got %v", tree.SortedPaths())
	}
	if v, ok := tree["emptyList"]; !ok || v.Kind != KindEmptyList {
		t.Errorf("empty list should be a leaf, got %v", tree.SortedPaths())
	}
}

func TestSecretCandidates(t *testing.T) {
	cands := SecretCandidates("containers.env[name=DB_PASSWORD].value")
	found := false
	for _, c := range cands {
		if c == "DB_PASSWORD" {
			found = true
		}
	}
	if !found {
		t.Errorf("identity value should be a secret candidate, got %v", cands)
	}
	if cands[0] != "value" {
		t.Errorf("first candidate should be last segment, got %v", cands)
	}
}
