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
