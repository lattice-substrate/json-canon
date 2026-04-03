package replay_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestCreateAndVerifyBundle(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jcs-canon")
	worker := filepath.Join(dir, "jcs-offline-worker")
	matrix := filepath.Join(dir, "matrix.yaml")
	profile := filepath.Join(dir, "profile.yaml")
	vectorsDir := filepath.Join(dir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0o750); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	mustWrite(t, bin, []byte("binary"), 0o755)
	mustWrite(t, worker, []byte("worker"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: x86_64\nnodes: []\n"), 0o644)
	mustWrite(t, profile, []byte("version: v1\nname: p\nrequired_suites: [a]\nmin_cold_replays: 1\nhard_release_gate: true\nevidence_required: true\n"), 0o644)
	mustWrite(t, filepath.Join(vectorsDir, "core.jsonl"), []byte("{}\n"), 0o644)

	bundlePath := filepath.Join(dir, "bundle.tgz")
	manifest, err := replay.CreateBundle(replay.BundleOptions{
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
	if manifest.CreatedAtUTC != "1970-01-01T00:00:00Z" {
		t.Fatalf("created_at_utc = %q, want deterministic epoch timestamp", manifest.CreatedAtUTC)
	}
	_, bundleSHA, err := replay.VerifyBundle(bundlePath)
	if err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	if bundleSHA == "" {
		t.Fatal("expected non-empty bundle sha")
	}
}

func TestCreateBundleIsByteDeterministic(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jcs-canon")
	worker := filepath.Join(dir, "jcs-offline-worker")
	matrix := filepath.Join(dir, "matrix.yaml")
	profile := filepath.Join(dir, "profile.yaml")
	vectorsDir := filepath.Join(dir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0o750); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	mustWrite(t, bin, []byte("binary"), 0o755)
	mustWrite(t, worker, []byte("worker"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: x86_64\nnodes: []\n"), 0o644)
	mustWrite(t, profile, []byte("version: v1\nname: p\nrequired_suites: [a]\nmin_cold_replays: 1\nhard_release_gate: true\nevidence_required: true\n"), 0o644)
	mustWrite(t, filepath.Join(vectorsDir, "core.jsonl"), []byte("{}\n"), 0o644)

	firstPath := filepath.Join(dir, "bundle-first.tgz")
	secondPath := filepath.Join(dir, "bundle-second.tgz")
	opts := replay.BundleOptions{
		OutputPath:  firstPath,
		BinaryPath:  bin,
		WorkerPath:  worker,
		MatrixPath:  matrix,
		ProfilePath: profile,
		VectorsGlob: filepath.Join(vectorsDir, "*.jsonl"),
	}
	if _, err := replay.CreateBundle(opts); err != nil {
		t.Fatalf("create first bundle: %v", err)
	}
	opts.OutputPath = secondPath
	if _, err := replay.CreateBundle(opts); err != nil {
		t.Fatalf("create second bundle: %v", err)
	}

	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 test reads the temp bundle path it just wrote.
	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first bundle: %v", err)
	}
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 test reads the temp bundle path it just wrote.
	secondBytes, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second bundle: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("bundle bytes differ for identical inputs")
	}
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
