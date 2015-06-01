// Package version holds some version data common to bosun and scollector.
// Most of these values will be inserted at build time with `-ldFlags` directives for official builds.
package version // import "bosun.org/version"

import (
	"fmt"
	"time"
)

// These variables will be set at linking time for official builds.
// build.go will set date and sha, but `go get` will set none of these.
var (
	// Version number for official releases Updated manually before each release.
	Version = "0.1.0"

	// Set to any non-empty value by official release script
	OfficialBuild string
	// Date and time of build. Should be in YYYYMMDDHHMMSS format
	VersionDate string
	// VersionSHA should be set at build time as the most recent commit hash.
	VersionSHA string
)

// Get a string representing the version information for the current binary.
func GetVersionInfo(app string) string {
	if OfficialBuild == "" {
		Version = Version + "-dev"
	}
	timeString := ""
	buildTime, err := time.Parse("20060102150405", VersionDate)
	if err == nil {
		timeString = " Built " + buildTime.Format(time.RFC822)
	}
	return fmt.Sprintf("%s version %v (%v)%s\n", app, Version, VersionSHA, timeString)
}
