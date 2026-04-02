package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const toolchainDownloadTimeout = 10 * time.Minute

var toolchainHTTPClient = &http.Client{Timeout: toolchainDownloadTimeout}

type syncedToolArtifact struct {
	Artifact       replay.ToolchainArtifact
	DownloadPath   string
	ExecutablePath string
}

func cmdSyncToolchain(flags map[string]string, stdout io.Writer) error {
	lockPath := requireFlag(flags, "--lock")
	outputDir := requireFlag(flags, "--output-dir")
	if lockPath == "" || outputDir == "" {
		return fmt.Errorf("sync-toolchain requires --lock and --output-dir")
	}
	hostArch, err := resolveToolchainHostArch(flags)
	if err != nil {
		return err
	}
	selected, err := selectedToolchainArtifacts(lockPath, hostArch, parsePurposeFlags(flags["--purposes"]))
	if err != nil {
		return err
	}
	synced, err := syncSelectedToolchainArtifacts(selected, outputDir)
	if err != nil {
		return err
	}
	envFile := strings.TrimSpace(flags["--env-file"])
	if envFile != "" {
		if err := writeToolchainEnvFile(envFile, outputDir, hostArch, synced); err != nil {
			return err
		}
	}
	for _, info := range synced {
		if err := writef(stdout, "synced %s %s\n", info.Artifact.ID, info.DownloadPath); err != nil {
			return err
		}
	}
	return nil
}

func collectToolchainEvidence(flags map[string]string, manifestOutputPath string) ([]replay.InfraManifestTool, error) {
	lockPath := requireFlag(flags, "--toolchain-lock")
	toolchainRoot := requireFlag(flags, "--toolchain-root")
	if lockPath == "" || toolchainRoot == "" {
		return nil, fmt.Errorf("write-infra-manifest requires --toolchain-lock and --toolchain-root")
	}
	hostArch, err := resolveToolchainHostArch(flags)
	if err != nil {
		return nil, err
	}
	selected, err := selectedToolchainArtifacts(lockPath, hostArch, parsePurposeFlags(flags["--purposes"]))
	if err != nil {
		return nil, err
	}
	manifestDir := filepath.Dir(manifestOutputPath)
	tools := make([]replay.InfraManifestTool, 0, len(selected))
	for _, artifact := range selected {
		tool, collectErr := collectToolEvidence(manifestDir, toolchainRoot, artifact)
		if collectErr != nil {
			return nil, collectErr
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func resolveToolchainHostArch(flags map[string]string) (string, error) {
	hostArch := requireFlag(flags, "--host-arch")
	if hostArch == "" {
		hostArch = runtime.GOARCH
	}
	hostArch = replay.NormalizeToolchainArch(hostArch)
	switch hostArch {
	case "amd64", "arm64":
		return hostArch, nil
	default:
		return "", fmt.Errorf("unsupported host arch %q", hostArch)
	}
}

func syncToolArtifact(artifact replay.ToolchainArtifact, outputDir string) (syncedToolArtifact, error) {
	downloadPath := toolchainArtifactDownloadPath(outputDir, artifact)
	if err := downloadPinnedArtifact(artifact, downloadPath); err != nil {
		return syncedToolArtifact{}, err
	}
	info := syncedToolArtifact{
		Artifact:     artifact,
		DownloadPath: downloadPath,
	}
	if artifact.ExecutablePath == "" {
		return info, nil
	}
	executablePath, err := materializeToolExecutable(artifact, downloadPath, outputDir)
	if err != nil {
		return syncedToolArtifact{}, err
	}
	info.ExecutablePath = executablePath
	return info, nil
}

func syncSelectedToolchainArtifacts(selected []replay.ToolchainArtifact, outputDir string) ([]syncedToolArtifact, error) {
	synced := make([]syncedToolArtifact, 0, len(selected))
	for _, artifact := range selected {
		info, err := syncToolArtifact(artifact, outputDir)
		if err != nil {
			return nil, err
		}
		synced = append(synced, info)
	}
	return synced, nil
}

func selectedToolchainArtifacts(lockPath, hostArch string, purposes []string) ([]replay.ToolchainArtifact, error) {
	lock, err := replay.LoadToolchainLock(lockPath)
	if err != nil {
		return nil, fmt.Errorf("load toolchain lock: %w", err)
	}
	selected, err := replay.SelectToolchainArtifactsForPurposes(lock, hostArch, purposes)
	if err != nil {
		return nil, fmt.Errorf("select toolchain artifacts: %w", err)
	}
	return selected, nil
}

func parsePurposeFlags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]struct{})
	purposes := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		purpose := strings.TrimSpace(part)
		if purpose == "" {
			continue
		}
		if _, ok := seen[purpose]; ok {
			continue
		}
		seen[purpose] = struct{}{}
		purposes = append(purposes, purpose)
	}
	sort.Strings(purposes)
	return purposes
}

