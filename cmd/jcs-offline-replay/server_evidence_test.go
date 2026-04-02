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

func TestBuildRemoteReplayCommandNativeVM(t *testing.T) {
	t.Parallel()

	node := replay.NodeSpec{
		ID:           "aws-native-debian13-amd-x86_64",
		Mode:         replay.NodeModeVM,
		Distro:       "debian-13",
		KernelFamily: "cloud-amd",
	}
	cmd, err := buildRemoteReplayCommand(node, 3, "/tmp/run", replay.EvidenceSchemaVersionV2)
	if err != nil {
		t.Fatalf("buildRemoteReplayCommand: %v", err)
	}
	for _, needle := range []string{
		"/tmp/run/jcs-offline-worker",
		"--bundle '/tmp/run/bundle.tgz'",
		"--evidence '/tmp/run/evidence.json'",
		"--schema-version",
		replay.EvidenceSchemaVersionV2,
	} {
		if !strings.Contains(cmd, needle) {
			t.Fatalf("native vm command missing %q in %q", needle, cmd)
		}
	}
}

func TestServerSSHTargetEnvKey(t *testing.T) {
	t.Parallel()

	got := serverSSHTargetEnvKey("aws-native-ubuntu2404-minimal-arm64")
	want := "JCS_SERVER_SSH_TARGET_AWS_NATIVE_UBUNTU2404_MINIMAL_ARM64"
	if got != want {
		t.Fatalf("env key mismatch: got %q want %q", got, want)
	}
}
