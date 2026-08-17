// Package version carries build-time version metadata, injected via ldflags by
// the release pipeline (goreleaser).
package version

import "fmt"

// These values are overridden at build time via
// -ldflags "-X github.com/GuoMonth/dsh-isolated-runtime/internal/version.Version=…".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version line.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
