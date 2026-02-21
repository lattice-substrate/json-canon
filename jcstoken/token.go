// Package jcstoken provides a strict JSON tokenizer that enforces the
// lattice-canon §3.3 strict input domain constraints.
//
// This tokenizer is not a general-purpose JSON parser. It is purpose-built
// for JCS canonicalization and rejects inputs that standard JSON parsers
// would accept, including duplicate keys, lone surrogates, noncharacters,
// and the -0 numeric token.
//
// The output is an ordered tree of JSON values suitable for canonical
// re-serialization.
package jcstoken

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Limits for denial-of-service protection.
const (
	// DefaultMaxDepth is the maximum nesting depth for objects and arrays.
	DefaultMaxDepth = 1000

	// DefaultMaxInputSize is the maximum input size in bytes (64 MiB).
	DefaultMaxInputSize = 64 * 1024 * 1024
)

// Value represents a parsed JSON value.
type Value struct {
	Kind    Kind
	Str     string   // For KindString: the decoded Unicode string; for KindBool: "true" or "false"
	Num     float64  // For KindNumber: the IEEE 754 double
	Members []Member // For KindObject: ordered members (as parsed)
	Elems   []Value  // For KindArray: ordered elements
}

// Kind identifies the type of a JSON value.
type Kind int

const (
	// KindNull identifies a JSON null value.
	KindNull Kind = iota
	// KindBool identifies a JSON boolean value.
	KindBool
	// KindNumber identifies a JSON number value.
	KindNumber
	// KindString identifies a JSON string value.
	KindString
	// KindArray identifies a JSON array value.
	KindArray
	// KindObject identifies a JSON object value.
	KindObject
)

// Member is a key-value pair in a JSON object.
type Member struct {
	Key   string
	Value Value
}

// ParseError is returned when the input violates a constraint.
type ParseError struct {
	Offset int
	Msg    string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("jcstoken: at byte %d: %s", e.Offset, e.Msg)
}

// Options controls parser behavior.
type Options struct {
	MaxDepth     int // 0 means DefaultMaxDepth
	MaxInputSize int // 0 means DefaultMaxInputSize
}

func (o *Options) maxDepth() int {
	if o != nil && o.MaxDepth > 0 {
		return o.MaxDepth
	}
	return DefaultMaxDepth
}

func (o *Options) maxInputSize() int {
	if o != nil && o.MaxInputSize > 0 {
		return o.MaxInputSize
	}
	return DefaultMaxInputSize
}

// parser holds the state for parsing.
type parser struct {
	data     []byte
	pos      int
	depth    int
	maxDepth int
}

// Parse parses a complete JSON text under the strict domain constraints of
// lattice-canon §3.3. It returns the parsed value tree or a ParseError.
//
// Constraints enforced:
//   - No duplicate object member names (compared as decoded Unicode scalars)
//   - No lone surrogates in \uXXXX escapes
//   - No Unicode noncharacters in strings or member names
//   - No -0 numeric literal (or any token that parses to negative zero)
//   - No non-finite numbers
//   - No numeric tokens that underflow to zero (non-representable)
//   - Valid surrogate pairs decoded to supplementary-plane scalars
//   - Nesting depth bounded by MaxDepth
//   - Input size bounded by MaxInputSize
func Parse(data []byte) (*Value, error) {
	return ParseWithOptions(data, nil)
}

// ParseWithOptions is like Parse but accepts configuration options.
func ParseWithOptions(data []byte, opts *Options) (*Value, error) {
	maxInput := opts.maxInputSize()
	if len(data) > maxInput {
		return nil, &ParseError{
			Offset: 0,
			Msg:    fmt.Sprintf("input size %d exceeds maximum %d", len(data), maxInput),
		}
	}

	p := &parser{
		data:     data,
		pos:      0,
		depth:    0,
		maxDepth: opts.maxDepth(),
	}

	p.skipWhitespace()
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if p.pos != len(p.data) {
		return nil, p.errorf("trailing content after JSON value")
	}
	return v, nil
}

