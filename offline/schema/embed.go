// Package schema provides embedded JSON schemas for internal offline replay
// validation. These are black-box implementation schemas — they live inside
// the binary and are never exposed as external API surface.
//
// The canonical schemas are owned by jcs-spec. These internal copies are
// validated for structural parity by jcs-integration-tests. If they drift,
// integration tests fail closed.
package schema

import "embed"

//go:embed *.json

// FS contains the embedded JSON schema files for internal offline replay validation.
var FS embed.FS
