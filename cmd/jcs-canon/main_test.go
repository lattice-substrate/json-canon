package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lattice-substrate/json-canon/jcserr"
)

func TestWriteClassifiedErrorWrapped(t *testing.T) {
	inner := jcserr.New(jcserr.InvalidUTF8, 3, "bad byte")
	err := fmt.Errorf("outer: %w", inner)
	var stderr bytes.Buffer
	code := writeClassifiedError(&stderr, err)
	if code != jcserr.InvalidUTF8.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InvalidUTF8.ExitCode(), code)
	}
}

func TestWriteClassifiedErrorFallback(t *testing.T) {
	err := fmt.Errorf("unclassified failure")
	var stderr bytes.Buffer
	code := writeClassifiedError(&stderr, err)
	if code != jcserr.InternalError.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalError.ExitCode(), code)
	}
}