func (p *parser) errorf(format string, args ...any) *ParseError {
	return &ParseError{Offset: p.pos, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) peek() (byte, bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	return p.data[p.pos], true
}

func (p *parser) next() (byte, bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	b := p.data[p.pos]
	p.pos++
	return b, true
}

func (p *parser) expect(b byte) error {
	c, ok := p.next()
	if !ok {
		return p.errorf("unexpected end of input, expected %q", string(b))
	}
	if c != b {
		return p.errorf("expected %q, got %q", string(b), string(c))
	}
	return nil
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) pushDepth() error {
	p.depth++
	if p.depth > p.maxDepth {
		return p.errorf("nesting depth %d exceeds maximum %d", p.depth, p.maxDepth)
	}
	return nil
}

func (p *parser) popDepth() {
	p.depth--
}

func (p *parser) parseValue() (*Value, error) {
	c, ok := p.peek()
	if !ok {
		return nil, p.errorf("unexpected end of input")
	}

	switch c {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		return p.parseString()
	case 't', 'f':
		return p.parseBool()
	case 'n':
		return p.parseNull()
	default:
		return p.parseNumber()
	}
}

func (p *parser) parseObject() (*Value, error) {
	if err := p.pushDepth(); err != nil {
		return nil, err
	}
	defer p.popDepth()

	if err := p.expect('{'); err != nil {
		return nil, err
	}
	p.skipWhitespace()

	return p.parseObjectMembers()
}

func (p *parser) parseObjectMembers() (*Value, error) {
	v := &Value{Kind: KindObject}
	seen := make(map[string]int)

	empty, err := p.consumeEmptyObject()
	if err != nil {
		return nil, err
	}
	if empty {
		return v, nil
	}

	for {
		member, done, parseErr := p.parseObjectMember(seen)
		if parseErr != nil {
			return nil, parseErr
		}
		v.Members = append(v.Members, member)
		if done {
			return v, nil
		}
	}
}

func (p *parser) consumeEmptyObject() (bool, error) {
	p.skipWhitespace()
	c, ok := p.peek()
	if !ok {
		return false, p.errorf("unexpected end of input in object")
	}
	if c != '}' {
		return false, nil
	}
	p.pos++
	return true, nil
}

func (p *parser) parseObjectMember(seen map[string]int) (Member, bool, error) {
	p.skipWhitespace()
	keyStart := p.pos

	keyVal, err := p.parseString()
	if err != nil {
		return Member{}, false, err
	}
	key := keyVal.Str

	err = rejectDuplicateKey(seen, key, keyStart)
	if err != nil {
		return Member{}, false, err
	}

	err = p.expectObjectColon()
	if err != nil {
		return Member{}, false, err
	}
	val, err := p.parseValue()
	if err != nil {
		return Member{}, false, err
	}

	done, err := p.consumeObjectSeparator()
	if err != nil {
		return Member{}, false, err
	}
	return Member{Key: key, Value: *val}, done, nil
}

func rejectDuplicateKey(seen map[string]int, key string, keyStart int) error {
	if firstOff, exists := seen[key]; exists {
		return &ParseError{
			Offset: keyStart,
			Msg:    fmt.Sprintf("duplicate object key %q (first at byte %d)", key, firstOff),
		}
	}
	seen[key] = keyStart
	return nil
}

func (p *parser) expectObjectColon() error {
	p.skipWhitespace()
	if err := p.expect(':'); err != nil {
		return err
	}
	p.skipWhitespace()
	return nil
}

func (p *parser) consumeObjectSeparator() (bool, error) {
	p.skipWhitespace()
	c, ok := p.peek()
	if !ok {
		return false, p.errorf("unexpected end of input in object")
	}
	if c == '}' {
		p.pos++
		return true, nil
	}
	if c == ',' {
		p.pos++
		return false, nil
	}
	return false, p.errorf("expected ',' or '}' in object, got %q", string(c))
}

func (p *parser) parseArray() (*Value, error) {
	if err := p.pushDepth(); err != nil {
		return nil, err
	}
	defer p.popDepth()

	if err := p.expect('['); err != nil {
		return nil, err
	}
	p.skipWhitespace()

	v := &Value{Kind: KindArray}

	c, ok := p.peek()
	if !ok {
		return nil, p.errorf("unexpected end of input in array")
	}
	if c == ']' {
		p.pos++
		return v, nil
	}

	for {
		p.skipWhitespace()
		elem, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		v.Elems = append(v.Elems, *elem)

		p.skipWhitespace()
		c, ok := p.peek()
		if !ok {
			return nil, p.errorf("unexpected end of input in array")
		}
		if c == ']' {
			p.pos++
			return v, nil
		}
		if c == ',' {
			p.pos++
			continue
		}
		return nil, p.errorf("expected ',' or ']' in array, got %q", string(c))
	}
}

// parseString parses a JSON string and decodes all escapes. It enforces:
//   - No lone surrogates
//   - Valid surrogate pairs decoded to supplementary-plane scalars
//   - No Unicode noncharacters
func (p *parser) parseString() (*Value, error) {
	if err := p.expect('"'); err != nil {
		return nil, err
	}

	var buf []byte
	for {
		done, err := p.consumeStringChunk(&buf)
		if err != nil {
			return nil, err
		}
		if done {
			return &Value{Kind: KindString, Str: string(buf)}, nil
		}
	}
}

// parseEscape handles the character after '\'. Returns the decoded rune.
func (p *parser) parseEscape() (rune, error) {
	if p.pos >= len(p.data) {
		return 0, p.errorf("unterminated escape sequence")
	}
	b := p.data[p.pos]
	p.pos++

	if b == 'u' {
		return p.parseUnicodeEscape()
	}
	r, ok := escapedRune(b)
	if !ok {
		return 0, p.errorf("invalid escape character %q", string(b))
	}
	return r, nil
}

// parseUnicodeEscape parses \uXXXX (and \uXXXX\uXXXX for surrogate pairs).
func (p *parser) parseUnicodeEscape() (rune, error) {
	r1, err := p.readHex4()
	if err != nil {
		return 0, err
	}

	if !utf16.IsSurrogate(r1) {
		return r1, nil
	}
	if r1 >= 0xDC00 {
		return 0, p.errorf("lone low surrogate U+%04X", r1)
	}

	r2, err := p.readFollowingLowSurrogate(r1)
	if err != nil {
		return 0, err
	}

	decoded := utf16.DecodeRune(r1, r2)
	if decoded == unicode.ReplacementChar {
		return 0, p.errorf("invalid surrogate pair U+%04X U+%04X", r1, r2)
	}
	return decoded, nil
}

func escapedRune(b byte) (rune, bool) {
	switch b {
	case '"':
		return '"', true
	case '\\':
		return '\\', true
	case '/':
		return '/', true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	default:
		return 0, false
	}
}

func (p *parser) readFollowingLowSurrogate(high rune) (rune, error) {
	if p.pos+1 >= len(p.data) || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
		return 0, p.errorf("lone high surrogate U+%04X (no following \\u)", high)
	}
	p.pos += 2

	r2, err := p.readHex4()
	if err != nil {
		return 0, err
	}
	if r2 < 0xDC00 || r2 > 0xDFFF {
		return 0, p.errorf("high surrogate U+%04X followed by non-low-surrogate U+%04X", high, r2)
	}
	return r2, nil
}

