# Implementation Checklist

**Status:** Draft

Use this checklist to build the implementation in a provable order.

## Phase 1: Strict parsing foundation
- [ ] RFC 8259 tokenization (structure, whitespace)
- [ ] Strict RFC 8259 number lexing (reject +, leading zeros, non-decimal)
- [ ] Strict string scanning (reject raw control chars)
- [ ] Escape decoding (`\uXXXX`, standard escapes)

## Phase 2: I‑JSON enforcement
- [ ] UTF‑8 validation of all decoded string bytes (RFC 3629)
- [ ] Reject surrogate code points in UTF‑8 (RFC 3629)
- [ ] Reject Surrogates/Noncharacters in names/strings (RFC 7493)
- [ ] Duplicate key rejection after unescaping (RFC 7493)

## Phase 3: Canonicalization
- [ ] UTF‑16 key sorting (RFC 8785)
- [ ] Whitespace-free emission (RFC 8785)
- [ ] UTF‑8 output emission (RFC 8785)
- [ ] Canonical string escaping (RFC 8259)
- [ ] Number serialization (RFC 8785: Appendix B + V8 differential)

## Phase 4: Evidence
- [ ] Requirement IDs mapped to tests
- [ ] Corpus harness (valid + invalid)
- [ ] Determinism matrix runs recorded

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 3629: https://www.rfc-editor.org/rfc/rfc3629
