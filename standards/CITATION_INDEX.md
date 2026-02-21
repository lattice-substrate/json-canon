# Standards Citation Index

Maps each requirement ID to the exact authoritative clause that governs it.

## Source Documents

| Short Name | Full Title | URL |
|------------|-----------|-----|
| RFC 8785 | JSON Canonicalization Scheme (JCS) | https://www.rfc-editor.org/rfc/rfc8785 |
| RFC 8259 | The JavaScript Object Notation (JSON) Data Interchange Format | https://www.rfc-editor.org/rfc/rfc8259 |
| RFC 7493 | The I-JSON Message Format | https://www.rfc-editor.org/rfc/rfc7493 |
| RFC 3629 | UTF-8, a transformation format of ISO 10646 | https://www.rfc-editor.org/rfc/rfc3629 |
| ECMA-262 | ECMAScript Language Specification | https://tc39.es/ecma262/ |
| IEEE 754 | IEEE Standard for Floating-Point Arithmetic | https://ieeexplore.ieee.org/document/8766229 |

## Requirement → Clause Mapping

### UTF-8 Input Validation

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| PARSE-UTF8-001 | RFC 3629 | §3 (Definition of UTF-8) | Octets MUST conform to the encoding scheme in Table 1. Invalid sequences (continuation without start, truncated, 0xFE/0xFF) are ill-formed. |
| PARSE-UTF8-002 | RFC 3629 | §3 (Table 1, rows 1-2) | Overlong forms are explicitly excluded by the restricted ranges in Table 1 (e.g., two-byte sequences start at U+0080). |

### JSON Grammar

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| PARSE-GRAM-001 | RFC 8259 | §6 ¶3 | "A number…begins with an optional minus sign…The integer component…MUST NOT have leading zeros." |
| PARSE-GRAM-002 | RFC 8259 | §4 ¶1 | object = `{` [ member *( `,` member ) ] `}` — no trailing comma in grammar. |
| PARSE-GRAM-003 | RFC 8259 | §5 ¶1 | array = `[` [ value *( `,` value ) ] `]` — no trailing comma in grammar. |
| PARSE-GRAM-004 | RFC 8259 | §7 ¶1 | "All Unicode characters may be placed…except for the characters that MUST be escaped: quotation mark, reverse solidus, and the control characters (U+0000 through U+001F)." |
| PARSE-GRAM-005 | RFC 8259 | §2 ¶1 | "A JSON text is a serialized value." (any value type is valid at top level) |
| PARSE-GRAM-006 | RFC 8259 | §2 ¶2 | "Insignificant whitespace is allowed before or after any of the six structural characters." |
| PARSE-GRAM-007 | RFC 8259 | §3 ¶1 | `false = %x66.61.6c.73.65`, `null = %x6e.75.6c.6c`, `true = %x74.72.75.65` — exact byte sequences required. |
| PARSE-GRAM-008 | RFC 8259 | §2 ¶1 | "A JSON text is a serialized value." — exactly one value, no trailing content. |
| PARSE-GRAM-009 | RFC 8259 | §6 ¶1 | number = `[ minus ] int [ frac ] [ exp ]` — full grammar specified. |
| PARSE-GRAM-010 | RFC 8259 | §7 ¶2 | "Any character may be escaped" via listed forms: `\" \\ \/ \b \f \n \r \t \uXXXX`. |

### I-JSON Constraints

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| IJSON-DUP-001 | RFC 7493 | §2.3 ¶1 | "An I-JSON message MUST NOT have any object members with duplicate names." |
| IJSON-DUP-002 | RFC 7493 | §2.3 ¶1 | Duplicate detection applies to decoded names (escape resolution before comparison). |
| IJSON-SUR-001 | RFC 7493 | §2.1 ¶3 | "I-JSON messages MUST NOT include…unpaired surrogates." (high without low) |
| IJSON-SUR-002 | RFC 7493 | §2.1 ¶3 | "I-JSON messages MUST NOT include…unpaired surrogates." (low without high) |
| IJSON-SUR-003 | RFC 7493 | §2.1 ¶3 | Valid \uHHHH\uLLLL surrogate pairs decode to supplementary-plane characters (per RFC 8259 §7). |
| IJSON-NONC-001 | RFC 7493 | §2.1 ¶3 | "I-JSON messages MUST NOT include…code points…the 66 Unicode Noncharacters" (U+FDD0..U+FDEF, U+xFFFE/U+xFFFF for planes 0-16). |

