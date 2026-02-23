package jcserr_test

import (
	"errors"
	"testing"

	"github.com/SolutionsExcite/json-canon/jcserr"
)

func TestFailureClassExitCodes(t *testing.T) {
	cases := []struct {
		class    jcserr.FailureClass
		wantExit int
	}{
		{jcserr.InvalidUTF8, 2},
		{jcserr.InvalidGrammar, 2},
		{jcserr.DuplicateKey, 2},
		{jcserr.LoneSurrogate, 2},
		{jcserr.Noncharacter, 2},
		{jcserr.NumberOverflow, 2},
		{jcserr.NumberNegZero, 2},
		{jcserr.NumberUnderflow, 2},
		{jcserr.BoundExceeded, 2},
		{jcserr.NotCanonical, 2},
		{jcserr.CLIUsage, 2},
		{jcserr.InternalIO, 10},
		{jcserr.InternalError, 10},
	}
	for _, tc := range cases {
		if got := tc.class.ExitCode(); got != tc.wantExit {
			t.Errorf("%s.ExitCode() = %d, want %d", tc.class, got, tc.wantExit)
		}
	}
}

func TestErrorFormat(t *testing.T) {
	e := jcserr.New(jcserr.InvalidUTF8, 42, "bad byte 0xFF")
	if e.Error() != "jcserr: INVALID_UTF8 at byte 42: bad byte 0xFF" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}

func TestErrorFormatNoOffset(t *testing.T) {
	e := jcserr.New(jcserr.InternalError, -1, "unexpected state")
	if e.Error() != "jcserr: INTERNAL_ERROR: unexpected state" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying")
	e := jcserr.Wrap(jcserr.InternalIO, -1, "write failed", cause)
	if !errors.Is(e, cause) {
		t.Fatal("Unwrap did not return cause")
	}
	if got := e.Error(); got != "jcserr: INTERNAL_IO: write failed: underlying" {
		t.Fatalf("unexpected wrapped error string: %s", got)
	}
}

func TestErrorAs(t *testing.T) {
	e := jcserr.New(jcserr.DuplicateKey, 10, "duplicate key \"a\"")
	var target *jcserr.Error
	if !errors.As(e, &target) {
		t.Fatal("errors.As failed")
	}
	if target.Class != jcserr.DuplicateKey {
		t.Fatalf("class = %s, want DUPLICATE_KEY", target.Class)
	}
}
