package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

//nolint:gosec // CLI-CMD-001 strict JSON decoding reads explicit operator/runtime artifact paths.
func decodeStrictJSONFile(path string, kind string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", kind, err)
	}
	return decodeStrictJSONBytes(kind, data, target)
}

func decodeStrictJSONBytes(kind string, data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", kind, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode %s: unexpected trailing json content", kind)
		}
		return fmt.Errorf("decode %s: decode trailing json token: %w", kind, err)
	}
	return nil
}
