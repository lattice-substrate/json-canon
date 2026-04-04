package conformance_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialCyberphoneCanonicalPairs(t *testing.T) {
	h := testHarness(t)
	checkOfficialCyberphoneVectors(t, h)
}

func TestOfficialCyberphoneFixtureProvenance(t *testing.T) {
	runHarnessOfficialSuite(t, resolveHarnessRepoRoot(t, ""), resolveRepoRootFromHarness(t), "TestOfficialCyberphoneFixtureProvenance", nil, "30m")
}

func TestOfficialRFC8785Vectors(t *testing.T) {
	h := testHarness(t)
	checkOfficialRFC8785Vectors(t, h)
}

func TestOfficialES6CorpusChecksums10K(t *testing.T) {
	h := testHarness(t)
	checkOfficialES6Corpus10K(t, h)
}

func TestOfficialES6CorpusChecksums100M(t *testing.T) {
	if lookupEnvTrimmed("JCS_OFFICIAL_ES6_ENABLE_100M") != "1" {
		t.Skip("set JCS_OFFICIAL_ES6_ENABLE_100M=1 to run 100M official ES6 checksum gate")
	}
	runHarnessOfficialSuite(t, resolveHarnessRepoRoot(t, ""), resolveRepoRootFromHarness(t), "TestOfficialES6CorpusChecksums100M", map[string]string{
		"JCS_OFFICIAL_ES6_ENABLE_100M": "1",
	}, "6h")
}

func TestOfficialES6100MReleaseGatePolicy(t *testing.T) {
	h := testHarness(t)
	checkOfficialES6100MReleaseGatePolicy(t, h)
}

func checkOfficialCyberphoneVectors(t *testing.T, h *harness) {
	t.Helper()
	runHarnessOfficialSuite(t, resolveHarnessRepoRoot(t, h.root), h.root, "TestOfficialCyberphoneCanonicalPairs", nil, "30m")
}

func checkOfficialRFC8785Vectors(t *testing.T, h *harness) {
	t.Helper()
	runHarnessOfficialSuite(t, resolveHarnessRepoRoot(t, h.root), h.root, "TestOfficialRFC8785Vectors", nil, "30m")
}

func checkOfficialES6Corpus10K(t *testing.T, h *harness) {
	t.Helper()
	runHarnessOfficialSuite(t, resolveHarnessRepoRoot(t, h.root), h.root, "TestOfficialES6CorpusChecksums10K", nil, "30m")
}

func checkOfficialES6100MReleaseGatePolicy(t *testing.T, h *harness) {
	t.Helper()
	releaseWorkflow := mustReadText(t, filepath.Join(h.root, ".github", "workflows", "release.yml"))
	assertContains(t, releaseWorkflow, "official ES6 100M checksum gate", "release workflow official 100M gate step")
	assertContains(t, releaseWorkflow, "JCS_OFFICIAL_ES6_ENABLE_100M", "release workflow official 100M gate env")
	assertContains(t, releaseWorkflow, "go -C ../jcs-conformance-harness test ./official", "release workflow external official gate invocation")
	assertContains(t, releaseWorkflow, "TestOfficialES6CorpusChecksums100M", "release workflow official 100M test name")

	releaseDoc := mustReadText(t, filepath.Join(h.root, "CONTRIBUTING.md"))
	assertContains(t, releaseDoc, "JCS_OFFICIAL_ES6_ENABLE_100M=1", "release process 100M command")
	assertContains(t, releaseDoc, "go -C ../jcs-conformance-harness test ./official", "release process external official gate invocation")
	assertContains(t, releaseDoc, "TestOfficialES6CorpusChecksums100M", "release process 100M test name")
}

func runHarnessOfficialSuite(t *testing.T, harnessRoot, jsonCanonRoot, pattern string, env map[string]string, timeout string) {
	t.Helper()
	args := []string{"-C", harnessRoot, "test", "./official", "-run", pattern, "-count=1"}
	if strings.TrimSpace(timeout) != "" {
		args = append(args, "-timeout="+timeout)
	}
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "JCS_CONFORMANCE_JSON_CANON_REPO="+jsonCanonRoot)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("run harness official suite %s: %v\n%s", pattern, err, output.String())
	}
}

func resolveHarnessRepoRoot(t *testing.T, repoRoot string) string {
	t.Helper()
	if root := lookupEnvTrimmed("JCS_CONFORMANCE_REPO"); root != "" {
		return root
	}
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = resolveRepoRootFromHarness(t)
	}
	root := filepath.Clean(filepath.Join(repoRoot, "..", "jcs-conformance-harness"))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve jcs-conformance-harness repo root: %v", err)
	}
	return root
}

func resolveRepoRootFromHarness(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		t.Fatalf("resolve json-canon repo root: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func lookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