func (p *parser) consumeStringChunk(buf *[]byte) (bool, error) {
	if p.pos >= len(p.data) {
		return false, p.errorf("unterminated string")
	}
	b := p.data[p.pos]
	if b == '"' {
		p.pos++
		s := string(*buf)
		if err := p.validateString(s); err != nil {
			return false, err
		}
		return true, nil
	}
	if b == '\\' {
		return false, p.consumeEscapedRune(buf)
	}
	if b < 0x20 {
		return false, p.errorf("unescaped control character 0x%02X in string", b)
	}
	return false, p.consumeUTF8Chunk(buf)
}

func (p *parser) consumeEscapedRune(buf *[]byte) error {
	p.pos++
	r, err := p.parseEscape()
	if err != nil {
		return err
	}
	var tmp [4]byte
	n := utf8.EncodeRune(tmp[:], r)
	*buf = append(*buf, tmp[:n]...)
	return nil
}

func (p *parser) consumeUTF8Chunk(buf *[]byte) error {
	b := p.data[p.pos]
	r, size := utf8.DecodeRune(p.data[p.pos:])
	if r == utf8.RuneError && size <= 1 {
		return p.errorf("invalid UTF-8 byte 0x%02X in string", b)
	}
	*buf = append(*buf, p.data[p.pos:p.pos+size]...)
	p.pos += size
	return nil
}

// readHex4 reads exactly 4 hex digits and returns the rune value.
func (p *parser) readHex4() (rune, error) {
	if p.pos+4 > len(p.data) {
		return 0, p.errorf("incomplete \\u escape")
	}
	hex := string(p.data[p.pos : p.pos+4])
	p.pos += 4
	val, err := strconv.ParseUint(hex, 16, 16)
	if err != nil {
		return 0, p.errorf("invalid hex in \\u escape: %q", hex)
	}
	return rune(val), nil
}

// validateString checks that a decoded string contains no Unicode noncharacters
// and no surrogate code points.
func (p *parser) validateString(s string) error {
	for i, r := range s {
		if isNoncharacter(r) {
			return &ParseError{
				Offset: p.pos - len(s) + i,
				Msg:    fmt.Sprintf("string contains Unicode noncharacter U+%04X", r),
			}
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return &ParseError{
				Offset: p.pos - len(s) + i,
				Msg:    fmt.Sprintf("string contains surrogate code point U+%04X", r),
			}
		}
	}
	return nil
}

// isNoncharacter returns true if r is a Unicode noncharacter.
// Noncharacters are: U+FDD0..U+FDEF and U+xFFFE, U+xFFFF for all planes 0-16.
func isNoncharacter(r rune) bool {
	if r >= 0xFDD0 && r <= 0xFDEF {
		return true
	}
	if r&0xFFFE == 0xFFFE && r <= 0x10FFFF {
		return true
	}
	return false
}

