package conformance_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/SolutionsExcite/json-canon/jcsfloat"
)

const (
	officialES6Checksum10K  = "b9f7a8e75ef22a835685a52ccba7f7d6bdc99e34b010992cbc5864cd12be6892"
	officialES6Checksum100M = "0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272"
)

type officialES6Target struct {
	lines int
	sum   string
}

type officialUpstreamManifest struct {
	SourceRepository string            `json:"source_repository"`
	SourceCommit     string            `json:"source_commit"`
	RetrievedUTC     string            `json:"retrieved_utc"`
	FilesSHA256      map[string]string `json:"files_sha256"`
}

func TestOfficialCyberphoneCanonicalPairs(t *testing.T) {
	h := testHarness(t)
	checkOfficialCyberphoneVectors(t, h)
}

func TestOfficialCyberphoneFixtureProvenance(t *testing.T) {
	h := testHarness(t)
	manifestPath := filepath.Join(h.root, "conformance", "official", "cyberphone", "UPSTREAM.json")
	data := mustReadBinaryFile(t, manifestPath)
	var manifest officialUpstreamManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode %s: %v", manifestPath, err)
	}
	if manifest.SourceRepository != "https://github.com/cyberphone/json-canonicalization" {
		t.Fatalf("unexpected source repository: %q", manifest.SourceRepository)
	}
	if ok, err := regexp.MatchString(`^[0-9a-f]{40}$`, manifest.SourceCommit); err != nil || !ok {
		t.Fatalf("invalid source_commit %q", manifest.SourceCommit)
	}
	if strings.TrimSpace(manifest.RetrievedUTC) == "" {
		t.Fatal("retrieved_utc must not be empty")
	}
	if len(manifest.FilesSHA256) == 0 {
		t.Fatal("files_sha256 must not be empty")
	}

	for rel, want := range manifest.FilesSHA256 {
		if strings.TrimSpace(rel) == "" {
			t.Fatal("manifest contains empty relative path")
		}
		if strings.TrimSpace(want) == "" {
			t.Fatalf("manifest contains empty checksum for %s", rel)
		}
		path := filepath.Join(h.root, "conformance", "official", "cyberphone", filepath.FromSlash(rel))
		buf := mustReadBinaryFile(t, path)
		sum := sha256.Sum256(buf)
		got := fmt.Sprintf("%x", sum[:])
		if got != want {
			t.Fatalf("fixture checksum mismatch for %s: got=%s want=%s", rel, got, want)
		}
	}
}

func TestOfficialRFC8785Vectors(t *testing.T) {
	h := testHarness(t)
	checkOfficialRFC8785Vectors(t, h)
}

func TestOfficialES6CorpusChecksums10K(t *testing.T) {
	verifyOfficialES6Checksums(t, []officialES6Target{{lines: 10_000, sum: officialES6Checksum10K}})
}

func TestOfficialES6CorpusChecksums100M(t *testing.T) {
	if lookupEnvTrimmed("JCS_OFFICIAL_ES6_ENABLE_100M") != "1" {
		t.Skip("set JCS_OFFICIAL_ES6_ENABLE_100M=1 to run 100M official ES6 checksum gate")
	}
	verifyOfficialES6Checksums(t, []officialES6Target{{lines: 100_000_000, sum: officialES6Checksum100M}})
}

func TestOfficialES6100MReleaseGatePolicy(t *testing.T) {
	h := testHarness(t)
	checkOfficialES6100MReleaseGatePolicy(t, h)
}

func checkOfficialCyberphoneVectors(t *testing.T, h *harness) {
	t.Helper()
	inDir := filepath.Join(h.root, "conformance", "official", "cyberphone", "input")
	entries, err := os.ReadDir(inDir)
	if err != nil {
		t.Fatalf("read cyberphone input fixtures: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no cyberphone input fixtures found")
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			input := mustReadBinaryFile(t, filepath.Join(h.root, "conformance", "official", "cyberphone", "input", name))
			expected := strings.TrimSpace(string(mustReadBinaryFile(t, filepath.Join(h.root, "conformance", "official", "cyberphone", "output", name))))
			hexPath := filepath.Join(h.root, "conformance", "official", "cyberphone", "outhex", strings.TrimSuffix(name, ".json")+".txt")
			expectedHexBytes := decodeHexByteDump(t, string(mustReadBinaryFile(t, hexPath)))

			res := runCLI(t, h, []string{"canonicalize", "-"}, input)
			if res.exitCode != 0 {
				t.Fatalf("canonicalize failed for %s: %+v", name, res)
			}
			if res.stdout != expected {
				t.Fatalf("canonical output mismatch for %s:\n got=%q\nwant=%q", name, res.stdout, expected)
			}
			if res.stdout != string(expectedHexBytes) {
				t.Fatalf("canonical bytes mismatch for %s against outhex fixture", name)
			}
		})
	}
}

