package jcs_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

func BenchmarkSerialize(b *testing.B) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"flat_10_keys", buildFlatObject(10)},
		{"flat_100_keys", buildFlatObject(100)},
		{"nested_10_deep", benchNestedObject(10)},
		{"array_1000", benchLargeArray(1000)},
		{"unicode_keys", buildUnicodeKeyObject()},
	}

	for _, tc := range cases {
		v, err := jcstoken.Parse(tc.input)
		if err != nil {
			b.Fatalf("parse %s: %v", tc.name, err)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.input)))
			for i := 0; i < b.N; i++ {
				_, _ = jcs.Serialize(v)
			}
		})
	}
}

func BenchmarkCanonicalize(b *testing.B) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"small_object", []byte(`{"b":2,"a":1,"c":"hello"}`)},
		{"medium_payload", buildFlatObject(50)},
		{"large_payload", buildFlatObject(200)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.input)))
			for i := 0; i < b.N; i++ {
				v, err := jcstoken.Parse(tc.input)
				if err != nil {
					b.Fatal(err)
				}
				_, _ = jcs.Serialize(v)
			}
		})
	}
}

func buildFlatObject(n int) []byte {
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"key_%03d":%d`, i, i))
	}
	sb.WriteString("}")
	return []byte(sb.String())
}

func benchNestedObject(depth int) []byte {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf(`{"k%d":`, i))
	}
	sb.WriteString("1")
	for i := 0; i < depth; i++ {
		sb.WriteString("}")
	}
	return []byte(sb.String())
}

func benchLargeArray(n int) []byte {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%d", i))
	}
	sb.WriteString("]")
	return []byte(sb.String())
}

func buildUnicodeKeyObject() []byte {
	keys := []string{
		"\u00e9", "\u00e8", "\u00ea", "\u00eb",
		"\u00c0", "\u00c1", "\u00c2", "\u00c3",
		"\u0100", "\u0101", "\u0102", "\u0103",
		"\u4e00", "\u4e01", "\u4e02", "\u4e03",
	}
	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"%s":%d`, k, i))
	}
	sb.WriteString("}")
	return []byte(sb.String())
}
