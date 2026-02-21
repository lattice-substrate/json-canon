# Normative Requirement Registry

Formal catalog of normative RFC/ECMA requirements implemented by `json-canon`.

## Legend

| Column | Meaning |
|--------|---------|
| ID | Stable requirement identifier: `DOMAIN-NNN` |
| Spec | Normative source document |
| Section | Section or clause within the source |
| Level | MUST, SHALL, or SHOULD |
| Requirement | Normative text (paraphrased) |

---
## PARSE-UTF8 — UTF-8 Input Validation (RFC 3629)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| PARSE-UTF8-001 | RFC 3629 | §3 | MUST | Input MUST be valid UTF-8. Invalid byte sequences (continuation bytes without start, truncated multibyte, bytes 0xFE/0xFF) MUST be rejected. |
| PARSE-UTF8-002 | RFC 3629 | §3 | MUST | Overlong UTF-8 encodings MUST be rejected (e.g. 0xC0 0xAF for U+002F). |

## PARSE-GRAM — JSON Grammar (RFC 8259)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| PARSE-GRAM-001 | RFC 8259 | §6 | MUST | Leading zeros in numbers MUST be rejected (e.g. `01`). |
| PARSE-GRAM-002 | RFC 8259 | §4 | MUST | Trailing commas in objects MUST be rejected (e.g. `{"a":1,}`). |
| PARSE-GRAM-003 | RFC 8259 | §5 | MUST | Trailing commas in arrays MUST be rejected (e.g. `[1,]`). |
| PARSE-GRAM-004 | RFC 8259 | §7 | MUST | Unescaped control characters U+0000..U+001F in strings MUST be rejected. |
| PARSE-GRAM-005 | RFC 8259 | §2 | MUST | Top-level value MAY be any JSON value (object, array, string, number, boolean, null). |
| PARSE-GRAM-006 | RFC 8259 | §2 | MUST | Insignificant whitespace (space, tab, LF, CR) before/after structural characters MUST be accepted and ignored. |
| PARSE-GRAM-007 | RFC 8259 | §3 | MUST | Invalid literals (e.g. `tru`, `fals`, `nul`) MUST be rejected. |
| PARSE-GRAM-008 | RFC 8259 | §2 | MUST | Trailing content after a complete JSON value MUST be rejected. |
| PARSE-GRAM-009 | RFC 8259 | §6 | MUST | Number tokens MUST match the RFC 8259 grammar: optional minus, integer, optional fraction, optional exponent. |
| PARSE-GRAM-010 | RFC 8259 | §7 | MUST | String escape sequences MUST be one of: `\" \\ \/ \b \f \n \r \t \uXXXX`. Invalid escapes MUST be rejected. |

## IJSON-DUP — Duplicate Key Rejection (RFC 7493)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| IJSON-DUP-001 | RFC 7493 | §2.3 | MUST | Objects with duplicate member names MUST be rejected. |
| IJSON-DUP-002 | RFC 7493 | §2.3 | MUST | Duplicate detection MUST compare keys after escape decoding (e.g. `\u0061` equals `a`). |

## IJSON-SUR — Surrogate Handling (RFC 7493)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| IJSON-SUR-001 | RFC 7493 | §2.1 | MUST | Lone high surrogates (U+D800..U+DBFF not followed by U+DC00..U+DFFF) MUST be rejected. |
| IJSON-SUR-002 | RFC 7493 | §2.1 | MUST | Lone low surrogates (U+DC00..U+DFFF without preceding high surrogate) MUST be rejected. |
| IJSON-SUR-003 | RFC 7493 | §2.1 | MUST | Valid surrogate pairs (\uD800-\uDBFF followed by \uDC00-\uDFFF) MUST be decoded to supplementary-plane scalar values. |

## IJSON-NONC — Noncharacter Rejection (RFC 7493)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| IJSON-NONC-001 | RFC 7493 | §2.1 | MUST | Unicode noncharacters MUST be rejected. The 66 noncharacters are: U+FDD0..U+FDEF (32 codepoints) and U+xFFFE/U+xFFFF for planes 0-16 (34 codepoints). |

## CANON-WS — Whitespace (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CANON-WS-001 | RFC 8785 | §3.2.1 | MUST | Canonical output MUST NOT contain insignificant whitespace between tokens. |

