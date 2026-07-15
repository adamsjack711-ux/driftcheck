package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCapture runs the CLI in-process, capturing stdout and the exit code.
func runCapture(t *testing.T, args ...string) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := run(args)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String(), code
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompareIdenticalExitsZero(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.json", `{"port": 8080}`)
	b := writeTemp(t, dir, "b.json", `{"port": 8080}`)
	out, code := runCapture(t, "compare", a, b)
	if code != 0 {
		t.Errorf("exit = %d, want 0\n%s", code, out)
	}
}

func TestCompareDriftExitsOne(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.json", `{"port": 8080, "extra": 1}`)
	b := writeTemp(t, dir, "b.json", `{"port": 9090}`)
	out, code := runCapture(t, "compare", a, b)
	if code != 1 {
		t.Errorf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "MISSING IN") || !strings.Contains(out, "VALUE DRIFT") {
		t.Errorf("report missing sections:\n%s", out)
	}
}

func TestCompareParseErrorExitsTwo(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.json", `{broken`)
	b := writeTemp(t, dir, "b.json", `{"port": 1}`)
	out, code := runCapture(t, "compare", a, b)
	if code != 2 {
		t.Errorf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "ERROR") {
		t.Errorf("report should surface the parse error:\n%s", out)
	}
}

func TestCompareMissingFileExitsTwo(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.json", `{}`)
	_, code := runCapture(t, "compare", a, filepath.Join(dir, "nope.json"))
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCrossFormatTypeDrift(t *testing.T) {
	dir := t.TempDir()
	env := writeTemp(t, dir, "app.env", "PORT=8080\n")
	yaml := writeTemp(t, dir, "app.yaml", "PORT: 8080\n")
	out, code := runCapture(t, "compare", env, yaml)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (type drift is drift)\n%s", code, out)
	}
	if !strings.Contains(out, "TYPE DRIFT") {
		t.Errorf("expected TYPE DRIFT section:\n%s", out)
	}
	if !strings.Contains(out, `"8080" (string) -> 8080 (int)`) {
		t.Errorf("expected typed rendering:\n%s", out)
	}
}

func TestSecretsRedactedByDefault(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "API_KEY=supersecret-a\n")
	b := writeTemp(t, dir, "b.env", "API_KEY=supersecret-b\n")

	out, _ := runCapture(t, "compare", a, b)
	if strings.Contains(out, "supersecret") {
		t.Errorf("secret leaked in default output:\n%s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Errorf("expected [redacted]:\n%s", out)
	}

	out, _ = runCapture(t, "compare", "--show-secrets", a, b)
	if !strings.Contains(out, "supersecret-a") {
		t.Errorf("--show-secrets should reveal values:\n%s", out)
	}

	// JSON mode must redact too — CI logs are the most common leak vector.
	out, _ = runCapture(t, "compare", "--json", a, b)
	if strings.Contains(out, "supersecret") {
		t.Errorf("secret leaked in JSON output:\n%s", out)
	}
}

func TestIgnoreRulesSuppressDriftAndExitCode(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "DATABASE_URL=postgres://dev\nAPP=x\n")
	b := writeTemp(t, dir, "b.env", "DATABASE_URL=postgres://prod\nAPP=x\n")
	cfg := writeTemp(t, dir, "rules.yaml", "ignore:\n  - DATABASE_URL\n")

	out, code := runCapture(t, "compare", "--config", cfg, a, b)
	if code != 0 {
		t.Errorf("exit = %d, want 0 (only ignored drift)\n%s", code, out)
	}
	if !strings.Contains(out, "matched ignore rules") {
		t.Errorf("expected ignored-drift note:\n%s", out)
	}
}

func TestExplicitMissingConfigExitsTwo(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.json", `{}`)
	b := writeTemp(t, dir, "b.json", `{}`)
	_, code := runCapture(t, "compare", "--config", filepath.Join(dir, "absent.yaml"), a, b)
	if code != 2 {
		t.Errorf("exit = %d, want 2 for explicitly named missing config", code)
	}
}

