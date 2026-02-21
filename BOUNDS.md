# Resource Bounds and Memory Behavior

This document describes the parser's resource limits, memory behavior, and
amplification characteristics for operators deploying `json-canon` in
security-sensitive or resource-constrained environments.

## Default Bounds

| Bound | Default | Constant |
|-------|---------|----------|
| Max nesting depth | 1,000 | `DefaultMaxDepth` |
| Max input size | 64 MiB | `DefaultMaxInputSize` |
| Max JSON values | 1,000,000 | `DefaultMaxValues` |
| Max object members | 250,000 per object | `DefaultMaxObjectMembers` |
| Max array elements | 250,000 per array | `DefaultMaxArrayElements` |
| Max string bytes | 8 MiB (source bytes) | `DefaultMaxStringBytes` |
| Max number chars | 4,096 | `DefaultMaxNumberChars` |

All bounds are configurable via `jcstoken.Options`. The CLI uses defaults.

Bound violations produce `BOUND_EXCEEDED` (exit code 2) with a diagnostic
indicating which bound was exceeded.

## Memory Behavior

### Parse Phase

The parser (`jcstoken.Parse`) builds a complete in-memory tree (`jcstoken.Value`).
Memory consumption is proportional to:

- **Input size**: the parser holds the original input byte slice and builds
  decoded string copies for all string values and object keys.
- **Structure**: each `Value` struct occupies ~112 bytes on 64-bit platforms.
  Each `Member` occupies ~128 bytes (key string header + Value).
- **String decoding**: escape sequences like `\uXXXX` are decoded to UTF-8,
  which may be shorter or longer than the source representation. The worst-case
  amplification is ~1:1 (6-byte `\uXXXX` → 3-byte UTF-8 for BMP, or
  12-byte surrogate pair → 4-byte UTF-8 for supplementary).

### Amplification Bounds

| Input Pattern | Amplification | Notes |
|---------------|--------------|-------|
| Deeply nested arrays `[[[...]]]` | ~112 bytes/level × depth | Bounded by `MaxDepth` (default 1,000) |
| Flat array of small values | ~112 bytes/value | Bounded by `MaxValues` (1M) → ~107 MiB |
| Large object with small keys | ~128 bytes/member + key | Bounded by `MaxObjectMembers` (250K) |
| String with escape sequences | ≤1:1 (shrinks or equal) | Bounded by `MaxStringBytes` (8 MiB) |
| Long number literals | ~48 bytes overhead per number | Bounded by `MaxNumberChars` (4,096) |

**Parse-phase memory only** (input + parsed tree, excluding canonical output):
approximately 2x input size for typical JSON, up to ~200 MiB for adversarial
inputs that maximize value count.

### Serialize Phase

`jcs.Serialize` writes to an in-memory byte buffer. Canonical output is
deterministic, but it is **not always smaller than input bytes**. Number
normalization can expand compact exponent forms (example: `1e20` -> `100000000000000000000`).

In practice, expansion is bounded by ECMA-262 Number::toString output rules for
IEEE 754 binary64 values, but operators should budget for canonical output to
exceed original input size on number-heavy payloads.

### CLI Behavior

The CLI reads the entire input into memory before parsing (`readBounded`).
Peak memory during canonicalization is approximately:

```
input_bytes + parsed_tree + canonical_output
```

For mixed payloads, ~3x input size is typical. For adversarial number-heavy
payloads, provision for higher peaks because canonical output may expand.
With the default 64 MiB input bound, budgeting 256-384 MiB process memory is a
safer operational baseline.

## Recommendations for Constrained Environments

1. **Reduce `MaxInputSize`** to match expected payload size (e.g., 1 MiB for API payloads).
2. **Reduce `MaxValues`** if processing simple structures (e.g., 10,000).
3. **Reduce `MaxDepth`** if nesting beyond a few levels is unexpected (e.g., 32).
4. Use the library API (`jcstoken.ParseWithOptions`) for fine-grained control;
   the CLI uses defaults only.

## Nondeterminism Sources

The parser and serializer contain **no nondeterminism sources**:
- No `math/rand` or `crypto/rand` imports.
- No `time`-dependent behavior.
- No map iteration in output-affecting paths.
- Object key ordering uses deterministic UTF-16 code-unit comparison.

This is enforced by `DET-NOSOURCE-001` in the conformance harness, which performs
AST inspection of all source packages to detect prohibited imports and patterns.
