package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestRun_EndToEndBundleReplay(t *testing.T) {
	root := t.TempDir()
	vectorPath := filepath.Join(root, "vectors", "cases.jsonl")
	mustMkdirAllWorker(t, filepath.Dir(vectorPath))
	writeVectorCases(t, vectorPath, []vectorCase{
		{
			ID:         "canonicalize-case",
			Mode:       "canonicalize",
			Input:      `{"b":1,"a":2}`,
			WantStdout: strPtr(`{"a":2,"b":1}`),
			WantExit:   0,
		},
		{
			ID:         "verify-case",
			Mode:       "verify",
			Input:      `{"a":2,"b":1}`,
			WantStdout: strPtr("verified"),
			WantExit:   0,
		},
	})

	binaryPath := filepath.Join(root, "bin", "jcs-canon")
	workerPath := filepath.Join(root, "bin", "jcs-offline-worker")
	matrixPath := filepath.Join(root, "inputs", "matrix.json")
	profilePath := filepath.Join(root, "inputs", "profile.json")

	writeFakeCanonicalizer(t, binaryPath)
	mustWriteFileWorker(t, workerPath, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o700)
	mustMkdirAllWorker(t, filepath.Dir(matrixPath))
	mustWriteFileWorker(t, matrixPath, []byte("{\"fixture\":true}\n"), 0o600)
	mustWriteFileWorker(t, profilePath, []byte("{\"fixture\":true}\n"), 0o600)

	bundlePath := filepath.Join(root, "offline-bundle.tgz")
	_, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  binaryPath,
		WorkerPath:  workerPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: filepath.Join(root, "vectors", "*.jsonl"),
		Version:     "bundle.v1.test",
	})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}

	evidencePath := filepath.Join(root, "node-evidence.json")
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{
		"--bundle", bundlePath,
		"--evidence", evidencePath,
		"--node-id", "node-a",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "1",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q stdout=%q", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "ok node=node-a replay=1 cases=2") {
		t.Fatalf("unexpected stdout: %q", out.String())
	}

	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 test fixture reads the evidence path it just wrote inside t.TempDir().
	evidenceData, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	var evidence replay.NodeRunEvidence
	if err := json.Unmarshal(evidenceData, &evidence); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if evidence.CaseCount != 2 {
		t.Fatalf("case count=%d want 2", evidence.CaseCount)
	}
	if evidence.NodeID != "node-a" {
		t.Fatalf("node id=%q want node-a", evidence.NodeID)
	}
}

func TestRunVectors_NoExecutedCases(t *testing.T) {
	root := t.TempDir()
	vectorPath := filepath.Join(root, "vectors", "empty.jsonl")
	mustMkdirAllWorker(t, filepath.Dir(vectorPath))
	mustWriteFileWorker(t, vectorPath, []byte("# comment\n\n"), 0o600)

	manifest := &replay.BundleManifest{
		VectorFiles: []string{"vectors/empty.jsonl"},
	}
	_, err := runVectors(
		"/bin/true",
		root,
		manifest,
		&digestAccumulator{},
		&digestAccumulator{},
		&digestAccumulator{},
		&digestAccumulator{},
	)
	if err == nil || !strings.Contains(err.Error(), "no vector cases executed") {
		t.Fatalf("expected no-vector-cases error, got %v", err)
	}
}

func TestExtractTarFile_UnsafePath(t *testing.T) {
	err := extractTarFile(nil, t.TempDir(), &tar.Header{
		Name: "../escape",
		Mode: 0o644,
		Size: 0,
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe tar path") {
		t.Fatalf("expected unsafe tar path error, got %v", err)
	}
}

func TestRunCLI_MissingBinary(t *testing.T) {
	_, err := runCLI(filepath.Join(t.TempDir(), "missing-binary"), []string{"canonicalize", "-"}, []byte(`{}`), nil)
	if err == nil {
		t.Fatal("expected runCLI error for missing binary")
	}
}

func writeVectorCases(t *testing.T, path string, cases []vectorCase) {
	t.Helper()
	lines := make([]string, 0, len(cases))
	for _, tc := range cases {
		encoded, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("marshal vector case %q: %v", tc.ID, err)
		}
		lines = append(lines, string(encoded))
	}
	mustWriteFileWorker(t, path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func writeFakeCanonicalizer(t *testing.T, path string) {
	t.Helper()
	mustMkdirAllWorker(t, filepath.Dir(path))
	script := `#!/usr/bin/env bash
set -euo pipefail
mode="${1:-}"
if [ "$mode" = "canonicalize" ]; then
  cat >/dev/null
  printf '{"a":2,"b":1}'
  exit 0
fi
if [ "$mode" = "verify" ]; then
  cat >/dev/null
  printf 'verified'
  exit 0
fi
echo "jcserr: CLI_USAGE: unsupported mode" >&2
exit 2
`
	mustWriteFileWorker(t, path, []byte(script), 0o700)
}

func mustMkdirAllWorker(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFileWorker(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func strPtr(s string) *string {
	return &s
}
