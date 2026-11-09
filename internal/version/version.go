// Package version holds build metadata injected via ldflags.
package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the human-readable version line.
func String() string {
	return fmt.Sprintf("landrop %s (%s, %s)", Version, Commit, Date)
}