func collectToolEvidence(manifestDir, toolchainRoot string, artifact replay.ToolchainArtifact) (replay.InfraManifestTool, error) {
	downloadPath := toolchainArtifactDownloadPath(toolchainRoot, artifact)
	actualSHA, err := fileSHA256(downloadPath)
	if err != nil {
		return replay.InfraManifestTool{}, fmt.Errorf("hash tool artifact %s: %w", artifact.ID, err)
	}
	if actualSHA != artifact.SHA256 {
		return replay.InfraManifestTool{}, fmt.Errorf("tool artifact %s sha256 mismatch: got=%s want=%s", artifact.ID, actualSHA, artifact.SHA256)
	}
	artifactRelPath, err := filepath.Rel(manifestDir, downloadPath)
	if err != nil {
		return replay.InfraManifestTool{}, fmt.Errorf("tool artifact %s relative path: %w", artifact.ID, err)
	}
	execRelPath, err := relativeExecutablePath(manifestDir, toolchainRoot, artifact)
	if err != nil {
		return replay.InfraManifestTool{}, err
	}
	return replay.InfraManifestTool{
		ID:                     artifact.ID,
		Scope:                  artifact.Scope,
		Purpose:                artifact.Purpose,
		Name:                   artifact.Name,
		Version:                artifact.Version,
		OS:                     artifact.OS,
		Arch:                   artifact.Arch,
		Format:                 artifact.Format,
		SourceURL:              artifact.SourceURL,
		SHA256:                 artifact.SHA256,
		ArtifactRelativePath:   filepath.ToSlash(artifactRelPath),
		ExecutableRelativePath: filepath.ToSlash(execRelPath),
	}, nil
}

func relativeExecutablePath(manifestDir, toolchainRoot string, artifact replay.ToolchainArtifact) (string, error) {
	if artifact.ExecutablePath == "" {
		return "", nil
	}
	execPath := toolchainArtifactExecutablePath(toolchainRoot, artifact)
	if _, statErr := os.Stat(execPath); statErr != nil {
		return "", fmt.Errorf("stat tool executable %s: %w", artifact.ID, statErr)
	}
	execRelPath, err := filepath.Rel(manifestDir, execPath)
	if err != nil {
		return "", fmt.Errorf("tool executable %s relative path: %w", artifact.ID, err)
	}
	return execRelPath, nil
}

func toolchainArtifactDownloadPath(root string, artifact replay.ToolchainArtifact) string {
	return filepath.Join(root, "downloads", artifact.ID, path.Base(artifact.SourceURL))
}

func toolchainArtifactExecutablePath(root string, artifact replay.ToolchainArtifact) string {
	if artifact.Format == "raw" {
		return toolchainArtifactDownloadPath(root, artifact)
	}
	return filepath.Join(root, ".extracted", artifact.ID, filepath.FromSlash(artifact.ExecutablePath))
}

func downloadPinnedArtifact(artifact replay.ToolchainArtifact, dest string) error {
	if actualSHA, err := fileSHA256(dest); err == nil && actualSHA == artifact.SHA256 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create tool artifact dir: %w", err)
	}
	tmpPath, err := downloadArtifactToTemp(artifact, filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer removeFileIgnoreError(tmpPath)
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("install tool artifact %s: %w", artifact.ID, err)
	}
	return nil
}

