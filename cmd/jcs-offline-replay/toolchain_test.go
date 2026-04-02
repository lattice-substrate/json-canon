package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestCmdSyncToolchainAndCollectEvidence(t *testing.T) {
	fixtures := buildToolchainFixtures(t)
	server := newToolchainFixtureServer(t, fixtures)
	defer server.Close()
	oldClient := toolchainHTTPClient
	toolchainHTTPClient = server.Client()
	defer func() {
		toolchainHTTPClient = oldClient
	}()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "toolchain.lock.tsv")
	if err := os.WriteFile(lockPath, []byte(toolchainLockFixture(server.URL, fixtures)), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	toolchainRoot := filepath.Join(dir, "toolchain")
	envPath := filepath.Join(dir, "toolchain.env")
	runToolchainSync(t, lockPath, toolchainRoot, envPath)
	assertExecutableMode(t, filepath.Join(toolchainRoot, "extracted", "go-linux-amd64", "go", "bin", "go"))
	assertExecutableMode(t, filepath.Join(toolchainRoot, "extracted", "go-linux-amd64", "go", "pkg", "tool", "linux_amd64", "compile"))
	tools, err := collectToolchainEvidenceForTest(lockPath, toolchainRoot, filepath.Join(dir, "release", "infra-manifest.v1.json"))
	if err != nil {
		t.Fatalf("collectToolchainEvidence: %v", err)
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	if tools[0].ArtifactRelativePath == "" {
		t.Fatal("expected relative artifact path")
	}
}

func buildTarGZFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, contents := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(contents)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := io.WriteString(tw, contents); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func buildZIPFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, contents := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		})
		if err != nil {
			t.Fatalf("create zip header: %v", err)
		}
		if _, err := io.WriteString(w, contents); err != nil {
			t.Fatalf("write zip body: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func buildToolchainFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"/go-linux-amd64.tar.gz": buildTarGZFixture(t, map[string]string{
			"go/bin/go":                       "#!/bin/sh\n",
			"go/pkg/tool/linux_amd64/compile": "#!/bin/sh\n",
		}),
		"/tofu-linux-amd64.zip": buildZIPFixture(t, map[string]string{
			"tofu": "#!/bin/sh\n",
		}),
		"/jq-linux-amd64": []byte("#!/bin/sh\n"),
		"/docker-linux-amd64.tgz": buildTarGZFixture(t, map[string]string{
			"docker/docker": "#!/bin/sh\n",
		}),
		"/docker-linux-arm64.tgz": buildTarGZFixture(t, map[string]string{
			"docker/docker": "#!/bin/sh\n",
		}),
	}
}

func newToolchainFixtureServer(t *testing.T, fixtures map[string][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, ok := fixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("write fixture %s: %v", r.URL.Path, err)
		}
	}))
}

func toolchainLockFixture(serverURL string, fixtures map[string][]byte) string {
	return strings.Join([]string{
		"# schema_version=toolchain-lock.v1",
		"id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path",
		"go-linux-amd64\thost\tbuild\tgo\t1.24.13\tlinux\tamd64\ttar.gz\t" + serverURL + "/go-linux-amd64.tar.gz\t" + sha256Hex(fixtures["/go-linux-amd64.tar.gz"]) + "\tgo/bin/go",
		"tofu-linux-amd64\thost\tprovision\topentofu\t1.10.6\tlinux\tamd64\tzip\t" + serverURL + "/tofu-linux-amd64.zip\t" + sha256Hex(fixtures["/tofu-linux-amd64.zip"]) + "\ttofu",
		"jq-linux-amd64\thost\tworkflow-json-query\tjq\t1.8.1\tlinux\tamd64\traw\t" + serverURL + "/jq-linux-amd64\t" + sha256Hex(fixtures["/jq-linux-amd64"]) + "\tjq-linux-amd64",
		"docker-static-linux-amd64\tremote\tcontainer-runtime\tdocker\t29.3.1\tlinux\tamd64\ttar.gz\t" + serverURL + "/docker-linux-amd64.tgz\t" + sha256Hex(fixtures["/docker-linux-amd64.tgz"]) + "\t",
		"docker-static-linux-arm64\tremote\tcontainer-runtime\tdocker\t29.3.1\tlinux\tarm64\ttar.gz\t" + serverURL + "/docker-linux-arm64.tgz\t" + sha256Hex(fixtures["/docker-linux-arm64.tgz"]) + "\t",
		"",
	}, "\n")
}

func runToolchainSync(t *testing.T, lockPath, toolchainRoot, envPath string) {
	t.Helper()
	var out bytes.Buffer
	if err := cmdSyncToolchain(map[string]string{
		"--lock":       lockPath,
		"--output-dir": toolchainRoot,
		"--env-file":   envPath,
		"--host-arch":  "amd64",
	}, &out); err != nil {
		t.Fatalf("cmdSyncToolchain: %v", err)
	}
	if !strings.Contains(out.String(), "synced go-linux-amd64") {
		t.Fatalf("unexpected sync output: %q", out.String())
	}
	//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 test reads a temp env file it just wrote.
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	for _, needle := range []string{"JCS_TOOL_GO", "JCS_TOOL_TOFU", "JCS_TOOL_JQ", "JCS_TOOL_DOCKER_STATIC_AMD64", "JCS_TOOL_DOCKER_STATIC_ARM64"} {
		if !strings.Contains(string(envData), needle) {
			t.Fatalf("env file missing %s: %s", needle, string(envData))
		}
	}
}

func collectToolchainEvidenceForTest(lockPath, toolchainRoot, manifestPath string) ([]replay.InfraManifestTool, error) {
	if mkdirErr := os.MkdirAll(filepath.Dir(manifestPath), 0o750); mkdirErr != nil {
		return nil, fmt.Errorf("mkdir manifest dir: %w", mkdirErr)
	}
	return collectToolchainEvidence(map[string]string{
		"--toolchain-lock": lockPath,
		"--toolchain-root": toolchainRoot,
		"--host-arch":      "amd64",
	}, manifestPath)
}

func assertExecutableMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("expected owner-executable mode for %s, got %o", path, info.Mode().Perm())
	}
}
