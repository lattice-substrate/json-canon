package conformance_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func TestOfflineMatrixAndProfileContracts(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		matrixPath   string
		profilePath  string
		architecture string
	}{
		{
			matrixPath:   filepath.Join(root, "offline", "matrix.yaml"),
			profilePath:  filepath.Join(root, "offline", "profiles", "maximal.yaml"),
			architecture: "x86_64",
		},
		{
			matrixPath:   filepath.Join(root, "offline", "matrix.arm64.yaml"),
			profilePath:  filepath.Join(root, "offline", "profiles", "maximal.arm64.yaml"),
			architecture: "arm64",
		},
	}

	for _, tc := range tests {
		m, err := replay.LoadMatrix(tc.matrixPath)
		if err != nil {
			t.Fatalf("load matrix %s: %v", tc.matrixPath, err)
		}
		if m.Architecture != tc.architecture {
			t.Fatalf("expected architecture %q for %s, got %q", tc.architecture, tc.matrixPath, m.Architecture)
		}
		archErr := replay.ValidateReleaseArchitecture(m)
		if archErr != nil {
			t.Fatalf("validate release architecture for %s: %v", tc.matrixPath, archErr)
		}

		p, err := replay.LoadProfile(tc.profilePath)
		if err != nil {
			t.Fatalf("load profile %s: %v", tc.profilePath, err)
		}
		if p.MinColdReplays < 5 {
			t.Fatalf("expected min cold replays >= 5 for %s, got %d", tc.profilePath, p.MinColdReplays)
		}
		if !p.HardReleaseGate {
			t.Fatalf("expected hard_release_gate=true for %s", tc.profilePath)
		}
	}
}

func TestOfflineEvidenceSchemaPresent(t *testing.T) {
	root := repoRoot(t)
	schemaPath := filepath.Join(root, "offline", "schema", "evidence.v1.json")
	// #nosec G304 -- conformance test intentionally reads repository schema path.
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	for _, needle := range []string{"schema_version", "node_replays", "aggregate_canonical_sha256"} {
		if !strings.Contains(string(data), needle) {
			t.Fatalf("schema missing %q", needle)
		}
	}
}

func TestOfflineReleaseGateDocumentation(t *testing.T) {
	root := repoRoot(t)
	releaseDoc := mustReadText(t, filepath.Join(root, "RELEASE_PROCESS.md"))
	if !strings.Contains(releaseDoc, "offline") {
		t.Fatal("RELEASE_PROCESS.md missing offline gate section")
	}
	if !strings.Contains(releaseDoc, "go test ./offline/conformance") {
		t.Fatal("RELEASE_PROCESS.md missing offline conformance gate command")
	}
	releaseWorkflow := mustReadText(t, filepath.Join(root, ".github", "workflows", "release.yml"))
	if !strings.Contains(releaseWorkflow, "TestOfflineReplayEvidenceReleaseGate") {
		t.Fatal("release workflow missing explicit offline evidence gate test invocation")
	}
	if !strings.Contains(releaseWorkflow, "JCS_OFFLINE_EVIDENCE") {
		t.Fatal("release workflow missing JCS_OFFLINE_EVIDENCE for offline evidence gate")
	}
	if !strings.Contains(releaseWorkflow, "JCS_OFFLINE_MATRIX") {
		t.Fatal("release workflow missing JCS_OFFLINE_MATRIX for offline evidence gate")
	}
	if !strings.Contains(releaseWorkflow, "JCS_OFFLINE_PROFILE") {
		t.Fatal("release workflow missing JCS_OFFLINE_PROFILE for offline evidence gate")
	}
	if !strings.Contains(releaseWorkflow, "offline evidence gate arm64") {
		t.Fatal("release workflow missing explicit arm64 offline evidence gate")
	}
}

func TestOfflineReplayEvidenceReleaseGate(t *testing.T) {
	root := repoRoot(t)
	evidencePath := lookupEnvTrimmed("JCS_OFFLINE_EVIDENCE")
	if evidencePath == "" {
		t.Skip("set JCS_OFFLINE_EVIDENCE to validate offline evidence bundle")
	}
	bundlePath := lookupEnvTrimmed("JCS_OFFLINE_BUNDLE")
	controlBinaryPath := lookupEnvTrimmed("JCS_OFFLINE_CONTROL_BINARY")
	if bundlePath == "" || controlBinaryPath == "" {
		defaultBundle, defaultControl := defaultEvidenceArtifactPaths(evidencePath)
		if bundlePath == "" {
			bundlePath = defaultBundle
		}
		if controlBinaryPath == "" {
			controlBinaryPath = defaultControl
		}
	}

	matrixPath := lookupEnvTrimmed("JCS_OFFLINE_MATRIX")
	if matrixPath == "" {
		matrixPath = filepath.Join(root, "offline", "matrix.yaml")
	}
	profilePath := lookupEnvTrimmed("JCS_OFFLINE_PROFILE")
	if profilePath == "" {
		profilePath = filepath.Join(root, "offline", "profiles", "maximal.yaml")
	}
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if err := replay.ValidateEvidenceBundle(evidence, matrix, profile, replay.EvidenceValidationOptions{
		ExpectedBundleSHA256:        mustFileSHA256(t, bundlePath),
		ExpectedControlBinarySHA256: mustFileSHA256(t, controlBinaryPath),
		ExpectedMatrixSHA256:        mustFileSHA256(t, matrixPath),
		ExpectedProfileSHA256:       mustFileSHA256(t, profilePath),
		ExpectedArchitecture:        matrix.Architecture,
	}); err != nil {
		t.Fatalf("offline evidence gate failed: %v", err)
	}
}

func defaultEvidenceArtifactPaths(evidencePath string) (string, string) {
	base := filepath.Dir(evidencePath)
	return filepath.Join(base, "offline-bundle.tgz"), filepath.Join(base, "bin", "jcs-canon")
}

func lookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func mustReadText(t *testing.T, path string) string {
	t.Helper()
	// #nosec G304 -- conformance test intentionally reads repository documentation paths.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func mustFileSHA256(t *testing.T, path string) string {
	t.Helper()
	// #nosec G304 -- conformance test intentionally reads explicit artifact paths.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
