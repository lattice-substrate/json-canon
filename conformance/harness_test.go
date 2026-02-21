package conformance_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"jcs-canon/jcs"
	"jcs-canon/jcsfloat"
	"jcs-canon/jcstoken"
)

type harness struct {
	root string
	bin  string
}

type cliResult struct {
	exitCode int
	stdout   string
	stderr   string
}

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

func TestConformanceRequirements(t *testing.T) {
	h := testHarness(t)
	requirements := loadRequirementIDs(t, filepath.Join(h.root, "spec", "requirements.md"))
	checks := requirementChecks()
	validateRequirementCoverage(t, requirements, checks)

	for _, id := range requirements {
		id := id
		t.Run(id, func(t *testing.T) {
			checks[id](t, h)
		})
	}
}

func requirementChecks() map[string]func(*testing.T, *harness) {
	return map[string]func(*testing.T, *harness){
		"REQ-ABI-001":     checkABICommandFunctional,
		"REQ-ABI-002":     checkNoCommandExitCode,
		"REQ-ABI-003":     checkUnknownCommandExitCode,
		"REQ-ABI-004":     checkInternalWriteFailureExitCode,
		"REQ-CLI-001":     checkUnknownOptionRejected,
		"REQ-CLI-002":     checkMultipleInputRejected,
		"REQ-CLI-003":     checkFileAndStdinParity,
		"REQ-CLI-004":     checkVerifyOkEmission,
		"REQ-CLI-005":     checkVerifyQuietSuppressesOk,
		"REQ-CLI-006":     checkCanonicalizeStdoutOnly,
		"REQ-RFC8259-001": checkLeadingZeroRejected,
		"REQ-RFC8259-002": checkTrailingCommaObjectRejected,
		"REQ-RFC8259-003": checkTrailingCommaArrayRejected,
		"REQ-RFC8259-004": checkUnescapedControlRejected,
		"REQ-RFC8259-005": checkTopLevelScalarAccepted,
		"REQ-RFC8259-006": checkInsignificantWhitespaceAccepted,
		"REQ-RFC8259-007": checkInvalidLiteralRejected,
		"REQ-RFC3629-001": checkInvalidUTF8Rejected,
		"REQ-RFC3629-002": checkOverlongUTF8Rejected,
		"REQ-RFC7493-001": checkDuplicateKeyRejected,
		"REQ-RFC7493-002": checkDuplicateKeyAfterUnescapeRejected,
		"REQ-RFC7493-003": checkLoneHighSurrogateRejected,
		"REQ-RFC7493-004": checkLoneLowSurrogateRejected,
		"REQ-RFC7493-005": checkNoncharacterRejected,
		"REQ-NUM-001":     checkNumberOverflowRejected,
		"REQ-NUM-002":     checkNegativeZeroRejected,
		"REQ-NUM-003":     checkUnderflowNonZeroRejected,
		"REQ-BOUND-001":   checkDepthLimitEnforced,
		"REQ-BOUND-002":   checkObjectMemberLimitEnforced,
		"REQ-BOUND-003":   checkArrayElementLimitEnforced,
		"REQ-BOUND-004":   checkStringByteLimitEnforced,
		"REQ-BOUND-005":   checkNumberTokenLengthLimitEnforced,
		"REQ-BOUND-006":   checkValueCountLimitEnforced,
		"REQ-RFC8785-001": checkWhitespaceRemovedInCanonicalOutput,
		"REQ-RFC8785-002": checkUTF16KeyOrdering,
		"REQ-RFC8785-003": checkControlEscapingExactness,
		"REQ-RFC8785-004": checkSolidusNotEscaped,
		"REQ-RFC8785-005": checkHexLowercaseEscapes,
		"REQ-RFC8785-006": checkRecursiveObjectSort,
		"REQ-RFC8785-007": checkTopLevelScalarCanonicalization,
		"REQ-RFC8785-008": checkVerifyRejectsNonCanonicalOrder,
		"REQ-RFC8785-009": checkVerifyRejectsNonCanonicalWhitespace,
		"REQ-ECMA-001":    checkBaseGoldenOracle,
		"REQ-ECMA-002":    checkStressGoldenOracle,
		"REQ-ECMA-003":    checkECMABoundaryConstants,
		"REQ-DET-001":     checkDeterministicReplay,
		"REQ-DET-002":     checkParseSerializeIdempotence,
		"REQ-BUILD-001":   checkDeterministicStaticBuildCommand,
	}
}

