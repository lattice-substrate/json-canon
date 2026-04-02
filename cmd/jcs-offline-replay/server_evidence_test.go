package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestParseSSHTarget(t *testing.T) {
	t.Parallel()

	user, addr, err := parseSSHTarget("admin@203.0.113.7")
	if err != nil {
		t.Fatalf("parseSSHTarget: %v", err)
	}
	if user != "admin" {
		t.Fatalf("user mismatch: got %q want %q", user, "admin")
	}
	if addr != "203.0.113.7:22" {
		t.Fatalf("addr mismatch: got %q want %q", addr, "203.0.113.7:22")
	}
}

func TestResolveGitHeadCommitLooseGitDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitDir := filepath.Join(root, ".git-real")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o750); err != nil {
		t.Fatalf("mkdir git refs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: .git-real\n"), 0o600); err != nil {
		t.Fatalf("write .git indirection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	want := strings.Repeat("a", 40)
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	got, err := resolveGitHeadCommit(root)
	if err != nil {
		t.Fatalf("resolveGitHeadCommit: %v", err)
	}
	if got != want {
		t.Fatalf("commit mismatch: got %q want %q", got, want)
	}
}

func TestBuildRemoteReplayCommandContainerUsesSudoDocker(t *testing.T) {
	t.Parallel()

	node := testMatrixNodeContainerSSH()
	cmd, err := buildRemoteReplayCommand(node, 3, "/tmp/run", replay.EvidenceSchemaVersionV2, "1000", "1000")
	if err != nil {
		t.Fatalf("buildRemoteReplayCommand: %v", err)
	}
	for _, needle := range []string{
		"sudo docker run",
		"--image-digest",
		"/work/jcs-offline-worker",
		"--bundle '/work/bundle.tgz'",
		"--evidence '/work/out/evidence.json'",
		"--schema-version",
		replay.EvidenceSchemaVersionV2,
	} {
		if !strings.Contains(cmd, needle) {
			t.Fatalf("container command missing %q in %q", needle, cmd)
		}
	}
}

func testMatrixNodeContainerSSH() replay.NodeSpec {
	return replay.NodeSpec{
		ID:           "aws-debian13-container-x86_64",
		Mode:         replay.NodeModeContainer,
		Distro:       "debian-13",
		KernelFamily: "cloud",
		Runner: replay.RunnerConfig{
			Env: map[string]string{
				"JCS_SERVER_CONTAINER_IMAGE": "debian@sha256:" + strings.Repeat("b", 64),
			},
		},
	}
}
