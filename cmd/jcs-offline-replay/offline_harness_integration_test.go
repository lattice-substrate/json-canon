package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const testDigestSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestRunSuiteAndAuditSummary_WithFakeToolchain(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	mustMkdirAll(t, fakeBin)
	writeFakeGoBinary(t, filepath.Join(fakeBin, "go"))
	replayScript := writeFakeReplayCommand(t, filepath.Join(fakeBin, "fake-replay"))
	writeMinimalVectorFixture(t, root)

	matrixPath, profilePath := writeHarnessMatrixAndProfile(t, root, "x86_64", replayScript, "single")
	pathValue, _ := os.LookupEnv("PATH")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+pathValue)
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", strings.Repeat("1", 40))
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "v1.2.3-test")

	withWorkingDirectory(t, root, func() {
		outDir := filepath.Join(root, "runs", "single")
		var runOut bytes.Buffer
		artifacts, err := runSuite(runSuiteOptions{
			MatrixPath:      matrixPath,
			ProfilePath:     profilePath,
			OutputDir:       outDir,
			Timeout:         time.Minute,
			Version:         "v0.0.1-test",
			SkipPreflight:   true,
			SkipReleaseGate: true,
		}, &runOut)
		if err != nil {
			t.Fatalf("runSuite: %v\nstdout:\n%s", err, runOut.String())
		}
		if artifacts == nil {
			t.Fatal("runSuite returned nil artifacts")
		}
		for _, p := range []string{
			filepath.Join(outDir, "offline-bundle.tgz"),
			filepath.Join(outDir, "offline-evidence.json"),
			filepath.Join(outDir, "RUN_INDEX.txt"),
			filepath.Join(outDir, "audit", "audit-summary.json"),
			filepath.Join(outDir, "audit", "audit-summary.md"),
			filepath.Join(outDir, "audit", "controller-report.txt"),
		} {
			if _, statErr := os.Stat(p); statErr != nil {
				t.Fatalf("expected artifact %q: %v", p, statErr)
			}
		}
		evidence, err := replay.LoadEvidence(filepath.Join(outDir, "offline-evidence.json"))
		if err != nil {
			t.Fatalf("load evidence: %v", err)
		}
		if len(evidence.NodeReplays) != 2 {
			t.Fatalf("node replay count=%d want 2", len(evidence.NodeReplays))
		}

		auditOutDir := filepath.Join(root, "runs", "audit-summary")
		var auditOut bytes.Buffer
		err = cmdAuditSummary(map[string]string{
			"--matrix":     matrixPath,
			"--profile":    profilePath,
			"--evidence":   filepath.Join(outDir, "offline-evidence.json"),
			"--output-dir": auditOutDir,
		}, &auditOut)
		if err != nil {
			t.Fatalf("cmdAuditSummary: %v\nstdout:\n%s", err, auditOut.String())
		}
		if !strings.Contains(auditOut.String(), "[audit] wrote:") {
			t.Fatalf("unexpected audit output: %q", auditOut.String())
		}

		var wrappedOut bytes.Buffer
		err = cmdRunSuite(map[string]string{
			"--matrix":            matrixPath,
			"--profile":           profilePath,
			"--output-dir":        filepath.Join(root, "runs", "wrapped"),
			"--timeout":           "30s",
			"--version":           "v0.0.2-test",
			"--skip-preflight":    boolTrue,
			"--skip-release-gate": boolTrue,
		}, &wrappedOut)
		if err != nil {
			t.Fatalf("cmdRunSuite: %v\nstdout:\n%s", err, wrappedOut.String())
		}
	})
}

