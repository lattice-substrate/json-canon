package conformance_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcsfloat"
	"github.com/lattice-substrate/json-canon/jcstoken"
	"github.com/lattice-substrate/json-canon/offline/replay"
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

type vectorCase struct {
	ID                 string   `json:"id"`
	Mode               string   `json:"mode,omitempty"`
	Args               []string `json:"args,omitempty"`
	Input              string   `json:"input"`
	WantStdout         *string  `json:"want_stdout,omitempty"`
	WantStderr         *string  `json:"want_stderr,omitempty"`
	WantStderrContains *string  `json:"want_stderr_contains,omitempty"`
	WantExit           int      `json:"want_exit"`
}

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// TestConformanceRequirements runs all requirement checks.
func TestConformanceRequirements(t *testing.T) {
	h := testHarness(t)
	requirements := loadRequirementIDs(
		t,
		filepath.Join(h.root, "REQ_REGISTRY_NORMATIVE.md"),
		filepath.Join(h.root, "REQ_REGISTRY_POLICY.md"),
	)
	checks := requirementChecks()
	validateRequirementCoverage(t, requirements, checks)

	for _, id := range requirements {
		id := id
		t.Run(id, func(t *testing.T) {
			checks[id](t, h)
		})
	}
}

// TestConformanceVectors executes JSONL vectors under conformance/vectors.
func TestConformanceVectors(t *testing.T) {
	h := testHarness(t)
	pattern := filepath.Join(h.root, "conformance", "vectors", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob vectors: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no vector files found for pattern %q", pattern)
	}

	executed := 0
	for _, f := range files {
		file := f
		t.Run(filepath.Base(file), func(t *testing.T) {
			fd, err := os.Open(file)
			if err != nil {
				t.Fatalf("open vector file: %v", err)
			}
			defer func() { _ = fd.Close() }()

			sc := bufio.NewScanner(fd)
			sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			lineNo := 0
			for sc.Scan() {
				lineNo++
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				var v vectorCase
				if err := json.Unmarshal([]byte(line), &v); err != nil {
					t.Fatalf("%s:%d decode vector: %v", file, lineNo, err)
				}
				if v.ID == "" {
					t.Fatalf("%s:%d vector missing id", file, lineNo)
				}

				args := v.Args
				if len(args) == 0 {
					if v.Mode == "" {
						t.Fatalf("%s:%d id=%s requires mode or args", file, lineNo, v.ID)
					}
					args = []string{v.Mode, "-"}
				}

				res := runCLI(t, h, args, []byte(v.Input))
				if res.exitCode != v.WantExit {
					t.Fatalf("%s:%d id=%s exit mismatch got=%d want=%d stdout=%q stderr=%q", file, lineNo, v.ID, res.exitCode, v.WantExit, res.stdout, res.stderr)
				}
				if v.WantStdout != nil && res.stdout != *v.WantStdout {
					t.Fatalf("%s:%d id=%s stdout mismatch got=%q want=%q", file, lineNo, v.ID, res.stdout, *v.WantStdout)
				}
				if v.WantStderr != nil && res.stderr != *v.WantStderr {
					t.Fatalf("%s:%d id=%s stderr mismatch got=%q want=%q", file, lineNo, v.ID, res.stderr, *v.WantStderr)
				}
				if v.WantStderrContains != nil && !strings.Contains(res.stderr, *v.WantStderrContains) {
					t.Fatalf("%s:%d id=%s stderr missing substring %q in %q", file, lineNo, v.ID, *v.WantStderrContains, res.stderr)
				}
				executed++
			}
			if err := sc.Err(); err != nil {
				t.Fatalf("%s scan error: %v", file, err)
			}
		})
	}
	if executed == 0 {
		t.Fatal("no vectors executed")
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
		"CANON-STR-012": checkStringEnclosedInQuotes,
		// CANON-SORT
		"CANON-SORT-001": checkUTF16KeyOrdering,
		"CANON-SORT-002": checkRecursiveObjectSort,
		"CANON-SORT-003": checkArrayOrderPreserved,
		"CANON-SORT-004": checkSortUsesRawPropertyNames,
		"CANON-SORT-005": checkSortLexicographicRule,
		// CANON-LIT
		"CANON-LIT-001": checkLowercaseLiterals,
		// CANON-ENC
		"CANON-ENC-001": checkOutputIsUTF8,
		"CANON-ENC-002": checkOutputHasNoBOM,
		// GEN-GRAM
		"GEN-GRAM-001": checkGeneratorProducesGrammarConformingJSON,
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
		"ECMA-FMT-010": checkECMANegativeSign,
		"ECMA-FMT-011": checkECMAMinimalK,
		"ECMA-FMT-012": checkECMAScientificK1,
		// ECMA-VEC
		"ECMA-VEC-001": checkBaseGoldenOracle,
		"ECMA-VEC-002": checkStressGoldenOracle,
		"ECMA-VEC-003": checkECMABoundaryConstants,
		// PROF-NUM
		"PROF-NEGZ-001":  checkNegativeZeroRejected,
		"PROF-OFLOW-001": checkNumberOverflowRejected,
		"PROF-UFLOW-001": checkUnderflowNonZeroRejected,
		// BOUND
		"BOUND-DEPTH-001":    checkDepthLimitEnforced,
		"BOUND-INPUT-001":    checkInputSizeLimitEnforced,
		"BOUND-VALUES-001":   checkValueCountLimitEnforced,
		"BOUND-MEMBERS-001":  checkObjectMemberLimitEnforced,
		"BOUND-ELEMS-001":    checkArrayElementLimitEnforced,
		"BOUND-STRBYTES-001": checkStringByteLimitEnforced,
		"BOUND-NUMCHARS-001": checkNumberTokenLengthLimitEnforced,
		// CLI
		"CLI-CMD-001":   checkCanonicalizeFunctional,
		"CLI-CMD-002":   checkVerifyFunctional,
		"CLI-EXIT-001":  checkNoCommandExitCode,
		"CLI-EXIT-002":  checkUnknownCommandExitCode,
		"CLI-EXIT-003":  checkInputViolationExitCode,
		"CLI-EXIT-004":  checkInternalWriteFailureExitCode,
		"CLI-FLAG-001":  checkUnknownOptionRejected,
		"CLI-FLAG-002":  checkVerifyQuietSuppressesOk,
		"CLI-FLAG-003":  checkHelpExitsZero,
		"CLI-FLAG-004":  checkVersionExitsZero,
		"CLI-IO-001":    checkStdinReading,
		"CLI-IO-002":    checkMultipleInputRejected,
		"CLI-IO-003":    checkFileAndStdinParity,
		"CLI-IO-004":    checkCanonicalizeStdoutOnly,
		"CLI-IO-005":    checkVerifyOkEmission,
		"CLI-CLASS-001": checkErrorDiagnosticsIncludeFailureClass,
		// ABI/Supply/Governance/Traceability policy
		"ABI-PARITY-001":       checkABIManifestBehaviorParity,
		"SUPPLY-PIN-001":       checkGitHubActionsPinnedBySHA,
		"SUPPLY-PROV-001":      checkReleaseWorkflowVerificationArtifacts,
		"GOV-DUR-001":          checkGovernanceDurabilityClausesPresent,
		"TRACE-LINK-001":       checkBehaviorTestsLinkedToRequirements,
		"OFFLINE-MATRIX-001":   checkOfflineMatrixManifestPresent,
		"OFFLINE-COLD-001":     checkOfflineProfileColdReplayPolicy,
		"OFFLINE-EVIDENCE-001": checkOfflineEvidenceSchemaAndVerifyCLI,
		"OFFLINE-GATE-001":     checkOfflineReleaseGatePolicy,
		"OFFLINE-ARCH-001":     checkOfflineArchScopeX8664,
		// VERIFY
		"VERIFY-ORDER-001": checkVerifyRejectsNonCanonicalOrder,
		"VERIFY-WS-001":    checkVerifyRejectsNonCanonicalWhitespace,
		// DET
		"DET-REPLAY-001":     checkDeterministicReplay,
		"DET-IDEMPOTENT-001": checkParseSerializeIdempotence,
		"DET-STATIC-001":     checkDeterministicStaticBuildCommand,
		"DET-NOSOURCE-001":   checkNoNondeterminismSources,
	}
}

