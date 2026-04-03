# Conformance Vector Format

`json-canon` uses JSONL vectors under `conformance/vectors/`.

Each non-empty, non-comment line is one JSON object test case.

## Required Fields

- `id` (string, unique across all vector files)
- `intent` (string: `positive`, `negative`, or `adversarial`)
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
- `intent` classifies the vector as a success-path proof (`positive`),
  a fail-closed regression guard (`negative`), or an adversarial fail-closed
  guard (`adversarial`).
- `mode` is an abbreviated command selector for harness execution.
- `args` is an explicit CLI argument array (used when `mode` is insufficient).
- `want_exit` is the expected process exit code.
- `want_stdout` and `want_stderr` require exact channel content match.
- `want_stderr_contains` asserts substring containment in stderr.
- `positive` vectors must assert exact success output on stdout or stderr and
  must exit `0`.
- `negative` vectors must fail closed (`want_exit != 0`) and must assert stderr
  diagnostic evidence.
- `adversarial` vectors must fail closed (`want_exit != 0`) and must assert
  root-cause or byte-offset stderr diagnostics.
- Every CLI command represented in the vector corpus must carry at least one
  `positive`, one `negative`, and one `adversarial` vector.

## Validation

Vector schema and uniqueness are enforced by
`conformance/harness_test.go` (`TestVectorSchemaValid`).
Vector execution is enforced by `TestConformanceVectors`.
Triad coverage and per-intent semantics are enforced by
`TestConformanceRequirements` (`CONF-VEC-001..008`).

## Evolution Policy

- Additive vectors are allowed in minor/patch releases.
- Existing vector IDs must remain stable.
- If behavior changes for an existing vector, treat it as compatibility-impacting
  and update requirements, matrix mappings, and changelog rationale.
