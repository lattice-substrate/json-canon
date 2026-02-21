# 05. JCS Serialization

Serialization MUST produce RFC 8785-compatible canonical JSON bytes.

## 5.1 General

- Output MUST contain no insignificant whitespace.
- Serialization MUST be deterministic.

## 5.2 String Escaping

The serializer MUST apply the following escapes:

- `\"` for `"`
- `\\\\` for `\\`
- `\\b`, `\\t`, `\\n`, `\\f`, `\\r` for control shortcuts
- `\\u00xx` (lowercase hex) for other `U+0000..U+001F`

The serializer MUST NOT escape `/`, `<`, `>`, or `&` solely for convenience.

Unicode normalization MUST NOT be applied.

## 5.3 Object Key Ordering

Object members MUST be sorted by UTF-16 code unit lexicographic order.

Sort order MUST NOT use UTF-8 byte order as a substitute.

## 5.4 Number Serialization

Numbers MUST be serialized according to ECMAScript Number-to-string behavior required by RFC 8785.

Formatting requirements include:

- lowercase `e` in exponent form,
- explicit `+` for positive exponent,
- fixed vs exponent threshold behavior per ECMAScript rules,
- midpoint tie handling per even-digit rule.

NaN and Infinity MUST be rejected.
