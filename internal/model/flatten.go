package model

import (
	"fmt"
	"sort"
	"strings"
)

// Tree is the normalized form of one parsed config file: every leaf value
// keyed by its full path. Nested maps use dot-separated segments
// ("server.timeout"), list elements use bracketed indices ("hosts[0]").
// Literal dots or brackets inside a key segment are escaped with a backslash
// so paths stay unambiguous.
type Tree map[string]Value

// EscapeSegment escapes path metacharacters in a single key segment.
func EscapeSegment(seg string) string {
	r := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `[`, `\[`)
	return r.Replace(seg)
}

// Flatten walks a decoded config document (maps, slices, scalars) and
// produces a Tree. A bare scalar document flattens to the single path "".
func Flatten(doc any) Tree {
	t := Tree{}
	flattenInto(t, "", doc)
	return t
}

func flattenInto(t Tree, path string, x any) {
	switch node := x.(type) {
	case map[string]any:
		for k, v := range node {
			flattenInto(t, joinPath(path, EscapeSegment(k)), v)
		}
	case map[any]any: // yaml.v2-style / non-string keys
		for k, v := range node {
			flattenInto(t, joinPath(path, EscapeSegment(fmt.Sprint(k))), v)
		}
	case []any:
		for i, v := range node {
			flattenInto(t, fmt.Sprintf("%s[%d]", path, i), v)
		}
	case []map[string]any: // BurntSushi/toml arrays of tables
		for i, v := range node {
			flattenInto(t, fmt.Sprintf("%s[%d]", path, i), v)
		}
	default:
		t[path] = FromGoAlways(x)
	}
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

// LastSegment returns the final key segment of a path, with escapes removed —
// the piece secret detection matches against ("db.api_key" -> "api_key").
func LastSegment(path string) string {
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
	last := segs[len(segs)-1]
	// Strip a trailing list index: "hosts[0]" -> "hosts".
	if i := strings.IndexByte(last, '['); i > 0 {
		last = last[:i]
	}
	return last
}
