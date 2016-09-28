// Package util defines utilities for bosun.
package util // import "bosun.org/util"

import (
	"net"
	"os"
	"regexp"
	"strings"
)

var (
	// Hostname is the machine's hostname.
	Hostname string
	// FullHostname will, if false, uses the hostname upto the first ".". Run Set()
	// manually after changing.
	FullHostname bool
)

// Clean cleans a hostname based on the current FullHostname setting.
func Clean(s string) string {
	if !FullHostname {
		//only split if string is not an IP address
		ip := net.ParseIP(s)
		if ip == nil {
			s = strings.SplitN(s, ".", 2)[0]
		}
	}
	return strings.ToLower(s)
}

// Set sets Hostntame based on the current preferences.
func Set() {
	h, err := os.Hostname()
	if err != nil {
		h = "unknown"
	}
	Hostname = Clean(h)
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

func init() {
	Set()
}
