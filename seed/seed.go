// Package seed provides access to embedded seed JSON files.
package seed

import "embed"

//go:embed *.json
var FS embed.FS