func checkOfficialRFC8785Vectors(t *testing.T, h *harness) {
	t.Helper()
	inputPath := filepath.Join(h.root, "conformance", "official", "rfc8785", "key_sorting_input.json")
	input := mustReadBinaryFile(t, inputPath)
	expected := "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"ö\":\"Latin Small Letter O With Diaeresis\",\"€\":\"Euro Sign\",\"😀\":\"Emoji: Grinning Face\",\"דּ\":\"Hebrew Letter Dalet With Dagesh\"}"
	res := runCLI(t, h, []string{"canonicalize", "-"}, input)
	if res.exitCode != 0 {
		t.Fatalf("canonicalize key_sorting_input.json failed: %+v", res)
	}
	if res.stdout != expected {
		t.Fatalf("RFC 8785 key sorting mismatch:\n got=%q\nwant=%q", res.stdout, expected)
	}

	appendixPath := filepath.Join(h.root, "conformance", "official", "rfc8785", "appendix_b.csv")
	verifyRFC8785AppendixB(t, appendixPath)
}

func checkOfficialES6Corpus10K(t *testing.T, _ *harness) {
	t.Helper()
	verifyOfficialES6Checksums(t, []officialES6Target{{lines: 10_000, sum: officialES6Checksum10K}})
}

func checkOfficialES6100MReleaseGatePolicy(t *testing.T, h *harness) {
	t.Helper()
	releaseWorkflow := mustReadText(t, filepath.Join(h.root, ".github", "workflows", "release.yml"))
	assertContains(t, releaseWorkflow, "official ES6 100M checksum gate", "release workflow official 100M gate step")
	assertContains(t, releaseWorkflow, "JCS_OFFICIAL_ES6_ENABLE_100M", "release workflow official 100M gate env")
	assertContains(t, releaseWorkflow, "TestOfficialES6CorpusChecksums100M", "release workflow official 100M gate invocation")

	releaseDoc := mustReadText(t, filepath.Join(h.root, "RELEASE_PROCESS.md"))
	assertContains(t, releaseDoc, "JCS_OFFICIAL_ES6_ENABLE_100M=1", "release process 100M command")
	assertContains(t, releaseDoc, "TestOfficialES6CorpusChecksums100M", "release process 100M test name")
}

func verifyRFC8785AppendixB(t *testing.T, path string) {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(string(mustReadBinaryFile(t, path))))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	rows := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if lineNo == 1 {
			continue
		}
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 2 {
			t.Fatalf("appendix fixture line %d malformed: %q", lineNo, line)
		}
		bits, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 64)
		if err != nil {
			t.Fatalf("appendix fixture line %d bits parse: %v", lineNo, err)
		}
		want := strings.TrimSpace(parts[1])
		got, fmtErr := jcsfloat.FormatDouble(math.Float64frombits(bits))
		if fmtErr != nil {
			t.Fatalf("appendix fixture line %d unexpected format error: %v", lineNo, fmtErr)
		}
		if got != want {
			t.Fatalf("appendix fixture line %d bits=%016x got=%q want=%q", lineNo, bits, got, want)
		}
		rows++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan appendix fixture: %v", err)
	}
	if rows == 0 {
		t.Fatal("no RFC 8785 appendix rows validated")
	}
}

func verifyOfficialES6Checksums(t *testing.T, targets []officialES6Target) {
	t.Helper()
	if len(targets) == 0 {
		t.Fatal("at least one ES6 checksum target is required")
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].lines < targets[j].lines
	})
	for i := range targets {
		if targets[i].lines < 1 {
			t.Fatalf("invalid target lines: %d", targets[i].lines)
		}
	}

	next := newOfficialES6Generator()
	h := sha256.New()
	line := make([]byte, 0, 96)
	targetIdx := 0
	maxLines := targets[len(targets)-1].lines

	for i := 1; i <= maxLines; i++ {
		f := next()
		formatted, fmtErr := jcsfloat.FormatDouble(f)
		if fmtErr != nil {
			t.Fatalf("line %d unexpected format error: %v", i, fmtErr)
		}
		line = strconv.AppendUint(line[:0], math.Float64bits(f), 16)
		line = append(line, ',')
		line = append(line, formatted...)
		line = append(line, '\n')
		if _, err := h.Write(line); err != nil {
			t.Fatalf("line %d checksum write failed: %v", i, err)
		}
		for targetIdx < len(targets) && i == targets[targetIdx].lines {
			got := fmt.Sprintf("%x", h.Sum(nil))
			want := strings.ToLower(strings.TrimSpace(targets[targetIdx].sum))
			if got != want {
				t.Fatalf("ES6 checksum mismatch lines=%d got=%s want=%s", targets[targetIdx].lines, got, want)
			}
			targetIdx++
		}
	}

	if targetIdx != len(targets) {
		t.Fatalf("unverified targets: verified=%d total=%d", targetIdx, len(targets))
	}
}

