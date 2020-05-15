// Package util defines utilities for bosun.
package util // import "bosun.org/util"

import (
	"bosun.org/host"
	"bosun.org/slog"

	"regexp"
)

// This is here only until we manage to refactor more of the system, allowing us to pass a host.Manager around
// the system, rather than holding onto global state
var hostManager host.Manager

// InitHostManager initialises the package-wide hostManager
//
// A custom hostname can be passed in which will be used instead of the hostname the operating system reports.
// Also takes a boolean flag that indicates whether or not to use the full domain name or only the lowest level
func InitHostManager(customHostname string, useFullHostName bool) {
	var hm host.Manager
	var err error

	if customHostname != "" {
		hm, err = host.NewManagerForHostname(customHostname, useFullHostName)
	} else {
		hm, err = host.NewManager(useFullHostName)
	}

	if err != nil {
		slog.Fatalf("couldn't initialise host factory: %v", err)
	}

	SetHostManager(hm)
}

// SetHostManager sets the package-wide hostManager
func SetHostManager(hm host.Manager) {
	hostManager = hm
}

// GetHostManager returns the package-wide hostManager
func GetHostManager() host.Manager {
	return hostManager
}

// NameMatches tests if a string matches any of the given regular expressions
func NameMatches(name string, regexes []*regexp.Regexp) bool {
	for _, r := range regexes {
		if r.MatchString(name) {
			return true
		}
	}
	return false
}

// Btoi converts a bool to an integer
func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
