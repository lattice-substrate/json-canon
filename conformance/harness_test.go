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

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcsfloat"
	"github.com/lattice-substrate/json-canon/jcstoken"
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

// TestConformanceRequirements runs all requirement checks.
func TestConformanceRequirements(t *testing.T) {
	h := testHarness(t)
	requirements := loadRequirementIDs(t, filepath.Join(h.root, "REQ_REGISTRY.md"))
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
		// PARSE-UTF8
		"PARSE-UTF8-001": checkInvalidUTF8Rejected,
		"PARSE-UTF8-002": checkOverlongUTF8Rejected,
		// PARSE-GRAM
		"PARSE-GRAM-001": checkLeadingZeroRejected,
		"PARSE-GRAM-002": checkTrailingCommaObjectRejected,
		"PARSE-GRAM-003": checkTrailingCommaArrayRejected,
		"PARSE-GRAM-004": checkUnescapedControlRejected,
		"PARSE-GRAM-005": checkTopLevelScalarAccepted,
		"PARSE-GRAM-006": checkInsignificantWhitespaceAccepted,
		"PARSE-GRAM-007": checkInvalidLiteralRejected,
		"PARSE-GRAM-008": checkTrailingContentRejected,
		"PARSE-GRAM-009": checkNumberGrammarEnforced,
		"PARSE-GRAM-010": checkInvalidEscapeRejected,
		// IJSON-DUP
		"IJSON-DUP-001": checkDuplicateKeyRejected,
		"IJSON-DUP-002": checkDuplicateKeyAfterUnescapeRejected,
		// IJSON-SUR
		"IJSON-SUR-001": checkLoneHighSurrogateRejected,
		"IJSON-SUR-002": checkLoneLowSurrogateRejected,
		"IJSON-SUR-003": checkValidSurrogatePairDecoded,
		// IJSON-NONC
		"IJSON-NONC-001": checkNoncharacterRejected,
		// CANON-WS
		"CANON-WS-001": checkWhitespaceRemovedInCanonicalOutput,
		// CANON-STR
		"CANON-STR-001": checkBackspaceEscaped,
		"CANON-STR-002": checkTabEscaped,
		"CANON-STR-003": checkLineFeedEscaped,
		"CANON-STR-004": checkFormFeedEscaped,
		"CANON-STR-005": checkCarriageReturnEscaped,
		"CANON-STR-006": checkOtherControlsEscapedLowerHex,
		"CANON-STR-007": checkQuotationMarkEscaped,
		"CANON-STR-008": checkBackslashEscaped,
		"CANON-STR-009": checkSolidusNotEscaped,
		"CANON-STR-010": checkAboveU001FRawUTF8,
		"CANON-STR-011": checkNoNormalization,
		// CANON-SORT
		"CANON-SORT-001": checkUTF16KeyOrdering,
		"CANON-SORT-002": checkRecursiveObjectSort,
		"CANON-SORT-003": checkArrayOrderPreserved,
		// CANON-LIT
		"CANON-LIT-001": checkLowercaseLiterals,
		// CANON-ENC
		"CANON-ENC-001": checkOutputIsUTF8,
		// ECMA-FMT
		"ECMA-FMT-001": checkECMANaNRejected,
		"ECMA-FMT-002": checkECMANegZeroSerializes,
		"ECMA-FMT-003": checkECMAInfinityRejected,
		"ECMA-FMT-004": checkECMAIntegerFixed,
		"ECMA-FMT-005": checkECMAFractionFixed,
		"ECMA-FMT-006": checkECMASmallFraction,
		"ECMA-FMT-007": checkECMAExponential,
		"ECMA-FMT-008": checkECMAShortestRoundTrip,
		"ECMA-FMT-009": checkECMAEvenDigitTieBreak,
		// ECMA-VEC
		"ECMA-VEC-001": checkBaseGoldenOracle,
		"ECMA-VEC-002": checkStressGoldenOracle,
		"ECMA-VEC-003": checkECMABoundaryConstants,
		// PROF-NUM
		"PROF-NEGZ-001":  checkNegativeZeroRejected,
		"PROF-OFLOW-001": checkNumberOverflowRejected,
		"PROF-UFLOW-001": checkUnderflowNonZeroRejected,
		// BOUND
		"BOUND-DEPTH-001":   checkDepthLimitEnforced,
		"BOUND-INPUT-001":   checkInputSizeLimitEnforced,
		"BOUND-VALUES-001":  checkValueCountLimitEnforced,
		"BOUND-MEMBERS-001": checkObjectMemberLimitEnforced,
		"BOUND-ELEMS-001":   checkArrayElementLimitEnforced,
		"BOUND-STRBYTES-001": checkStringByteLimitEnforced,
		"BOUND-NUMCHARS-001": checkNumberTokenLengthLimitEnforced,
		// CLI
		"CLI-CMD-001":  checkCanonicalizeFunctional,
		"CLI-CMD-002":  checkVerifyFunctional,
		"CLI-EXIT-001": checkNoCommandExitCode,
		"CLI-EXIT-002": checkUnknownCommandExitCode,
		"CLI-EXIT-003": checkInputViolationExitCode,
		"CLI-EXIT-004": checkInternalWriteFailureExitCode,
		"CLI-FLAG-001": checkUnknownOptionRejected,
		"CLI-FLAG-002": checkVerifyQuietSuppressesOk,
		"CLI-FLAG-003": checkHelpExitsZero,
		"CLI-IO-001":   checkStdinReading,
		"CLI-IO-002":   checkMultipleInputRejected,
		"CLI-IO-003":   checkFileAndStdinParity,
		"CLI-IO-004":   checkCanonicalizeStdoutOnly,
		"CLI-IO-005":   checkVerifyOkEmission,
		// VERIFY
		"VERIFY-ORDER-001": checkVerifyRejectsNonCanonicalOrder,
		"VERIFY-WS-001":    checkVerifyRejectsNonCanonicalWhitespace,
		// DET
		"DET-REPLAY-001":    checkDeterministicReplay,
		"DET-IDEMPOTENT-001": checkParseSerializeIdempotence,
		"DET-STATIC-001":    checkDeterministicStaticBuildCommand,
		"DET-NOSOURCE-001":  checkNoNondeterminismSources,
	}
}

