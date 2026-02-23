package jcsfloat_test

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/SolutionsExcite/json-canon/jcsfloat"
)

// === ECMA-FMT-001: NaN rejected ===

func TestFormatDouble_ECMA_FMT_001(t *testing.T) {
	_, err := jcsfloat.FormatDouble(math.NaN())
	if err == nil {
		t.Fatal("expected error for NaN")
	}
}

// === ECMA-FMT-002: -0 → "0" ===

func TestFormatDouble_ECMA_FMT_002(t *testing.T) {
	got, err := jcsfloat.FormatDouble(math.Copysign(0, -1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0" {
		t.Fatalf("got %q want %q", got, "0")
	}
	// Also verify +0
	got, err = jcsfloat.FormatDouble(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0" {
		t.Fatalf("got %q want %q", got, "0")
	}
}

// === ECMA-FMT-003: ±Infinity rejected ===

func TestFormatDouble_ECMA_FMT_003(t *testing.T) {
	for _, v := range []float64{math.Inf(+1), math.Inf(-1)} {
		_, err := jcsfloat.FormatDouble(v)
		if err == nil {
			t.Fatalf("expected error for %v", v)
		}
	}
}

// === ECMA-FMT-004: integer fixed (k ≤ n ≤ 21) ===

func TestFormatDouble_ECMA_FMT_004(t *testing.T) {
	cases := []struct {
		input float64
		want  string
	}{
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{1e15, "1000000000000000"},
		{1e20, "100000000000000000000"},
	}
	for _, tc := range cases {
		got, err := jcsfloat.FormatDouble(tc.input)
		if err != nil {
			t.Fatalf("FormatDouble(%v): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("FormatDouble(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// === ECMA-FMT-005: fraction fixed (0 < n ≤ 21, n < k) ===

func TestFormatDouble_ECMA_FMT_005(t *testing.T) {
	cases := []struct {
		input float64
		want  string
	}{
		{0.5, "0.5"},
		{1.5, "1.5"},
		{1.2345678901234567, "1.2345678901234567"},
	}
	for _, tc := range cases {
		got, err := jcsfloat.FormatDouble(tc.input)
		if err != nil {
			t.Fatalf("FormatDouble(%v): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("FormatDouble(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// === ECMA-FMT-006: small fraction (-6 < n ≤ 0) ===

func TestFormatDouble_ECMA_FMT_006(t *testing.T) {
	got, err := jcsfloat.FormatDouble(0.000001)
	if err != nil {
		t.Fatalf("FormatDouble(0.000001): %v", err)
	}
	if got != "0.000001" {
		t.Fatalf("got %q want %q", got, "0.000001")
	}
}

// === ECMA-FMT-007: exponential notation ===

func TestFormatDouble_ECMA_FMT_007(t *testing.T) {
	cases := []struct {
		input float64
		want  string
	}{
		{1e21, "1e+21"},
		{1e-7, "1e-7"},
		{math.MaxFloat64, "1.7976931348623157e+308"},
	}
	for _, tc := range cases {
		got, err := jcsfloat.FormatDouble(tc.input)
		if err != nil {
			t.Fatalf("FormatDouble(%v): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("FormatDouble(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// === ECMA-FMT-008: shortest round-trip representation ===

func TestFormatDouble_ECMA_FMT_008(t *testing.T) {
	cases := []float64{5e-324, 1e-7, 1e-6, 0.1, 0.2, 1.1, 1, 2, 1e20, 1e21, math.MaxFloat64}
	for _, c := range cases {
		f1, err := jcsfloat.FormatDouble(c)
		if err != nil {
			t.Fatalf("format(%.17g): %v", c, err)
		}
		v, parseErr := strconv.ParseFloat(f1, 64)
		if parseErr != nil {
			t.Fatalf("parse %q: %v", f1, parseErr)
		}
		if v != c {
			t.Fatalf("round-trip failed for %.17g: formatted %q, parsed back as %.17g", c, f1, v)
		}
	}
}

// === ECMA-FMT-009: even-digit tie-breaking (banker's rounding) ===

func TestFormatDouble_ECMA_FMT_009(t *testing.T) {
	// Idempotency is a consequence of correct tie-breaking
	for i := uint64(1); i < 5000; i += 97 {
		v := math.Float64frombits(i * 0x9e3779b97f4a7c15)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		f1, err := jcsfloat.FormatDouble(v)
		if err != nil {
			t.Fatalf("format bits=%016x: %v", math.Float64bits(v), err)
		}
		parsed, parseErr := strconv.ParseFloat(f1, 64)
		if parseErr != nil {
			t.Fatalf("parse bits=%016x text=%q: %v", math.Float64bits(v), f1, parseErr)
		}
		f2, err := jcsfloat.FormatDouble(parsed)
		if err != nil {
			t.Fatalf("re-format bits=%016x: %v", math.Float64bits(v), err)
		}
		if f1 != f2 {
			t.Fatalf("round-trip mismatch bits=%016x: %s != %s", math.Float64bits(v), f1, f2)
		}
	}
}

// === ECMA-FMT-010: negative numbers serialize with leading '-' (step 3) ===

func TestFormatDouble_ECMA_FMT_010(t *testing.T) {
	got, err := jcsfloat.FormatDouble(-12.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "-12.5" {
		t.Fatalf("got %q want %q", got, "-12.5")
	}
}

// === ECMA-FMT-011: choose minimal k (step 5 shortest representation basis) ===

func TestFormatDouble_ECMA_FMT_011(t *testing.T) {
	got, err := jcsfloat.FormatDouble(1.2300000000000002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2300000000000002" {
		t.Fatalf("got %q", got)
	}
}

// === ECMA-FMT-012: scientific format k=1 omits decimal point (step 10 branch) ===

func TestFormatDouble_ECMA_FMT_012(t *testing.T) {
	got, err := jcsfloat.FormatDouble(1e21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1e+21" {
		t.Fatalf("got %q want %q", got, "1e+21")
	}
}

// === ECMA-VEC-001: base golden oracle ===

func TestGoldenOracle(t *testing.T) {
	verifyOracle(t, "testdata/golden_vectors.csv", 54445,
		"593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636")
}

// === ECMA-VEC-002: stress golden oracle ===

func TestStressOracle(t *testing.T) {
	verifyOracle(t, "testdata/golden_stress_vectors.csv", 231917,
		"287d21ac87e5665550f1baf86038302a0afc67a74a020dffb872f1a93b26d410")
}

// === ECMA-VEC-003: boundary constants ===

func TestBoundaryConstants(t *testing.T) {
	cases := []struct {
		bits uint64
		want string
	}{
		{0x0000000000000000, "0"},                        // +0
		{0x8000000000000000, "0"},                        // -0
		{0x0000000000000001, "5e-324"},                   // MIN_VALUE
		{0x7fefffffffffffff, "1.7976931348623157e+308"},  // MAX_VALUE
		{0x3eb0c6f7a0b5ed8d, "0.000001"},                 // 1e-6 boundary
		{0x3eb0c6f7a0b5ed8c, "9.999999999999997e-7"},     // just below
		{0x3eb0c6f7a0b5ed8e, "0.0000010000000000000002"}, // just above
		{0x444b1ae4d6e2ef50, "1e+21"},                    // 1e21 boundary
		{0x444b1ae4d6e2ef4f, "999999999999999900000"},    // just below
		{0x444b1ae4d6e2ef51, "1.0000000000000001e+21"},   // just above
	}
	for _, tc := range cases {
		got, err := jcsfloat.FormatDouble(math.Float64frombits(tc.bits))
		if err != nil {
			t.Fatalf("format bits=%016x: %v", tc.bits, err)
		}
		if got != tc.want {
			t.Fatalf("bits=%016x got=%q want=%q", tc.bits, got, tc.want)
		}
	}
}

// --- Helpers ---

func verifyOracle(t *testing.T, path string, expectedRows int, expectedSHA256 string) {
	t.Helper()

	// #nosec G304 -- oracle fixture path is explicit test input.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open oracle: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Errorf("close oracle %s: %v", path, closeErr)
		}
	})

	h := sha256.New()
	tee := io.TeeReader(f, h)
	sc := bufio.NewScanner(tee)
	sc.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)

	rows := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rows++
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed oracle line %d: %q", rows, line)
		}
		bits, err := strconv.ParseUint(parts[0], 16, 64)
		if err != nil {
			t.Fatalf("line %d parse bits: %v", rows, err)
		}
		got, fmtErr := jcsfloat.FormatDouble(math.Float64frombits(bits))
		if fmtErr != nil {
			t.Fatalf("line %d unexpected format error: %v", rows, fmtErr)
		}
		if got != parts[1] {
			t.Fatalf("line %d bits=%016x got=%q want=%q", rows, bits, got, parts[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan oracle: %v", err)
	}
	if rows != expectedRows {
		t.Fatalf("oracle row count mismatch: got %d want %d", rows, expectedRows)
	}
	gotSHA := fmt.Sprintf("%x", h.Sum(nil))
	if gotSHA != expectedSHA256 {
		t.Fatalf("oracle checksum mismatch: got %s want %s", gotSHA, expectedSHA256)
	}
}
