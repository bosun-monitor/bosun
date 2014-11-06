// +build !windows,!nacl,!plan9

package main

import "github.com/StackExchange/slog"

func init() {
	err := slog.SetSyslog()
	if err != nil {
		panic(err)
	}
}
