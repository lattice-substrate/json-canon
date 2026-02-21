package jcstoken_test

import (
	"bytes"
	"testing"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

// FuzzParseCanonicalRoundTrip: parse → serialize → parse → serialize idempotence.
func FuzzParseCanonicalRoundTrip(f *testing.F) {
	seeds := [][]byte{
		[]byte(`null`),
		[]byte(`true`),
		[]byte(`{"a":1,"z":[3,2,1]}`),
		[]byte(`{"\uE000":1,"\uD800\uDC00":2}`),
		[]byte(`"a\/b"`),
		[]byte(`1e21`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) > 1<<20 {
			return
		}

		v, err := jcstoken.Parse(in)
		if err != nil {
			return
		}

		out1, err := jcs.Serialize(v)
		if err != nil {
			t.Fatalf("serialize parsed value: %v", err)
		}

		v2, err := jcstoken.Parse(out1)
		if err != nil {
			t.Fatalf("reparse canonical output: %v", err)
		}
		out2, err := jcs.Serialize(v2)
		if err != nil {
			t.Fatalf("reserialize canonical output: %v", err)
		}
		if !bytes.Equal(out1, out2) {
			t.Fatalf("non-deterministic canonical bytes: %q vs %q", out1, out2)
		}
	})
}
