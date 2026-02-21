# 03. Data Model

The implementation operates on a parsed tree with the following kinds:

- null
- bool
- number (IEEE 754 binary64)
- string (decoded Unicode scalar sequence)
- array (ordered values)
- object (ordered members as parsed)

Object member ordering in the parse tree is preserved as input order.
Canonical sort order is applied during serialization.

## Numeric Domain

Finite IEEE 754 binary64 values are supported.

- NaN and Infinity are invalid for canonical serialization.
- Negative zero (`-0`) is profile-rejected at parse time.
- Non-zero tokens that underflow to zero are profile-rejected.
