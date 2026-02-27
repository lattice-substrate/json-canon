package conformance_test

import (
	"strings"
	"testing"
)

// These vectors document recorded cases where the Cyberphone Go canonicalizer
// accepted and rewrote non-compliant inputs that json-canon rejects.
// They intentionally avoid importing external modules to keep go.mod dependency-free.
func TestCyberphoneGoDifferentialInvalidAcceptance(t *testing.T) {
	h := testHarness(t)

	type testCase struct {
		name                string
		input               []byte
		recordedCyberOutput []byte
		wantClass           string
	}

	cases := []testCase{
		{
			name:                "hex_float_literal",
			input:               []byte(`{"n":0x1p-2}`),
			recordedCyberOutput: []byte(`{"n":0.25}`),
			wantClass:           "INVALID_GRAMMAR",
		},
		{
			name:                "plus_prefixed_number",
			input:               []byte(`{"n":+1}`),
			recordedCyberOutput: []byte(`{"n":1}`),
			wantClass:           "INVALID_GRAMMAR",
		},
		{
			name:                "leading_zero_number",
			input:               []byte(`{"n":01}`),
			recordedCyberOutput: []byte(`{"n":1}`),
			wantClass:           "INVALID_GRAMMAR",
		},
		{
			name:                "invalid_utf8_in_string",
			input:               []byte{'{', '"', 's', '"', ':', '"', 0xff, '"', '}'},
			recordedCyberOutput: []byte{'{', '"', 's', '"', ':', '"', 0xff, '"', '}'},
			wantClass:           "INVALID_UTF8",
		},
		{
			name:                "invalid_surrogate_pair",
			input:               []byte(`{"s":"\uD800\u0041"}`),
			recordedCyberOutput: []byte("{\"s\":\"\uFFFD\"}"),
			wantClass:           "LONE_SURROGATE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.recordedCyberOutput) == 0 {
				t.Fatalf("recorded Cyberphone output must be present for case %q", tc.name)
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
