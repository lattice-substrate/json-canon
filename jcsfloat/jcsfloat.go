// Package jcsfloat implements the ECMAScript Number::toString algorithm for
// IEEE 754 double-precision floating-point values, as required by RFC 8785.
//
// The algorithm is specified in ECMA-262 as Number::toString (historically
// §7.1.12.1 in ES2015/6th Edition; §6.1.6.1.20 in the 2026 living standard).
// The section number varies by edition; the algorithm content is normative.
//
// This is a pure-Go, zero-dependency (standard library only) implementation
// suitable for use in JCS (RFC 8785) canonicalization. The output of FormatDouble
// is byte-identical to ECMAScript's String(number) for all finite doubles.
//
// The implementation uses math/big.Int for exact multiprecision arithmetic in
// digit generation, following the Burger-Dybvig algorithm with correct ECMA-262
// Note 2 (even-digit) tie-breaking.
package jcsfloat

import (
	"errors"
	"math"
	"math/big"
)

var (
	ErrNotFinite = errors.New("jcsfloat: value is not finite (NaN or Infinity)")

	bigOne = big.NewInt(1)
	bigTwo = big.NewInt(2)
	bigTen = big.NewInt(10)
)

// FormatDouble formats an IEEE 754 double-precision value exactly as specified
// by the ECMAScript Number::toString algorithm (ECMA-262, radix 10). The output
// is the canonical string representation required by RFC 8785 JCS.
//
// Special cases:
//   - Negative zero (-0) returns "0".
//   - NaN and ±Infinity return an error (ErrNotFinite).
//
// For all finite doubles, the output is byte-identical to JavaScript's String(x).
func FormatDouble(f float64) (string, error) {
	if math.IsNaN(f) {
		return "", ErrNotFinite
	}
	if f == 0 {
		return "0", nil
	}
	if math.IsInf(f, 0) {
		return "", ErrNotFinite
	}

	negative := false
	if f < 0 {
		negative = true
		f = -f
	}

	digits, n := generateDigits(f)
	return formatECMA(negative, digits, n), nil
}

