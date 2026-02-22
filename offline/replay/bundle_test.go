package replay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndVerifyBundle(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jcs-canon")
	worker := filepath.Join(dir, "jcs-offline-worker")
	matrix := filepath.Join(dir, "matrix.yaml")
	profile := filepath.Join(dir, "profile.yaml")
	vectorsDir := filepath.Join(dir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0o755); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	mustWrite(t, bin, []byte("binary"), 0o755)
	mustWrite(t, worker, []byte("worker"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: x86_64\nnodes: []\n"), 0o644)
	mustWrite(t, profile, []byte("version: v1\nname: p\nrequired_suites: [a]\nmin_cold_replays: 1\nhard_release_gate: true\nevidence_required: true\n"), 0o644)
	mustWrite(t, filepath.Join(vectorsDir, "core.jsonl"), []byte("{}\n"), 0o644)

	bundlePath := filepath.Join(dir, "bundle.tgz")
	manifest, err := CreateBundle(BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  bin,
		WorkerPath:  worker,
		MatrixPath:  matrix,
		ProfilePath: profile,
		VectorsGlob: filepath.Join(vectorsDir, "*.jsonl"),
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if manifest.BinarySHA256 == "" || manifest.VectorSetSHA256 == "" {
		t.Fatalf("manifest missing checksums: %+v", manifest)
	}
	_, bundleSHA, err := VerifyBundle(bundlePath)
	if err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	if bundleSHA == "" {
		t.Fatal("expected non-empty bundle sha")
	}
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