func validateRequirementCoverage(t *testing.T, reqs []string, checks map[string]func(*testing.T, *harness)) {
	t.Helper()
	if len(reqs) == 0 {
		t.Fatal("no requirements found in split registries")
	}

	seen := make(map[string]struct{}, len(reqs))
	for _, id := range reqs {
		seen[id] = struct{}{}
		if checks[id] == nil {
			t.Fatalf("requirement %s has no conformance check", id)
		}
	}
	sortedCheckIDs := make([]string, 0, len(checks))
	for id := range checks {
		sortedCheckIDs = append(sortedCheckIDs, id)
	}
	sort.Strings(sortedCheckIDs)
	for _, id := range sortedCheckIDs {
		if _, ok := seen[id]; !ok {
			t.Fatalf("check %s exists but is not listed in split registries", id)
		}
	}
}

func loadRequirementIDs(t *testing.T, paths ...string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\|\s*([A-Z]+-[A-Z0-9]+-[0-9]+)\s*\|`)
	seen := make(map[string]struct{})
	ids := make([]string, 0, 128)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read requirements file %q: %v", path, err)
		}
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			if _, ok := seen[m[1]]; ok {
				t.Fatalf("duplicate requirement id across registries: %s", m[1])
			}
			seen[m[1]] = struct{}{}
			ids = append(ids, m[1])
		}
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
	cases := []struct {
		input []byte
		need  string
	}{
		{[]byte{'"', 0xff, '"'}, "valid UTF-8"},
		{[]byte{'"', 0xe2, 0x82, '"'}, "valid UTF-8"},
		{[]byte{'"', 0xed, 0xa0, 0x80, '"'}, "valid UTF-8"},
	}
	for _, tc := range cases {
		assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, tc.input), tc.need)
	}
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
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{'"', 0x00, '"'}), "control")
}

func checkTopLevelScalarAccepted(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`42`))
	if res.exitCode != 0 || res.stdout != "42" {
		t.Fatalf("unexpected result: %+v", res)
	}
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{}), "unexpected end of input")
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte{0xEF, 0xBB, 0xBF, '4', '2'}), "invalid number character")
}

func checkInsignificantWhitespaceAccepted(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(" \n\t { \"a\" : 1 } \r "))
	if res.exitCode != 0 || res.stdout != `{"a":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
	res = runCLI(t, h, []string{"canonicalize", "-"}, []byte("\r\n{\r\n\"a\"\r\n:\r\n1\r\n}\r\n"))
	if res.exitCode != 0 || res.stdout != `{"a":1}` {
		t.Fatalf("unexpected CRLF result: %+v", res)
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
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"\uD83F\uDFFE"`)), "noncharacter")
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
	nfc := "\u00E9"       // é as single codepoint
	nfd := "\u0065\u0301" // e + combining acute
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

func checkStringEnclosedInQuotes(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`"abc"`))
	if res.exitCode != 0 || res.stdout != `"abc"` {
		t.Fatalf("unexpected result: %+v", res)
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

	res = runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"\uE000":5,"\uD83D\uDE00":4,"\uD800\uDC00":3,"aa":2,"":1}`))
	want = "{\"\":1,\"aa\":2,\"𐀀\":3,\"😀\":4,\"\uE000\":5}"
	if res.exitCode != 0 || res.stdout != want {
		t.Fatalf("unexpected mixed-order result: exitCode=%d stdout=%q want=%q stderr=%q",
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

func checkSortUsesRawPropertyNames(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"\\n":1,"\n":2}`))
	if res.exitCode != 0 || res.stdout != `{"\n":2,"\\n":1}` {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkSortLexicographicRule(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"ab":4,"aa":3,"":1,"a":2}`))
	if res.exitCode != 0 || res.stdout != `{"":1,"a":2,"aa":3,"ab":4}` {
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

func checkOutputHasNoBOM(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`{"a":1}`))
	if res.exitCode != 0 {
		t.Fatalf("unexpected exit: %+v", res)
	}
	if strings.HasPrefix(res.stdout, "\uFEFF") {
		t.Fatalf("canonical output contains UTF-8 BOM prefix: %q", res.stdout)
	}
}

func checkGeneratorProducesGrammarConformingJSON(t *testing.T, _ *harness) {
	v, err := jcstoken.Parse([]byte(`{"z":[{"b":"\u0000","a":1e21}],"a":true}`))
	if err != nil {
		t.Fatalf("parse input: %v", err)
	}
	out, err := jcs.Serialize(v)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if _, err := jcstoken.Parse(out); err != nil {
		t.Fatalf("generated output violates JSON grammar: %v", err)
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

func checkECMANegativeSign(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(-12.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "-12.5" {
		t.Fatalf("got %q want %q", got, "-12.5")
	}
}

func checkECMAMinimalK(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(1.2300000000000002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2300000000000002" {
		t.Fatalf("got %q", got)
	}
}

func checkECMAScientificK1(t *testing.T, _ *harness) {
	got, err := jcsfloat.FormatDouble(1e21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1e+21" {
		t.Fatalf("got %q want %q", got, "1e+21")
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
	cases := []struct {
		bits uint64
		want string
	}{
		{0x0000000000000000, "0"},
		{0x8000000000000000, "0"},
		{0x0000000000000001, "5e-324"},
		{0x7fefffffffffffff, "1.7976931348623157e+308"},
		{0x3eb0c6f7a0b5ed8d, "0.000001"},
		{0x3eb0c6f7a0b5ed8c, "9.999999999999997e-7"},
		{0x3eb0c6f7a0b5ed8e, "0.0000010000000000000002"},
		{0x444b1ae4d6e2ef50, "1e+21"},
		{0x444b1ae4d6e2ef4f, "999999999999999900000"},
		{0x444b1ae4d6e2ef51, "1.0000000000000001e+21"},
	}
	for _, tc := range cases {
		got, err := jcsfloat.FormatDouble(math.Float64frombits(tc.bits))
		if err != nil {
			t.Fatalf("format bits=%016x: %v", tc.bits, err)
		}
		if got != tc.want {
			t.Fatalf("bits=%016x got=%q want=%q", tc.bits, got, tc.want)
		}
	}
}

// ==================== PROF-NUM ====================

func checkNegativeZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`-0`)), "negative zero token")
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`-0.0e1`)), "negative zero token")
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`-0e-1`)), "negative zero token")
}

func checkNumberOverflowRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e999999`)), "overflows IEEE 754 double")
}

func checkUnderflowNonZeroRejected(t *testing.T, h *harness) {
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e-400`)), "underflows to IEEE 754 zero")
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`1e-324`)), "underflows to IEEE 754 zero")
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(`2e-324`)), "underflows to IEEE 754 zero")

	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`3e-324`))
	if res.exitCode != 0 || res.stdout != "5e-324" {
		t.Fatalf("expected accepted boundary rounding to min subnormal, got %+v", res)
	}
}

// ==================== BOUND ====================

func checkDepthLimitEnforced(t *testing.T, h *harness) {
	exact := strings.Repeat("[", jcstoken.DefaultMaxDepth) + strings.Repeat("]", jcstoken.DefaultMaxDepth)
	okRes := runCLI(t, h, []string{"canonicalize", "-"}, []byte(exact))
	if okRes.exitCode != 0 || okRes.stdout != exact {
		t.Fatalf("expected exact depth limit to pass, got %+v", okRes)
	}

	input := strings.Repeat("[", 1001) + strings.Repeat("]", 1001)
	assertInvalid(t, runCLI(t, h, []string{"canonicalize", "-"}, []byte(input)), "nesting depth")
}

func checkInputSizeLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[]`), &jcstoken.Options{MaxInputSize: 2})
	if err != nil {
		t.Fatalf("expected exact input-size limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxInputSize: 2})
	if err == nil {
		t.Fatal("expected input size error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded {
		t.Fatalf("expected BOUND_EXCEEDED, got %v", err)
	}
}

func checkValueCountLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 3})
	if err != nil {
		t.Fatalf("expected exact value-count limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxValues: 2})
	if err == nil {
		t.Fatal("expected value count error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded || !strings.Contains(je.Message, "value count") {
		t.Fatalf("expected BOUND_EXCEEDED value count error, got %v", err)
	}
}

func checkObjectMemberLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions(
		[]byte(`{"a":1,"b":2}`),
		&jcstoken.Options{MaxObjectMembers: 2},
	)
	if err != nil {
		t.Fatalf("expected exact member-count limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions(
		[]byte(`{"a":1,"b":2}`),
		&jcstoken.Options{MaxObjectMembers: 1},
	)
	if err == nil {
		t.Fatal("expected object member limit error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded || !strings.Contains(je.Message, "object member count exceeds maximum") {
		t.Fatalf("expected BOUND_EXCEEDED object member limit error, got %v", err)
	}
}

func checkArrayElementLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxArrayElements: 2})
	if err != nil {
		t.Fatalf("expected exact array-element limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`[1,2]`), &jcstoken.Options{MaxArrayElements: 1})
	if err == nil {
		t.Fatal("expected array limit error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded || !strings.Contains(je.Message, "array element count exceeds maximum") {
		t.Fatalf("expected BOUND_EXCEEDED array limit error, got %v", err)
	}
}

func checkStringByteLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxStringBytes: 2})
	if err != nil {
		t.Fatalf("expected exact string-byte limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`"ab"`), &jcstoken.Options{MaxStringBytes: 1})
	if err == nil {
		t.Fatal("expected string limit error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded || !strings.Contains(je.Message, "string decoded length exceeds maximum") {
		t.Fatalf("expected BOUND_EXCEEDED string limit error, got %v", err)
	}
}

func checkNumberTokenLengthLimitEnforced(t *testing.T, _ *harness) {
	_, err := jcstoken.ParseWithOptions([]byte(`12345`), &jcstoken.Options{MaxNumberChars: 5})
	if err != nil {
		t.Fatalf("expected exact number-token length limit to pass, got %v", err)
	}

	_, err = jcstoken.ParseWithOptions([]byte(`12345`), &jcstoken.Options{MaxNumberChars: 4})
	if err == nil {
		t.Fatal("expected number length error")
	}
	var je *jcserr.Error
	if !errors.As(err, &je) || je.Class != jcserr.BoundExceeded || !strings.Contains(je.Message, "number token length") {
		t.Fatalf("expected BOUND_EXCEEDED number length error, got %v", err)
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
	if res.exitCode != 2 || !strings.Contains(res.stderr, "usage:") || !strings.Contains(res.stderr, string(jcserr.CLIUsage)) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkUnknownCommandExitCode(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"bogus"}, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown command") || !strings.Contains(res.stderr, string(jcserr.CLIUsage)) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func checkInputViolationExitCode(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`01`))
	if res.exitCode != 2 || !strings.Contains(res.stderr, string(jcserr.InvalidGrammar)) {
		t.Fatalf("expected exit 2, got %d: %+v", res.exitCode, res)
	}

	dir := t.TempDir()
	oversizedPath := filepath.Join(dir, "oversized.json")
	if err := os.WriteFile(oversizedPath, bytes.Repeat([]byte{'x'}, jcstoken.DefaultMaxInputSize+1), 0o600); err != nil {
		t.Fatalf("write oversized fixture: %v", err)
	}

	fromFile := runCLI(t, h, []string{"canonicalize", oversizedPath}, nil)
	if fromFile.exitCode != 2 || !strings.Contains(fromFile.stderr, string(jcserr.BoundExceeded)) {
		t.Fatalf("expected BOUND_EXCEEDED exit 2 for oversized file, got %+v", fromFile)
	}

	fromStdin := runCLI(t, h, []string{"canonicalize", "-"}, bytes.Repeat([]byte{'x'}, jcstoken.DefaultMaxInputSize+1))
	if fromStdin.exitCode != 2 || !strings.Contains(fromStdin.stderr, string(jcserr.BoundExceeded)) {
		t.Fatalf("expected BOUND_EXCEEDED exit 2 for oversized stdin, got %+v", fromStdin)
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
	if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown option") || !strings.Contains(res.stderr, string(jcserr.CLIUsage)) {
		t.Fatalf("unexpected result: %+v", res)
	}
	res = runCLI(t, h, []string{"verify", "--"}, nil)
	if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown option") || !strings.Contains(res.stderr, string(jcserr.CLIUsage)) {
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
	res := runCLI(t, h, []string{"--help"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for top-level --help, got %d", res.exitCode)
	}
	if !strings.Contains(res.stdout, "usage: jcs-canon") {
		t.Fatalf("expected help on stdout, got stdout=%q", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("expected empty stderr for top-level --help, got stderr=%q", res.stderr)
	}
	// Frozen stream policy: subcommand --help writes to stdout.
	res = runCLI(t, h, []string{"canonicalize", "--help"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for canonicalize --help, got %d", res.exitCode)
	}
	if !strings.Contains(res.stdout, "usage: jcs-canon canonicalize") {
		t.Fatalf("expected help on stdout for canonicalize --help, got stdout=%q", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("expected empty stderr for canonicalize --help, got stderr=%q", res.stderr)
	}
	res = runCLI(t, h, []string{"verify", "--help"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for verify --help, got %d", res.exitCode)
	}
	if !strings.Contains(res.stdout, "usage: jcs-canon verify") {
		t.Fatalf("expected help on stdout for verify --help, got stdout=%q", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("expected empty stderr for verify --help, got stderr=%q", res.stderr)
	}
}

func checkVersionExitsZero(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"--version"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("expected exit 0 for --version, got %d", res.exitCode)
	}
	if !strings.HasPrefix(strings.TrimSpace(res.stdout), "jcs-canon v") {
		t.Fatalf("unexpected version output: %+v", res)
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
	if res.exitCode != 2 || !strings.Contains(res.stderr, "multiple input files") || !strings.Contains(res.stderr, string(jcserr.CLIUsage)) {
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

func checkErrorDiagnosticsIncludeFailureClass(t *testing.T, h *harness) {
	parseErr := runCLI(t, h, []string{"canonicalize", "-"}, []byte(`01`))
	if parseErr.exitCode != 2 || !strings.Contains(parseErr.stderr, string(jcserr.InvalidGrammar)) {
		t.Fatalf("expected INVALID_GRAMMAR class token, got %+v", parseErr)
	}

	usageErr := runCLI(t, h, []string{"verify", "--nope"}, nil)
	if usageErr.exitCode != 2 || !strings.Contains(usageErr.stderr, string(jcserr.CLIUsage)) {
		t.Fatalf("expected CLI_USAGE class token, got %+v", usageErr)
	}

	notCanonical := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"b":1,"a":2}`))
	if notCanonical.exitCode != 2 || !strings.Contains(notCanonical.stderr, string(jcserr.NotCanonical)) {
		t.Fatalf("expected NOT_CANONICAL class token, got %+v", notCanonical)
	}
}

// ==================== VERIFY ====================

func checkVerifyRejectsNonCanonicalOrder(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte(`{"b":1,"a":2}`))
	assertInvalid(t, res, "not canonical")
	if !strings.Contains(res.stderr, string(jcserr.NotCanonical)) {
		t.Fatalf("expected NOT_CANONICAL class token, got %+v", res)
	}
}

