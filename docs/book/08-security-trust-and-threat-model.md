[Previous: Offline Replay and Release Gates](07-offline-replay-and-release-gates.md) | [Book Home](README.md) | [Next: Operations and Runbooks](09-operations-runbooks.md)

# Chapter 9: Security, Trust, and Threat Model

Security posture combines runtime constraints and release supply-chain
verification.

## Runtime Security Constraints

Core runtime guarantees include:

1. Linux-only supported runtime,
2. static release binary,
3. no outbound network calls in core runtime,
4. no subprocess execution in core runtime.

These constraints reduce attack surface and nondeterministic behavior channels.

## Release Trust Controls

Release trust is built from:

1. checksums (`SHA256SUMS`),
2. provenance attestation,
3. reproducible build checks,
4. offline replay evidence gate validation.

## Threat Model Boundaries

The project threat model focuses on:

- malformed input handling,
- deterministic behavior integrity,
- release artifact authenticity,
- operational misuse risks in automation environments.

## Verification Practice

Consumers should verify all published artifacts before pinning:

1. checksum validation,
2. attestation validation,
3. optional local reproducible build,
4. offline evidence validation for RC adoption.

## References

- `THREAT_MODEL.md`
- `SECURITY.md`
- `VERIFICATION.md`
- `RELEASE_PROCESS.md`

[Previous: Offline Replay and Release Gates](07-offline-replay-and-release-gates.md) | [Book Home](README.md) | [Next: Operations and Runbooks](09-operations-runbooks.md)