func (p *parser) parseNumber() (*Value, error) {
	start := p.pos

	p.consumeNumberSign()
	if err := p.scanIntegerPart(); err != nil {
		return nil, err
	}
	if err := p.scanFractionPart(); err != nil {
		return nil, err
	}
	if err := p.scanExponentPart(); err != nil {
		return nil, err
	}

	raw := string(p.data[start:p.pos])
	return p.buildNumberValue(start, raw)
}

func (p *parser) consumeNumberSign() {
	if p.pos < len(p.data) && p.data[p.pos] == '-' {
		p.pos++
	}
}

func (p *parser) scanIntegerPart() error {
	if p.pos >= len(p.data) {
		return p.errorf("unexpected end of input in number")
	}

	if p.data[p.pos] == '0' {
		p.pos++
		if p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			return p.errorf("leading zero in number")
		}
		return nil
	}

	if p.data[p.pos] < '1' || p.data[p.pos] > '9' {
		return p.errorf("invalid number character %q", string(p.data[p.pos]))
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

func (p *parser) scanFractionPart() error {
	if p.pos >= len(p.data) || p.data[p.pos] != '.' {
		return nil
	}
	p.pos++

	if p.pos >= len(p.data) || !isDigit(p.data[p.pos]) {
		return p.errorf("expected digit after decimal point")
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

func (p *parser) scanExponentPart() error {
	if !p.hasExponentMarker() {
		return nil
	}
	p.pos++

	p.consumeExponentSign()
	if !p.hasExponentDigit() {
		return p.errorf("expected digit in exponent")
	}
	p.consumeDigits()
	return nil
}

func (p *parser) buildNumberValue(start int, raw string) (*Value, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, &ParseError{Offset: start, Msg: fmt.Sprintf("invalid number: %v", err)}
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, &ParseError{Offset: start, Msg: "number overflows IEEE 754 double"}
	}
	if f == 0 && math.Signbit(f) {
		return nil, &ParseError{
			Offset: start,
			Msg:    "numeric token parses to negative zero (rejected per EID 7920)",
		}
	}
	if f == 0 && !isZeroToken(raw) {
		return nil, &ParseError{
			Offset: start,
			Msg:    fmt.Sprintf("numeric token %q underflows to zero (not representable as IEEE 754 double)", raw),
		}
	}
	return &Value{Kind: KindNumber, Num: f}, nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func (p *parser) hasExponentMarker() bool {
	return p.pos < len(p.data) && (p.data[p.pos] == 'e' || p.data[p.pos] == 'E')
}

func (p *parser) consumeExponentSign() {
	if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
		p.pos++
	}
}

func (p *parser) hasExponentDigit() bool {
	return p.pos < len(p.data) && isDigit(p.data[p.pos])
}

func (p *parser) consumeDigits() {
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
}

// isZeroToken returns true if the raw JSON number token represents exactly zero.
// Zero tokens are: "0", "-0", "0.0", "0.00", "0e0", "0.0e5", etc.
// (Note: -0 is caught separately before this is called, but we include it
// for completeness.)
func isZeroToken(raw string) bool {
	i := 0
	if i < len(raw) && raw[i] == '-' {
		i++
	}
	// All digit characters in the significand must be '0'
	for i < len(raw) && raw[i] != 'e' && raw[i] != 'E' {
		if raw[i] != '0' && raw[i] != '.' {
			return false
		}
		i++
	}
	// Exponent doesn't matter if significand is all zeros
	return true
}

func (p *parser) parseBool() (*Value, error) {
	if p.pos+4 <= len(p.data) && string(p.data[p.pos:p.pos+4]) == "true" {
		p.pos += 4
		return &Value{Kind: KindBool, Str: "true"}, nil
	}
	if p.pos+5 <= len(p.data) && string(p.data[p.pos:p.pos+5]) == "false" {
		p.pos += 5
		return &Value{Kind: KindBool, Str: "false"}, nil
	}
	return nil, p.errorf("invalid literal")
}

func (p *parser) parseNull() (*Value, error) {
	if p.pos+4 <= len(p.data) && string(p.data[p.pos:p.pos+4]) == "null" {
		p.pos += 4
		return &Value{Kind: KindNull}, nil
	}
	return nil, p.errorf("invalid literal")
}

// Exported sentinel errors for use by callers.
var (
	ErrDuplicateKey  = errors.New("duplicate object key")
	ErrLoneSurrogate = errors.New("lone surrogate")
	ErrNoncharacter  = errors.New("noncharacter")
	ErrNegativeZero  = errors.New("negative zero token")
)
