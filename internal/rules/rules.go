// Package rules loads .driftcheck.yaml and answers three questions: is a key
// expected to differ between environments (ignore / ignore_values), should a
// file be skipped in directory comparisons (ignore_files), and does a key
// name or value look like a secret (redact).
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultFileName is searched for in the working directory and its parents
// when --config is not given.
const DefaultFileName = ".driftcheck.yaml"

// fileSchema is the on-disk shape of .driftcheck.yaml.
type fileSchema struct {
	// Ignore lists key paths whose drift is fully expected — value drift AND
	// presence drift are suppressed. Matched against the full flattened path;
	// "*" matches any run of characters (including dots).
	Ignore []string `yaml:"ignore"`
	// IgnoreValues lists key paths whose VALUE is expected to differ per
	// environment but which must still exist on both sides — a missing key
	// still counts as drift. This is usually what you want for things like
	// DATABASE_URL.
	IgnoreValues []string `yaml:"ignore_values"`
	// IgnoreFiles lists relative file paths (same glob syntax) that
	// compare-dir should skip entirely — e.g. per-environment patch files in
	// kustomize overlays that legitimately exist on only one side.
	IgnoreFiles []string `yaml:"ignore_files"`
	// SecretPatterns are extra Go regexes matched (case-insensitively)
	// against key names, in addition to the built-in secret patterns.
	SecretPatterns []string `yaml:"secret_patterns"`
	// NoDefaultSecrets disables the built-in secret name/value patterns.
	NoDefaultSecrets bool `yaml:"no_default_secrets"`
}

// Rules is the compiled, queryable form.
type Rules struct {
	ignore       []*regexp.Regexp
	ignoreValues []*regexp.Regexp
	ignoreFiles  []*regexp.Regexp
	secrets      []*regexp.Regexp
	secretValues []*regexp.Regexp
}

// builtinSecretNameRe matches key names that conventionally hold credentials.
var builtinSecretNameRe = regexp.MustCompile(
	`(?i)(^|[_.-])(api[_-]?key|secret([_-]key)?|token|passw(or)?d|pwd|credentials?|private[_-]?key|access[_-]?key|dsn|connection[_-]?string|authorization|bearer)([_.-]|$)`,
)

// builtinSecretValueRes match values that are credentials regardless of what
// the key is called — the most common real leak is a password embedded in a
// connection URL under an innocent name.
var builtinSecretValueRes = []*regexp.Regexp{
	regexp.MustCompile(`://[^/\s@:]+:[^/\s@]+@`),                           // URL userinfo with password
	regexp.MustCompile(`\b(AKIA|ASIA)[A-Z0-9]{16}\b`),                      // AWS access key id
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.`), // JWT
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`),                    // PEM private key
}

// Default returns the rules used when no config file exists: no ignores,
// built-in secret patterns only.
func Default() *Rules {
	return &Rules{
		secrets:      []*regexp.Regexp{builtinSecretNameRe},
		secretValues: builtinSecretValueRes,
	}
}

// Discover finds the rules file to use: the explicit --config path if given
// (missing then = error), else the nearest .driftcheck.yaml walking up from
// the working directory. Returns the loaded rules and the path actually used
// ("" when falling back to defaults) so reports can say which file applied.
func Discover(explicitPath string) (*Rules, string, error) {
	if explicitPath != "" {
		r, err := Load(explicitPath, true)
		return r, explicitPath, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return Default(), "", nil
	}
	for {
		candidate := filepath.Join(dir, DefaultFileName)
		if _, err := os.Stat(candidate); err == nil {
			r, err := Load(candidate, true)
			return r, candidate, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Default(), "", nil
		}
		dir = parent
	}
}

// Load reads and compiles a rules file. explicit says the caller named the
// file directly, in which case a missing file is an error instead of a
// silent fallback to defaults.
func Load(path string, explicit bool) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return Default(), nil
		}
		return nil, err
	}
	var schema fileSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("%s: invalid rules file: %w", path, err)
	}
	return compile(schema, path)
}

func compile(schema fileSchema, path string) (*Rules, error) {
	r := &Rules{}
	globs := []struct {
		pats []string
		dst  *[]*regexp.Regexp
		name string
	}{
		{schema.Ignore, &r.ignore, "ignore"},
		{schema.IgnoreValues, &r.ignoreValues, "ignore_values"},
		{schema.IgnoreFiles, &r.ignoreFiles, "ignore_files"},
	}
	for _, g := range globs {
		for _, pat := range g.pats {
			re, err := globToRegexp(pat)
			if err != nil {
				return nil, fmt.Errorf("%s: bad %s pattern %q: %w", path, g.name, pat, err)
			}
			*g.dst = append(*g.dst, re)
		}
	}
	if !schema.NoDefaultSecrets {
		r.secrets = append(r.secrets, builtinSecretNameRe)
		r.secretValues = append(r.secretValues, builtinSecretValueRes...)
	}
	for _, pat := range schema.SecretPatterns {
		re, err := regexp.Compile("(?i)" + pat)
		if err != nil {
			return nil, fmt.Errorf("%s: bad secret pattern %q: %w", path, pat, err)
		}
		r.secrets = append(r.secrets, re)
	}
	return r, nil
}

// globToRegexp compiles an ignore pattern: literal text except "*", which
// matches any run of characters, dots included. Anchored on both ends.
func globToRegexp(pat string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString(`^`)
	for i, part := range strings.Split(pat, "*") {
		if i > 0 {
			b.WriteString(`.*`)
		}
		b.WriteString(regexp.QuoteMeta(part))
	}
	b.WriteString(`$`)
	return regexp.Compile(b.String())
}

func matchAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// IsIgnored reports whether a key path's drift is fully expected
// (presence and value).
func (r *Rules) IsIgnored(path string) bool { return matchAny(r.ignore, path) }

// IsValueIgnored reports whether a key path's VALUE drift is expected.
// Presence drift on these keys still counts.
func (r *Rules) IsValueIgnored(path string) bool { return matchAny(r.ignoreValues, path) }

// IsFileIgnored reports whether a relative file path should be skipped in
// directory comparisons. Paths are matched slash-separated.
func (r *Rules) IsFileIgnored(relPath string) bool {
	return matchAny(r.ignoreFiles, filepath.ToSlash(relPath))
}

// IsSecretName reports whether a key name looks like it holds a credential.
func (r *Rules) IsSecretName(name string) bool { return matchAny(r.secrets, name) }

// IsSecretValue reports whether a value itself looks like a credential
// (URL-embedded password, AWS key, JWT, PEM), regardless of the key name.
func (r *Rules) IsSecretValue(value string) bool { return matchAny(r.secretValues, value) }
