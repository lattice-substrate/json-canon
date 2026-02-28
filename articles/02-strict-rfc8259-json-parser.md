---
title: "What Your JSON Parser Doesn't Reject: Building a Strict RFC 8259 Parser in Go"
published: false
tags: go, programming, parsing, json
series: "Building Infrastructure-Grade JSON Canonicalization in Go"
---

Most JSON parsers are lenient by design. They accept input that RFC 8259 forbids, because lenient parsing is convenient and rarely causes visible problems. But when your downstream depends on deterministic processing — canonical signatures, content-addressed storage, reproducible builds — leniency is a defect. A value silently accepted by one parser and rejected by another breaks your contract.

I built a strict JSON parser in 809 lines of Go. It enforces every constraint in [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) and adds the [RFC 7493](https://www.rfc-editor.org/rfc/rfc7493) (I-JSON) restrictions that [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) requires. This article walks through the design and implementation, focusing on the parts where strictness requires real work: surrogate pairs, noncharacter detection, duplicate keys after escape decoding, and resource bounds.

## The Gap: What Canonicalization Cannot Accept

Go's `encoding/json` is a solid general-purpose parser. For canonicalization, some of its documented behaviors are intentionally too permissive. On Go 1.25.6, representative examples look like this:

| Input | `encoding/json` result | Strict parser | Why it matters |
|-------|-------------------------|---------------|----------------|
| `"\uD800\u0041"` | Decodes as `"�A"` (invalid surrogate replaced) | Rejected: lone surrogate | RFC 7493 §2.1 forbids surrogates |
| `"\uFDD0"` | Accepts noncharacter | Rejected: noncharacter | RFC 7493 §2.1 forbids noncharacters |
| `{"a":1,"a":2}` | Later key wins (`a=2`) | Rejected: duplicate key | RFC 7493 §2.3 requires unique names |
| raw bytes `22 ff 22` | Decodes as `"�"` | Rejected: invalid UTF-8 | silent byte substitution changes meaning |
| `-0` | Accepts lexical `-0` | Rejected: negative zero token | policy profile forbids lexical ambiguity |
| `1e-400` | Parses to `0` | Rejected: underflow | non-zero token collapses to zero |

These are not bugs in `encoding/json`; they are compatibility choices. The [Go package documentation for `Unmarshal`](https://pkg.go.dev/encoding/json#Unmarshal) explicitly states that invalid UTF-8 / invalid UTF-16 surrogates are replaced by U+FFFD, and duplicate object keys are processed in order with later values replacing earlier ones.

Minimal reproduction (Go 1.25.6):

```go
var v any
_ = json.Unmarshal([]byte(`"\uD800\u0041"`), &v) // v == "�A"
_ = json.Unmarshal([]byte(`{"a":1,"a":2}`), &v)  // v == map[string]any{"a": float64(2)}
_ = json.Unmarshal([]byte("1e-400"), &v)         // v == float64(0)
```

For application code, replacement and merge behavior is pragmatic. A canonicalization engine has a different contract: reject ambiguous or lossy inputs so every accepted value has a single deterministic canonical form. If two parsers disagree on interpretation, canonical output is undefined. Rejection is the safe outcome.

## Parser Architecture

The parser uses single-pass recursive descent with byte-level dispatch. The core structure is a `parser` struct that holds the input bytes, a cursor position, depth tracking, and configurable bounds:

```go
type parser struct {
    data             []byte
    pos              int
    depth            int
    valueCount       int
    maxDepth         int
    maxValues        int
    maxObjectMembers int
    maxArrayElements int
    maxStringBytes   int
    maxNumberChars   int
}
```

Value dispatch is a single `switch` on the first byte:

```go
func (p *parser) parseValue() (*Value, error) {
    p.valueCount++
    if p.valueCount > p.maxValues {
        return nil, p.newErrorf(jcserr.BoundExceeded,
            "value count %d exceeds maximum %d", p.valueCount, p.maxValues)
    }

    c, ok := p.peek()
    if !ok {
        return nil, p.newError("unexpected end of input")
    }

    switch c {
    case '{':  return p.parseObject()
    case '[':  return p.parseArray()
    case '"':  return p.parseString()
    case 't', 'f':  return p.parseBool()
    case 'n':  return p.parseNull()
    default:   return p.parseNumber()
    }
}
```

No lookahead beyond the first byte is needed for type discrimination. Every call to `parseValue` checks the bound counter first.

## UTF-8 Validation: Upfront and Incremental

The parser validates UTF-8 twice, for different reasons.

**Upfront bulk validation** runs before any parsing begins:

```go
if !utf8.Valid(data) {
    return nil, jcserr.New(jcserr.InvalidUTF8, firstInvalidUTF8Offset(data),
        "input is not valid UTF-8")
}
```

This catches structural UTF-8 violations: continuation bytes without start bytes, truncated multibyte sequences, overlong encodings, and raw UTF-8 encodings of surrogate code points. The `firstInvalidUTF8Offset` function locates the exact byte offset of the first violation by scanning rune-by-rune until `utf8.DecodeRune` returns a replacement character:

```go
func firstInvalidUTF8Offset(data []byte) int {
    for i := 0; i < len(data); {
        _, size := utf8.DecodeRune(data[i:])
        if size == 1 && data[i] >= 0x80 {
            return i
        }
        i += size
    }
    return 0
}
```

**Incremental validation** happens during string parsing. After verifying the input is well-formed UTF-8, the string parser still decodes each rune individually to validate *semantic* constraints: noncharacter rejection and surrogate detection that apply to decoded values, not raw bytes.

Why both? The upfront check is fast — Go's `utf8.Valid` is optimized — and gives clear error reporting for malformed input before the parser's position tracking complicates offset calculation. The incremental check applies rules that operate on decoded code points, not byte patterns.

## Number Grammar: Four Layers of Enforcement

RFC 8259 §6 defines a specific number grammar. The parser enforces it in four phases.

### Leading Zero Rejection

```go
func (p *parser) scanZeroIntegerPart() *jcserr.Error {
    p.pos++
    if p.pos < len(p.data) && isDigit(p.data[p.pos]) {
        return p.newError("leading zero in number")
    }
    return nil
}
```

`01`, `007`, `00.5` — all rejected. A zero integer part must be exactly `0`, followed by a non-digit (or end of number). This is RFC 8259 §6: "A number is represented in base 10 using decimal digits. It contains [...] with no superfluous leading zero."

### Missing Fraction Digits

```go
func (p *parser) scanFractionPart(numStart int) *jcserr.Error {
    if p.pos >= len(p.data) || p.data[p.pos] != '.' {
        return nil
    }
    p.pos++

    if p.pos >= len(p.data) || !isDigit(p.data[p.pos]) {
        return p.newError("expected digit after decimal point")
    }
    // ... scan remaining digits
}
```

`1.` and `1.e5` are rejected — the grammar requires at least one digit after the decimal point.

### Lexical Negative Zero

This is a policy enforcement from RFC 7493, not a grammar rule. The value `-0` is mathematically zero, but the *lexical* token `-0` is ambiguous:

```go
// PROF-NEGZ-001: lexical negative zero
if strings.HasPrefix(raw, "-") && tokenRepresentsZero(raw) {
    return nil, jcserr.New(jcserr.NumberNegZero, start,
        "negative zero token is not allowed")
}
```

This rejects `-0`, `-0.0`, `-0e0`, `-0.0e+0`, and every other way to spell negative zero in JSON number syntax. The `tokenRepresentsZero` function checks whether the mantissa portion of the number token contains any non-zero digit, ignoring the exponent entirely.

Note: this is a *different* requirement from serialization. At serialization time, the IEEE 754 bit pattern for -0 outputs as `"0"`. At parse time, the lexical token `-0` is rejected. The two requirements are independent — one governs input, the other governs output.

### Overflow and Underflow

After grammar validation, the number string is parsed with `strconv.ParseFloat`:

```go
f, err := strconv.ParseFloat(raw, 64)

// Overflow: value exceeds IEEE 754 range
if math.IsNaN(f) || math.IsInf(f, 0) {
    return nil, jcserr.New(jcserr.NumberOverflow, start,
        "number overflows IEEE 754 double")
}

// Underflow: non-zero token rounds to zero
if f == 0 && !tokenRepresentsZero(raw) {
    return nil, jcserr.New(jcserr.NumberUnderflow, start,
        "non-zero number underflows to IEEE 754 zero")
}
```

`1e999999` overflows to infinity — rejected. `1e-400` is a non-zero number that underflows to zero in IEEE 754 — also rejected. The underflow check uses the same `tokenRepresentsZero` function: if the *token* has non-zero digits but `ParseFloat` returns zero, the precision was lost.

## String Parsing: Two Paths

The string parser is a linear scan with two code paths: escape sequences and raw UTF-8 bytes.

```go
func (p *parser) parseString() (*Value, error) {
    if err := p.expect('"'); err != nil {
        return nil, err
    }

    var buf []byte
    for {
        if p.pos >= len(p.data) {
            return nil, p.newError("unterminated string")
        }
        b := p.data[p.pos]
        if b == '"' {
            p.pos++
            return &Value{Kind: KindString, Str: string(buf)}, nil
        }
        if b == '\\' {
            // Escape sequence path
            escapeStart := p.pos
            p.pos++
            r, err := p.parseEscape(escapeStart)
            if err != nil { return nil, err }
            if err := validateStringRune(r, escapeStart); err != nil { return nil, err }
            var tmp [4]byte
            n := utf8.EncodeRune(tmp[:], r)
            buf = append(buf, tmp[:n]...)
            continue
        }
        if b < 0x20 {
            // PARSE-GRAM-004: reject unescaped control characters
            return nil, p.newErrorf(jcserr.InvalidGrammar,
                "unescaped control character 0x%02X in string", b)
        }
        // Raw UTF-8 path
        sourceOffset := p.pos
        r, size := utf8.DecodeRune(p.data[p.pos:])
        if err := validateStringRune(r, sourceOffset); err != nil { return nil, err }
        buf = append(buf, p.data[p.pos:p.pos+size]...)
        p.pos += size
    }
}
```

Both paths call `validateStringRune` to check for noncharacters and surrogates. The escape path first decodes the escape to a rune, then validates the decoded value. The raw path decodes the rune from UTF-8 bytes, then validates. The decoded results are identical — `\u0041` and `A` both produce rune 0x41 — which is critical for duplicate key detection later.

The escape dispatch is a straightforward table:

```go
func escapedRune(b byte) (rune, bool) {
    switch b {
    case '"':  return '"', true
    case '\\': return '\\', true
    case '/':  return '/', true
    case 'b':  return '\b', true
    case 'f':  return '\f', true
    case 'n':  return '\n', true
    case 'r':  return '\r', true
    case 't':  return '\t', true
    default:   return 0, false
    }
}
```

RFC 8259 §7 defines exactly these eight simple escapes plus `\uXXXX`. Anything else — `\x41`, `\U0041`, `\a` — is rejected with `InvalidGrammar`.

Control characters below U+0020 that appear unescaped are also rejected per §7: "All Unicode characters may be placed within the quotation marks, except for the characters that MUST be escaped: quotation mark, reverse solidus, and the control characters (U+0000 through U+001F)."

## Surrogate Pairs: 2-Character Lookahead

UTF-16 surrogate pairs in JSON appear as `\uD800\uDC00` — two consecutive `\uXXXX` escapes where the first is a high surrogate (U+D800-U+DBFF) and the second is a low surrogate (U+DC00-U+DFFF). Together they encode a supplementary-plane character (U+10000 and above).

The parser must handle five cases:

1. **Not a surrogate**: Return the rune as-is
2. **Lone low surrogate** (U+DC00-U+DFFF appearing first): Error
3. **High surrogate with no following `\u`**: Error
4. **High surrogate followed by non-low-surrogate**: Error
5. **Valid pair**: Decode to supplementary-plane scalar

```go
func (p *parser) parseUnicodeEscape(sourceOffset int) (rune, *jcserr.Error) {
    r1, err := p.readHex4(sourceOffset)
    if err != nil {
        return 0, err
    }

    if !utf16.IsSurrogate(r1) {
        return r1, nil                              // Case 1: not a surrogate
    }

    if r1 >= 0xDC00 {
        return 0, jcserr.New(jcserr.LoneSurrogate,  // Case 2: lone low surrogate
            sourceOffset, fmt.Sprintf("lone low surrogate U+%04X", r1))
    }

    // Case 3: high surrogate must be followed by \uXXXX
    if p.pos+1 >= len(p.data) || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
        return 0, jcserr.New(jcserr.LoneSurrogate, sourceOffset,
            fmt.Sprintf("lone high surrogate U+%04X (no following \\u)", r1))
    }
    secondEscapeOffset := p.pos
    p.pos += 2                                      // Skip past '\u'

    r2, err := p.readHex4(secondEscapeOffset)
    if err != nil {
        return 0, err
    }
    if r2 < 0xDC00 || r2 > 0xDFFF {
        return 0, jcserr.New(jcserr.LoneSurrogate,  // Case 4: not a low surrogate
            secondEscapeOffset,
            fmt.Sprintf("high surrogate U+%04X followed by non-low-surrogate U+%04X", r1, r2))
    }

    // Case 5: valid pair
    decoded := utf16.DecodeRune(r1, r2)
    if decoded == unicode.ReplacementChar {
        return 0, jcserr.New(jcserr.LoneSurrogate, sourceOffset,
            fmt.Sprintf("invalid surrogate pair U+%04X U+%04X", r1, r2))
    }
    return decoded, nil
}
```

The 2-character lookahead on line 522 checks for `\u` without consuming input. This is the only lookahead in the parser beyond single-byte dispatch. If the two bytes aren't `\u`, the high surrogate is lone and the error points to the *first* escape's offset. If the second escape exists but decodes to a non-low surrogate (like `\uD800\u0041`), the error points to the *second* escape's offset — because that's where the violation occurs.

Compare with `encoding/json`: it replaces lone surrogates with U+FFFD (documented behavior in the `Unmarshal` docs). This is convenient for display pipelines, but it silently changes semantic content. A canonicalization engine cannot do this — changing input bytes means the canonical output no longer represents the original data.

## Noncharacter Rejection: 66 Forbidden Code Points

RFC 7493 §2.1 forbids Unicode noncharacters in I-JSON text. There are exactly 66:

- 32 in the range U+FDD0 to U+FDEF
- 34 at U+xFFFE and U+xFFFF for each of the 17 Unicode planes (0-16)

The implementation uses a compact two-branch check:

```go
func IsNoncharacter(r rune) bool {
    if r >= 0xFDD0 && r <= 0xFDEF {
        return true
    }
    if r&0xFFFE == 0xFFFE && r <= 0x10FFFF {
        return true
    }
    return false
}
```

The bitwise trick on the second branch is worth explaining. The mask `r & 0xFFFE` zeros the lowest bit, so both U+xFFFE and U+xFFFF match the pattern `0xFFFE`. The bound `r <= 0x10FFFF` limits this to planes 0 through 16. This catches U+FFFE, U+FFFF, U+1FFFE, U+1FFFF, all the way up to U+10FFFE and U+10FFFF — 2 per plane times 17 planes = 34 code points.

The validation runs on *decoded* runes, not raw bytes. This matters because supplementary-plane noncharacters like U+1FFFE can appear as either raw UTF-8 bytes or as surrogate pair escapes (`\uD83F\uDFFE`). Both paths decode to the same rune and hit the same check.

## Duplicate Key Detection: After Escape Decoding

RFC 7493 §2.3 requires that JSON objects not contain duplicate member names. The subtle requirement is that `"\u0061"` and `"a"` are the *same key* — both decode to the string "a".

The parser handles this by comparing *decoded* strings, not raw lexemes:

```go
func (p *parser) parseObject() (*Value, error) {
    // ...
    v := &Value{Kind: KindObject}
    seen := make(map[string]int)

    for {
        keyStart := p.pos

        keyVal, err := p.parseString()   // Decodes all escapes
        if err != nil { return nil, err }
        key := keyVal.Str                // Decoded Unicode string

        if firstOff, exists := seen[key]; exists {
            return nil, jcserr.New(jcserr.DuplicateKey, keyStart,
                fmt.Sprintf("duplicate object key %q (first at byte %d)", key, firstOff))
        }
        seen[key] = keyStart

        // ... parse colon and value
    }
}
```

The `seen` map uses the decoded string as the key. When the parser encounters `"\u0061"`, it decodes the escape to produce "a", and the map lookup finds a match with a previously seen raw `"a"`. The error message includes the byte offset of *both* occurrences, enabling precise diagnostics.

This is a design choice that some parsers skip entirely (`encoding/json` processes duplicate keys in input order, with later values replacing or merging earlier ones) or implement on raw bytes (which misses escape-decoded equivalence). For canonicalization, decoded comparison is the only correct approach, because canonical output normalizes escape sequences.

## Resource Bounds: Seven Independent Limits

The parser enforces seven configurable bounds, each checked at a different point in the parse:

| Bound | Default | Checked At |
|-------|---------|------------|
| Input size | 64 MB | Before parsing begins |
| Nesting depth | 1,000 | On each `{` or `[` |
| Total values | 1,000,000 | On each `parseValue` call |
| Object members | 250,000 | After adding each member |
| Array elements | 250,000 | After adding each element |
| String bytes | 8 MB | During string decode (per-string) |
| Number chars | 4,096 | During number scan (per-token) |

All bounds are fail-fast: the parser stops at the first violation rather than continuing to consume input. The number character bound is checked *during* digit scanning, not just after:

```go
func (p *parser) scanNonZeroIntegerDigits(numStart int) *jcserr.Error {
    for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
        p.pos++
        if p.pos-numStart > p.maxNumberChars {
            return jcserr.New(jcserr.BoundExceeded, numStart,
                fmt.Sprintf("number token length %d exceeds maximum %d",
                    p.pos-numStart, p.maxNumberChars))
        }
    }
    return nil
}
```

This prevents a 100 MB number token from being fully scanned before the bound check fires.

Every bound violation returns a `BoundExceeded` failure class, regardless of which bound was hit. This is a deliberate classification choice: the *cause* is "resource policy violation," not "invalid grammar" or "I/O error." Machines consuming the exit code can distinguish policy rejections from syntax errors.

## Error Offset Tracking

Every parser error includes a byte offset pointing to the source position of the violation:

```go
func (p *parser) newError(msg string) *jcserr.Error {
    return jcserr.New(jcserr.InvalidGrammar, p.pos, msg)
}
```

For most errors, the offset is the parser's current position — the byte where parsing failed. But for escape sequences and surrogate pairs, the offset points to the *start* of the escape that caused the problem, not the byte where the violation was detected:

```go
if b == '\\' {
    escapeStart := p.pos   // Record position of the backslash
    p.pos++
    r, err := p.parseEscape(escapeStart)  // Pass source position
    // ...
}
```

For a lone high surrogate in `"\uD800"`, the offset is 1 (the backslash). For a high surrogate followed by a non-low surrogate in `"\uD800\u0041"`, the offset is 7 (the second backslash). This matters for tooling: an editor or diagnostic tool can highlight the exact source token that caused the rejection.

The offset semantics are stable across the failure taxonomy. Parse errors always report byte positions in the original input. CLI errors use offset -1 (not applicable). Bound violations report the start of the bounded element (e.g., the first byte of a too-long number token).

## What Strictness Buys You

A lenient parser makes these implicit decisions:
- "Leading zeros are fine" → Different parsers may interpret `012` as octal 10 or decimal 12
- "Lone surrogates get replaced" → The canonical output of `\uD800` is undefined
- "Duplicate keys use last value" → Or first value, depending on implementation
- "Trailing content is ignored" → `{"a":1}extra` silently becomes `{"a":1}`

A strict parser converts these implicit decisions into explicit rejections. The downstream consumer never has to wonder whether the input was ambiguous. If it parsed, it has exactly one interpretation. If it didn't parse, the failure class and byte offset tell the consumer exactly what went wrong and where.

For infrastructure that depends on deterministic processing, this is the difference between "it works in my tests" and "it works because ambiguous input is structurally excluded."

### Strictness as Error Budget

There's a useful way to think about parser strictness: it's an error budget. A lenient parser spends its error budget on user convenience — accepting malformed input so users don't have to fix their data. A strict parser spends its error budget on correctness guarantees — ensuring every accepted input has exactly one interpretation.

Neither is wrong. They serve different purposes. But when you build infrastructure that sits between systems — processing input from one machine and producing output consumed by another — spending the error budget on convenience is spending it on the wrong consumer. The machine downstream doesn't benefit from leniency. It benefits from guarantees.

The 809 lines of parser code in the `jcstoken` package exist to provide one guarantee: if the parser returns a value, that value has a single, unambiguous, deterministic canonical representation. Every rejection — leading zeros, lone surrogates, duplicate keys, noncharacters, overflow, underflow, negative zero — eliminates a case where two consumers might disagree.

The implementation lives in the `jcstoken` package of [json-canon](https://github.com/nicholasgasior/json-canon), an RFC 8785 JSON canonicalization library.
