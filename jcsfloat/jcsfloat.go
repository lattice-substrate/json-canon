// Package jcsfloat implements the ECMAScript Number::toString algorithm for
// IEEE 754 double-precision floating-point values, as required by RFC 8785.
//
// The algorithm is specified in ECMA-262 §6.1.6.1.20 (Number::toString).
// This is a pure-Go, zero-dependency (standard library only) implementation.
// FormatDouble is validated against large pinned ECMAScript oracle datasets and
// round-trip fuzzing for finite doubles.
//
// The implementation uses math/big.Int for exact multiprecision arithmetic in
// digit generation, following the Burger-Dybvig algorithm with correct ECMA-262
// Note 2 (even-digit) tie-breaking.
package jcsfloat

import (
	"math"
	"math/big"

	"github.com/lattice-substrate/json-canon/jcserr"
)

var bigTen = big.NewInt(10)

// FormatDouble formats an IEEE 754 double-precision value exactly as specified
// by the ECMAScript Number::toString algorithm (ECMA-262, radix 10).
//
// ECMA-FMT-001: NaN returns error.
// ECMA-FMT-002: -0 returns "0".
// ECMA-FMT-003: ±Infinity returns error.
// ECMA-FMT-008: Shortest round-trip representation.
// ECMA-FMT-009: Even-digit tie-breaking.
func FormatDouble(f float64) (string, *jcserr.Error) {
	// ECMA-FMT-001: NaN → error
	if math.IsNaN(f) {
		return "", jcserr.New(jcserr.InvalidGrammar, -1, "NaN is not representable in JSON")
	}
	// ECMA-FMT-002: -0 and +0 → "0"
	if f == 0 {
		return "0", nil
	}
	// ECMA-FMT-003: ±Infinity → error
	if math.IsInf(f, 0) {
		return "", jcserr.New(jcserr.InvalidGrammar, -1, "Infinity is not representable in JSON")
	}

	negative := false
	if f < 0 {
		negative = true
		f = -f
	}

	digits, n := generateDigits(f)
	return formatECMA(negative, digits, n), nil
}

// formatECMA applies the ECMA-262 §6.1.6.1.20 formatting rules (steps 7-10).
//
// digits: significand digit string (k digits)
// n: decimal exponent (number of integer digits in fixed-point view)
func formatECMA(negative bool, digits string, n int) string {
	k := len(digits)

	var buf []byte
	if negative {
		buf = append(buf, '-')
	}

	switch {
	case isIntegerFixed(k, n):
		// ECMA-FMT-004: 1 ≤ n ≤ 21, k ≤ n → integer with trailing zeros
		buf = appendIntegerFixed(buf, digits, k, n)
	case isFractionFixed(n):
		// ECMA-FMT-005: 0 < n ≤ 21, n < k → fixed decimal
		buf = appendFractionFixed(buf, digits, n)
	case isSmallFraction(n):
		// ECMA-FMT-006: -6 < n ≤ 0 → 0.000...digits
		buf = appendSmallFraction(buf, digits, n)
	default:
		// ECMA-FMT-007: exponential notation
		buf = appendExponential(buf, digits, k, n)
	}

	return string(buf)
}

func isIntegerFixed(k, n int) bool {
	return k <= n && n <= 21
}

func isFractionFixed(n int) bool {
	return 0 < n && n <= 21
}

func isSmallFraction(n int) bool {
	return -6 < n && n <= 0
}

func appendIntegerFixed(buf []byte, digits string, k, n int) []byte {
	buf = append(buf, digits...)
	for i := 0; i < n-k; i++ {
		buf = append(buf, '0')
	}
	return buf
}

func appendFractionFixed(buf []byte, digits string, n int) []byte {
	buf = append(buf, digits[:n]...)
	buf = append(buf, '.')
	buf = append(buf, digits[n:]...)
	return buf
}

func appendSmallFraction(buf []byte, digits string, n int) []byte {
	buf = append(buf, '0', '.')
	for i := 0; i < -n; i++ {
		buf = append(buf, '0')
	}
	buf = append(buf, digits...)
	return buf
}

func appendExponential(buf []byte, digits string, k, n int) []byte {
	buf = append(buf, digits[0])
	if k > 1 {
		buf = append(buf, '.')
		buf = append(buf, digits[1:]...)
	}
	buf = append(buf, 'e')
	exp := n - 1
	if exp >= 0 {
		buf = append(buf, '+')
	}
	return appendInt(buf, exp)
}

