// Package replay implements offline replay orchestration contracts and artifacts.
package replay

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const bundleManifestPath = "manifest.json"

// BundleOptions configures offline replay bundle creation.
type BundleOptions struct {
	OutputPath  string
	BinaryPath  string
	WorkerPath  string
	MatrixPath  string
	ProfilePath string
	VectorsGlob string
	Version     string
}

// BundleManifest tracks checksums for immutable replay inputs.
type BundleManifest struct {
	Version         string            `json:"version"`
	CreatedAtUTC    string            `json:"created_at_utc"`
	BinaryPath      string            `json:"binary_path"`
	BinarySHA256    string            `json:"binary_sha256"`
	WorkerPath      string            `json:"worker_path,omitempty"`
	WorkerSHA256    string            `json:"worker_sha256,omitempty"`
	MatrixPath      string            `json:"matrix_path"`
	MatrixSHA256    string            `json:"matrix_sha256"`
	ProfilePath     string            `json:"profile_path"`
	ProfileSHA256   string            `json:"profile_sha256"`
	VectorFiles     []string          `json:"vector_files"`
	VectorSHA256    map[string]string `json:"vector_sha256"`
	VectorSetSHA256 string            `json:"vector_set_sha256"`
}

type bundleEntry struct {
	path string
	data []byte
	mode int64
}

// CreateBundle packages immutable replay inputs and their digests into a bundle.
//
//nolint:gocyclo,cyclop,funlen,gosec // REQ:OFFLINE-EVIDENCE-001 bundle assembly is explicit; file paths are validated operator inputs.
func CreateBundle(opts BundleOptions) (*BundleManifest, error) {
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("bundle output path is required")
	}
	if opts.BinaryPath == "" || opts.WorkerPath == "" || opts.MatrixPath == "" || opts.ProfilePath == "" {
		return nil, fmt.Errorf("binary, worker, matrix, and profile paths are required")
	}
	if opts.VectorsGlob == "" {
		opts.VectorsGlob = filepath.Join("conformance", "vectors", "*.jsonl")
	}
	if opts.Version == "" {
		opts.Version = "bundle.v1"
	}

	binaryBytes, err := os.ReadFile(opts.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("read binary: %w", err)
	}
	workerBytes, err := os.ReadFile(opts.WorkerPath)
	if err != nil {
		return nil, fmt.Errorf("read worker: %w", err)
	}
	matrixBytes, err := os.ReadFile(opts.MatrixPath)
	if err != nil {
		return nil, fmt.Errorf("read matrix: %w", err)
	}
	profileBytes, err := os.ReadFile(opts.ProfilePath)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	vectorFiles, err := filepath.Glob(opts.VectorsGlob)
	if err != nil {
		return nil, fmt.Errorf("glob vectors: %w", err)
	}
	if len(vectorFiles) == 0 {
		return nil, fmt.Errorf("no vector files matched %q", opts.VectorsGlob)
	}
	sort.Strings(vectorFiles)

	manifest := &BundleManifest{
		Version:       opts.Version,
		CreatedAtUTC:  wallClockNowUTC().Format(time.RFC3339Nano),
		BinaryPath:    "bundle/jcs-canon",
		BinarySHA256:  sha256Hex(binaryBytes),
		WorkerPath:    "bundle/jcs-offline-worker",
		WorkerSHA256:  sha256Hex(workerBytes),
		MatrixPath:    "bundle/matrix.yaml",
		MatrixSHA256:  sha256Hex(matrixBytes),
		ProfilePath:   "bundle/profile.yaml",
		ProfileSHA256: sha256Hex(profileBytes),
		VectorSHA256:  make(map[string]string, len(vectorFiles)),
	}

	entries := []bundleEntry{
		{path: manifest.BinaryPath, data: binaryBytes, mode: 0o755},
		{path: manifest.WorkerPath, data: workerBytes, mode: 0o755},
		{path: manifest.MatrixPath, data: matrixBytes, mode: 0o644},
		{path: manifest.ProfilePath, data: profileBytes, mode: 0o644},
	}
	vectorDigestInput := make([]string, 0, len(vectorFiles))
	for _, vectorPath := range vectorFiles {
		vectorData, readErr := os.ReadFile(vectorPath)
		if readErr != nil {
			return nil, fmt.Errorf("read vector %s: %w", vectorPath, readErr)
		}
		base := filepath.ToSlash(filepath.Base(vectorPath))
		rel := "bundle/vectors/" + base
		manifest.VectorFiles = append(manifest.VectorFiles, rel)
		manifest.VectorSHA256[rel] = sha256Hex(vectorData)
		vectorDigestInput = append(vectorDigestInput, rel+":"+manifest.VectorSHA256[rel])
		entries = append(entries, bundleEntry{path: rel, data: vectorData, mode: 0o644})
	}
	sort.Strings(manifest.VectorFiles)
	sort.Strings(vectorDigestInput)
	manifest.VectorSetSHA256 = sha256Hex([]byte(strings.Join(vectorDigestInput, "\n")))

	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal bundle manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	entries = append(entries, bundleEntry{path: bundleManifestPath, data: manifestJSON, mode: 0o644})

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	if err := writeBundleTarGz(opts.OutputPath, entries); err != nil {
		return nil, err
	}
	return manifest, nil
}

