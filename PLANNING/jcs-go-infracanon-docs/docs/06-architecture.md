# Architecture Overview

**Status:** Draft  
**Goal:** dependency-free, deterministic, strict JCS implementation in Go.

## 1. Constraints
- No third-party runtime dependencies.
- Deterministic output across Linux systems and architectures.
- Fail-closed on any RFC 8259 / I‑JSON violation.
- Stable API + stable error codes.

## 2. High-level pipeline
1. **Tokenizer (RFC 8259)**
   - byte-level scanning with explicit token types
   - strict number token recognition (ABNF)

2. **Decoder / Validator**
   - strict UTF‑8 validation (RFC 3629)
   - JSON escape handling; surrogate pair handling
   - I‑JSON forbidden code points checks (RFC 7493)

3. **Canonicalizer**
   - arrays: preserve order, recurse
   - objects: buffer members, detect duplicates, sort keys by UTF‑16 code units (RFC 8785)

4. **Emitter**
   - write canonical JSON with no insignificant whitespace (RFC 8785)
   - UTF‑8 output
   - ECMAScript-compatible number serialization (RFC 8785)

## 3. Data representation
- Strings are validated and represented as sequences of Unicode scalar values, plus an encoded UTF‑8 form for emission.
- Object keys maintain a cached UTF‑16 code unit slice used only for sorting, as mandated by RFC 8785.

## 4. Streaming considerations
JCS requires sorting object properties. Therefore, objects cannot be emitted in a purely streaming fashion without buffering that object’s members. Arrays can be streamed element-by-element, but any nested object still requires buffering at that object boundary.

## 5. Reference: key sorting requirement
RFC 8785 requires sorting property names in their raw (unescaped) form, as arrays of UTF‑16 code units, compared as unsigned values independent of locale.  
Source: https://www.rfc-editor.org/rfc/rfc8785
