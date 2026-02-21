# Failure Taxonomy

Stable enumeration of failure classes for `json-canon`.
Each class maps to a fixed exit code and is referenced by conformance vectors.

## Failure Classes

| Class | Exit Code | Description |
|-------|-----------|-------------|
| INVALID_UTF8 | 2 | Invalid UTF-8 byte sequences (RFC 3629 §3 violation) |
| INVALID_GRAMMAR | 2 | RFC 8259 JSON grammar violation (leading zeros, trailing commas, bad escapes, etc.) |
| DUPLICATE_KEY | 2 | Duplicate object member name after escape decoding (RFC 7493 §2.3) |
| LONE_SURROGATE | 2 | Lone surrogate code point in string (RFC 7493 §2.1) |
| NONCHARACTER | 2 | Unicode noncharacter in string (RFC 7493 §2.1) |
| NUMBER_OVERFLOW | 2 | Number overflows IEEE 754 binary64 range |
| NUMBER_NEGZERO | 2 | Lexical negative zero token (`-0`, `-0.0`, etc.) |
| NUMBER_UNDERFLOW | 2 | Non-zero number underflows to IEEE 754 zero |
| BOUND_EXCEEDED | 2 | Resource/input policy bound exceeded (depth, size, count, etc.) regardless of stdin/file source |
| NOT_CANONICAL | 2 | Valid JSON but not byte-identical to canonical form |
| CLI_USAGE | 2 | Invalid CLI usage (unknown command/flag, multiple inputs, unreadable file path) |
| INTERNAL_IO | 10 | Output write failure or I/O error |
| INTERNAL_ERROR | 10 | Unexpected internal error |

## Exit Code Summary

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 2 | Input rejection (parse, profile, non-canonical, CLI usage) |
| 10 | Internal error (I/O failure, unexpected state) |

## Offset Semantics

`jcserr.Error.Offset` uses **source-byte positions** in the original input stream.
For escaped string diagnostics, offsets point to the originating escape sequence start (or second escape start for malformed surrogate pairs).

## Mapping to Requirements

| Failure Class | Triggered By Requirements |
|---------------|--------------------------|
| INVALID_UTF8 | PARSE-UTF8-001, PARSE-UTF8-002 |
| INVALID_GRAMMAR | PARSE-GRAM-001 through PARSE-GRAM-010 |
| DUPLICATE_KEY | IJSON-DUP-001, IJSON-DUP-002 |
| LONE_SURROGATE | IJSON-SUR-001, IJSON-SUR-002 |
| NONCHARACTER | IJSON-NONC-001 |
| NUMBER_OVERFLOW | PROF-OFLOW-001 |
| NUMBER_NEGZERO | PROF-NEGZ-001 |
| NUMBER_UNDERFLOW | PROF-UFLOW-001 |
| BOUND_EXCEEDED | BOUND-DEPTH-001 through BOUND-NUMCHARS-001 |
| NOT_CANONICAL | VERIFY-ORDER-001, VERIFY-WS-001 |
| CLI_USAGE | CLI-EXIT-001, CLI-EXIT-002, CLI-FLAG-001, CLI-IO-002 |
| INTERNAL_IO | CLI-EXIT-004 |
| INTERNAL_ERROR | — (defensive) |
