[Previous: What This Project Is Not](02-what-this-project-is-not.md) | [Book Home](README.md) | [Next: Architecture](04-architecture.md)

# Chapter 4: Why This Exists

`json-canon` exists to solve a specific infrastructure problem: producing and
verifying canonical JSON bytes with behavior that remains dependable over many
years.

## Problem Statement

In distributed systems, signing and hashing workflows fail when JSON rendering
is nondeterministic.

Common failure modes in generic stacks include:

1. Key ordering drift,
2. number rendering inconsistencies,
3. parser acceptance differences,
4. unstable error behavior,
5. undocumented ABI changes.

## Project Goals

This project was designed to provide:

1. deterministic canonical output as an explicit contract,
2. strict and testable acceptance/rejection policy,
3. machine-consumable ABI stability,
4. traceability from requirements to executable evidence,
5. release candidate confidence from reproducible and offline proof workflows.

## Why the Offline Harness Matters

Unit and conformance tests prove behavior in controlled environments.

Offline replay extends that proof across distro/kernel lanes and architectures,
so maintainers and external consumers can evaluate real operational stability.

## Why This Matters to Integrators

For teams pinning a release candidate, documentation and evidence must answer:

- What exactly is guaranteed?
- What can break compatibility?
- What proofs exist for this candidate now?
- How do we independently verify trust?

This handbook and the linked contracts are meant to answer those questions
without guesswork.

[Previous: What This Project Is Not](02-what-this-project-is-not.md) | [Book Home](README.md) | [Next: Architecture](04-architecture.md)
