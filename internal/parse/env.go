package parse

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// parseEnv reads dotenv-style KEY=VALUE files. All values are strings —
// .env has no type system, and pretending otherwise would manufacture
// type drift between two .env files. Malformed lines become warnings,
// not errors, so one bad line doesn't sink the whole comparison.
func parseEnv(data []byte) (model.Tree, []string, error) {
	tree := model.Tree{}
	var warnings []string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			warnings = append(warnings, fmt.Sprintf("line %d: no '=' found, skipped: %s", lineNo, truncate(line, 60)))
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if !envKeyRe.MatchString(key) {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid key %q, skipped", lineNo, truncate(key, 60)))
			continue
		}
		value := parseEnvValue(strings.TrimSpace(line[eq+1:]))
		if _, dup := tree[model.EscapeSegment(key)]; dup {
			warnings = append(warnings, fmt.Sprintf("line %d: duplicate key %q, later value wins", lineNo, key))
		}
		tree[model.EscapeSegment(key)] = model.StringVal(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, warnings, err
	}
	return tree, warnings, nil
}

// parseEnvValue strips quotes and inline comments the way dotenv loaders do:
// double quotes expand escapes, single quotes are literal, and an unquoted
// " #" starts a comment.
func parseEnvValue(raw string) string {
	if len(raw) >= 2 {
		switch {
		case raw[0] == '"' && raw[len(raw)-1] == '"':
			return expandEscapes(raw[1 : len(raw)-1])
		case raw[0] == '\'' && raw[len(raw)-1] == '\'':
			return raw[1 : len(raw)-1]
		}
	}
	if i := strings.Index(raw, " #"); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	return raw
}

func expandEscapes(s string) string {
	var b strings.Builder
	escaped := false
	for _, r := range s {
		if !escaped {
			if r == '\\' {
				escaped = true
			} else {
				b.WriteRune(r)
			}
			continue
		}
		switch r {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '"', '\\', '\'':
			b.WriteRune(r)
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
		escaped = false
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