func validateRequirementCoverage(t *testing.T, reqs []string, checks map[string]func(*testing.T, *harness)) {
	t.Helper()
	if len(reqs) == 0 {
		t.Fatal("no requirements found in spec/requirements.md")
	}

	seen := make(map[string]struct{}, len(reqs))
	for _, id := range reqs {
		seen[id] = struct{}{}
		if checks[id] == nil {
			t.Fatalf("requirement %s has no conformance check", id)
		}
	}
	for id := range checks {
		if _, ok := seen[id]; !ok {
			t.Fatalf("check %s exists but is not listed in spec/requirements.md", id)
		}
	}
}

func loadRequirementIDs(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read requirements file: %v", err)
	}

	re := regexp.MustCompile(`(?m)^\|\s*(REQ-[A-Z0-9-]+)\s*\|`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m[1])
	}
	return ids
}

func testHarness(t *testing.T) *harness {
	t.Helper()
	root := repoRoot(t)
	buildOnce.Do(func() {
		binPath, buildErr = buildConformanceBinary(root)
	})
	if buildErr != nil {
		t.Fatalf("build conformance binary: %v", buildErr)
	}
	return &harness{root: root, bin: binPath}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), ".."))
}

func buildConformanceBinary(root string) (string, error) {
	binDir, err := os.MkdirTemp("", "jcs-canon-conformance-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(binDir, "jcs-canon")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", bin, "./cmd/jcs-canon",
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(out.String()))
	}
	return bin, nil
}

func runCLI(t *testing.T, h *harness, args []string, stdin []byte) cliResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run cli %v: %v", args, err)
		}
	}
	return cliResult{exitCode: code, stdout: outBuf.String(), stderr: errBuf.String()}
}

func runCLIToWriter(t *testing.T, h *harness, args []string, stdin []byte, stdout io.Writer) cliResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = stdout

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run cli %v: %v", args, err)
		}
	}
	return cliResult{exitCode: code, stderr: errBuf.String()}
}

func checkABICommandFunctional(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"z":3,"a":1}`))
	if res.exitCode != 0 || res.stdout != `{"a":1,"z":3}` {
		t.Fatalf("canonicalize failed: code=%d stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}

	res = runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"a":1,"z":3}`))
	if res.exitCode != 0 {
		t.Fatalf("verify failed: code=%d stderr=%q", res.exitCode, res.stderr)
	}
}

func checkNoCommandExitCode(t *testing.T, h *harness) {
	res := runCLI(t, h, nil, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "usage:") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkUnknownCommandExitCode(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"bogus"}, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown command") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkInternalWriteFailureExitCode(t *testing.T, h *harness) {
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open /dev/full: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	res := runCLIToWriter(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1}`), f)
	if res.exitCode != 10 {
		t.Fatalf("expected exit 10, got %d stderr=%q", res.exitCode, res.stderr)
	}
}

func checkUnknownOptionRejected(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--nope"}, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown option") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkMultipleInputRejected(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "a.json", "b.json"}, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "multiple input files") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkFileAndStdinParity(t *testing.T, h *harness) {
	dir := t.TempDir()
	p := filepath.Join(dir, "in.json")
	if err := os.WriteFile(p, []byte(`{"b":2,"a":1}`), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	fromFile := runCLI(t, h, []string{"canonicalize", p}, nil)
	fromStdin := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"b":2,"a":1}`))
	if fromFile.exitCode != 0 || fromStdin.exitCode != 0 || fromFile.stdout != fromStdin.stdout {
		t.Fatalf("file/stdin mismatch file=%+v stdin=%+v", fromFile, fromStdin)
	}
}

func checkVerifyOkEmission(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || res.stderr != "ok\n" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkVerifyQuietSuppressesOk(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || strings.Contains(res.stderr, "ok") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkCanonicalizeStdoutOnly(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || res.stdout != `{"a":1}` || res.stderr != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkLeadingZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`01`)), "leading zero")
}

func checkTrailingCommaObjectRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1,}`)), "expected")
}

func checkTrailingCommaArrayRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`[1,]`)), "invalid")
}

func checkUnescapedControlRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0x01, '"'}), "control")
}

func checkTopLevelScalarAccepted(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`42`))
	if res.exitCode != 0 || res.stdout != "42" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkInsignificantWhitespaceAccepted(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(" \n\t { \"a\" : 1 } \r "))
	if res.exitCode != 0 || res.stdout != `{"a":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkInvalidLiteralRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`tru`)), "invalid")
}

func checkInvalidUTF8Rejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0xff, '"'}), "valid UTF-8")
}

func checkOverlongUTF8Rejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0xc0, 0xaf, '"'}), "valid UTF-8")
}

func checkDuplicateKeyRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1,"a":2}`)), "duplicate object key")
}

func checkDuplicateKeyAfterUnescapeRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"\u0061":1,"a":2}`)), "duplicate object key")
}

func checkLoneHighSurrogateRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uD800"`)), "lone high surrogate")
}

func checkLoneLowSurrogateRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uDC00"`)), "lone low surrogate")
}

func checkNoncharacterRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uFDD0"`)), "noncharacter")
}

func checkNumberOverflowRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e999999`)), "overflows IEEE 754 double")
}

func checkNegativeZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`-0`)), "negative zero token")
}

func checkUnderflowNonZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e-400`)), "underflows to IEEE 754 zero")
}

func checkDepthLimitEnforced(t *testing.T, h *harness) {
	input := strings.Repeat("[", 1001) + strings.Repeat("]", 1001)
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(input)), "nesting depth")
}

func checkObjectMemberLimitEnforced(t *testing.T, h *harness) {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < 250001; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "\"k%06d\":1", i)
	}
	b.WriteByte('}')
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(b.String())), "member count exceeds maximum")
}

func checkArrayElementLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxArrayElements: 1})
	if err == nil || !strings.Contains(err.Error(), "array element count exceeds maximum") {
		t.Fatalf("expected array limit error, got %v", err)
	}
}

func checkStringByteLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxStringBytes: 1})
	if err == nil || !strings.Contains(err.Error(), "string decoded length exceeds maximum") {
		t.Fatalf("expected string limit error, got %v", err)
	}
}

func checkNumberTokenLengthLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`12345`), &jcstoken.Options{MaxNumberChars: 4})
	if err == nil || !strings.Contains(err.Error(), "number token length") {
		t.Fatalf("expected number length error, got %v", err)
	}
}

func checkValueCountLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 2})
	if err == nil || !strings.Contains(err.Error(), "value count") {
		t.Fatalf("expected value count error, got %v", err)
	}
}

func checkWhitespaceRemovedInCanonicalOutput(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(` { "a" : 1 } `))
	if res.exitCode != 0 || res.stdout != `{"a":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkUTF16KeyOrdering(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"\uE000":1,"\uD800\uDC00":2}`))
	if res.exitCode != 0 || res.stdout != `{"𐀀":2,"":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkControlEscapingExactness(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u0008\u0009\u000a\u000c\u000d\u001f"`))
	if res.exitCode != 0 || res.stdout != `"\b\t\n\f\r\u001f"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkSolidusNotEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"a\/b"`))
	if res.exitCode != 0 || res.stdout != `"a/b"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkHexLowercaseEscapes(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u001F"`))
	if res.exitCode != 0 || res.stdout != `"\u001f"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkRecursiveObjectSort(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"b":[{"z":1,"a":2}],"a":3}`))
	if res.exitCode != 0 || res.stdout != `{"a":3,"b":[{"a":2,"z":1}]}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkTopLevelScalarCanonicalization(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"hello"`))
	if res.exitCode != 0 || res.stdout != `"hello"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkVerifyRejectsNonCanonicalOrder(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"b":1,"a":2}`)), "not canonical")
}

func checkVerifyRejectsNonCanonicalWhitespace(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte("{\"a\":1}\n")), "not canonical")
}

func checkBaseGoldenOracle(t *testing.T, h *harness) {
	verifyFloatOracle(t, filepath.Join(h.root, "jcsfloat", "testdata", "golden_vectors.csv"), 54445,
		"593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636")
}

func checkStressGoldenOracle(t *testing.T, h *harness) {
	verifyFloatOracle(t, filepath.Join(h.root, "jcsfloat", "testdata", "golden_stress_vectors.csv"), 231917,
		"287d21ac87e5665550f1baf86038302a0afc67a74a020dffb872f1a93b26d410")
}