func checkVerifyRejectsNonCanonicalWhitespace(t *testing.T, h *harness) {
	res := runCLI(t, h, []string{"verify", "--quiet", "-"}, []byte("{\"a\":1}\n"))
	assertInvalid(t, res, "not canonical")
	if !strings.Contains(res.stderr, string(jcserr.NotCanonical)) {
		t.Fatalf("expected NOT_CANONICAL class token, got %+v", res)
	}
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

	// Linux support policy: release binaries must be fully static (no PT_INTERP).
	if runtime.GOOS == "linux" {
		assertELFStatic(t, out)
	}
}

func assertELFStatic(t *testing.T, path string) {
	t.Helper()

	f, err := elf.Open(path)
	if err != nil {
		t.Fatalf("open ELF %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			t.Fatalf("binary %s is dynamically linked (PT_INTERP present), expected fully static", path)
		}
	}
}

func checkNoNondeterminismSources(t *testing.T, h *harness) {
	// Verify no nondeterministic imports, no outbound/network subprocess imports,
	// and no map iteration in core/runtime paths.
	badImports := map[string]struct{}{
		"math/rand":   {},
		"crypto/rand": {},
		"time":        {},
		"net":         {},
		"net/http":    {},
		"net/url":     {},
		"net/netip":   {},
		"os/exec":     {},
	}
	srcDirs := []string{"jcserr", "jcsfloat", "jcstoken", "jcs", "cmd/jcs-canon"}
	for _, dir := range srcDirs {
		entries, err := os.ReadDir(filepath.Join(h.root, dir))
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(h.root, dir, entry.Name())
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse file %s: %v", path, err)
			}

			for _, is := range f.Imports {
				imp := strings.Trim(is.Path.Value, "\"")
				if _, bad := badImports[imp]; bad {
					t.Fatalf("file %s imports nondeterministic package %q", path, imp)
				}
			}

			mapVars := collectMapVars(f)
			ast.Inspect(f, func(n ast.Node) bool {
				rng, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				switch x := rng.X.(type) {
				case *ast.CompositeLit:
					if _, ok := x.Type.(*ast.MapType); ok {
						t.Fatalf("file %s iterates over map literal; map iteration order is nondeterministic", path)
					}
				case *ast.Ident:
					if _, ok := mapVars[x.Name]; ok {
						t.Fatalf("file %s iterates over map variable %q; map iteration order is nondeterministic", path, x.Name)
					}
				}
				return true
			})
		}
	}
}

func collectMapVars(f *ast.File) map[string]struct{} {
	mapVars := make(map[string]struct{})
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ValueSpec:
			isMap := false
			if _, ok := v.Type.(*ast.MapType); ok {
				isMap = true
			}
			if !isMap {
				for _, val := range v.Values {
					if isMapInitializer(val) {
						isMap = true
						break
					}
				}
			}
			if isMap {
				for _, name := range v.Names {
					mapVars[name.Name] = struct{}{}
				}
			}
		case *ast.AssignStmt:
			for i, rhs := range v.Rhs {
				if !isMapInitializer(rhs) || i >= len(v.Lhs) {
					continue
				}
				if ident, ok := v.Lhs[i].(*ast.Ident); ok {
					mapVars[ident.Name] = struct{}{}
				}
			}
		}
		return true
	})
	return mapVars
}

func isMapInitializer(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		_, ok := x.Type.(*ast.MapType)
		return ok
	case *ast.CallExpr:
		if fun, ok := x.Fun.(*ast.Ident); ok && fun.Name == "make" && len(x.Args) > 0 {
			_, ok := x.Args[0].(*ast.MapType)
			return ok
		}
	}
	return false
}

// --- Helpers ---

// ==================== TRACEABILITY GATES ====================

// TestMatrixRegistryParity verifies that every requirement ID in the split
// registries appears in the enforcement matrix, and vice versa.
func TestMatrixRegistryParity(t *testing.T) {
	h := testHarness(t)

	regIDs := loadRequirementIDs(
		t,
		filepath.Join(h.root, "REQ_REGISTRY_NORMATIVE.md"),
		filepath.Join(h.root, "REQ_REGISTRY_POLICY.md"),
	)
	matrixIDs := loadMatrixIDs(t, filepath.Join(h.root, "REQ_ENFORCEMENT_MATRIX.md"))

	regSet := make(map[string]struct{}, len(regIDs))
	for _, id := range regIDs {
		regSet[id] = struct{}{}
	}

	// Every registry ID must appear in matrix.
	for _, id := range regIDs {
		if _, ok := matrixIDs[id]; !ok {
			t.Errorf("registry ID %s missing from enforcement matrix", id)
		}
	}

	// Every matrix ID must appear in a registry.
	sortedMatIDs := make([]string, 0, len(matrixIDs))
	for id := range matrixIDs {
		sortedMatIDs = append(sortedMatIDs, id)
	}
	sort.Strings(sortedMatIDs)
	for _, id := range sortedMatIDs {
		if _, ok := regSet[id]; !ok {
			t.Errorf("matrix ID %s not found in any registry", id)
		}
	}
}

