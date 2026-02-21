# Conformance Terms, Definitions, and Compliance Levels

**Status:** Draft

## 1. Requirements language (BCP 14)
The key words “MUST”, “MUST NOT”, “REQUIRED”, “SHALL”, “SHALL NOT”, “SHOULD”, “SHOULD NOT”, “RECOMMENDED”, “MAY”, and “OPTIONAL” are to be interpreted as described in RFC 2119 and RFC 8174 (BCP 14).  
Sources:
- RFC 2119: https://datatracker.ietf.org/doc/html/rfc2119
- RFC 8174: https://www.rfc-editor.org/rfc/rfc8174.html

## 2. Terms
- **JSON text**: as defined by RFC 8259.
- **I‑JSON message**: a JSON text that satisfies RFC 7493 constraints.
- **Canonical JSON**: the byte sequence produced by this project’s JCS transform.
- **Fail‑closed**: any input that violates the accepted domain is rejected with a deterministic error; the canonicalizer never “repairs” malformed data.

## 3. Compliance levels
This project defines two compliance levels:

### Level 1 — JCS Transform (strict)
A Level 1 conforming implementation:
- rejects any input that is not valid RFC 8259 JSON,
- enforces all RFC 7493 I‑JSON constraints relevant to JCS processing,
- produces canonical output exactly as RFC 8785 specifies,
- provides stable error codes and traceability.

### Level 2 — JCS Transform (strict + evidence bundle)
In addition to Level 1, Level 2 requires:
- deterministic evidence across a documented environment matrix (distro, arch, kernel),
- stored manifests of expected outputs for a frozen corpus,
- documented procedures to reproduce the evidence.

## 4. Testable compliance claim
A release of the canonicalizer can claim compliance only when:
- every requirement in **docs/10-requirements/requirements.md** is mapped to at least one test and one code anchor, and
- the corpus and determinism checks pass for the declared environment matrix.
