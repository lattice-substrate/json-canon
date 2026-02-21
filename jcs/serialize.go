// Package jcs implements RFC 8785 JSON Canonicalization Scheme serialization.
//
// Given a parsed Value tree (from jcstoken), this package produces the exact
// canonical byte sequence specified by RFC 8785. It depends on jcsfloat for
// ECMA-262-compliant number serialization and uses UTF-16 code-unit ordering
// for object property name sorting as required by RFC 8785 §3.2.3.
package jcs

import (
	"fmt"
	"math"
	"sort"
	"unicode/utf16"
	"unicode/utf8"

	"jcs-canon/jcsfloat"
	"jcs-canon/jcstoken"
)

// Serialize produces the RFC 8785 JCS canonical byte sequence for a parsed
// JSON value.
//
// The output is deterministic: for any given value tree, the output bytes are
// always identical. This is the core invariant that enables byte-identical replay.
func Serialize(v *jcstoken.Value) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("jcs: nil value")
	}
	state := &serializeValidationState{}
	if err := validateValueTree(v, 0, state); err != nil {
		return nil, err
	}

	var buf []byte
	var err error
	buf, err = serializeValue(buf, v)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func serializeValue(buf []byte, v *jcstoken.Value) ([]byte, error) {
	switch v.Kind {
	case jcstoken.KindNull:
		return append(buf, "null"...), nil
	case jcstoken.KindBool:
		return append(buf, v.Str...), nil // validated as "true" or "false"
	case jcstoken.KindNumber:
		return serializeNumber(buf, v.Num)
	case jcstoken.KindString:
		return serializeString(buf, v.Str), nil
	case jcstoken.KindArray:
		return serializeArray(buf, v)
	case jcstoken.KindObject:
		return serializeObject(buf, v)
	default:
		return nil, fmt.Errorf("jcs: unknown value kind %d", v.Kind)
	}
}

// serializeNumber uses jcsfloat.FormatDouble for ECMA-262 compliant output.
func serializeNumber(buf []byte, f float64) ([]byte, error) {
	s, err := jcsfloat.FormatDouble(f)
	if err != nil {
		return nil, fmt.Errorf("jcs: number serialization error: %w", err)
	}
	return append(buf, s...), nil
}

// serializeString applies JCS string escaping rules (RFC 8785 §3.2.2.2):
//   - " → \"
//   - \ → \\
//   - U+0008 → \b, U+0009 → \t, U+000A → \n, U+000C → \f, U+000D → \r
//   - Other control chars U+0000-U+001F → \u00xx (lowercase hex)
//   - Everything else: raw UTF-8, no escaping
//   - No Unicode normalization
func serializeString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	for i := 0; i < len(s); {
		next, consumed := appendEscapedByte(buf, s[i])
		if consumed {
			buf = next
			i++
			continue
		}

		size := byteSpanForCopy(s, i)
		buf = append(buf, s[i:i+size]...)
		i += size
	}
	buf = append(buf, '"')
	return buf
}

func appendEscapedByte(buf []byte, b byte) ([]byte, bool) {
	switch b {
	case '"':
		return append(buf, '\\', '"'), true
	case '\\':
		return append(buf, '\\', '\\'), true
	case '\b':
		return append(buf, '\\', 'b'), true
	case '\t':
		return append(buf, '\\', 't'), true
	case '\n':
		return append(buf, '\\', 'n'), true
	case '\f':
		return append(buf, '\\', 'f'), true
	case '\r':
		return append(buf, '\\', 'r'), true
	default:
		if b < 0x20 {
			return append(buf, '\\', 'u', '0', '0', hexDigit(b>>4), hexDigit(b&0x0F)), true
		}
		return buf, false
	}
}

func byteSpanForCopy(s string, i int) int {
	b := s[i]
	if b < 0x80 {
		return 1
	}

	size := utf8SeqLen(b)
	if i+size > len(s) {
		return len(s) - i
	}
	return size
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'a' + (b - 10)
}

// utf8SeqLen returns the byte length of a UTF-8 sequence from its leading byte.
func utf8SeqLen(b byte) int {
	switch {
	case b < 0x80:
		return 1
	case b < 0xE0:
		return 2
	case b < 0xF0:
		return 3
	default:
		return 4
	}
}

func serializeArray(buf []byte, v *jcstoken.Value) ([]byte, error) {
	buf = append(buf, '[')
	for i := range v.Elems {
		if i > 0 {
			buf = append(buf, ',')
		}
		var err error
		buf, err = serializeValue(buf, &v.Elems[i])
		if err != nil {
			return nil, err
		}
	}
	buf = append(buf, ']')
	return buf, nil
}

