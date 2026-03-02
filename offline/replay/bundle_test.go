package replay_test

import (
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
	_, bundleSHA, err := replay.VerifyBundle(bundlePath)
	if err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	if bundleSHA == "" {
		t.Fatal("expected non-empty bundle sha")
	}
}

func TestCreateBundleWindowsSuffix(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jcs-canon.exe")
	worker := filepath.Join(dir, "jcs-offline-worker.exe")
	matrix := filepath.Join(dir, "matrix.yaml")
	profile := filepath.Join(dir, "profile.yaml")
	vectorsDir := filepath.Join(dir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0o750); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	mustWrite(t, bin, []byte("binary-exe"), 0o755)
	mustWrite(t, worker, []byte("worker-exe"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: windows_amd64\nnodes: []\n"), 0o644)
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
	if manifest.BinaryPath != "bundle/jcs-canon.exe" {
		t.Fatalf("expected binary path bundle/jcs-canon.exe, got %s", manifest.BinaryPath)
	}
	if manifest.WorkerPath != "bundle/jcs-offline-worker.exe" {
		t.Fatalf("expected worker path bundle/jcs-offline-worker.exe, got %s", manifest.WorkerPath)
	}
	if manifest.BinarySHA256 == "" || manifest.WorkerSHA256 == "" || manifest.VectorSetSHA256 == "" {
		t.Fatalf("manifest missing checksums: %+v", manifest)
	}
	_, bundleSHA, err := replay.VerifyBundle(bundlePath)
	if err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	if bundleSHA == "" {
		t.Fatal("expected non-empty bundle sha")
	}
}

func TestExtractWorkerBinary(t *testing.T) {
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
	mustWrite(t, worker, []byte("worker-content-here"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: x86_64\nnodes: []\n"), 0o644)
	mustWrite(t, profile, []byte("version: v1\nname: p\nrequired_suites: [a]\nmin_cold_replays: 1\nhard_release_gate: true\nevidence_required: true\n"), 0o644)
	mustWrite(t, filepath.Join(vectorsDir, "core.jsonl"), []byte("{}\n"), 0o644)

	bundlePath := filepath.Join(dir, "bundle.tgz")
	if _, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  bin,
		WorkerPath:  worker,
		MatrixPath:  matrix,
		ProfilePath: profile,
		VectorsGlob: filepath.Join(vectorsDir, "*.jsonl"),
	}); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	extractDir := filepath.Join(dir, "extract")
	if err := os.MkdirAll(extractDir, 0o750); err != nil {
		t.Fatalf("mkdir extract: %v", err)
	}
	workerPath, err := replay.ExtractWorkerBinary(bundlePath, extractDir)
	if err != nil {
		t.Fatalf("extract worker: %v", err)
	}
	if filepath.Base(workerPath) != "jcs-offline-worker" {
		t.Fatalf("expected jcs-offline-worker, got %s", filepath.Base(workerPath))
	}
	data, err := os.ReadFile(workerPath)
	if err != nil {
		t.Fatalf("read extracted worker: %v", err)
	}
	if string(data) != "worker-content-here" {
		t.Fatalf("unexpected worker content: %q", data)
	}
}

func TestExtractWorkerBinaryWindowsSuffix(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jcs-canon.exe")
	worker := filepath.Join(dir, "jcs-offline-worker.exe")
	matrix := filepath.Join(dir, "matrix.yaml")
	profile := filepath.Join(dir, "profile.yaml")
	vectorsDir := filepath.Join(dir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0o750); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	mustWrite(t, bin, []byte("binary-exe"), 0o755)
	mustWrite(t, worker, []byte("worker-exe-content"), 0o755)
	mustWrite(t, matrix, []byte("version: v1\narchitecture: windows_amd64\nnodes: []\n"), 0o644)
	mustWrite(t, profile, []byte("version: v1\nname: p\nrequired_suites: [a]\nmin_cold_replays: 1\nhard_release_gate: true\nevidence_required: true\n"), 0o644)
	mustWrite(t, filepath.Join(vectorsDir, "core.jsonl"), []byte("{}\n"), 0o644)

	bundlePath := filepath.Join(dir, "bundle.tgz")
	if _, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  bin,
		WorkerPath:  worker,
		MatrixPath:  matrix,
		ProfilePath: profile,
		VectorsGlob: filepath.Join(vectorsDir, "*.jsonl"),
	}); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	extractDir := filepath.Join(dir, "extract")
	if err := os.MkdirAll(extractDir, 0o750); err != nil {
		t.Fatalf("mkdir extract: %v", err)
	}
	workerPath, err := replay.ExtractWorkerBinary(bundlePath, extractDir)
	if err != nil {
		t.Fatalf("extract worker: %v", err)
	}
	if filepath.Base(workerPath) != "jcs-offline-worker.exe" {
		t.Fatalf("expected jcs-offline-worker.exe, got %s", filepath.Base(workerPath))
	}
	data, err := os.ReadFile(workerPath)
	if err != nil {
		t.Fatalf("read extracted worker: %v", err)
	}
	if string(data) != "worker-exe-content" {
		t.Fatalf("unexpected worker content: %q", data)
	}
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
