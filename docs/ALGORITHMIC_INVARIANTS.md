# Algorithmic Invariants

This document captures high-impact correctness invariants for strict JCS
behavior. It does not replace normative registries; it summarizes implementation
contracts auditors and maintainers should preserve.

## Parsing Domain

- Accept only RFC 8259 JSON grammar.
- Reject non-JSON number forms (`+1`, `01`, hex floats, `NaN`, `Infinity`).
- Reject invalid UTF-8 byte sequences.
- Enforce I-JSON constraints:
  - reject duplicate keys after unescape decoding
  - reject lone surrogates
  - reject Unicode noncharacters

## Canonicalization

- Emit no insignificant whitespace.
- Sort object member names by UTF-16 code units (raw/unescaped key values).
- Preserve array order.
- Preserve Unicode string data without normalization.
- Serialize numbers in ECMA-262-compatible form with oracle-backed validation.

## Determinism and Safety

- Canonical output depends only on input bytes.
- Canonicalization is idempotent for valid inputs.
- Core runtime packages prohibit nondeterministic and external side effects:
  - no time/random dependence
  - no runtime outbound network calls
  - no subprocess execution
- Linux release binaries must be fully static.

## Enforcement Anchors

Primary enforcement is executable:

- conformance requirement checks in `conformance/harness_test.go`
- matrix-to-symbol parity checks
- vector schema and execution checks
- V8 oracle corpus checks for number formatting
- static/no-outbound runtime source checks
