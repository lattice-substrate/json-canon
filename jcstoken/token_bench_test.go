package jcstoken_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/jcstoken"
)

func BenchmarkParse(b *testing.B) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"small_object", []byte(`{"a":1,"b":"hello","c":true,"d":null}`)},
		{"nested_10_deep", buildNestedObject(10)},
		{"array_1000", buildLargeArray(1000)},
		{"strings_no_escape", buildStringArray(100, "hello world")},
		{"strings_with_escapes", buildStringArray(100, `hello\nworld\t\"quoted\"`)},
		{"diverse_numbers", buildNumberArray()},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.input)))
			for i := 0; i < b.N; i++ {
				if _, err := jcstoken.Parse(tc.input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func buildNestedObject(depth int) []byte {
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

func buildLargeArray(n int) []byte {
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

func buildStringArray(n int, s string) []byte {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"`)
		sb.WriteString(s)
		sb.WriteString(`"`)
	}
	sb.WriteString("]")
	return []byte(sb.String())
}

func buildNumberArray() []byte {
	numbers := []string{
		"0", "1", "-1", "3.14", "1e10", "1.5e-3", "999999999",
		"1.7976931348623157e308", "2.2250738585072014e-308",
		"0.1", "0.01", "100", "123456789", "1.23456789",
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, n := range numbers {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(n)
	}
	sb.WriteString("]")
	return []byte(sb.String())
}
