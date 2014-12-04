// +build !windows,!nacl,!plan9

package main

import "bosun.org/slog"

func init() {
	err := slog.SetSyslog()
	if err != nil {
		slog.Error(err)
	}
}
