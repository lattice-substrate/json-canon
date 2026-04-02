package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadBoundedStagingObject(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentLength int64
		maxBytes      int64
		wantErr       string
	}{
		{
			name:          "valid payload",
			body:          "evidence",
			contentLength: int64(len("evidence")),
			maxBytes:      32,
		},
		{
			name:          "declared size exceeds maximum",
			body:          "tiny",
			contentLength: 64,
			maxBytes:      32,
			wantErr:       "exceeds maximum",
		},
		{
			name:          "body exceeds maximum",
			body:          strings.Repeat("a", 33),
			contentLength: 32,
			maxBytes:      32,
			wantErr:       "exceeds maximum",
		},
		{
			name:          "declared length mismatch",
			body:          "payload",
			contentLength: 9,
			maxBytes:      32,
			wantErr:       "content length mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := readBoundedStagingObject(bytes.NewBufferString(tc.body), tc.contentLength, tc.maxBytes)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("readBoundedStagingObject: %v", err)
				}
				if string(data) != tc.body {
					t.Fatalf("payload = %q, want %q", string(data), tc.body)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("readBoundedStagingObject error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestSanitizeBucketToken(t *testing.T) {
	if got := sanitizeBucketToken("v1.2.3-rc1"); got != "1-2-3-rc1" {
		t.Fatalf("sanitizeBucketToken = %q", got)
	}
	if got := sanitizeBucketToken("   !!!   "); got != "release" {
		t.Fatalf("sanitizeBucketToken fallback = %q", got)
	}
}

func TestRandomBucketSuffix(t *testing.T) {
	oldReader := rand.Reader
	t.Cleanup(func() {
		rand.Reader = oldReader
	})

	got := randomBucketSuffix()
	if len(got) != 10 {
		t.Fatalf("randomBucketSuffix length = %d, want 10", len(got))
	}
	rand.Reader = errReader{}
	if got := randomBucketSuffix(); got != "fallback1234" {
		t.Fatalf("randomBucketSuffix fallback = %q", got)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

var _ io.Reader = errReader{}
