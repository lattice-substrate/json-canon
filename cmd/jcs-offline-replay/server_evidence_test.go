package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

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

func TestBuildRemoteReplaySSMCommandNativeVM(t *testing.T) {
	t.Parallel()

	node := replay.NodeSpec{
		ID:           "aws-native-debian13-amd-x86_64",
		Mode:         replay.NodeModeVM,
		Distro:       "debian-13",
		KernelFamily: "cloud-amd",
	}
	cmd, err := buildRemoteReplaySSMCommand(
		node,
		3,
		"https://example.com/bundle",
		"https://example.com/worker",
		"https://example.com/evidence",
	)
	if err != nil {
		t.Fatalf("buildRemoteReplaySSMCommand: %v", err)
	}
	for _, needle := range []string{
		`curl -fsSL 'https://example.com/bundle' -o "$tmp/bundle.tgz"`,
		`curl -fsSL 'https://example.com/worker' -o "$tmp/jcs-offline-worker"`,
		`--bundle "$tmp/bundle.tgz"`,
		`--evidence "$tmp/evidence.json"`,
		"--schema-version",
		replay.EvidenceSchemaVersionV3,
		`curl -fsS -X PUT -T "$tmp/evidence.json" 'https://example.com/evidence'`,
	} {
		if !strings.Contains(cmd, needle) {
			t.Fatalf("native vm command missing %q in %q", needle, cmd)
		}
	}
}

func TestParseEvidenceSHA256(t *testing.T) {
	t.Parallel()

	got := parseEvidenceSHA256("noise\nevidence_sha256=abc123\nmore-noise\n")
	if got != "abc123" {
		t.Fatalf("sha mismatch: got %q want %q", got, "abc123")
	}
}
