// +build !windows,!nacl,!plan9

package main

import "github.com/bosun-monitor/scollector/_third_party/github.com/StackExchange/slog"

func init() {
	err := slog.SetSyslog()
	if err != nil {
		slog.Error(err)
	}
}
