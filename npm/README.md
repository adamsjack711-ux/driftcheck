# driftcheck

> Semantic diffing of structured config files across environments.

Compares `.env`, JSON, YAML, and TOML files by *meaning* — key paths and typed
values — instead of lines. Key reordering and formatting churn are invisible;
a silently diverged timeout, missing key, or stale feature flag is not.

![driftcheck demo](https://raw.githubusercontent.com/adamsjack711-ux/driftcheck/main/assets/demo.gif)

```
$ driftcheck compare dev.env prod.env

dev.env <-> prod.env

MISSING IN prod.env (2)
  - DEBUG                 "true"
  - FEATURE_NEW_CHECKOUT  "true"

VALUE DRIFT (2)
  ~ API_KEY               [redacted] -> [redacted]
  ~ CACHE_TTL             "60" -> "300"

TYPE DRIFT (1)
  ! server.port           "8080" (string) -> 8080 (int)
```

## Install

```sh
npm install -g driftcheck-cli
```

This installs the `driftcheck` command as a prebuilt native binary (the CLI
is written in Go). Ships macOS and Linux, arm64 and x64 — including the
GitHub Actions / GitLab CI runners this tool is designed to gate. Elsewhere,
build from source and point `DRIFTCHECK_BIN` at the binary.

## Usage

```sh
driftcheck compare <fileA> <fileB>       # two files, any mix of formats
driftcheck compare-dir <dirA> <dirB>     # recursive, pairs files by relative path
```

Flags: `--json` (CI output), `--verbose`, `--show-secrets`, `--strict`
(parse warnings fail the run), `--fail-on missing,value,type,files`,
`--format env|json|yaml|toml` (stdin / extension-less files),
`--config PATH`, `--no-color`.

**Exit codes** — designed as a CI gate: `0` no unexpected drift, `1` drift
found, `2` error.

Highlights:

- **Type-aware**: `"8080"` vs `8080` is reported as *type drift*, distinct
  from value drift — usually a quoting/templating bug.
- **Secret redaction by default**: keys matching `API_KEY`, `*_TOKEN`,
  `*_SECRET`, `PASSWORD`, … are compared on real values but printed
  `[redacted]`, in both human and `--json` output.
- **Keyed-list matching**: Kubernetes-style lists (`env:`, `ports:`) are
  matched by their `name`/`key`/`id` field — inserting one element is one
  drift, not a misaligned cascade.
- **Ignore rules**: `.driftcheck.yaml` `ignore_values:` forgives per-
  environment value differences (`DATABASE_URL`) while still failing if the
  key goes missing; `ignore_files:` silences expected per-env overlay files.

Full documentation: https://github.com/adamsjack711-ux/driftcheck

## License

MIT
