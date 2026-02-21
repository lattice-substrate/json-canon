package jcstoken_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"jcs-canon/jcstoken"
)

func mustParse(t *testing.T, in string) *jcstoken.Value {
	t.Helper()
	v, err := jcstoken.Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return v
}

func mustParseErr(t *testing.T, in string) error {
	t.Helper()
	_, err := jcstoken.Parse([]byte(in))
	if err == nil {
		t.Fatalf("expected error for %q", in)
	}
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
	return err
}

func TestParseBasicObject(t *testing.T) {
	v := mustParse(t, `{"a":1,"b":[true,null,"x"]}`)
	if v.Kind != jcstoken.KindObject || len(v.Members) != 2 {
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
	if v.Kind != jcstoken.KindString || v.Str != "😀" {
		t.Fatalf("got %q want 😀", v.Str)
	}
}

func TestParseRejectsNoncharacterEscape(t *testing.T) {
	err := mustParseErr(t, `"\uFDD0"`)
	if !strings.Contains(err.Error(), "noncharacter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAllowsNegativeZero(t *testing.T) {
	v := mustParse(t, `-0`)
	if v.Kind != jcstoken.KindNumber || !math.Signbit(v.Num) || v.Num != 0 {
		t.Fatalf("bad parse for -0: %+v", v)
	}
}

func TestParseAllowsUnderflowToZero(t *testing.T) {
	v := mustParse(t, `1e-400`)
	if v.Kind != jcstoken.KindNumber || math.Signbit(v.Num) || v.Num != 0 {
		t.Fatalf("bad parse for underflow: %+v", v)
	}
}

func TestParseRejectsOverflow(t *testing.T) {
	err := mustParseErr(t, `1e999999`)
	if !strings.Contains(err.Error(), "overflows IEEE 754 double") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAllowsZeroTokenVariants(t *testing.T) {
	for _, in := range []string{"0", "0.0", "0e10", "0.000e+9"} {
		v := mustParse(t, in)
		if v.Kind != jcstoken.KindNumber || math.Signbit(v.Num) || v.Num != 0 {
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
		t.Fatalf("unexpected trailing-comma object error: %v", err)
	}
}

func TestParseRejectsTrailingCommaArray(t *testing.T) {
	err := mustParseErr(t, `[1,]`)
	if !strings.Contains(err.Error(), "invalid number character") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDepthLimit(t *testing.T) {
	_, err := jcstoken.ParseWithOptions([]byte(`[[[]]]`), &jcstoken.Options{MaxDepth: 2})
	if err == nil {
		t.Fatal("expected max-depth error")
	}
	if !strings.Contains(err.Error(), "nesting depth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAllowsDuplicateKeysInDifferentScopes(t *testing.T) {
	v := mustParse(t, `{"a":1,"nested":{"a":2}}`)
	if v.Kind != jcstoken.KindObject || len(v.Members) != 2 {
		t.Fatalf("unexpected parse result: %+v", v)
	}
}

func TestParseRejectsInvalidUTF8Input(t *testing.T) {
	_, err := jcstoken.Parse([]byte{'"', 0xff, '"'})
	if err == nil {
		t.Fatal("expected UTF-8 error")
	}
	if !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("unexpected error: %v", err)
	}
}