func checkECMABoundaryConstants(t *testing.T, _ *harness) {
	cases := map[uint64]string{
		0x0000000000000000: "0",
		0x8000000000000000: "0",
		0x0000000000000001: "5e-324",
		0x7fefffffffffffff: "1.7976931348623157e+308",
		0x3eb0c6f7a0b5ed8d: "0.000001",
		0x3eb0c6f7a0b5ed8c: "9.999999999999997e-7",
		0x3eb0c6f7a0b5ed8e: "0.0000010000000000000002",
		0x444b1ae4d6e2ef50: "1e+21",
		0x444b1ae4d6e2ef4f: "999999999999999900000",
		0x444b1ae4d6e2ef51: "1.0000000000000001e+21",
	}
	for bits, want := range cases {
		got, err := jcsfloat.FormatDouble(math.Float64frombits(bits))
		if err != nil {
			t.Fatalf("format bits=%016x: %v", bits, err)
		}
		if got != want {
			t.Fatalf("bits=%016x got=%q want=%q", bits, got, want)
		}
	}
}

func checkDeterministicReplay(t *testing.T, h *harness) {
	input := []byte(`{"z":3,"a":1,"arr":[3,2,1],"n":1e21}`)
	first := runCLI(t, h, []string{"canonicalize", "-"}, input)
	if first.exitCode != 0 {
		t.Fatalf("first run failed: %+v", first)
	}

	for i := 0; i < 200; i++ {
		res := runCLI(t, h, []string{"canonicalize", "-"}, input)
		if res.exitCode != 0 || res.stdout != first.stdout {
			t.Fatalf("iteration %d mismatch: first=%+v got=%+v", i, first, res)
		}
	}
}

func checkParseSerializeIdempotence(t *testing.T, _ *harness) {
	input := []byte(`{"z":3,"a":[{"x":1,"y":2}],"n":1e21}`)
	v1, err := jcstoken.Parse(input)
	if err != nil {
		t.Fatalf("parse1: %v", err)
	}
	o1, err := jcs.Serialize(v1)
	if err != nil {
		t.Fatalf("serialize1: %v", err)
	}
	v2, err := jcstoken.Parse(o1)
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	o2, err := jcs.Serialize(v2)
	if err != nil {
		t.Fatalf("serialize2: %v", err)
	}
	if !bytes.Equal(o1, o2) {
		t.Fatalf("idempotence mismatch: %q vs %q", o1, o2)
	}
}

func checkDeterministicStaticBuildCommand(t *testing.T, h *harness) {
	out := filepath.Join(t.TempDir(), "jcs-canon")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(
		ctx,
		"go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", out, "./cmd/jcs-canon",
	)
	cmd.Dir = h.root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("build command failed: %v output=%s", err, buf.String())
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("expected built binary, stat err=%v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty built binary")
	}
}

func verifyFloatOracle(t *testing.T, path string, expectedRows int, expectedSHA256 string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open oracle: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	h := sha256.New()
	tee := io.TeeReader(f, h)
	sc := bufio.NewScanner(tee)
	sc.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)

	rows := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rows++
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed oracle line %d: %q", rows, line)
		}
		bits, err := strconv.ParseUint(parts[0], 16, 64)
		if err != nil {
			t.Fatalf("line %d parse bits: %v", rows, err)
		}
		got, err := jcsfloat.FormatDouble(math.Float64frombits(bits))
		if err != nil {
			t.Fatalf("line %d unexpected format error: %v", rows, err)
		}
		if got != parts[1] {
			t.Fatalf("line %d bits=%016x got=%q want=%q", rows, bits, got, parts[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan oracle: %v", err)
	}
	if rows != expectedRows {
		t.Fatalf("oracle row count mismatch: got %d want %d", rows, expectedRows)
	}
	gotSHA := fmt.Sprintf("%x", h.Sum(nil))
	if gotSHA != expectedSHA256 {
		t.Fatalf("oracle checksum mismatch: got %s want %s", gotSHA, expectedSHA256)
	}
}

func assertInvalid(t *testing.T, res cliResult, needle string) {
	t.Helper()
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stderr, needle) {
		t.Fatalf("stderr missing %q: %q", needle, res.stderr)
	}
}
