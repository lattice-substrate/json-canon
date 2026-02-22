[Previous: Architecture](04-architecture.md) | [Book Home](README.md) | [Next: CLI and ABI Contract](06-cli-and-abi.md)

# Chapter 6: How Canonicalization Works

This chapter explains the behavior model behind `canonicalize` and `verify`.

## Input Acceptance Model

Input must satisfy:

1. valid UTF-8 and JSON grammar,
2. profile constraints enforced by parser policy,
3. explicit resource limits.

Rejected inputs map to stable failure classes and exit code behavior.

## Canonical Serialization Rules

High-level serialization behavior includes:

1. deterministic object member ordering,
2. canonical string escaping behavior,
3. ECMA-compatible numeric rendering,
4. no whitespace or formatting variability.

## Number Semantics

Numeric rendering is handled by `jcsfloat` to avoid runtime-dependent formatting
drift.

Key expectations:

- shortest round-tripping decimal form,
- deterministic exponent/plain notation boundary behavior,
- stable treatment of edge conditions.

## Verify Semantics

`verify` is strict byte comparison after canonicalization.

Success means canonical output exactly equals input bytes. Equivalent parsed
structure is not enough.

## Failure Classes

Failures are classified by root cause, not by caller or file/stdin source.

See `FAILURE_TAXONOMY.md` for the complete class contract.

## Where to Validate These Rules

- Behavior contract: `SPECIFICATION.md`
- Algorithmic constraints: `docs/ALGORITHMIC_INVARIANTS.md`
- Executable tests: `go test ./...`, `go test ./conformance -v`

[Previous: Architecture](04-architecture.md) | [Book Home](README.md) | [Next: CLI and ABI Contract](06-cli-and-abi.md)
