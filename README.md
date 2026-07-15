# driftcheck

Semantic diffing of structured config files across environments. Compares
`.env`, JSON, YAML, and TOML files by *meaning* — key paths and typed values —
instead of lines, so key reordering and formatting churn are invisible, and a
silently diverged timeout or stale feature flag is not.

```
$ driftcheck compare dev.env prod.env

dev.env <-> prod.env

MISSING IN prod.env (2)
  - DEBUG                 "true"
  - FEATURE_NEW_CHECKOUT  "true"

MISSING IN dev.env (1)
  + RATE_LIMIT            "100"

VALUE DRIFT (2)
  ~ API_KEY               [redacted] -> [redacted]
  ~ CACHE_TTL             "60" -> "300"

  1 drift(s) matched ignore rules (--verbose to list)

  Summary: 5 drift(s) (2 missing in B, 1 missing in A, 2 value), 1 ignored, 1 identical
```

## Install

```sh
npm install -g driftcheck        # prebuilt binary (macOS arm64)
```

or build from source (single static binary, no runtime dependencies):

```sh
go build -trimpath -ldflags="-s -w" -o driftcheck .
```

## Usage

```sh
driftcheck compare <fileA> <fileB>       # two files, any mix of formats
driftcheck compare-dir <dirA> <dirB>     # recursive, pairs files by relative path
```

Flags: `--json` (CI output), `--verbose` (show identical + ignored keys),
`--show-secrets` (disable redaction), `--config PATH` (rules file),
`--no-color`.

**Exit codes** — designed as a CI gate:

| code | meaning |
|------|---------|
| 0    | no unexpected drift |
| 1    | drift found (missing key, value drift, type drift, unmatched file in dir mode) |
| 2    | error (unreadable/unparseable file, bad arguments, bad rules file) |

A parse failure in one file never aborts the run: the error is reported, every
other pair is still compared, and the run exits 2 so CI knows coverage was
incomplete.

## How it works

### Normalized tree

Every format parses into the same shape: a flat map of **key path → typed
value** (`internal/model.Tree`). Nested YAML/JSON/TOML maps flatten to
dot-separated paths (`server.timeout`), list elements to bracketed indices
(`hosts[0]`, `workers[1].name`), and literal dots in a key are escaped
(`dotted\.key`). A flat `.env` file is already in this shape. That single
representation is what makes `.env` vs YAML comparison possible.

Leaf values carry a normalized `Kind` — string, int, float, bool, null — plus
the typed payload:

- JSON decodes with `json.Number` so `8080` stays an int and `80.5` a float.
- `.env` values are always strings (the format has no types; inventing them
  would manufacture fake type drift between two `.env` files).
- YAML timestamps and TOML datetimes normalize to RFC 3339 strings.

### Comparison

For each path in the union of both trees:

| condition | classification |
|-----------|----------------|
| present in A only / B only | `missing_in_b` / `missing_in_a` |
| same kind, same value | `same` (hidden unless `--verbose`) |
| different canonical value | `value_drift` |
| same canonical value, different kind (`"8080"` vs `8080`, `"true"` vs `true`) | `type_drift` |

Type drift is reported separately from value drift because it usually means a
quoting or templating bug, not an intentional config change.

### Secrets

Key names matching built-in patterns (`API_KEY`, `*_TOKEN`, `*_SECRET`,
`PASSWORD`, `PASSWD`, `PWD`, `CREDENTIALS`, `PRIVATE_KEY`, `ACCESS_KEY`, …)
are compared on their real values but rendered as `[redacted]` in both human
and `--json` output. `--show-secrets` opts out; `secret_patterns` in the rules
file adds patterns; `no_default_secrets: true` disables the built-ins.

### Rules file

`driftcheck` reads `.driftcheck.yaml` from the working directory (or
`--config PATH`; explicitly named files must exist):

```yaml
ignore:              # keys expected to differ per environment
  - DATABASE_URL
  - features.*       # '*' matches any characters, dots included
  - "*_HOST"
secret_patterns:     # extra regexes (case-insensitive) for secret key names
  - internal_cred
no_default_secrets: false
```

Ignored keys are still compared and counted, but don't affect the exit code
and are only listed with `--verbose`.

### Directory mode

`compare-dir` walks both trees (skipping `.git`, `node_modules`, `vendor`, …),
pairs config files by relative path — `.yml` and `.yaml` are treated as the
same name — and compares each pair. Files present on only one side are
reported and count as drift.

## Extension seam

File loading goes through `parse.Source` (`internal/parse/parse.go`); a future
Vault or AWS Secrets Manager provider implements the same interface — the
comparison engine only ever sees normalized trees.

## Not in v1

Auto-syncing configs, a web UI, and cloud secret-manager backends.
