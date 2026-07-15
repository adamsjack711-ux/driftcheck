package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, content string) *Rules {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".driftcheck.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestIgnoreGlobs(t *testing.T) {
	r := load(t, `
ignore:
  - DATABASE_URL
  - "features.*"
  - "*_HOST"
  - "logging.level"
`)
	cases := map[string]bool{
		"DATABASE_URL":       true,
		"DATABASE_URL_EXTRA": false, // anchored: exact unless starred
		"features.a":         true,
		"features.deep.b":    true, // '*' crosses dots
		"features":           false,
		"REDIS_HOST":         true,
		"logging.level":      true,
		"logging.format":     false,
	}
	for path, want := range cases {
		if got := r.IsIgnored(path); got != want {
			t.Errorf("IsIgnored(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDefaultSecretPatterns(t *testing.T) {
	r := Default()
	secret := []string{
		"API_KEY", "api_key", "apikey", "STRIPE_SECRET", "SECRET_KEY",
		"AUTH_TOKEN", "token", "PASSWORD", "passwd", "PWD", "DB_CREDENTIALS",
		"private_key", "ACCESS_KEY",
	}
	for _, s := range secret {
		if !r.IsSecretName(s) {
			t.Errorf("IsSecret(%q) = false, want true", s)
		}
	}
	notSecret := []string{"MONKEY", "TIMEOUT", "TOKENIZER", "AUTHOR", "keyboard"}
	for _, s := range notSecret {
		if r.IsSecretName(s) {
			t.Errorf("IsSecret(%q) = true, want false", s)
		}
	}
}

func TestExtraSecretPatternsAndOptOut(t *testing.T) {
	r := load(t, `
secret_patterns:
  - internal_cred
`)
	if !r.IsSecretName("MY_INTERNAL_CRED") {
		t.Error("extra pattern should match case-insensitively")
	}
	if !r.IsSecretName("API_KEY") {
		t.Error("built-ins should still apply alongside extras")
	}

	r = load(t, `
no_default_secrets: true
secret_patterns:
  - internal_cred
`)
	if r.IsSecretName("API_KEY") {
		t.Error("no_default_secrets should disable built-ins")
	}
	if !r.IsSecretName("internal_cred_x") {
		t.Error("extra patterns should survive no_default_secrets")
	}
}

func TestMissingDefaultConfigFallsBack(t *testing.T) {
	r, err := Load(filepath.Join(t.TempDir(), "absent.yaml"), false)
	if err != nil {
		t.Fatalf("implicit missing config should not error: %v", err)
	}
	if !r.IsSecretName("API_KEY") || r.IsIgnored("anything") {
		t.Error("fallback should be defaults: built-in secrets, no ignores")
	}
}

func TestBadPatternErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.yaml")
	os.WriteFile(path, []byte("secret_patterns:\n  - \"([\"\n"), 0o644)
	if _, err := Load(path, true); err == nil {
		t.Error("invalid regex should error")
	}
}