func downloadArtifactToTemp(artifact replay.ToolchainArtifact, dir string) (string, error) {
	tmp, err := os.CreateTemp(dir, "tool-download-*")
	if err != nil {
		return "", fmt.Errorf("create tool temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer closeIgnoreError(tmp)

	body, err := downloadArtifactBody(artifact)
	if err != nil {
		return "", err
	}
	defer closeIgnoreError(body)

	if _, copyErr := io.Copy(tmp, body); copyErr != nil {
		return "", fmt.Errorf("download tool artifact %s: %w", artifact.ID, copyErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return "", fmt.Errorf("close tool artifact %s: %w", artifact.ID, closeErr)
	}
	actualSHA, err := fileSHA256(tmpPath)
	if err != nil {
		return "", fmt.Errorf("hash tool artifact %s: %w", artifact.ID, err)
	}
	if actualSHA != artifact.SHA256 {
		return "", fmt.Errorf("tool artifact %s sha256 mismatch: got=%s want=%s", artifact.ID, actualSHA, artifact.SHA256)
	}
	return tmpPath, nil
}

func downloadArtifactBody(artifact replay.ToolchainArtifact) (io.ReadCloser, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.SourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build tool download request: %w", err)
	}
	resp, err := toolchainHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download tool artifact %s: %w", artifact.ID, err)
	}
	if resp.StatusCode != http.StatusOK {
		closeIgnoreError(resp.Body)
		return nil, fmt.Errorf("download tool artifact %s: unexpected status %s", artifact.ID, resp.Status)
	}
	return resp.Body, nil
}

func materializeToolExecutable(artifact replay.ToolchainArtifact, downloadPath, root string) (string, error) {
	if artifact.Format == "raw" {
		//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 raw pinned tools must be marked executable before use.
		if err := os.Chmod(downloadPath, 0o700); err != nil {
			return "", fmt.Errorf("chmod raw tool artifact %s: %w", artifact.ID, err)
		}
		return downloadPath, nil
	}
	extractRoot, err := resetExtractRoot(root, artifact.ID)
	if err != nil {
		return "", err
	}
	if err := extractToolArchive(artifact, downloadPath, extractRoot); err != nil {
		return "", err
	}
	executablePath := toolchainArtifactExecutablePath(root, artifact)
	if _, err := os.Stat(executablePath); err != nil {
		return "", fmt.Errorf("stat tool executable %s: %w", artifact.ID, err)
	}
	//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 verified extracted tools must be executable to serve as the trusted toolchain.
	if err := os.Chmod(executablePath, 0o700); err != nil {
		return "", fmt.Errorf("chmod tool executable %s: %w", artifact.ID, err)
	}
	return executablePath, nil
}

func resetExtractRoot(root, artifactID string) (string, error) {
	extractRoot := filepath.Join(root, ".extracted", artifactID)
	if err := os.RemoveAll(extractRoot); err != nil {
		return "", fmt.Errorf("reset extract root %s: %w", artifactID, err)
	}
	if err := os.MkdirAll(extractRoot, 0o750); err != nil {
		return "", fmt.Errorf("create extract root %s: %w", artifactID, err)
	}
	return extractRoot, nil
}

func extractToolArchive(artifact replay.ToolchainArtifact, downloadPath, extractRoot string) error {
	switch artifact.Format {
	case "tar.gz":
		return extractTarGZ(downloadPath, extractRoot)
	case "zip":
		return extractZIP(downloadPath, extractRoot)
	default:
		return fmt.Errorf("unsupported tool artifact format %q", artifact.Format)
	}
}

//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 extraction only operates on verified artifacts under the toolchain root.
func extractTarGZ(archivePath, destRoot string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tool archive: %w", err)
	}
	defer closeIgnoreError(f)
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip tool archive: %w", err)
	}
	defer closeIgnoreError(gzr)
	tr := tar.NewReader(gzr)
	for {
		header, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return nil
		}
		if nextErr != nil {
			return fmt.Errorf("read tar tool archive: %w", nextErr)
		}
		if err := extractTarEntry(destRoot, header, tr); err != nil {
			return err
		}
	}
}

func extractZIP(archivePath, destRoot string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip tool archive: %w", err)
	}
	defer closeIgnoreError(zr)
	for _, file := range zr.File {
		if err := extractZIPEntry(destRoot, file); err != nil {
			return err
		}
	}
	return nil
}

func extractTarEntry(destRoot string, header *tar.Header, reader io.Reader) error {
	targetPath, err := safeArchivePath(destRoot, header.Name)
	if err != nil {
		return err
	}
	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(targetPath, 0o750); err != nil {
			return fmt.Errorf("create tar dir: %w", err)
		}
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
			return fmt.Errorf("create tar parent dir: %w", err)
		}
		//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 extracted file paths are sanitized under the toolchain root.
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, normalizedArchiveFileMode(header.FileInfo().Mode()))
		if err != nil {
			return fmt.Errorf("create tar file: %w", err)
		}
		copyErr := copyExact(out, reader, header.Size)
		closeErr := out.Close()
		if copyErr != nil {
			return fmt.Errorf("write tar file: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close tar file: %w", closeErr)
		}
	}
	return nil
}

