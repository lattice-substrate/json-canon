# Conformance Vector Format

`json-canon` uses JSONL vectors under `conformance/vectors/`.

Each non-empty, non-comment line is one JSON object test case.

## Required Fields

- `id` (string, unique across all vector files)
- `want_exit` (integer)
- one of:
  - `mode` (string)
  - `args` (string array)

## Optional Fields

- `input` (string)
- `want_stdout` (string)
- `want_stderr` (string)
- `want_stderr_contains` (string)

## Semantics

- `id` is the stable test vector identifier.
- `mode` is an abbreviated command selector for harness execution.
- `args` is an explicit CLI argument array (used when `mode` is insufficient).
- `want_exit` is the expected process exit code.
- `want_stdout` and `want_stderr` require exact channel content match.
- `want_stderr_contains` asserts substring containment in stderr.

## Validation

Vector schema and uniqueness are enforced by
`conformance/harness_test.go` (`TestVectorSchemaValid`).
Vector execution is enforced by `TestConformanceVectors`.

## Evolution Policy

- Additive vectors are allowed in minor/patch releases.
- Existing vector IDs must remain stable.
- If behavior changes for an existing vector, treat it as compatibility-impacting
  and update requirements, matrix mappings, and changelog rationale.
