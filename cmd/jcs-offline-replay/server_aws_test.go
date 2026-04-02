package main

import (
	"bytes"
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
