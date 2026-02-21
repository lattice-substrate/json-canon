// Package jcs implements RFC 8785 JSON Canonicalization Scheme serialization.
//
// Given a parsed Value tree (from jcstoken), this package produces the exact
// canonical byte sequence specified by RFC 8785. It depends on jcsfloat for
// ECMA-262-compliant number serialization and uses UTF-16 code-unit ordering
// for object property name sorting as required by RFC 8785 §3.2.3.
package jcs

import (
	"fmt"
	"sort"
	"unicode/utf16"

	"lattice-canon/jcsfloat"
	"lattice-canon/jcstoken"
)

// Serialize produces the RFC 8785 JCS canonical byte sequence for a parsed
// JSON value. No trailing LF is appended (that is the GJCS1 envelope's concern).
//
// The output is deterministic: for any given value tree, the output bytes are
// always identical. This is the core invariant that enables byte-identical replay.
func Serialize(v *jcstoken.Value) ([]byte, error) {
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
		return append(buf, v.Str...), nil // "true" or "false"
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
		b := s[i]
		switch {
		case b == '"':
			buf = append(buf, '\\', '"')
			i++
		case b == '\\':
			buf = append(buf, '\\', '\\')
			i++
		case b == '\b':
			buf = append(buf, '\\', 'b')
			i++
		case b == '\t':
			buf = append(buf, '\\', 't')
			i++
		case b == '\n':
			buf = append(buf, '\\', 'n')
			i++
		case b == '\f':
			buf = append(buf, '\\', 'f')
			i++
		case b == '\r':
			buf = append(buf, '\\', 'r')
			i++
		case b < 0x20:
			// Other control characters: \u00xx with lowercase hex
			buf = append(buf, '\\', 'u', '0', '0',
				hexDigit(b>>4), hexDigit(b&0x0F))
			i++
		default:
			// Raw UTF-8 byte(s) — no escaping
			// For multi-byte sequences, just copy all bytes of the character
			if b < 0x80 {
				buf = append(buf, b)
				i++
			} else {
				// Find the length of this UTF-8 sequence and copy it verbatim
				size := utf8SeqLen(b)
				if i+size > len(s) {
					// Should not happen with valid input from jcstoken
					size = len(s) - i
				}
				buf = append(buf, s[i:i+size]...)
				i += size
			}
		}
	}
	buf = append(buf, '"')
	return buf
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
	// Sort members by key using UTF-16 code-unit ordering (RFC 8785 §3.2.3)
	sorted := make([]jcstoken.Member, len(v.Members))
	copy(sorted, v.Members)
	sort.SliceStable(sorted, func(i, j int) bool {
		return compareUTF16(sorted[i].Key, sorted[j].Key) < 0
	})

	buf = append(buf, '{')
	for i := range sorted {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = serializeString(buf, sorted[i].Key)
		buf = append(buf, ':')
		var err error
		buf, err = serializeValue(buf, &sorted[i].Value)
		if err != nil {
			return nil, err
		}
	}
	buf = append(buf, '}')
	return buf, nil
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