// TestMatrixImplSymbolsExist verifies that every impl_file+impl_symbol
// referenced in the enforcement matrix exists in the source tree.
func TestMatrixImplSymbolsExist(t *testing.T) {
	h := testHarness(t)
	rows := loadMatrixRows(t, filepath.Join(h.root, "REQ_ENFORCEMENT_MATRIX.md"))

	symbolsCache := make(map[string]map[string]symbolRange)
	lineCountCache := make(map[string]int)
	checked := 0
	for _, row := range rows {
		if row.implFile == "" || row.implSymbol == "" {
			continue
		}
		path := filepath.Join(h.root, row.implFile)

		symbols, ok := symbolsCache[path]
		if !ok {
			var err error
			symbols, lineCountCache[path], err = loadGoTopLevelSymbols(path)
			if err != nil {
				t.Errorf("%s: impl_file %q not parseable: %v", row.reqID, row.implFile, err)
				continue
			}
			symbolsCache[path] = symbols
		}

		loc, ok := symbols[row.implSymbol]
		if !ok {
			t.Errorf("%s: impl_symbol %q not found in %s", row.reqID, row.implSymbol, row.implFile)
			continue
		}

		if row.implLine != "" {
			lineNo, err := strconv.Atoi(row.implLine)
			if err != nil || lineNo < 1 {
				t.Errorf("%s: invalid impl_line %q in matrix", row.reqID, row.implLine)
			} else if maxLine := lineCountCache[path]; lineNo > maxLine {
				t.Errorf("%s: impl_line %d out of range for %s (max %d)", row.reqID, lineNo, row.implFile, maxLine)
			} else if lineNo < loc.start || lineNo > loc.end {
				t.Errorf("%s: impl_line %d does not point into impl_symbol %s (range %d-%d) in %s",
					row.reqID, lineNo, row.implSymbol, loc.start, loc.end, row.implFile)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no matrix impl symbols checked")
	}
	t.Logf("checked %d impl symbol references", checked)
}

// TestMatrixTestSymbolsExist verifies that every test_file+test_function
// referenced in the enforcement matrix exists in the source tree.
func TestMatrixTestSymbolsExist(t *testing.T) {
	h := testHarness(t)
	rows := loadMatrixRows(t, filepath.Join(h.root, "REQ_ENFORCEMENT_MATRIX.md"))

	funcsCache := make(map[string]map[string]struct{})
	checked := 0
	for _, row := range rows {
		if row.testFile == "" || row.testFunc == "" {
			continue
		}
		path := filepath.Join(h.root, row.testFile)

		funcNames, ok := funcsCache[path]
		if !ok {
			var err error
			funcNames, err = loadGoFunctionNames(path)
			if err != nil {
				t.Errorf("%s: test_file %q not parseable: %v", row.reqID, row.testFile, err)
				continue
			}
			funcsCache[path] = funcNames
		}
		// Handle conformance subtest names like TestConformanceRequirements/ID
		baseFunc := row.testFunc
		if idx := strings.IndexByte(baseFunc, '/'); idx >= 0 {
			baseFunc = baseFunc[:idx]
		}
		if _, ok := funcNames[baseFunc]; !ok {
			t.Errorf("%s: test_function %q not found in %s", row.reqID, baseFunc, row.testFile)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no matrix test symbols checked")
	}
	t.Logf("checked %d test symbol references", checked)
}

// TestBehaviorTestsLinkedToRequirements verifies that behavior test symbols in
// runtime packages are linked in the enforcement matrix.
func TestBehaviorTestsLinkedToRequirements(t *testing.T) {
	h := testHarness(t)
	checkBehaviorTestsLinkedToRequirements(t, h)
}

func checkBehaviorTestsLinkedToRequirements(t *testing.T, h *harness) {
	rows := loadMatrixRows(t, filepath.Join(h.root, "REQ_ENFORCEMENT_MATRIX.md"))

	mapped := make(map[string]map[string]struct{})
	for _, row := range rows {
		if row.testFile == "" || row.testFunc == "" {
			continue
		}
		base := row.testFunc
		if idx := strings.IndexByte(base, '/'); idx >= 0 {
			base = base[:idx]
		}
		if mapped[row.testFile] == nil {
			mapped[row.testFile] = make(map[string]struct{})
		}
		mapped[row.testFile][base] = struct{}{}
	}

	behaviorTestFiles := []string{
		"cmd/jcs-canon/main_test.go",
		"cmd/jcs-canon/blackbox_cli_test.go",
		"jcs/serialize_test.go",
		"jcserr/errors_test.go",
		"jcsfloat/jcsfloat_test.go",
		"jcstoken/token_test.go",
	}

	for _, rel := range behaviorTestFiles {
		funcs, err := loadGoFunctionNames(filepath.Join(h.root, rel))
		if err != nil {
			t.Fatalf("load test functions %s: %v", rel, err)
		}
		for fn := range funcs {
			if !strings.HasPrefix(fn, "Test") {
				continue
			}
			if _, ok := mapped[rel][fn]; !ok {
				t.Errorf("behavior test %s::%s is not linked in REQ_ENFORCEMENT_MATRIX.md", rel, fn)
			}
		}
	}
}

// TestRegistryIDFormat verifies all requirement IDs conform to the
// DOMAIN-NAME-NNN pattern.
func TestRegistryIDFormat(t *testing.T) {
	h := testHarness(t)
	ids := loadRequirementIDs(
		t,
		filepath.Join(h.root, "REQ_REGISTRY_NORMATIVE.md"),
		filepath.Join(h.root, "REQ_REGISTRY_POLICY.md"),
	)

	re := regexp.MustCompile(`^[A-Z]+-[A-Z0-9]+-[0-9]+$`)
	for _, id := range ids {
		if !re.MatchString(id) {
			t.Errorf("malformed requirement ID: %q", id)
		}
	}
	t.Logf("validated %d requirement ID formats", len(ids))
}

// TestVectorSchemaValid verifies all JSONL vector files conform to the
// expected schema: required fields id, mode/args, want_exit.
func TestVectorSchemaValid(t *testing.T) {
	h := testHarness(t)
	pattern := filepath.Join(h.root, "conformance", "vectors", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob vectors: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no vector files found")
	}

	seenIDs := make(map[string]string) // id → file
	totalVectors := 0

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := strings.Split(string(data), "\n")
		for lineNo, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				t.Errorf("%s:%d invalid JSON: %v", filepath.Base(f), lineNo+1, err)
				continue
			}

			// Required: id
			if _, ok := raw["id"]; !ok {
				t.Errorf("%s:%d missing required field 'id'", filepath.Base(f), lineNo+1)
				continue
			}
			var id string
			if err := json.Unmarshal(raw["id"], &id); err != nil {
				t.Errorf("%s:%d 'id' is not a string: %v", filepath.Base(f), lineNo+1, err)
				continue
			}

			// Unique ID across all files
			if prev, dup := seenIDs[id]; dup {
				t.Errorf("%s:%d duplicate vector ID %q (first in %s)", filepath.Base(f), lineNo+1, id, prev)
			}
			seenIDs[id] = filepath.Base(f)

			// Required: mode or args
			_, hasMode := raw["mode"]
			_, hasArgs := raw["args"]
			if !hasMode && !hasArgs {
				t.Errorf("%s:%d id=%s requires 'mode' or 'args'", filepath.Base(f), lineNo+1, id)
			}

			// Required: want_exit
			if _, ok := raw["want_exit"]; !ok {
				t.Errorf("%s:%d id=%s missing required field 'want_exit'", filepath.Base(f), lineNo+1, id)
			}

			totalVectors++
		}
	}
	if totalVectors == 0 {
		t.Fatal("no vectors validated")
	}
	t.Logf("validated %d vectors across %d files, %d unique IDs", totalVectors, len(files), len(seenIDs))
}

// TestABIManifestValid verifies the ABI manifest is valid JSON and contains
// expected top-level keys.
func TestABIManifestValid(t *testing.T) {
	h := testHarness(t)
	path := filepath.Join(h.root, "abi_manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read abi_manifest.json: %v", err)
	}

	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("abi_manifest.json is not valid JSON: %v", err)
	}

	required := []string{"abi_version", "tool", "commands", "exit_codes", "failure_classes", "stream_policy", "compatibility"}
	for _, key := range required {
		if _, ok := manifest[key]; !ok {
			t.Errorf("abi_manifest.json missing required key %q", key)
		}
	}

	// Validate failure_classes matches jcserr constants
	var classes []struct {
		Name     string `json:"name"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(manifest["failure_classes"], &classes); err != nil {
		t.Fatalf("parse failure_classes: %v", err)
	}

	expectedClasses := map[string]int{
		"INVALID_UTF8":     2,
		"INVALID_GRAMMAR":  2,
		"DUPLICATE_KEY":    2,
		"LONE_SURROGATE":   2,
		"NONCHARACTER":     2,
		"NUMBER_OVERFLOW":  2,
		"NUMBER_NEGZERO":   2,
		"NUMBER_UNDERFLOW": 2,
		"BOUND_EXCEEDED":   2,
		"NOT_CANONICAL":    2,
		"CLI_USAGE":        2,
		"INTERNAL_IO":      10,
		"INTERNAL_ERROR":   10,
	}

	for _, c := range classes {
		expected, ok := expectedClasses[c.Name]
		if !ok {
			t.Errorf("unexpected failure class in manifest: %s", c.Name)
			continue
		}
		if c.ExitCode != expected {
			t.Errorf("failure class %s: exit_code=%d, want %d", c.Name, c.ExitCode, expected)
		}
		delete(expectedClasses, c.Name)
	}
	for name := range expectedClasses {
		t.Errorf("missing failure class in manifest: %s", name)
	}
}

// TestABIManifestBehaviorParity verifies that the runtime CLI surface encoded in
// source matches abi_manifest.json for commands and flags.
func TestABIManifestBehaviorParity(t *testing.T) {
	h := testHarness(t)
	checkABIManifestBehaviorParity(t, h)
}

func checkABIManifestBehaviorParity(t *testing.T, h *harness) {
	manifest := loadABIManifest(t, filepath.Join(h.root, "abi_manifest.json"))

	srcCommands, srcGlobalFlags, srcCommandFlags := loadCLISurfaceFromSource(
		t,
		filepath.Join(h.root, "cmd", "jcs-canon", "main.go"),
	)
	assertSetEqual(t, "ABI commands", srcCommands, mapKeys(manifest.Commands))

	wantGlobalFlags := decodeManifestFlagSet(t, manifest.GlobalFlags)
	assertSetEqual(t, "ABI global flags", srcGlobalFlags, wantGlobalFlags)

	for cmdName, cmd := range manifest.Commands {
		wantCmdFlags := decodeManifestFlagSet(t, cmd.Flags)
		assertSetEqual(t, "ABI command flags "+cmdName, srcCommandFlags, wantCmdFlags)
	}
	for cmdName := range manifest.Commands {
		res := runCLI(t, h, []string{cmdName, "--help"}, nil)
		if res.exitCode != 0 {
			t.Fatalf("manifested command %q --help failed: %+v", cmdName, res)
		}
	}
}

// TestGitHubActionsPinnedBySHA verifies all workflow actions are pinned to full commit SHA.
func TestGitHubActionsPinnedBySHA(t *testing.T) {
	h := testHarness(t)
	checkGitHubActionsPinnedBySHA(t, h)
}

func checkGitHubActionsPinnedBySHA(t *testing.T, h *harness) {
	workflowFiles, err := filepath.Glob(filepath.Join(h.root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatalf("glob workflow files: %v", err)
	}
	if len(workflowFiles) == 0 {
		t.Fatal("no workflow files found")
	}

	usesRe := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*([^\s@]+)@([^\s#]+)`)
	shaRe := regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
	for _, path := range workflowFiles {
		text := mustReadText(t, path)
		matches := usesRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			actionRef := m[1]
			ref := m[2]
			if strings.HasPrefix(actionRef, "./") {
				continue
			}
			if !shaRe.MatchString(ref) {
				t.Errorf("%s has unpinned or non-SHA action ref: %s@%s", filepath.Base(path), actionRef, ref)
			}
		}
	}
}

