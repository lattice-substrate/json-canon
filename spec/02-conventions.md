# 02. Conventions And Terminology

## Requirement Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" are interpreted as described in RFC 2119 and RFC 8174.

## Core Terms

- `JSON value`: a value as defined by RFC 8259.
- `strict profile`: additional validity constraints beyond base RFC 8259 syntax.
- `canonical bytes`: deterministic RFC 8785 serialization bytes for a parsed JSON value.
- `canonical input`: input bytes exactly equal to canonical bytes.

## Important Distinction

Canonicalization is value-based, not lexeme-preserving.

- Different source lexemes MAY map to the same canonical bytes.
- The strict profile MAY reject lexemes before canonicalization.
