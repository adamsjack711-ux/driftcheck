package model

import (
	"fmt"
	"sort"
	"strings"
)

// Tree is the normalized form of one parsed config file: every leaf value
// keyed by its full path. Nested maps use dot-separated segments
// ("server.timeout"). List elements use bracketed indices ("hosts[0]") —
// unless every element is a map sharing a unique identity field ("name",
// "key", or "id"), in which case elements are keyed by identity
// ("env[name=LOG_LEVEL].value") so that inserting or reordering elements
// doesn't misalign every element after it. Empty maps and lists become
// leaves of their own so their existence is still visible. Literal path
// metacharacters inside a key segment are escaped with a backslash.
type Tree map[string]Value

// identityKeys are candidate fields for keying list-of-map elements, tried
// in order. "name" first: it is the Kubernetes convention (env, ports,
// volumes) and the most common in hand-written config.
var identityKeys = []string{"name", "key", "id"}

// EscapeSegment escapes path metacharacters in a single key segment.
func EscapeSegment(seg string) string {
	r := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `[`, `\[`, `]`, `\]`)
	return r.Replace(seg)
}

// Flatten walks a decoded config document (maps, slices, scalars) and
// produces a Tree. A bare scalar document flattens to the single path "".
func Flatten(doc any) Tree {
	t := Tree{}
	flattenInto(t, "", normalize(doc))
	return t
}

// normalize converts decoder-specific container types ([]map[string]any from
// BurntSushi/toml, map[any]any from older YAML paths) into the two canonical
// container shapes so the walk below only handles those.
func normalize(x any) any {
	switch node := x.(type) {
	case []map[string]any:
		out := make([]any, len(node))
		for i, m := range node {
			out[i] = m
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(node))
		for k, v := range node {
			out[fmt.Sprint(k)] = v
		}
		return out
	default:
		return x
	}
}

func flattenInto(t Tree, path string, x any) {
	switch node := x.(type) {
	case map[string]any:
		if len(node) == 0 {
			t[path] = EmptyMap()
			return
		}
		for k, v := range node {
			flattenInto(t, joinPath(path, EscapeSegment(k)), normalize(v))
		}
	case []any:
		if len(node) == 0 {
			t[path] = EmptyList()
			return
		}
		for i := range node {
			node[i] = normalize(node[i])
		}
		if idKey := listIdentityKey(node); idKey != "" {
			for _, el := range node {
				m := el.(map[string]any)
				id, _ := FromGo(m[idKey])
				seg := fmt.Sprintf("%s[%s=%s]", path, idKey, EscapeSegment(id.Canonical()))
				flattenInto(t, seg, el)
			}
			return
		}
		for i, v := range node {
			flattenInto(t, fmt.Sprintf("%s[%d]", path, i), v)
		}
	default:
		t[path] = FromGoAlways(x)
	}
}

// listIdentityKey returns the field to key this list's elements by, or ""
// for positional indexing. Keyed matching requires every element to be a
// map carrying the candidate field with a unique scalar value — anything
// less (scalar lists, missing fields, duplicate identities) stays
// positional, because order may then be the semantics (e.g. command args).
func listIdentityKey(items []any) string {
	for _, cand := range identityKeys {
		seen := make(map[string]bool, len(items))
		ok := true
		for _, it := range items {
			m, isMap := it.(map[string]any)
			if !isMap {
				return "" // non-map element: no candidate can work
			}
			raw, has := m[cand]
			if !has {
				ok = false
				break
			}
			v, scalar := FromGo(raw)
			if !scalar {
				ok = false
				break
			}
			id := v.Canonical()
			if seen[id] {
				ok = false
				break
			}
			seen[id] = true
		}
		if ok {
			return cand
		}
	}
	return ""
}

func joinPath(base, seg string) string {
	if base == "" {
		return seg
	}
	return base + "." + seg
}

// SortedPaths returns every key path in the tree in lexical order.
func (t Tree) SortedPaths() []string {
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// SecretCandidates returns the name-like pieces of a path that secret
// detection should test: the final key segment plus every identity value
// used in keyed-list segments — so env[name=DB_PASSWORD].value is caught
// even though its last segment is just "value".
func SecretCandidates(path string) []string {
	segs := splitSegments(path)
	if len(segs) == 0 {
		return nil
	}
	last := segs[len(segs)-1]
	cands := []string{stripIndex(last)}
	for _, seg := range segs {
		if i := strings.Index(seg, "["); i >= 0 {
			inner := strings.TrimSuffix(seg[i+1:], "]")
			if j := strings.Index(inner, "="); j >= 0 {
				cands = append(cands, inner[j+1:])
			}
		}
	}
	return cands
}

// LastSegment returns the final key segment of a path, with escapes removed
// and any list suffix stripped ("db.api_key" -> "api_key", "hosts[0]" -> "hosts").
func LastSegment(path string) string {
	segs := splitSegments(path)
	if len(segs) == 0 {
		return ""
	}
	return stripIndex(segs[len(segs)-1])
}

// splitSegments splits a path on unescaped dots and removes escapes.
func splitSegments(path string) []string {
	var segs []string
	var cur strings.Builder
	escaped := false
	for _, r := range path {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '.':
			segs = append(segs, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	segs = append(segs, cur.String())
	return segs
}

func stripIndex(seg string) string {
	if i := strings.IndexByte(seg, '['); i > 0 {
		return seg[:i]
	}
	return seg
}
