package main_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type cliVector struct {
	Name           string   `json:"name"`
	Args           []string `json:"args"`
	Stdin          string   `json:"stdin"`
	StdinHex       string   `json:"stdin_hex"`
	ExitCode       int      `json:"exit_code"`
	Stdout         string   `json:"stdout"`
	StderrContains []string `json:"stderr_contains"`
}

type cliResult struct {
	exitCode int
	stdout   string
	stderr   string
}

var (
	buildBinaryOnce sync.Once
	cachedBinary    string
	errCachedBuild  error
)

func TestCLICanonicalizeVectors(t *testing.T) {
	t.Parallel()

	vectors := loadVectors(t, "canonicalize.json")
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			input := vectorInput(t, v)
			res := runCLI(t, v.Args, input)
			assertCLIResult(t, v, res)
		})
	}
}

func TestCLIVerifyVectors(t *testing.T) {
	t.Parallel()

	vectors := loadVectors(t, "verify.json")
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			input := vectorInput(t, v)
			res := runCLI(t, v.Args, input)
			assertCLIResult(t, v, res)
		})
	}
}

func TestCLIAdversarialDepthBomb(t *testing.T) {
	t.Parallel()

	depth := 1200
	input := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	res := runCLI(t, []string{"canonicalize", "-"}, []byte(input))
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d stderr=%q", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stderr, "nesting depth") {
		t.Fatalf("expected nesting-depth error, got stderr=%q", res.stderr)
	}
}

func TestCLIAdversarialHugeExponent(t *testing.T) {
	t.Parallel()

	res := runCLI(t, []string{"verify", "--quiet", "-"}, []byte("1e999999"))
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d stderr=%q", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stderr, "overflows IEEE 754 double") {
		t.Fatalf("expected range error, got stderr=%q", res.stderr)
	}
}

func TestCLIAdversarialUnknownCommand(t *testing.T) {
	t.Parallel()

	res := runCLI(t, []string{"bogus"}, nil)
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", res.exitCode)
	}
	if !strings.Contains(res.stderr, "unknown command") {
		t.Fatalf("expected unknown command error, got stderr=%q", res.stderr)
	}
}

func TestCLIDeterministicReplay(t *testing.T) {
	t.Parallel()

	input := []byte(`{"z":3,"a":1,"n":1e21}`)
	first := runCLI(t, []string{"canonicalize", "-"}, input)
	if first.exitCode != 0 {
		t.Fatalf("first run failed: %d stderr=%q", first.exitCode, first.stderr)
	}

	for i := 0; i < 20; i++ {
		res := runCLI(t, []string{"canonicalize", "-"}, input)
		if res.exitCode != 0 {
			t.Fatalf("iteration %d failed: %d stderr=%q", i, res.exitCode, res.stderr)
		}
		if res.stdout != first.stdout {
			t.Fatalf("iteration %d output mismatch: got %q want %q", i, res.stdout, first.stdout)
		}
	}
}

func TestCLIFileAndStdinParity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	input := []byte(`{"b":2,"a":1}`)
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	fromFile := runCLI(t, []string{"canonicalize", inputPath}, nil)
	if fromFile.exitCode != 0 {
		t.Fatalf("file mode failed: %d stderr=%q", fromFile.exitCode, fromFile.stderr)
	}

	fromStdin := runCLI(t, []string{"canonicalize", "-"}, input)
	if fromStdin.exitCode != 0 {
		t.Fatalf("stdin mode failed: %d stderr=%q", fromStdin.exitCode, fromStdin.stderr)
	}

	if fromFile.stdout != fromStdin.stdout {
		t.Fatalf("file/stdin output mismatch: file=%q stdin=%q", fromFile.stdout, fromStdin.stdout)
	}
}

func loadVectors(t *testing.T, fileName string) []cliVector {
	t.Helper()

	path := filepath.Join("testdata", "vectors", fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors %q: %v", path, err)
	}

	var vectors []cliVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("decode vectors %q: %v", path, err)
	}
	if len(vectors) == 0 {
		t.Fatalf("empty vectors in %q", path)
	}
	return vectors
}

func vectorInput(t *testing.T, v cliVector) []byte {
	t.Helper()
	if v.StdinHex != "" {
		b, err := hex.DecodeString(v.StdinHex)
		if err != nil {
			t.Fatalf("decode stdin_hex for %q: %v", v.Name, err)
		}
		return b
	}
	return []byte(v.Stdin)
}

func assertCLIResult(t *testing.T, v cliVector, res cliResult) {
	t.Helper()

	if res.exitCode != v.ExitCode {
		t.Fatalf("%s: exit code got %d want %d stderr=%q", v.Name, res.exitCode, v.ExitCode, res.stderr)
	}
	if v.Stdout != "" && res.stdout != v.Stdout {
		t.Fatalf("%s: stdout got %q want %q", v.Name, res.stdout, v.Stdout)
	}
	for _, needle := range v.StderrContains {
		if !strings.Contains(res.stderr, needle) {
			t.Fatalf("%s: stderr missing %q: %q", v.Name, needle, res.stderr)
		}
	}
}

func runCLI(t *testing.T, args []string, stdin []byte) cliResult {
	t.Helper()

	bin := cliBinaryPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run cli %v: %v", args, err)
		}
	}

	return cliResult{exitCode: exitCode, stdout: outBuf.String(), stderr: errBuf.String()}
}

func cliBinaryPath(t *testing.T) string {
	t.Helper()
	buildBinaryOnce.Do(func() {
		cachedBinary, errCachedBuild = buildCLI()
	})
	if errCachedBuild != nil {
		t.Fatalf("build cli binary: %v", errCachedBuild)
	}
	return cachedBinary
}

func buildCLI() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	binDir, err := os.MkdirTemp("", "jcs-canon-blackbox-*")
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(binDir, "jcs-canon")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/jcs-canon")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(outBuf.String()))
	}

	return binPath, nil
}
