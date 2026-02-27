[Previous: What This Project Is Not](02-what-this-project-is-not.md) | [Book Home](README.md) | [Next: Architecture](04-architecture.md)

# Chapter 4: Why This Exists

`json-canon` exists to solve a specific infrastructure problem: producing and
verifying canonical JSON bytes with behavior that remains dependable over many
years.

## Problem Statement

In systems where governed artifacts are compared by raw bytes, JSON
canonicalization is not a formatting convenience — it is a correctness
primitive. When the canonical form of a document changes (due to key ordering
drift, number rendering inconsistency, or parser acceptance differences), the
consequences cascade:

1. **Signatures break.** A signature computed over canonical bytes becomes
   invalid if the canonicalizer produces different bytes for the same logical
   data.
2. **Content-addressed lookups fail.** Hashes diverge, and artifacts that
   should be identical produce different digests.
3. **Determinism proofs fail.** Replay-based validation (same input must
   produce byte-identical output) reports false nondeterminism.
4. **Gates fail across dependent systems.** Every system that validates
   canonical form rejects the artifact, even though the logical content has
   not changed.

These are not theoretical risks. Common failure modes in generic JSON stacks
include:

1. Key ordering drift across serializer versions,
2. number rendering inconsistencies (`1.0` vs `1` vs `1e0`),
3. parser acceptance differences (some accept invalid JSON silently),
4. unstable error behavior (error messages and exit codes change between
   releases),
5. undocumented ABI changes (flags, commands, or output contracts shift
   without notice).

Concrete differential examples against Cyberphone Go are tracked in
`docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md`.

## Project Goals

This project was designed to provide:

1. deterministic canonical output as an explicit contract,
2. strict and testable acceptance/rejection policy,
3. machine-consumable ABI stability,
4. traceability from requirements to executable evidence,
5. release candidate confidence from reproducible and offline proof workflows.

## Why the Offline Harness Matters

Unit and conformance tests prove behavior in a single CI environment.

But infrastructure primitives deploy across different Linux distributions,
kernel versions, and CPU architectures. A canonicalizer that produces correct
bytes on Ubuntu x86_64 in CI but different bytes on Alpine arm64 in production
has a latent determinism failure that no amount of CI testing will catch.

Offline replay closes this gap by executing the same vector corpus across a
matrix of real environments — container lanes, VM lanes, multiple
architectures — and comparing output byte-for-byte. The result is executable
evidence that canonical output is stable, not just in the CI lane, but across
the conditions where the binary will actually run.

## Why This Matters to Integrators

For teams pinning a release candidate, documentation and evidence must answer:

- What exactly is guaranteed?
- What can break compatibility?
- What proofs exist for this candidate now?
- How do we independently verify trust?

This handbook and the linked contracts are meant to answer those questions
without guesswork.

[Previous: What This Project Is Not](02-what-this-project-is-not.md) | [Book Home](README.md) | [Next: Architecture](04-architecture.md)