## CANON-STR — String Serialization (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CANON-STR-001 | RFC 8785 | §3.2.2.2 | MUST | U+0008 (backspace) MUST be escaped as `\b`. |
| CANON-STR-002 | RFC 8785 | §3.2.2.2 | MUST | U+0009 (tab) MUST be escaped as `\t`. |
| CANON-STR-003 | RFC 8785 | §3.2.2.2 | MUST | U+000A (line feed) MUST be escaped as `\n`. |
| CANON-STR-004 | RFC 8785 | §3.2.2.2 | MUST | U+000C (form feed) MUST be escaped as `\f`. |
| CANON-STR-005 | RFC 8785 | §3.2.2.2 | MUST | U+000D (carriage return) MUST be escaped as `\r`. |
| CANON-STR-006 | RFC 8785 | §3.2.2.2 | MUST | Other control characters U+0000..U+001F (excluding \b \t \n \f \r) MUST be escaped as `\u00xx` with lowercase hex digits. |
| CANON-STR-007 | RFC 8785 | §3.2.2.2 | MUST | U+0022 (quotation mark) MUST be escaped as `\"`. |
| CANON-STR-008 | RFC 8785 | §3.2.2.2 | MUST | U+005C (reverse solidus) MUST be escaped as `\\`. |
| CANON-STR-009 | RFC 8785 | §3.2.2.2 | MUST | U+002F (solidus) MUST NOT be escaped in canonical output. |
| CANON-STR-010 | RFC 8785 | §3.2.2.2 | MUST | Characters above U+001F (except `"` and `\`) MUST be output as raw UTF-8, not escaped. |
| CANON-STR-011 | RFC 8785 | §3.2.2.2 | MUST | No Unicode normalization SHALL be applied to strings. |
| CANON-STR-012 | RFC 8785 | §3.2.2.2 | MUST | Serialized strings MUST be enclosed in double quotes. |

## CANON-SORT — Object Key Sorting (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CANON-SORT-001 | RFC 8785 | §3.2.3 | MUST | Object members MUST be sorted by key using UTF-16 code-unit comparison (NOT UTF-8 byte order). |
| CANON-SORT-002 | RFC 8785 | §3.2.3 | MUST | Sorting MUST be applied recursively to nested objects. |
| CANON-SORT-003 | RFC 8785 | §3.2.3 | MUST | Array element order MUST be preserved (not sorted). |
| CANON-SORT-004 | RFC 8785 | §3.2.3 | MUST | Sorting MUST be based on property names in raw (unescaped) form. |
| CANON-SORT-005 | RFC 8785 | §3.2.3 | MUST | Lexicographic order MUST compare UTF-16 code units at first differing index; if equal prefix, shorter string precedes longer. |

## CANON-LIT — Literal Serialization (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CANON-LIT-001 | RFC 8785 | §3.2.1 | MUST | Literals MUST be lowercase: `null`, `true`, `false`. |

## CANON-ENC — Output Encoding (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CANON-ENC-001 | RFC 8785 | §3.2 | MUST | Canonical output MUST be encoded as UTF-8. |
| CANON-ENC-002 | RFC 8259 | §8.1 | MUST | JSON generator output MUST NOT include a UTF-8 BOM prefix (U+FEFF). |

## GEN-GRAM — Generator Grammar Conformance (RFC 8259)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| GEN-GRAM-001 | RFC 8259 | §10 | MUST | JSON generator output MUST strictly conform to the JSON grammar. |

## ECMA-FMT — Number Formatting (ECMA-262)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| ECMA-FMT-001 | ECMA-262 | §6.1.6.1.20 | MUST | NaN MUST be rejected (not representable in JSON). |
| ECMA-FMT-002 | ECMA-262 | §6.1.6.1.20 | MUST | Negative zero (-0) MUST serialize as `"0"` (step 2). |
| ECMA-FMT-003 | ECMA-262 | §6.1.6.1.20 | MUST | Infinity MUST be rejected (not representable in JSON). |
| ECMA-FMT-004 | ECMA-262 | §6.1.6.1.20 | MUST | When 1 ≤ n ≤ 21 and k ≤ n: emit integer digits followed by (n-k) zeros (step 7). |
| ECMA-FMT-005 | ECMA-262 | §6.1.6.1.20 | MUST | When 0 < n ≤ 21 and n < k: emit n integer digits, decimal point, then remaining digits (step 8). |
| ECMA-FMT-006 | ECMA-262 | §6.1.6.1.20 | MUST | When -6 < n ≤ 0: emit `0.` followed by (-n) zeros then all digits (step 9). |
| ECMA-FMT-007 | ECMA-262 | §6.1.6.1.20 | MUST | Otherwise: exponential notation with lowercase `e`, explicit `+` for positive exponents (step 10). |
| ECMA-FMT-008 | ECMA-262 | §6.1.6.1.20 | MUST | Digit generation MUST produce the shortest representation that round-trips to the same IEEE 754 double. |
| ECMA-FMT-009 | ECMA-262 | §6.1.6.1.20 Note 2 | MUST | Tie-breaking MUST use even-digit (banker's rounding) rule. |
| ECMA-FMT-010 | ECMA-262 | §6.1.6.1.20 | MUST | Negative finite numbers MUST serialize with leading `-` followed by serialization of absolute value (step 3). |
| ECMA-FMT-011 | ECMA-262 | §6.1.6.1.20 | MUST | Intermediate `(n,k,s)` selection MUST use smallest possible `k` satisfying the algorithm constraints (step 5). |
| ECMA-FMT-012 | ECMA-262 | §6.1.6.1.20 | MUST | Scientific notation branch with single significant digit MUST omit decimal point (`k = 1` branch in step 10). |

## VERIFY — Canonical Verification (RFC 8785)

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| VERIFY-ORDER-001 | RFC 8785 | §3.2.3 | MUST | Non-canonical key ordering MUST be rejected by verify. |
| VERIFY-WS-001 | RFC 8785 | §3.2.1 | MUST | Non-canonical whitespace MUST be rejected by verify. |

