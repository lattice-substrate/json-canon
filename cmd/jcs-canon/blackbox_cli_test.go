package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var (
	buildBlackboxOnce sync.Once
	blackboxBin       string
	errBlackboxBuild  error
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func blackboxBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	buildBlackboxOnce.Do(func() {
		dir, err := os.MkdirTemp("", "jcs-canon-blackbox-*")
		if err != nil {
			errBlackboxBuild = err
			return
		}
		blackboxBin = filepath.Join(dir, "jcs-canon")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", blackboxBin, "./cmd/jcs-canon",
		)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		errBlackboxBuild = cmd.Run()
	})
	if errBlackboxBuild != nil {
		t.Fatalf("build blackbox binary: %v", errBlackboxBuild)
	}
	return blackboxBin
}

func runBlackbox(t *testing.T, args []string, stdin []byte) (int, []byte, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, blackboxBinary(t), args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.Bytes(), stderr.Bytes()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), stdout.Bytes(), stderr.Bytes()
	}
	t.Fatalf("run blackbox: %v", err)
	return 0, nil, nil
}

func TestBlackboxCanonicalizeVector(t *testing.T) {
	input, err := os.ReadFile(filepath.Join(repoRoot(t), "cmd", "jcs-canon", "testdata", "vectors", "canonical_minimal.json"))
	if err != nil {
		t.Fatalf("read vector: %v", err)
	}
	code, stdout, stderr := runBlackbox(t, []string{"canonicalize", "-"}, input)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, string(stderr))
	}
	if string(stdout) != `{"a":1}` {
		t.Fatalf("got %q", string(stdout))
	}
	if len(stderr) != 0 {
		t.Fatalf("expected empty stderr, got %q", string(stderr))
	}
}

func TestBlackboxVerifyRejectsNonCanonicalVector(t *testing.T) {
	input, err := os.ReadFile(filepath.Join(repoRoot(t), "cmd", "jcs-canon", "testdata", "vectors", "noncanonical_ws.json"))
	if err != nil {
		t.Fatalf("read vector: %v", err)
	}
	code, _, stderr := runBlackbox(t, []string{"verify", "--quiet", "-"}, input)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d stderr=%q", code, string(stderr))
	}
	if !bytes.Contains(stderr, []byte("NOT_CANONICAL")) {
		t.Fatalf("unexpected stderr: %q", string(stderr))
	}
}

func TestBlackboxTopLevelHelpExitZero(t *testing.T) {
	code, stdout, stderr := runBlackbox(t, []string{"--help"}, nil)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, string(stderr))
	}
	if !bytes.Contains(stdout, []byte("usage: jcs-canon")) {
		t.Fatalf("unexpected help output: %q", string(stdout))
	}
	if len(stderr) != 0 {
		t.Fatalf("expected empty stderr, got %q", string(stderr))
	}
}

func TestBlackboxTopLevelVersionExitZero(t *testing.T) {
	code, stdout, stderr := runBlackbox(t, []string{"--version"}, nil)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, string(stderr))
	}
	if !bytes.HasPrefix(bytes.TrimSpace(stdout), []byte("jcs-canon v")) {
		t.Fatalf("unexpected version output: %q", string(stdout))
	}
	if len(stderr) != 0 {
		t.Fatalf("expected empty stderr, got %q", string(stderr))
	}
}