func validateRequirementCoverage(t *testing.T, reqs []string, checks map[string]func(*testing.T, *harness)) {
	t.Helper()
	if len(reqs) == 0 {
		t.Fatal("no requirements found in REQ_REGISTRY.md")
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
			t.Fatalf("check %s exists but is not listed in REQ_REGISTRY.md", id)
		}
	}
}

func loadRequirementIDs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read requirements file: %v", err)
	}

	re := regexp.MustCompile(`(?m)^\|\s*([A-Z]+-[A-Z0-9]+-[0-9]+)\s*\|`)
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

func assertInvalid(t *testing.T, res cliResult, needle string) {
	t.Helper()
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stderr, needle) {
		t.Fatalf("stderr missing %q: %q", needle, res.stderr)
	}
}

// ==================== PARSE-UTF8 ====================

func checkInvalidUTF8Rejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0xff, '"'}), "valid UTF-8")
}

func checkOverlongUTF8Rejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0xc0, 0xaf, '"'}), "valid UTF-8")
}

// ==================== PARSE-GRAM ====================

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

func checkTrailingContentRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`42 "extra"`)), "trailing content")
}

func checkNumberGrammarEnforced(t *testing.T, h *harness) {
	// Valid
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`0.5`))
	if res.exitCode != 0 {
		t.Fatalf("valid number rejected: %+v", res)
	}
	// Invalid: no digits after decimal
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1.`)), "expected digit")
}

func checkInvalidEscapeRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\x"`)), "invalid escape")
}

// ==================== IJSON ====================

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

func checkValidSurrogatePairDecoded(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uD83D\uDE00"`))
	if res.exitCode != 0 || res.stdout != `"😀"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkNoncharacterRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uFDD0"`)), "noncharacter")
}

// ==================== CANON-WS ====================

func checkWhitespaceRemovedInCanonicalOutput(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(` { "a" : 1 } `))
	if res.exitCode != 0 || res.stdout != `{"a":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// ==================== CANON-STR ====================

func checkBackspaceEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u0008"`))
	if res.exitCode != 0 || res.stdout != `"\b"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkTabEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u0009"`))
	if res.exitCode != 0 || res.stdout != `"\t"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkLineFeedEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u000a"`))
	if res.exitCode != 0 || res.stdout != `"\n"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkFormFeedEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u000c"`))
	if res.exitCode != 0 || res.stdout != `"\f"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkCarriageReturnEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u000d"`))
	if res.exitCode != 0 || res.stdout != `"\r"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkOtherControlsEscapedLowerHex(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\u001F"`))
	if res.exitCode != 0 || res.stdout != `"\u001f"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkQuotationMarkEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"a\"b"`))
	if res.exitCode != 0 || res.stdout != `"a\"b"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkBackslashEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"a\\b"`))
	if res.exitCode != 0 || res.stdout != `"a\\b"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkSolidusNotEscaped(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"a\/b"`))
	if res.exitCode != 0 || res.stdout != `"a/b"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkAboveU001FRawUTF8(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"<>&"`))
	if res.exitCode != 0 || res.stdout != `"<>&"` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkNoNormalization(t *testing.T, _ *harness) {
	// NFC and NFD forms should produce different output
	nfc := "\u00E9"            // é as single codepoint
	nfd := "\u0065\u0301"     // e + combining acute
	v1 := &jcstoken.Value{Kind: jcstoken.KindString, Str: nfc}
	v2 := &jcstoken.Value{Kind: jcstoken.KindString, Str: nfd}
	o1, err := jcs.Serialize(v1)
	if err != nil {
		t.Fatalf("serialize NFC: %v", err)
	}
	o2, err := jcs.Serialize(v2)
	if err != nil {
		t.Fatalf("serialize NFD: %v", err)
	}
	if bytes.Equal(o1, o2) {
		t.Fatal("normalization was applied — NFC and NFD should produce different output")
	}
}

