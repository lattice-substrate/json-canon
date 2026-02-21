# Integration Guide for Infrastructure Tooling

**Status:** Draft  
**Audience:** tool authors integrating canonicalization into signing/hashing flows.

## 1. Recommended usage pattern
1. Receive raw JSON bytes.
2. Canonicalize with `Transform`.
3. Hash/sign the canonical output bytes.
4. Store (a) original bytes, (b) canonical bytes, (c) digest/signature, (d) tool version for audit trails.

This aligns with RFC 8785’s purpose of producing a “hashable” representation usable by cryptographic methods.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 2. Don’t silently accept errors
Treat any canonicalization error as a protocol violation in audited security contexts. I‑JSON explicitly allows protocols to require receivers to reject or not trust non-conforming messages.  
Source: RFC 7493 §3. https://www.rfc-editor.org/rfc/rfc7493.html

## 3. Trailing newlines
Canonical output is defined as bytes; adding a trailing newline changes the cryptographic input. If a CLI tool prints a newline for UX, it MUST provide a “raw bytes” mode for cryptographic use.

## 4. Version pinning
Pin the canonicalizer version in:
- build system / lock files
- artifacts metadata
- provenance attestations

## 5. When not to use this tool
- As a lenient parser for untrusted “JSON-ish” text.
- For transmitting exact large integers or decimals as JSON numbers; prefer strings per I‑JSON guidance.  
  Source: RFC 7493 §2.2. https://www.rfc-editor.org/rfc/rfc7493.html