// TestReleaseWorkflowVerificationArtifacts verifies checksum and provenance steps are present.
func TestReleaseWorkflowVerificationArtifacts(t *testing.T) {
	h := testHarness(t)
	checkReleaseWorkflowVerificationArtifacts(t, h)
}

func checkReleaseWorkflowVerificationArtifacts(t *testing.T, h *harness) {
	releaseWorkflow := mustReadText(t, filepath.Join(h.root, ".github", "workflows", "release.yml"))

	assertContains(t, releaseWorkflow, "SHA256SUMS", "release workflow checksums")
	assertContains(t, releaseWorkflow, "sha256sum", "release workflow checksums")
	assertContains(t, releaseWorkflow, "actions/attest-build-provenance@", "release workflow provenance")
	assertContains(t, releaseWorkflow, "attestations: write", "release workflow permissions")
	assertContains(t, releaseWorkflow, "id-token: write", "release workflow permissions")
}

// TestCIReproducibleBuildCheckPresent verifies CI includes deterministic-build validation.
func TestCIReproducibleBuildCheckPresent(t *testing.T) {
	h := testHarness(t)
	ciWorkflow := mustReadText(t, filepath.Join(h.root, ".github", "workflows", "ci.yml"))
	assertContains(t, ciWorkflow, "build twice and compare sha256", "ci reproducible build gate")
	assertContains(t, ciWorkflow, "-buildid=", "ci deterministic build flags")
	assertContains(t, ciWorkflow, "CGO_ENABLED=0 go build", "ci static build gate")
}

// TestGovernanceDurabilityClausesPresent verifies governance durability clauses are documented.
func TestGovernanceDurabilityClausesPresent(t *testing.T) {
	h := testHarness(t)
	checkGovernanceDurabilityClausesPresent(t, h)
}

func checkGovernanceDurabilityClausesPresent(t *testing.T, h *harness) {
	gov := mustReadText(t, filepath.Join(h.root, "GOVERNANCE.md"))
	assertContains(t, gov, "## Maintainer Policy", "governance maintainer policy")
	assertContains(t, gov, "### Review Requirements", "governance review requirements")
	assertContains(t, gov, "### Maintainer Succession", "governance succession policy")
	assertContains(t, gov, "## Support Window Policy", "governance support window")
}

// TestOfflineMatrixManifestPresent verifies offline matrix contract files exist and parse.
func TestOfflineMatrixManifestPresent(t *testing.T) {
	h := testHarness(t)
	checkOfflineMatrixManifestPresent(t, h)
}

func checkOfflineMatrixManifestPresent(t *testing.T, h *harness) {
	matrixPath := filepath.Join(h.root, "offline", "matrix.yaml")
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		t.Fatalf("load offline matrix: %v", err)
	}
	if matrix.Architecture == "" {
		t.Fatal("offline matrix architecture must be set")
	}
}

// TestOfflineProfileColdReplayPolicy verifies maximal offline profile policy knobs.
func TestOfflineProfileColdReplayPolicy(t *testing.T) {
	h := testHarness(t)
	checkOfflineProfileColdReplayPolicy(t, h)
}

func checkOfflineProfileColdReplayPolicy(t *testing.T, h *harness) {
	profilePath := filepath.Join(h.root, "offline", "profiles", "maximal.yaml")
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		t.Fatalf("load offline profile: %v", err)
	}
	if profile.MinColdReplays < 5 {
		t.Fatalf("min_cold_replays=%d, want >=5", profile.MinColdReplays)
	}
	if !profile.HardReleaseGate {
		t.Fatal("hard_release_gate must be true")
	}
}

// TestOfflineEvidenceSchemaAndVerifyCLI verifies schema and CLI verifier command exist.
func TestOfflineEvidenceSchemaAndVerifyCLI(t *testing.T) {
	h := testHarness(t)
	checkOfflineEvidenceSchemaAndVerifyCLI(t, h)
}

func checkOfflineEvidenceSchemaAndVerifyCLI(t *testing.T, h *harness) {
	schema := mustReadText(t, filepath.Join(h.root, "offline", "schema", "evidence.v1.json"))
	assertContains(t, schema, "\"schema_version\"", "offline evidence schema")
	assertContains(t, schema, "\"node_replays\"", "offline evidence schema")

	cli := mustReadText(t, filepath.Join(h.root, "cmd", "jcs-offline-replay", "main.go"))
	assertContains(t, cli, "verify-evidence", "offline replay verifier command")
	assertContains(t, cli, "ValidateEvidenceBundle", "offline replay verifier implementation")
}

// TestOfflineReleaseGatePolicy verifies offline evidence gate is documented.
func TestOfflineReleaseGatePolicy(t *testing.T) {
	h := testHarness(t)
	checkOfflineReleaseGatePolicy(t, h)
}

func checkOfflineReleaseGatePolicy(t *testing.T, h *harness) {
	releaseDoc := mustReadText(t, filepath.Join(h.root, "RELEASE_PROCESS.md"))
	assertContains(t, releaseDoc, "go test ./offline/conformance", "offline release gate command")
	assertContains(t, releaseDoc, "JCS_OFFLINE_EVIDENCE", "offline release gate environment variable")
}

// TestOfflineArchScopeX8664 verifies phase-1 offline profile architecture scope.
func TestOfflineArchScopeX8664(t *testing.T) {
	h := testHarness(t)
	checkOfflineArchScopeX8664(t, h)
}

func checkOfflineArchScopeX8664(t *testing.T, h *harness) {
	matrix, err := replay.LoadMatrix(filepath.Join(h.root, "offline", "matrix.yaml"))
	if err != nil {
		t.Fatalf("load offline matrix: %v", err)
	}
	if err := replay.ValidatePhaseOneArchitecture(matrix); err != nil {
		t.Fatalf("offline matrix architecture scope check failed: %v", err)
	}
}