func TestCmdCrossArch_WithFakeToolchain(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	mustMkdirAll(t, fakeBin)
	writeFakeGoBinary(t, filepath.Join(fakeBin, "go"))
	replayScript := writeFakeReplayCommand(t, filepath.Join(fakeBin, "fake-replay"))
	writeMinimalVectorFixture(t, root)

	x86Matrix, x86Profile := writeHarnessMatrixAndProfile(t, root, "x86_64", replayScript, "x86")
	armMatrix, armProfile := writeHarnessMatrixAndProfile(t, root, "arm64", replayScript, "arm64")

	pathValue, _ := os.LookupEnv("PATH")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+pathValue)
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", strings.Repeat("2", 40))
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "v2.0.0-test")

	withWorkingDirectory(t, root, func() {
		outDir := filepath.Join(root, "runs", "cross")
		var out bytes.Buffer
		err := cmdCrossArch(map[string]string{
			"--x86-matrix":        x86Matrix,
			"--x86-profile":       x86Profile,
			"--arm64-matrix":      armMatrix,
			"--arm64-profile":     armProfile,
			"--output-dir":        outDir,
			"--timeout":           "2m",
			"--skip-preflight":    boolTrue,
			"--skip-release-gate": boolTrue,
		}, &out)
		if err != nil {
			t.Fatalf("cmdCrossArch: %v\nstdout:\n%s", err, out.String())
		}
		if !strings.Contains(out.String(), "[cross-arch] RESULT=PASS") {
			t.Fatalf("unexpected cross-arch output: %q", out.String())
		}
		for _, p := range []string{
			filepath.Join(outDir, "cross-arch-compare.json"),
			filepath.Join(outDir, "cross-arch-compare.md"),
		} {
			if _, statErr := os.Stat(p); statErr != nil {
				t.Fatalf("expected report %q: %v", p, statErr)
			}
		}
	})
}

func TestRunPreflight_PassAndFailModes(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	mustMkdirAll(t, fakeBin)
	writeFakeCommand(t, filepath.Join(fakeBin, "go"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "tar"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "podman"), "#!/usr/bin/env bash\nset -euo pipefail\nif [ \"${1:-}\" = \"info\" ]; then exit 0; fi\nif [ \"${1:-}\" = \"image\" ] && [ \"${2:-}\" = \"inspect\" ]; then exit 0; fi\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "ssh"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "scp"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "virsh"), "#!/usr/bin/env bash\nset -euo pipefail\nif [ \"${1:-}\" = \"dominfo\" ]; then exit 0; fi\nif [ \"${1:-}\" = \"snapshot-list\" ]; then echo snapshot-cold; exit 0; fi\nexit 0\n")

	matrixPath, _ := writeHarnessMatrixAndProfile(t, root, "x86_64", "/bin/true", "preflight")

	pathValue, _ := os.LookupEnv("PATH")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+pathValue)
	var passOut bytes.Buffer
	if err := runPreflight(matrixPath, true, &passOut); err != nil {
		t.Fatalf("runPreflight(pass): %v\nstdout:\n%s", err, passOut.String())
	}
	if !strings.Contains(passOut.String(), "[preflight] RESULT=PASS") {
		t.Fatalf("missing pass marker: %q", passOut.String())
	}

	t.Setenv("PATH", t.TempDir())
	var failOut bytes.Buffer
	err := runPreflight(matrixPath, false, &failOut)
	if err == nil {
		t.Fatal("expected preflight failure with missing tooling")
	}
	if !strings.Contains(failOut.String(), "[preflight] RESULT=FAIL") {
		t.Fatalf("missing fail marker: %q", failOut.String())
	}
}

func TestCompareCrossArchEvidence_MismatchReturnsError(t *testing.T) {
	root := t.TempDir()
	x86Path := filepath.Join(root, "x86.json")
	armPath := filepath.Join(root, "arm.json")

	x86 := &replay.EvidenceBundle{
		AggregateCanonical: testDigestSHA256,
		AggregateVerify:    testDigestSHA256,
		AggregateClass:     testDigestSHA256,
		AggregateExitCode:  testDigestSHA256,
	}
	arm := &replay.EvidenceBundle{
		AggregateCanonical: testDigestSHA256,
		AggregateVerify:    strings.Repeat("b", 64),
		AggregateClass:     testDigestSHA256,
		AggregateExitCode:  testDigestSHA256,
	}
	if err := replay.WriteEvidence(x86Path, x86); err != nil {
		t.Fatalf("write x86 evidence: %v", err)
	}
	if err := replay.WriteEvidence(armPath, arm); err != nil {
		t.Fatalf("write arm evidence: %v", err)
	}

	report, err := compareCrossArchEvidence(
		x86Path,
		armPath,
		filepath.Join(root, "compare.json"),
		filepath.Join(root, "compare.md"),
		root,
	)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if report == nil || report.Result != resultFail {
		t.Fatalf("expected FAIL report, got %#v", report)
	}
}

