# Stable Error Code Registry

**Status:** Draft  
**Rule:** error codes are part of the stable interface.

## 1. Error type
The canonicalizer returns an error with:
- `Code` (stable string like `JCS_ERR_BAD_NUMBER_SYNTAX`)
- `Message` (human-readable; not stable)
- `Offset` (byte index; best-effort)
- `Context` (optional; may include token kind)

## 2. Error codes (initial set)
### Syntax / tokenization
- `JCS_ERR_UNEXPECTED_EOF`
- `JCS_ERR_BAD_TOKEN`
- `JCS_ERR_BAD_NUMBER_SYNTAX`
- `JCS_ERR_BAD_STRING_ESCAPE`

### Encoding / Unicode
- `JCS_ERR_INVALID_UTF8`
- `JCS_ERR_FORBIDDEN_CODEPOINT` (surrogate/noncharacter)

### Structural / semantic constraints
- `JCS_ERR_DUPLICATE_KEY`
- `JCS_ERR_NUMBER_OUT_OF_RANGE` (overflow to Inf, etc.)

### Internal invariants
- `JCS_ERR_INTERNAL` (should never occur; indicates a bug)

## 3. Mapping to standards
- `JCS_ERR_BAD_NUMBER_SYNTAX` maps to RFC 8259 §6 grammar and “Infinity and NaN are not permitted”.
- `JCS_ERR_INVALID_UTF8` maps to RFC 8259 §8.1 + RFC 7493 §2.1 + RFC 3629 validity rules.
- `JCS_ERR_DUPLICATE_KEY` maps to RFC 7493 §2.3.
- `JCS_ERR_FORBIDDEN_CODEPOINT` maps to RFC 7493 §2.1 and RFC 3629 surrogate prohibition.

## 4. Test requirements
Every error code MUST have:
- at least one unit test exercising it directly, and
- at least one corpus vector in corpus/invalid/ that expects it.
