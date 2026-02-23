package jcsfloat_test

import (
	"encoding/binary"
	"math"
	"strconv"
	"testing"

	"github.com/SolutionsExcite/json-canon/jcsfloat"
)

// FuzzFormatDoubleRoundTrip: uint64 bits → format → parse → verify round-trip.
func FuzzFormatDoubleRoundTrip(f *testing.F) {
	seeds := []uint64{
		0x0000000000000000, // +0
		0x8000000000000000, // -0
		0x0000000000000001, // MIN_VALUE
		0x7fefffffffffffff, // MAX_VALUE
		0x3ff0000000000000, // 1.0
		0x444b1ae4d6e2ef50, // 1e21
		0x3eb0c6f7a0b5ed8d, // 1e-6
	}
	for _, s := range seeds {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, s)
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 8 {
			return
		}
		bits := binary.BigEndian.Uint64(data[:8])
		fval := math.Float64frombits(bits)

		if math.IsNaN(fval) || math.IsInf(fval, 0) {
			// These should error
			_, err := jcsfloat.FormatDouble(fval)
			if err == nil {
				t.Fatal("expected error for non-finite value")
			}
			return
		}

		s, err := jcsfloat.FormatDouble(fval)
		if err != nil {
			t.Fatalf("FormatDouble(bits=%016x): %v", bits, err)
		}

		parsed, parseErr := strconv.ParseFloat(s, 64)
		if parseErr != nil {
			t.Fatalf("ParseFloat(%q): %v", s, parseErr)
		}

		// ECMA-FMT-002: -0 serializes as "0", which parses back as +0.
		// Bit-level identity is not expected for -0.
		if fval == 0 {
			if parsed != 0 {
				t.Fatalf("zero round-trip failed: bits=%016x → %q → %v", bits, s, parsed)
			}
			return
		}
		if math.Float64bits(parsed) != math.Float64bits(fval) {
			t.Fatalf("round-trip failed: bits=%016x → %q → bits=%016x",
				bits, s, math.Float64bits(parsed))
		}
	})
}
