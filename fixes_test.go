package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// End-to-end tests for the v0.2.0 review fixes.

func TestKeyedListSingleInsertion(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.yaml", `
env:
  - name: LOG_LEVEL
    value: info
  - name: TIMEOUT
    value: "30"
`)
	b := writeTemp(t, dir, "b.yaml", `
env:
  - name: NEW_FLAG
    value: "true"
  - name: LOG_LEVEL
    value: info
  - name: TIMEOUT
    value: "30"
`)
	out, code := runCapture(t, "compare", a, b)
	if code != 1 {
		t.Errorf("exit = %d, want 1\n%s", code, out)
	}
	// Exactly the inserted element's two leaves drift — nothing misaligns.
	if !strings.Contains(out, "2 missing in A") || strings.Contains(out, "VALUE DRIFT") {
		t.Errorf("insertion should be 2 missing-in-A and no value drift:\n%s", out)
	}
}

func TestStrictFailsOnWarnings(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "GARBAGE LINE\nOK=1\n")
	b := writeTemp(t, dir, "b.env", "OK=1\n")

	_, code := runCapture(t, "compare", a, b)
	if code != 0 {
		t.Errorf("without --strict, warnings alone should exit 0, got %d", code)
	}
	_, code = runCapture(t, "compare", "--strict", a, b)
	if code != 2 {
		t.Errorf("--strict should exit 2 on warnings, got %d", code)
	}
}

func TestIgnoreValuesStillCatchesMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := writeTemp(t, dir, "rules.yaml", "ignore_values:\n  - DATABASE_URL\n")

	// Value drift: forgiven.
	a := writeTemp(t, dir, "a.env", "DATABASE_URL=postgres://dev\n")
	b := writeTemp(t, dir, "b.env", "DATABASE_URL=postgres://prod\n")
	_, code := runCapture(t, "compare", "--config", cfg, a, b)
	if code != 0 {
		t.Errorf("ignore_values should forgive value drift, got exit %d", code)
	}

	// Missing key: still drift.
	c := writeTemp(t, dir, "c.env", "APP=x\n")
	d := writeTemp(t, dir, "d.env", "DATABASE_URL=postgres://prod\nAPP=x\n")
	out, code := runCapture(t, "compare", "--config", cfg, c, d)
	if code != 1 {
		t.Errorf("ignore_values must NOT forgive a missing key, got exit %d\n%s", code, out)
	}
}

func TestSecretValueDetection(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "DB_CONNECTION=postgres://admin:hunter2@db:5432/app\n")
	b := writeTemp(t, dir, "b.env", "DB_CONNECTION=postgres://admin:hunter3@db:5432/app\n")
	out, _ := runCapture(t, "compare", a, b)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "hunter3") {
		t.Errorf("URL-embedded password leaked:\n%s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Errorf("expected redaction:\n%s", out)
	}
}

func TestKeyedListSecretIdentity(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.yaml", "env:\n  - name: DB_PASSWORD\n    value: hunter2\n  - name: OTHER\n    value: x\n")
	b := writeTemp(t, dir, "b.yaml", "env:\n  - name: DB_PASSWORD\n    value: hunter3\n  - name: OTHER\n    value: x\n")
	out, _ := runCapture(t, "compare", a, b)
	if strings.Contains(out, "hunter") {
		t.Errorf("k8s-style env secret leaked via keyed list:\n%s", out)
	}
}

func TestEmptyContainerVisible(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.yaml", "foo: {}\nbar: 1\n")
	b := writeTemp(t, dir, "b.yaml", "bar: 1\n")
	out, code := runCapture(t, "compare", a, b)
	if code != 1 {
		t.Errorf("empty map vs absent must be drift, got exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "foo") {
		t.Errorf("foo should be reported missing:\n%s", out)
	}
}

func TestFailOnFilter(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.yaml", "port: 8080\n")
	b := writeTemp(t, dir, "b.yaml", "port: 9090\n")

	// Value drift present, but we only fail on missing keys.
	_, code := runCapture(t, "compare", "--fail-on", "missing", a, b)
	if code != 0 {
		t.Errorf("--fail-on missing should exit 0 on value drift, got %d", code)
	}
	_, code = runCapture(t, "compare", "--fail-on", "value", a, b)
	if code != 1 {
		t.Errorf("--fail-on value should exit 1, got %d", code)
	}
	_, code = runCapture(t, "compare", "--fail-on", "bogus", a, b)
	if code != 2 {
		t.Errorf("bad --fail-on should exit 2, got %d", code)
	}
}

func TestFormatFlagAndStdinError(t *testing.T) {
	dir := t.TempDir()
	// Extension-less file readable via --format.
	a := writeTemp(t, dir, "noext_a", `{"k": 1}`)
	b := writeTemp(t, dir, "noext_b", `{"k": 2}`)
	_, code := runCapture(t, "compare", "--format", "json", a, b)
	if code != 1 {
		t.Errorf("--format json on extension-less files should work, got exit %d", code)
	}
	// Without --format they fail as before.
	_, code = runCapture(t, "compare", a, b)
	if code != 2 {
		t.Errorf("extension-less without --format should exit 2, got %d", code)
	}
}

func TestIgnoreFilesInCompareDir(t *testing.T) {
	root := t.TempDir()
	writeTemp(t, root, "A/common.yaml", "a: 1\n")
	writeTemp(t, root, "A/patches/dev-only.yaml", "x: 1\n")
	writeTemp(t, root, "B/common.yaml", "a: 1\n")
	cfg := writeTemp(t, root, "rules.yaml", "ignore_files:\n  - patches/*\n")

	out, code := runCapture(t, "compare-dir", "--config", cfg,
		filepath.Join(root, "A"), filepath.Join(root, "B"))
	if code != 0 {
		t.Errorf("ignored one-sided file should not be drift, got exit %d\n%s", code, out)
	}
	if strings.Contains(out, "dev-only") {
		t.Errorf("ignored file should not appear:\n%s", out)
	}
}

func TestJSONSchemaVersionAndWarnings(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "BAD LINE\nK=1\n")
	b := writeTemp(t, dir, "b.env", "K=1\n")
	out, _ := runCapture(t, "compare", "--json", a, b)
	if !strings.Contains(out, `"schema_version": 1`) {
		t.Errorf("missing schema_version:\n%s", out)
	}
	if !strings.Contains(out, `"warnings": 1`) {
		t.Errorf("summary should count warnings:\n%s", out)
	}
}