// ==================== CANON-SORT ====================

func checkUTF16KeyOrdering(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"\uE000":1,"\uD800\uDC00":2}`))
	// U+10000 (𐀀) sorts before U+E000 () in UTF-16 code-unit order
	// because U+10000 encodes as surrogate pair D800 DC00 (0xD800 < 0xE000)
	want := "{\"𐀀\":2,\"\uE000\":1}"
	if res.exitCode != 0 || res.stdout != want {
		t.Fatalf("unexpected result: exitCode=%d stdout=%q want=%q stderr=%q",
			res.exitCode, res.stdout, want, res.stderr)
	}
}

func checkRecursiveObjectSort(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"b":[{"z":1,"a":2}],"a":3}`))
	if res.exitCode != 0 || res.stdout != `{"a":3,"b":[{"a":2,"z":1}]}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkArrayOrderPreserved(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`[3,1,2]`))
	if res.exitCode != 0 || res.stdout != `[3,1,2]` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// ==================== CANON-LIT ====================

func checkLowercaseLiterals(t *testing.T, h *harness) {
	for _, lit := range []string{"true", "false", "null"} {
		res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(lit))
		if res.exitCode != 0 || res.stdout != lit {
			t.Fatalf("unexpected result for %s: %+v", lit, res)
		}
	}
}

// ==================== CANON-ENC ====================

func checkOutputIsUTF8(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"key":"value"}`))
	if res.exitCode != 0 {
		t.Fatalf("unexpected exit: %+v", res)
	}
	for i := 0; i < len(res.stdout); {
		r, size := decodeRuneInString(res.stdout, i)
		if r == 0xFFFD && size == 1 {
			t.Fatalf("invalid UTF-8 at byte %d", i)
		}
		i += size
	}
}

func decodeRuneInString(s string, i int) (rune, int) {
	if s[i] < 0x80 {
		return rune(s[i]), 1
	}
	b := []byte(s[i:])
	r, size := decodeRune(b)
	return r, size
}

func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0xFFFD, 1
	}
	if b[0] < 0x80 {
		return rune(b[0]), 1
	}
	// multi-byte
	var size int
	switch {
	case b[0] < 0xE0:
		size = 2
	case b[0] < 0xF0:
		size = 3
	default:
		size = 4
	}
	if len(b) < size {
		return 0xFFFD, 1
	}
	var r rune
	switch size {
	case 2:
		r = rune(b[0]&0x1F)<<6 | rune(b[1]&0x3F)
	case 3:
		r = rune(b[0]&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F)
	case 4:
		r = rune(b[0]&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F)
	}
	return r, size
}

// ==================== ECMA-FMT ====================

func checkECMANaNRejected(t *testing.T, _ *harness) {
	_, err := jcsfloat.FormatDouble(math.NaN())
	if err == nil {
		t.Fatal("expected error for NaN")
	}
}

func checkECMANegZeroSerializes(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(math.Copysign(0, -1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0" {
		t.Fatalf("got %q want %q", got, "0")
	}
}

func checkECMAInfinityRejected(t *testing.T, _ *harness) {
	_, err := jcsfloat.FormatDouble(math.Inf(1))
	if err == nil {
		t.Fatal("expected error for Infinity")
	}
}

func checkECMAIntegerFixed(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(1e20)
	if err != nil {
		t.Fatal(err)
	}
	if got != "100000000000000000000" {
		t.Fatalf("got %q", got)
	}
}

func checkECMAFractionFixed(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(0.5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.5" {
		t.Fatalf("got %q", got)
	}
}

func checkECMASmallFraction(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(0.000001)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.000001" {
		t.Fatalf("got %q", got)
	}
}

func checkECMAExponential(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(1e21)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1e+21" {
		t.Fatalf("got %q", got)
	}
}

func checkECMAShortestRoundTrip(t *testing.T, _ *harness) {
	for _, v := range []float64{0.1, 0.2, 1e-7, 5e-324, math.MaxFloat64} {
		s, err := jcsfloat.FormatDouble(v)
		if err != nil {
			t.Fatal(err)
		}
		parsed, parseErr := strconv.ParseFloat(s, 64)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if parsed != v {
			t.Fatalf("round-trip failed for %v: %q → %v", v, s, parsed)
		}
	}
}

