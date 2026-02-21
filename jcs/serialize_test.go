package jcs_test

import (
	"math"
	"testing"

	"jcs-canon/jcs"
	"jcs-canon/jcstoken"
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

func TestSerializeWhitespaceRemoval(t *testing.T) {
	got := canon(t, `{ "a" : 1 }`)
	if got != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeSortsBMPKeys(t *testing.T) {
	got := canon(t, `{"z":3,"a":1}`)
	if got != `{"a":1,"z":3}` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeUTF16SortDivergence(t *testing.T) {
	got := canon(t, `{"\uE000":1,"\uD800\uDC00":2}`)
	if got != "{\"𐀀\":2,\"\ue000\":1}" {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeEscapesControlCharacters(t *testing.T) {
	got := canon(t, `"\u0008\u0009\u000a\u000c\u000d\u001f"`)
	if got != `"\b\t\n\f\r\u001f"` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeNoEscapeChars(t *testing.T) {
	if got := canon(t, `"<>&"`); got != `"<>&"` {
		t.Fatalf("got %q", got)
	}
	if got := canon(t, `"a\/b"`); got != `"a/b"` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeHexLowercase(t *testing.T) {
	got := canon(t, `"\u001F"`)
	if got != `"\u001f"` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeBoundary1e20(t *testing.T) {
	got := canon(t, `1e20`)
	if got != `100000000000000000000` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeBoundary1e21(t *testing.T) {
	got := canon(t, `1e21`)
	if got != `1e+21` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeExponentFormat(t *testing.T) {
	got := canon(t, `1e-7`)
	if got != `1e-7` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeNegativeZero(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindNumber, Num: math.Copysign(0, -1)}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	got := string(out)
	if got != `0` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeUnderflowToZero(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindNumber, Num: 0}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	got := string(out)
	if got != `0` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeLiterals(t *testing.T) {
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

func TestSerializeRecursiveSort(t *testing.T) {
	got := canon(t, `{"b":[{"z":1,"a":2}],"a":3}`)
	if got != `{"a":3,"b":[{"a":2,"z":1}]}` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeSurrogatePairDecode(t *testing.T) {
	got := canon(t, `"\uD83D\uDE00"`)
	if got != `"😀"` {
		t.Fatalf("got %q", got)
	}
}

func TestSerializeQuoteBackslash(t *testing.T) {
	got := canon(t, `"a\"b\\c"`)
	if got != `"a\"b\\c"` {
		t.Fatalf("got %q", got)
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

func TestSerializeRejectsNonFiniteNumber(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindNumber, Num: math.Inf(1)}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSerializeRejectsNilValue(t *testing.T) {
	_, err := jcs.Serialize(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSerializeRejectsInvalidBoolPayload(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindBool, Str: "TRUE"}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSerializeRejectsInvalidUTF8StringPayload(t *testing.T) {
	v := &jcstoken.Value{Kind: jcstoken.KindString, Str: string([]byte{0xff})}
	_, err := jcs.Serialize(v)
	if err == nil {
		t.Fatal("expected error")
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
}
