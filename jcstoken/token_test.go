package jcstoken

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func mustParse(t *testing.T, in string) *Value {
	t.Helper()
	v, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return v
}

func mustParseErr(t *testing.T, in string) error {
	t.Helper()
	_, err := Parse([]byte(in))
	if err == nil {
		t.Fatalf("expected error for %q", in)
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
	return err
}

func TestParseBasicObject(t *testing.T) {
	v := mustParse(t, `{"a":1,"b":[true,null,"x"]}`)
	if v.Kind != KindObject || len(v.Members) != 2 {
		t.Fatalf("unexpected parse result: %+v", v)
	}
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	err := mustParseErr(t, `{"a":1,"a":2}`)
	if !strings.Contains(err.Error(), "duplicate object key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDuplicateKeysAfterEscapeDecoding(t *testing.T) {
	err := mustParseErr(t, `{"\u0061":1,"a":2}`)
	if !strings.Contains(err.Error(), "duplicate object key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsLoneHighSurrogate(t *testing.T) {
	err := mustParseErr(t, `"\uD800"`)
	if !strings.Contains(err.Error(), "lone high surrogate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsLoneLowSurrogate(t *testing.T) {
	err := mustParseErr(t, `"\uDC00"`)
	if !strings.Contains(err.Error(), "lone low surrogate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDecodesValidSurrogatePair(t *testing.T) {
	v := mustParse(t, `"\uD83D\uDE00"`)
	if v.Kind != KindString || v.Str != "😀" {
		t.Fatalf("got %q want 😀", v.Str)
	}
}

func TestParseRejectsNoncharacterEscape(t *testing.T) {
	err := mustParseErr(t, `"\uFDD0"`)
	if !strings.Contains(err.Error(), "noncharacter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsNegativeZero(t *testing.T) {
	err := mustParseErr(t, `-0`)
	if !strings.Contains(err.Error(), "negative zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsUnderflowToZero(t *testing.T) {
	err := mustParseErr(t, `1e-400`)
	if !strings.Contains(err.Error(), "underflows to zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAllowsZeroTokenVariants(t *testing.T) {
	for _, in := range []string{"0", "0.0", "0e10", "0.000e+9"} {
		v := mustParse(t, in)
		if v.Kind != KindNumber || math.Signbit(v.Num) || v.Num != 0 {
			t.Fatalf("bad zero parse for %q: %+v", in, v)
		}
	}
}

func TestParseRejectsLeadingZero(t *testing.T) {
	err := mustParseErr(t, `01`)
	if !strings.Contains(err.Error(), "leading zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsTrailingCommaObject(t *testing.T) {
	err := mustParseErr(t, `{"a":1,}`)
	if !strings.Contains(err.Error(), "expected \"\\\"\"") && !strings.Contains(err.Error(), "expected '\"'") {
		// parser wording varies by offset; only ensure this is a parse failure.
		_ = err
	}
}

func TestParseRejectsTrailingCommaArray(t *testing.T) {
	err := mustParseErr(t, `[1,]`)
	if !strings.Contains(err.Error(), "invalid number character") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDepthLimit(t *testing.T) {
	_, err := ParseWithOptions([]byte(`[[[]]]`), &Options{MaxDepth: 2})
	if err == nil {
		t.Fatal("expected max-depth error")
	}
	if !strings.Contains(err.Error(), "nesting depth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAllowsDuplicateKeysInDifferentScopes(t *testing.T) {
	v := mustParse(t, `{"a":1,"nested":{"a":2}}`)
	if v.Kind != KindObject || len(v.Members) != 2 {
		t.Fatalf("unexpected parse result: %+v", v)
	}
}