func appendInt(buf []byte, v int) []byte {
	if v < 0 {
		buf = append(buf, '-')
		v = -v
	}
	if v == 0 {
		return append(buf, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(buf, tmp[i:]...)
}

// generateDigits implements the Burger-Dybvig "free-format" shortest-output
// algorithm using exact big.Int arithmetic, producing the shortest decimal
// significand and its decimal exponent for a positive finite nonzero double.
//
// Returns (digits, n) where value = 0.<digits> × 10^n.
func generateDigits(f float64) (string, int) {
	parts := decodeFloatParts(f)
	state := initScaledState(parts)

	k := estimateK(f)
	scaleByPower10(state, k)

	n := k
	n = applyHighFixup(state, parts.isEven, n)
	n = applyLowFixup(state, parts.isEven, n)

	return extractDigits(state, parts.isEven, n)
}

type floatParts struct {
	mantissa      uint64
	biasedExp     int
	fMant         uint64
	fExp          int
	lowerBoundary bool
	isEven        bool
}

type digitState struct {
	r      *big.Int
	s      *big.Int
	mPlus  *big.Int
	mMinus *big.Int
}

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

func initScaledState(parts floatParts) *digitState {
	state := &digitState{
		r:      new(big.Int),
		s:      new(big.Int),
		mPlus:  new(big.Int),
		mMinus: new(big.Int),
	}
	if parts.fExp >= 0 {
		initScaledPositiveExp(state, parts)
		return state
	}
	initScaledNegativeExp(state, parts)
	return state
}

func initScaledPositiveExp(state *digitState, parts floatParts) {
	if !parts.lowerBoundary {
		state.r.SetUint64(parts.fMant)
		lshByInt(state.r, parts.fExp+1)
		state.s.SetInt64(2)
		state.mPlus.SetInt64(1)
		lshByInt(state.mPlus, parts.fExp)
		state.mMinus.Set(state.mPlus)
		return
	}

	state.r.SetUint64(parts.fMant)
	lshByInt(state.r, parts.fExp+2)
	state.s.SetInt64(4)
	state.mPlus.SetInt64(1)
	lshByInt(state.mPlus, parts.fExp+1)
	state.mMinus.SetInt64(1)
	lshByInt(state.mMinus, parts.fExp)
}

func initScaledNegativeExp(state *digitState, parts floatParts) {
	if !parts.lowerBoundary {
		state.r.SetUint64(parts.fMant)
		lshByInt(state.r, 1)
		state.s.SetInt64(1)
		lshByInt(state.s, -parts.fExp+1)
		state.mPlus.SetInt64(1)
		state.mMinus.SetInt64(1)
		return
	}

	state.r.SetUint64(parts.fMant)
	lshByInt(state.r, 2)
	state.s.SetInt64(1)
	lshByInt(state.s, -parts.fExp+2)
	state.mPlus.SetInt64(2)
	state.mMinus.SetInt64(1)
}

func scaleByPower10(state *digitState, k int) {
	switch {
	case k > 0:
		p := pow10Big(k)
		state.s.Mul(state.s, p)
	case k < 0:
		p := pow10Big(-k)
		state.r.Mul(state.r, p)
		state.mPlus.Mul(state.mPlus, p)
		state.mMinus.Mul(state.mMinus, p)
	}
}

func applyHighFixup(state *digitState, isEven bool, n int) int {
	high := new(big.Int).Add(state.r, state.mPlus)
	if cmpHigh(high, state.s, isEven) {
		state.s.Mul(state.s, bigTen)
		return n + 1
	}
	return n
}

func applyLowFixup(state *digitState, isEven bool, n int) int {
	for {
		tenR := new(big.Int).Mul(state.r, bigTen)
		if !cmpLow(tenR, state.s, isEven) {
			return n
		}

		tenHigh := new(big.Int).Mul(new(big.Int).Add(state.r, state.mPlus), bigTen)
		if !cmpLow(tenHigh, state.s, isEven) {
			return n
		}

		state.r.Mul(state.r, bigTen)
		state.mPlus.Mul(state.mPlus, bigTen)
		state.mMinus.Mul(state.mMinus, bigTen)
		n--
	}
}

func cmpLow(lhs, rhs *big.Int, isEven bool) bool {
	if isEven {
		return lhs.Cmp(rhs) < 0
	}
	return lhs.Cmp(rhs) <= 0
}

func cmpHigh(lhs, rhs *big.Int, isEven bool) bool {
	if isEven {
		return lhs.Cmp(rhs) >= 0
	}
	return lhs.Cmp(rhs) > 0
}

func extractDigits(state *digitState, isEven bool, n int) (string, int) {
	// Safety invariant: digitBuf is 30 bytes.
	//
	// IEEE 754 binary64 has at most 17 significant decimal digits (shortest
	// round-trip representation). The Burger-Dybvig algorithm produces at most
	// 17 digits before termination. normalizeDigitBuffer may expand by 1 byte
	// on carry-out (worst case: 18). ECMA-262 formatting never adds digits
	// beyond the significand. 30 bytes provides >60% headroom above the
	// theoretical maximum of 18.
	//
	// This bound is validated by ECMA-VEC-001/002 (286,362 oracle vectors)
	// and FuzzFormatDoubleRoundTrip.
	var digitBuf [30]byte
	dIdx := 0
	quot := new(big.Int)
	rem := new(big.Int)

	for {
		scaleDigitState(state)
		d := divideAndRemainder(state, quot, rem)

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

func scaleDigitState(state *digitState) {
	state.r.Mul(state.r, bigTen)
	state.mPlus.Mul(state.mPlus, bigTen)
	state.mMinus.Mul(state.mMinus, bigTen)
}

func divideAndRemainder(state *digitState, quot, rem *big.Int) int {
	quot.DivMod(state.r, state.s, rem)
	d := int(quot.Int64())
	state.r.Set(rem)
	return d
}

func terminationConditions(state *digitState, isEven bool) (bool, bool) {
	tc1 := cmpRoundDown(state.r, state.mMinus, isEven)
	high := new(big.Int).Add(state.r, state.mPlus)
	tc2 := cmpHigh(high, state.s, isEven)
	return tc1, tc2
}

func cmpRoundDown(lhs, rhs *big.Int, isEven bool) bool {
	if isEven {
		return lhs.Cmp(rhs) <= 0
	}
	return lhs.Cmp(rhs) < 0
}

// finalDigit resolves the last digit when at least one termination condition
// fires. ECMA-FMT-009: even-digit tie-breaking in midpointDigit.
func finalDigit(d int, tc1, tc2 bool, r, s *big.Int) byte {
	switch {
	case tc1 && !tc2:
		return byte('0' + d)
	case !tc1 && tc2:
		return byte('0' + d + 1)
	default:
		return midpointDigit(d, r, s)
	}
}

func midpointDigit(d int, r, s *big.Int) byte {
	twoR := new(big.Int).Lsh(r, 1)
	cmp := twoR.Cmp(s)
	if cmp < 0 {
		return byte('0' + d)
	}
	if cmp > 0 {
		return byte('0' + d + 1)
	}
	// ECMA-FMT-009: even-digit tie-breaking (banker's rounding)
	if d%2 == 0 {
		return byte('0' + d)
	}
	return byte('0' + d + 1)
}

func normalizeDigitBuffer(digitBuf []byte, dIdx int, dIdxPtr *int, n int) int {
	// Propagate carries from digit overflow (e.g., 9+1=10)
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
		n++
	}

	// Strip trailing zeros
	for dIdx > 1 && digitBuf[dIdx-1] == '0' {
		dIdx--
	}
	*dIdxPtr = dIdx
	return n
}

func exponentBits(bits uint64) uint16 {
	hi := byte((bits >> 56) & 0xFF)
	lo := byte((bits >> 48) & 0xFF)
	return (uint16(hi&0x7F) << 4) | uint16(lo>>4)
}

func lshByInt(z *big.Int, n int) {
	if n <= 0 {
		return
	}
	z.Lsh(z, uint(n))
}

// estimateK returns an estimate of ceil(log10(f)) for f > 0.
func estimateK(f float64) int {
	bits := math.Float64bits(f)
	expBits := exponentBits(bits)
	biasedExp := int(expBits)

	var log2f float64
	if biasedExp == 0 {
		// Subnormal
		log2f = math.Log2(f)
	} else {
		// Normal: f = 1.mantissa × 2^(biasedExp - 1023)
		log2f = float64(biasedExp-1023) + math.Log2(1.0+float64(bits&((1<<52)-1))/float64(uint64(1)<<52))
	}

	k := int(math.Ceil(log2f / math.Log2(10)))
	return k
}

// pow10Cache caches computed powers of 10.
var pow10Cache [700]*big.Int

func init() {
	pow10Cache[0] = big.NewInt(1)
	for i := 1; i < len(pow10Cache); i++ {
		pow10Cache[i] = new(big.Int).Mul(pow10Cache[i-1], bigTen)
	}
}

// pow10Big returns 10^n as a *big.Int. Uses pre-computed cache for n < 700.
// The returned value MUST NOT be mutated by the caller.
func pow10Big(n int) *big.Int {
	if n >= 0 && n < len(pow10Cache) {
		return pow10Cache[n]
	}
	return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}
