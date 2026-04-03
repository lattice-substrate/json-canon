package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testIMDSToken = "token-123"

func mustWriteHTTPBody(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write http response: %v", err)
	}
}

//nolint:gocognit // REQ:AWS-GATE-001 inspect-host test fixture keeps each IMDS branch explicit.
func TestRunInspectHostSuccess(t *testing.T) {
	oldReadFile := readFileFunc
	oldClient := imdsHTTPClient
	oldAddress := imdsAddress
	t.Cleanup(func() {
		readFileFunc = oldReadFile
		imdsHTTPClient = oldClient
		imdsAddress = oldAddress
	})

	readFileFunc = func(path string) ([]byte, error) {
		switch path {
		case "/proc/cpuinfo":
			return []byte("model name\t: Example CPU\n"), nil
		case "/proc/version":
			return []byte("Linux version 6.8.0-test (builder@example) #1 SMP\n"), nil
		case "/etc/os-release":
			return []byte("ID=debian\nVERSION_ID=13\n"), nil
		default:
			return nil, errors.New("unexpected path: " + path)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			if r.Method != http.MethodPut {
				t.Fatalf("method for token = %s, want PUT", r.Method)
			}
			mustWriteHTTPBody(t, w, testIMDSToken)
		case "/latest/dynamic/instance-identity/document":
			if got := r.Header.Get("X-aws-ec2-metadata-token"); got != testIMDSToken {
				t.Fatalf("document token = %q, want %s", got, testIMDSToken)
			}
			mustWriteHTTPBody(t, w, `{"availabilityZone":"us-east-1a","imageId":"ami-123","instanceId":"i-123","region":"us-east-1"}`)
		case "/latest/dynamic/instance-identity/signature":
			if got := r.Header.Get("X-aws-ec2-metadata-token"); got != testIMDSToken {
				t.Fatalf("signature token = %q, want %s", got, testIMDSToken)
			}
			mustWriteHTTPBody(t, w, "c2lnbmVk")
		case "/latest/dynamic/instance-identity/pkcs7":
			if got := r.Header.Get("X-aws-ec2-metadata-token"); got != testIMDSToken {
				t.Fatalf("pkcs7 token = %q, want %s", got, testIMDSToken)
			}
			mustWriteHTTPBody(t, w, "cGtjczc=")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	imdsHTTPClient = srv.Client()
	imdsAddress = srv.URL

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"inspect-host"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run inspect-host exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	var got hostInspection
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode inspect-host output: %v", err)
	}
	if got.OSID != "debian" || got.OSVersionID != "13" {
		t.Fatalf("unexpected os release: %#v", got)
	}
	if got.CPU != "Example CPU" {
		t.Fatalf("cpu = %q, want Example CPU", got.CPU)
	}
	if got.Kernel != "6.8.0-test" {
		t.Fatalf("kernel = %q, want 6.8.0-test", got.Kernel)
	}
	if got.InstanceID != "i-123" || got.ImageID != "ami-123" || got.Region != "us-east-1" {
		t.Fatalf("unexpected identity fields: %#v", got)
	}
	if got.IIDDocumentSHA256 == "" || got.IIDSignatureSHA256 == "" || got.IIDPKCS7SHA256 == "" {
		t.Fatalf("expected iid hashes, got %#v", got)
	}
	if got.IIDDocument == "" || got.IIDSignature == "" || got.IIDPKCS7 == "" {
		t.Fatalf("expected raw iid material, got %#v", got)
	}
}

func TestRunInspectHostFailureWritesError(t *testing.T) {
	oldClient := imdsHTTPClient
	oldAddress := imdsAddress
	t.Cleanup(func() {
		imdsHTTPClient = oldClient
		imdsAddress = oldAddress
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no token", http.StatusForbidden)
	}))
	defer srv.Close()

	imdsHTTPClient = srv.Client()
	imdsAddress = srv.URL

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"inspect-host"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run inspect-host exit=%d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "request imds token") {
		t.Fatalf("stderr = %q, want imds token error", stderr.String())
	}
}

func TestDiscoverCPUAndOSFallbacks(t *testing.T) {
	oldReadFile := readFileFunc
	t.Cleanup(func() {
		readFileFunc = oldReadFile
	})

	readFileFunc = func(path string) ([]byte, error) {
		switch path {
		case "/proc/cpuinfo":
			return []byte("CPU architecture : 8\nCPU implementer : 0x41\nCPU part : 0xd0c\n"), nil
		case "/proc/version":
			return []byte("malformed\n"), nil
		case "/etc/os-release":
			return []byte("ID=\"amzn\"\nVERSION_ID=\"2023\"\n"), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}

	if got := discoverCPU(); got != "ARM arch 8 impl 0x41 part 0xd0c" {
		t.Fatalf("discoverCPU = %q", got)
	}
	if got := discoverKernel(); got != "" {
		t.Fatalf("discoverKernel = %q, want empty", got)
	}
	osRelease := readOSRelease()
	if osRelease["ID"] != "amzn" || osRelease["VERSION_ID"] != "2023" {
		t.Fatalf("unexpected os-release parse: %#v", osRelease)
	}
}
