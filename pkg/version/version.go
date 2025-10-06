// Package version provides version information for Hellfire
package version

var (
	// Version is the current Hellfire version
	// This will be set at build time using -ldflags
	Version = "dev"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildDate is the build date
	BuildDate = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version with git commit and build date
func GetFullVersion() string {
	return Version + " (" + GitCommit + ") built on " + BuildDate
}
