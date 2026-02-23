package jcstoken_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/SolutionsExcite/json-canon/jcserr"
	"github.com/SolutionsExcite/json-canon/jcstoken"
)

func mustParse(t *testing.T, in string) *jcstoken.Value {
	t.Helper()
	v, err := jcstoken.Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return v
}

func mustParseErr(t *testing.T, in string) *jcserr.Error {
	t.Helper()
	_, err := jcstoken.Parse([]byte(in))
	if err == nil {
		t.Fatalf("expected error for %q", in)
	}
	var je *jcserr.Error
	if !errors.As(err, &je) {
		t.Fatalf("expected *jcserr.Error, got %T: %v", err, err)
	}
	return je
}

func mustParseErrBytes(t *testing.T, in []byte) *jcserr.Error {
	t.Helper()
	_, err := jcstoken.Parse(in)
	if err == nil {
		t.Fatalf("expected error for %q", in)
	}
	var je *jcserr.Error
	if !errors.As(err, &je) {
		t.Fatalf("expected *jcserr.Error, got %T: %v", err, err)
	}
	return je
}

// === PARSE-UTF8-001: Invalid UTF-8 rejected ===

func TestParse_PARSE_UTF8_001(t *testing.T) {
	cases := [][]byte{
		{'"', 0xff, '"'},
		{'"', 0xe2, 0x82, '"'},       // truncated 3-byte sequence
		{'"', 0xed, 0xa0, 0x80, '"'}, // raw UTF-8 surrogate encoding (invalid scalar)
	}
	for _, in := range cases {
		je := mustParseErrBytes(t, in)
		if je.Class != jcserr.InvalidUTF8 {
			t.Fatalf("expected INVALID_UTF8, got %s for %v", je.Class, in)
		}
	}
}

// === PARSE-UTF8-002: Overlong UTF-8 rejected ===

func TestParse_PARSE_UTF8_002(t *testing.T) {
	// 0xC0 0xAF is an overlong encoding of U+002F
	je := mustParseErrBytes(t, []byte{'"', 0xc0, 0xaf, '"'})
	if je.Class != jcserr.InvalidUTF8 {
		t.Fatalf("expected INVALID_UTF8, got %s", je.Class)
	}
}

// === PARSE-GRAM-001: Leading zeros rejected ===

