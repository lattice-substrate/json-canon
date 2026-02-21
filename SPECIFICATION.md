# Specification

## Status

This document defines the product-level normative behavior contract for
`json-canon`.

Normative requirement IDs remain the executable source of truth in:

- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`

This specification describes required behavior in cohesive form and must remain
consistent with those registries.

## RFC 2119 Language

The keywords `MUST`, `MUST NOT`, `SHALL`, `SHALL NOT`, `SHOULD`, `SHOULD NOT`,
`MAY`, and `OPTIONAL` are normative.

## Normative References

- RFC 8785 (JCS)
- RFC 8259 (JSON)
- RFC 7493 (I-JSON)
- RFC 3629 (UTF-8)
- ECMA-262 `Number::toString`
- IEEE 754 binary64 semantics

Clause-level mappings are recorded in `standards/CITATION_INDEX.md`.

## Conformance Targets

A conforming implementation of this project MUST:

1. accept and reject JSON according to RFC and profile constraints,
2. emit canonical bytes according to RFC 8785,
3. provide stable CLI ABI behavior,
4. satisfy all requirement IDs and conformance tests.

## Input Domain

### UTF-8 and Grammar

1. Input bytes MUST be valid UTF-8.
2. JSON grammar MUST conform to RFC 8259.
3. Invalid escapes, malformed numbers, trailing content, and illegal grammar forms MUST be rejected.

### I-JSON Constraints

1. Duplicate object keys after unescape decoding MUST be rejected.
2. Lone surrogates MUST be rejected.
3. Unicode noncharacters MUST be rejected.

### Project Number Profile

In addition to RFC 8259 grammar, project policy requires:

1. Lexical negative zero tokens (for example `-0`, `-0.0`, `-0e0`) MUST be rejected.
2. Numeric overflow to infinity MUST be rejected.
3. Non-zero numeric underflow to zero MUST be rejected.

## Canonicalization Requirements

### Structural Form

1. Canonical output MUST contain no insignificant whitespace.
2. Array order MUST be preserved.
3. Object members MUST be sorted by UTF-16 code-unit order of raw property names.

### String Serialization

1. Control characters MUST be escaped per RFC 8785 rules.
2. Required short escapes (`\b`, `\t`, `\n`, `\f`, `\r`) MUST be used where mandated.
3. Other control escapes MUST use lowercase hex `\u00xx` form.
4. Solidus (`/`) MUST NOT be escaped in canonical output.
5. No Unicode normalization may be applied.

### Number Serialization

1. Numeric serialization MUST follow ECMA-262 `Number::toString` behavior for binary64 values.
2. Exponential output MUST use lowercase `e` and explicit sign for positive exponent.
3. Boundary branch behavior around `1e-6` and `1e21` MUST match the ECMA algorithm.
4. Output MUST round-trip to the same IEEE 754 binary64 value.

### Encoding

1. Canonical output MUST be UTF-8.
2. BOM MUST NOT be emitted.

## Verification Semantics

`verify` mode MUST:

1. parse and canonicalize input under the same strict domain,
2. compare canonical bytes against original bytes,
3. return success only if bytes are identical.

Non-identical but valid JSON MUST be classified as `NOT_CANONICAL`.

## CLI Contract

The CLI command set includes:

- `jcs-canon canonicalize [--quiet] [file|-]`
- `jcs-canon verify [--quiet] [file|-]`
- `jcs-canon --help`
- `jcs-canon --version`

Required CLI behavior:

1. Top-level and command-level `--help`/`-h` MUST exit `0`.
2. Unknown commands/flags and invalid usage MUST exit `2`.
3. Internal runtime faults MUST exit `10`.
4. `canonicalize` success output MUST go to `stdout` only.
5. `verify` success text (`ok\n`) MUST go to `stderr` unless `--quiet`.
6. File and stdin inputs with identical content MUST produce identical behavior.

## Failure and Exit Code Contract

Stable class and exit mappings are defined in `FAILURE_TAXONOMY.md`.

Required properties:

1. Class assignment is based on root cause.
2. Equivalent failures across input modes classify identically.
3. Class-to-exit mapping is stable across minor/patch releases.

## Determinism and Side-Effect Contract

1. For fixed input and options, output bytes MUST be identical across runs.
2. Runtime canonicalization logic MUST NOT depend on wall-clock time, randomness, network state, subprocess output, or locale.
3. Core runtime packages MUST NOT perform outbound network calls or subprocess execution.

## Resource Bounds

The implementation MUST enforce explicit bounds for depth, input size, values,
object members, array elements, string bytes, and number-token length.

Default values and operational guidance are defined in `BOUNDS.md`.

## Compatibility Policy

1. Stable CLI ABI follows strict SemVer.
2. Breaking changes to ABI surface REQUIRE a major version.
3. Minor/patch releases MUST preserve existing ABI behavior.

ABI source of truth is `abi_manifest.json`.

## Traceability and Enforcement

All normative and policy requirements MUST be:

1. identified by stable requirement IDs,
2. mapped to implementation symbols,
3. mapped to executable tests,
4. validated by conformance gates.

Required checks and parity rules are described in `docs/TRACEABILITY_MODEL.md`.
