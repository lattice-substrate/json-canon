# Threat Model

## Purpose

This document captures the primary security and reliability threats relevant to
`json-canon` as infrastructure-grade canonicalization tooling.

## Assets to Protect

1. Correct canonical output bytes.
2. Stable and machine-usable ABI behavior.
3. Availability under adversarial inputs.
4. Integrity and authenticity of release artifacts.
5. Auditability of conformance claims.

## Trust Boundaries

1. JSON input bytes from stdin/file are untrusted.
2. Build and release pipeline is trusted only through verifiable provenance.
3. Repository source and CI policy are trusted controls when pinned and reviewed.

## Threat Actors

1. Malicious input producer attempting parser confusion or resource exhaustion.
2. Supply-chain attacker attempting build/release artifact substitution.
3. Accidental maintainer regression introducing nondeterminism or ABI drift.

## Threats and Controls

| Threat | Impact | Primary Controls |
|-------|--------|------------------|
| Invalid/ambiguous encoding input | incorrect parse/canonical result | UTF-8 validation, strict RFC grammar checks |
| Duplicate-key/surrogate/noncharacter payloads | divergent semantics across parsers | I-JSON enforcement, explicit rejection classes |
| Numeric edge-case divergence | cross-runtime signature mismatch | dedicated number formatter + oracle-backed vectors |
| Resource exhaustion inputs | process instability/DoS | explicit bounds on depth/size/count/token length |
| CLI compatibility drift | automation breakage | strict SemVer + ABI manifest + conformance tests |
| Hidden nondeterminism | non-reproducible canonical bytes | source checks + replay/idempotence validation |
| Release tampering | untrusted binaries | checksums, provenance attestation, verification guide |
| CI dependency compromise | malicious workflow behavior | action pinning by commit SHA |

## Out-of-Scope Threats

1. Host-level compromise of maintainer machines.
2. Kernel or hardware-level side-channel attacks.
3. Malicious behavior in external systems that consume canonical output.

These remain operational concerns but are not directly solved by project code.

## Security Invariants

1. Core runtime performs no outbound network calls.
2. Core runtime performs no subprocess execution.
3. Failures classify predictably into stable classes.
4. Canonicalization remains deterministic for identical input/options.

## Residual Risks

1. Single-maintainer concentration risk for long-term governance continuity.
2. Incomplete external interop testing against independent implementations.
3. Human error in policy/document updates despite automated gates.

## Review Cadence

This threat model SHOULD be reviewed at least once per major release and when:

- runtime boundaries change,
- release trust model changes,
- new high-severity security findings are reported.
