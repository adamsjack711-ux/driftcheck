# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.2.2]
### Added
- GitHub Actions CI: lint (golangci-lint), tests, and an install-path smoke
  test on every push and PR.
- Automated releases: pushing a `v*` tag runs GoReleaser (GitHub release with
  linux/darwin amd64+arm64 tarballs) and publishes `driftcheck-cli` to npm.
- Contributor docs: CONTRIBUTING.md, issue templates, Dependabot config.
### Changed
- `driftcheck version` now reports the build-time stamped version (git tag via
  GoReleaser; `dev` for local builds) instead of a hardcoded constant.
- CI and lint fixes for Go 1.26; GitHub Actions bumped to latest majors.

## [0.2.1]
### Added
- VHS-recorded demo GIF in the README (`assets/demo.tape` regenerates
  `assets/demo.gif`).
### Changed
- Example rules file updated to the recommended `ignore_values` form.

## [0.2.0]
### Added
- Keyed-list diffing: lists of maps with a unique `name`/`key`/`id` field
  match by identity (`env[name=LOG_LEVEL].value`) instead of by position, so
  one inserted element registers as one drift instead of misaligning
  everything after it.
- `--strict`: parse warnings (skipped malformed lines) fail the run instead
  of passing silently; warning count added to the JSON summary.
- `ignore_values` rule: forgives expected per-environment value differences
  while still failing if the key goes missing entirely.
- `ignore_files` rule: skip expected per-environment files in `compare-dir`.
- Value-based secret detection (URL-embedded passwords, AWS key IDs, JWTs,
  PEM keys) in addition to name-based detection; new default name patterns
  (`DSN`, `CONNECTION_STRING`, `AUTHORIZATION`, `BEARER`).
- `--fail-on missing,value,type,files` to control which drift categories
  affect the exit code.
- `--format` flag and `-` stdin support.
- `schema_version` and `rules_file` fields in `--json` output.
- npm package ships darwin + linux, arm64 + x64 binaries.
- Fuzz test for the `.env` parser; 10 new end-to-end regression tests.
### Changed
- Empty maps/lists now flatten to `{}` / `[]` leaves instead of vanishing,
  so `foo: {}` vs. no `foo` at all is still visible drift.
- `.driftcheck.yaml` discovery walks up from the working directory;
  `--verbose` reports which rules file was used.
### Renamed
- npm package renamed to `driftcheck-cli` (npm's typosquat guard blocked the
  bare `driftcheck` name). The installed command is unchanged.

## [0.1.0]
### Added
- Initial release: semantic diffing of `.env`, JSON, YAML, and TOML files by
  typed key-path rather than by line.
- Secret redaction by default (name-based).
- `.driftcheck.yaml` ignore rules.
- Human and `--json` output; exit codes `0`/`1`/`2` designed as a CI gate.
- npm binary-shim package.

[Unreleased]: https://github.com/adamsjack711-ux/driftcheck/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/adamsjack711-ux/driftcheck/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/adamsjack711-ux/driftcheck/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/adamsjack711-ux/driftcheck/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/adamsjack711-ux/driftcheck/releases/tag/v0.1.0