// TestCitationIndexCoversNormativeRequirements verifies every normative
// requirement ID appears in the standards citation index.
func TestCitationIndexCoversNormativeRequirements(t *testing.T) {
	h := testHarness(t)
	normIDs := loadRequirementIDs(t, filepath.Join(h.root, "REQ_REGISTRY_NORMATIVE.md"))
	citationPath := filepath.Join(h.root, "standards", "CITATION_INDEX.md")
	entries := loadCitationIndexEntries(t, citationPath)

	normSet := make(map[string]struct{}, len(normIDs))
	for _, id := range normIDs {
		normSet[id] = struct{}{}
	}
	for _, id := range normIDs {
		entry, ok := entries[id]
		if !ok {
			t.Errorf("normative requirement %s missing from standards/CITATION_INDEX.md", id)
			continue
		}
		if strings.TrimSpace(entry.source) == "" || strings.Trim(entry.source, "- ") == "" {
			t.Errorf("normative requirement %s has empty source in citation index", id)
		}
		if strings.TrimSpace(entry.clause) == "" || strings.Trim(entry.clause, "- ") == "" {
			t.Errorf("normative requirement %s has empty clause in citation index", id)
		}
	}
	for id := range entries {
		if _, ok := normSet[id]; !ok {
			t.Errorf("citation index contains non-normative or unknown requirement ID %s", id)
		}
	}
	t.Logf("verified %d normative requirement IDs in citation index with structured mappings", len(normIDs))
}

// TestRequiredDocumentationPresent verifies required official docs exist.
func TestRequiredDocumentationPresent(t *testing.T) {
	h := testHarness(t)
	required := []string{
		"README.md",
		"AGENTS.md",
		"CLAUDE.md",
		"GOVERNANCE.md",
		"ARCHITECTURE.md",
		"ABI.md",
		"NORMATIVE_REFERENCES.md",
		"SPECIFICATION.md",
		"CONFORMANCE.md",
		"THREAT_MODEL.md",
		"RELEASE_PROCESS.md",
		"REQ_REGISTRY_NORMATIVE.md",
		"REQ_REGISTRY_POLICY.md",
		"REQ_ENFORCEMENT_MATRIX.md",
		"FAILURE_TAXONOMY.md",
		"abi_manifest.json",
		"standards/CITATION_INDEX.md",
		"docs/README.md",
	}

	for _, rel := range required {
		path := filepath.Join(h.root, rel)
		st, err := os.Stat(path)
		if err != nil {
			t.Errorf("required documentation file missing: %s (%v)", rel, err)
			continue
		}
		if st.IsDir() {
			t.Errorf("required documentation path is a directory, expected file: %s", rel)
			continue
		}
		if st.Size() == 0 {
			t.Errorf("required documentation file is empty: %s", rel)
		}
	}
}

// TestRequirementRegistryIndexCounts verifies REQ_REGISTRY.md count values
// stay synchronized with split requirement registries.
func TestRequirementRegistryIndexCounts(t *testing.T) {
	h := testHarness(t)
	normIDs := loadRequirementIDs(t, filepath.Join(h.root, "REQ_REGISTRY_NORMATIVE.md"))
	policyIDs := loadRequirementIDs(t, filepath.Join(h.root, "REQ_REGISTRY_POLICY.md"))

	indexPath := filepath.Join(h.root, "REQ_REGISTRY.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read REQ_REGISTRY.md: %v", err)
	}
	text := string(data)

	normCount := extractIntByPattern(t, text, `(?m)^-\s*Normative requirements:\s*([0-9]+)\s*$`, "normative count")
	policyCount := extractIntByPattern(t, text, `(?m)^-\s*Policy requirements:\s*([0-9]+)\s*$`, "policy count")
	totalCount := extractIntByPattern(t, text, `(?m)^-\s*Total requirements:\s*([0-9]+)\s*$`, "total count")

	if want := len(normIDs); normCount != want {
		t.Errorf("REQ_REGISTRY.md normative count=%d, want %d", normCount, want)
	}
	if want := len(policyIDs); policyCount != want {
		t.Errorf("REQ_REGISTRY.md policy count=%d, want %d", policyCount, want)
	}
	if want := len(normIDs) + len(policyIDs); totalCount != want {
		t.Errorf("REQ_REGISTRY.md total count=%d, want %d", totalCount, want)
	}
}

// TestABIDocsAlignedWithManifest verifies that human-readable ABI docs track
// the machine-readable manifest for commands, flags, and exit codes.
func TestABIDocsAlignedWithManifest(t *testing.T) {
	h := testHarness(t)
	manifest := loadABIManifest(t, filepath.Join(h.root, "abi_manifest.json"))

	abiDoc := mustReadText(t, filepath.Join(h.root, "ABI.md"))
	specDoc := mustReadText(t, filepath.Join(h.root, "SPECIFICATION.md"))

	commands := make([]string, 0, len(manifest.Commands))
	for name := range manifest.Commands {
		commands = append(commands, name)
	}
	sort.Strings(commands)
	for _, cmdName := range commands {
		assertContains(t, abiDoc, "`"+cmdName+"`", "ABI.md command")
		assertContains(t, specDoc, "`jcs-canon "+cmdName, "SPECIFICATION.md command synopsis")
	}

	globalFlags := make([]string, 0, len(manifest.GlobalFlags))
	for f := range manifest.GlobalFlags {
		globalFlags = append(globalFlags, f)
	}
	sort.Strings(globalFlags)
	for _, flag := range globalFlags {
		assertContains(t, abiDoc, "`"+flag+"`", "ABI.md global flag")
		assertContains(t, specDoc, flag, "SPECIFICATION.md global flag")
	}

	cmdFlags := make(map[string]struct{})
	for _, cmd := range manifest.Commands {
		for f := range cmd.Flags {
			cmdFlags[f] = struct{}{}
		}
	}
	flags := make([]string, 0, len(cmdFlags))
	for f := range cmdFlags {
		flags = append(flags, f)
	}
	sort.Strings(flags)
	for _, flag := range flags {
		assertContains(t, abiDoc, "`"+flag+"`", "ABI.md command flag")
		assertContains(t, specDoc, flag, "SPECIFICATION.md command flag")
	}

	exitCodes := make([]string, 0, len(manifest.ExitCodes))
	for code := range manifest.ExitCodes {
		exitCodes = append(exitCodes, code)
	}
	sort.Strings(exitCodes)
	for _, code := range exitCodes {
		assertContains(t, abiDoc, "`"+code+"`", "ABI.md exit code")
		assertContains(t, specDoc, "`"+code+"`", "SPECIFICATION.md exit code")
	}
}

// TestFailureTaxonomyDocAlignedWithManifest verifies every failure class in
// abi_manifest.json is documented in FAILURE_TAXONOMY.md.
func TestFailureTaxonomyDocAlignedWithManifest(t *testing.T) {
	h := testHarness(t)
	manifest := loadABIManifest(t, filepath.Join(h.root, "abi_manifest.json"))
	failureDoc := mustReadText(t, filepath.Join(h.root, "FAILURE_TAXONOMY.md"))

	for _, c := range manifest.FailureClasses {
		pattern := fmt.Sprintf(`(?m)^\|\s*%s\s*\|`, regexp.QuoteMeta(c.Name))
		if ok, err := regexp.MatchString(pattern, failureDoc); err != nil {
			t.Fatalf("compile taxonomy pattern for %s: %v", c.Name, err)
		} else if !ok {
			t.Errorf("FAILURE_TAXONOMY.md missing failure class table row for %s", c.Name)
		}
	}
}

// --- Matrix parsing helpers ---

type matrixRow struct {
	reqID      string
	domain     string
	level      string
	implFile   string
	implSymbol string
	implLine   string
	testFile   string
	testFunc   string
	gate       string
}

type citationEntry struct {
	source string
	clause string
}

type abiManifest struct {
	Commands       map[string]abiCommand      `json:"commands"`
	GlobalFlags    map[string]json.RawMessage `json:"global_flags"`
	ExitCodes      map[string]json.RawMessage `json:"exit_codes"`
	FailureClasses []abiFailureClass          `json:"failure_classes"`
}

type abiCommand struct {
	Flags map[string]json.RawMessage `json:"flags"`
}

