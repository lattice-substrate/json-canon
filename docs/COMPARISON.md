# Comparison and Evaluation Guide

This document helps teams evaluate `json-canon` against alternative JCS
implementations and decide whether it fits their use case.

## What Makes json-canon Different

1. **Strict parsing** — rejects invalid JSON that other implementations silently
   accept (hex floats, leading zeros, plus-prefixed numbers, lone surrogates,
   invalid UTF-8). See [differential strictness](#differential-strictness) below.

2. **Classified errors** — every rejection maps to a stable failure class with a
   documented exit code. Automation can branch on exit codes without parsing
   stderr. See [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md).

3. **ABI versioning** — the CLI command set, flags, exit codes, and stream
   contracts are governed by strict SemVer. Breaking changes require a major
   version bump. See [ABI.md](../ABI.md).

4. **Offline evidence** — release candidates include cross-architecture replay
   evidence that proves behavior outside CI. See
   [docs/OFFLINE_REPLAY_HARNESS.md](OFFLINE_REPLAY_HARNESS.md).

5. **Zero-dependency core** — the canonicalization engine uses only the Go
   standard library. No transitive supply-chain risk in the runtime path.

## Differential Strictness

These cases demonstrate where `json-canon` rejects inputs that Cyberphone Go
silently accepts and canonicalizes. All cases are encoded as executable tests in
`conformance/cyberphone_differential_test.go` with recorded differential outputs.

| Case | Input | Cyberphone Go | json-canon |
|------|-------|---------------|------------|
| Hex float literal | `{"n":0x1p-2}` | `{"n":0.25}` | reject (`INVALID_GRAMMAR`) |
| Plus-prefixed number | `{"n":+1}` | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| Leading-zero number | `{"n":01}` | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| Invalid UTF-8 byte | `{"s":"<0xFF>"}` | `{"s":"<0xFF>"}` | reject (`INVALID_UTF8`) |
| Invalid surrogate pair | `{"s":"\uD800\u0041"}` | `{"s":"\uFFFD"}` | reject (`LONE_SURROGATE`) |

Full details: [CYBERPHONE_DIFFERENTIAL_EXAMPLES.md](CYBERPHONE_DIFFERENTIAL_EXAMPLES.md).

## Differential Gate

Run the recorded differential strictness gate:

```bash
go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1 -v
```

Expected behavior: `json-canon` rejects all listed malformed inputs with stable
failure classes (`INVALID_GRAMMAR`, `INVALID_UTF8`, `LONE_SURROGATE`).

## When json-canon Is the Right Choice

- **Signing and verification** — you need byte-deterministic canonical form
  before computing signatures or MACs.
- **Content-addressable storage** — you hash JSON documents and need identical
  content to produce identical hashes regardless of formatting.
- **Audit trails** — you need to prove that a document has not been modified,
  and the canonicalization step must be reproducible and traceable.
- **Pipeline validation** — you need strict input gates that reject malformed
  JSON before it enters downstream processing, with stable exit codes for
  automation.

## When json-canon May Be Overkill

- **Pretty-printing or reformatting** — `json-canon` produces minimal canonical
  output only. Use `jq` or similar tools for human-readable formatting.
- **macOS or Windows runtime** — supported runtime is Linux only.
- **General JSON processing** — `json-canon` is not a query engine, schema
  validator, or transformation toolkit. Use `encoding/json`, `jq`, or
  purpose-built libraries for those needs.
- **Lenient parsing** — if you need to accept and repair malformed input,
  `json-canon` will reject it by design.

## Feature Matrix

| Capability | json-canon | Cyberphone Go | encoding/json |
|------------|-----------|---------------|---------------|
| RFC 8785 canonical output | Yes | Yes | No |
| ECMA-262 Number::toString | Yes (hand-written Burger-Dybvig) | Yes (strconv-based) | No |
| UTF-16 code-unit key sort | Yes | Yes | No (byte order) |
| Strict RFC 8259 grammar | Yes | Partial (accepts some invalid forms) | Partial |
| I-JSON constraints (RFC 7493) | Yes (duplicates, surrogates, noncharacters) | No | No |
| Classified error taxonomy | Yes (12 stable classes) | No | No |
| Stable CLI ABI (SemVer) | Yes | N/A (library only) | N/A |
| Configurable resource bounds | Yes | No | No |
| Offline replay evidence | Yes | No | No |
| External dependencies | None (stdlib only) | None | N/A (stdlib) |
| Platform support | Linux | Cross-platform | Cross-platform |

## Further Reading

- [QUICKSTART.md](QUICKSTART.md) — get started in 5 minutes
- [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md) — real-world deployment patterns
- [CYBERPHONE_DIFFERENTIAL_EXAMPLES.md](CYBERPHONE_DIFFERENTIAL_EXAMPLES.md) — full differential test documentation
- [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md) — complete failure class reference
