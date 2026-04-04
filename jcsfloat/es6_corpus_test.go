package jcsfloat_test

// ES6 corpus checksum tests verify that jcsfloat.FormatDouble produces the exact
// byte sequence defined by the ECMAScript Number::toString algorithm across a
// deterministic sweep of 10K (CI) and 100M (opt-in) representative float64 values.
// The expected checksums are derived from the cyberphone/json-canonicalization
// reference implementation.

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/jcsfloat"
)

const (
	officialES6Checksum10K  = "b9f7a8e75ef22a835685a52ccba7f7d6bdc99e34b010992cbc5864cd12be6892"
	officialES6Checksum100M = "0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272"
)

type officialES6Target struct {
	lines int
	sum   string
}

func TestOfficialES6CorpusChecksums10K(t *testing.T) {
	verifyOfficialES6Checksums(t, []officialES6Target{{lines: 10_000, sum: officialES6Checksum10K}})
}

func TestOfficialES6CorpusChecksums100M(t *testing.T) {
	if es6LookupEnvTrimmed("JCS_OFFICIAL_ES6_ENABLE_100M") != "1" {
		t.Skip("set JCS_OFFICIAL_ES6_ENABLE_100M=1 to run 100M official ES6 checksum gate")
	}
	verifyOfficialES6Checksums(t, []officialES6Target{{lines: 100_000_000, sum: officialES6Checksum100M}})
}

func verifyOfficialES6Checksums(t *testing.T, targets []officialES6Target) {
	t.Helper()
	if len(targets) == 0 {
		t.Fatal("at least one ES6 checksum target is required")
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].lines < targets[j].lines
	})
	for i := range targets {
		if targets[i].lines < 1 {
			t.Fatalf("invalid target lines: %d", targets[i].lines)
		}
	}

	next := jcsfloat.NewOfficialES6Generator()
	h := sha256.New()
	line := make([]byte, 0, 96)
	targetIdx := 0
	maxLines := targets[len(targets)-1].lines

	for i := 1; i <= maxLines; i++ {
		f := next()
		formatted, fmtErr := jcsfloat.FormatDouble(f)
		if fmtErr != nil {
			t.Fatalf("line %d unexpected format error: %v", i, fmtErr)
		}
		line = strconv.AppendUint(line[:0], math.Float64bits(f), 16)
		line = append(line, ',')
		line = append(line, formatted...)
		line = append(line, '\n')
		if _, err := h.Write(line); err != nil {
			t.Fatalf("line %d checksum write failed: %v", i, err)
		}
		for targetIdx < len(targets) && i == targets[targetIdx].lines {
			got := fmt.Sprintf("%x", h.Sum(nil))
			want := strings.ToLower(strings.TrimSpace(targets[targetIdx].sum))
			if got != want {
				t.Fatalf("ES6 checksum mismatch lines=%d got=%s want=%s", targets[targetIdx].lines, got, want)
			}
			targetIdx++
		}
	}

	if targetIdx != len(targets) {
		t.Fatalf("unverified targets: verified=%d total=%d", targetIdx, len(targets))
	}
}

func es6LookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

