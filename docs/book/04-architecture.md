[Previous: Why This Exists](03-why-this-exists.md) | [Book Home](README.md) | [Next: How Canonicalization Works](05-how-canonicalization-works.md)

# Chapter 5: Architecture

This chapter gives a practical architectural map. For the full contract, see
`ARCHITECTURE.md`.

## Layered Packages

The system is intentionally layered with one-way dependencies:

1. `jcserr` - stable failure taxonomy and exit mapping.
2. `jcsfloat` - deterministic Number::toString behavior.
3. `jcstoken` - strict parsing and profile enforcement.
4. `jcs` - canonical serialization.
5. `cmd/jcs-canon` - CLI boundary and process behavior.

Offline proof components in `offline/` are separate operational tooling.

## Data Flow

Canonicalize flow:

1. Read bytes from stdin/file.
2. Parse and validate against strict grammar/policy.
3. Build internal value representation.
4. Serialize to canonical RFC 8785 output.
5. Emit bytes to stdout.

Verify flow:

1. Canonicalize in-memory.
2. Compare canonical bytes to original input bytes.
3. Return success only on byte-identical equality.

## Determinism Controls

Determinism is enforced by design:

- UTF-16 code-unit key sorting,
- stable number formatting rules,
- explicit resource bounds,
- no runtime network/process side effects,
- no dependence on map iteration order or clock.

## Architecture Boundaries

Stable external boundary:

- CLI command surface,
- flags and semantics,
- stream contract,
- exit/failure classes,
- canonical output bytes.

Internal refactors are acceptable only when these external contracts remain
unchanged or are versioned deliberately.

[Previous: Why This Exists](03-why-this-exists.md) | [Book Home](README.md) | [Next: How Canonicalization Works](05-how-canonicalization-works.md)
