# Motivation: Why a New Strict JCS Canonicalizer in Go

**Status:** Draft  
**Problem statement:** Go lacks a dependency-free, audit-grade implementation of JCS (RFC 8785) that is *strict* about the input domain and therefore cannot silently normalize malformed JSON or non–I‑JSON data.

## 1. Why this matters in “byte-identical cryptographic” domains

RFC 8785 frames canonicalization as a prerequisite for cryptographic repeatability: JCS produces a “hashable” representation of JSON data for cryptographic methods.  
> “Cryptographic operations like hashing and signing need the data to be expressed in an invariant format…” (RFC 8785)  
See RFC 8785 Introduction. [1]

For audited systems, the canonicalizer sits on a **trust boundary**:
- **Accept**: only JSON texts that satisfy the agreed domain (RFC 8259 JSON constrained to I‑JSON).
- **Transform**: apply only the canonicalization rules that the standard defines.
- **Reject**: anything outside the domain, rather than “fixing” it.

If a canonicalizer accepts invalid JSON or non–I‑JSON and rewrites it into a canonical-looking output, this is **silent normalization**. Silent normalization destroys auditability because:
- the “signed bytes” may no longer correspond to the sender’s original message,
- different implementations might accept different invalid inputs, and
- systems can be tricked into signing data that downstream validators would have rejected.

## 2. What “true JCS” requires (high level)

RFC 8785 defines canonicalization by:
- building on strict serialization compatible with ECMAScript’s JSON serialization (e.g., JSON.stringify semantics),
- constraining input to I‑JSON (RFC 7493),
- sorting properties deterministically based on UTF‑16 code units,
- emitting UTF‑8 with no insignificant whitespace. [1]

The two requirements that most strongly affect auditability are the **I‑JSON domain constraint** and **UTF‑8 correctness**.

### 2.1 I‑JSON is a domain intended for predictable, rejectable semantics
I‑JSON mandates:
- UTF‑8 encoding (RFC 7493 §2.1),
- no Surrogates or Noncharacters in member names or string values (including directly encoded UTF‑8),
- no duplicate member names (after unescaping), and
- guidance that protocols can require receivers to reject or not trust non-conforming messages (RFC 7493 §3). [2]

### 2.2 RFC 8259 numbers are strict; “JSON-ish floats” are not JSON
RFC 8259 defines JSON numbers in base 10, optional leading minus, with restrictions such as “Leading zeros are not allowed” and “Infinity and NaN are not permitted.” (RFC 8259 §6) [3]

A canonicalizer that accepts other syntaxes (leading “+”, hexadecimal float forms, NaN/Inf spellings) has crossed from “canonicalizing JSON” into “normalizing non-JSON”.

### 2.3 UTF‑8 validation is security-critical
RFC 3629 states UTF‑8 prohibits encoding surrogate code points and requires decoders to protect against invalid sequences; decoding invalid sequences may have security consequences. (RFC 3629 §3, §4, §10) [4]

For canonicalization, invalid UTF‑8 must be rejected, not preserved or repaired.

## 3. Why existing Go approaches are insufficient for audited JCS

### 3.1 `encoding/json` is not a canonicalizer and performs coercions
The Go standard library’s `encoding/json` marshaler historically returns `InvalidUTF8Error`, but since Go 1.2 it instead coerces strings to valid UTF‑8 by replacing invalid bytes with U+FFFD. (Go source commentary in encoding/json) [5]

This behavior is incompatible with the JCS requirement that parsed string data MUST NOT be altered during subsequent serializations and that Unicode string data must be preserved “as is”. (RFC 8785 §3.2 + note on Unicode normalization) [1]

Additionally, `encoding/json` is not designed to:
- sort keys by UTF‑16 code units (RFC 8785 requires this),
- implement ECMAScript-compatible number serialization, or
- enforce I‑JSON constraints (duplicate keys, surrogates/noncharacters, strict UTF‑8). [1][2]