func TestParse_PARSE_GRAM_001(t *testing.T) {
	je := mustParseErr(t, `01`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
	if !strings.Contains(je.Message, "leading zero") {
		t.Fatalf("unexpected message: %s", je.Message)
	}
}

// === PARSE-GRAM-002: Trailing commas in objects rejected ===

func TestParse_PARSE_GRAM_002(t *testing.T) {
	je := mustParseErr(t, `{"a":1,}`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
}

// === PARSE-GRAM-003: Trailing commas in arrays rejected ===

func TestParse_PARSE_GRAM_003(t *testing.T) {
	je := mustParseErr(t, `[1,]`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
}

// === PARSE-GRAM-004: Unescaped control characters rejected ===

func TestParse_PARSE_GRAM_004(t *testing.T) {
	for _, in := range [][]byte{
		{'"', 0x01, '"'},
		{'"', 0x00, '"'}, // explicit raw NUL probe
	} {
		je := mustParseErrBytes(t, in)
		if je.Class != jcserr.InvalidGrammar {
			t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
		}
		if !strings.Contains(je.Message, "control character") {
			t.Fatalf("unexpected message: %s", je.Message)
		}
	}
}

// === PARSE-GRAM-005: Top-level scalar accepted ===

func TestParse_PARSE_GRAM_005(t *testing.T) {
	for _, in := range []string{`42`, `"hello"`, `true`, `false`, `null`} {
		v := mustParse(t, in)
		if v == nil {
			t.Fatalf("nil value for %q", in)
		}
	}

	// Empty input is not a JSON value.
	if je := mustParseErrBytes(t, []byte{}); je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR for empty input, got %s", je.Class)
	}

	// BOM-prefixed input is not accepted as leading JSON whitespace.
	if je := mustParseErrBytes(t, []byte{0xEF, 0xBB, 0xBF, '4', '2'}); je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR for BOM-prefixed input, got %s", je.Class)
	}
}

// === PARSE-GRAM-006: Insignificant whitespace accepted ===

func TestParse_PARSE_GRAM_006(t *testing.T) {
	v := mustParse(t, " \n\t { \"a\" : 1 } \r ")
	if v.Kind != jcstoken.KindObject || len(v.Members) != 1 {
		t.Fatalf("unexpected result: %+v", v)
	}

	v = mustParse(t, "\r\n{\r\n\"a\"\r\n:\r\n1\r\n}\r\n")
	if v.Kind != jcstoken.KindObject || len(v.Members) != 1 {
		t.Fatalf("unexpected CRLF parse result: %+v", v)
	}
}

// === PARSE-GRAM-007: Invalid literals rejected ===

func TestParse_PARSE_GRAM_007(t *testing.T) {
	for _, in := range []string{`tru`, `fals`, `nul`, `True`, `FALSE`, `NULL`} {
		je := mustParseErr(t, in)
		if je.Class != jcserr.InvalidGrammar {
			t.Fatalf("expected INVALID_GRAMMAR for %q, got %s", in, je.Class)
		}
	}
}

// === PARSE-GRAM-008: Trailing content rejected ===

func TestParse_PARSE_GRAM_008(t *testing.T) {
	je := mustParseErr(t, `42 "extra"`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
	if !strings.Contains(je.Message, "trailing content") {
		t.Fatalf("unexpected message: %s", je.Message)
	}
}

// === PARSE-GRAM-009: Number grammar enforced ===

func TestParse_PARSE_GRAM_009(t *testing.T) {
	// Valid numbers
	for _, in := range []string{`0`, `1`, `-1`, `0.5`, `1e10`, `1.5e-3`} {
		mustParse(t, in)
	}
	// Invalid: no digits after decimal
	je := mustParseErr(t, `1.`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
}

// === PARSE-GRAM-010: Invalid escape sequences rejected ===

func TestParse_PARSE_GRAM_010(t *testing.T) {
	je := mustParseErr(t, `"\x"`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
	if !strings.Contains(je.Message, "invalid escape") {
		t.Fatalf("unexpected message: %s", je.Message)
	}
}

// === IJSON-DUP-001: Duplicate keys rejected ===

func TestParse_IJSON_DUP_001(t *testing.T) {
	je := mustParseErr(t, `{"a":1,"a":2}`)
	if je.Class != jcserr.DuplicateKey {
		t.Fatalf("expected DUPLICATE_KEY, got %s", je.Class)
	}
}

// === IJSON-DUP-002: Duplicate keys after escape decoding rejected ===

func TestParse_IJSON_DUP_002(t *testing.T) {
	je := mustParseErr(t, `{"\u0061":1,"a":2}`)
	if je.Class != jcserr.DuplicateKey {
		t.Fatalf("expected DUPLICATE_KEY, got %s", je.Class)
	}
}

// === IJSON-SUR-001: Lone high surrogate rejected ===

func TestParse_IJSON_SUR_001(t *testing.T) {
	je := mustParseErr(t, `"\uD800"`)
	if je.Class != jcserr.LoneSurrogate {
		t.Fatalf("expected LONE_SURROGATE, got %s", je.Class)
	}
	if je.Offset != 1 {
		t.Fatalf("expected source-byte offset 1, got %d", je.Offset)
	}
}

// === IJSON-SUR-002: Lone low surrogate rejected ===

func TestParse_IJSON_SUR_002(t *testing.T) {
	je := mustParseErr(t, `"\uDC00"`)
	if je.Class != jcserr.LoneSurrogate {
		t.Fatalf("expected LONE_SURROGATE, got %s", je.Class)
	}
	if je.Offset != 1 {
		t.Fatalf("expected source-byte offset 1, got %d", je.Offset)
	}
}

// === IJSON-SUR-003: Valid surrogate pair decoded ===

func TestParse_IJSON_SUR_003(t *testing.T) {
	v := mustParse(t, `"\uD83D\uDE00"`)
	if v.Kind != jcstoken.KindString || v.Str != "😀" {
		t.Fatalf("got %q want 😀", v.Str)
	}
}

// === IJSON-NONC-001: Noncharacter rejected ===

func TestParse_IJSON_NONC_001(t *testing.T) {
	je := mustParseErr(t, `"\uFDD0"`)
	if je.Class != jcserr.Noncharacter {
		t.Fatalf("expected NONCHARACTER, got %s", je.Class)
	}
	if je.Offset != 1 {
		t.Fatalf("expected source-byte offset 1, got %d", je.Offset)
	}
	// Also test U+FFFE (plane 0)
	je = mustParseErr(t, `"\uFFFE"`)
	if je.Class != jcserr.Noncharacter {
		t.Fatalf("expected NONCHARACTER for U+FFFE, got %s", je.Class)
	}
	// Supplementary-plane noncharacter U+1FFFE
	je = mustParseErr(t, `"\uD83F\uDFFE"`)
	if je.Class != jcserr.Noncharacter {
		t.Fatalf("expected NONCHARACTER for U+1FFFE, got %s", je.Class)
	}
	if je.Offset != 1 {
		t.Fatalf("expected source-byte offset 1, got %d", je.Offset)
	}
}

func TestParse_IJSON_SUR_OffsetsSecondEscape(t *testing.T) {
	je := mustParseErr(t, `"\uD800\u0041"`)
	if je.Class != jcserr.LoneSurrogate {
		t.Fatalf("expected LONE_SURROGATE, got %s", je.Class)
	}
	if je.Offset != 7 {
		t.Fatalf("expected source-byte offset 7 for second escape, got %d", je.Offset)
	}

	je = mustParseErr(t, `"\uD800\u12"`)
	if je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %s", je.Class)
	}
	if je.Offset != 7 {
		t.Fatalf("expected source-byte offset 7 for malformed second escape, got %d", je.Offset)
	}
}

// === PROF-NEGZ-001: Lexical -0 rejected ===

func TestParse_PROF_NEGZ_001(t *testing.T) {
	for _, in := range []string{`-0`, `-0.0`, `-0e0`, `-0.0e+0`, `-0.0e1`, `-0e-1`} {
		je := mustParseErr(t, in)
		if je.Class != jcserr.NumberNegZero {
			t.Fatalf("expected NUMBER_NEGZERO for %q, got %s", in, je.Class)
		}
	}
}

// === PROF-OFLOW-001: Number overflow rejected ===

func TestParse_PROF_OFLOW_001(t *testing.T) {
	je := mustParseErr(t, `1e999999`)
	if je.Class != jcserr.NumberOverflow {
		t.Fatalf("expected NUMBER_OVERFLOW, got %s", je.Class)
	}
}

// === PROF-UFLOW-001: Non-zero underflow to zero rejected ===

func TestParse_PROF_UFLOW_001(t *testing.T) {
	for _, in := range []string{`1e-400`, `1e-324`, `2e-324`} {
		je := mustParseErr(t, in)
		if je.Class != jcserr.NumberUnderflow {
			t.Fatalf("expected NUMBER_UNDERFLOW for %q, got %s", in, je.Class)
		}
	}
	// Boundary that rounds to the minimum subnormal and must remain accepted.
	v := mustParse(t, `3e-324`)
	if v.Kind != jcstoken.KindNumber || v.Num == 0 {
		t.Fatalf("expected non-zero parsed number, got %+v", v)
	}
}

// === BOUND-DEPTH-001: Nesting depth bounded ===

func TestParse_BOUND_DEPTH_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions([]byte(`[[[]]]`), &jcstoken.Options{MaxDepth: 3})
	if err != nil {
		t.Fatalf("expected exact max-depth input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`[[[]]]`), &jcstoken.Options{MaxDepth: 2})
	if err == nil {
		t.Fatal("expected max-depth error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-INPUT-001: Input size bounded ===

func TestParse_BOUND_INPUT_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions([]byte(`[]`), &jcstoken.Options{MaxInputSize: 2})
	if err != nil {
		t.Fatalf("expected exact max-input input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxInputSize: 2})
	if err == nil {
		t.Fatal("expected input-size error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-VALUES-001: Value count bounded ===

func TestParse_BOUND_VALUES_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 3})
	if err != nil {
		t.Fatalf("expected exact value-count input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 2})
	if err == nil {
		t.Fatal("expected value-count error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-MEMBERS-001: Object member count bounded ===

func TestParse_BOUND_MEMBERS_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`{"a":1,"b":2}`),
		&jcstoken.Options{MaxObjectMembers: 2},
	)
	if err != nil {
		t.Fatalf("expected exact member-count input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions(
		[]byte(`{"a":1,"b":2}`),
		&jcstoken.Options{MaxObjectMembers: 1},
	)
	if err == nil {
		t.Fatal("expected object-member-limit error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-ELEMS-001: Array element count bounded ===

func TestParse_BOUND_ELEMS_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`[1,2]`),
		&jcstoken.Options{MaxArrayElements: 2},
	)
	if err != nil {
		t.Fatalf("expected exact array-element input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions(
		[]byte(`[1,2]`),
		&jcstoken.Options{MaxArrayElements: 1},
	)
	if err == nil {
		t.Fatal("expected array-element-limit error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-STRBYTES-001: String byte length bounded ===

func TestParse_BOUND_STRBYTES_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`"ab"`),
		&jcstoken.Options{MaxStringBytes: 2},
	)
	if err != nil {
		t.Fatalf("expected exact string-byte input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions(
		[]byte(`"ab"`),
		&jcstoken.Options{MaxStringBytes: 1},
	)
	if err == nil {
		t.Fatal("expected string-length error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === BOUND-NUMCHARS-001: Number token character length bounded ===

func TestParse_BOUND_NUMCHARS_001(t *testing.T) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`1234`),
		&jcstoken.Options{MaxNumberChars: 4},
	)
	if err != nil {
		t.Fatalf("expected exact number-char input to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions(
		[]byte(`12345`),
		&jcstoken.Options{MaxNumberChars: 4},
	)
	if err == nil {
		t.Fatal("expected number-length error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

// === Additional: Zero variants accepted ===

func TestParseAllowsZeroTokenVariants(t *testing.T) {
	for _, in := range []string{"0", "0.0", "0e10", "0.000e+9"} {
		v := mustParse(t, in)
		if v.Kind != jcstoken.KindNumber || math.Signbit(v.Num) || v.Num != 0 {
			t.Fatalf("bad zero parse for %q: %+v", in, v)
		}
	}
}

// === Additional: Duplicate keys in different scopes OK ===

func TestParseAllowsDuplicateKeysInDifferentScopes(t *testing.T) {
	v := mustParse(t, `{"a":1,"nested":{"a":2}}`)
	if v.Kind != jcstoken.KindObject || len(v.Members) != 2 {
		t.Fatalf("unexpected parse result: %+v", v)
	}
}
