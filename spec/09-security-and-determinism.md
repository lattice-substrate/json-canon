# 09. Security And Determinism

## 9.1 Determinism Requirements

Implementations MUST avoid ambient nondeterminism in canonicalization and verification behavior.

Examples:

- no time-dependent output,
- no random behavior affecting bytes,
- stable object ordering algorithm.

## 9.2 Fail-Closed Posture

On invalid, ambiguous, or non-canonical input, operations MUST fail and MUST NOT silently normalize profile-invalid lexemes.

Specifically in this profile:

- `-0` MUST be rejected at parse time.

## 9.3 Isolation Boundary

This component validates structure and bytes only.

- It MUST NOT infer domain semantics.
- It MUST NOT execute intents or side effects outside canonicalization/verification scope.

## 9.4 Static Tooling Requirement

For infrastructure-grade deployment, release binaries SHOULD be built with:

- `CGO_ENABLED=0`
- `-trimpath -buildvcs=false`
- stripped linker flags with empty build id (`-ldflags="-s -w -buildid="`).

Runtime external dependencies SHOULD be avoided.
