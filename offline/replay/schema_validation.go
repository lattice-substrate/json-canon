package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

func schemaPath(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("offline", "schema", name)
	}
	return filepath.Join(filepath.Dir(file), "..", "schema", name)
}

func validateSchemaBytes(kind string, schemaFile string, data []byte) error {
	result, err := gojsonschema.Validate(
		gojsonschema.NewReferenceLoader("file://"+filepath.ToSlash(schemaPath(schemaFile))),
		gojsonschema.NewBytesLoader(data),
	)
	if err != nil {
		return fmt.Errorf("validate %s schema: %w", kind, err)
	}
	if result.Valid() {
		return nil
	}
	issues := make([]string, 0, len(result.Errors()))
	for _, item := range result.Errors() {
		issues = append(issues, strings.TrimSpace(item.String()))
	}
	return fmt.Errorf("validate %s schema: %s", kind, strings.Join(issues, "; "))
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 strict JSON decoding reads explicit operator/runtime artifact paths.
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
