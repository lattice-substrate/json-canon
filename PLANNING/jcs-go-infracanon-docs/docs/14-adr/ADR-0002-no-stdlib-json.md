# ADR-0002: Do Not Use `encoding/json` for Canonical Emission

- **ADR ID:** ADR-0002
- **Date:** 2026-02-21
- **Status:** Accepted

## Context
The stdlib `encoding/json` is not an RFC 8785 canonicalizer and coerces invalid UTF‑8 strings by replacing invalid bytes with U+FFFD.  
Source: Go `encoding/json` source: https://go.googlesource.com/go/+/refs/tags/go1.21.5/src/encoding/json/encode.go

RFC 8785 requires that parsed string data MUST NOT be altered in subsequent serializations and that Unicode string data must be preserved “as is.”  
Source: RFC 8785: https://www.rfc-editor.org/rfc/rfc8785

## Decision
Implement a dedicated tokenizer/validator/emitter for JCS; do not use `encoding/json` marshaling for canonical output.

## Rationale
Avoids silent normalization and escaping differences that can corrupt cryptographic invariants.

## Consequences
More implementation work; significantly better audit posture.
