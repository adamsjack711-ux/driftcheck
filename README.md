# driftcheck

[![CI](https://github.com/adamsjack711-ux/driftcheck/actions/workflows/ci.yml/badge.svg)](https://github.com/adamsjack711-ux/driftcheck/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/driftcheck-cli.svg)](https://www.npmjs.com/package/driftcheck-cli)
[![Go Report Card](https://goreportcard.com/badge/github.com/adamsjack711-ux/driftcheck)](https://goreportcard.com/report/github.com/adamsjack711-ux/driftcheck)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Semantic diffing of structured config files across environments. Compares
`.env`, JSON, YAML, and TOML files by *meaning* — key paths and typed values —
instead of lines, so key reordering and formatting churn are invisible, and a
silently diverged timeout or stale feature flag is not.

![driftcheck demo](assets/demo.gif)

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
npm install -g driftcheck-cli    # prebuilt binaries (macOS + Linux, arm64 + x64), installs `driftcheck`
```

or build from source (single static binary, no runtime dependencies):

```sh
go build -trimpath -ldflags="-s -w" -o driftcheck .
```

## Usage

```sh
driftcheck compare <fileA> <fileB>       # two files, any mix of formats
driftcheck compare-dir <dirA> <dirB>     # recursive, pairs files by relative path
driftcheck compare - prod.yaml --format yaml   # "-" reads stdin
```

Flags: `--json` (CI output), `--verbose` (show identical + ignored keys and
the rules file used), `--show-secrets` (disable redaction), `--strict`
(parse warnings fail the run), `--fail-on missing,value,type,files` (which
drift categories fail the build; default `any`), `--format env|json|yaml|toml`
(force format for stdin or extension-less files), `--config PATH`,
`--no-color`.

**Exit codes** — designed as a CI gate:

| code | meaning |
|------|---------|
| 0    | no unexpected drift |
| 1    | drift found (missing key, value drift, type drift, unmatched file in dir mode) |
| 2    | error (unreadable/unparseable file, bad arguments, bad rules file) |

A parse failure in one file never aborts the run: the error is reported, every
other pair is still compared, and the run exits 2 so CI knows coverage was
incomplete. Malformed lines *within* a .env file are warnings — they don't
fail the run by default (the file may still be mostly comparable), but
`--strict` turns any warning into exit 2 so a truncated file can't slip
through as "no drift".

## How it works

### Normalized tree

Every format parses into the same shape: a flat map of **key path → typed
value** (`internal/model.Tree`). Nested YAML/JSON/TOML maps flatten to
dot-separated paths (`server.timeout`), and literal dots in a key are escaped
(`dotted\.key`). A flat `.env` file is already in this shape. That single
representation is what makes `.env` vs YAML comparison possible.

Lists flatten one of two ways:

- **Keyed**: when every element is a map sharing a unique `name`, `key`, or
  `id` field (the Kubernetes convention — env vars, ports, volumes), elements
  are addressed by identity: `env[name=LOG_LEVEL].value`. Inserting or
  reordering elements doesn't misalign the rest — one added env var is one
  drift, not N.
- **Positional**: everything else (`hosts[0]`, scalar lists) — where order
  may be the semantics, as in command args.

Empty maps and lists become leaves of their own (`{}`, `[]`), so
`foo: {}` vs no `foo` at all is still visible drift.

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

Two detection layers, both on by default:

- **By name**: key names matching built-in patterns (`API_KEY`, `*_TOKEN`,
  `*_SECRET`, `PASSWORD`, `DSN`, `CONNECTION_STRING`, `AUTHORIZATION`, …).
  Keyed-list identities count as names too, so
  `env[name=DB_PASSWORD].value` is caught.
- **By value**: values that look like credentials regardless of key name —
  passwords embedded in connection URLs (`postgres://user:pass@host`), AWS
  access key IDs, JWTs, PEM private keys.

Secrets are compared on their real values but rendered `[redacted]` in both
human and `--json` output. `--show-secrets` opts out; `secret_patterns` adds
name patterns; `no_default_secrets: true` disables the built-ins.

### Rules file

`driftcheck` uses the nearest `.driftcheck.yaml` walking up from the working
directory (or `--config PATH`; explicitly named files must exist). `--verbose`
prints which rules file applied.

```yaml
ignore:              # drift fully expected: value AND presence
  - features.*       # '*' matches any characters, dots included
ignore_values:       # value may differ per environment, but the key
  - DATABASE_URL     #   must still exist on both sides
  - "*_HOST"
ignore_files:        # compare-dir: skip these relative paths entirely
  - patches/*        #   (per-environment kustomize patches, etc.)
secret_patterns:     # extra regexes (case-insensitive) for secret key names
  - internal_cred
no_default_secrets: false
```

Prefer `ignore_values` over `ignore`: it forgives the expected value
difference while still catching the key going missing — which is exactly the
bug class this tool exists for. Ignored drift is still compared and counted,
but doesn't affect the exit code and is only listed with `--verbose`.

### Directory mode

`compare-dir` walks both trees (skipping `.git`, `node_modules`, `vendor`, …),
pairs config files by relative path — `.yml` and `.yaml` are treated as the
same name — and compares each pair. Files present on only one side are
reported and count as drift unless matched by `ignore_files`.

## Extension seam

File loading goes through `parse.Source` (`internal/parse/parse.go`); a future
Vault or AWS Secrets Manager provider implements the same interface — the
comparison engine only ever sees normalized trees.

## Not in v1

Auto-syncing configs, a web UI, and cloud secret-manager backends.