// ReadBundleManifest loads the embedded bundle manifest from a bundle archive.
//
//nolint:gocyclo,cyclop,gosec // REQ:OFFLINE-EVIDENCE-001 tar stream traversal stays explicit for audit/error attribution.
func ReadBundleManifest(bundlePath string) (*BundleManifest, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open bundle gzip stream: %w", err)
	}
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read bundle tar: %w", err)
		}
		if hdr.Name != bundleManifestPath {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read bundle manifest entry: %w", err)
		}
		var manifest BundleManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("decode bundle manifest: %w", err)
		}
		return &manifest, nil
	}
	return nil, fmt.Errorf("bundle manifest entry %q not found", bundleManifestPath)
}

// VerifyBundle validates required manifest fields and returns the bundle digest.
//
//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 bundle verification reads operator-provided bundle artifacts.
func VerifyBundle(bundlePath string) (*BundleManifest, string, error) {
	manifest, err := ReadBundleManifest(bundlePath)
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, "", fmt.Errorf("read bundle for sha256: %w", err)
	}
	bundleSHA := sha256Hex(b)
	if manifest.BinarySHA256 == "" || manifest.WorkerSHA256 == "" || manifest.MatrixSHA256 == "" || manifest.ProfileSHA256 == "" {
		return nil, "", fmt.Errorf("bundle manifest missing required checksums")
	}
	if len(manifest.VectorFiles) == 0 {
		return nil, "", fmt.Errorf("bundle manifest has no vector files")
	}
	for _, path := range manifest.VectorFiles {
		if manifest.VectorSHA256[path] == "" {
			return nil, "", fmt.Errorf("bundle manifest missing vector digest for %s", path)
		}
	}
	return manifest, bundleSHA, nil
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 bundle writer persists deterministic artifacts to configured output path.
func writeBundleTarGz(path string, entries []bundleEntry) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	gz := gzip.NewWriter(out)
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	tw := tar.NewWriter(gz)
	defer func() {
		if closeErr := tw.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	fixed := time.Unix(0, 0).UTC()
	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.path,
			Mode:    e.mode,
			Size:    int64(len(e.data)),
			ModTime: fixed,
			Uid:     0,
			Gid:     0,
			Uname:   "root",
			Gname:   "root",
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write tar header for %s: %w", e.path, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			return fmt.Errorf("write tar entry %s: %w", e.path, err)
		}
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

//nolint:forbidigo // REQ:OFFLINE-EVIDENCE-001 bundle creation records an actual creation timestamp in manifest metadata.
func wallClockNowUTC() time.Time {
	return time.Now().UTC()
}
