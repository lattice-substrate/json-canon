package replay_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func validToolchainLockFixture() string {
	return strings.Join([]string{
		"# schema_version=toolchain-lock.v1",
		"id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path",
		"go-linux-amd64\thost\tbuild\tgo\t1.24.13\tlinux\tamd64\ttar.gz\thttps://go.dev/dl/go1.24.13.linux-amd64.tar.gz\t" + strings.Repeat("a", 64) + "\tgo/bin/go",
		"go-linux-arm64\thost\tbuild\tgo\t1.24.13\tlinux\tarm64\ttar.gz\thttps://go.dev/dl/go1.24.13.linux-arm64.tar.gz\t" + strings.Repeat("b", 64) + "\tgo/bin/go",
		"docker-static-linux-amd64\tremote\tcontainer-runtime\tdocker\t29.3.1\tlinux\tamd64\ttar.gz\thttps://download.docker.com/linux/static/stable/x86_64/docker-29.3.1.tgz\t" + strings.Repeat("c", 64) + "\t",
		"docker-static-linux-arm64\tremote\tcontainer-runtime\tdocker\t29.3.1\tlinux\tarm64\ttar.gz\thttps://download.docker.com/linux/static/stable/aarch64/docker-29.3.1.tgz\t" + strings.Repeat("d", 64) + "\t",
		"",
	}, "\n")
}

func TestLoadToolchainLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolchain.lock.tsv")
	if err := os.WriteFile(path, []byte(validToolchainLockFixture()), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	lock, err := replay.LoadToolchainLock(path)
	if err != nil {
		t.Fatalf("load toolchain lock: %v", err)
	}
	if lock.SchemaVersion != replay.ToolchainLockSchemaVersion {
		t.Fatalf("schema_version mismatch: %q", lock.SchemaVersion)
	}
	if len(lock.Artifacts) != 4 {
		t.Fatalf("expected 4 artifacts, got %d", len(lock.Artifacts))
	}
}

func TestLoadToolchainLockRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "missing schema",
			raw: strings.Join([]string{
				"id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path",
			}, "\n"),
			want: "schema_version",
		},
		{
			name: "bad header",
			raw: strings.Join([]string{
				"# schema_version=toolchain-lock.v1",
				"bad\theader",
			}, "\n"),
			want: "invalid header",
		},
		{
			name: "host missing executable path",
			raw: strings.Join([]string{
				"# schema_version=toolchain-lock.v1",
				"id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path",
				"go-linux-amd64\thost\tbuild\tgo\t1.24.13\tlinux\tamd64\ttar.gz\thttps://go.dev/dl/go1.24.13.linux-amd64.tar.gz\t" + strings.Repeat("a", 64) + "\t",
			}, "\n"),
			want: "executable_path is required",
		},
		{
			name: "duplicate id",
			raw: strings.Join([]string{
				"# schema_version=toolchain-lock.v1",
				"id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path",
				"go-linux-amd64\thost\tbuild\tgo\t1.24.13\tlinux\tamd64\ttar.gz\thttps://go.dev/dl/go1.24.13.linux-amd64.tar.gz\t" + strings.Repeat("a", 64) + "\tgo/bin/go",
				"go-linux-amd64\tremote\tcontainer-runtime\tdocker\t29.3.1\tlinux\tamd64\ttar.gz\thttps://download.docker.com/linux/static/stable/x86_64/docker-29.3.1.tgz\t" + strings.Repeat("b", 64) + "\t",
			}, "\n"),
			want: "duplicate artifact id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "toolchain.lock.tsv")
			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatalf("write lock: %v", err)
			}
			_, err := replay.LoadToolchainLock(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSelectToolchainArtifacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolchain.lock.tsv")
	if err := os.WriteFile(path, []byte(validToolchainLockFixture()), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	lock, err := replay.LoadToolchainLock(path)
	if err != nil {
		t.Fatalf("load toolchain lock: %v", err)
	}
	selected, err := replay.SelectToolchainArtifacts(lock, "x86_64")
	if err != nil {
		t.Fatalf("select toolchain artifacts: %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected artifacts, got %d", len(selected))
	}
	for _, artifact := range selected {
		if artifact.Scope == replay.ToolchainScopeHost && artifact.Arch != "amd64" {
			t.Fatalf("unexpected host artifact arch: %#v", artifact)
		}
	}
}
