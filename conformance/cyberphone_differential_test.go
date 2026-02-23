package conformance_test

import (
	"bytes"
	"strings"
	"testing"

	cyberphone "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// These vectors document observed cases where the Cyberphone Go canonicalizer
// accepts and rewrites non-compliant inputs that json-canon rejects.
func TestCyberphoneGoDifferentialInvalidAcceptance(t *testing.T) {
	h := testHarness(t)

	type testCase struct {
		name        string
		input       []byte
		cyberOutput []byte
		wantClass   string
	}

	cases := []testCase{
		{
			name:        "hex_float_literal",
			input:       []byte(`{"n":0x1p-2}`),
			cyberOutput: []byte(`{"n":0.25}`),
			wantClass:   "INVALID_GRAMMAR",
		},
		{
			name:        "plus_prefixed_number",
			input:       []byte(`{"n":+1}`),
			cyberOutput: []byte(`{"n":1}`),
			wantClass:   "INVALID_GRAMMAR",
		},
		{
			name:        "leading_zero_number",
			input:       []byte(`{"n":01}`),
			cyberOutput: []byte(`{"n":1}`),
			wantClass:   "INVALID_GRAMMAR",
		},
		{
			name:        "invalid_utf8_in_string",
			input:       []byte{'{', '"', 's', '"', ':', '"', 0xff, '"', '}'},
			cyberOutput: []byte{'{', '"', 's', '"', ':', '"', 0xff, '"', '}'},
			wantClass:   "INVALID_UTF8",
		},
		{
			name:        "invalid_surrogate_pair",
			input:       []byte(`{"s":"\uD800\u0041"}`),
			cyberOutput: []byte("{\"s\":\"\uFFFD\"}"),
			wantClass:   "LONE_SURROGATE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCyber, err := cyberphone.Transform(tc.input)
			if err != nil {
				t.Fatalf("cyberphone unexpectedly rejected input: %v", err)
			}
			if !bytes.Equal(gotCyber, tc.cyberOutput) {
				t.Fatalf("cyberphone output mismatch got=%q want=%q", gotCyber, tc.cyberOutput)
			}

			res := runCLI(t, h, []string{"canonicalize", "-"}, tc.input)
			if res.exitCode != 2 {
				t.Fatalf("json-canon expected exit 2, got=%d stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
			}
			if !strings.Contains(res.stderr, tc.wantClass) {
				t.Fatalf("json-canon stderr missing class %q: %q", tc.wantClass, res.stderr)
			}
		})
	}
}