func decodeHexByteDump(t *testing.T, text string) []byte {
	t.Helper()
	fields := strings.Fields(text)
	buf := make([]byte, 0, len(fields))
	for _, field := range fields {
		if len(field) != 2 {
			t.Fatalf("invalid hex byte token %q", field)
		}
		v, err := strconv.ParseUint(field, 16, 8)
		if err != nil {
			t.Fatalf("invalid hex byte token %q: %v", field, err)
		}
		buf = append(buf, byte(v))
	}
	if len(buf) == 0 {
		t.Fatal("hex byte dump decoded to empty payload")
	}
	return buf
}

//nolint:gosec // REQ:OFFICIAL-VEC-001 official fixture loader reads repository-controlled fixture paths.
func mustReadBinaryFile(t *testing.T, path string) []byte {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return buf
}

func lookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

type officialES6Generator struct {
	idx   int
	data  []byte
	block [sha256.Size]byte
}

func newOfficialES6Generator() func() float64 {
	g := &officialES6Generator{}
	return g.next
}

func (g *officialES6Generator) next() float64 {
	const serialCount = 2000
	var f float64
	switch {
	case g.idx < len(officialES6StaticU64s):
		f = math.Float64frombits(officialES6StaticU64s[g.idx])
	case g.idx < len(officialES6StaticU64s)+serialCount:
		//nolint:gosec // REQ:OFFICIAL-VEC-003 index is bounded by deterministic test loop size.
		f = math.Float64frombits(0x0010000000000000 + uint64(g.idx-len(officialES6StaticU64s)))
	default:
		for f == 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			if len(g.data) == 0 {
				g.block = sha256.Sum256(g.block[:])
				g.data = g.block[:]
			}
			f = math.Float64frombits(binary.LittleEndian.Uint64(g.data[:8]))
			g.data = g.data[8:]
		}
	}
	g.idx++
	return f
}