### Canonical Form (RFC 8785)

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| CANON-WS-001 | RFC 8785 | §3.2.1 | "No whitespace is emitted between the elements." |
| CANON-STR-001 | RFC 8785 | §3.2.2.2 | U+0008 → `\b` |
| CANON-STR-002 | RFC 8785 | §3.2.2.2 | U+0009 → `\t` |
| CANON-STR-003 | RFC 8785 | §3.2.2.2 | U+000A → `\n` |
| CANON-STR-004 | RFC 8785 | §3.2.2.2 | U+000C → `\f` |
| CANON-STR-005 | RFC 8785 | §3.2.2.2 | U+000D → `\r` |
| CANON-STR-006 | RFC 8785 | §3.2.2.2 | Other controls U+0000..U+001F → `\u00xx` (lowercase hex). |
| CANON-STR-007 | RFC 8785 | §3.2.2.2 | U+0022 → `\"` |
| CANON-STR-008 | RFC 8785 | §3.2.2.2 | U+005C → `\\` |
| CANON-STR-009 | RFC 8785 | §3.2.2.2 | U+002F → `/` (NOT escaped). |
| CANON-STR-010 | RFC 8785 | §3.2.2.2 | Characters > U+001F (except `"` and `\`) → raw UTF-8 bytes. |
| CANON-STR-011 | RFC 8785 | §3.2.2.2 | "No…normalization is applied." |
| CANON-STR-012 | RFC 8785 | §3.2.2.2 | Strings enclosed in double quotes. |
| CANON-SORT-001 | RFC 8785 | §3.2.3 ¶2 | "The properties are sorted…based on the Unicode code units of their names" (UTF-16 code unit order). |
| CANON-SORT-002 | RFC 8785 | §3.2.3 ¶3 | "Applied recursively to all sub-objects." |
| CANON-SORT-003 | RFC 8785 | §3.2.3 ¶4 | "Array element order is preserved." |
| CANON-SORT-004 | RFC 8785 | §3.2.3 ¶2 | Sorting based on unescaped property name values. |
| CANON-SORT-005 | RFC 8785 | §3.2.3 ¶2 | Lexicographic: compare UTF-16 code units at first difference; shorter prefix precedes longer. |
| CANON-LIT-001 | RFC 8785 | §3.2.1 | "Literal names MUST be serialized as follows: null, true, false" (lowercase). |
| CANON-ENC-001 | RFC 8785 | §3.2 ¶1 | "JCS…MUST be encoded in UTF-8." |
| CANON-ENC-002 | RFC 8259 | §8.1 ¶3 | "Implementations MUST NOT add a byte order mark." |
| GEN-GRAM-001 | RFC 8259 | §10 | "A JSON text…MUST strictly conform to the JSON grammar." |

### Number Formatting (ECMA-262)

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| ECMA-FMT-001 | ECMA-262 | §6.1.6.1.20 step 1 | "If m is NaN, return the String 'NaN'." (JCS rejects NaN instead) |
| ECMA-FMT-002 | ECMA-262 | §6.1.6.1.20 step 2 | "If m is +0 or -0, return the String '0'." |
| ECMA-FMT-003 | ECMA-262 | §6.1.6.1.20 step 4 | "If m is +∞, return the String 'Infinity'." (JCS rejects Infinity instead) |
| ECMA-FMT-004 | ECMA-262 | §6.1.6.1.20 step 7 | k ≤ n ≤ 21 → integer digits + trailing zeros. |
| ECMA-FMT-005 | ECMA-262 | §6.1.6.1.20 step 8 | 0 < n ≤ 21, n < k → fixed decimal notation. |
| ECMA-FMT-006 | ECMA-262 | §6.1.6.1.20 step 9 | -6 < n ≤ 0 → "0." + leading zeros + digits. |
| ECMA-FMT-007 | ECMA-262 | §6.1.6.1.20 step 10 | Otherwise → exponential notation with `e+`/`e-`. |
| ECMA-FMT-008 | ECMA-262 | §6.1.6.1.20 step 5 | Shortest k such that the decimal representation round-trips. |
| ECMA-FMT-009 | ECMA-262 | §6.1.6.1.20 Note 2 | "If there are multiple…even-valued digit is used." |
| ECMA-FMT-010 | ECMA-262 | §6.1.6.1.20 step 3 | "If m < 0, return the string-concatenation of '-' and…" |
| ECMA-FMT-011 | ECMA-262 | §6.1.6.1.20 step 5 | "Choose…the smallest possible value of k." |
| ECMA-FMT-012 | ECMA-262 | §6.1.6.1.20 step 10a | k = 1 branch: single digit + exponent, no decimal point. |

### Verification

| Requirement ID | Source | Clause | Normative Text (paraphrased) |
|---------------|--------|--------|------------------------------|
| VERIFY-ORDER-001 | RFC 8785 | §3.2.3 | Non-canonical key ordering detected via byte comparison. |
| VERIFY-WS-001 | RFC 8785 | §3.2.1 | Non-canonical whitespace detected via byte comparison. |
