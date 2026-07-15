// Package rules loads .driftcheck.yaml and answers two questions about a key:
// is it expected to differ between environments (ignore), and does its name
// look like a secret (redact).
package rules

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultFileName is looked up in the working directory when --config is not given.
const DefaultFileName = ".driftcheck.yaml"

// fileSchema is the on-disk shape of .driftcheck.yaml.
type fileSchema struct {
	// Ignore lists key paths expected to differ per environment. Entries are
	// matched against the full flattened path; "*" matches any run of
	// characters (including dots), so "features.*" covers a whole subtree.
	Ignore []string `yaml:"ignore"`
	// SecretPatterns are extra Go regexes matched (case-insensitively) against
	// the final key segment, in addition to the built-in secret patterns.
	SecretPatterns []string `yaml:"secret_patterns"`
	// NoDefaultSecrets disables the built-in secret name patterns.
	NoDefaultSecrets bool `yaml:"no_default_secrets"`
}

// Rules is the compiled, queryable form.
type Rules struct {
	ignore  []*regexp.Regexp
	secrets []*regexp.Regexp
}

// builtinSecretRe matches key names that conventionally hold credentials:
// API_KEY, AUTH_TOKEN, DB_PASSWORD, client_secret, private_key, ...
var builtinSecretRe = regexp.MustCompile(
	`(?i)(^|[_.-])(api[_-]?key|secret([_-]key)?|token|passw(or)?d|pwd|credentials?|private[_-]?key|access[_-]?key)([_.-]|$)`,
)

// Default returns the rules used when no config file exists: no ignores,
// built-in secret patterns only.
func Default() *Rules {
	return &Rules{secrets: []*regexp.Regexp{builtinSecretRe}}
}

// Load reads and compiles a rules file. explicit says the user passed
// --config themselves, in which case a missing file is an error instead of
// silently falling back to defaults.
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
	for _, pat := range schema.Ignore {
		re, err := globToRegexp(pat)
		if err != nil {
			return nil, fmt.Errorf("%s: bad ignore pattern %q: %w", path, pat, err)
		}
		r.ignore = append(r.ignore, re)
	}
	if !schema.NoDefaultSecrets {
		r.secrets = append(r.secrets, builtinSecretRe)
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

// IsIgnored reports whether a full key path matches any ignore rule.
func (r *Rules) IsIgnored(path string) bool {
	for _, re := range r.ignore {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// IsSecret reports whether a key's final segment looks like a credential.
func (r *Rules) IsSecret(lastSegment string) bool {
	for _, re := range r.secrets {
		if re.MatchString(lastSegment) {
			return true
		}
	}
	return false
}
