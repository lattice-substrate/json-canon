# Glossary

**Status:** Draft

- **BCP 14:** The IETF Best Current Practice describing normative requirement keywords (RFC 2119, RFC 8174).
- **Canonicalization:** Transforming data into a unique, deterministic representation.
- **ECMAScript:** The language specification that defines JSON serialization behavior used by RFC 8785.
- **I‑JSON:** A restricted JSON profile for predictable interoperability (RFC 7493).
- **JCS:** JSON Canonicalization Scheme (RFC 8785).
- **Noncharacter:** Unicode code points reserved for internal use and not for interchange; I‑JSON forbids these in strings/names.
- **Surrogate:** UTF‑16 code units used for representing code points above U+FFFF; I‑JSON forbids surrogates in strings/names unless they form a valid pair in escape sequences.
- **Silent normalization:** Accepting invalid input and rewriting it into valid output without error.
