# driftcheck

> Semantic diffing of structured config files across environments.

Compares `.env`, JSON, YAML, and TOML files by *meaning* — key paths and typed
values — instead of lines. Key reordering and formatting churn are invisible;
a silently diverged timeout, missing key, or stale feature flag is not.

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
is written in Go). Currently
macOS arm64 only; on other platforms build from source and point
`DRIFTCHECK_BIN` at the binary.

## Usage

```sh
driftcheck compare <fileA> <fileB>       # two files, any mix of formats
driftcheck compare-dir <dirA> <dirB>     # recursive, pairs files by relative path
```

Flags: `--json` (CI output), `--verbose`, `--show-secrets`, `--config PATH`,
`--no-color`.

**Exit codes** — designed as a CI gate: `0` no unexpected drift, `1` drift
found, `2` error.

Highlights:

- **Type-aware**: `"8080"` vs `8080` is reported as *type drift*, distinct
  from value drift — usually a quoting/templating bug.
- **Secret redaction by default**: keys matching `API_KEY`, `*_TOKEN`,
  `*_SECRET`, `PASSWORD`, … are compared on real values but printed
  `[redacted]`, in both human and `--json` output.
- **Ignore rules**: a `.driftcheck.yaml` lists keys expected to differ per
  environment (`DATABASE_URL`, `features.*`) so they never page you.

Full documentation: https://github.com/adamsjack711-ux/driftcheck

## License

MIT
