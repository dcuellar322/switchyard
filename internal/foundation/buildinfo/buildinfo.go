// Package buildinfo exposes immutable version metadata injected at build time.
package buildinfo

import "time"

var (
	version = "dev"
	commit  = "unknown"
	builtAt = ""
)

// Info is the public build identity of the running binary.
type Info struct {
	Version string
	Commit  string
	BuiltAt *time.Time
}

// Current returns a parsed copy of the build identity.
func Current() Info {
	info := Info{Version: version, Commit: commit}
	if parsed, err := time.Parse(time.RFC3339, builtAt); err == nil {
		parsed = parsed.UTC()
		info.BuiltAt = &parsed
	}
	return info
}