func checkECMAEvenDigitTieBreak(t *testing.T, _ *harness) {
	// Verify idempotency (consequence of correct tie-breaking)
	for i := uint64(1); i < 100; i += 7 {
		v := math.Float64frombits(i * 0x9e3779b97f4a7c15)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		s, err := jcsfloat.FormatDouble(v)
		if err != nil {
			t.Fatal(err)
		}
		parsed, parseErr := strconv.ParseFloat(s, 64)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		s2, err := jcsfloat.FormatDouble(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if s != s2 {
			t.Fatalf("idempotency: %q != %q", s, s2)
		}
	}
}

// ==================== ECMA-VEC ====================

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

// ==================== PROF-NUM ====================

func checkNegativeZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`-0`)), "negative zero token")
}

func checkNumberOverflowRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e999999`)), "overflows IEEE 754 double")
}

func checkUnderflowNonZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e-400`)), "underflows to IEEE 754 zero")
}

// ==================== BOUND ====================

func checkDepthLimitEnforced(t *testing.T, h *harness) {
	input := strings.Repeat("[", 1001) + strings.Repeat("]", 1001)
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(input)), "nesting depth")
}

func checkInputSizeLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxInputSize: 2})
	if err == nil {
		t.Fatal("expected input size error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

func checkValueCountLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 2})
	if err == nil || !strings.Contains(err.Error(), "value count") {
		t.Fatalf("expected value count error, got %v", err)
	}
}

func checkObjectMemberLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`{"a":1,"b":2}`),
		&jcstoken.Options{MaxObjectMembers: 1},
	)
	if err == nil || !strings.Contains(err.Error(), "object member count exceeds maximum") {
		t.Fatalf("expected object member limit error, got %v", err)
	}
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

// ==================== CLI ====================

func checkCanonicalizeFunctional(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"z":3,"a":1}`))
	if res.exitCode != 0 || res.stdout != `{"a":1,"z":3}` {
		t.Fatalf("canonicalize failed: %+v", res)
	}
}

func checkVerifyFunctional(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"a":1,"z":3}`))
	if res.exitCode != 0 {
		t.Fatalf("verify failed: %+v", res)
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

func checkInputViolationExitCode(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`01`))
	if res.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d: %+v", res.exitCode, res)
	}
}

func checkInternalWriteFailureExitCode(t *testing.T, h *harness) {
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("skip: cannot open /dev/full: %v", err)
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

func checkVerifyQuietSuppressesOk(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || strings.Contains(res.stderr, "ok") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkHelpExitsZero(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "--help"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", res.exitCode)
	}
	res = runCLI(t, h, []string{"verify", "--help"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", res.exitCode)
	}
}

func checkStdinReading(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`42`))
	if res.exitCode != 0 || res.stdout != "42" {
		t.Fatalf("unexpected result: %+v", res)
	}
	res = runCLI(t, h, []string{"canonicalize"}, []byte(`42`))
	if res.exitCode != 0 || res.stdout != "42" {
		t.Fatalf("unexpected result without -: %+v", res)
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

func checkCanonicalizeStdoutOnly(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || res.stdout != `{"a":1}` || res.stderr != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkVerifyOkEmission(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 || res.stderr != "ok\n" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// ==================== VERIFY ====================

func checkVerifyRejectsNonCanonicalOrder(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"b":1,"a":2}`)), "not canonical")
}

func checkVerifyRejectsNonCanonicalWhitespace(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte("{\"a\":1}\n")), "not canonical")
}

// ==================== DET ====================

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

func checkNoNondeterminismSources(t *testing.T, h *harness) {
	// Verify that the source code does not import nondeterministic packages
	// (this is a static check of the source files)
	badImports := []string{
		"math/rand",
		"crypto/rand",
		"time",
	}
	srcDirs := []string{"jcsfloat", "jcstoken", "jcs"}
	for _, dir := range srcDirs {
		entries, err := os.ReadDir(filepath.Join(h.root, dir))
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(h.root, dir, entry.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			content := string(data)
			for _, bad := range badImports {
				if strings.Contains(content, fmt.Sprintf("%q", bad)) {
					t.Fatalf("file %s/%s imports nondeterministic package %q", dir, entry.Name(), bad)
				}
			}
		}
	}
}

// --- Helpers ---

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
		got, fmtErr := jcsfloat.FormatDouble(math.Float64frombits(bits))
		if fmtErr != nil {
			t.Fatalf("line %d unexpected format error: %v", rows, fmtErr)
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
