// Package util defines utilities for bosun.
package util // import "bosun.org/util"

import (
	"bosun.org/host"
	"regexp"
)

// This is here only until we manage to refactor more of the system, allowing us to pass a host.Manager around
// the system, rather than holding onto global state
var hostManager host.Manager

func SetHostManager(hm host.Manager) {
	hostManager = hm
}

func GetHostManager() host.Manager {
	return hostManager
}

func NameMatches(name string, regexes []*regexp.Regexp) bool {
	for _, r := range regexes {
		if r.MatchString(name) {
			return true
		}
	}
	return false
}

func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
