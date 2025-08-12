package main

import (
	"fmt"
	"runtime"
)

// Build variables - these are set via ldflags during build
var (
	Version   = "dev"       // Semantic version
	GitCommit = "unknown"   // Git commit hash
	BuildDate = "unknown"   // Build timestamp
	GoVersion = runtime.Version()
)

// VersionInfo returns formatted version information
func VersionInfo() string {
	return fmt.Sprintf("HyprMon %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, GoVersion)
}

// ShortVersion returns just the version number
func ShortVersion() string {
	return Version
}