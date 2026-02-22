package conformance_test

import (
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
	matrixPath := filepath.Join(root, "offline", "matrix.yaml")
	profilePath := filepath.Join(root, "offline", "profiles", "maximal.yaml")

	m, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	if m.Architecture != "x86_64" {
		t.Fatalf("expected x86_64 architecture, got %q", m.Architecture)
	}

	p, err := replay.LoadProfile(profilePath)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if p.MinColdReplays < 5 {
		t.Fatalf("expected min cold replays >= 5, got %d", p.MinColdReplays)
	}
	if !p.HardReleaseGate {
		t.Fatal("expected hard_release_gate=true")
	}
}

func TestOfflineEvidenceSchemaPresent(t *testing.T) {
	root := repoRoot(t)
	schemaPath := filepath.Join(root, "offline", "schema", "evidence.v1.json")
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
}

func TestOfflineReplayEvidenceReleaseGate(t *testing.T) {
	root := repoRoot(t)
	evidencePath := strings.TrimSpace(os.Getenv("JCS_OFFLINE_EVIDENCE"))
	if evidencePath == "" {
		t.Skip("set JCS_OFFLINE_EVIDENCE to validate offline evidence bundle")
	}
	matrix, err := replay.LoadMatrix(filepath.Join(root, "offline", "matrix.yaml"))
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	profile, err := replay.LoadProfile(filepath.Join(root, "offline", "profiles", "maximal.yaml"))
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if err := replay.ValidateEvidenceBundle(evidence, matrix, profile); err != nil {
		t.Fatalf("offline evidence gate failed: %v", err)
	}
}

func mustReadText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