// formatECMA applies the ECMA-262 §7.1.12.1 formatting rules (steps 6-9).
//
// digits: significand digit string
// n: decimal exponent (number of integer digits in fixed-point)
func formatECMA(negative bool, digits string, n int) string {
	k := len(digits)

	var buf []byte
	if negative {
		buf = append(buf, '-')
	}

	if k <= n && n <= 21 {
		// Step 6: integer with trailing zeros
		buf = append(buf, digits...)
		for i := 0; i < n-k; i++ {
			buf = append(buf, '0')
		}
	} else if 0 < n && n <= 21 {
		// Step 7: fixed-point, decimal within digits (n < k)
		buf = append(buf, digits[:n]...)
		buf = append(buf, '.')
		buf = append(buf, digits[n:]...)
	} else if -6 < n && n <= 0 {
		// Step 8: 0.000...digits
		buf = append(buf, '0', '.')
		for i := 0; i < -n; i++ {
			buf = append(buf, '0')
		}
		buf = append(buf, digits...)
	} else {
		// Step 9: exponential notation
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
		buf = appendInt(buf, exp)
	}

	return string(buf)
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
// Returns (digits, n) where value = 0.<digits> * 10^n.
func generateDigits(f float64) (string, int) {
	bits := math.Float64bits(f)
	mantissa := bits & ((1 << 52) - 1)
	biasedExp := int((bits >> 52) & 0x7FF)

	var fMant uint64
	var fExp int
	if biasedExp == 0 {
		fMant = mantissa
		fExp = 1 - 1023 - 52 // -1074
	} else {
		fMant = (1 << 52) | mantissa
		fExp = biasedExp - 1023 - 52
	}

	// Is f at the lower boundary of a binade?
	// True when mantissa bits are all zero (fMant = 2^52 for normals)
	// and there exists a lower binade (biasedExp > 1).
	// At a lower boundary, the gap below is smaller than the gap above.
	lowerBoundary := biasedExp > 1 && mantissa == 0

	// For round-to-nearest-even, boundaries are inclusive when fMant is even.
	isEven := fMant%2 == 0

	// Burger-Dybvig scaled integers r, s, mPlus, mMinus such that:
	//   r/s = f
	//   mPlus/s = distance from f to upper boundary midpoint
	//   mMinus/s = distance from f to lower boundary midpoint
	r := new(big.Int)
	s := new(big.Int)
	mPlus := new(big.Int)
	mMinus := new(big.Int)

	if fExp >= 0 {
		be := uint(fExp)
		if !lowerBoundary {
			r.SetUint64(fMant)
			r.Lsh(r, be+1)
			s.SetInt64(2)
			mPlus.SetInt64(1)
			mPlus.Lsh(mPlus, be)
			mMinus.Set(mPlus)
		} else {
			r.SetUint64(fMant)
			r.Lsh(r, be+2)
			s.SetInt64(4)
			mPlus.SetInt64(1)
			mPlus.Lsh(mPlus, be+1)
			mMinus.SetInt64(1)
			mMinus.Lsh(mMinus, be)
		}
	} else {
		nbe := uint(-fExp)
		if !lowerBoundary {
			r.SetUint64(fMant)
			r.Lsh(r, 1)
			s.SetInt64(1)
			s.Lsh(s, nbe+1)
			mPlus.SetInt64(1)
			mMinus.SetInt64(1)
		} else {
			r.SetUint64(fMant)
			r.Lsh(r, 2)
			s.SetInt64(1)
			s.Lsh(s, nbe+2)
			mPlus.SetInt64(2)
			mMinus.SetInt64(1)
		}
	}

	// Estimate decimal exponent: k ≈ ceil(log10(f))
	// This is an estimate; the fixup loop below corrects it.
	k := estimateK(f)

	// Scale by 10^k: if k >= 0, multiply s; if k < 0, multiply r and m's.
	if k > 0 {
		p := pow10Big(k)
		s.Mul(s, p)
	} else if k < 0 {
		p := pow10Big(-k)
		r.Mul(r, p)
		mPlus.Mul(mPlus, p)
		mMinus.Mul(mMinus, p)
	}

	// n is the ECMA "decimal exponent": the position of the decimal point.
	// After scaling by 10^k, we have r/s ≈ f/10^k.
	// The digit extraction produces digits d1 d2 ... and n = k tells us
	// where the decimal point goes.
	n := k

	// Fixup: ensure the first digit will be in [1,9].
	// Check the "high" condition first: can we round up to the next power of 10?
	// The upper bound of the interval is (r + mPlus) / s.
	// If this >= 1, our first digit might be 10 (overflow), so multiply s by 10.
	{
		high := new(big.Int).Add(r, mPlus)
		if isEven {
			if high.Cmp(s) >= 0 {
				s.Mul(s, bigTen)
				n++
			}
		} else {
			if high.Cmp(s) > 0 {
				s.Mul(s, bigTen)
				n++
			}
		}
	}

	// Check the "low" condition: is the first digit going to be 0?
	// If 10*r < s (or <= for non-even), the leading digit is 0.
	for {
		tenR := new(big.Int).Mul(r, bigTen)
		low := false
		if isEven {
			low = tenR.Cmp(s) < 0
		} else {
			low = tenR.Cmp(s) <= 0
		}
		// Also check upper bound
		if low {
			tenHigh := new(big.Int).Mul(new(big.Int).Add(r, mPlus), bigTen)
			highOk := false
			if isEven {
				highOk = tenHigh.Cmp(s) < 0
			} else {
				highOk = tenHigh.Cmp(s) <= 0
			}
			if highOk {
				// First digit is 0 and we can't round up past it
				r.Mul(r, bigTen)
				mPlus.Mul(mPlus, bigTen)
				mMinus.Mul(mMinus, bigTen)
				n--
				continue
			}
		}
		break
	}

	// Digit extraction loop
	var digitBuf [30]byte
	dIdx := 0
	quot := new(big.Int)
	rem := new(big.Int)

	for {
		// Multiply r, mPlus, mMinus by 10
		r.Mul(r, bigTen)
		mPlus.Mul(mPlus, bigTen)
		mMinus.Mul(mMinus, bigTen)

		// digit = floor(r / s), r = r mod s
		quot.DivMod(r, s, rem)
		d := int(quot.Int64())
		r.Set(rem)

		// Termination: can we round down? can we round up?
		var tc1, tc2 bool
		if isEven {
			tc1 = r.Cmp(mMinus) <= 0
		} else {
			tc1 = r.Cmp(mMinus) < 0
		}
		{
			high := new(big.Int).Add(r, mPlus)
			if isEven {
				tc2 = high.Cmp(s) >= 0
			} else {
				tc2 = high.Cmp(s) > 0
			}
		}

		if !tc1 && !tc2 {
			// Not done yet
			digitBuf[dIdx] = byte('0' + d)
			dIdx++
			continue
		}

		if tc1 && !tc2 {
			// Round down
			digitBuf[dIdx] = byte('0' + d)
			dIdx++
			break
		}

		if !tc1 && tc2 {
			// Round up
			digitBuf[dIdx] = byte('0' + d + 1)
			dIdx++
			break
		}

		// Both: compare 2*r with s for midpoint
		twoR := new(big.Int).Lsh(r, 1)
		cmp := twoR.Cmp(s)
		if cmp < 0 {
			digitBuf[dIdx] = byte('0' + d)
		} else if cmp > 0 {
			digitBuf[dIdx] = byte('0' + d + 1)
		} else {
			// Exact midpoint: ECMA Note 2 — even digit
			if d%2 == 0 {
				digitBuf[dIdx] = byte('0' + d)
			} else {
				digitBuf[dIdx] = byte('0' + d + 1)
			}
		}
		dIdx++
		break
	}

	// Carry propagation
	for i := dIdx - 1; i > 0; i-- {
		if digitBuf[i] > '9' {
			digitBuf[i] = '0'
			digitBuf[i-1]++
		}
	}
	if dIdx > 0 && digitBuf[0] > '9' {
		copy(digitBuf[1:dIdx+1], digitBuf[0:dIdx])
		digitBuf[0] = '1'
		digitBuf[1] = '0'
		dIdx++
		n++
	}

	// Strip trailing zeros (can occur from carry propagation)
	for dIdx > 1 && digitBuf[dIdx-1] == '0' {
		dIdx--
	}

	return string(digitBuf[:dIdx]), n
}

// estimateK returns an estimate of ceil(log10(f)) for f > 0.
func estimateK(f float64) int {
	// log10(f) = log2(f) / log2(10)
	// Use the bit representation for a quick estimate.
	bits := math.Float64bits(f)
	biasedExp := int((bits >> 52) & 0x7FF)

	var log2f float64
	if biasedExp == 0 {
		// Subnormal
		log2f = math.Log2(f)
	} else {
		// Normal: f = 1.mantissa * 2^(biasedExp - 1023)
		log2f = float64(biasedExp-1023) + math.Log2(1.0+float64(bits&((1<<52)-1))/float64(uint64(1)<<52))
	}

	k := int(math.Ceil(log2f / math.Log2(10)))
	return k
}

// pow10Cache caches computed powers of 10.
var pow10Cache [700]*big.Int

func init() {
	// Pre-compute powers of 10 for the range we need.
	// IEEE 754 doubles have exponents from roughly -1074 to +308,
	// so we need 10^0 through 10^~700 at most.
	pow10Cache[0] = big.NewInt(1)
	for i := 1; i < len(pow10Cache); i++ {
		pow10Cache[i] = new(big.Int).Mul(pow10Cache[i-1], bigTen)
	}
}

// pow10Big returns 10^n as a *big.Int. Uses a pre-computed cache for n < 700.
// The returned value MUST NOT be mutated by the caller.
func pow10Big(n int) *big.Int {
	if n >= 0 && n < len(pow10Cache) {
		return pow10Cache[n]
	}
	return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}
