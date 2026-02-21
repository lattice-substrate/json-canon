# 01. Introduction

This specification defines `jcs-canon`, a canonicalization and verification primitive for strict JSON inputs.

`jcs-canon` composes:

1. RFC 8785 JSON Canonicalization Scheme (JCS) serialization behavior.
2. A strict input profile for deterministic and fail-closed infrastructure use.

Primary invariant:

- Same accepted JSON value + same implementation version -> byte-identical canonical output.

This specification is intended for black-box CLI integration in deterministic infrastructure pipelines.

## Scope

This specification covers:

- accepted and rejected JSON input behavior,
- canonical output requirements,
- CLI command behavior and exit codes,
- conformance requirements.

This specification does not define:

- domain semantics of JSON content,
- schema validation for user payloads,
- side effects outside parse/canonicalize/verify operations.