func extractZIPEntry(destRoot string, file *zip.File) error {
	targetPath, err := safeArchivePath(destRoot, file.Name)
	if err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		if mkdirErr := os.MkdirAll(targetPath, 0o750); mkdirErr != nil {
			return fmt.Errorf("create zip dir: %w", mkdirErr)
		}
		return nil
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(targetPath), 0o750); mkdirErr != nil {
		return fmt.Errorf("create zip parent dir: %w", mkdirErr)
	}
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip file: %w", err)
	}
	defer closeIgnoreError(rc)
	//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 extracted file paths are sanitized under the toolchain root.
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, normalizedArchiveFileMode(file.Mode()))
	if err != nil {
		return fmt.Errorf("create zip file: %w", err)
	}
	uncompressedSize, err := zipEntrySize(file)
	if err != nil {
		closeIgnoreError(out)
		return err
	}
	copyErr := copyExact(out, rc, uncompressedSize)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("write zip file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close zip file: %w", closeErr)
	}
	return nil
}

func normalizedArchiveFileMode(mode fs.FileMode) fs.FileMode {
	if mode.Perm()&0o111 != 0 {
		return 0o700
	}
	return 0o600
}

func safeArchivePath(root, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." {
		return root, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("tool archive entry %q escapes extraction root", name)
	}
	return filepath.Join(root, clean), nil
}

func copyExact(dst io.Writer, src io.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("negative archive entry size %d", size)
	}
	written, err := io.CopyN(dst, src, size)
	if err != nil {
		return fmt.Errorf("copy exact %d bytes: %w", size, err)
	}
	if written != size {
		return fmt.Errorf("copied %d bytes, want %d", written, size)
	}
	return nil
}

func zipEntrySize(file *zip.File) (int64, error) {
	if file.UncompressedSize64 > math.MaxInt64 {
		return 0, fmt.Errorf("zip entry %q too large to extract safely", file.Name)
	}
	size, err := strconv.ParseInt(strconv.FormatUint(file.UncompressedSize64, 10), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse zip entry %q size: %w", file.Name, err)
	}
	return size, nil
}

func writeToolchainEnvFile(path, outputDir, hostArch string, synced []syncedToolArtifact) error {
	lines := []string{
		"export JCS_TOOLCHAIN_ROOT=" + shellQuote(outputDir),
		"export JCS_TOOLCHAIN_HOST_ARCH=" + shellQuote(hostArch),
	}
	sort.Slice(synced, func(i, j int) bool {
		return synced[i].Artifact.ID < synced[j].Artifact.ID
	})
	for _, info := range synced {
		name, value, err := toolchainEnvVar(info)
		if err != nil {
			return err
		}
		lines = append(lines, "export "+name+"="+shellQuote(value))
	}
	data := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return fmt.Errorf("write toolchain env file: %w", err)
	}
	return nil
}

func toolchainEnvVar(info syncedToolArtifact) (string, string, error) {
	switch info.Artifact.ID {
	case "go-linux-amd64", "go-linux-arm64":
		return "JCS_TOOL_GO", info.ExecutablePath, nil
	case "tofu-linux-amd64", "tofu-linux-arm64":
		return "JCS_TOOL_TOFU", info.ExecutablePath, nil
	case "jq-linux-amd64", "jq-linux-arm64":
		return "JCS_TOOL_JQ", info.ExecutablePath, nil
	case "docker-static-linux-amd64":
		return "JCS_TOOL_DOCKER_STATIC_AMD64", info.DownloadPath, nil
	case "docker-static-linux-arm64":
		return "JCS_TOOL_DOCKER_STATIC_ARM64", info.DownloadPath, nil
	default:
		return "", "", fmt.Errorf("unmapped toolchain artifact id %q", info.Artifact.ID)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func closeIgnoreError(c io.Closer) {
	if err := c.Close(); err != nil {
		_ = err
	}
}

func removeFileIgnoreError(path string) {
	if err := os.Remove(path); err != nil {
		_ = err
	}
}
