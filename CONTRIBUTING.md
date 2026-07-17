# Contributing to driftcheck

Thanks for considering a contribution. driftcheck is a small, focused tool —
most of the value is in getting the comparison semantics right, so tests
matter more than volume of code.

## Development

```sh
git clone https://github.com/adamsjack711-ux/driftcheck.git
cd driftcheck
go build -o driftcheck .
go test ./...
```

Requires the Go version in `go.mod`.

## Before opening a PR

- `go vet ./...` and `go test ./... -race` should pass.
- If you have [golangci-lint](https://golangci-lint.run) installed, run
  `golangci-lint run` — CI runs the same config (`.golangci.yml`).
- Add or update tests alongside behavior changes. `internal/parse`,
  `internal/model`, `internal/diff`, and `internal/rules` each have their
  own `_test.go`; `fixes_test.go` and `main_test.go` cover end-to-end CLI
  behavior. New format-specific edge cases belong in the parser's own test
  file; new comparison semantics belong in `internal/diff`.
- If you're touching `internal/parse`, consider whether the fuzz test
  (`internal/parse/fuzz_test.go`) needs a new seed corpus entry.

## Scope

Config drift detection across `.env`/JSON/YAML/TOML is the core. A web UI,
auto-syncing configs, and cloud secret-manager backends are explicitly out
of scope for now (see "Not in v1" in the README). Open an issue to discuss
before sending a large PR in that direction, so scope can be aligned first.

## Reporting bugs

Include the input files (redact real secrets), the exact command you ran,
and the actual vs. expected output. A minimal repro in `.env`/YAML/JSON is
enormously helpful — the issue template will prompt for this.
