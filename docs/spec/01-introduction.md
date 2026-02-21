# 01. Introduction

This specification defines `lattice-canon`, a canonicalization and verification primitive for governed JSON artifacts.

`lattice-canon` composes:

1. RFC 8785 JSON Canonicalization Scheme (JCS) serialization behavior.
2. A strict input profile for governed infrastructure bytes.
3. A file envelope format named GJCS1.

The primary invariant is:

- Same semantic JSON value + same profile + same implementation -> byte-identical governed output.

This specification is intended for black-box CLI integration in deterministic infrastructure pipelines.

## Scope

This specification covers:

- accepted and rejected JSON input behavior,
- canonical output requirements,
- GJCS1 file-level constraints,
- CLI command behavior and exit codes,
- conformance requirements.

This specification does not define:

- domain semantics of JSON content,
- intent execution behavior,
- schema definitions for external systems.
