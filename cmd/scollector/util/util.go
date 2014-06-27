package util

import (
	"os"
	"strings"
)

var Hostname string

func init() {
	if h, err := os.Hostname(); err == nil {
		h = strings.SplitN(h, ".", 2)[0]
		Hostname = strings.ToLower(h)
	} else {
		Hostname = "unknown"
	}
}
