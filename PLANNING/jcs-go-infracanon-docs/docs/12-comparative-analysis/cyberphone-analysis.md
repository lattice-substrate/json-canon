# Analysis: cyberphone/WebPKI Go JCS Canonicalizer vs Strict Audit Requirements

**Status:** Draft  
**Target implementation:** `cyberphone/json-canonicalization` (Go folder: `go/src/webpki.org/jsoncanonicalizer`)

This analysis focuses on **audit-grade** requirements: strict RFC 8259 input acceptance + full I‑JSON enforcement + “no silent normalization”.

## 1. What it does well (alignment with RFC 8785 outputs)
- Implements UTF‑16 key sorting by converting the decoded key string to UTF‑16 code units and using lexicographic comparisons (aligns with RFC 8785 §3.2.3).  
  Source: RFC 8785 sorting definition: https://www.rfc-editor.org/rfc/rfc8785
- Implements a number formatter intended to mimic ECMAScript formatting via `strconv.FormatFloat` with post-processing (and `-0` → `0`).  
  Source: cyberphone `NumberToJSON`: https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/es6numfmt.go

These aspects can be adequate when inputs are already validated and audit strictness is not required.

## 2. Audit-critical gaps: silent normalization risk

### 2.1 Non-JSON numeric syntaxes may be accepted
In `parseSimpleType`, any token not equal to `true/false/null` is treated as a number and parsed with `strconv.ParseFloat(value, 64)`.  
Source: https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/jsoncanonicalizer.go

Go’s `strconv.ParseFloat` “accepts decimal and hexadecimal floating-point numbers as defined by the Go syntax for floating-point literals.”  
Source: https://pkg.go.dev/strconv

RFC 8259 defines JSON numbers in base 10 with an optional leading minus, disallows leading zeros, and states “Infinity and NaN are not permitted.” (RFC 8259 §6)  
Source: https://datatracker.ietf.org/doc/html/rfc8259

**Impact:** a canonicalizer can accept inputs that are not valid JSON (e.g., `0x1p-2`, or `+1` if tokenization allows it) and produce canonical JSON output. That is silent normalization and is unacceptable for audited cryptographic contexts.

### 2.2 UTF‑8 inside strings is not validated
The string parser appends bytes ≥ 0x80 while scanning a quoted string, without validating UTF‑8 structure.  
Source: https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/jsoncanonicalizer.go

But:
- I‑JSON messages MUST be encoded using UTF‑8 and MUST NOT include Surrogates or Noncharacters in names/strings, even when encoded directly in UTF‑8. (RFC 7493 §2.1)  
  Source: https://www.rfc-editor.org/rfc/rfc7493.html
- UTF‑8 decoders MUST protect against invalid sequences; invalid decoding may have security consequences. (RFC 3629 §4, §10)  
  Source: https://www.rfc-editor.org/rfc/rfc3629

**Impact:** invalid UTF‑8 or forbidden code points can slip through to canonical output, undermining cross-implementation consistency and auditability.

## 3. What the new canonicalizer must do differently
### 3.1 Strict lexical number scanning (RFC 8259)
- Implement the RFC 8259 number ABNF in the tokenizer.
- Reject any non-conforming lexical forms before conversion to binary64.

### 3.2 Strict UTF‑8 validation and I‑JSON forbidden code points
- Validate UTF‑8 byte sequences in all decoded string content (including bytes ≥ 0x80 inside strings).
- Reject surrogate code points in UTF‑8 (RFC 3629) and reject Surrogates/Noncharacters per I‑JSON (RFC 7493).

### 3.3 Evidence-first philosophy
- Requirements registry + traceability artifacts.
- Corpus of valid/invalid vectors plus RFC-derived vectors.
- V8 differential number corpus per RFC 8785 recommendation.

## 4. References
- cyberphone repository: https://github.com/cyberphone/json-canonicalization
- cyberphone Go parser source: https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/jsoncanonicalizer.go
- cyberphone number formatter: https://raw.githubusercontent.com/cyberphone/json-canonicalization/master/go/src/webpki.org/jsoncanonicalizer/es6numfmt.go
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 3629: https://www.rfc-editor.org/rfc/rfc3629
- Go strconv ParseFloat docs: https://pkg.go.dev/strconv