func TestJSONOutputSchema(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.yaml", "port: 8080\nname: dev\n")
	b := writeTemp(t, dir, "b.yaml", "port: 9090\nname: dev\n")

	out, code := runCapture(t, "compare", "--json", a, b)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	var rep struct {
		Pairs []struct {
			Drifts []struct {
				Path string `json:"path"`
				Type string `json:"type"`
				A    struct {
					Type  string `json:"type"`
					Value any    `json:"value"`
				} `json:"a"`
			} `json:"drifts"`
		} `json:"pairs"`
		Summary struct {
			Drift  int `json:"drift"`
			Errors int `json:"errors"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if rep.Summary.Drift != 1 || len(rep.Pairs) != 1 || len(rep.Pairs[0].Drifts) != 1 {
		t.Errorf("unexpected report shape:\n%s", out)
	}
	d := rep.Pairs[0].Drifts[0]
	if d.Path != "port" || d.Type != "value_drift" || d.A.Type != "int" {
		t.Errorf("drift = %+v, want port/value_drift/int", d)
	}
}

func TestCompareDir(t *testing.T) {
	root := t.TempDir()
	// dirA: base.yaml + only-in-a.env + sub/app.toml
	writeTemp(t, root, "A/base.yaml", "replicas: 2\nimage: app:v1\n")
	writeTemp(t, root, "A/only-in-a.env", "X=1\n")
	writeTemp(t, root, "A/sub/app.toml", "[server]\nport = 8080\n")
	writeTemp(t, root, "A/README.md", "not a config\n")
	// dirB: base.yml (yml/yaml matched), sub/app.toml with drift, extra file
	writeTemp(t, root, "B/base.yml", "replicas: 5\nimage: app:v1\n")
	writeTemp(t, root, "B/sub/app.toml", "[server]\nport = 8443\n")
	writeTemp(t, root, "B/only-in-b.json", `{"y": 2}`)

	out, code := runCapture(t, "compare-dir", filepath.Join(root, "A"), filepath.Join(root, "B"))
	if code != 1 {
		t.Errorf("exit = %d, want 1\n%s", code, out)
	}
	for _, want := range []string{
		"only-in-a.env",
		"only-in-b.json",
		"replicas",
		"port",
		"CONFIG FILES ONLY IN",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "README.md") {
		t.Errorf("non-config files should not appear:\n%s", out)
	}
}

func TestCompareDirPartialParseFailure(t *testing.T) {
	root := t.TempDir()
	writeTemp(t, root, "A/good.yaml", "a: 1\n")
	writeTemp(t, root, "A/bad.json", "{broken")
	writeTemp(t, root, "B/good.yaml", "a: 1\n")
	writeTemp(t, root, "B/bad.json", "{broken")

	out, code := runCapture(t, "compare-dir", filepath.Join(root, "A"), filepath.Join(root, "B"))
	if code != 2 {
		t.Errorf("exit = %d, want 2 (parse errors present)\n%s", code, out)
	}
	// The good pair must still have been compared.
	if !strings.Contains(out, "good.yaml") {
		t.Errorf("good pair should still be reported:\n%s", out)
	}
	if !strings.Contains(out, "ERROR") {
		t.Errorf("errors should be surfaced:\n%s", out)
	}
}

func TestVerboseShowsSame(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.env", "SAME=1\nDIFF=a\n")
	b := writeTemp(t, dir, "b.env", "SAME=1\nDIFF=b\n")

	out, _ := runCapture(t, "compare", a, b)
	if strings.Contains(out, "SAME (") {
		t.Errorf("same-value section should be hidden by default:\n%s", out)
	}
	out, _ = runCapture(t, "compare", "--verbose", a, b)
	if !strings.Contains(out, "SAME (1)") {
		t.Errorf("--verbose should show same-value section:\n%s", out)
	}
}

func TestUnknownCommandExitsTwo(t *testing.T) {
	if _, code := runCapture(t, "bogus"); code != 2 {
		t.Errorf("unknown command exit = %d, want 2", code)
	}
	if _, code := runCapture(t); code != 2 {
		t.Errorf("no args exit = %d, want 2", code)
	}
}
