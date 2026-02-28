---
title: "Shortest Round-Trip: Implementing IEEE 754 to Decimal Conversion in Go"
published: true
tags: go, programming, algorithms, ieee754
series: "Building Infrastructure-Grade JSON Canonicalization in Go"
---

Every programmer has seen this:

```
0.1 + 0.2 = 0.30000000000000004
```

The joke is that floating-point arithmetic is broken. It isn't. IEEE 754 is doing exactly what it specifies. The problem surfaces when you need to *serialize* these values to text — and when two different systems need to produce *exactly the same text* for the same value.

This is what [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) (JSON Canonicalization Scheme) requires: byte-deterministic JSON output. And the hardest part of that requirement is number formatting. You need the *shortest* decimal string that, when parsed back, recovers the original IEEE 754 bits. You need to agree on tie-breaking when two representations are equally short. And you need to match the exact output format specified by [ECMA-262 Number serialization](https://tc39.es/ecma262/#sec-numeric-types-number-tostring), because that's what RFC 8785 mandates.

Go's `strconv.FormatFloat` is a high-quality shortest-round-trip formatter, but it is not an ECMA-262 conformance contract. So I implemented the [Burger-Dybvig algorithm](https://legacy.cs.indiana.edu/~dyb/pubs/FP-Printing-PLDI96.pdf) from scratch in 490 lines of Go, validated against 286,362 oracle test vectors. This article walks through the entire implementation.

## IEEE 754 Anatomy: 64 Bits of Structure

Before generating digits, you need to understand the raw material. An IEEE 754 double-precision float is 64 bits:

```
[1 bit: sign] [11 bits: biased exponent] [52 bits: mantissa]
```

The value it represents (for normal numbers) is:

```
(-1)^sign × 1.mantissa × 2^(exponent - 1023)
```

The `1.mantissa` is key — normal numbers have an implicit leading 1 bit, giving you 53 bits of precision total. Subnormal numbers (biased exponent = 0) lose the implicit bit, giving `0.mantissa × 2^(-1022)`.

Here's how the implementation extracts these parts:

```go
func decodeFloatParts(f float64) floatParts {
    bits := math.Float64bits(f)
    mantissa := bits & ((uint64(1) << 52) - 1)
    expBits := exponentBits(bits)
    biasedExp := int(expBits)

    fMant := mantissa
    fExp := 1 - 1023 - 52
    if biasedExp != 0 {
        fMant = (uint64(1) << 52) | mantissa
        fExp = biasedExp - 1023 - 52
    }

    lowerBoundary := biasedExp > 1 && mantissa == 0

    return floatParts{
        mantissa:      mantissa,
        biasedExp:     biasedExp,
        fMant:         fMant,
        fExp:          fExp,
        lowerBoundary: lowerBoundary,
        isEven:        fMant%2 == 0,
    }
}
```

Two things to note here. First, the subnormal branch: when `biasedExp == 0`, there's no implicit leading 1, and the effective exponent is fixed at -1074 (so the smallest representable positive value is 2^-1074 ≈ 5 × 10^-324). Second, the `lowerBoundary` flag: when the mantissa is zero and the exponent is above the minimum normal range, the float sits at a power-of-two boundary where the gap to the *lower* adjacent representable is half the gap to the *upper* adjacent. This asymmetry matters for the algorithm.

The exponent extraction itself works on the raw bit pattern:

```go
func exponentBits(bits uint64) uint16 {
    // Equivalent to (bits >> 52) & 0x7FF, but expressed as byte-level
    // extraction to mirror the IEEE 754 layout diagram above.
    hi := byte((bits >> 56) & 0xFF)
    lo := byte((bits >> 48) & 0xFF)
    return (uint16(hi&0x7F) << 4) | uint16(lo>>4)
}
```

This reconstructs the 11-bit biased exponent from the raw byte layout. The conventional one-liner `(bits >> 52) & 0x7FF` is equivalent; the byte-level form is used here to make the bit-field boundaries explicit.

## The Core Insight: Why You Need Multiprecision Arithmetic

The Burger-Dybvig algorithm answers this question: what is the shortest decimal string `d` such that `parse(d) == f` for our original float `f`?

To answer it, you need to compute *exact* boundaries. Every float `f` has two adjacent representable values. The "shortest round-trip" string must map back to `f` and not to either neighbor. This means computing the midpoints between `f` and its neighbors with *exact* arithmetic — not floating-point arithmetic, which would introduce the very imprecision you're trying to eliminate.

The algorithm represents the value and its boundaries as ratios of big integers. For a float with integer significand `fMant` and exponent `fExp`, the value is `fMant × 2^fExp`. The boundaries M- and M+ define the interval within which any decimal representation will round back to `f`.

## State Initialization: R, S, M+, M-

The algorithm works with four multiprecision integers:

- **R** (remainder): represents the current value, scaled into the digit-extraction space
- **S** (scale): the denominator; digits are extracted by dividing R by S
- **M+** (upper margin): how far R can grow before rounding to the next float
- **M-** (lower margin): how far R can shrink before rounding to the previous float

Initialization depends on the exponent sign:

```go
func initScaledPositiveExp(state *digitState, parts floatParts) {
    if !parts.lowerBoundary {
        state.r.SetUint64(parts.fMant)
        lshByInt(state.r, parts.fExp+1)  // r = fMant × 2^(fExp+1)
        state.s.SetInt64(2)               // s = 2
        state.mPlus.SetInt64(1)
        lshByInt(state.mPlus, parts.fExp) // m+ = 2^fExp
        state.mMinus.Set(state.mPlus)     // m- = m+
        return
    }

    // Lower boundary: asymmetric margins
    state.r.SetUint64(parts.fMant)
    lshByInt(state.r, parts.fExp+2)       // r = fMant × 2^(fExp+2)
    state.s.SetInt64(4)                    // s = 4
    state.mPlus.SetInt64(1)
    lshByInt(state.mPlus, parts.fExp+1)   // m+ = 2^(fExp+1)
    state.mMinus.SetInt64(1)
    lshByInt(state.mMinus, parts.fExp)    // m- = 2^fExp (half of m+)
}
```

The `lowerBoundary` case (mantissa is zero, exponent above minimum) requires special handling. At a power-of-two boundary, the float sits at the *top* of one binade and the *bottom* of the next. The gap to the lower neighbor is half the gap to the upper neighbor. The algorithm accounts for this by doubling all values (multiplying R and S by 2) and setting `m+ = 2 × m-`.

For negative exponents, the roles are similar but the scaling direction reverses — instead of left-shifting R, we left-shift S:

```go
func initScaledNegativeExp(state *digitState, parts floatParts) {
    if !parts.lowerBoundary {
        state.r.SetUint64(parts.fMant)
        lshByInt(state.r, 1)               // r = fMant × 2
        state.s.SetInt64(1)
        lshByInt(state.s, -parts.fExp+1)   // s = 2^(-fExp+1)
        state.mPlus.SetInt64(1)            // m+ = 1
        state.mMinus.SetInt64(1)           // m- = 1
        return
    }

    state.r.SetUint64(parts.fMant)
    lshByInt(state.r, 2)                   // r = fMant × 4
    state.s.SetInt64(1)
    lshByInt(state.s, -parts.fExp+2)       // s = 2^(-fExp+2)
    state.mPlus.SetInt64(2)                // m+ = 2
    state.mMinus.SetInt64(1)               // m- = 1
}
```

## Power-of-10 Scaling

After initialization, R, S, M+, and M- are all in base-2 space. To extract decimal digits, we need to scale them so that the first digit comes from `floor(R/S)`. This requires estimating `k ≈ ceil(log10(f))` and scaling accordingly:

```go
func scaleByPower10(state *digitState, k int) {
    switch {
    case k > 0:
        p := pow10Big(k)
        state.s.Mul(state.s, p)           // Scale denominator up
    case k < 0:
        p := pow10Big(-k)
        state.r.Mul(state.r, p)           // Scale numerator up
        state.mPlus.Mul(state.mPlus, p)   // Scale margins too
        state.mMinus.Mul(state.mMinus, p)
    }
}
```

The `k` estimate uses floating-point logarithms and is allowed to be off by one. Two fixup passes correct any estimation error:

```go
func applyHighFixup(state *digitState, isEven bool, n int) int {
    high := new(big.Int).Add(state.r, state.mPlus)
    if cmpHigh(high, state.s, isEven) {
        state.s.Mul(state.s, bigTen)
        return n + 1
    }
    return n
}
```

If `R + M+` already exceeds `S` after initial scaling, we need one more decimal position — multiply S by 10 and increment the exponent count. The low fixup works in the opposite direction, looping while `10R` and `10(R + M+)` are both less than S.

Computing `pow10Big` efficiently matters because these are big integer multiplications. The implementation pre-computes 700 powers of 10 at init time:

```go
var pow10Cache [700]*big.Int

func init() {
    pow10Cache[0] = big.NewInt(1)
    for i := 1; i < len(pow10Cache); i++ {
        pow10Cache[i] = new(big.Int).Mul(pow10Cache[i-1], bigTen)
    }
}

func pow10Big(n int) *big.Int {
    if n >= 0 && n < len(pow10Cache) {
        return new(big.Int).Set(pow10Cache[n])  // Defensive copy
    }
    return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}
```

The defensive copy is critical — without it, callers that mutate the returned `*big.Int` would corrupt the cache. IEEE 754 binary64 ranges from approximately 10^-324 to 10^308 (a span of ~632), so the 700-entry cache covers all practical `k` values with headroom.

## Digit Generation: The Main Loop

With scaling complete, digit extraction is a simple loop. Multiply R, M+, and M- by 10, then divide R by S to get one decimal digit. Repeat until termination:

```go
func extractDigits(state *digitState, isEven bool, n int) (string, int) {
    var digitBuf [30]byte
    dIdx := 0
    quot := new(big.Int)
    rem := new(big.Int)

    for {
        scaleDigitState(state)                              // R, M+, M- *= 10
        d := divideAndRemainder(state, quot, rem)           // d = R / S; R = R mod S

        tc1, tc2 := terminationConditions(state, isEven)
        if !tc1 && !tc2 {
            digitBuf[dIdx] = byte('0' + d)
            dIdx++
            continue
        }

        digitBuf[dIdx] = finalDigit(d, tc1, tc2, state.r, state.s)
        dIdx++
        break
    }

    n = normalizeDigitBuffer(digitBuf[:], dIdx, &dIdx, n)
    return string(digitBuf[:dIdx]), n
}
```

The 30-byte buffer is generous — IEEE 754 binary64 produces at most 17 significant digits in shortest form, with carry propagation adding at most one more.

The two termination conditions test whether we've reached a boundary:

```go
func terminationConditions(state *digitState, isEven bool) (bool, bool) {
    tc1 := cmpRoundDown(state.r, state.mMinus, isEven)
    high := new(big.Int).Add(state.r, state.mPlus)
    tc2 := cmpHigh(high, state.s, isEven)
    return tc1, tc2
}
```

- **tc1** (round down): the remainder R is small enough that rounding down to digit `d` would still land in the correct interval
- **tc2** (round up): the remainder R plus the upper margin M+ meets or exceeds S, meaning rounding up to `d+1` is within range

When neither condition fires, we aren't yet at the shortest representation — emit the digit and continue. When at least one fires, this is the last digit.

## Even-Digit Tie-Breaking

The most subtle part of the algorithm is what happens when both termination conditions fire simultaneously — the value sits at the exact midpoint between two representations. ECMA-262 Note 2 mandates *even-digit* tie-breaking (banker's rounding):

```go
func finalDigit(d int, tc1, tc2 bool, r, s *big.Int) byte {
    switch {
    case tc1 && !tc2:
        return byte('0' + d)      // Only round-down applies: use d
    case !tc1 && tc2:
        return byte('0' + d + 1)  // Only round-up applies: use d+1
    default:
        return midpointDigit(d, r, s)  // Both apply: tie-break
    }
}

func midpointDigit(d int, r, s *big.Int) byte {
    twoR := new(big.Int).Lsh(r, 1)
    cmp := twoR.Cmp(s)
    if cmp < 0 {
        return byte('0' + d)      // Closer to lower: round down
    }
    if cmp > 0 {
        return byte('0' + d + 1)  // Closer to upper: round up
    }
    // Exactly at midpoint: even-digit tie-breaking
    if d%2 == 0 {
        return byte('0' + d)      // d is even: keep it
    }
    return byte('0' + d + 1)      // d is odd: round up to even
}
```

The midpoint test compares `2R` against `S`. If `2R < S`, the remainder is less than half the scale — round down. If `2R > S`, round up. When `2R == S` exactly, the algorithm breaks the tie by choosing the even digit.

This even-digit preference also influences the boundary comparisons themselves. The `cmpRoundDown`, `cmpHigh`, and `cmpLow` functions use different comparison operators depending on whether the mantissa is even or odd:

```go
func cmpRoundDown(lhs, rhs *big.Int, isEven bool) bool {
    if isEven {
        return lhs.Cmp(rhs) <= 0   // Inclusive: allow rounding to even at boundary
    }
    return lhs.Cmp(rhs) < 0        // Strict: odd mantissa
}

func cmpHigh(lhs, rhs *big.Int, isEven bool) bool {
    if isEven {
        return lhs.Cmp(rhs) >= 0   // Inclusive
    }
    return lhs.Cmp(rhs) > 0        // Strict
}
```

This asymmetry is the difference between ECMA-262 compliance and "close enough." When the mantissa is even, the boundary comparisons are inclusive — they allow termination at exact boundary values, creating the conditions for even-digit tie-breaking. When odd, strict comparisons ensure the algorithm doesn't terminate prematurely.

## Carry Propagation

When `finalDigit` returns `d+1`, it may produce a 10 (the character after '9'). This overflow must propagate:

```go
func normalizeDigitBuffer(digitBuf []byte, dIdx int, dIdxPtr *int, n int) int {
    // Propagate carries
    for i := dIdx - 1; i > 0; i-- {
        if digitBuf[i] > '9' {
            digitBuf[i] = '0'
            digitBuf[i-1]++
        }
    }

    // Handle carry out of the first digit
    if dIdx > 0 && digitBuf[0] > '9' {
        copy(digitBuf[1:dIdx+1], digitBuf[0:dIdx])
        digitBuf[0] = '1'
        digitBuf[1] = '0'
        dIdx++
        n++    // One more integer digit
    }

    // Strip trailing zeros
    for dIdx > 1 && digitBuf[dIdx-1] == '0' {
        dIdx--
    }
    *dIdxPtr = dIdx
    return n
}
```

Consider a carry cascade: digits `[9, 9, 10]` become `[9, 10, 0]` then `[10, 0, 0]`, and finally the carry-out case shifts everything right to produce `[1, 0, 0, 0]` with an incremented exponent. Trailing zeros are then stripped since the algorithm produces shortest representations.

## ECMA-262 Output Formatting

The Burger-Dybvig algorithm produces a digit string and an exponent `n`. The ECMA-262 specification (§6.1.6.1.20) defines four formatting branches based on `n`:

```go
func formatECMA(negative bool, digits string, n int) string {
    k := len(digits)

    var buf []byte
    if negative {
        buf = append(buf, '-')
    }

    switch {
    case k <= n && n <= 21:
        // Integer fixed: "12300" (digits + trailing zeros)
        buf = appendIntegerFixed(buf, digits, k, n)
    case 0 < n && n <= 21:
        // Fraction fixed: "12.345" (decimal point within digits)
        buf = appendFractionFixed(buf, digits, n)
    case -6 < n && n <= 0:
        // Small fraction: "0.00123" (leading zeros after decimal point)
        buf = appendSmallFraction(buf, digits, n)
    default:
        // Exponential: "1.23e+20" or "1e-7"
        buf = appendExponential(buf, digits, k, n)
    }

    return string(buf)
}
```

The boundary constants come directly from the ECMA-262 specification:

| Branch | Condition | Example Input | Output |
|--------|-----------|---------------|--------|
| Integer fixed | `k ≤ n ≤ 21` | `1e20` | `100000000000000000000` |
| Fraction fixed | `0 < n ≤ 21, n < k` | `1.5` | `1.5` |
| Small fraction | `-6 < n ≤ 0` | `1e-6` | `0.000001` |
| Exponential | otherwise | `1e21` | `1e+21` |

The boundaries are exact. A value of exactly 10^21 uses exponential notation (`1e+21`), while 999999999999999900000 (just below 10^21) uses integer fixed notation. A value of exactly 10^-6 uses small fraction notation (`0.000001`), while anything below that switches to exponential.

These boundary behaviors can be verified against specific bit patterns:

```
0x444b1ae4d6e2ef50 → "1e+21"                        (exponential)
0x444b1ae4d6e2ef4f → "999999999999999900000"         (integer fixed)
0x3eb0c6f7a0b5ed8d → "0.000001"                      (small fraction)
0x3eb0c6f7a0b5ed8c → "9.999999999999997e-7"          (exponential)
```

## Why strconv.FormatFloat Is Insufficient

Go's `strconv.FormatFloat` is the standard library's number-to-string conversion. With format `'e'`, `'f'`, or `'g'` and precision `-1`, it produces shortest-round-trip representations. The issue for JCS is not quality; the issue is contract mismatch. RFC 8785 requires ECMAScript-compatible rendering rules, and `FormatFloat` does not expose that contract.

The gaps are concrete.

**Format-policy divergence.** ECMA-262 and `FormatFloat` make different notation choices at key boundaries. Examples:

| Value | ECMA-262 / RFC 8785 expected | `FormatFloat(v, 'g', -1, 64)` |
|-------|-------------------------------|--------------------------------|
| `1e20` | `100000000000000000000` | `1e+20` |
| `1e-6` | `0.000001` | `1e-06` |
| `1000000` | `1000000` | `1e+06` |

These are all valid shortest representations, but only one side matches the JCS contract (see RFC 8785's ECMAScript-compatible serialization examples in Appendix B).

These three examples are sufficient to establish the core point: ECMA-262 rendering switches policy across multiple exponent ranges, while any single `FormatFloat` mode has one fixed rendering policy. Once boundary cases disagree, conformance requires a dedicated ECMA-262 formatting layer.

**Single-mode mismatch.** ECMA-262 output policy spans multiple branches (fixed integer, fixed fraction, small fraction, exponential) with exact thresholds. No single `FormatFloat` mode (`'e'`, `'f'`, or `'g'`) reproduces all branches across the full range.

**Upstream algorithm drift risk.** Go's shortest-mode internals are not static over time. As of **Go 1.26.0 (February 2026)**, shortest mode in [`internal/strconv/ftoa.go`](https://go.googlesource.com/go/+/refs/tags/go1.26.0/src/internal/strconv/ftoa.go) explicitly says "Use the Dragonbox algorithm" and calls `dboxFtoa`; the Dragonbox implementation in [`ftoadbox.go`](https://go.googlesource.com/go/+/refs/tags/go1.26.0/src/internal/strconv/ftoadbox.go) states round-to-nearest, ties-to-even behavior. Older Go releases used different internals. That evolution is normal for a standard library, but it is exactly why canonicalization code should not outsource its normative behavior to implementation details of an external runtime.

Building from scratch eliminates these issues. The Burger-Dybvig path always uses exact multiprecision arithmetic, applies explicit midpoint handling, and formats according to ECMA-262 branch rules. The cost is performance — multiprecision arithmetic is meaningfully slower than `strconv.FormatFloat`'s optimized Dragonbox path — but for canonicalization, correctness is the constraint. Benchmarks and performance analysis are covered in a later article in this series.

## Special Values

Three special cases are handled before the algorithm runs:

```go
func FormatDouble(f float64) (string, *jcserr.Error) {
    if math.IsNaN(f) {
        return "", jcserr.New(jcserr.InvalidGrammar, -1,
            "NaN is not representable in JSON")
    }
    if f == 0 {
        return "0", nil    // Both +0 and -0 produce "0"
    }
    if math.IsInf(f, 0) {
        return "", jcserr.New(jcserr.InvalidGrammar, -1,
            "Infinity is not representable in JSON")
    }
    // ... proceed with Burger-Dybvig
}
```

NaN and Infinity are errors — JSON has no representation for them. Negative zero is normalized to `"0"` because IEEE 754's -0 and +0 are *mathematically* equal, and canonical JSON should not distinguish them. (Note: lexical `-0` in JSON *input* is a separate concern. RFC 8259 permits it, but json-canon rejects it as a library policy violation to enforce stricter numeric hygiene — the two requirements are independent.)

## Validation: 286,362 Oracle Vectors

The implementation is validated against two pinned oracle datasets:

- **golden_vectors.csv**: 54,445 test vectors covering boundary values, powers of 10, subnormals, and edge cases
- **golden_stress_vectors.csv**: 231,917 stress test vectors targeting tie-breaking and carry propagation scenarios

Each CSV row contains a 16-character hex encoding of the IEEE 754 bits and the expected output string:

```
0000000000000000,0
0000000000000001,5e-324
3ff0000000000000,1
3eb0c6f7a0b5ed8d,0.000001
```

The test function validates three properties per dataset:

1. **Semantic correctness**: `FormatDouble(bits) == expected` for every row
2. **Cardinality**: The exact row count matches expectations (catches truncation)
3. **Integrity**: SHA-256 of the entire file matches a pinned hash (catches corruption)

```go
func TestGoldenOracle(t *testing.T) {
    verifyOracle(t, "testdata/golden_vectors.csv", 54445,
        "593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636")
}

func TestStressOracle(t *testing.T) {
    verifyOracle(t, "testdata/golden_stress_vectors.csv", 231917,
        "287d21ac87e5665550f1baf86038302a0afc67a74a020dffb872f1a93b26d410")
}
```

The SHA-256 checksums pin the test data against silent modification. If someone edits a single byte in either file, the test fails.

## Round-Trip and Fuzz Testing

Beyond oracle vectors, the implementation uses two additional validation strategies.

**Round-trip testing** verifies the shortest representation property directly: format a value, parse it back with `strconv.ParseFloat`, and confirm the original bits are recovered:

```go
cases := []float64{5e-324, 1e-7, 1e-6, 0.1, 0.2, 1.1, 1, 2, 1e20, 1e21, math.MaxFloat64}
for _, c := range cases {
    formatted, _ := jcsfloat.FormatDouble(c)
    parsed, _ := strconv.ParseFloat(formatted, 64)
    if parsed != c {
        t.Fatalf("round-trip failed for %.17g: formatted %q, parsed back as %.17g",
            c, formatted, parsed)
    }
}
```

**Fuzz testing** generates random 64-bit patterns, interprets them as IEEE 754 doubles, and verifies round-trip stability:

```go
func FuzzFormatDoubleRoundTrip(f *testing.F) {
    f.Fuzz(func(t *testing.T, data []byte) {
        if len(data) < 8 { return }
        bits := binary.BigEndian.Uint64(data[:8])
        fval := math.Float64frombits(bits)

        if math.IsNaN(fval) || math.IsInf(fval, 0) {
            _, err := jcsfloat.FormatDouble(fval)
            if err == nil { t.Fatal("expected error for non-finite") }
            return
        }

        s, err := jcsfloat.FormatDouble(fval)
        if err != nil { t.Fatalf("unexpected error: %v", err) }

        parsed, _ := strconv.ParseFloat(s, 64)
        if fval == 0 {
            if parsed != 0 { t.Fatal("zero round-trip failed") }
            return
        }
        if math.Float64bits(parsed) != math.Float64bits(fval) {
            t.Fatalf("round-trip failed: bits=%016x → %q → bits=%016x",
                bits, s, math.Float64bits(parsed))
        }
    })
}
```

Note the special case for zero: IEEE 754 -0 formats as `"0"`, which parses back as +0. The bit patterns differ (0x8000000000000000 vs 0x0000000000000000), but mathematically the values are equal, so the round-trip comparison uses `== 0` instead of bit equality.

**Idempotency testing** verifies that format → parse → format produces the same string, which is a consequence of correct tie-breaking:

```go
for i := uint64(1); i < 5000; i += 97 {
    v := math.Float64frombits(i * 0x9e3779b97f4a7c15)
    if math.IsNaN(v) || math.IsInf(v, 0) { continue }

    f1, _ := jcsfloat.FormatDouble(v)
    parsed, _ := strconv.ParseFloat(f1, 64)
    f2, _ := jcsfloat.FormatDouble(parsed)

    if f1 != f2 {
        t.Fatalf("idempotency failed for bits=%016x: %s != %s",
            math.Float64bits(v), f1, f2)
    }
}
```

If tie-breaking were inconsistent, formatting a value at the exact midpoint could oscillate between two representations. Even-digit tie-breaking prevents this by always choosing the same direction at midpoints.

## The Complete Pipeline

Putting it all together, the conversion from IEEE 754 bits to canonical decimal string follows this path:

1. **Decode** the 64-bit pattern into mantissa, exponent, and boundary flags
2. **Initialize** R, S, M+, M- as big integers with appropriate scaling
3. **Estimate** the decimal exponent `k` using floating-point logarithms
4. **Scale** by 10^k using pre-computed powers from the 700-entry cache
5. **Fix up** the scaling if the estimate was off by one (high or low)
6. **Extract digits** one at a time by multiplying by 10 and dividing
7. **Terminate** when boundary conditions indicate the shortest representation
8. **Tie-break** using even-digit preference when at the exact midpoint
9. **Propagate carries** if rounding up causes overflow
10. **Format** using the appropriate ECMA-262 branch based on the exponent

The entire implementation is 490 lines of Go with zero external dependencies (only `math`, `math/big`, and the project's own error package). It is deterministic, locale-independent, and produces identical output regardless of platform or Go runtime version.

The implementation lives in the `jcsfloat` package of [json-canon](https://github.com/lattice-substrate/json-canon), an RFC 8785 JSON canonicalization library.
