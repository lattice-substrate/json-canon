package conformance_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialCyberphoneCanonicalPairs(t *testing.T) {
	h := testHarness(t)
	checkOfficialHarnessVector(t, h, "official/req-0354-positive")
}

func TestOfficialRFC8785Vectors(t *testing.T) {
	h := testHarness(t)
	checkOfficialHarnessVector(t, h, "official/req-0355-positive")
}

func TestOfficialES6CorpusChecksums10K(t *testing.T) {
	h := testHarness(t)
	checkOfficialHarnessVector(t, h, "official/req-0356-positive")
}

func TestOfficialES6CorpusChecksums100M(t *testing.T) {
	if lookupEnvTrimmed("JCS_OFFICIAL_ES6_ENABLE_100M") != "1" {
		t.Skip("set JCS_OFFICIAL_ES6_ENABLE_100M=1 to run 100M official ES6 checksum gate")
	}
	h := testHarness(t)
	res := runCLI(t, h, []string{"check-es6-corpus", "--lines", "100000000"}, nil)
	if res.exitCode != 0 {
		t.Fatalf("check-es6-corpus 100M failed: %+v", res)
	}
	const want = "0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272"
	if strings.TrimSpace(res.stdout) != want {
		t.Fatalf("ES6 100M checksum mismatch: got=%s want=%s", strings.TrimSpace(res.stdout), want)
	}
}

func TestOfficialES6100MReleaseGatePolicy(t *testing.T) {
	h := testHarness(t)
	checkOfficialES6100MReleaseGatePolicy(t, h)
}

func checkOfficialHarnessVector(t *testing.T, h *harness, vectorID string) {
	t.Helper()
	results := runHarnessOfficialFamily(t, h.root, h.bin)
	verdict, ok := results[vectorID]
	if !ok {
		t.Fatalf("missing official vector result %q", vectorID)
	}
	if verdict != "pass" {
		t.Fatalf("official vector %s failed", vectorID)
	}
}

func checkOfficialCyberphoneVectors(t *testing.T, h *harness) {
	t.Helper()
	checkOfficialHarnessVector(t, h, "official/req-0354-positive")
}

func checkOfficialRFC8785Vectors(t *testing.T, h *harness) {
	t.Helper()
	checkOfficialHarnessVector(t, h, "official/req-0355-positive")
}

func checkOfficialES6Corpus10K(t *testing.T, h *harness) {
	t.Helper()
	checkOfficialHarnessVector(t, h, "official/req-0356-positive")
}

func checkOfficialES6100MReleaseGatePolicy(t *testing.T, h *harness) {
	t.Helper()
	releaseWorkflow := mustReadText(t, filepath.Join(h.root, ".github", "workflows", "release.yml"))
	assertContains(t, releaseWorkflow, "official ES6 100M checksum gate", "release workflow official 100M gate step")
	assertContains(t, releaseWorkflow, "JCS_OFFICIAL_ES6_ENABLE_100M", "release workflow official 100M gate env")
	assertContains(t, releaseWorkflow, "go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h -v", "release workflow local official 100M gate invocation")

	releaseDoc := mustReadText(t, filepath.Join(h.root, "CONTRIBUTING.md"))
	assertContains(t, releaseDoc, "JCS_OFFICIAL_ES6_ENABLE_100M=1", "release process 100M command")
	assertContains(t, releaseDoc, "go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h", "release process local official 100M gate invocation")
}

func runHarnessOfficialFamily(t *testing.T, repoRoot, implPath string) map[string]string {
	t.Helper()
	harnessRoot := resolveHarnessRepoRoot(t, repoRoot)
	specRoot := resolveSpecRepoRoot(t, repoRoot)
	outputPath := filepath.Join(t.TempDir(), "official.result.json")
	args := []string{
		"-C", harnessRoot,
		"run", "./cmd/jcs-conformance", "run",
		"--impl", implPath,
		"--known-reqs", filepath.Join(specRoot, "registries", "requirements-registry.json"),
		"--family", "official",
		"--output", outputPath,
		"--quiet",
	}
	//nolint:gosec // REQ:CONFORMANCE-001 subprocess args are test-controlled harness paths, not user input.
	cmd := exec.Command("go", args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("run harness official family: %v\n%s", err, output.String())
	}

	//nolint:gosec // REQ:CONFORMANCE-001 output path is test-controlled temporary file.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read official result artifact: %v", err)
	}
	var artifact struct {
		Results []struct {
			VectorID string `json:"vector_id"`
			Verdict  string `json:"verdict"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decode official result artifact: %v", err)
	}
	results := make(map[string]string, len(artifact.Results))
	for _, result := range artifact.Results {
		results[result.VectorID] = result.Verdict
	}
	return results
}

func resolveHarnessRepoRoot(t *testing.T, repoRoot string) string {
	t.Helper()
	if root := lookupEnvTrimmed("JCS_CONFORMANCE_REPO"); root != "" {
		return root
	}
	root := filepath.Clean(filepath.Join(repoRoot, "..", "jcs-conformance-harness"))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve jcs-conformance-harness repo root: %v", err)
	}
	return root
}

func resolveSpecRepoRoot(t *testing.T, repoRoot string) string {
	t.Helper()
	if root := lookupEnvTrimmed("JCS_SPEC_REPO"); root != "" {
		return root
	}
	root := filepath.Clean(filepath.Join(repoRoot, "..", "jcs-spec"))
	if _, err := os.Stat(filepath.Join(root, "registries", "requirements-registry.json")); err != nil {
		t.Fatalf("resolve jcs-spec repo root: %v", err)
	}
	return root
}

func lookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