// officialES6StaticU64s is copied from cyberphone/json-canonicalization testdata/numgen.go.
var officialES6StaticU64s = [...]uint64{
	0x0000000000000000, 0x8000000000000000, 0x0000000000000001, 0x8000000000000001,
	0xc46696695dbd1cc3, 0xc43211ede4974a35, 0xc3fce97ca0f21056, 0xc3c7213080c1a6ac,
	0xc39280f39a348556, 0xc35d9b1f5d20d557, 0xc327af4c4a80aaac, 0xc2f2f2a36ecd5556,
	0xc2be51057e155558, 0xc28840d131aaaaac, 0xc253670dc1555557, 0xc21f0b4935555557,
	0xc1e8d5d42aaaaaac, 0xc1b3de4355555556, 0xc17fca0555555556, 0xc1496e6aaaaaaaab,
	0xc114585555555555, 0xc0e046aaaaaaaaab, 0xc0aa0aaaaaaaaaaa, 0xc074d55555555555,
	0xc040aaaaaaaaaaab, 0xc00aaaaaaaaaaaab, 0xbfd5555555555555, 0xbfa1111111111111,
	0xbf6b4e81b4e81b4f, 0xbf35d867c3ece2a5, 0xbf0179ec9cbd821e, 0xbecbf647612f3696,
	0xbe965e9f80f29212, 0xbe61e54c672874db, 0xbe2ca213d840baf8, 0xbdf6e80fe033c8c6,
	0xbdc2533fe68fd3d2, 0xbd8d51ffd74c861c, 0xbd5774ccac3d3817, 0xbd22c3d6f030f9ac,
	0xbcee0624b3818f79, 0xbcb804ea293472c7, 0xbc833721ba905bd3, 0xbc4ebe9c5db3c61e,
	0xbc18987d17c304e5, 0xbbe3ad30dfcf371d, 0xbbaf7b816618582f, 0xbb792f9ab81379bf,
	0xbb442615600f9499, 0xbb101e77800c76e1, 0xbad9ca58cce0be35, 0xbaa4a1e0a3e6fe90,
	0xba708180831f320d, 0xba3a68cd9e985016, 0x446696695dbd1cc3, 0x443211ede4974a35,
	0x43fce97ca0f21056, 0x43c7213080c1a6ac, 0x439280f39a348556, 0x435d9b1f5d20d557,
	0x4327af4c4a80aaac, 0x42f2f2a36ecd5556, 0x42be51057e155558, 0x428840d131aaaaac,
	0x4253670dc1555557, 0x421f0b4935555557, 0x41e8d5d42aaaaaac, 0x41b3de4355555556,
	0x417fca0555555556, 0x41496e6aaaaaaaab, 0x4114585555555555, 0x40e046aaaaaaaaab,
	0x40aa0aaaaaaaaaaa, 0x4074d55555555555, 0x4040aaaaaaaaaaab, 0x400aaaaaaaaaaaab,
	0x3fd5555555555555, 0x3fa1111111111111, 0x3f6b4e81b4e81b4f, 0x3f35d867c3ece2a5,
	0x3f0179ec9cbd821e, 0x3ecbf647612f3696, 0x3e965e9f80f29212, 0x3e61e54c672874db,
	0x3e2ca213d840baf8, 0x3df6e80fe033c8c6, 0x3dc2533fe68fd3d2, 0x3d8d51ffd74c861c,
	0x3d5774ccac3d3817, 0x3d22c3d6f030f9ac, 0x3cee0624b3818f79, 0x3cb804ea293472c7,
	0x3c833721ba905bd3, 0x3c4ebe9c5db3c61e, 0x3c18987d17c304e5, 0x3be3ad30dfcf371d,
	0x3baf7b816618582f, 0x3b792f9ab81379bf, 0x3b442615600f9499, 0x3b101e77800c76e1,
	0x3ad9ca58cce0be35, 0x3aa4a1e0a3e6fe90, 0x3a708180831f320d, 0x3a3a68cd9e985016,
	0x4024000000000000, 0x4014000000000000, 0x3fe0000000000000, 0x3fa999999999999a,
	0x3f747ae147ae147b, 0x3f40624dd2f1a9fc, 0x3f0a36e2eb1c432d, 0x3ed4f8b588e368f1,
	0x3ea0c6f7a0b5ed8d, 0x3e6ad7f29abcaf48, 0x3e35798ee2308c3a, 0x3ed539223589fa95,
	0x3ed4ff26cd5a7781, 0x3ed4f95a762283ff, 0x3ed4f8c60703520c, 0x3ed4f8b72f19cd0d,
	0x3ed4f8b5b31c0c8d, 0x3ed4f8b58d1c461a, 0x3ed4f8b5894f7f0e, 0x3ed4f8b588ee37f3,
	0x3ed4f8b588e47da4, 0x3ed4f8b588e3849c, 0x3ed4f8b588e36bb5, 0x3ed4f8b588e36937,
	0x3ed4f8b588e368f8, 0x3ed4f8b588e368f1, 0x3ff0000000000000, 0xbff0000000000000,
	0xbfeffffffffffffa, 0xbfeffffffffffffb, 0x3feffffffffffffa, 0x3feffffffffffffb,
	0x3feffffffffffffc, 0x3feffffffffffffe, 0xbfefffffffffffff, 0xbfefffffffffffff,
	0x3fefffffffffffff, 0x3fefffffffffffff, 0x3fd3333333333332, 0x3fd3333333333333,
	0x3fd3333333333334, 0x0010000000000000, 0x000ffffffffffffd, 0x000fffffffffffff,
	0x7fefffffffffffff, 0xffefffffffffffff, 0x4340000000000000, 0xc340000000000000,
	0x4430000000000000, 0x44b52d02c7e14af5, 0x44b52d02c7e14af6, 0x44b52d02c7e14af7,
	0x444b1ae4d6e2ef4e, 0x444b1ae4d6e2ef4f, 0x444b1ae4d6e2ef50, 0x3eb0c6f7a0b5ed8c,
	0x3eb0c6f7a0b5ed8d, 0x41b3de4355555553, 0x41b3de4355555554, 0x41b3de4355555555,
	0x41b3de4355555556, 0x41b3de4355555557, 0xbecbf647612f3696, 0x43143ff3c1cb0959,
}
