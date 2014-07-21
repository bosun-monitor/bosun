package util

import (
	"os"
	"strings"
)

var (
	Hostname string
	// FullHostname, if false, uses the hostname upto the first ".". Run Set()
	// manually after changing.
	FullHostname bool
)

// Clean cleans a hostname based on the current FullHostname setting.
func Clean(s string) string {
	if !FullHostname {
		s = strings.SplitN(s, ".", 2)[0]
	}
	return strings.ToLower(s)
}

func Set() {
	h, err := os.Hostname()
	if err == nil {
		if !FullHostname {
			h = strings.SplitN(h, ".", 2)[0]
		}
	} else {
		h = "unknown"
	}
	Hostname = Clean(h)
}

func init() {
	Set()
}