### 3.2 The commonly referenced cyberphone/WebPKI Go implementation is not fail‑closed
The popular repository `cyberphone/json-canonicalization` includes a Go implementation (`webpki.org/jsoncanonicalizer`). It produces plausible outputs for many valid inputs, but its parser choices mean it can accept non‑JSON and non–I‑JSON inputs.

Two audit-relevant examples:

**(A) Number parsing accepts non-JSON forms.**  
The implementation tokenizes a “simple type” and if it is not `true/false/null`, it calls `strconv.ParseFloat(value, 64)`, then serializes the resulting float. [6]  
Go’s `strconv.ParseFloat` explicitly accepts decimal and **hexadecimal floating-point numbers** per Go literal syntax. [7]  
RFC 8259 JSON numbers do not allow hexadecimal float syntax or leading plus signs, and do not permit Infinity or NaN as numeric values. [3]  
This creates a silent-normalization risk: non-JSON inputs may be accepted and rewritten into canonical JSON output.

**(B) UTF‑8 in strings is not validated.**  
The implementation appends bytes ≥ 0x80 while parsing quoted strings without validating UTF‑8 sequences, which conflicts with RFC 7493’s “MUST be UTF‑8” + “MUST NOT include Surrogates or Noncharacters (including directly encoded UTF‑8)” and RFC 3629’s invalid-sequence protections. [2][4][6]

This does not necessarily matter for non-audited use where upstream validation is guaranteed. It is unacceptable when the canonicalizer itself is part of the enforcement boundary.

## 4. What a new “infrastructure-grade” canonicalizer provides

A new implementation designed for auditability provides:

1. **Strict, explicit domain enforcement**
   - Strict RFC 8259 tokenizer (numbers, strings, whitespace, structure).
   - Full I‑JSON enforcement (UTF‑8 only; ban surrogates and noncharacters; reject duplicate keys after unescaping). [2][3][4]

2. **Spec-traceability**
   - Every normative requirement has an ID.
   - Every requirement links to: source clause → code anchor → tests → vectors.
   - Evidence artifacts show byte-identical outputs across environments.

3. **Stable behavior for decades**
   - No reliance on “whatever the stdlib happens to do”.
   - Avoid weak coupling to Go’s float formatting behavior by implementing a controlled, ECMAScript-compatible number serializer and locking it down with RFC 8785 Appendix B + V8 differential tests (recommended in RFC 8785). [1]

4. **Operational clarity**
   - Stable error codes suitable for infrastructure tooling and governance enforcement.
   - Determinism test matrix (distro, libc, arch, kernel) with recorded evidence.

## 5. Non-goals / where strict JCS is not appropriate
Strict JCS is intentionally *not* a lenient parser:
- It is not appropriate for ingesting “JSON-ish” logs or permissive formats (JSON5, comments, trailing commas, NaN/Infinity, etc.).
- It is not a solution for exact exchange of large integers beyond IEEE-754 binary64 safe integer range; I‑JSON recommends encoding such values as strings. (RFC 7493 §2.2) [2]

## References
[1] RFC 8785 (JSON Canonicalization Scheme). https://www.rfc-editor.org/rfc/rfc8785  
[2] RFC 7493 (I‑JSON). https://www.rfc-editor.org/rfc/rfc7493.html  
[3] RFC 8259 (JSON). https://datatracker.ietf.org/doc/html/rfc8259  
[4] RFC 3629 (UTF‑8). https://www.rfc-editor.org/rfc/rfc3629  
[5] Go `encoding/json` source (Marshal coerces invalid UTF‑8). https://go.googlesource.com/go/+/refs/tags/go1.21.5/src/encoding/json/encode.go  
[6] cyberphone Go canonicalizer source (`ParseFloat` usage; string parsing). https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/jsoncanonicalizer.go  
[7] Go `strconv` docs (`ParseFloat` accepts decimal and hexadecimal floats). https://pkg.go.dev/strconv
