[Previous: How Canonicalization Works](05-how-canonicalization-works.md) | [Book Home](README.md) | [Next: Offline Replay and Release Gates](07-offline-replay-and-release-gates.md)

# Chapter 7: CLI and ABI Contract

This chapter summarizes the public machine-facing contract.

## Commands

Stable command set:

1. `canonicalize`
2. `verify`

Global flags are also part of the ABI contract (`--help`, `-h`, `--version`).

## Exit Semantics

Stable exit code contract:

1. `0` success
2. `2` input/usage rejection
3. `10` internal error

## Stream Contract

- Canonical output bytes are written to stdout.
- `verify` success token (`ok`) is written to stderr unless `--quiet`.

Automation should treat this as contractual behavior.

## SemVer Rules

1. Patch release: no ABI behavior changes.
2. Minor release: additive, backward-compatible changes only.
3. Major release: required for breaking ABI changes.

## ABI Sources of Truth

Use these files for exact ABI evaluation:

- `ABI.md`
- `abi_manifest.json`
- `FAILURE_TAXONOMY.md`

Any ABI-impacting change requires updates to manifest, tests, and changelog.

[Previous: How Canonicalization Works](05-how-canonicalization-works.md) | [Book Home](README.md) | [Next: Offline Replay and Release Gates](07-offline-replay-and-release-gates.md)
