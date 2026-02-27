# ABI Contract

## Purpose

This document defines the stable CLI ABI contract for `json-canon` in a
human-readable form. The machine-readable source of truth is
`abi_manifest.json`; both documents must remain consistent.

## Why ABI Stability Is Non-Negotiable

When systems depend on a canonicalizer as an infrastructure primitive, they
pin its version and build automation around its command surface, exit codes,
and output byte contract. A silent change to any of these — a renamed flag,
a shifted exit code, a different byte sequence for the same input — breaks
every dependent system simultaneously.

Strict SemVer is not a policy preference. It is the mechanism that prevents
a canonicalizer upgrade from becoming a coordinated emergency across every
system that depends on byte-identical output.

## Stability Policy

The ABI follows strict SemVer.

1. Patch releases MUST NOT break ABI.
2. Minor releases MAY add backward-compatible behavior.
3. Major releases are REQUIRED for breaking ABI changes.

## ABI Surface

The stable ABI includes:

1. executable and command names,
2. command and flag semantics,
3. process exit code behavior,
4. failure class mapping,
5. output stream contract (`stdout` vs `stderr`),
6. machine-observable output grammar.

## Command Contract

### Commands

- `canonicalize`
- `verify`

### Top-Level Flags

- `--help`, `-h` (exit 0)
- `--version` (exit 0; machine-parseable form)

### Command Flags

- `--help`, `-h` (exit 0)
- `--quiet` (for `verify`; suppresses `ok\n` success text; accepted by `canonicalize` for command symmetry and has no success-output effect)

## Input Contract

1. One optional input argument (`file` or `-`) is supported.
2. No file argument or `-` reads stdin.
3. Multiple input files are invalid usage.
4. File and stdin with identical bytes MUST produce identical canonical output.

## Output Stream Contract

1. `canonicalize` success emits canonical bytes to `stdout`; `stderr` is empty.
2. `verify` success emits `ok\n` to `stderr` unless `--quiet`.
3. Help text is user-facing and exits with status `0`.
4. Error diagnostics are emitted to `stderr`.

## Exit Code Contract

Stable process exits:

- `0`: success
- `2`: input rejection or CLI usage violation
- `10`: internal error

Detailed class mapping is defined in `FAILURE_TAXONOMY.md`.

## Compatibility Rules

1. Changing existing command/flag semantics is breaking.
2. Changing exit mapping is breaking.
3. Changing stream placement for existing machine-consumed output is breaking.
4. Changing failure-class semantics is breaking unless major versioned.
5. Adding a new command or flag is non-breaking only if existing behavior is preserved.

## Change Control

Any ABI-impacting change MUST include:

1. update to `abi_manifest.json`,
2. update to this file,
3. ABI-focused tests,
4. release notes in `CHANGELOG.md`.