type abiFailureClass struct {
	Name     string `json:"name"`
	ExitCode int    `json:"exit_code"`
}

type symbolRange struct {
	start int
	end   int
}

func loadMatrixIDs(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	rows := loadMatrixRows(t, path)
	ids := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		ids[r.reqID] = struct{}{}
	}
	return ids
}

func loadMatrixRows(t *testing.T, path string) []matrixRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read matrix: %v", err)
	}

	// Extract CSV block between ```csv and ```
	content := string(data)
	csvStart := strings.Index(content, "```csv\n")
	if csvStart < 0 {
		t.Fatalf("no ```csv block in matrix file")
	}
	csvStart += len("```csv\n")
	csvEnd := strings.Index(content[csvStart:], "```")
	if csvEnd < 0 {
		t.Fatalf("unterminated ```csv block in matrix file")
	}
	csvBlock := content[csvStart : csvStart+csvEnd]

	var rows []matrixRow
	for i, line := range strings.Split(csvBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip header
		if strings.HasPrefix(line, "requirement_id,") {
			continue
		}
		parts := strings.SplitN(line, ",", 9)
		if len(parts) < 9 {
			t.Fatalf("matrix line %d: expected 9 CSV fields, got %d: %q", i+1, len(parts), line)
		}
		rows = append(rows, matrixRow{
			reqID:      parts[0],
			domain:     parts[1],
			level:      parts[2],
			implFile:   parts[3],
			implSymbol: parts[4],
			implLine:   parts[5],
			testFile:   parts[6],
			testFunc:   parts[7],
			gate:       parts[8],
		})
		if parts[1] != "normative" && parts[1] != "policy" {
			t.Fatalf("matrix line %d: invalid domain %q", i+1, parts[1])
		}
		if parts[2] != "L1" && parts[2] != "L3" {
			t.Fatalf("matrix line %d: invalid level %q", i+1, parts[2])
		}
		if parts[8] != "TEST" && parts[8] != "CONFORMANCE" {
			t.Fatalf("matrix line %d: invalid gate %q", i+1, parts[8])
		}
	}
	if len(rows) == 0 {
		t.Fatal("no matrix rows found")
	}
	return rows
}

func loadGoTopLevelSymbols(path string) (map[string]symbolRange, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, 0, err
	}

	symbols := make(map[string]symbolRange)
	declRange := func(n ast.Node) symbolRange {
		return symbolRange{
			start: fset.Position(n.Pos()).Line,
			end:   fset.Position(n.End()).Line,
		}
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			symbols[d.Name.Name] = declRange(d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					symbols[s.Name.Name] = declRange(s)
				case *ast.ValueSpec:
					for _, name := range s.Names {
						symbols[name.Name] = declRange(s)
					}
				}
			}
		}
	}

	lineCount := 1 + strings.Count(string(data), "\n")
	return symbols, lineCount, nil
}

func loadGoFunctionNames(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, err
	}

	funcs := make(map[string]struct{})
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs[fn.Name.Name] = struct{}{}
		}
	}
	return funcs, nil
}

func loadCitationIndexEntries(t *testing.T, path string) map[string]citationEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read citation index: %v", err)
	}

	idPattern := regexp.MustCompile(`^[A-Z]+-[A-Z0-9]+-[0-9]+$`)
	entries := make(map[string]citationEntry)
	for lineNo, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cols := splitMarkdownTableLine(line)
		if len(cols) < 3 {
			continue
		}

		id := cols[0]
		if !idPattern.MatchString(id) {
			continue
		}
		if _, dup := entries[id]; dup {
			t.Fatalf("citation index duplicate requirement ID %s at line %d", id, lineNo+1)
		}
		entries[id] = citationEntry{
			source: cols[1],
			clause: cols[2],
		}
	}
	if len(entries) == 0 {
		t.Fatal("no requirement mappings found in citation index")
	}
	return entries
}

func splitMarkdownTableLine(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.Trim(trimmed, "|")
	raw := strings.Split(trimmed, "|")
	for i := range raw {
		raw[i] = strings.TrimSpace(raw[i])
	}
	return raw
}

func mustReadText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack, needle, context string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s missing required token %q", context, needle)
	}
}

func extractIntByPattern(t *testing.T, text, pattern, label string) int {
	t.Helper()
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) != 2 {
		t.Fatalf("failed to locate %s using pattern %q", label, pattern)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parse %s %q: %v", label, m[1], err)
	}
	return n
}

func loadABIManifest(t *testing.T, path string) abiManifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read abi manifest: %v", err)
	}
	var m abiManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse abi manifest: %v", err)
	}
	if len(m.Commands) == 0 {
		t.Fatal("abi manifest has no commands")
	}
	if len(m.GlobalFlags) == 0 {
		t.Fatal("abi manifest has no global_flags")
	}
	if len(m.ExitCodes) == 0 {
		t.Fatal("abi manifest has no exit_codes")
	}
	if len(m.FailureClasses) == 0 {
		t.Fatal("abi manifest has no failure_classes")
	}
	return m
}

func mapKeys[T any](m map[string]T) map[string]struct{} {
	keys := make(map[string]struct{}, len(m))
	for k := range m {
		keys[k] = struct{}{}
	}
	return keys
}

func decodeManifestFlagSet(t *testing.T, flags map[string]json.RawMessage) map[string]struct{} {
	t.Helper()
	type manifestFlag struct {
		Short string `json:"short"`
	}
	set := make(map[string]struct{}, len(flags))
	for longName, raw := range flags {
		set[longName] = struct{}{}
		var mf manifestFlag
		if err := json.Unmarshal(raw, &mf); err != nil {
			t.Fatalf("decode manifest flag %s: %v", longName, err)
		}
		if mf.Short != "" {
			set[mf.Short] = struct{}{}
		}
	}
	return set
}

func loadCLISurfaceFromSource(t *testing.T, path string) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	t.Helper()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse CLI source %s: %v", path, err)
	}

	commands := make(map[string]struct{})
	globalFlags := make(map[string]struct{})
	commandFlags := make(map[string]struct{})

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		switch fn.Name.Name {
		case "run":
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok || !isIndexZeroExpr(sw.Tag, "args") {
					return true
				}
				for _, stmt := range sw.Body.List {
					cc, ok := stmt.(*ast.CaseClause)
					if !ok {
						continue
					}
					for _, expr := range cc.List {
						value, ok := stringLiteralValue(expr)
						if !ok {
							continue
						}
						if strings.HasPrefix(value, "-") {
							globalFlags[value] = struct{}{}
						} else {
							commands[value] = struct{}{}
						}
					}
				}
				return true
			})
		case "parseFlags":
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok {
					return true
				}
				ident, ok := sw.Tag.(*ast.Ident)
				if !ok || ident.Name != "arg" {
					return true
				}
				for _, stmt := range sw.Body.List {
					cc, ok := stmt.(*ast.CaseClause)
					if !ok {
						continue
					}
					for _, expr := range cc.List {
						value, ok := stringLiteralValue(expr)
						if !ok {
							continue
						}
						if strings.HasPrefix(value, "-") && value != "-" {
							commandFlags[value] = struct{}{}
						}
					}
				}
				return true
			})
		}
	}

	return commands, globalFlags, commandFlags
}

func isIndexZeroExpr(expr ast.Expr, identName string) bool {
	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	base, ok := idx.X.(*ast.Ident)
	if !ok || base.Name != identName {
		return false
	}
	lit, ok := idx.Index.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func assertSetEqual(t *testing.T, label string, got, want map[string]struct{}) {
	t.Helper()
	missing := setDifference(want, got)
	extra := setDifference(got, want)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	t.Fatalf("%s mismatch missing=%v extra=%v", label, missing, extra)
}

func setDifference(a, b map[string]struct{}) []string {
	var diff []string
	for k := range a {
		if _, ok := b[k]; !ok {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

// --- Float oracle helpers ---

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
