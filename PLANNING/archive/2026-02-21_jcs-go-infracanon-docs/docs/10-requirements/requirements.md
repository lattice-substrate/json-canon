# Requirements Registry (Draft)

**Status:** Draft  
**Traceability rule:** every requirement MUST map to (a) source clause, (b) code anchor, and (c) at least one test and/or corpus vector.

See **traceability.md** for the mapping process and how audits are supported.

| ID | Category | Requirement | Source | Acceptance criteria | Tests/Vectors | Code anchor |
|---|---|---|---|---|---|---|
| JCS-REQ-0001 | Conformance | The canonicalizer MUST accept only inputs that are valid JSON texts per RFC 8259. | RFC 8259 (§3, §6–§8) | Given any non-RFC8259 input, Transform returns an error (non-nil). | unit:TestRejectNonJSON; corpus:invalid/* | internal/token/*, internal/parse/* |
| JCS-REQ-0002 | Conformance | The canonicalizer MUST enforce the I‑JSON constraints required by RFC 8785. | RFC 8785 / RFC 7493 (RFC8785 Intro; RFC7493 §2) | Inputs violating RFC7493 constraints are rejected with deterministic codes. | unit:TestIJSONConstraints; corpus:invalid/ijson_* | internal/ijson/* |
| JCS-REQ-0003 | Output | Whitespace between JSON tokens MUST NOT be emitted. | RFC 8785 (§3.2.1) | Canonical output contains no spaces/tabs/newlines outside of strings. | unit:TestNoWhitespaceEmission | internal/emit/* |
| JCS-REQ-0004 | Output | Canonical output MUST be encoded in UTF‑8. | RFC 8785 (§3.2.4) | Output bytes are valid UTF‑8. | unit:TestOutputUTF8 | internal/emit/* |
| JCS-REQ-0100 | Encoding | Input JSON text exchanged between systems MUST be UTF‑8. | RFC 8259 (§8.1) | Non-UTF8 inputs are rejected. | corpus:invalid/utf8_* | internal/utf8/* |
| JCS-REQ-0101 | Encoding | I‑JSON messages MUST be encoded using UTF‑8. | RFC 7493 (§2.1) | Non-UTF8 inputs are rejected. | corpus:invalid/utf8_* | internal/utf8/* |
| JCS-REQ-0102 | Encoding | Decoding MUST protect against invalid UTF‑8 sequences (overlongs, invalid continuations, truncations). | RFC 3629 (§4, §10) | Invalid UTF‑8 sequences produce JCS_ERR_INVALID_UTF8. | unit:TestInvalidUTF8; corpus:invalid/utf8_* | internal/utf8/validate.go |
| JCS-REQ-0103 | Encoding | UTF‑8 MUST NOT encode surrogate code points U+D800..U+DFFF; such sequences MUST be rejected. | RFC 3629 (§3) | Any decoded scalar in surrogate range yields JCS_ERR_FORBIDDEN_CODEPOINT. | unit:TestSurrogateUTF8; corpus:invalid/utf8_surrogate_* | internal/utf8/validate.go |
| JCS-REQ-0110 | Strings | JSON strings MUST reject unescaped control characters U+0000..U+001F. | RFC 8259 (§7) | Input containing such characters inside strings without escape is rejected. | unit:TestUnescapedControlChar | internal/token/scan_string.go |
| JCS-REQ-0111 | Strings | The canonicalizer MUST implement RFC 8259 escape sequences, including \uXXXX. | RFC 8259 (§7) | All valid escapes decode to correct scalar values. | unit:TestEscapes | internal/unesc/* |
| JCS-REQ-0112 | Strings | Unpaired UTF‑16 surrogates in \u escapes MUST be rejected. | RFC 7493 (§2.1) | "\uDEAD" yields JCS_ERR_FORBIDDEN_CODEPOINT or a dedicated surrogate error. | unit:TestUnpairedSurrogateEscape; corpus:invalid/ijson_unpaired_surrogate.json | internal/unesc/* |
| JCS-REQ-0113 | Strings | Surrogate pairs in \u escapes MUST be decoded to the corresponding Unicode scalar value. | RFC 8259 / RFC 7493 (RFC8259 §8.2; RFC7493 §2.1) | Valid surrogate pair decodes to one scalar and is emitted correctly. | unit:TestSurrogatePairDecode; corpus:valid/emoji.json | internal/unesc/* |
| JCS-REQ-0114 | Strings | Names and string values MUST NOT include Surrogates or Noncharacters (direct UTF‑8 or escaped). | RFC 7493 (§2.1) | Forbidden scalars produce JCS_ERR_FORBIDDEN_CODEPOINT. | unit:TestNoncharacters; corpus:invalid/ijson_noncharacter_* | internal/ijson/* |
| JCS-REQ-0115 | Strings | Parsed JSON string data MUST NOT be altered during subsequent serializations; Unicode normalization MUST NOT be applied. | RFC 8785 (§3.1 note) | Decode+emit preserves scalar sequence; no normalization transformations. | unit:TestNoNormalization | internal/emit/string.go |
| JCS-REQ-0200 | Numbers | A JSON number MUST be base‑10 with optional leading minus; leading zeros are not allowed. | RFC 8259 (§6) | Reject +1, 01, 00, -01. | corpus:invalid/num_* | internal/token/scan_number.go |
| JCS-REQ-0201 | Numbers | Numeric values such as Infinity and NaN are not permitted in JSON input. | RFC 8259 (§6) | Reject tokens 'NaN', 'Infinity', 'Inf'. | corpus:invalid/num_nan_inf_* | internal/token/scan_number.go |
| JCS-REQ-0202 | Numbers | I‑JSON senders SHOULD NOT include numbers exceeding binary64 precision/range; the canonicalizer MUST reject conversions that overflow to Infinity. | RFC 7493 / RFC 8785 (RFC7493 §2.2; RFC8785 number rules) | Inputs that overflow when parsed are rejected. | unit:TestNumberOverflow; corpus:invalid/num_overflow_* | internal/numfmt/* |
| JCS-REQ-0203 | Numbers | Number serialization MUST be compatible with ECMAScript JSON serialization and RFC 8785 guidance; -0 MUST serialize as 0. | RFC 8785 (§3.2.2 + Appendix B) | Input -0 emits '0'. | unit:TestMinusZero; corpus:rfc8785/number_samples.json | internal/numfmt/* |
| JCS-REQ-0204 | Numbers | Number serializer behavior MUST be validated using RFC 8785 Appendix B and a V8 differential corpus as recommended by RFC 8785. | RFC 8785 (§3.2.2; Appendix B; Appendix guidance) | Test harness passes against Appendix B; V8 differential test passes for corpus. | harness:TestV8Corpus | tools/gen_v8_numbers.js; internal/numfmt/* |
| JCS-REQ-0300 | Objects | Objects MUST NOT have members with duplicate names after unescaping. | RFC 7493 (§2.3) | Duplicate keys rejected deterministically. | unit:TestDuplicateKeys; corpus:invalid/dup_key_* | internal/parse/object.go |
| JCS-REQ-0301 | Objects | Property name sorting MUST be applied to raw (unescaped) form and use UTF‑16 code unit arrays. | RFC 8785 (§3.2.3) | Sorting matches RFC 8785 test vector. | corpus:rfc8785/key_sorting.json | internal/sort16/* |
| JCS-REQ-0302 | Objects | UTF‑16 sorting MUST compare code units as unsigned integers, independent of locale. | RFC 8785 (§3.2.3) | Sorting invariant under locale env vars. | determinism:LocaleMatrix | internal/sort16/* |
| JCS-REQ-0310 | Arrays | Array element order MUST be preserved while recursively canonicalizing elements. | RFC 8785 (§3.2) | Array output order matches input. | unit:TestArrayOrder | internal/parse/array.go |
| JCS-REQ-0400 | Emitter | Strings MUST be emitted as RFC 8259 JSON strings escaping only required characters; no HTML escaping. | RFC 8259 (§7) | Output contains literal '<' and '&' when present in input. | unit:TestNoHTMLEscape | internal/emit/string.go |
| JCS-REQ-0401 | Emitter | Object emission MUST use ':' and ',' separators without extra whitespace. | RFC 8785 (§3.2.1) | No whitespace around separators. | unit:TestNoWhitespaceAroundSeparators | internal/emit/* |
| JCS-REQ-0500 | Determinism | Canonical output MUST depend only on input bytes and be byte-identical across supported Linux systems. | RFC 8785 (design intent) (Intro; §3.2.4) | Determinism matrix passes across distros/arch/kernel. | matrix:docs/11-testing/determinism-matrix.md | tools/run_determinism_matrix.sh |
| JCS-REQ-0501 | Determinism | Canonicalization MUST be idempotent for all valid inputs. | RFC 8785 (canonicalization property) (Derived) | canon(canon(x)) == canon(x). | unit:TestIdempotence; corpus:valid/* | internal/* |

## Notes
- Some requirements are derived properties (e.g., idempotence). Where derived, the “Source” column indicates “Derived” and the rationale is described in traceability.md.
- This registry is intentionally small in early drafts; it expands as implementation details become fixed.
