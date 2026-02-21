# ADR-0001: Strict Domain, Fail-Closed Canonicalization

- **ADR ID:** ADR-0001
- **Date:** 2026-02-21
- **Status:** Accepted

## Context
Audit-grade cryptographic workflows require that canonicalization does not silently normalize malformed inputs. RFC 8785 depends on I‑JSON constraints and produces a “hashable” representation for crypto methods. I‑JSON further notes that security protocols may require rejecting or not trusting non-conforming messages.  
Sources:
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 7493 §3: https://www.rfc-editor.org/rfc/rfc7493.html

## Decision
The canonicalizer validates strict RFC 8259 JSON and enforces I‑JSON constraints at the canonicalization boundary. Any violation is rejected with deterministic error codes.

## Rationale
- Prevents silent normalization.
- Aligns with I‑JSON’s “receivers can reject” guidance for security protocols.
- Improves auditability and interoperability.

## Consequences
- Some “JSON-ish” inputs will be rejected.
- Callers must validate/clean data upstream if they want permissive ingestion.

## Alternatives considered
- Lenient parsing mode: rejected for safety; could be a separate package later if needed.
