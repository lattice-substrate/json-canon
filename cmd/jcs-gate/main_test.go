package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

type fakeRunner struct {
	calls  []string
	failAt int
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string, _ io.Writer, _ io.Writer) error {
	f.calls = append(f.calls, fmt.Sprintf("%s %v", name, args))
	if f.failAt > 0 && len(f.calls) == f.failAt {
		return errors.New("boom")
	}
	return nil
}

func TestRunHelp(t *testing.T) {
	fr := &fakeRunner{}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--help"}, &out, &errOut, fr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("expected no command invocations, got %d", len(fr.calls))
	}
}

func TestRunExecutesAllRequiredGates(t *testing.T) {
	fr := &fakeRunner{}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run(nil, &out, &errOut, fr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, errOut.String())
	}
	if len(fr.calls) != len(requiredGateSteps) {
		t.Fatalf("expected %d calls, got %d", len(requiredGateSteps), len(fr.calls))
	}
}

func TestRunStopsOnFirstFailure(t *testing.T) {
	fr := &fakeRunner{failAt: 3}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run(nil, &out, &errOut, fr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if len(fr.calls) != 3 {
		t.Fatalf("expected to stop at failing gate, got %d calls", len(fr.calls))
	}
}

func TestRunUnknownArgument(t *testing.T) {
	fr := &fakeRunner{}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--nope"}, &out, &errOut, fr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("expected no command invocations, got %d", len(fr.calls))
	}
}
