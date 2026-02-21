# Tokenizer and RFC 8259 Parsing Rules

**Status:** Draft  
**Normative:** RFC 8259

## 1. Principle
The canonicalizer MUST accept only JSON texts that conform to RFC 8259. Inputs outside RFC 8259 MUST be rejected with a deterministic error code.

## 2. Token model
The tokenizer operates on bytes and produces a stream of tokens:
- structural: `{ } [ ] : ,`
- string: `"..."` (with escape sequences; decoded later)
- number: RFC 8259 `number` token (validated lexically here)
- literals: `true`, `false`, `null`
- whitespace is skipped

## 3. Strict number lexical validation (no `ParseFloat` on arbitrary tokens)
RFC 8259 requires:
- base 10 digits,
- optional leading minus,
- no leading zeros (except the literal `0`),
- exponent `e`/`E` may be followed by `+`/`-`,
- “Infinity and NaN are not permitted.”  
See RFC 8259 §6. Source: https://datatracker.ietf.org/doc/html/rfc8259

### 3.1 Accepted grammar (RFC 8259 ABNF)
Implement the ABNF exactly:
`number = [ minus ] int [ frac ] [ exp ]`  
`int = zero / ( digit1-9 *DIGIT )`  
`frac = "." 1*DIGIT`  
`exp = ("e" / "E") [ "-" / "+" ] 1*DIGIT`  
(Quoted from RFC 8259 §6.) Source: https://datatracker.ietf.org/doc/html/rfc8259

### 3.2 Explicitly rejected examples
- `+1` (leading plus not in ABNF)
- `01` (leading zeros not allowed)
- `0x1p-2` (hex float is not JSON)
- `NaN`, `Infinity` (not permitted)

## 4. Duplicate key handling is *not* a tokenizer concern
Duplicate name detection is performed at the object canonicalization step, after string decoding/unescaping, as required by I‑JSON (RFC 7493). The tokenizer only identifies member-name string tokens.

## 5. Output: parse events
The parser should build events or a small AST sufficient to:
- buffer object members for sorting,
- stream arrays where possible,
- recursively canonicalize nested objects/arrays.

## References
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
