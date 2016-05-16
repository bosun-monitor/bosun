// Package version holds some version data common to bosun and scollector.
// Most of these values will be inserted at build time with `-ldFlags` directives for official builds.
package version // import "bosun.org/_version"

import (
	"fmt"
	"time"
)

// These variables will be set at linking time for official builds.
// build.go will set date and sha, but `go get` will set none of these.
var (
	// Version number for official releases Updated manually before each release.
	Version = "0.5.0-rc3"

	// Set to any non-empty value by official release script
	OfficialBuild string
	// Date and time of build. Should be in YYYYMMDDHHMMSS format
	VersionDate string
	// VersionSHA should be set at build time as the most recent commit hash.
	VersionSHA string
)

// Get a string representing the version information for the current binary.
func GetVersionInfo(app string) string {
	var sha, build string
	version := ShortVersion()
	if buildTime, err := time.Parse("20060102150405", VersionDate); err == nil {
		build = " built " + buildTime.Format(time.RFC3339)
	}
	if VersionSHA != "" {
		sha = fmt.Sprintf(" (%s)", VersionSHA)
	}
	return fmt.Sprintf("%s version %s%s%s", app, version, sha, build)
}

func ShortVersion() string {
	version := Version

	if OfficialBuild == "" {
		version += "-dev"
	}

	return version
}