func writeHarnessMatrixAndProfile(t *testing.T, root, architecture, replayScript, suffix string) (string, string) {
	t.Helper()
	matrixPath := filepath.Join(root, "offline", "fixtures", "matrix-"+suffix+".json")
	profilePath := filepath.Join(root, "offline", "fixtures", "profile-"+suffix+".json")
	mustMkdirAll(t, filepath.Dir(matrixPath))

	matrix := replay.Matrix{
		Version:      "v1",
		Architecture: architecture,
		Nodes: []replay.NodeSpec{
			{
				ID:           "container-node",
				Mode:         replay.NodeModeContainer,
				Distro:       "debian",
				KernelFamily: "host",
				Replays:      1,
				Runner: replay.RunnerConfig{
					Kind:   "container_command",
					Replay: []string{replayScript, "local-image"},
				},
			},
			{
				ID:           "vm-node",
				Mode:         replay.NodeModeVM,
				Distro:       "ubuntu",
				KernelFamily: "ga",
				Replays:      1,
				Runner: replay.RunnerConfig{
					Kind:   "libvirt_command",
					Replay: []string{replayScript, "vm-domain", "snapshot-cold"},
				},
			},
		},
	}
	profile := replay.Profile{
		Version:          "v1",
		Name:             "maximal-offline",
		RequiredNodes:    []string{"container-node", "vm-node"},
		RequiredSuites:   []string{"canonical-byte-stability"},
		MinColdReplays:   1,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}

	encodedMatrix, err := json.MarshalIndent(matrix, "", "  ")
	if err != nil {
		t.Fatalf("marshal matrix: %v", err)
	}
	encodedProfile, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	encodedMatrix = append(encodedMatrix, '\n')
	encodedProfile = append(encodedProfile, '\n')
	mustWriteFile(t, matrixPath, encodedMatrix, 0o600)
	mustWriteFile(t, profilePath, encodedProfile, 0o600)
	return matrixPath, profilePath
}

func writeMinimalVectorFixture(t *testing.T, root string) {
	t.Helper()
	vectorDir := filepath.Join(root, "conformance", "vectors")
	mustMkdirAll(t, vectorDir)
	mustWriteFile(t, filepath.Join(vectorDir, "fixture.jsonl"), []byte("{\"id\":\"noop\"}\n"), 0o600)
}

func writeFakeGoBinary(t *testing.T, path string) {
	t.Helper()
	script := `#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "build" ]; then
  out=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "-o" ]; then
      out="$arg"
    fi
    prev="$arg"
  done
  if [ -z "$out" ]; then
    echo "missing -o" >&2
    exit 2
  fi
  mkdir -p "$(dirname "$out")"
  cat >"$out" <<'EOS'
#!/usr/bin/env sh
exit 0
EOS
  chmod +x "$out"
  exit 0
fi
if [ "${1:-}" = "test" ]; then
  exit 0
fi
exit 0
`
	writeFakeCommand(t, path, script)
}

func writeFakeReplayCommand(t *testing.T, path string) string {
	t.Helper()
	script := `#!/usr/bin/env bash
set -euo pipefail
cat >"${JCS_EVIDENCE_PATH}" <<EOF
{
  "node_id": "${JCS_NODE_ID}",
  "mode": "${JCS_NODE_MODE}",
  "distro": "${JCS_NODE_DISTRO}",
  "kernel_family": "${JCS_NODE_KERNEL_FAMILY}",
  "replay_index": ${JCS_REPLAY_INDEX},
  "session_id": "session-${JCS_NODE_ID}-${JCS_REPLAY_INDEX}",
  "started_at_utc": "2026-03-01T00:00:00Z",
  "completed_at_utc": "2026-03-01T00:00:01Z",
  "case_count": 1,
  "passed": true,
  "canonical_sha256": "` + testDigestSHA256 + `",
  "verify_sha256": "` + testDigestSHA256 + `",
  "failure_class_sha256": "` + testDigestSHA256 + `",
  "exit_code_sha256": "` + testDigestSHA256 + `"
}
EOF
`
	writeFakeCommand(t, path, script)
	return path
}

func writeFakeCommand(t *testing.T, path, script string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	mustWriteFile(t, path, []byte(script), 0o700)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, dirPerm); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func withWorkingDirectory(t *testing.T, path string, fn func()) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir to %s: %v", path, err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("restore cwd %s: %v", wd, chdirErr)
		}
	})
	fn()
}
