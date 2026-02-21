package jcs_test

import (
	"errors"
	"math"
	"testing"
	"unicode/utf8"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

func canon(t *testing.T, in string) string {
	t.Helper()
	v, err := jcstoken.Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize %q: %v", in, err)
	}
	return string(out)
}

// === CANON-WS-001: No insignificant whitespace ===

func TestSerialize_CANON_WS_001(t *testing.T) {
	got := canon(t, `{ "a" : 1 }`)
	if got != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-001: U+0008 → \b ===

func TestSerialize_CANON_STR_001(t *testing.T) {
	got := canon(t, `"\u0008"`)
	if got != `"\b"` {
		t.Fatalf("got %q want %q", got, `"\b"`)
	}
}

// === CANON-STR-002: U+0009 → \t ===

func TestSerialize_CANON_STR_002(t *testing.T) {
	got := canon(t, `"\u0009"`)
	if got != `"\t"` {
		t.Fatalf("got %q want %q", got, `"\t"`)
	}
}

// === CANON-STR-003: U+000A → \n ===

func TestSerialize_CANON_STR_003(t *testing.T) {
	got := canon(t, `"\u000a"`)
	if got != `"\n"` {
		t.Fatalf("got %q want %q", got, `"\n"`)
	}
}

// === CANON-STR-004: U+000C → \f ===

func TestSerialize_CANON_STR_004(t *testing.T) {
	got := canon(t, `"\u000c"`)
	if got != `"\f"` {
		t.Fatalf("got %q want %q", got, `"\f"`)
	}
}

// === CANON-STR-005: U+000D → \r ===

func TestSerialize_CANON_STR_005(t *testing.T) {
	got := canon(t, `"\u000d"`)
	if got != `"\r"` {
		t.Fatalf("got %q want %q", got, `"\r"`)
	}
}

// === CANON-STR-006: Other controls → \u00xx lowercase ===

func TestSerialize_CANON_STR_006(t *testing.T) {
	got := canon(t, `"\u001f"`)
	if got != `"\u001f"` {
		t.Fatalf("got %q", got)
	}
	got = canon(t, `"\u0000"`)
	if got != `"\u0000"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-007: U+0022 → \" ===

func TestSerialize_CANON_STR_007(t *testing.T) {
	got := canon(t, `"a\"b"`)
	if got != `"a\"b"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-008: U+005C → \\ ===

func TestSerialize_CANON_STR_008(t *testing.T) {
	got := canon(t, `"a\\b"`)
	if got != `"a\\b"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-009: Solidus NOT escaped ===

func TestSerialize_CANON_STR_009(t *testing.T) {
	got := canon(t, `"a\/b"`)
	if got != `"a/b"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-010: Characters above U+001F → raw UTF-8 ===

func TestSerialize_CANON_STR_010(t *testing.T) {
	// < > & should not be escaped
	if got := canon(t, `"<>&"`); got != `"<>&"` {
		t.Fatalf("got %q", got)
	}
	// Emoji should be raw UTF-8
	got := canon(t, `"\uD83D\uDE00"`)
	if got != `"😀"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-STR-011: No Unicode normalization ===

func TestSerialize_CANON_STR_011(t *testing.T) {
	// NFC é (U+00E9) vs NFD e + ́ (U+0065 U+0301) must remain distinct
	nfc := "\u00E9"  // single codepoint U+00E9
	nfd := "e\u0301" // two codepoints U+0065 + U+0301
	v1 := &jcstoken.Value{Kind: jcstoken.KindString, Str: nfc}
	v2 := &jcstoken.Value{Kind: jcstoken.KindString, Str: nfd}
	o1, err := jcs.Serialize(v1)
	if err != nil {
		t.Fatalf("serialize NFC: %v", err)
	}
	o2, err := jcs.Serialize(v2)
	if err != nil {
		t.Fatalf("serialize NFD: %v", err)
	}
	if string(o1) == string(o2) {
		t.Fatal("normalization was applied — NFC and NFD should produce different output")
	}
}

// === CANON-STR-012: Strings are enclosed in quotes ===

func TestSerialize_CANON_STR_012(t *testing.T) {
	if got := canon(t, `""`); got != `""` {
		t.Fatalf("got %q", got)
	}
	if got := canon(t, `"abc"`); got != `"abc"` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-SORT-001: UTF-16 code-unit sort ===

func TestSerialize_CANON_SORT_001(t *testing.T) {
	// Basic BMP sort
	got := canon(t, `{"z":3,"a":1}`)
	if got != `{"a":1,"z":3}` {
		t.Fatalf("got %q", got)
	}
	// UTF-16 vs UTF-8 divergence: supplementary plane
	got = canon(t, `{"\uE000":1,"\uD800\uDC00":2}`)
	if got != "{\"𐀀\":2,\"\ue000\":1}" {
		t.Fatalf("got %q", got)
	}
	// Mixed empty/prefix/BMP/supplementary ordering.
	got = canon(t, `{"\uE000":5,"\uD83D\uDE00":4,"\uD800\uDC00":3,"aa":2,"":1}`)
	if got != "{\"\":1,\"aa\":2,\"𐀀\":3,\"😀\":4,\"\ue000\":5}" {
		t.Fatalf("got %q", got)
	}
}

// === CANON-SORT-002: Recursive sorting ===

func TestSerialize_CANON_SORT_002(t *testing.T) {
	got := canon(t, `{"b":[{"z":1,"a":2}],"a":3}`)
	if got != `{"a":3,"b":[{"a":2,"z":1}]}` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-SORT-003: Array order preserved ===

func TestSerialize_CANON_SORT_003(t *testing.T) {
	got := canon(t, `[3,1,2]`)
	if got != `[3,1,2]` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-SORT-004: Sorting uses unescaped/raw property names ===

func TestSerialize_CANON_SORT_004(t *testing.T) {
	got := canon(t, `{"\\n":1,"\n":2}`)
	if got != `{"\n":2,"\\n":1}` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-SORT-005: Lexicographic rule with prefix ordering ===

func TestSerialize_CANON_SORT_005(t *testing.T) {
	got := canon(t, `{"ab":4,"aa":3,"":1,"a":2}`)
	if got != `{"":1,"a":2,"aa":3,"ab":4}` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-LIT-001: Lowercase literals ===

func TestSerialize_CANON_LIT_001(t *testing.T) {
	if got := canon(t, `true`); got != `true` {
		t.Fatalf("got %q", got)
	}
	if got := canon(t, `false`); got != `false` {
		t.Fatalf("got %q", got)
	}
	if got := canon(t, `null`); got != `null` {
		t.Fatalf("got %q", got)
	}
}

// === CANON-ENC-001: Output is UTF-8 ===

func TestSerialize_CANON_ENC_001(t *testing.T) {
	got := canon(t, `{"key":"value"}`)
	if !utf8.ValidString(got) {
		t.Fatal("output is not valid UTF-8")
	}
}

// === CANON-ENC-002: Output does not include UTF-8 BOM prefix ===

func TestSerialize_CANON_ENC_002(t *testing.T) {
	got := canon(t, `{"a":1}`)
	if len(got) >= 3 && got[0] == 0xEF && got[1] == 0xBB && got[2] == 0xBF {
		t.Fatalf("unexpected UTF-8 BOM prefix in %q", got)
	}
}

// === GEN-GRAM-001: Generator output strictly conforms to RFC 8259 grammar ===

func TestSerialize_GEN_GRAM_001(t *testing.T) {
	v, err := jcstoken.Parse([]byte(`{"z":[{"b":"\u0000","a":1e21}],"a":true}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if _, err := jcstoken.Parse(out); err != nil {
		t.Fatalf("generated output is not valid JSON grammar: %v", err)
	}
}

// === Serializer validation tests ===

func TestSerializeRejectsNonFiniteNumber(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindNumber, Num: math.Inf(1)}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %v", err)
	}
}

func TestSerializeRejectsNilValue(t *testing.T) {
	_, err := jcs.Serialize(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.InternalError {
		t.Fatalf("expected INTERNAL_ERROR, got %v", err)
	}
}

func TestSerializeRejectsInvalidBoolPayload(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindBool, Str: "TRUE"}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.InvalidGrammar {
		t.Fatalf("expected INVALID_GRAMMAR, got %v", err)
	}
}

func TestSerializeRejectsInvalidUTF8StringPayload(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindString, Str: string([]byte{0xff})}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.InvalidUTF8 {
		t.Fatalf("expected INVALID_UTF8, got %v", err)
	}
}

func TestSerializeRejectsDuplicateKeysInValueTree(t *testing.T) {
	v := &jcstoken.Value{
		Kind: jcstoken.KindObject,
		Members: []jcstoken.Member{
			{Key: "a", Value: jcstoken.Value{Kind: jcstoken.KindNumber, Num: 1}},
			{Key: "a", Value: jcstoken.Value{Kind: jcstoken.KindNumber, Num: 2}},
		},
	}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.DuplicateKey {
		t.Fatalf("expected DUPLICATE_KEY, got %v", err)
	}
}

func TestSerializeNegativeZero(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindNumber, Num: math.Copysign(0, -1)}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != `0` {
		t.Fatalf("got %q", string(out))
	}
}

func TestSerializeNonObjectTopLevel(t *testing.T) {
	if got := canon(t, `"hello"`); got != `"hello"` {
		t.Fatalf("got %q", got)
	}
	if got := canon(t, `42`); got != `42` {
		t.Fatalf("got %q", got)
	}
}
