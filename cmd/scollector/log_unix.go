// +build !windows,!nacl,!plan9

package main

import "bosun.org/slog"

func init() {
	err := slog.SetSyslog("scollector")
	if err != nil {
		slog.Error(err)
	}
}