func serializeObject(buf []byte, v *jcstoken.Value) ([]byte, error) {
	// Sort members by key using UTF-16 code-unit ordering (RFC 8785 §3.2.3).
	// UTF-16 encodings are precomputed once per key to avoid repeated allocations.
	sorted := make([]sortableMember, len(v.Members))
	for i := range v.Members {
		sorted[i] = sortableMember{
			member: v.Members[i],
			key16:  utf16.Encode([]rune(v.Members[i].Key)),
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return compareUTF16Units(sorted[i].key16, sorted[j].key16) < 0
	})

	buf = append(buf, '{')
	for i := range sorted {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = serializeString(buf, sorted[i].member.Key)
		buf = append(buf, ':')
		var err error
		buf, err = serializeValue(buf, &sorted[i].member.Value)
		if err != nil {
			return nil, err
		}
	}
	buf = append(buf, '}')
	return buf, nil
}

type sortableMember struct {
	member jcstoken.Member
	key16  []uint16
}

// compareUTF16 compares two Go strings by their UTF-16 code-unit arrays,
// as required by RFC 8785 §3.2.3.
//
// For BMP-only strings, this produces the same order as a simple byte
// comparison. It diverges for supplementary-plane characters (U+10000+),
// where UTF-16 surrogate pair code units sort differently than the
// corresponding UTF-8 byte sequences.
func compareUTF16(a, b string) int {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	return compareUTF16Units(ua, ub)
}

func compareUTF16Units(ua, ub []uint16) int {
	minLen := len(ua)
	if len(ub) < minLen {
		minLen = len(ub)
	}
	for i := 0; i < minLen; i++ {
		if ua[i] < ub[i] {
			return -1
		}
		if ua[i] > ub[i] {
			return 1
		}
	}
	if len(ua) < len(ub) {
		return -1
	}
	if len(ua) > len(ub) {
		return 1
	}
	return 0
}

type serializeValidationState struct {
	values int
}

func validateValueTree(v *jcstoken.Value, depth int, state *serializeValidationState) error {
	state.values++
	if state.values > jcstoken.DefaultMaxValues {
		return fmt.Errorf("jcs: value count exceeds maximum %d", jcstoken.DefaultMaxValues)
	}
	if depth > jcstoken.DefaultMaxDepth {
		return fmt.Errorf("jcs: value nesting depth exceeds maximum %d", jcstoken.DefaultMaxDepth)
	}

	switch v.Kind {
	case jcstoken.KindNull:
		return nil
	case jcstoken.KindBool:
		if v.Str != "true" && v.Str != "false" {
			return fmt.Errorf("jcs: invalid boolean payload %q", v.Str)
		}
		return nil
	case jcstoken.KindNumber:
		if math.IsNaN(v.Num) || math.IsInf(v.Num, 0) {
			return fmt.Errorf("jcs: number is not finite")
		}
		return nil
	case jcstoken.KindString:
		if err := validateString(v.Str); err != nil {
			return err
		}
		return nil
	case jcstoken.KindArray:
		if len(v.Elems) > jcstoken.DefaultMaxArrayElements {
			return fmt.Errorf("jcs: array element count exceeds maximum %d", jcstoken.DefaultMaxArrayElements)
		}
		for i := range v.Elems {
			if err := validateValueTree(&v.Elems[i], depth+1, state); err != nil {
				return err
			}
		}
		return nil
	case jcstoken.KindObject:
		if len(v.Members) > jcstoken.DefaultMaxObjectMembers {
			return fmt.Errorf("jcs: object member count exceeds maximum %d", jcstoken.DefaultMaxObjectMembers)
		}
		seen := make(map[string]struct{}, len(v.Members))
		for i := range v.Members {
			if err := validateString(v.Members[i].Key); err != nil {
				return fmt.Errorf("jcs: invalid object key: %w", err)
			}
			if _, ok := seen[v.Members[i].Key]; ok {
				return fmt.Errorf("jcs: duplicate object key %q", v.Members[i].Key)
			}
			seen[v.Members[i].Key] = struct{}{}
			if err := validateValueTree(&v.Members[i].Value, depth+1, state); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("jcs: unknown value kind %d", v.Kind)
	}
}

func validateString(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("jcs: string is not valid UTF-8")
	}
	if len(s) > jcstoken.DefaultMaxStringBytes {
		return fmt.Errorf("jcs: string length exceeds maximum %d bytes", jcstoken.DefaultMaxStringBytes)
	}
	for _, r := range s {
		if isNoncharacter(r) {
			return fmt.Errorf("jcs: string contains noncharacter U+%04X", r)
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return fmt.Errorf("jcs: string contains surrogate code point U+%04X", r)
		}
	}
	return nil
}

func isNoncharacter(r rune) bool {
	if r >= 0xFDD0 && r <= 0xFDEF {
		return true
	}
	return r <= 0x10FFFF && (r&0xFFFE == 0xFFFE)
}
