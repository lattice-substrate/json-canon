# 02. Conventions And Terminology

## Requirement Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this specification are to be interpreted as described in RFC 2119 and RFC 8174.

## Core Terms

- `JSON value`: A value as defined by RFC 8259.
- `strict profile`: Additional validity constraints beyond base RFC 8259 syntax.
- `canonical bytes`: Deterministic JCS serialization bytes for a parsed JSON value.
- `GJCS1`: `JCS(value) || LF` where LF is one trailing byte `0x0A`.
- `governed file`: A byte sequence that passes GJCS1 verification.

## Important Distinction

Canonicalization is value-based, not lexeme-preserving.

- Different source lexemes MAY map to the same canonical bytes.
- The strict profile MAY reject lexemes before canonicalization.
