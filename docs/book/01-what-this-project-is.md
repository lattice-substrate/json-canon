[Previous: How To Use This Handbook](00-how-to-use-this-book.md) | [Book Home](README.md) | [Next: What This Project Is Not](02-what-this-project-is-not.md)

# Chapter 2: What This Project Is

`json-canon` is an infrastructure-grade implementation of RFC 8785 JSON
Canonicalization Scheme (JCS).

It is built for machine consumers that need deterministic canonical bytes for
hashing, signatures, or reproducible workflows.

## Core Product

The core product is a Linux CLI binary:

- command: `jcs-canon`
- primary operations: `canonicalize` and `verify`
- stable ABI behavior under strict SemVer

## Design Commitments

1. Deterministic output for identical input and options.
2. Strict JSON acceptance and explicit rejection classes.
3. Stable machine-facing contract (commands, flags, exits, streams).
4. Traceable requirements mapped to implementation and tests.
5. Auditable release process with offline replay evidence gates.

## What "Infrastructure-Grade" Means

"Infrastructure-grade" is not a quality label — it is a set of concrete
engineering constraints that follow from the project's role as a
canonicalization primitive:

1. **Correctness is contractual.** Every normative behavior is traced from an
   RFC clause to a requirement ID to a test. Untested claims are treated as
   unimplemented.
2. **Stability is versioned.** The CLI command surface, exit codes, failure
   classes, and output bytes are governed by strict SemVer. Breaking changes
   require a major version and migration guidance.
3. **Determinism is architectural.** Output is a pure function of input bytes.
   No wall-clock, locale, network, randomness, or map-iteration-order
   dependence exists in the runtime path.
4. **Evidence is reproducible.** Conformance is not just tested in CI — it is
   proven across architectures and kernel versions via offline replay, so
   maintainers and consumers can independently verify behavior stability.

These constraints exist because systems that depend on canonical bytes for
signatures, hashes, and determinism proofs inherit the canonicalizer's
correctness properties. If the canonicalizer is not infrastructure-grade,
the dependent system cannot be either.

## Runtime Envelope

The supported runtime is Linux.

Release artifacts are static binaries built with `CGO_ENABLED=0`.

The core runtime path is intentionally narrow:

- no outbound network calls,
- no subprocess execution,
- no hidden dependence on environment randomness or locale.

## Where Correctness Is Defined

Correctness is defined by the union of:

- RFC and policy requirement registries,
- specification and ABI contracts,
- conformance harness tests,
- offline replay evidence schema and release gates.

[Previous: How To Use This Handbook](00-how-to-use-this-book.md) | [Book Home](README.md) | [Next: What This Project Is Not](02-what-this-project-is-not.md)
