package jcsfloat

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestFormatDoubleGoldenVectors(t *testing.T) {
	f, err := os.Open("testdata/golden_vectors.csv")
	if err != nil {
		t.Fatalf("open golden vectors: %v", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			t.Fatalf("line %d malformed: %q", lineNo, line)
		}

		bits, err := strconv.ParseUint(parts[0], 16, 64)
		if err != nil {
			t.Fatalf("line %d bad bits %q: %v", lineNo, parts[0], err)
		}
		expect := parts[1]
		input := math.Float64frombits(bits)
		got, err := FormatDouble(input)
		if err != nil {
			t.Fatalf("line %d unexpected error for %016x: %v", lineNo, bits, err)
		}
		if got != expect {
			t.Fatalf("line %d bits=%016x: got %q want %q", lineNo, bits, got, expect)
		}
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan golden vectors: %v", err)
	}
	if lineNo != 54445 {
		t.Fatalf("golden vector line count mismatch: got %d want 54445", lineNo)
	}
}

func TestFormatDoubleRejectsNonFinite(t *testing.T) {
	cases := []float64{math.NaN(), math.Inf(+1), math.Inf(-1)}
	for _, c := range cases {
		_, err := FormatDouble(c)
		if err == nil {
			t.Fatalf("expected error for %v", c)
		}
	}
}

func TestFormatDoubleNegativeZero(t *testing.T) {
	got, err := FormatDouble(math.Copysign(0, -1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0" {
		t.Fatalf("got %q want %q", got, "0")
	}
}

func TestFormatDoubleRoundTripProperty(t *testing.T) {
	cases := []float64{5e-324, 1e-7, 1e-6, 0.1, 0.2, 1.1, 1, 2, 1e20, 1e21, math.MaxFloat64}
	for _, c := range cases {
		f1, err := FormatDouble(c)
		if err != nil {
			t.Fatalf("format(%.17g): %v", c, err)
		}
		v, err := strconv.ParseFloat(f1, 64)
		if err != nil {
			t.Fatalf("parse %q: %v", f1, err)
		}
		f2, err := FormatDouble(v)
		if err != nil {
			t.Fatalf("re-format(%.17g): %v", v, err)
		}
		if f1 != f2 {
			t.Fatalf("idempotency failed for %.17g: first=%q second=%q", c, f1, f2)
		}
	}

	for i := uint64(1); i < 5000; i += 97 {
		v := math.Float64frombits(i * 0x9e3779b97f4a7c15)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		f1, err := FormatDouble(v)
		if err != nil {
			t.Fatalf("format bits=%016x: %v", math.Float64bits(v), err)
		}
		parsed, err := strconv.ParseFloat(f1, 64)
		if err != nil {
			t.Fatalf("parse bits=%016x text=%q: %v", math.Float64bits(v), f1, err)
		}
		f2, err := FormatDouble(parsed)
		if err != nil {
			t.Fatalf("re-format bits=%016x: %v", math.Float64bits(v), err)
		}
		if f1 != f2 {
			t.Fatalf("round-trip mismatch bits=%016x: %s != %s", math.Float64bits(v), f1, f2)
		}
	}

	if testing.Verbose() {
		fmt.Println("jcsfloat round-trip property checks passed")
	}
}
